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
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func retrieveHPA(ctx context.Context, kubernetesClient client.Client, hpaName, namespace string) (autoscalingv2.HorizontalPodAutoscalerSpec, error) {
	hpa := autoscalingv2.HorizontalPodAutoscaler{}
	if err := kubernetesClient.Get(ctx, types.NamespacedName{Name: hpaName, Namespace: namespace}, &hpa); err != nil {
		return autoscalingv2.HorizontalPodAutoscalerSpec{}, err
	}
	return hpa.Spec, nil
}

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
