package metrics_watcher

import (
	"context"
	"errors"
	"fmt"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	scaleclient "k8s.io/client-go/scale"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func IsEqual(a autoscalingv2.MetricSpec, b autoscalingv2.MetricSpec) bool {
	if a.Type != b.Type {
		return false
	}
	switch a.Type {
	case autoscalingv2.ContainerResourceMetricSourceType:
		return equalContainerResourceMetricSource(*a.ContainerResource, *b.ContainerResource)
	case autoscalingv2.ExternalMetricSourceType:
		return equalExternalMetricSource(*a.External, *b.External)
	case autoscalingv2.ObjectMetricSourceType:
		return equalObjectMetricSource(*a.Object, *b.Object)
	case autoscalingv2.PodsMetricSourceType:
		return equalPodsMetricSource(*a.Pods, *b.Pods)
	case autoscalingv2.ResourceMetricSourceType:
		return equalResourceMetricSource(*a.Resource, *b.Resource)
	default:
		return false
	}
}

func equalContainerResourceMetricSource(a autoscalingv2.ContainerResourceMetricSource, b autoscalingv2.ContainerResourceMetricSource) bool {
	return a.Name == b.Name && a.Container == b.Container && equalMetricTarget(a.Target, b.Target)
}

func equalExternalMetricSource(a autoscalingv2.ExternalMetricSource, b autoscalingv2.ExternalMetricSource) bool {
	return equalMetricIdentifier(a.Metric, b.Metric) && equalMetricTarget(a.Target, b.Target)
}

func equalObjectMetricSource(a autoscalingv2.ObjectMetricSource, b autoscalingv2.ObjectMetricSource) bool {
	if a.DescribedObject.APIVersion != b.DescribedObject.APIVersion ||
		a.DescribedObject.Kind != b.DescribedObject.Kind ||
		a.DescribedObject.Name != b.DescribedObject.Name {
		return false
	}
	return equalMetricIdentifier(a.Metric, b.Metric) && equalMetricTarget(a.Target, b.Target)
}

func equalPodsMetricSource(a autoscalingv2.PodsMetricSource, b autoscalingv2.PodsMetricSource) bool {
	return equalMetricIdentifier(a.Metric, b.Metric) && equalMetricTarget(a.Target, b.Target)
}

func equalResourceMetricSource(a autoscalingv2.ResourceMetricSource, b autoscalingv2.ResourceMetricSource) bool {
	return a.Name == b.Name && equalMetricTarget(a.Target, b.Target)
}

func equalMetricIdentifier(a autoscalingv2.MetricIdentifier, b autoscalingv2.MetricIdentifier) bool {
	if a.Name != b.Name {
		return false
	}

	if a.Selector == nil && b.Selector == nil {
		return true
	}

	if a.Selector == nil || b.Selector == nil {
		return false
	}

	return a.Selector.String() == b.Selector.String()
}

func equalMetricTarget(a autoscalingv2.MetricTarget, b autoscalingv2.MetricTarget) bool {
	if a.Type != b.Type {
		return false
	}

	switch a.Type {
	case autoscalingv2.UtilizationMetricType:
		return a.AverageUtilization != nil && b.AverageUtilization != nil && *a.AverageUtilization == *b.AverageUtilization
	case autoscalingv2.ValueMetricType:
		return a.Value != nil && b.Value != nil && a.Value.Equal(*b.Value)
	case autoscalingv2.AverageValueMetricType:
		return a.AverageValue != nil && b.AverageValue != nil && a.AverageValue.Equal(*b.AverageValue)
	}

	return false
}

func getReplicasInfo(ctx context.Context, kubernetesClient client.Client, scaleClient scaleclient.ScalesGetter, watchTarget *v1.ObjectReference) (int32, labels.Selector, error) {
	log.FromContext(ctx).Info("getReplicasInfo", "watchTarget", watchTarget)
	targetGroupVersion, err := schema.ParseGroupVersion(watchTarget.APIVersion)
	if err != nil {
		log.FromContext(ctx).Error(err, "Error parsing group version")
		return 0, nil, err
	}

	targetGK := schema.GroupKind{
		Group: targetGroupVersion.Group,
		Kind:  watchTarget.Kind,
	}

	mapper, err := kubernetesClient.RESTMapper().RESTMappings(targetGK)
	if err != nil {
		log.FromContext(ctx).Error(err, "Error getting REST mappings")
		return 0, nil, err
	}

	var firstErr error
	for i, mapper := range mapper {
		scale, err := scaleClient.Scales(watchTarget.Namespace).Get(ctx, mapper.Resource.GroupResource(), watchTarget.Name, metav1.GetOptions{})
		if err == nil {
			selector, err := parseHPASelector(scale.Status.Selector)
			if err != nil {
				log.FromContext(ctx).Error(err, "Error parsing selector")
				return 0, nil, err
			}
			return scale.Status.Replicas, selector, nil
		}
		if i == 0 {
			log.FromContext(ctx).Error(err, "Error getting scale")
			firstErr = err
		}
	}

	if firstErr != nil {
		log.FromContext(ctx).Error(firstErr, "Error getting replicas info")
		return 0, nil, firstErr
	}

	log.FromContext(ctx).Error(fmt.Errorf("failed to get replicas info for %s", watchTarget.Name), "Error getting replicas info")
	return 0, nil, fmt.Errorf("failed to get replicas info for %s", watchTarget.Name)
}

func parseHPASelector(selector string) (labels.Selector, error) {
	if selector == "" {
		return nil, errors.New("selector is required")
	}

	parsedSelector, err := labels.Parse(selector)
	if err != nil {
		return nil, err
	}
	return parsedSelector, nil
}

func findRequestedResource(ctx context.Context, kubeClient client.Client, namespace string, labelSelector labels.Selector, resourceName v1.ResourceName) (int64, error) {
	// get pod requests
	podList := &v1.PodList{}
	err := kubeClient.List(ctx, podList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: labelSelector})
	if err != nil {
		return 0, err
	}
	var requestedResource int64
	for _, pod := range podList.Items {
		for _, container := range pod.Spec.Containers {
			switch resourceName {
			case v1.ResourceCPU:
				requestedResource += container.Resources.Requests.Cpu().MilliValue()
			case v1.ResourceMemory:
				requestedResource += container.Resources.Requests.Memory().MilliValue()
			case v1.ResourceEphemeralStorage:
				requestedResource += container.Resources.Requests.StorageEphemeral().MilliValue()
			case v1.ResourceStorage:
				requestedResource += container.Resources.Requests.Storage().MilliValue()
			}
		}
	}

	return requestedResource, nil
}
