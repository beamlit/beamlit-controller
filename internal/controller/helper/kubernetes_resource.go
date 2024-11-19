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

package helper

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// retrievePodTemplate retrieves a pod template from a Kubernetes resource
// It fails if the resource is not a Deployment, StatefulSet, DaemonSet, ReplicaSet
func retrievePodTemplate(ctx context.Context, kubernetesClient client.Client, kind, name, namespace string) (corev1.PodTemplateSpec, error) {
	var obj client.Object
	var podTemplate *corev1.PodTemplateSpec

	switch kind {
	case "Deployment":
		d := &appsv1.Deployment{}
		obj, podTemplate = d, &d.Spec.Template
	case "StatefulSet":
		s := &appsv1.StatefulSet{}
		obj, podTemplate = s, &s.Spec.Template
	case "DaemonSet":
		d := &appsv1.DaemonSet{}
		obj, podTemplate = d, &d.Spec.Template
	case "ReplicaSet":
		r := &appsv1.ReplicaSet{}
		obj, podTemplate = r, &r.Spec.Template
	default:
		return corev1.PodTemplateSpec{}, fmt.Errorf("unexpected object type: %s", kind)
	}

	if err := kubernetesClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, obj); err != nil {
		return corev1.PodTemplateSpec{}, err
	}

	return *podTemplate, nil
}

func RetrievePodPort(ctx context.Context, kubernetesClient client.Client, serviceReference *corev1.ObjectReference, targetPort int) (int, error) {
	service := corev1.Service{}
	if err := kubernetesClient.Get(ctx, types.NamespacedName{Name: serviceReference.Name, Namespace: serviceReference.Namespace}, &service); err != nil {
		return 0, err
	}
	for _, port := range service.Spec.Ports {
		if int(port.Port) == targetPort {
			return int(port.TargetPort.IntVal), nil
		}
	}
	return 0, fmt.Errorf("port %d not found", targetPort)
}
