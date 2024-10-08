package service

import (
	"context"
	"fmt"

	modelv1alpha1 "github.com/beamlit/operator/api/v1alpha1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// mirrorEndpointSlices mirrors the endpoint slices for the given service reference and removes the target port from the user's service endpoint slice
func (s *serviceController) mirrorEndpointSlices(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference, targetPort int32) error {
	beamlitServiceEndpoints, err := s.kubeClient.DiscoveryV1().EndpointSlices(serviceRef.Namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: "kubernetes.io/service-name=" + fmt.Sprintf("%s-beamlit", serviceRef.Name),
	})
	if err != nil {
		return err
	}

	defer beamlitServiceEndpoints.Stop()

	for event := range beamlitServiceEndpoints.ResultChan() {
		if event.Type == watch.Error {
			return fmt.Errorf("error watching endpoint slice: %v", event.Object)
		}

		endpointSlice, ok := event.Object.(*discoveryv1.EndpointSlice)
		if !ok {
			return fmt.Errorf("unexpected object type: %T", event.Object)
		}

		for i, port := range endpointSlice.Ports {
			if port.Port == nil {
				continue
			}
			if *port.Port == targetPort {
				// remove the port
				endpointSlice.Ports = append(endpointSlice.Ports[:i], endpointSlice.Ports[i+1:]...)
			}
		}

		userServiceEndpointSlices, err := s.kubeClient.DiscoveryV1().EndpointSlices(serviceRef.Namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "kubernetes.io/service-name=" + serviceRef.Name,
		})

		if err != nil {
			return err
		}

		var userServiceEndpointSlice discoveryv1.EndpointSlice
		for _, slice := range userServiceEndpointSlices.Items {
			if slice.Labels["beamlit.io/to-update"] == "true" {
				userServiceEndpointSlice = slice
				break
			}
		}

		if userServiceEndpointSlice.Name == "" {
			return fmt.Errorf("second endpoint slice not found")
		}

		userServiceEndpointSlice.Ports = endpointSlice.Ports
		userServiceEndpointSlice.AddressType = endpointSlice.AddressType
		userServiceEndpointSlice.Endpoints = endpointSlice.Endpoints

		_, err = s.kubeClient.DiscoveryV1().EndpointSlices(serviceRef.Namespace).Update(ctx, &userServiceEndpointSlice, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
