package service

//TODO: support app protocol

import (
	"context"
	"fmt"

	modelv1alpha1 "github.com/beamlit/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	v1 "k8s.io/client-go/applyconfigurations/core/v1"
	discoveryv1apply "k8s.io/client-go/applyconfigurations/discovery/v1"
)

func (s *serviceController) Register(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference) error {
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

func (s *serviceController) createBeamlitModelService(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference) (*corev1.Service, error) {
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
func (s *serviceController) addPortToGatewayService(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference) error {
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
func (s *serviceController) takeOverEndpointsSlices(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference) error {
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
func (s *serviceController) createMirroredEndpointsSlice(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference, targetPort int32) error {
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

func (s *serviceController) cleanUnusedEndpointSlices(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference) error {
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
