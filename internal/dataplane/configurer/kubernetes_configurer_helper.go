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

package configurer

import (
	"context"
	"fmt"

	modelv1alpha1 "github.com/beamlit/operator/api/v1alpha1/deployment"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// mirrorEndpointSlices mirrors the endpoint slices for the given service reference and removes the target port from the user's service endpoint slice
func (s *kubernetesConfigurer) mirrorEndpointSlices(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference, targetPort int32, stopCh <-chan bool) error {
	beamlitServiceEndpoints, err := s.kubeClient.DiscoveryV1().EndpointSlices(serviceRef.Namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: "kubernetes.io/service-name=" + fmt.Sprintf("%s-beamlit", serviceRef.Name),
	})
	if err != nil {
		return err
	}

	for {
		select {
		case <-stopCh:
			beamlitServiceEndpoints.Stop()
			return nil
		case <-ctx.Done():
			beamlitServiceEndpoints.Stop()
			return ctx.Err()
		case event := <-beamlitServiceEndpoints.ResultChan():
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
				if slice.Labels["beamlit.com/to-update"] == "true" {
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
	}
}

// watchService watches the service for changes and calls the appropriate methods on the service controller
func (s *kubernetesConfigurer) watchService(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference, stopCh <-chan bool) error {
	<-stopCh
	return nil
	// TODO: fix this
	/*
		serviceWatcher, err := s.kubeClient.CoreV1().Services(serviceRef.Namespace).Watch(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("metadata.name=%s", serviceRef.Name),
		})
		if err != nil {
			return err
		}

		for {
			select {
			case <-stopCh:
				serviceWatcher.Stop()
				return nil
			case <-ctx.Done():
				serviceWatcher.Stop()
				return ctx.Err()
			case event := <-serviceWatcher.ResultChan():
				if event.Type == watch.Error {
					return fmt.Errorf("error watching service: %v", event.Object)
				}

				_, ok := event.Object.(*corev1.Service)
				if !ok {
					return fmt.Errorf("unexpected object type: %T", event.Object)
				}

				switch event.Type {
				case watch.Added, watch.Modified:
					s.Configure(ctx, serviceRef)
				case watch.Deleted:
					s.Unconfigure(ctx, serviceRef)
					return nil // we don't need to watch the service anymore
				}
			}
			}
	*/
}
