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

	modelv1alpha1 "github.com/beamlit/operator/api/v1alpha1"
	"github.com/beamlit/operator/internal/beamlit"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	v1 "sigs.k8s.io/gateway-api/apis/applyconfiguration/apis/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1client "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// gatewayAPIOffloader is an offloader that uses Gateway API to offload models.
// It configures an HTTPRoute to route traffic to the local service and the remote service, with the given backend weight.
type gatewayAPIOffloader struct {
	kubeClient       kubernetes.Interface
	gatewayClient    gatewayv1client.Interface
	gatewayName      string
	gatewayNamespace string
	httpRoutes       map[string]*types.NamespacedName
}

func newGatewayAPIOffloader(ctx context.Context, kubeClient kubernetes.Interface, gatewayClient gatewayv1client.Interface, gatewayName string, gatewayNamespace string) (Offloader, error) {
	gwapiOffloader := &gatewayAPIOffloader{
		kubeClient:       kubeClient,
		gatewayClient:    gatewayClient,
		gatewayName:      gatewayName,
		gatewayNamespace: gatewayNamespace,
		httpRoutes:       make(map[string]*types.NamespacedName),
	}
	go gwapiOffloader.watch(ctx)
	return gwapiOffloader, nil
}

func (o *gatewayAPIOffloader) watch(ctx context.Context) error {
	// TODO: Implement watch for Token Refresh
	for {
		select {
		case <-ctx.Done():
			return nil
		}
	}
}

func (o *gatewayAPIOffloader) Configure(ctx context.Context, model *modelv1alpha1.ModelDeployment, backendServiceRef *modelv1alpha1.ServiceReference, remoteServiceRef *modelv1alpha1.ServiceReference, backendWeight int) error {
	service, err := o.kubeClient.CoreV1().Services(model.Spec.OffloadingConfig.LocalServiceRef.Namespace).Get(ctx, model.Spec.OffloadingConfig.LocalServiceRef.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	token, err := beamlit.NewBeamlitToken()
	if err != nil {
		return err
	}
	tokenString, err := token.GetToken(ctx)
	if err != nil {
		return err
	}

	httpRouteApply := v1.HTTPRoute(fmt.Sprintf("%s-http-route", model.Name), service.Namespace).
		WithSpec(v1.HTTPRouteSpec().
			WithParentRefs(v1.ParentReference().
				WithName(gatewayv1.ObjectName(o.gatewayName)).
				WithKind(gatewayv1.Kind("Gateway")).
				WithGroup(gatewayv1.Group("gateway.networking.k8s.io")).
				WithNamespace(gatewayv1.Namespace(o.gatewayNamespace))).
			WithHostnames(
				gatewayv1.Hostname(service.Spec.ClusterIP),
				gatewayv1.Hostname(service.Name),
				gatewayv1.Hostname(fmt.Sprintf("%s.%s", service.Name, service.Namespace)),
				gatewayv1.Hostname(fmt.Sprintf("%s.%s.svc", service.Name, service.Namespace)),
				gatewayv1.Hostname(fmt.Sprintf("%s.%s.svc.cluster.local", service.Name, service.Namespace)),
			).
			WithRules(
				v1.HTTPRouteRule().
					WithBackendRefs(
						v1.HTTPBackendRef().
							WithKind(gatewayv1.Kind("Service")).
							WithName(gatewayv1.ObjectName(backendServiceRef.Name)).
							WithNamespace(gatewayv1.Namespace(backendServiceRef.Namespace)).
							WithPort(gatewayv1.PortNumber(backendServiceRef.TargetPort)).
							WithWeight(int32(100-backendWeight)),
						v1.HTTPBackendRef().
							WithKind(gatewayv1.Kind("Service")).
							WithName(gatewayv1.ObjectName(remoteServiceRef.Name)).
							WithNamespace(gatewayv1.Namespace(remoteServiceRef.Namespace)).
							WithPort(gatewayv1.PortNumber(remoteServiceRef.TargetPort)).
							WithWeight(int32(backendWeight))).
					WithMatches(v1.HTTPRouteMatch().
						WithPath(v1.HTTPPathMatch().
							WithType(gatewayv1.PathMatchType("PathPrefix")).
							WithValue("/"))).
					WithFilters(v1.HTTPRouteFilter().
						WithType(gatewayv1.HTTPRouteFilterType("RequestHeaderModifier")).
						WithRequestHeaderModifier(v1.HTTPHeaderFilter().
							WithSet(
								v1.HTTPHeader().WithName("X-Beamlit-Model").WithValue(model.Name),
								v1.HTTPHeader().WithName("X-Beamlit-Authorization").WithValue("Bearer "+tokenString),
							)))))

	_, err = o.gatewayClient.GatewayV1().HTTPRoutes(service.Namespace).Apply(ctx, httpRouteApply, metav1.ApplyOptions{
		FieldManager: "beamlit-operator",
	})
	if err != nil {
		return err
	}
	o.httpRoutes[service.Namespace] = &types.NamespacedName{
		Namespace: service.Namespace,
		Name:      fmt.Sprintf("%s-http-route", model.Name),
	}
	return nil
}

func (o *gatewayAPIOffloader) Cleanup(ctx context.Context, model *modelv1alpha1.ModelDeployment) error {
	httpRoute, ok := o.httpRoutes[model.Namespace]
	if !ok {
		return nil
	}
	err := o.gatewayClient.GatewayV1().HTTPRoutes(httpRoute.Namespace).Delete(ctx, httpRoute.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}
