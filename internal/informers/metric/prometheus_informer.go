package metric

import (
	"context"
	"fmt"
	"time"

	prometheus "github.com/prometheus/client_golang/api"
	prometheusapi "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PrometheusMetricInformer struct {
	metricChan chan MetricStatus
	watchers   map[string]*prometheusMetricWatcher
	stopChs    map[string]chan bool
	client     prometheusapi.API
}

func newPrometheusMetricInformer(ctx context.Context, restConfig *rest.Config) (MetricInformer, error) {
	client, err := prometheus.NewClient(prometheus.Config{Address: restConfig.Host})
	if err != nil {
		return nil, err
	}
	prometheusClient := prometheusapi.NewAPI(client)
	return &PrometheusMetricInformer{
		client:     prometheusClient,
		metricChan: make(chan MetricStatus),
		watchers:   make(map[string]*prometheusMetricWatcher),
		stopChs:    make(map[string]chan bool),
	}, nil
}

func (p *PrometheusMetricInformer) Register(ctx context.Context, model string, metrics []autoscalingv2.MetricSpec, resource v1.ObjectReference, scrapeInterval time.Duration, window time.Duration) {
	stopChan := make(chan bool)
	p.stopChs[model] = stopChan
	p.watchers[model] = &prometheusMetricWatcher{
		client:            p.client,
		model:             model,
		metrics:           metrics,
		resource:          resource,
		scrapeInterval:    scrapeInterval,
		window:            window,
		metricChan:        p.metricChan,
		lastReachedStatus: false,
		stopChan:          stopChan,
		condition: metricConditionStatus{
			window:                      window,
			sinceTime:                   time.Time{},
			currentMetricReachedMetrics: []autoscalingv2.MetricSpec{},
			reached:                     false,
		},
	}
	go p.watchers[model].start(ctx)
}

func (p *PrometheusMetricInformer) Unregister(ctx context.Context, model string) {
	stopChan, ok := p.stopChs[model]
	if !ok {
		return
	}
	close(stopChan)
	delete(p.stopChs, model)
	delete(p.watchers, model)
}

func (p *PrometheusMetricInformer) Start(ctx context.Context) <-chan MetricStatus {
	return p.metricChan
}

func (p *PrometheusMetricInformer) Stop() {
	for _, stopChan := range p.stopChs {
		close(stopChan)
	}
	for model := range p.watchers {
		delete(p.watchers, model)
	}
	for model := range p.stopChs {
		delete(p.stopChs, model)
	}
}

type prometheusMetricWatcher struct {
	client            prometheusapi.API
	model             string
	metrics           []autoscalingv2.MetricSpec
	resource          v1.ObjectReference
	scrapeInterval    time.Duration
	window            time.Duration
	condition         metricConditionStatus
	metricChan        chan MetricStatus
	lastReachedStatus bool // last status of the metric
	stopChan          chan bool
}

func (p *prometheusMetricWatcher) start(ctx context.Context) {
	logger := log.FromContext(ctx)
	logger.Info("Starting prometheus metric watcher", "model", p.model)
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		case <-time.After(p.scrapeInterval):
			for _, metric := range p.metrics {
				logger.Info("Scraping metric", "metric", metric)
				if metric.Type != autoscalingv2.ExternalMetricSourceType {
					continue
				}
				queryBuilder := &QueryBuilder{
					metricName: metric.External.Metric.Name,
				}
				if metric.External.Metric.Selector != nil {
					for key, value := range metric.External.Metric.Selector.MatchLabels {
						queryBuilder.WithSelector(key, value)
					}
				}
				if metric.External.Target.AverageValue != nil {
					queryBuilder.WithKind(ValueKindAverage)
				}
				if metric.External.Target.Value != nil {
					queryBuilder.WithKind(ValueKindCount)
				}
				queryBuilder.WithWindow(p.window)
				query := queryBuilder.Build()
				result, warnings, err := p.client.Query(ctx, query, time.Now())
				if err != nil {
					logger.Error(err, "Error querying prometheus")
					continue
				}
				for _, warning := range warnings {
					logger.V(1).Info("Prometheus warning", "warning", warning)
				}
				logger.Info("Query", "query", query, "result", result)
				value := float64(0)
				if metric.External.Target.AverageValue != nil {
					value = metric.External.Target.AverageValue.AsApproximateFloat64()
				} else if metric.External.Target.Value != nil {
					value = metric.External.Target.Value.AsApproximateFloat64()
				}
				switch result.Type() {
				case model.ValScalar:
					// Transform the result to a scalar
					resultBytes, err := result.Type().MarshalJSON()
					if err != nil {
						logger.Error(err, "Error marshalling result to scalar")
						continue
					}
					scalar := model.Scalar{}
					err = scalar.UnmarshalJSON(resultBytes)
					if err != nil {
						logger.Error(err, "Error unmarshalling result to scalar")
						continue
					}
					if float64(scalar.Value) > value {
						p.condition.update(ctx, metric, true)
						continue
					}
					p.condition.update(ctx, metric, false)
				case model.ValVector:
					reached := false
					for _, sample := range result.(model.Vector) {
						if float64(sample.Value) > value {
							reached = true
							break
						}
					}
					p.condition.update(ctx, metric, reached)
				case model.ValMatrix:
					logger.Error(fmt.Errorf("unsupported metric type: %s", result.Type()), "Unsupported metric type, only scalar is supported")
				case model.ValNone:
					logger.Error(fmt.Errorf("no data returned from prometheus"), "No data returned from prometheus")
				default:
					logger.Error(fmt.Errorf("unsupported metric type: %s", result.Type()), "Unsupported metric type, only scalar is supported")
				}
				if p.lastReachedStatus != p.condition.reached {
					p.lastReachedStatus = p.condition.reached
					p.metricChan <- MetricStatus{
						ModelName: p.model,
						Reached:   p.condition.reached,
					}
				}
			}
		}
	}
}

type ValueKind string

const (
	ValueKindAverage ValueKind = "avg"
	ValueKindCount   ValueKind = "count"
)

// QueryBuilder is a builder for prometheus queries
type QueryBuilder struct {
	metricName string
	kind       ValueKind
	selectors  map[string]string
	window     time.Duration
}

func (qb *QueryBuilder) WithSelector(key, value string) {
	if qb.selectors == nil {
		qb.selectors = make(map[string]string)
	}
	qb.selectors[key] = value
}

func (qb *QueryBuilder) WithKind(kind ValueKind) {
	qb.kind = kind
}

func (qb *QueryBuilder) WithWindow(window time.Duration) {
	qb.window = time.Second * 60 // default to 60s, make it configurable later
}

func (qb *QueryBuilder) Build() string {
	query := qb.metricName
	for key, value := range qb.selectors {
		query = fmt.Sprintf("%s{%s=\"%s\"}", query, key, value)
	}
	if qb.kind != "" && qb.kind == ValueKindAverage {
		query = fmt.Sprintf("rate(%s[%s])", query, qb.window)
	}
	return query
}
