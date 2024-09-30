package offloading

import (
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func createServiceKind() *gatewayv1.Kind {
	kind := gatewayv1.Kind("Service")
	return &kind
}

func createNamespace(namespace string) *gatewayv1.Namespace {
	ns := gatewayv1.Namespace(namespace)
	return &ns
}

func createWeight(weight int32) *int32 {
	return &weight
}

func createPathMatchType(matchType string) *gatewayv1.PathMatchType {
	pathMatchType := gatewayv1.PathMatchType(matchType)
	return &pathMatchType
}

func createPathValue(value string) *string {
	return &value
}

func createPortNumber(port int32) *gatewayv1.PortNumber {
	portNumber := gatewayv1.PortNumber(port)
	return &portNumber
}
