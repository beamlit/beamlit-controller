package metrics_watcher

import (
	"context"
	"errors"
	"fmt"

	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/scale"
	k8smetrics "k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
)

type metricWatcher struct {
	scaleClient   scale.ScalesGetter
	kubeClient    client.Client
	metricsClient k8smetrics.MetricsClient
	condition     metricConditionStatus
	watchTarget   v1.ObjectReference
	metric        []autoscalingv2.MetricSpec
	callback      func(reached bool) error
	cancel        context.CancelFunc
}

func (mw *metricWatcher) CheckMetric(ctx context.Context) (bool, error) {
	var errs []error
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	mw.cancel = cancel
	for _, metric := range mw.metric {
		switch metric.Type {
		case autoscalingv2.ObjectMetricSourceType:
			if metric.Object == nil {
				errs = append(errs, fmt.Errorf("metric %s is not valid", metric.Object.Metric.Name))
				continue
			}
			metricSelector, err := metav1.LabelSelectorAsSelector(metric.Object.Metric.Selector)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			usage, _, err := mw.metricsClient.GetObjectMetric(metric.Object.Metric.Name, mw.watchTarget.Namespace, &autoscalingv2.CrossVersionObjectReference{
				APIVersion: mw.watchTarget.APIVersion,
				Kind:       mw.watchTarget.Kind,
				Name:       mw.watchTarget.Name,
			}, metricSelector)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			replicas, _, err := getReplicasInfo(ctx, mw.kubeClient, mw.scaleClient, &mw.watchTarget)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			reached, err := isMetricReached(
				replicas,
				metric.Object.Target,
				[]int64{usage},
			)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if reached {
				mw.condition.update(metric, reached)
			}
		case autoscalingv2.PodsMetricSourceType:
			if metric.Pods == nil {
				errs = append(errs, fmt.Errorf("metric %s is not valid", metric.Pods.Metric.Name))
				continue
			}
			metricSelector, err := metav1.LabelSelectorAsSelector(metric.Pods.Metric.Selector)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			replicas, labelSelector, err := getReplicasInfo(ctx, mw.kubeClient, mw.scaleClient, &mw.watchTarget)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			usage, _, err := mw.metricsClient.GetRawMetric(metric.Pods.Metric.Name, mw.watchTarget.Namespace, labelSelector, metricSelector)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			metrics := []int64{}
			for _, podMetric := range usage {
				metrics = append(metrics, int64(podMetric.Value))
			}
			reached, err := isMetricReached(
				replicas,
				metric.Pods.Target,
				metrics,
			)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if reached {
				mw.condition.update(metric, reached)
			}
		case autoscalingv2.ResourceMetricSourceType: // only case with averageUtilization
			log.FromContext(ctx).Info("ResourceMetricSourceType", "metric", metric)
			if metric.Resource == nil {
				errs = append(errs, fmt.Errorf("metric %s is not valid", metric.Resource.Name))
				continue
			}
			replicas, labelSelector, err := getReplicasInfo(ctx, mw.kubeClient, mw.scaleClient, &mw.watchTarget)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			usage, _, err := mw.metricsClient.GetResourceMetric(ctx, v1.ResourceName(metric.Resource.Name), mw.watchTarget.Namespace, labelSelector, "")
			if err != nil {
				errs = append(errs, err)
				continue
			}
			reached := false
			if metric.Resource.Target.AverageUtilization != nil {
				requestedResource, err := findRequestedResource(ctx, mw.kubeClient, mw.watchTarget.Namespace, labelSelector, v1.ResourceName(metric.Resource.Name))
				if err != nil {
					errs = append(errs, err)
					continue
				}
				// calculate the average utilization
				averageUtilization := int64(0)
				for _, metric := range usage {
					averageUtilization += metric.Value
				}
				averageUtilization *= 100 // conver to percentage first, then divide (to avoid loss of precision)
				averageUtilization /= requestedResource
				averageUtilization /= int64(replicas)
				reached = averageUtilization >= int64(*metric.Resource.Target.AverageUtilization)
			} else {
				metrics := []int64{}
				for _, podMetric := range usage {
					metrics = append(metrics, int64(podMetric.Value))
				}
				reached, err = isMetricReached(
					replicas,
					metric.Resource.Target,
					metrics,
				)
				if err != nil {
					errs = append(errs, err)
					continue
				}
			}
			if reached {
				mw.condition.update(metric, reached)
			}
		case autoscalingv2.ContainerResourceMetricSourceType:
			if metric.ContainerResource == nil {
				errs = append(errs, fmt.Errorf("metric %s is not valid", metric.ContainerResource.Name))
				continue
			}
			replicas, labelSelector, err := getReplicasInfo(ctx, mw.kubeClient, mw.scaleClient, &mw.watchTarget)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			usage, _, err := mw.metricsClient.GetResourceMetric(ctx, v1.ResourceName(metric.ContainerResource.Container), mw.watchTarget.Namespace, labelSelector, metric.ContainerResource.Container)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			metrics := []int64{}
			for _, podMetric := range usage {
				metrics = append(metrics, int64(podMetric.Value))
			}
			reached, err := isMetricReached(
				replicas,
				metric.ContainerResource.Target,
				metrics,
			)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if reached {
				mw.condition.update(metric, reached)
			}
		case autoscalingv2.ExternalMetricSourceType:
			if metric.External == nil {
				errs = append(errs, fmt.Errorf("metric %s is not valid", metric.External.Metric.Name))
				continue
			}
			metricSelector, err := metav1.LabelSelectorAsSelector(metric.External.Metric.Selector)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			usage, _, err := mw.metricsClient.GetExternalMetric(metric.External.Metric.Name, mw.watchTarget.Namespace, metricSelector)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			replicas, _, err := getReplicasInfo(ctx, mw.kubeClient, mw.scaleClient, &mw.watchTarget)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			reached, err := isMetricReached(
				replicas,
				metric.External.Target,
				usage,
			)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if reached {
				mw.condition.update(metric, reached)
			}
		default:
			panic("unsupported metric type")
		}
	}
	if len(errs) > 0 {
		return false, fmt.Errorf("error checking metrics: %w", errors.Join(errs...))
	}
	return mw.condition.isReached(), nil
}

func (mw *metricWatcher) TriggerAction(reached bool) error {
	return mw.callback(reached)
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
