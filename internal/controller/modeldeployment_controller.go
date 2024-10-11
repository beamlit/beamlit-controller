/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/beamlit/operator/api/v1alpha1"
	modelv1alpha1 "github.com/beamlit/operator/api/v1alpha1"
	"github.com/beamlit/operator/internal/beamlit"
	"github.com/beamlit/operator/internal/controller/helper"
	"github.com/beamlit/operator/internal/dataplane/configurer"
	"github.com/beamlit/operator/internal/dataplane/offloader"
	"github.com/beamlit/operator/internal/informers/health"
	"github.com/beamlit/operator/internal/informers/metric"
	v1 "k8s.io/api/core/v1"
)

const modelDeploymentFinalizer = "modeldeployment.beamlit.io/finalizer"

// ModelDeploymentReconciler reconciles a ModelDeployment object
type ModelDeploymentReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	BeamlitClient           *beamlit.Client
	MetricInformer          metric.MetricInformer
	HealthInformer          health.HealthInformer
	HealthStatusChan        <-chan health.HealthStatus
	MetricStatusChan        <-chan metric.MetricStatus
	OngoingOffloadings      sync.Map // key: namespace/name, value: percentage
	Offloader               offloader.Offloader
	Configurer              configurer.Configurer
	DefaultRemoteServiceRef *modelv1alpha1.ServiceReference
	ManagedModels           map[string]v1.ObjectReference
}

// +kubebuilder:rbac:groups=model.beamlit.io,resources=modeldeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=model.beamlit.io,resources=modeldeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=model.beamlit.io,resources=modeldeployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ModelDeployment object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *ModelDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(0).Info("Reconciling ModelDeployment", "Name", req.NamespacedName)
	var model v1alpha1.ModelDeployment
	if err := r.Get(ctx, req.NamespacedName, &model); err != nil {
		if errors.IsNotFound(err) {
			logger.V(0).Info("ModelDeployment not found", "Name", req.NamespacedName)
			return ctrl.Result{}, nil
		}
		logger.V(0).Error(err, "Failed to get ModelDeployment")
		return ctrl.Result{}, err
	}

	if model.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(&model, modelDeploymentFinalizer) {
			logger.V(0).Info("Finalizing ModelDeployment", "Name", model.Name)
			if err := r.finalizeModel(ctx, &model); err != nil {
				logger.V(0).Error(err, "Failed to finalize ModelDeployment")
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(&model, modelDeploymentFinalizer)
			if err := r.Update(ctx, &model); err != nil {
				logger.V(0).Error(err, "Failed to update ModelDeployment")
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&model, modelDeploymentFinalizer) {
		logger.V(0).Info("Adding finalizer to ModelDeployment", "Name", model.Name)
		controllerutil.AddFinalizer(&model, modelDeploymentFinalizer)
		if err := r.Update(ctx, &model); err != nil {
			logger.V(0).Error(err, "Failed to update ModelDeployment")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if err := r.createOrUpdate(ctx, &model); err != nil {
		if errors.IsConflict(err) {
			logger.V(0).Info("Conflict detected, retrying", "error", err)
			return ctrl.Result{Requeue: true}, nil
		}
		logger.V(0).Error(err, "Failed to create or update ModelDeployment")
		return ctrl.Result{}, err
	}
	logger.V(0).Info("Successfully created or updated ModelDeployment", "Name", model.Name)
	return ctrl.Result{}, nil
}

func (r *ModelDeploymentReconciler) createOrUpdate(ctx context.Context, model *modelv1alpha1.ModelDeployment) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Converting ModelDeployment to Beamlit ModelDeployment", "Name", model.Name)
	beamlitModelDeployment, err := helper.ToBeamlitModelDeployment(ctx, r.Client, model)
	if err != nil {
		logger.V(0).Error(err, "Failed to convert ModelDeployment to Beamlit ModelDeployment")
		return err
	}
	logger.V(1).Info("Creating or updating ModelDeployment on Beamlit", "Name", model.Name)
	updatedModelDeployment, err := r.BeamlitClient.CreateOrUpdateModelDeployment(ctx, &beamlitModelDeployment)
	if err != nil {
		logger.V(0).Error(err, "Failed to create or update ModelDeployment on Beamlit")
		return err
	}
	logger.V(1).Info("Successfully created or updated ModelDeployment on Beamlit", "Name", model.Name)
	if err := r.configureOffloading(ctx, model); err != nil {
		logger.V(0).Error(err, "Failed to configure offloading for ModelDeployment")
		return err
	}
	logger.V(1).Info("Successfully configured offloading for ModelDeployment", "Name", model.Name)
	updateModelStatus(model, updatedModelDeployment)
	logger.V(1).Info("Successfully updated ModelDeployment status", "Name", model.Name)
	return nil
}

func (r *ModelDeploymentReconciler) configureOffloading(ctx context.Context, model *modelv1alpha1.ModelDeployment) error {
	logger := log.FromContext(ctx)
	if model.Spec.OffloadingConfig == nil {
		return nil
	}
	if model.Spec.OffloadingConfig.Disabled {
		logger.V(1).Info("Unregistering offloading for ModelDeployment", "Name", model.Name)
		if err := r.Configurer.Unconfigure(ctx, model.Spec.OffloadingConfig.LocalServiceRef); err != nil {
			logger.V(0).Error(err, "Failed to unconfigure local service for ModelDeployment")
			return err
		}
		r.HealthInformer.Unregister(ctx, model.Name)
		r.MetricInformer.Unregister(ctx, model.Name)
		if err := r.Offloader.Cleanup(ctx, model); err != nil {
			logger.V(0).Error(err, "Failed to cleanup offloading for ModelDeployment")
			return err
		}
		logger.V(1).Info("Successfully unregistered offloading for ModelDeployment", "Name", model.Name)
		return nil
	}
	if model.Spec.OffloadingConfig.RemoteServiceRef == nil { // TODO: Make this really configurable
		logger.V(1).Info("Setting default remote service reference for ModelDeployment", "Name", model.Name)
		model.Spec.OffloadingConfig.RemoteServiceRef = r.DefaultRemoteServiceRef
	}
	logger.V(1).Info("Registering local service for ModelDeployment", "Name", model.Name)
	if err := r.Configurer.Configure(ctx, model.Spec.OffloadingConfig.LocalServiceRef); err != nil {
		logger.V(0).Error(err, "Failed to configure offloading for ModelDeployment")
		return err
	}
	logger.V(1).Info("Successfully configured local service for ModelDeployment", "Name", model.Name)
	// TODO: Make condition duration configurable
	logger.V(1).Info("Registering metrics watcher for ModelDeployment", "Name", model.Name)
	r.MetricInformer.Register(ctx, model.Name, model.Spec.OffloadingConfig.Metrics, model.Spec.ModelSourceRef, 5*time.Second, 5*time.Second)
	logger.V(1).Info("Successfully registered metrics watcher for ModelDeployment", "Name", model.Name)
	logger.V(1).Info("Registering health watcher for ModelDeployment", "Name", model.Name)
	r.HealthInformer.Register(ctx, model.Name, model.Spec.ModelSourceRef)
	logger.V(1).Info("Successfully registered health watcher for ModelDeployment", "Name", model.Name)
	backendServiceRef := model.Spec.OffloadingConfig.LocalServiceRef.DeepCopy()
	backendServiceRef.Name = fmt.Sprintf("%s-beamlit", backendServiceRef.Name) // TODO: Make this returned by the service controller
	logger.V(1).Info("Configuring offloading for ModelDeployment", "Name", model.Name)
	if err := r.Offloader.Configure(ctx, model, backendServiceRef, model.Spec.OffloadingConfig.RemoteServiceRef, 0); err != nil {
		logger.V(0).Error(err, "Failed to configure offloading for ModelDeployment")
		return err
	}
	logger.V(1).Info("Successfully registered offloading for ModelDeployment", "Name", model.Name)
	return nil
}

func updateModelStatus(model *modelv1alpha1.ModelDeployment, _ *beamlit.ModelDeployment) {
	// TODO: Set AvailableReplicas, DesiredReplicas ...
	return
}

func (r *ModelDeploymentReconciler) finalizeModel(ctx context.Context, model *modelv1alpha1.ModelDeployment) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Finalizing ModelDeployment", "Name", model.Name)
	if err := r.BeamlitClient.DeleteModelDeployment(ctx, model.Name); err != nil {
		logger.V(0).Error(err, "Failed to delete ModelDeployment")
		return err
	}
	r.OngoingOffloadings.Delete(fmt.Sprintf("%s/%s", model.Namespace, model.Name))
	logger.V(1).Info("Successfully deleted offloading for ModelDeployment", "Name", model.Name)
	if err := r.Offloader.Cleanup(ctx, model); err != nil {
		logger.V(0).Error(err, "Failed to cleanup offloading for ModelDeployment")
		return err
	}
	logger.V(1).Info("Successfully cleaned up offloading for ModelDeployment", "Name", model.Name)
	if err := r.Configurer.Unconfigure(ctx, model.Spec.OffloadingConfig.LocalServiceRef); err != nil {
		logger.V(0).Error(err, "Failed to unconfigure local service for ModelDeployment")
		return err
	}
	logger.V(1).Info("Successfully unregistered local service for ModelDeployment", "Name", model.Name)
	r.HealthInformer.Unregister(ctx, model.Name)
	logger.V(1).Info("Successfully removed metrics watcher for ModelDeployment", "Name", model.Name)
	logger.V(1).Info("Successfully finalized ModelDeployment", "Name", model.Name)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&modelv1alpha1.ModelDeployment{}).
		Complete(r)
}

func (r *ModelDeploymentReconciler) watchForInformerUpdates(ctx context.Context) error {
	logger := log.FromContext(ctx)
	for {
		select {
		case <-ctx.Done():
			logger.V(0).Info("Stopping watch for informer updates")
			return nil
		case healthStatus := <-r.HealthStatusChan:
			logger.V(1).Info("Health status update", "ModelName", healthStatus.ModelName, "HealthStatus", healthStatus.Healthy)
			if value, ok := r.ManagedModels[healthStatus.ModelName]; ok {
				model := &modelv1alpha1.ModelDeployment{}
				logger.V(1).Info("Getting ModelDeployment", "Name", value.Name)
				if err := r.Client.Get(ctx, types.NamespacedName{Namespace: value.Namespace, Name: value.Name}, model); err != nil {
					logger.V(0).Error(err, "Failed to get ModelDeployment", "Name", value.Name)
					continue
				}
				logger.V(1).Info("Handling health check callback for ModelDeployment", "Name", model.Name)
				if err := r.healthCheckCallback(ctx, model, healthStatus.Healthy); err != nil {
					logger.V(0).Error(err, "Failed to handle health check callback for ModelDeployment", "Name", model.Name)
					continue
				}
				logger.V(1).Info("Successfully handled health check callback for ModelDeployment", "Name", model.Name)
			}
		case metricStatus := <-r.MetricStatusChan:
			logger.V(1).Info("Metric status update", "ModelName", metricStatus.ModelName, "MetricStatus", metricStatus.Reached)
			if value, ok := r.ManagedModels[metricStatus.ModelName]; ok {
				model := &modelv1alpha1.ModelDeployment{}
				logger.V(1).Info("Getting ModelDeployment", "Name", value.Name)
				if err := r.Client.Get(ctx, types.NamespacedName{Namespace: value.Namespace, Name: value.Name}, model); err != nil {
					logger.V(0).Error(err, "Failed to get ModelDeployment", "Name", value.Name)
					continue
				}
				logger.V(1).Info("Handling metric callback for ModelDeployment", "Name", model.Name)
				if err := r.metricCallback(ctx, model, metricStatus.Reached); err != nil {
					logger.V(0).Error(err, "Failed to handle metric callback for ModelDeployment", "Name", model.Name)
					continue
				}
				logger.V(1).Info("Successfully handled metric callback for ModelDeployment", "Name", model.Name)
			}
		}
	}
}

func (r *ModelDeploymentReconciler) metricCallback(ctx context.Context, model *modelv1alpha1.ModelDeployment, reached bool) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Metric callback for ModelDeployment", "Name", model.Name, "reached", reached)
	if !reached {
		if value, ok := r.OngoingOffloadings.Load(fmt.Sprintf("%s/%s", model.Namespace, model.Name)); ok {
			logger.V(1).Info("Metric not reached for ModelDeployment", "Name", model.Name)
			if value.(int) == 100 {
				logger.V(1).Info("Already offloaded to 100% for ModelDeployment", "Name", model.Name)
				return nil // probably already offload for unhealthy status
			}
			logger.V(1).Info("Offloading model deployment to 0%", "Name", model.Name)
			r.OngoingOffloadings.Delete(fmt.Sprintf("%s/%s", model.Namespace, model.Name))
			localServiceRef, err := r.Configurer.GetLocalBeamlitService(ctx, model.Spec.OffloadingConfig.LocalServiceRef)
			if err != nil {
				logger.V(0).Error(err, "Failed to get local service for ModelDeployment", "Name", model.Name)
				return err
			}

			if err := r.Offloader.Configure(ctx, model, localServiceRef, model.Spec.OffloadingConfig.RemoteServiceRef, 0); err != nil {
				logger.V(0).Error(err, "Failed to offload model deployment to 0%", "Name", model.Name)
				return err
			}
			logger.V(1).Info("Successfully offloaded model deployment to 0%", "Name", model.Name)
		}
		return nil
	}
	if _, ok := r.OngoingOffloadings.Load(fmt.Sprintf("%s/%s", model.Namespace, model.Name)); !ok {
		logger.V(1).Info("Offloading model deployment", "Name", model.Name, "Percentage", model.Spec.OffloadingConfig.Behavior.Percentage)
		localServiceRef, err := r.Configurer.GetLocalBeamlitService(ctx, model.Spec.OffloadingConfig.LocalServiceRef)
		if err != nil {
			logger.V(0).Error(err, "Failed to get local service for ModelDeployment", "Name", model.Name)
			return err
		}
		if err := r.Offloader.Configure(ctx, model, localServiceRef, model.Spec.OffloadingConfig.RemoteServiceRef, int(model.Spec.OffloadingConfig.Behavior.Percentage)); err != nil {
			logger.V(0).Error(err, "Failed to offload model deployment", "Name", model.Name)
			return err
		}
		r.OngoingOffloadings.Store(fmt.Sprintf("%s/%s", model.Namespace, model.Name), int(model.Spec.OffloadingConfig.Behavior.Percentage))
		logger.V(1).Info("Successfully offloaded model deployment", "Name", model.Name, "Namespace", model.Namespace)
	}
	return nil
}

func (r *ModelDeploymentReconciler) healthCheckCallback(ctx context.Context, model *modelv1alpha1.ModelDeployment, healthStatus bool) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Health check callback for ModelDeployment", "Name", model.Name, "healthStatus", healthStatus)
	if !healthStatus {
		// 100% offload
		logger.V(1).Info("Offloading model deployment to 100% due to unhealthy status", "Name", model.Name)
		localServiceRef, err := r.Configurer.GetLocalBeamlitService(ctx, model.Spec.OffloadingConfig.LocalServiceRef)
		if err != nil {
			logger.V(0).Error(err, "Failed to get local service for ModelDeployment", "Name", model.Name)
			return err
		}
		if err := r.Offloader.Configure(ctx, model, localServiceRef, model.Spec.OffloadingConfig.RemoteServiceRef, 100); err != nil {
			logger.V(0).Error(err, "Failed to offload model deployment to 100%", "Name", model.Name)
			return err
		}
		r.OngoingOffloadings.Store(fmt.Sprintf("%s/%s", model.Namespace, model.Name), 100)
		logger.V(1).Info("Successfully offloaded model deployment", "Name", model.Name, "Namespace", model.Namespace)
		return nil
	}
	if value, ok := r.OngoingOffloadings.Load(fmt.Sprintf("%s/%s", model.Namespace, model.Name)); ok {
		logger.V(1).Info("Checking if model deployment is already offloaded to desired percentage", "Name", model.Name, "Percentage", value.(int))
		if value.(int) == int(model.Spec.OffloadingConfig.Behavior.Percentage) {
			logger.V(1).Info("Model deployment is already offloaded to desired percentage", "Name", model.Name, "Percentage", value.(int))
			return nil
		}
		logger.V(1).Info("Offloading model deployment back to desired percentage", "Name", model.Name, "Percentage", model.Spec.OffloadingConfig.Behavior.Percentage)
		localServiceRef, err := r.Configurer.GetLocalBeamlitService(ctx, model.Spec.OffloadingConfig.LocalServiceRef)
		if err != nil {
			logger.V(0).Error(err, "Failed to get local service for ModelDeployment", "Name", model.Name)
			return err
		}
		// If the health check is successful, we need to offload back to the original percentage
		if err := r.Offloader.Configure(ctx, model, localServiceRef, model.Spec.OffloadingConfig.RemoteServiceRef, int(model.Spec.OffloadingConfig.Behavior.Percentage)); err != nil {
			return err
		}
		r.OngoingOffloadings.Store(fmt.Sprintf("%s/%s", model.Namespace, model.Name), int(model.Spec.OffloadingConfig.Behavior.Percentage))
		logger.V(1).Info("Successfully offloaded model deployment", "Name", model.Name, "Namespace", model.Namespace)
	}
	return nil
}
