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
	"fmt"
	"time"

	"github.com/beamlit/operator/internal/informers"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/scale"
	scaleclient "k8s.io/client-go/scale"
	"k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
	resourceclient "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	"k8s.io/metrics/pkg/client/custom_metrics"
	"k8s.io/metrics/pkg/client/external_metrics"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// clientset is the set of clients needed by the k8sMetricInformer.
type clientset struct {
	scaleClient      scaleclient.ScalesGetter
	kubernetesClient kubernetes.Interface
	metricsClient    metrics.MetricsClient
}

// k8sMetricInformer is an interface for a k8s metric informer.
// It is rely on metrics-server to get the metrics of the k8s resources.
type k8sMetricInformer struct {
	clientset
	metricChan chan MetricStatus
	errChan    chan informers.ErrWrapper
	watchers   map[string]*k8sMetricWatcher // model: watcher
}

func newK8sMetricInformer(ctx context.Context, restConfig *rest.Config) (MetricInformer, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	resourceclient, err := resourceclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	customMetricsClient := custom_metrics.NewForConfig(
		restConfig,
		restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient)),
		custom_metrics.NewAvailableAPIsGetter(discoveryClient))

	externalMetricsClient, err := external_metrics.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	metricsClient := metrics.NewRESTMetricsClient(
		resourceclient,
		customMetricsClient,
		externalMetricsClient,
	)

	kubernetesClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	scaleKindResolver := scale.NewDiscoveryScaleKindResolver(discoveryClient)
	scaleClient, err := scale.NewForConfig(restConfig, restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient)), dynamic.LegacyAPIPathResolverFunc, scaleKindResolver)
	if err != nil {
		return nil, err
	}

	return &k8sMetricInformer{
		clientset: clientset{
			scaleClient:      scaleClient,
			kubernetesClient: kubernetesClient,
			metricsClient:    metricsClient,
		},
		metricChan: make(chan MetricStatus),
		watchers:   make(map[string]*k8sMetricWatcher),
		errChan:    make(chan informers.ErrWrapper),
	}, nil
}

func (k *k8sMetricInformer) Start(ctx context.Context) <-chan MetricStatus {
	logger := log.FromContext(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-k.errChan:
				logger.Error(err.Err, "error in metric informer", "modelName", err.ModelName)
			}
		}
	}()
	return k.metricChan
}

func (k *k8sMetricInformer) Stop() {
	for _, watcher := range k.watchers {
		watcher.cancel()
	}
	close(k.metricChan)
	close(k.errChan)
}

func (k *k8sMetricInformer) Register(ctx context.Context, model string, metrics []autoscalingv2.MetricSpec, resource v1.ObjectReference, scrapeInterval time.Duration, window time.Duration) {
	if _, ok := k.watchers[model]; ok {
		k.Unregister(ctx, model)
	}
	watcher := &k8sMetricWatcher{
		clientset: clientset{
			scaleClient:      k.scaleClient,
			kubernetesClient: k.kubernetesClient,
			metricsClient:    k.metricsClient,
		},
		model:          model,
		metrics:        metrics,
		scrapeInterval: scrapeInterval,
		watchTarget:    resource,
		metricChan:     k.metricChan,
		errChan:        k.errChan,
		latestStatus:   MetricStatus{},
		cancel:         nil,
		condition: metricConditionStatus{
			currentMetricReachedMetrics: make([]autoscalingv2.MetricSpec, 0),
			window:                      window,
			sinceTime:                   time.Time{},
			reached:                     false,
		},
	}
	k.watchers[model] = watcher
	go watcher.start(ctx)
}

func (k *k8sMetricInformer) Unregister(ctx context.Context, model string) {
	if value, ok := k.watchers[model]; ok {
		value.cancel()
		delete(k.watchers, model)
	}
}

type k8sMetricWatcher struct {
	clientset
	model          string
	scrapeInterval time.Duration
	latestStatus   MetricStatus
	cancel         context.CancelFunc
	condition      metricConditionStatus
	metrics        []autoscalingv2.MetricSpec
	watchTarget    v1.ObjectReference
	metricChan     chan<- MetricStatus
	errChan        chan<- informers.ErrWrapper
}

func (mw *k8sMetricWatcher) start(ctx context.Context) {
	logger := log.FromContext(ctx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	mw.cancel = cancel
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(mw.scrapeInterval):
			reached, err := mw.CheckMetric(ctx)
			if err != nil {
				logger.Error(err, "error checking metric", "model", mw.model)
				mw.errChan <- informers.ErrWrapper{
					ModelName: mw.model,
					Err:       err,
				}
				continue
			}
			mw.latestStatus.ModelName = mw.model
			mw.latestStatus.Reached = reached
			mw.metricChan <- mw.latestStatus
		}
	}
}

func (mw *k8sMetricWatcher) CheckMetric(ctx context.Context) (bool, error) {
	errs := []error{}
	for _, metric := range mw.metrics {
		replicas, labelSelector, err := getReplicasInfo(ctx, restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(mw.kubernetesClient.Discovery())), mw.scaleClient, &mw.watchTarget)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if replicas == 0 {
			mw.condition.update(ctx, metric, false)
			continue
		}
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
			reached, err := isMetricReached(
				replicas,
				metric.Object.Target,
				[]int64{usage},
			)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			mw.condition.update(ctx, metric, reached)
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
			mw.condition.update(ctx, metric, reached)
		case autoscalingv2.ResourceMetricSourceType: // only case with averageUtilization
			if metric.Resource == nil {
				errs = append(errs, fmt.Errorf("metric %s is not valid", metric.Resource.Name))
				continue
			}
			usage, _, err := mw.metricsClient.GetResourceMetric(ctx, v1.ResourceName(metric.Resource.Name), mw.watchTarget.Namespace, labelSelector, "")
			if err != nil {
				errs = append(errs, err)
				continue
			}
			reached := false
			if metric.Resource.Target.AverageUtilization != nil {
				requestedResource, err := findRequestedResource(ctx, mw.kubernetesClient, mw.watchTarget.Namespace, labelSelector, v1.ResourceName(metric.Resource.Name))
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
					if podMetric.Timestamp.Add(podMetric.Window).After(time.Now().Add(-mw.scrapeInterval)) {
						metrics = append(metrics, int64(podMetric.Value))
					}
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
			mw.condition.update(ctx, metric, reached)
		case autoscalingv2.ContainerResourceMetricSourceType:
			if metric.ContainerResource == nil {
				errs = append(errs, fmt.Errorf("metric %s is not valid", metric.ContainerResource.Name))
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
			mw.condition.update(ctx, metric, reached)
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
			metrics := []int64{}
			for _, metric := range usage {
				metrics = append(metrics, int64(metric))
			}
			reached, err := isMetricReached(
				replicas,
				metric.External.Target,
				metrics,
			)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			mw.condition.update(ctx, metric, reached)
		default:
			panic("unsupported metric type")
		}
	}
	if len(errs) > 0 {
		for _, err := range errs {
			if err.Error() == "no metrics returned from resource metrics API" { // TODO: find a better way to handle this
				continue
			}
			return false, err
		}
	}
	return mw.condition.isReached(), nil
}

type metricConditionStatus struct {
	currentMetricReachedMetrics []autoscalingv2.MetricSpec
	reached                     bool
	window                      time.Duration // The window for which the condition must be reached to trigger an event
	sinceTime                   time.Time     // The time when the condition was first reached
}

// update updates the metric condition status
func (mcs *metricConditionStatus) update(ctx context.Context, currentMetric autoscalingv2.MetricSpec, reached bool) {
	defer func() {
		if len(mcs.currentMetricReachedMetrics) == 0 {
			mcs.reached = false
			mcs.sinceTime = time.Time{}
		}
	}()

	for i, metric := range mcs.currentMetricReachedMetrics {
		isEqual := IsMetricSpecEqual(metric, currentMetric)
		if isEqual && !reached {
			// Remove the metric from the list if the condition is no more reached
			if len(mcs.currentMetricReachedMetrics) == 1 {
				mcs.currentMetricReachedMetrics = []autoscalingv2.MetricSpec{}
			} else {
				mcs.currentMetricReachedMetrics = append(mcs.currentMetricReachedMetrics[:i], mcs.currentMetricReachedMetrics[i+1:]...)
			}
			return
		}
		// If the metric is reached and the condition is already reached, do nothing
		if isEqual && reached {
			return
		}
	}

	// If the condition is reached, and it was not reached before, update the condition status
	if reached && !mcs.reached {
		mcs.currentMetricReachedMetrics = append(mcs.currentMetricReachedMetrics, currentMetric)
		mcs.reached = true
		mcs.sinceTime = time.Now()
		return
	}

	// If the condition is already reached, do nothing, just add the metric to the list
	if reached && mcs.reached {
		mcs.currentMetricReachedMetrics = append(mcs.currentMetricReachedMetrics, currentMetric)
		return
	}

}

// isReached returns true if the condition is reached, false otherwise
func (mcs *metricConditionStatus) isReached() bool {
	if !mcs.reached {
		return false
	}
	if time.Since(mcs.sinceTime) > mcs.window {
		return true
	}
	return false
}
