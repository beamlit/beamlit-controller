package k8s_resource_utils

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetReplicas retrieves the number of replicas for a given Kubernetes resource.
// It supports Deployment and StatefulSet.
// TODO: Add support for other kinds of resources
func GetReplicas(ctx context.Context, client client.Client, cr *v1.ObjectReference) (int32, error) {
	switch cr.Kind {
	case "Deployment":
		return getReplicaFromDeployment(ctx, client, cr.Namespace, cr.Name)
	case "StatefulSet":
		return getReplicaFromStatefulSet(ctx, client, cr.Namespace, cr.Name)
	default:
		return 0, fmt.Errorf("unsupported kind: %s", cr.Kind)
	}
}

func getReplicaFromDeployment(ctx context.Context, client client.Client, namespace, name string) (int32, error) {
	deployment := &appsv1.Deployment{}
	if err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, deployment); err != nil {
		return 0, err
	}
	return *deployment.Spec.Replicas, nil
}

func getReplicaFromStatefulSet(ctx context.Context, client client.Client, namespace, name string) (int32, error) {
	statefulSet := &appsv1.StatefulSet{}
	if err := client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, statefulSet); err != nil {
		return 0, err
	}
	return *statefulSet.Spec.Replicas, nil
}
