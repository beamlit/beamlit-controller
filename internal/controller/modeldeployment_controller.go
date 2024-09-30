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
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/beamlit/operator/api/v1alpha1"
	modelv1alpha1 "github.com/beamlit/operator/api/v1alpha1"
	"github.com/beamlit/operator/internal/beamlit"
	"github.com/beamlit/operator/internal/metrics_watcher"
	"github.com/beamlit/operator/internal/offloading"
	"github.com/beamlit/operator/internal/proxy"
)

const modelDeploymentFinalizer = "modeldeployment.beamlit.io/finalizer"

// ModelDeploymentReconciler reconciles a ModelDeployment object
type ModelDeploymentReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	BeamlitClient           *beamlit.Client
	MetricsWatcher          *metrics_watcher.MetricsWatcher
	Offloadings             sync.Map
	Proxy                   *proxy.Proxy
	Offloader               *offloading.Offloader
	DefaultRemoteServiceRef *modelv1alpha1.ServiceReference
}

// +kubebuilder:rbac:groups=model.beamlit.io,resources=modeldeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=model.beamlit.io,resources=modeldeployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=model.beamlit.io,resources=modeldeployments/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch
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

	var model v1alpha1.ModelDeployment
	if err := r.Get(ctx, req.NamespacedName, &model); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if model.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(&model, modelDeploymentFinalizer) {
			if err := r.finalizeModel(ctx, &model); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(&model, modelDeploymentFinalizer)
			if err := r.Update(ctx, &model); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&model, modelDeploymentFinalizer) {
		controllerutil.AddFinalizer(&model, modelDeploymentFinalizer)
		if err := r.Update(ctx, &model); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.createOrUpdate(ctx, &model); err != nil {
		if errors.IsConflict(err) {
			logger.Info("Conflict detected, retrying", "error", err)
			return ctrl.Result{Requeue: true}, nil
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ModelDeploymentReconciler) createOrUpdate(ctx context.Context, model *modelv1alpha1.ModelDeployment) error {
	logger := log.FromContext(ctx)
	logger.Info("Creating or updating ModelDeployment", "Name", model.Name)
	beamlitModelDeployment, err := Convert(ctx, r.Client, model)
	if err != nil {
		return err
	}
	updatedModelDeployment, err := r.BeamlitClient.CreateOrUpdateModelDeployment(ctx, &beamlitModelDeployment)
	if err != nil {
		return err
	}
	if model.Spec.OffloadingConfig != nil && !model.Spec.OffloadingConfig.Disabled {
		logger.Info("Watching metrics for ModelDeployment", "Name", model.Name)
		var modelCopy = model.DeepCopy()
		// TODO: Make condition duration configurable
		r.MetricsWatcher.Watch(ctx, modelCopy.Spec.ModelSourceRef, modelCopy.Spec.OffloadingConfig.Metrics, 5*time.Second, func(reached bool) error {
			return r.metricCallback(ctx, modelCopy, reached)
		})
		if modelCopy.Spec.OffloadingConfig.RemoteServiceRef == nil {
			logger.Info("Setting default remote service reference for ModelDeployment", "Name", model.Name)
			modelCopy.Spec.OffloadingConfig.RemoteServiceRef = r.DefaultRemoteServiceRef
		}
		logger.Info("Registering offloading for ModelDeployment", "Name", model.Name)
		if err := r.Offloader.Register(ctx, modelCopy.Spec.OffloadingConfig.LocalServiceRef, modelCopy.Spec.OffloadingConfig.RemoteServiceRef, model.Name); err != nil {
			return err
		}
	}
	updateModelStatus(model, updatedModelDeployment)
	if err := r.Status().Update(ctx, model); err != nil {
		return err
	}
	logger.Info("Successfully created or updated ModelDeployment", "Name", model.Name)
	return nil
}

func updateModelStatus(model *modelv1alpha1.ModelDeployment, _ *beamlit.ModelDeployment) {
	// TODO: Set AvailableReplicas, DesiredReplicas ...

	// Update ScalingStatus - always Active on Beamlit side
	model.Status.ScalingStatus = &modelv1alpha1.ScalingStatus{
		Status: "Active", // TODO: Change to actual status
	}

	model.Status.ScalingStatus.HPARef = nil

	// Update HPARef if ScalingConfig is present
	if model.Spec.ScalingConfig != nil && model.Spec.ScalingConfig.HPARef != nil {
		model.Status.ScalingStatus.HPARef = &corev1.ObjectReference{
			Kind:       "HorizontalPodAutoscaler",
			Name:       model.Spec.ScalingConfig.HPARef.Name,
			Namespace:  model.Spec.ScalingConfig.HPARef.Namespace,
			APIVersion: "autoscaling/v2",
		}
	}

	model.Status.OffloadingStatus = nil

	// Update OffloadingStatus if OffloadingConfig is present
	if model.Spec.OffloadingConfig != nil {
		model.Status.OffloadingStatus = &modelv1alpha1.OffloadingStatus{
			Status:   "Active",
			Behavior: model.Spec.OffloadingConfig.Behavior,
			Metrics:  model.Spec.OffloadingConfig.Metrics,
		}
		if model.Spec.OffloadingConfig.LocalServiceRef != nil {
			model.Status.OffloadingStatus.LocalServiceRef = &model.Spec.OffloadingConfig.LocalServiceRef.ObjectReference
		}
	}
}

func (r *ModelDeploymentReconciler) finalizeModel(ctx context.Context, model *modelv1alpha1.ModelDeployment) error {
	logger := log.FromContext(ctx)
	logger.Info("Finalizing ModelDeployment", "Name", model.Name)
	if err := r.BeamlitClient.DeleteModelDeployment(ctx, model.Name); err != nil {
		logger.Error(err, "Failed to delete ModelDeployment")
		return err
	}
	if _, ok := r.Offloadings.LoadAndDelete(model.Name); ok {
		logger.Info("Successfully deleted offloading for ModelDeployment", "Name", model.Name)
		if err := r.Offloader.Unregister(ctx, model.Spec.OffloadingConfig.LocalServiceRef); err != nil {
			return err
		}
	}
	logger.Info("Successfully finalized ModelDeployment", "Name", model.Name)
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&modelv1alpha1.ModelDeployment{}).
		Complete(r)
}

func (r *ModelDeploymentReconciler) metricCallback(ctx context.Context, model *modelv1alpha1.ModelDeployment, reached bool) error {
	logger := log.FromContext(ctx)
	logger.Info("Metric callback for ModelDeployment", "Name", model.Name, "reached", reached)
	if !reached {
		if _, ok := r.Offloadings.LoadAndDelete(model.Name); ok {
			return r.Offloader.Offload(ctx, model.Spec.OffloadingConfig.LocalServiceRef, 0)
		}
		return nil
	}
	if _, ok := r.Offloadings.Load(model.Name); !ok {
		if err := r.Offloader.Offload(ctx, model.Spec.OffloadingConfig.LocalServiceRef, int(model.Spec.OffloadingConfig.Behavior.Percentage)); err != nil {
			return err
		}
	}
	r.Offloadings.Store(model.Name, true)
	logger.Info("Successfully stored offloading for ModelDeployment", "Name", model.Name)
	return nil
}
