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

package offloader

import (
	"context"
	"fmt"

	modelv1alpha1 "github.com/beamlit/beamlit-controller/api/v1alpha1/deployment"
	beamlitclientset "github.com/beamlit/beamlit-controller/gateway/clientset"
	"k8s.io/client-go/kubernetes"
)

type OffloaderType string

const (
	GatewayAPIOffloaderType     OffloaderType = "gateway-api"
	BeamlitGatewayOffloaderType OffloaderType = "beamlit-gateway"
)

type offloaderFactory func(ctx context.Context, kubeClient kubernetes.Interface, proxyClient *beamlitclientset.ClientSet) (Offloader, error)

var offloaderFactories = map[OffloaderType]offloaderFactory{
	BeamlitGatewayOffloaderType: newBeamlitGatewayOffloader,
}

// NewOffloader creates a new offloader for the given type
func NewOffloader(ctx context.Context, offloaderType OffloaderType, kubeClient kubernetes.Interface, proxyClient *beamlitclientset.ClientSet) (Offloader, error) {
	factory, ok := offloaderFactories[offloaderType]
	if !ok {
		return nil, fmt.Errorf("unsupported offloader type: %s", offloaderType)
	}
	return factory(ctx, kubeClient, proxyClient)
}

//go:generate go run go.uber.org/mock/mockgen -source=offloader.go -destination=offloader_mock.go -package=offloader Offloader

// Offloader is responsible for configuring the underlying infrastructure to offload the model.
type Offloader interface {
	// Configure configures the offloader with the given model, backend service reference, remote service reference, and backend weight.
	Configure(ctx context.Context, model *modelv1alpha1.ModelDeployment, localBackend *modelv1alpha1.ServiceReference, remoteBackend *modelv1alpha1.RemoteBackend, remoteBackendWeight int) error
	// Cleanup cleans up the offloader for the given model. It should remove any resources created by the offloader for the given model.
	Cleanup(ctx context.Context, model *modelv1alpha1.ModelDeployment) error
}
