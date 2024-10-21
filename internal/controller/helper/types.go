/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*/

package helper

import (
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func toPtr[T any](v T) *T {
	return &v
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
