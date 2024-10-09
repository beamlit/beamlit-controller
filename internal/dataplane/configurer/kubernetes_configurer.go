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
	"slices"

	modelv1alpha1 "github.com/beamlit/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	v1 "k8s.io/client-go/applyconfigurations/core/v1"
	discoveryv1apply "k8s.io/client-go/applyconfigurations/discovery/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type kubernetesConfigurer struct {
	gatewayServiceRef             *modelv1alpha1.ServiceReference
	proxyServiceRef               *modelv1alpha1.ServiceReference
	kubeClient                    kubernetes.Interface
	beamlitServicesByModelService map[types.NamespacedName]*types.NamespacedName
	serviceInformer               cache.SharedIndexInformer
}

func newKubernetesConfigurer(ctx context.Context, kubeClient kubernetes.Interface) (Configurer, error) {
	return &kubernetesConfigurer{
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
	}, nil
}

func (s *kubernetesConfigurer) Start(ctx context.Context, proxyService *modelv1alpha1.ServiceReference, gatewayService *modelv1alpha1.ServiceReference) error {
	s.proxyServiceRef = proxyService
	s.gatewayServiceRef = gatewayService
	go s.serviceInformer.Run(ctx.Done())
	return nil
}

func (s *kubernetesConfigurer) GetLocalBeamlitService(ctx context.Context, service *modelv1alpha1.ServiceReference) (*modelv1alpha1.ServiceReference, error) {
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

func (s *kubernetesConfigurer) Configure(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference) error {
	beamlitService, err := s.createBeamlitModelService(ctx, serviceRef)
	if err != nil {
		return err
	}
	err = s.addPortToGatewayService(ctx, serviceRef)
	if err != nil {
		return err
	}
	err = s.takeOverEndpointsSlices(ctx, serviceRef)
	if err != nil {
		return err
	}

	var targetPort int32
	for _, port := range beamlitService.Spec.Ports {
		if port.Port == serviceRef.TargetPort {
			targetPort = port.TargetPort.IntVal
			break
		}
	}
	if targetPort == 0 {
		return fmt.Errorf("target port not found in beamlit service %s", beamlitService.Name)
	}

	err = s.createMirroredEndpointsSlice(ctx, serviceRef, targetPort)
	if err != nil {
		return err
	}

	err = s.cleanUnusedEndpointSlices(ctx, serviceRef)
	if err != nil {
		return err
	}

	go s.watchService(ctx, serviceRef)

	return nil
}

func (s *kubernetesConfigurer) createBeamlitModelService(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference) (*corev1.Service, error) {
	serviceToConfigure, err := s.kubeClient.CoreV1().Services(serviceRef.Namespace).Get(ctx, serviceRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	beamlitServiceApplyConfig := v1.Service(fmt.Sprintf("%s-beamlit", serviceRef.Name), serviceRef.Namespace).
		WithSpec(v1.ServiceSpec().
			WithPorts(func() []*v1.ServicePortApplyConfiguration {
				var ports []*v1.ServicePortApplyConfiguration
				for _, port := range serviceToConfigure.Spec.Ports {
					ports = append(ports, v1.ServicePort().
						WithName(port.Name).
						WithPort(port.Port).
						WithProtocol(port.Protocol).
						WithTargetPort(intstr.FromInt(int(port.TargetPort.IntVal))),
					)
				}
				return ports
			}()...).
			WithSelector(serviceToConfigure.Spec.Selector).
			WithType(corev1.ServiceTypeClusterIP))

	beamlitService, err := s.kubeClient.CoreV1().Services(serviceRef.Namespace).Apply(ctx, beamlitServiceApplyConfig, metav1.ApplyOptions{
		FieldManager: "beamlit-operator",
	})
	if err != nil {
		return nil, err
	}

	s.beamlitServicesByModelService[types.NamespacedName{Namespace: serviceRef.Namespace, Name: serviceRef.Name}] = &types.NamespacedName{Namespace: beamlitService.Namespace, Name: beamlitService.Name}

	return beamlitService, nil
}

// addPortToGatewayService adds a port to the gateway service.
// It checks if the port already exists, and if it does, it returns.
// Otherwise, it adds the port to the gateway service.
func (s *kubernetesConfigurer) addPortToGatewayService(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference) error {
	gatewayService, err := s.kubeClient.CoreV1().Services(s.gatewayServiceRef.Namespace).Get(ctx, s.gatewayServiceRef.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	serviceToConfigure, err := s.kubeClient.CoreV1().Services(serviceRef.Namespace).Get(ctx, serviceRef.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var portToAdd corev1.ServicePort
	for _, port := range serviceToConfigure.Spec.Ports {
		if port.Port == serviceRef.TargetPort {
			portToAdd = port
			break
		}
	}

	for _, port := range gatewayService.Spec.Ports {
		if port.Port == serviceRef.TargetPort && port.Protocol == portToAdd.Protocol {
			return nil
		}
	}

	gatewayService.Spec.Ports = append(gatewayService.Spec.Ports, corev1.ServicePort{
		Name:       fmt.Sprintf("%d-beamlit", serviceRef.TargetPort),
		Port:       serviceRef.TargetPort,
		Protocol:   portToAdd.Protocol,
		TargetPort: intstr.FromInt(int(s.gatewayServiceRef.TargetPort)),
	})

	gatewayService.Spec.ExternalIPs = append(gatewayService.Spec.ExternalIPs, serviceToConfigure.Spec.ClusterIPs...)
	_, err = s.kubeClient.CoreV1().Services(s.gatewayServiceRef.Namespace).Update(ctx, gatewayService, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

// takeOverEndpointsSlice takes over the endpoints slice for a given service reference.
// It updates the label of the endpoints slice.
func (s *kubernetesConfigurer) takeOverEndpointsSlices(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference) error {
	userServiceEndpoints, err := s.kubeClient.DiscoveryV1().EndpointSlices(serviceRef.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "kubernetes.io/service-name=" + serviceRef.Name,
	})
	if err != nil {
		return err
	}

	for _, endpoint := range userServiceEndpoints.Items {
		endpoint.Labels["endpointslice.kubernetes.io/managed-by"] = "beamlit-operator"
		endpoint.Labels["kubernetes.io/service-name"] = serviceRef.Name
		_, err = s.kubeClient.DiscoveryV1().EndpointSlices(serviceRef.Namespace).Update(ctx, &endpoint, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

// createMirroredEndpointsSlice creates a mirrored endpoints slice for a given service reference.
// It mirrors the endpoints slice of the model beamlit service created for the user service, minus the service target port.
func (s *kubernetesConfigurer) createMirroredEndpointsSlice(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference, targetPort int32) error {
	mirroredEndpointsSlice, err := s.kubeClient.DiscoveryV1().EndpointSlices(serviceRef.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "kubernetes.io/service-name=" + fmt.Sprintf("%s-beamlit", serviceRef.Name),
	})
	if err != nil {
		return err
	}
	if len(mirroredEndpointsSlice.Items) == 0 {
		return fmt.Errorf("no mirrored endpoints slice found for service %s", serviceRef.Name)
	}

	if len(mirroredEndpointsSlice.Items) > 1 {
		return fmt.Errorf("multiple mirrored endpoints slices found for service %s, this shouldn't happen", serviceRef.Name)
	}

	esApplyConfig := discoveryv1apply.EndpointSlice(fmt.Sprintf("%s-beamlit-mirrored", serviceRef.Name), serviceRef.Namespace).
		WithAddressType(mirroredEndpointsSlice.Items[0].AddressType).
		WithLabels(map[string]string{
			"beamlit.io/to-update":                   "true",
			"kubernetes.io/service-name":             serviceRef.Name,
			"endpointslice.kubernetes.io/managed-by": "beamlit-operator",
		})

	var endpoints []*discoveryv1apply.EndpointApplyConfiguration
	for _, endpoint := range mirroredEndpointsSlice.Items[0].Endpoints {
		endpointApply := discoveryv1apply.Endpoint().
			WithAddresses(endpoint.Addresses...).
			WithConditions(discoveryv1apply.EndpointConditions().
				WithReady(*endpoint.Conditions.Ready).
				WithServing(*endpoint.Conditions.Serving).
				WithTerminating(*endpoint.Conditions.Terminating))

		if endpoint.Hostname != nil {
			endpointApply.WithHostname(*endpoint.Hostname)
		}
		if endpoint.NodeName != nil {
			endpointApply.WithNodeName(*endpoint.NodeName)
		}
		endpoints = append(endpoints, endpointApply)
	}
	esApplyConfig.WithEndpoints(endpoints...)

	var ports []*discoveryv1apply.EndpointPortApplyConfiguration
	for _, port := range mirroredEndpointsSlice.Items[0].Ports {
		if port.Port != nil && *port.Port == targetPort {
			continue
		}
		portApply := discoveryv1apply.EndpointPort()
		if port.Name != nil {
			portApply.WithName(*port.Name)
		}
		if port.Port != nil {
			portApply.WithPort(*port.Port)
		}
		if port.Protocol != nil {
			portApply.WithProtocol(*port.Protocol)
		}
		ports = append(ports, portApply)
	}
	esApplyConfig.WithPorts(ports...)

	_, err = s.kubeClient.DiscoveryV1().EndpointSlices(serviceRef.Namespace).Apply(ctx, esApplyConfig, metav1.ApplyOptions{
		FieldManager: "beamlit-operator",
		Force:        true,
	})
	if err != nil {
		return err
	}

	go s.mirrorEndpointSlices(ctx, serviceRef, targetPort)

	return nil
}

func (s *kubernetesConfigurer) cleanUnusedEndpointSlices(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference) error {
	userServiceEndpoints, err := s.kubeClient.DiscoveryV1().EndpointSlices(serviceRef.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "kubernetes.io/service-name=" + serviceRef.Name,
	})
	if err != nil {
		return err
	}

	for _, endpoint := range userServiceEndpoints.Items {
		if endpoint.Name == fmt.Sprintf("%s-beamlit-mirrored", serviceRef.Name) {
			continue
		}
		endpoint.Labels["endpointslice.kubernetes.io/managed-by"] = "beamlit-operator"
		endpoint.Endpoints = nil
		endpoint.Ports = nil
		_, err = s.kubeClient.DiscoveryV1().EndpointSlices(serviceRef.Namespace).Update(ctx, &endpoint, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *kubernetesConfigurer) Unconfigure(ctx context.Context, service *modelv1alpha1.ServiceReference) error {
	if value, ok := s.beamlitServicesByModelService[types.NamespacedName{Namespace: service.Namespace, Name: service.Name}]; !ok || value == nil {
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
	err = s.deleteExternalIPsFromGatewayService(ctx, service)
	if err != nil {
		return err
	}
	return nil
}

func (s *kubernetesConfigurer) deleteBeamlitService(ctx context.Context, service *modelv1alpha1.ServiceReference) error {
	beamlitService, ok := s.beamlitServicesByModelService[types.NamespacedName{Namespace: service.Namespace, Name: service.Name}]
	if !ok {
		return fmt.Errorf("beamlit service not found for model service %s", service.Name)
	}
	delete(s.beamlitServicesByModelService, types.NamespacedName{Namespace: service.Namespace, Name: service.Name})
	return s.kubeClient.CoreV1().Services(beamlitService.Namespace).Delete(ctx, beamlitService.Name, metav1.DeleteOptions{})
}

func (s *kubernetesConfigurer) addKubernetesManagedEndpointsSlice(ctx context.Context, service *modelv1alpha1.ServiceReference) error {
	_, err := s.kubeClient.DiscoveryV1().EndpointSlices(service.Namespace).Create(ctx, &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-kubernetes-managed", service.Name),
			Labels: map[string]string{
				"kubernetes.io/service-name":             service.Name,
				"endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io",
			},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   []discoveryv1.Endpoint{},
		Ports:       []discoveryv1.EndpointPort{},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (s *kubernetesConfigurer) deleteBeamlitEndpointsSlice(ctx context.Context, service *modelv1alpha1.ServiceReference) error {
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

func (s *kubernetesConfigurer) deleteExternalIPsFromGatewayService(ctx context.Context, service *modelv1alpha1.ServiceReference) error {
	gatewayService, err := s.kubeClient.CoreV1().Services(s.gatewayServiceRef.Namespace).Get(ctx, s.gatewayServiceRef.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	userService, err := s.kubeClient.CoreV1().Services(service.Namespace).Get(ctx, service.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	var externalIPs []string
	for _, externalIP := range gatewayService.Spec.ExternalIPs {
		if !slices.Contains(userService.Spec.ClusterIPs, externalIP) {
			externalIPs = append(externalIPs, externalIP)
		}
	}
	gatewayService.Spec.ExternalIPs = externalIPs
	_, err = s.kubeClient.CoreV1().Services(s.gatewayServiceRef.Namespace).Update(ctx, gatewayService, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
