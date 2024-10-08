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

	corev1 "k8s.io/api/core/v1"
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
	"github.com/beamlit/operator/internal/healthcheck"
	"github.com/beamlit/operator/internal/metrics_watcher"
	"github.com/beamlit/operator/internal/service"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "sigs.k8s.io/gateway-api/apis/applyconfiguration/apis/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1client "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

const modelDeploymentFinalizer = "modeldeployment.beamlit.io/finalizer"

// ModelDeploymentReconciler reconciles a ModelDeployment object
type ModelDeploymentReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	BeamlitClient  *beamlit.Client
	MetricsWatcher *metrics_watcher.MetricsWatcher
	Gateway        struct {
		Name      string
		Namespace string
	}
	HealthManager           *healthcheck.Manager
	OngoingOffloadings      sync.Map // key: namespace/name, value: percentage
	GatewayClient           gatewayv1client.Interface
	ServiceController       service.Controller
	DefaultRemoteServiceRef *modelv1alpha1.ServiceReference
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
	if err := r.configureOffloading(ctx, model); err != nil {
		return err
	}
	updateModelStatus(model, updatedModelDeployment)
	logger.Info("Successfully created or updated ModelDeployment", "Name", model.Name)
	return nil
}

func (r *ModelDeploymentReconciler) configureOffloading(ctx context.Context, model *modelv1alpha1.ModelDeployment) error {
	logger := log.FromContext(ctx)
	if model.Spec.OffloadingConfig == nil {
		return nil
	}
	if model.Spec.OffloadingConfig.Disabled {
		r.ServiceController.Unregister(ctx, model.Spec.OffloadingConfig.LocalServiceRef)
		r.HealthManager.RemoveWatcher(model.Spec.ModelSourceRef)
		r.MetricsWatcher.RemoveWatcher(ctx, model.Spec.ModelSourceRef)
		r.deleteHTTPRoute(ctx, model) // ignore error
		return nil
	}
	if model.Spec.OffloadingConfig.RemoteServiceRef == nil { // TODO: Make this really configurable
		model.Spec.OffloadingConfig.RemoteServiceRef = r.DefaultRemoteServiceRef
	}
	logger.Info("Registering offloading for ModelDeployment", "Name", model.Name)
	if err := r.ServiceController.Register(ctx, model.Spec.OffloadingConfig.LocalServiceRef); err != nil {
		return err
	}
	// TODO: Make condition duration configurable
	r.MetricsWatcher.Watch(ctx, model.Spec.ModelSourceRef, model.Spec.OffloadingConfig.Metrics, 5*time.Second, func(reached bool) error {
		return r.metricCallback(ctx, model, reached)
	})
	r.HealthManager.AddWatcher(ctx, model.Spec.ModelSourceRef, func(ctx context.Context, healthStatus bool) error {
		return r.healthCheckCallback(ctx, model, healthStatus)
	})
	backendServiceRef := model.Spec.OffloadingConfig.LocalServiceRef.DeepCopy()
	backendServiceRef.Name = fmt.Sprintf("%s-beamlit", backendServiceRef.Name) // TODO: Make this returned by the service controller
	if err := r.applyHTTPRoute(ctx, model, backendServiceRef, model.Spec.OffloadingConfig.RemoteServiceRef, 0); err != nil {
		return err
	}
	logger.Info("Successfully registered offloading for ModelDeployment", "Name", model.Name)
	return nil
}

func updateModelStatus(model *modelv1alpha1.ModelDeployment, _ *beamlit.ModelDeployment) {
	// TODO: Set AvailableReplicas, DesiredReplicas ...
	return
}

func (r *ModelDeploymentReconciler) finalizeModel(ctx context.Context, model *modelv1alpha1.ModelDeployment) error {
	logger := log.FromContext(ctx)
	logger.Info("Finalizing ModelDeployment", "Name", model.Name)
	if err := r.BeamlitClient.DeleteModelDeployment(ctx, model.Name); err != nil {
		logger.Error(err, "Failed to delete ModelDeployment")
		return err
	}
	r.OngoingOffloadings.Delete(fmt.Sprintf("%s/%s", model.Namespace, model.Name))
	logger.Info("Successfully deleted offloading for ModelDeployment", "Name", model.Name)
	r.deleteHTTPRoute(ctx, model) // ignore error
	r.ServiceController.Unregister(ctx, model.Spec.OffloadingConfig.LocalServiceRef)
	r.HealthManager.RemoveWatcher(model.Spec.ModelSourceRef)
	r.MetricsWatcher.RemoveWatcher(ctx, model.Spec.ModelSourceRef)
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
		if value, ok := r.OngoingOffloadings.Load(fmt.Sprintf("%s/%s", model.Namespace, model.Name)); ok {
			if value.(int) == 100 {
				return nil // probably already offload for unhealthy status
			}
			r.OngoingOffloadings.Delete(fmt.Sprintf("%s/%s", model.Namespace, model.Name))
			localServiceRef, err := r.ServiceController.GetLocalBeamlitService(ctx, model.Spec.OffloadingConfig.LocalServiceRef)
			if err != nil {
				return err
			}
			return r.applyHTTPRoute(ctx, model, localServiceRef, model.Spec.OffloadingConfig.RemoteServiceRef, 0)
		}
		return nil
	}
	if _, ok := r.OngoingOffloadings.Load(fmt.Sprintf("%s/%s", model.Namespace, model.Name)); !ok {
		localServiceRef, err := r.ServiceController.GetLocalBeamlitService(ctx, model.Spec.OffloadingConfig.LocalServiceRef)
		if err != nil {
			return err
		}
		if err := r.applyHTTPRoute(ctx, model, localServiceRef, model.Spec.OffloadingConfig.RemoteServiceRef, int(model.Spec.OffloadingConfig.Behavior.Percentage)); err != nil {
			return err
		}
		r.OngoingOffloadings.Store(fmt.Sprintf("%s/%s", model.Namespace, model.Name), int(model.Spec.OffloadingConfig.Behavior.Percentage))
		logger.Info("Successfully offloaded model deployment", "Name", model.Name, "Namespace", model.Namespace)
	}
	return nil
}

func (r *ModelDeploymentReconciler) healthCheckCallback(ctx context.Context, model *modelv1alpha1.ModelDeployment, healthStatus bool) error {
	logger := log.FromContext(ctx)
	logger.Info("Health check callback for ModelDeployment", "Name", model.Name, "healthStatus", healthStatus)
	if !healthStatus {
		// 100% offload
		localServiceRef, err := r.ServiceController.GetLocalBeamlitService(ctx, model.Spec.OffloadingConfig.LocalServiceRef)
		if err != nil {
			return err
		}
		if err := r.applyHTTPRoute(ctx, model, localServiceRef, model.Spec.OffloadingConfig.RemoteServiceRef, 100); err != nil {
			return err
		}
		r.OngoingOffloadings.Store(fmt.Sprintf("%s/%s", model.Namespace, model.Name), 100)
		logger.Info("Successfully offloaded model deployment", "Name", model.Name, "Namespace", model.Namespace)
		return nil
	}
	if value, ok := r.OngoingOffloadings.Load(fmt.Sprintf("%s/%s", model.Namespace, model.Name)); ok {
		if value.(int) == int(model.Spec.OffloadingConfig.Behavior.Percentage) {
			return nil
		}
		localServiceRef, err := r.ServiceController.GetLocalBeamlitService(ctx, model.Spec.OffloadingConfig.LocalServiceRef)
		if err != nil {
			return err
		}
		// If the health check is successful, we need to offload back to the original percentage
		if err := r.applyHTTPRoute(ctx, model, localServiceRef, model.Spec.OffloadingConfig.RemoteServiceRef, int(model.Spec.OffloadingConfig.Behavior.Percentage)); err != nil {
			return err
		}
		r.OngoingOffloadings.Store(fmt.Sprintf("%s/%s", model.Namespace, model.Name), int(model.Spec.OffloadingConfig.Behavior.Percentage))
		logger.Info("Successfully offloaded model deployment", "Name", model.Name, "Namespace", model.Namespace)
	}
	return nil
}

func (r *ModelDeploymentReconciler) applyHTTPRoute(
	ctx context.Context,
	model *modelv1alpha1.ModelDeployment,
	backendServiceRef *modelv1alpha1.ServiceReference,
	remoteServiceRef *modelv1alpha1.ServiceReference,
	backendWeight int,
) error {
	service := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: model.Spec.OffloadingConfig.LocalServiceRef.Namespace,
		Name:      model.Spec.OffloadingConfig.LocalServiceRef.Name,
	}, service); err != nil {
		return err
	}

	token, err := beamlit.NewBeamlitToken()
	if err != nil {
		return err
	}
	tokenString, err := token.GetToken(ctx)
	if err != nil {
		return err
	}

	httpRouteApply := v1.HTTPRoute(fmt.Sprintf("%s-http-route", model.Name), service.Namespace).
		WithSpec(v1.HTTPRouteSpec().
			WithParentRefs(v1.ParentReference().
				WithName(gatewayv1.ObjectName(r.Gateway.Name)).
				WithKind(gatewayv1.Kind("Gateway")).
				WithGroup(gatewayv1.Group("gateway.networking.k8s.io")).
				WithNamespace(gatewayv1.Namespace(r.Gateway.Namespace))).
			WithHostnames(
				gatewayv1.Hostname(service.Spec.ClusterIP),
				gatewayv1.Hostname(service.Name),
				gatewayv1.Hostname(fmt.Sprintf("%s.%s", service.Name, service.Namespace)),
				gatewayv1.Hostname(fmt.Sprintf("%s.%s.svc", service.Name, service.Namespace)),
				gatewayv1.Hostname(fmt.Sprintf("%s.%s.svc.cluster.local", service.Name, service.Namespace)),
			).
			WithRules(
				v1.HTTPRouteRule().
					WithBackendRefs(
						v1.HTTPBackendRef().
							WithKind(gatewayv1.Kind("Service")).
							WithName(gatewayv1.ObjectName(backendServiceRef.Name)).
							WithNamespace(gatewayv1.Namespace(backendServiceRef.Namespace)).
							WithPort(gatewayv1.PortNumber(backendServiceRef.TargetPort)).
							WithWeight(int32(100-backendWeight)),
						v1.HTTPBackendRef().
							WithKind(gatewayv1.Kind("Service")).
							WithName(gatewayv1.ObjectName(remoteServiceRef.Name)).
							WithNamespace(gatewayv1.Namespace(remoteServiceRef.Namespace)).
							WithPort(gatewayv1.PortNumber(remoteServiceRef.TargetPort)).
							WithWeight(int32(backendWeight))).
					WithMatches(v1.HTTPRouteMatch().
						WithPath(v1.HTTPPathMatch().
							WithType(gatewayv1.PathMatchType("PathPrefix")).
							WithValue("/"))).
					WithFilters(v1.HTTPRouteFilter().
						WithType(gatewayv1.HTTPRouteFilterType("RequestHeaderModifier")).
						WithRequestHeaderModifier(v1.HTTPHeaderFilter().
							WithSet(
								v1.HTTPHeader().WithName("X-Beamlit-Model").WithValue(model.Name),
								v1.HTTPHeader().WithName("X-Beamlit-Authorization").WithValue("Bearer "+tokenString),
							)))))

	_, err = r.GatewayClient.GatewayV1().HTTPRoutes(service.Namespace).Apply(ctx, httpRouteApply, metav1.ApplyOptions{
		FieldManager: "beamlit-operator",
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *ModelDeploymentReconciler) deleteHTTPRoute(ctx context.Context, model *modelv1alpha1.ModelDeployment) error {
	logger := log.FromContext(ctx)
	logger.Info("Deleting HTTPRoute for ModelDeployment", "Name", model.Name)
	err := r.GatewayClient.GatewayV1().HTTPRoutes(model.Namespace).Delete(ctx, fmt.Sprintf("%s-http-route", model.Name), metav1.DeleteOptions{})
	if err != nil {
		logger.Error(err, "Failed to delete HTTPRoute")
		return err
	}
	logger.Info("Successfully deleted HTTPRoute for ModelDeployment", "Name", model.Name)
	return nil
}
