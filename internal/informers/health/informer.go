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

package health

import (
	"context"
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
)

type HealthStatus struct {
	ModelName string
	Healthy   bool
}

type HealthInformerType int

const (
	K8SHealthInformerType HealthInformerType = iota
)

// healthInformerFactory is a factory function for creating a HealthInformer.
// It should be used to create a HealthInformer for a specific configuration.
type healthInformerFactory func(ctx context.Context, restConfig *rest.Config) (HealthInformer, error)

var (
	// healthInformerFactories is a map of health informer factories for different types
	// when a new health informer is added, it should be registered here
	healthInformerFactories = map[HealthInformerType]healthInformerFactory{
		K8SHealthInformerType: newK8SHealthInformer,
	}

	ErrUnknownInformerType = errors.New("unknown health informer type")
)

// NewHealthInformer creates a new HealthInformer for a given type
// If the informer does not exist, it returns an error
func NewHealthInformer(ctx context.Context, restConfig *rest.Config, informerType HealthInformerType) (HealthInformer, error) {
	factory, ok := healthInformerFactories[informerType]

	if !ok {
		return nil, fmt.Errorf("%w: %d", ErrUnknownInformerType, informerType)
	}
	return factory(ctx, restConfig)
}

//go:generate go run go.uber.org/mock/mockgen -source=informer.go -destination=informer_mock.go -package=health HealthInformer

// HealthInformer informs on the health of the source model.
type HealthInformer interface {
	// Start is non-blocking. It returns a channel that sends the health status of the local model when it changes.
	Start(ctx context.Context) <-chan HealthStatus
	// Register a model to the health informer. Resource is the resource that the model is running on.
	Register(ctx context.Context, model string, resource v1.ObjectReference)
	Unregister(ctx context.Context, model string)
	// Stop stops the health informer.
	Stop()
}
