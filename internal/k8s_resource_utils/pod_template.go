package k8s_resource_utils

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PodTemplate retrieves a PodTemplate by name and namespace. It supports Deployment and StatefulSet.
// TODO: Add support for other kinds of resources
func PodTemplate(ctx context.Context, kubernetesClient client.Client, cr *v1.ObjectReference) (corev1.PodTemplateSpec, error) {
	switch cr.Kind {
	case "Deployment":
		return retrieveDeploymentPodTemplate(ctx, kubernetesClient, cr.Name, cr.Namespace)
	case "StatefulSet":
		return retrieveStatefulSetPodTemplate(ctx, kubernetesClient, cr.Name, cr.Namespace)
	default:
		return corev1.PodTemplateSpec{}, fmt.Errorf("unsupported kind: %s", cr.Kind)
	}
}

func retrieveDeploymentPodTemplate(ctx context.Context, kubernetesClient client.Client, name, namespace string) (corev1.PodTemplateSpec, error) {
	deployment := &appsv1.Deployment{}
	if err := kubernetesClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, deployment); err != nil {
		return corev1.PodTemplateSpec{}, err
	}
	return deployment.Spec.Template, nil
}

func retrieveStatefulSetPodTemplate(ctx context.Context, kubernetesClient client.Client, name, namespace string) (corev1.PodTemplateSpec, error) {
	statefulset := &appsv1.StatefulSet{}
	if err := kubernetesClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, statefulset); err != nil {
		return corev1.PodTemplateSpec{}, err
	}
	return statefulset.Spec.Template, nil
}
