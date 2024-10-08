package controller

import (
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func toIntPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}

func createKind(kind string) *gatewayv1.Kind {
	k := gatewayv1.Kind(kind)
	return &k
}

func createGroup(group string) *gatewayv1.Group {
	g := gatewayv1.Group(group)
	return &g
}

func createNamespace(namespace string) *gatewayv1.Namespace {
	n := gatewayv1.Namespace(namespace)
	return &n
}

func createServiceKind() *gatewayv1.Kind {
	k := gatewayv1.Kind("Service")
	return &k
}

func createWeight(weight int) *int32 {
	w := int32(weight)
	return &w
}

func createPortNumber(port int) *gatewayv1.PortNumber {
	p := gatewayv1.PortNumber(port)
	return &p
}

func createPathMatchType(pathMatchType string) *gatewayv1.PathMatchType {
	pathMatchTypeEnum := gatewayv1.PathMatchType(pathMatchType)
	return &pathMatchTypeEnum
}

func createPathValue(path string) *string {
	return &path
}
