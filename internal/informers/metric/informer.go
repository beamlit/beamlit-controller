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
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

type MetricStatus struct {
	ModelName string
	Reached   bool
}

type MetricInformerType int

const (
	K8SMetricInformerType MetricInformerType = iota
	PrometheusMetricInformerType
)

// MetricInformerFactory is a factory function for creating a MetricInformer.
// It should be used to create a MetricInformer for a specific configuration.
type metricInformerFactory func(ctx context.Context, restConfig *rest.Config) (MetricInformer, error)

var (
	// MetricInformerFactories is a map of metric informer factories for different types
	// when a new metric informer is added, it should be registered here
	metricInformerFactories = map[MetricInformerType]metricInformerFactory{
		K8SMetricInformerType:        newK8sMetricInformer,
		PrometheusMetricInformerType: newPrometheusMetricInformer,
	}

	ErrUnknownInformerType = errors.New("unknown metric informer type")
)

// NewMetricInformer creates a new MetricInformer for a given type
// If the informer does not exist, it returns an error
func NewMetricInformer(ctx context.Context, restConfig *rest.Config, informerType MetricInformerType) (MetricInformer, error) {
	factory, ok := metricInformerFactories[informerType]
	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrUnknownInformerType, informerType)
	}
	return factory(ctx, restConfig)
}

//go:generate go run go.uber.org/mock/mockgen -source=informer.go -destination=informer_mock.go -package=metric MetricInformer

// MetricInformer is an interface for a metric informer.
// It is used to get the status of a metric for a given model and resource.
// It is used by the controller to determine if the metric is reached for a given model and resource.
type MetricInformer interface {
	// Start starts the metric informer and returns a channel that will receive the status of the metric for a given model and resource.
	Start(ctx context.Context) <-chan MetricStatus
	// Register registers a model and resource to the metric informer.
	Register(ctx context.Context, model string, metrics []autoscalingv2.MetricSpec, resource v1.ObjectReference, scrapeInterval time.Duration, window time.Duration)
	// Unregister unregisters a model from the metric informer.
	Unregister(ctx context.Context, model string)
	// Stop stops the metric informer.
	Stop()
}
