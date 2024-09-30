package k8s_resource_utils

import (
	"context"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HPA Retrieves a HorizontalPodAutoscaler by name and namespace
func HPA(ctx context.Context, kubernetesClient client.Client, hpaName, namespace string) (autoscalingv2.HorizontalPodAutoscalerSpec, error) {
	hpa := autoscalingv2.HorizontalPodAutoscaler{}
	if err := kubernetesClient.Get(ctx, types.NamespacedName{Name: hpaName, Namespace: namespace}, &hpa); err != nil {
		return autoscalingv2.HorizontalPodAutoscalerSpec{}, err
	}
	return hpa.Spec, nil
}
