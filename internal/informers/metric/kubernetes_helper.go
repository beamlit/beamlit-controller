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

package metric

import (
	"context"
	"errors"
	"fmt"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	scaleclient "k8s.io/client-go/scale"
)

// IsMetricSpecEqual checks if two metric specs are deeply equal (no pointer comparison).
// It returns true if the two metric specs are equal, false otherwise.
func IsMetricSpecEqual(a autoscalingv2.MetricSpec, b autoscalingv2.MetricSpec) bool {
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

// getReplicasInfo gets the number of replicas and the selector of a given Kubernetes resource.
func getReplicasInfo(ctx context.Context, restMapper meta.RESTMapper, scaleClient scaleclient.ScalesGetter, watchTarget *v1.ObjectReference) (int32, labels.Selector, error) {
	targetGroupVersion, err := schema.ParseGroupVersion(watchTarget.APIVersion)
	if err != nil {
		return 0, nil, err
	}

	targetGK := schema.GroupKind{
		Group: targetGroupVersion.Group,
		Kind:  watchTarget.Kind,
	}

	mapper, err := restMapper.RESTMappings(targetGK)
	if err != nil {
		return 0, nil, err
	}

	var firstErr error
	for i, mapper := range mapper {
		scale, err := scaleClient.Scales(watchTarget.Namespace).Get(ctx, mapper.Resource.GroupResource(), watchTarget.Name, metav1.GetOptions{})
		if err == nil {
			selector, err := parseHPASelector(scale.Status.Selector)
			if err != nil {
				return 0, nil, err
			}
			return scale.Status.Replicas, selector, nil
		}
		if i == 0 {
			firstErr = err
		}
	}

	if firstErr != nil {
		return 0, nil, firstErr
	}

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

func findRequestedResource(ctx context.Context, kubeClient kubernetes.Interface, namespace string, labelSelector labels.Selector, resourceName v1.ResourceName) (int64, error) {
	// get pod requests
	podList, err := kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector.String()})
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

func isMetricReached(replicas int32, metric autoscalingv2.MetricTarget, metrics []int64) (bool, error) {
	switch metric.Type {
	case autoscalingv2.UtilizationMetricType:
		if metric.AverageUtilization == nil {
			return false, fmt.Errorf("averageUtilization is nil")
		}
		averageUtilization := int64(0)
		for _, metric := range metrics {
			averageUtilization += metric
		}
		averageUtilization /= int64(replicas)
		return averageUtilization >= int64(*metric.AverageUtilization), nil
	case autoscalingv2.ValueMetricType:
		metricValue := int64(0)
		for _, m := range metrics {
			if metric.Value != nil {
				metricValue += m
			}
		}
		return metric.Value.CmpInt64(metricValue) >= 0, nil
	case autoscalingv2.AverageValueMetricType:
		averageValue := int64(0)
		for _, metric := range metrics {
			averageValue += metric
		}
		averageValue /= int64(replicas)
		return metric.AverageValue.CmpInt64(averageValue) >= 0, nil
	default:
		return false, fmt.Errorf("unsupported metric type: %s", metric.Type)
	}
}
