package service

import (
	"context"
	"fmt"

	modelv1alpha1 "github.com/beamlit/operator/api/v1alpha1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (s *serviceController) deleteBeamlitService(ctx context.Context, service *modelv1alpha1.ServiceReference) error {
	delete(s.beamlitServicesByModelService, types.NamespacedName{Namespace: service.Namespace, Name: service.Name})
	return s.kubeClient.CoreV1().Services(service.Namespace).Delete(ctx, fmt.Sprintf("%s-beamlit", service.Name), metav1.DeleteOptions{})
}

func (s *serviceController) addKubernetesManagedEndpointsSlice(ctx context.Context, service *modelv1alpha1.ServiceReference) error {
	_, err := s.kubeClient.DiscoveryV1().EndpointSlices(service.Namespace).Create(ctx, &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-kubernetes-managed", service.Name),
			Labels: map[string]string{
				"kubernetes.io/service-name":             service.Name,
				"endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io",
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (s *serviceController) deleteBeamlitEndpointsSlice(ctx context.Context, service *modelv1alpha1.ServiceReference) error {
	endpointSlices, err := s.kubeClient.DiscoveryV1().EndpointSlices(service.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "kubernetes.io/service-name=" + service.Name,
	})
	if err != nil {
		return err
	}

	for _, endpointSlice := range endpointSlices.Items {
		if value, ok := endpointSlice.Labels["endpointslice.kubernetes.io/managed-by"]; ok && value == "beamlit-operator" {
			return s.kubeClient.DiscoveryV1().EndpointSlices(service.Namespace).Delete(ctx, endpointSlice.Name, metav1.DeleteOptions{})
		}
	}
	return nil
}

func (s *serviceController) deletePortFromGatewayService(ctx context.Context, service *modelv1alpha1.ServiceReference) error {
	// TODO: Implement this
	return nil
}

func (s *serviceController) Unregister(ctx context.Context, service *modelv1alpha1.ServiceReference) error {
	if s.beamlitServicesByModelService[types.NamespacedName{Namespace: service.Namespace, Name: service.Name}] == nil {
		return nil
	}

	err := s.addKubernetesManagedEndpointsSlice(ctx, service)
	if err != nil {
		return err
	}

	err = s.deleteBeamlitEndpointsSlice(ctx, service)
	if err != nil {
		return err
	}

	err = s.deletePortFromGatewayService(ctx, service)
	if err != nil {
		return err
	}

	err = s.deleteBeamlitService(ctx, service)
	if err != nil {
		return err
	}

	return nil
}
