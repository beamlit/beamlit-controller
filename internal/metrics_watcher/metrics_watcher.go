package metrics_watcher

import (
	"context"
	"sync"
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/scale"
	scaleclient "k8s.io/client-go/scale"
	"k8s.io/kubernetes/pkg/controller/podautoscaler/metrics"
	resourceclient "k8s.io/metrics/pkg/client/clientset/versioned/typed/metrics/v1beta1"
	"k8s.io/metrics/pkg/client/custom_metrics"
	"k8s.io/metrics/pkg/client/external_metrics"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// MetricsWatcher represents a watcher for Kubernetes metrics
type MetricsWatcher struct {
	scaleClient      scaleclient.ScalesGetter
	kubernetesClient client.Client
	metricsClient    metrics.MetricsClient
	interval         time.Duration
	watchers         []*metricWatcher // Start a watcher for each ModelDeployment with offloading enabled
}

// NewMetricsWatcher creates a new MetricsWatcher
func NewMetricsWatcher(restConfig *rest.Config, kubernetesClient client.Client, interval time.Duration) (*MetricsWatcher, error) {
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

	scaleKindResolver := scale.NewDiscoveryScaleKindResolver(discoveryClient)
	scaleClient, err := scale.NewForConfig(restConfig, kubernetesClient.RESTMapper(), dynamic.LegacyAPIPathResolverFunc, scaleKindResolver)
	if err != nil {
		return nil, err
	}
	return &MetricsWatcher{
		interval:         interval,
		kubernetesClient: kubernetesClient,
		scaleClient:      scaleClient,
		metricsClient:    metricsClient,
		watchers:         make([]*metricWatcher, 0),
	}, nil
}

// AddWatcher adds a new MetricWatcher to the MetricsWatcher
func (mw *MetricsWatcher) Watch(ctx context.Context, watchTarget v1.ObjectReference, metric []autoscalingv2.MetricSpec, conditionDuration time.Duration, action func(bool) error) {
	log.FromContext(ctx).Info("Watching metrics", "watchTarget", watchTarget, "metric", metric, "conditionDuration", conditionDuration)
	mw.watchers = append(mw.watchers, &metricWatcher{
		condition: metricConditionStatus{
			conditionDuration:           conditionDuration,
			currentMetricReachedMetrics: make([]autoscalingv2.MetricSpec, 0),
			reached:                     false,
			sinceTime:                   time.Time{},
		},
		metric:        metric,
		metricsClient: mw.metricsClient,
		scaleClient:   mw.scaleClient,
		kubeClient:    mw.kubernetesClient,
		watchTarget:   watchTarget,
		callback:      action,
	})
	log.FromContext(ctx).Info("Added watcher", "watchTarget", watchTarget, "metric", metric, "conditionDuration", conditionDuration)
}

// Start begins the metrics watching process
func (mw *MetricsWatcher) Start(ctx context.Context) {
	ticker := time.NewTicker(mw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mw.checkMetrics(ctx)
		}
	}
}

// checkMetrics checks all registered watchers and triggers actions if necessary
func (mw *MetricsWatcher) checkMetrics(ctx context.Context) {
	var wg sync.WaitGroup
	for _, watcher := range mw.watchers {
		wg.Add(1)
		go func(watcher *metricWatcher) {
			defer wg.Done()
			isReached, err := watcher.CheckMetric(ctx)
			if err != nil {
				log.FromContext(ctx).Error(err, "Error checking metric")
				return
			}
			err = watcher.TriggerAction(isReached)
			if err != nil {
				log.FromContext(ctx).Error(err, "Error triggering action")
			}
		}(watcher)
	}
	wg.Wait()
}
