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
	"time"

	modelv1alpha1 "github.com/beamlit/beamlit-controller/api/v1alpha1/deployment"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	v1 "k8s.io/client-go/applyconfigurations/core/v1"
	discoveryv1apply "k8s.io/client-go/applyconfigurations/discovery/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type kubernetesConfigurer struct {
	gatewayServiceRef              *modelv1alpha1.ServiceReference
	kubeClient                     kubernetes.Interface
	beamlitServicesByModelService  map[types.NamespacedName]*types.NamespacedName
	initialEndpointPerLocalService map[types.NamespacedName][]*types.NamespacedName
	stopChans                      map[types.NamespacedName][]chan bool
}

const (
	OperatorLabel = "beamlit-operator"
)

func newKubernetesConfigurer(ctx context.Context, kubeClient kubernetes.Interface) (Configurer, error) {
	return &kubernetesConfigurer{
		kubeClient:                     kubeClient,
		beamlitServicesByModelService:  make(map[types.NamespacedName]*types.NamespacedName),
		stopChans:                      make(map[types.NamespacedName][]chan bool),
		initialEndpointPerLocalService: make(map[types.NamespacedName][]*types.NamespacedName),
		gatewayServiceRef:              nil,
	}, nil
}

func (s *kubernetesConfigurer) Start(ctx context.Context, gatewayService *modelv1alpha1.ServiceReference) error {
	s.gatewayServiceRef = gatewayService
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

	stopCh := make(chan bool)
	s.stopChans[types.NamespacedName{Namespace: serviceRef.Namespace, Name: serviceRef.Name}] = append(s.stopChans[types.NamespacedName{Namespace: serviceRef.Namespace, Name: serviceRef.Name}], stopCh)
	go func() {
		if err := s.watchService(ctx, serviceRef, stopCh); err != nil {
			log.Log.Error(err, "error watching service")
		}
	}()

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
		FieldManager: OperatorLabel,
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

	addPort := true
	for _, port := range gatewayService.Spec.Ports {
		if port.Port == serviceRef.TargetPort && port.Protocol == portToAdd.Protocol {
			addPort = false
			break
		}
	}
	if addPort {
		gatewayService.Spec.Ports = append(gatewayService.Spec.Ports, corev1.ServicePort{
			Name:       fmt.Sprintf("%d-beamlit", serviceRef.TargetPort),
			Port:       serviceRef.TargetPort,
			Protocol:   portToAdd.Protocol,
			TargetPort: intstr.FromInt(int(s.gatewayServiceRef.TargetPort)),
		})
	}

	var externalIPs []string
	for _, clusterIP := range serviceToConfigure.Spec.ClusterIPs {
		if !slices.Contains(gatewayService.Spec.ExternalIPs, clusterIP) {
			externalIPs = append(externalIPs, clusterIP)
		}
	}
	gatewayService.Spec.ExternalIPs = externalIPs
	_, err = s.kubeClient.CoreV1().Services(s.gatewayServiceRef.Namespace).Update(ctx, gatewayService, metav1.UpdateOptions{
		FieldManager: OperatorLabel,
	})
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
	if _, ok := s.initialEndpointPerLocalService[types.NamespacedName{Namespace: serviceRef.Namespace, Name: serviceRef.Name}]; !ok {
		s.initialEndpointPerLocalService[types.NamespacedName{Namespace: serviceRef.Namespace, Name: serviceRef.Name}] = make([]*types.NamespacedName, 0)
	}
	for _, endpoint := range userServiceEndpoints.Items {
		if endpoint.Labels["endpointslice.kubernetes.io/managed-by"] == OperatorLabel {
			continue
		}
		s.initialEndpointPerLocalService[types.NamespacedName{Namespace: serviceRef.Namespace, Name: serviceRef.Name}] = append(s.initialEndpointPerLocalService[types.NamespacedName{Namespace: serviceRef.Namespace, Name: serviceRef.Name}], &types.NamespacedName{Namespace: endpoint.Namespace, Name: endpoint.Name})
		endpoint.Labels["endpointslice.kubernetes.io/managed-by"] = OperatorLabel
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
			"beamlit.com/to-update":                  "true",
			"kubernetes.io/service-name":             serviceRef.Name,
			"endpointslice.kubernetes.io/managed-by": "beamlit-operator",
		})

	endpoints := make([]*discoveryv1apply.EndpointApplyConfiguration, 0)
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

	ports := make([]*discoveryv1apply.EndpointPortApplyConfiguration, 0)
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

	stopCh := make(chan bool)
	s.stopChans[types.NamespacedName{Namespace: serviceRef.Namespace, Name: serviceRef.Name}] = append(s.stopChans[types.NamespacedName{Namespace: serviceRef.Namespace, Name: serviceRef.Name}], stopCh)
	go func() {
		if err := s.mirrorEndpointSlices(ctx, serviceRef, targetPort, stopCh); err != nil {
			log.FromContext(ctx).Error(err, "error while mirroring endpoints slices")
		}
	}()
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
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second) // TODO: change this
	defer cancel()
	if value, ok := s.beamlitServicesByModelService[types.NamespacedName{Namespace: service.Namespace, Name: service.Name}]; !ok || value == nil {
		return nil
	}
	logger := log.FromContext(ctx)
	logger.V(1).Info("Unconfiguring service", "Name", service.Name)
	err := s.stopWatchers(ctx, service)
	if err != nil {
		return err
	}
	logger.V(1).Info("Adding kubernetes managed endpoints slice", "Name", service.Name)
	err = s.addKubernetesManagedEndpointsSlice(ctx, service)
	if err != nil {
		return err
	}
	logger.V(1).Info("Deleting beamlit endpoints slice", "Name", service.Name)
	err = s.deleteBeamlitEndpointsSlice(ctx, service)
	if err != nil {
		return err
	}
	logger.V(1).Info("Deleting beamlit service", "Name", service.Name)
	err = s.deleteBeamlitService(ctx, service)
	if err != nil {
		return err
	}
	logger.V(1).Info("Deleting external IPs from gateway service", "Name", service.Name)
	err = s.deleteExternalIPsFromGatewayService(ctx, service)
	if err != nil {
		return err
	}
	err = s.watchEndpointsSliceToBeUpdated(ctx, service)
	if err != nil {
		return err
	}
	logger.V(1).Info("Successfully unregistered service", "Name", service.Name)
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

func (s *kubernetesConfigurer) stopWatchers(_ context.Context, serviceRef *modelv1alpha1.ServiceReference) error {
	stopCh, ok := s.stopChans[types.NamespacedName{Namespace: serviceRef.Namespace, Name: serviceRef.Name}]
	if !ok {
		return fmt.Errorf("stop channel not found for service %s", serviceRef.Name)
	}
	for _, ch := range stopCh {
		select {
		case _, ok := <-ch:
			if ok {
				close(ch)
			}
		default:
			return nil
		}
	}
	return nil
}

func (s *kubernetesConfigurer) addKubernetesManagedEndpointsSlice(ctx context.Context, service *modelv1alpha1.ServiceReference) error {
	initialEndpoints := s.initialEndpointPerLocalService[types.NamespacedName{Namespace: service.Namespace, Name: service.Name}]
	for _, endpoint := range initialEndpoints {
		endpointSlice, err := s.kubeClient.DiscoveryV1().EndpointSlices(endpoint.Namespace).Get(ctx, endpoint.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		endpointSlice.Labels["endpointslice.kubernetes.io/managed-by"] = "endpointslice-controller.k8s.io"
		_, err = s.kubeClient.DiscoveryV1().EndpointSlices(endpoint.Namespace).Update(ctx, endpointSlice, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
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
			err = s.kubeClient.DiscoveryV1().EndpointSlices(service.Namespace).Delete(ctx, endpointSlice.Name, metav1.DeleteOptions{})
			if err != nil {
				return err
			}
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

func (s *kubernetesConfigurer) watchEndpointsSliceToBeUpdated(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference) error {
	retry := 0
	maxRetries := 5
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			initialEndpoints := s.initialEndpointPerLocalService[types.NamespacedName{Namespace: serviceRef.Namespace, Name: serviceRef.Name}]
			if len(initialEndpoints) == 0 {
				return nil
			}
			for _, endpoint := range initialEndpoints {
				endpointSlice, err := s.kubeClient.DiscoveryV1().EndpointSlices(endpoint.Namespace).Get(ctx, endpoint.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if endpointSlice.Labels["endpointslice.kubernetes.io/managed-by"] == "endpointslice-controller.k8s.io" {
					// Check if there are endpoints in the endpoint slice
					if len(endpointSlice.Endpoints) > 0 {
						delete(s.initialEndpointPerLocalService, types.NamespacedName{Namespace: serviceRef.Namespace, Name: serviceRef.Name})
						return nil
					}
				}
			}
			retry++
			if retry >= maxRetries {
				delete(s.initialEndpointPerLocalService, types.NamespacedName{Namespace: serviceRef.Namespace, Name: serviceRef.Name})
				return nil
			}
			time.Sleep(time.Duration(retry) * 100 * time.Millisecond)
		}
	}
}
