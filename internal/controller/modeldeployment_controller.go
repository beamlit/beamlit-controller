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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/beamlit/operator/api/v1alpha1/deployment"
	"github.com/beamlit/operator/internal/beamlit"
	"github.com/beamlit/operator/internal/controller/helper"
	"github.com/beamlit/operator/internal/dataplane/configurer"
	"github.com/beamlit/operator/internal/dataplane/offloader"
	"github.com/beamlit/operator/internal/informers/health"
	"github.com/beamlit/operator/internal/informers/metric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const modelDeploymentFinalizer = "modeldeployment.beamlit.com/finalizer"

// ModelDeploymentReconciler reconciles a ModelDeployment object

type ManagedModel struct {
	namespace      string
	name           string
	healthy        bool
	lastGeneration int64
}

type ModelDeploymentReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	BeamlitClient *beamlit.Client

	Offloader        offloader.Offloader
	Configurer       configurer.Configurer
	MetricInformer   metric.MetricInformer
	HealthInformer   health.HealthInformer
	HealthStatusChan <-chan health.HealthStatus
	MetricStatusChan <-chan metric.MetricStatus

	OngoingOffloadings sync.Map // key: namespace/name, value: percentage
	ModelState         sync.Map // key: namespace/name, value: modelState
	ManagedModels      map[string]ManagedModel
	BeamlitModels      map[string]string // key: spec.model/spec.environment, value: modelDeployment name

	DefaultRemoteBackend *v1alpha1.RemoteBackend
}

// +kubebuilder:rbac:groups=deployment.beamlit.com,resources=modeldeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=deployment.beamlit.com,resources=modeldeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=deployment.beamlit.com,resources=modeldeployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments/scale,verbs=get;list;watch
// +kubebuilder:rbac:groups=metrics.k8s.io,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch;create;update;patch;delete

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
		r.ModelState.Delete(fmt.Sprintf("%s/%s", model.Namespace, model.Name))
		r.OngoingOffloadings.Delete(fmt.Sprintf("%s/%s", model.Namespace, model.Name))
		delete(r.ManagedModels, fmt.Sprintf("%s/%s", model.Namespace, model.Name))
		return ctrl.Result{}, err
	}
	logger.V(0).Info("Successfully created or updated ModelDeployment", "Name", model.Name)
	return ctrl.Result{}, nil
}

func (r *ModelDeploymentReconciler) createOrUpdate(ctx context.Context, model *v1alpha1.ModelDeployment) error {
	logger := log.FromContext(ctx)
	if value, ok := r.BeamlitModels[fmt.Sprintf("%s/%s", model.Spec.Environment, model.Spec.Model)]; ok {
		if value != model.Name {
			logger.V(1).Error(nil, "ModelDeployment already exists on Beamlit with a different name inside the cluster", "Name", model.Name, "ExistingName", value)
			return nil
		}
	}
	if value, ok := r.ManagedModels[fmt.Sprintf("%s/%s", model.Namespace, model.Name)]; ok {
		if value.lastGeneration == model.Generation {
			logger.V(1).Info("ModelDeployment generation has not changed, skipping", "Name", model.Name)
			return nil
		}
	}
	logger.V(1).Info("Converting ModelDeployment to Beamlit ModelDeployment", "Name", model.Name)
	if model.Spec.ServiceRef != nil {
		servingPort, err := helper.RetrievePodPort(ctx, r.Client, &v1.ObjectReference{
			Kind:      model.Spec.ServiceRef.Kind,
			Namespace: model.Spec.ServiceRef.Namespace,
			Name:      model.Spec.ServiceRef.Name,
		}, int(model.Spec.ServiceRef.TargetPort))
		if err != nil {
			logger.V(0).Error(err, "Failed to retrieve serving port for ModelDeployment", "Name", model.Name)
			return err
		}
		model.Status.ServingPort = int32(servingPort)
	}
	if model.Spec.MetricServiceRef != nil {
		metricPort, err := helper.RetrievePodPort(ctx, r.Client, &v1.ObjectReference{
			Kind:      model.Spec.MetricServiceRef.Kind,
			Namespace: model.Spec.MetricServiceRef.Namespace,
			Name:      model.Spec.MetricServiceRef.Name,
		}, int(model.Spec.MetricServiceRef.TargetPort))
		if err != nil {
			logger.V(0).Error(err, "Failed to retrieve metric port for ModelDeployment", "Name", model.Name)
			return err
		}
		model.Status.MetricPort = int32(metricPort)
	}
	beamlitModelDeployment, err := helper.ToBeamlitModelDeployment(ctx, r.Client, model)
	if err != nil {
		logger.V(0).Error(err, "Failed to convert ModelDeployment to Beamlit ModelDeployment")
		return err
	}
	r.BeamlitModels[fmt.Sprintf("%s/%s", model.Spec.Environment, model.Spec.Model)] = model.Name
	logger.V(1).Info("Creating or updating ModelDeployment on Beamlit", "Name", model.Name)
	updatedModelDeployment, err := r.BeamlitClient.CreateOrUpdateModelDeployment(ctx, beamlitModelDeployment)
	if err != nil {
		logger.V(0).Error(err, "Failed to create or update ModelDeployment on Beamlit")
		return err
	}
	model.Status.Workspace = *updatedModelDeployment.Workspace
	createdAt, err := time.Parse(time.RFC3339, *updatedModelDeployment.CreatedAt)
	if err != nil {
		logger.V(0).Error(err, "Failed to parse CreatedAt on Beamlit", "Name", model.Name)
		return err
	}
	model.Status.CreatedAtOnBeamlit = metav1.NewTime(createdAt)
	updatedAt, err := time.Parse(time.RFC3339, *updatedModelDeployment.UpdatedAt)
	if err != nil {
		logger.V(0).Error(err, "Failed to parse UpdatedAt on Beamlit", "Name", model.Name)
		return err
	}
	model.Status.UpdatedAtOnBeamlit = metav1.NewTime(updatedAt)
	if err := r.configureOffloading(ctx, model); err != nil {
		logger.V(0).Error(err, "Failed to configure offloading for ModelDeployment")
		return err
	}
	logger.V(1).Info("Successfully configured offloading for ModelDeployment", "Name", model.Name)
	if err := r.Status().Update(ctx, model); err != nil {
		logger.V(0).Error(err, "Failed to update ModelDeployment")
		return err
	}

	r.ManagedModels[fmt.Sprintf("%s/%s", model.Namespace, model.Name)] = ManagedModel{
		namespace:      model.Namespace,
		name:           model.Name,
		healthy:        true,
		lastGeneration: model.Generation,
	}

	return nil
}

func (r *ModelDeploymentReconciler) configureOffloading(ctx context.Context, model *v1alpha1.ModelDeployment) error {
	logger := log.FromContext(ctx)
	if model.Spec.OffloadingConfig == nil {
		return nil
	}
	_, present := r.OngoingOffloadings.Load(fmt.Sprintf("%s/%s", model.Namespace, model.Name))
	if !model.Spec.Enabled && present {
		logger.V(1).Info("Unregistering offloading for ModelDeployment", "Name", model.Name)
		if err := r.Configurer.Unconfigure(ctx, model.Spec.ServiceRef); err != nil {
			logger.V(0).Error(err, "Failed to unconfigure local service for ModelDeployment")
			return err
		}
		r.HealthInformer.Unregister(ctx, fmt.Sprintf("%s/%s", model.Namespace, model.Name))
		r.MetricInformer.Unregister(ctx, fmt.Sprintf("%s/%s", model.Namespace, model.Name))
		if err := r.Offloader.Cleanup(ctx, model); err != nil {
			logger.V(0).Error(err, "Failed to cleanup offloading for ModelDeployment")
			return err
		}
		r.OngoingOffloadings.Delete(fmt.Sprintf("%s/%s", model.Namespace, model.Name))
		r.ModelState.Delete(fmt.Sprintf("%s/%s", model.Namespace, model.Name))
		delete(r.ManagedModels, fmt.Sprintf("%s/%s", model.Namespace, model.Name))
		logger.V(1).Info("Successfully unregistered offloading for ModelDeployment", "Name", model.Name)
		return nil
	}
	if model.Spec.OffloadingConfig.RemoteBackend == nil { // TODO: Make this really configurable
		logger.V(1).Info("Setting default remote service reference for ModelDeployment", "Name", model.Name)
		model.Spec.OffloadingConfig.RemoteBackend = r.DefaultRemoteBackend
	}
	logger.V(1).Info("Registering local service for ModelDeployment", "Name", model.Name)
	if err := r.Configurer.Configure(ctx, model.Spec.ServiceRef); err != nil {
		logger.V(0).Error(err, "Failed to configure offloading for ModelDeployment")
		return err
	}
	logger.V(1).Info("Successfully configured local service for ModelDeployment", "Name", model.Name)
	r.OngoingOffloadings.Store(fmt.Sprintf("%s/%s", model.Namespace, model.Name), 0)
	r.ModelState.Store(fmt.Sprintf("%s/%s", model.Namespace, model.Name), true)
	// TODO: Make condition duration configurable
	logger.V(1).Info("Registering metrics watcher for ModelDeployment", "Name", model.Name)
	r.MetricInformer.Register(ctx, fmt.Sprintf("%s/%s", model.Namespace, model.Name), model.Spec.OffloadingConfig.Metrics, model.Spec.ModelSourceRef, 5*time.Second, 5*time.Second)
	logger.V(1).Info("Successfully registered metrics watcher for ModelDeployment", "Name", model.Name)
	logger.V(1).Info("Registering health watcher for ModelDeployment", "Name", model.Name)
	r.HealthInformer.Register(ctx, fmt.Sprintf("%s/%s", model.Namespace, model.Name), model.Spec.ModelSourceRef)
	logger.V(1).Info("Successfully registered health watcher for ModelDeployment", "Name", model.Name)
	backendServiceRef := model.Spec.ServiceRef.DeepCopy()
	backendServiceRef.Name = fmt.Sprintf("%s-beamlit", backendServiceRef.Name) // TODO: Make this returned by the service controller
	logger.V(1).Info("Configuring offloading for ModelDeployment", "Name", model.Name)
	if err := r.Offloader.Configure(ctx, model, backendServiceRef, model.Spec.OffloadingConfig.RemoteBackend, 0); err != nil {
		logger.V(0).Error(err, "Failed to configure offloading for ModelDeployment")
		return err
	}
	logger.V(1).Info("Successfully registered offloading for ModelDeployment", "Name", model.Name)
	return nil
}

func (r *ModelDeploymentReconciler) finalizeModel(ctx context.Context, model *v1alpha1.ModelDeployment) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Finalizing ModelDeployment", "Name", model.Name)
	delete(r.BeamlitModels, fmt.Sprintf("%s/%s", model.Spec.Environment, model.Spec.Model))
	r.OngoingOffloadings.Delete(fmt.Sprintf("%s/%s", model.Namespace, model.Name))
	r.ModelState.Delete(fmt.Sprintf("%s/%s", model.Namespace, model.Name))
	delete(r.ManagedModels, fmt.Sprintf("%s/%s", model.Namespace, model.Name))
	logger.V(1).Info("Successfully deleted offloading for ModelDeployment", "Name", model.Name)
	if model.Spec.OffloadingConfig == nil {
		return nil
	}
	logger.V(1).Info("Successfully cleaned up offloading for ModelDeployment", "Name", model.Name)
	if err := r.Configurer.Unconfigure(ctx, model.Spec.ServiceRef); err != nil {
		logger.V(0).Error(err, "Failed to unconfigure local service for ModelDeployment")
		return err
	}
	if err := r.Offloader.Cleanup(ctx, model); err != nil {
		logger.V(0).Error(err, "Failed to cleanup offloading for ModelDeployment")
		return err
	}
	logger.V(1).Info("Successfully unregistered local service for ModelDeployment", "Name", model.Name)
	r.MetricInformer.Unregister(ctx, fmt.Sprintf("%s/%s", model.Namespace, model.Name))
	logger.V(1).Info("Successfully removed metrics watcher for ModelDeployment", "Name", model.Name)
	r.HealthInformer.Unregister(ctx, fmt.Sprintf("%s/%s", model.Namespace, model.Name))
	logger.V(1).Info("Successfully removed health watcher for ModelDeployment", "Name", model.Name)
	if err := r.BeamlitClient.DeleteModelDeployment(ctx, model.Spec.Model, model.Spec.Environment); err != nil {
		logger.V(0).Error(err, "Failed to delete ModelDeployment")
		return err
	}
	logger.V(1).Info("Successfully deleted ModelDeployment", "Name", model.Name)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ModelDeployment{}).
		Complete(r)
}

func (r *ModelDeploymentReconciler) WatchForInformerUpdates(ctx context.Context) error {
	logger := log.FromContext(ctx)
	for {
		select {
		case <-ctx.Done():
			logger.V(0).Info("Stopping watch for informer updates")
			return nil
		case healthStatus := <-r.HealthStatusChan:
			logger.V(1).Info("Health status update", "ModelName", healthStatus.ModelName, "HealthStatus", healthStatus.Healthy)
			if value, ok := r.ManagedModels[healthStatus.ModelName]; ok {
				model := &v1alpha1.ModelDeployment{}
				logger.V(1).Info("Getting ModelDeployment", "Name", value.name)
				if err := r.Client.Get(ctx, types.NamespacedName{Namespace: value.namespace, Name: value.name}, model); err != nil {
					logger.V(0).Error(err, "Failed to get ModelDeployment", "Name", value.name)
					continue
				}
				if model.Spec.OffloadingConfig.RemoteBackend == nil {
					model.Spec.OffloadingConfig.RemoteBackend = r.DefaultRemoteBackend
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
				if !value.healthy {
					logger.V(1).Info("Skipping metric callback for ModelDeployment because it is not healthy", "Name", metricStatus.ModelName)
					continue
				}
				model := &v1alpha1.ModelDeployment{}
				logger.V(1).Info("Getting ModelDeployment", "Name", value.name)
				if err := r.Client.Get(ctx, types.NamespacedName{Namespace: value.namespace, Name: value.name}, model); err != nil {
					logger.V(0).Error(err, "Failed to get ModelDeployment", "Name", value.name)
					continue
				}
				if model.Spec.OffloadingConfig.RemoteBackend == nil {
					model.Spec.OffloadingConfig.RemoteBackend = r.DefaultRemoteBackend
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

func (r *ModelDeploymentReconciler) metricCallback(ctx context.Context, model *v1alpha1.ModelDeployment, reached bool) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Metric callback for ModelDeployment", "Name", model.Name, "reached", reached)
	if value, ok := r.ModelState.Load(fmt.Sprintf("%s/%s", model.Namespace, model.Name)); ok {
		if !value.(bool) {
			return nil
		}
	}
	value, ok := r.OngoingOffloadings.Load(fmt.Sprintf("%s/%s", model.Namespace, model.Name))
	if !ok {
		return nil
	}
	if !reached {
		if value.(int) != 0 {
			logger.V(1).Info("Offloading model deployment to 0%", "Name", model.Name)
			r.OngoingOffloadings.Delete(fmt.Sprintf("%s/%s", model.Namespace, model.Name))
			localServiceRef, err := r.Configurer.GetLocalBeamlitService(ctx, model.Spec.ServiceRef)
			if err != nil {
				logger.V(0).Error(err, "Failed to get local service for ModelDeployment", "Name", model.Name)
				return err
			}
			if err := r.Offloader.Configure(ctx, model, localServiceRef, model.Spec.OffloadingConfig.RemoteBackend, 0); err != nil {
				logger.V(0).Error(err, "Failed to offload model deployment to 0%", "Name", model.Name)
				return err
			}
			if err := r.notifyOnBeamlit(ctx, model, false); err != nil {
				logger.V(0).Error(err, "Failed to notify on Beamlit", "Name", model.Name)
			}
			logger.V(1).Info("Successfully offloaded model deployment to 0%", "Name", model.Name)
		}
		return nil
	}
	if value.(int) != int(model.Spec.OffloadingConfig.Behavior.Percentage) {
		logger.V(1).Info("Offloading model deployment", "Name", model.Name, "Percentage", model.Spec.OffloadingConfig.Behavior.Percentage)
		localServiceRef, err := r.Configurer.GetLocalBeamlitService(ctx, model.Spec.ServiceRef)
		if err != nil {
			logger.V(0).Error(err, "Failed to get local service for ModelDeployment", "Name", model.Name)
			return err
		}
		if err := r.Offloader.Configure(ctx, model, localServiceRef, model.Spec.OffloadingConfig.RemoteBackend, int(model.Spec.OffloadingConfig.Behavior.Percentage)); err != nil {
			logger.V(0).Error(err, "Failed to offload model deployment", "Name", model.Name)
			return err
		}
		r.OngoingOffloadings.Store(fmt.Sprintf("%s/%s", model.Namespace, model.Name), int(model.Spec.OffloadingConfig.Behavior.Percentage))
		if err := r.notifyOnBeamlit(ctx, model, true); err != nil {
			logger.V(0).Error(err, "Failed to notify on Beamlit", "Name", model.Name)
		}
		logger.V(1).Info("Successfully offloaded model deployment", "Name", model.Name, "Namespace", model.Namespace)
	}
	return nil
}

func (r *ModelDeploymentReconciler) healthCheckCallback(ctx context.Context, model *v1alpha1.ModelDeployment, healthStatus bool) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Health check callback for ModelDeployment", "Name", model.Name, "healthStatus", healthStatus)
	if !healthStatus {
		// 100% offload
		logger.V(1).Info("Offloading model deployment to 100% due to unhealthy status", "Name", model.Name)
		localServiceRef, err := r.Configurer.GetLocalBeamlitService(ctx, model.Spec.ServiceRef)
		if err != nil {
			logger.V(0).Error(err, "Failed to get local service for ModelDeployment", "Name", model.Name)
			return err
		}
		if err := r.Offloader.Configure(ctx, model, localServiceRef, model.Spec.OffloadingConfig.RemoteBackend, 100); err != nil {
			logger.V(0).Error(err, "Failed to offload model deployment to 100%", "Name", model.Name)
			return err
		}
		r.OngoingOffloadings.Store(fmt.Sprintf("%s/%s", model.Namespace, model.Name), 100)
		if err := r.notifyOnBeamlit(ctx, model, true); err != nil {
			logger.V(0).Error(err, "Failed to notify on Beamlit", "Name", model.Name)
		}
		r.ModelState.Store(fmt.Sprintf("%s/%s", model.Namespace, model.Name), false)
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
		localServiceRef, err := r.Configurer.GetLocalBeamlitService(ctx, model.Spec.ServiceRef)
		if err != nil {
			logger.V(0).Error(err, "Failed to get local service for ModelDeployment", "Name", model.Name)
			return err
		}
		// If the health check is successful, we need to offload back to the original percentage
		if err := r.Offloader.Configure(ctx, model, localServiceRef, model.Spec.OffloadingConfig.RemoteBackend, int(model.Spec.OffloadingConfig.Behavior.Percentage)); err != nil {
			return err
		}
		r.OngoingOffloadings.Store(fmt.Sprintf("%s/%s", model.Namespace, model.Name), int(model.Spec.OffloadingConfig.Behavior.Percentage))
		if err := r.notifyOnBeamlit(ctx, model, true); err != nil {
			logger.V(0).Error(err, "Failed to notify on Beamlit", "Name", model.Name)
		}
		r.ModelState.Store(fmt.Sprintf("%s/%s", model.Namespace, model.Name), true)
		logger.V(1).Info("Successfully offloaded model deployment", "Name", model.Name, "Namespace", model.Namespace)
	}
	return nil
}

func (r *ModelDeploymentReconciler) notifyOnBeamlit(ctx context.Context, model *v1alpha1.ModelDeployment, offloading bool) error {
	return r.BeamlitClient.NotifyOnModelOffloading(ctx, model.Spec.Model, model.Spec.Environment, offloading)
}

func remove(slice []types.NamespacedName, item types.NamespacedName) []types.NamespacedName {
	for i, v := range slice {
		if v == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

func (r *ModelDeploymentReconciler) PolicyUpdate(ctx context.Context) error {
	return nil
	/*
		logger := log.FromContext(ctx)
		resourceInterface := r.DynamicClient.Resource(schema.GroupVersionResource{
			Group:    "authorization.beamlit.com",
			Version:  "v1alpha1",
			Resource: "Policy",
		})

		logger.V(1).Info("Watching for Policy updates")
		watchInterface, err := resourceInterface.Watch(ctx, metav1.ListOptions{})
		if err != nil {
			logger.V(0).Error(err, "Failed to watch for Policy updates")
			return err
		}

		defer watchInterface.Stop()

		for {
			select {
			case event := <-watchInterface.ResultChan():
				logger.V(1).Info("Policy event", "Type", event.Type, "Object", event.Object)
				if event.Type == watch.Deleted {
					policy := event.Object.(*authorizationv1alpha1.Policy)
					if r.PolicyPerModels[policy.Name] == nil {
						continue
					}
					for _, model := range r.PolicyPerModels[policy.Name] {
						logger.V(1).Info("Removing policy from ModelDeployment", "ModelDeployment", model, "Policy", policy.Name)
						// patch model deployment to remove policy
						modelDeployment := &v1alpha1.ModelDeployment{}
						if err := r.Client.Get(ctx, model, modelDeployment); err != nil {
							logger.V(0).Error(err, "Failed to get ModelDeployment", "ModelDeployment", model)
							continue
						}
						modelDeployment.Spec.Policies = removePolicy(modelDeployment.Spec.Policies, policy.Name)
						if err := r.Client.Update(ctx, modelDeployment); err != nil {
							logger.V(0).Error(err, "Failed to update ModelDeployment", "ModelDeployment", model)
							continue
						}
						managedModel := r.ManagedModels[fmt.Sprintf("%s/%s", model.Namespace, model.Name)]
						managedModel.lastGeneration = modelDeployment.Generation
						r.ManagedModels[fmt.Sprintf("%s/%s", model.Namespace, model.Name)] = managedModel
					}
					delete(r.PolicyPerModels, policy.Name)
				}
			case <-ctx.Done():
				return nil
			}
		}
	*/
}

func removePolicy(policies []v1alpha1.PolicyRef, policyName string) []v1alpha1.PolicyRef {
	for i, policy := range policies {
		if policy.Name == policyName {
			return append(policies[:i], policies[i+1:]...)
		}
	}
	return policies
}
