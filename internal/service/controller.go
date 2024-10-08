package service

import (
	"context"
	"fmt"

	modelv1alpha1 "github.com/beamlit/operator/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Controller is an interface for configuring services to be proxied by Beamlit.
// It replaces the EndpointsSlice in the service with a new EndpointsSlice that points to the Beamlit proxy Service.
// It also creates a new Service that can be used by the proxy to route traffic to the internal pod
type Controller interface {
	// Start starts the service configurer.
	// It starts watching for changes to the proxy service and updates the service configuration accordingly.
	Start(ctx context.Context, proxyService *modelv1alpha1.ServiceReference, gatewayService *modelv1alpha1.ServiceReference) error
	// Register registers a service to be proxied by Beamlit.
	// Under the hood, it creates a new service that proxies the service to Beamlit.
	Register(ctx context.Context, service *modelv1alpha1.ServiceReference) error
	// Unregister unregisters a service from being proxied by Beamlit.
	// It deletes the service and the EndpointsSlice from the service.
	Unregister(ctx context.Context, service *modelv1alpha1.ServiceReference) error

	// GetService gets the service for a given service reference.
	GetLocalBeamlitService(ctx context.Context, service *modelv1alpha1.ServiceReference) (*modelv1alpha1.ServiceReference, error)
}

type serviceController struct {
	gatewayServiceRef             *modelv1alpha1.ServiceReference
	proxyServiceRef               *modelv1alpha1.ServiceReference
	kubeClient                    kubernetes.Interface
	beamlitServicesByModelService map[types.NamespacedName]*types.NamespacedName
	serviceInformer               cache.SharedIndexInformer
}

func NewServiceController(ctx context.Context, kubeClient kubernetes.Interface) Controller {
	return &serviceController{
		kubeClient: kubeClient,
		serviceInformer: cache.NewSharedIndexInformer(
			&cache.ListWatch{
				ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
					return kubeClient.CoreV1().Services("").List(ctx, options)
				},
				WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
					return kubeClient.CoreV1().Services("").Watch(ctx, options)
				},
			},
			&corev1.Service{},
			0,
			cache.Indexers{},
		),
		beamlitServicesByModelService: make(map[types.NamespacedName]*types.NamespacedName),
	}
}

func (s *serviceController) Start(ctx context.Context, proxyService *modelv1alpha1.ServiceReference, gatewayService *modelv1alpha1.ServiceReference) error {
	s.proxyServiceRef = proxyService
	s.gatewayServiceRef = gatewayService
	go s.serviceInformer.Run(ctx.Done())
	return nil
}

func (s *serviceController) GetLocalBeamlitService(ctx context.Context, service *modelv1alpha1.ServiceReference) (*modelv1alpha1.ServiceReference, error) {
	serviceKey := types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}
	serviceRef, ok := s.beamlitServicesByModelService[serviceKey]
	if !ok {
		return nil, fmt.Errorf("proxy service not found for model service %s", serviceKey.String())
	}

	return &modelv1alpha1.ServiceReference{
		ObjectReference: corev1.ObjectReference{
			Namespace: serviceRef.Namespace,
			Name:      serviceRef.Name,
		},
		TargetPort: service.TargetPort,
	}, nil
}
