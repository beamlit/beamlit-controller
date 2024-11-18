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

package configurer

import (
	"context"
	"fmt"

	modelv1alpha1 "github.com/beamlit/beamlit-controller/api/v1alpha1/deployment"
	"k8s.io/client-go/kubernetes"
)

type ConfigurerType string

const (
	KubernetesConfigurerType ConfigurerType = "kubernetes"
)

type configurerFactory func(ctx context.Context, kubeClient kubernetes.Interface) (Configurer, error)

var (
	configurerFactories = map[ConfigurerType]configurerFactory{
		KubernetesConfigurerType: newKubernetesConfigurer,
	}
)

// NewConfigurer creates a new configurer for the given type
func NewConfigurer(ctx context.Context, configurerType ConfigurerType, kubeClient kubernetes.Interface) (Configurer, error) {
	factory, ok := configurerFactories[configurerType]
	if !ok {
		return nil, fmt.Errorf("unknown configurer type: %s", configurerType)
	}
	return factory(ctx, kubeClient)
}

//go:generate go run go.uber.org/mock/mockgen -source=configurer.go -destination=configurer_mock.go -package=configurer Configurer

// Configurer is an interface for configuring services to be proxied by Beamlit.
// It replaces the EndpointsSlice in the service with a new EndpointsSlice that points to the Beamlit proxy Service.
// It also creates a new Service that can be used by the proxy to route traffic to the internal pod
type Configurer interface {
	// Start starts the service configurer.
	Start(ctx context.Context, gatewayService *modelv1alpha1.ServiceReference) error
	// Configure configures a service to be proxied by Beamlit.
	Configure(ctx context.Context, service *modelv1alpha1.ServiceReference) error
	// Unconfigure unconfigures a service from being proxied by Beamlit.
	Unconfigure(ctx context.Context, service *modelv1alpha1.ServiceReference) error

	// GetService gets the service for a given service reference.
	GetLocalBeamlitService(ctx context.Context, service *modelv1alpha1.ServiceReference) (*modelv1alpha1.ServiceReference, error)
}
