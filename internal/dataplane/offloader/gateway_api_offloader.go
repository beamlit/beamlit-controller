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
	"time"

	modelv1alpha1 "github.com/beamlit/operator/api/v1alpha1"
	"github.com/beamlit/operator/internal/beamlit"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/log"
	v1 "sigs.k8s.io/gateway-api/apis/applyconfiguration/apis/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayv1client "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
)

// gatewayAPIOffloader is an offloader that uses Gateway API to offload models.
// It configures an HTTPRoute to route traffic to the local service and the remote service, with the given backend weight.
type gatewayAPIOffloader struct {
	token            *beamlit.BeamlitToken
	kubeClient       kubernetes.Interface
	gatewayClient    gatewayv1client.Interface
	gatewayName      string
	gatewayNamespace string
	httpRoutes       map[string]*types.NamespacedName
	tokenString      string
	workspace        string
}

func newGatewayAPIOffloader(ctx context.Context, kubeClient kubernetes.Interface, gatewayClient gatewayv1client.Interface, gatewayName string, gatewayNamespace string) (Offloader, error) {
	token, err := beamlit.NewBeamlitToken()
	if err != nil {
		return nil, err
	}
	gwapiOffloader := &gatewayAPIOffloader{
		token:            token,
		kubeClient:       kubeClient,
		gatewayClient:    gatewayClient,
		gatewayName:      gatewayName,
		gatewayNamespace: gatewayNamespace,
		httpRoutes:       make(map[string]*types.NamespacedName),
		workspace:        "",
	}
	go gwapiOffloader.watch(ctx)
	return gwapiOffloader, nil
}

func (o *gatewayAPIOffloader) watch(ctx context.Context) error {
	logger := log.FromContext(ctx)
	tokenString, err := o.token.GetToken(ctx)
	if err != nil {
		return err
	}
	o.tokenString = tokenString
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second * 10):
			tokenString, err := o.token.GetToken(ctx)
			if err != nil {
				logger.Error(err, "failed to get token")
				continue
			}
			if tokenString != o.tokenString {
				o.tokenString = tokenString
				for _, httpRoute := range o.httpRoutes {
					err := o.updateAuthHeader(ctx, httpRoute)
					if err != nil {
						logger.Error(err, "failed to update auth header", "httpRoute", httpRoute)
					}
				}
			}
		}
	}
}

func (o *gatewayAPIOffloader) updateAuthHeader(ctx context.Context, httpRoute *types.NamespacedName) error {
	h, err := o.gatewayClient.GatewayV1().HTTPRoutes(httpRoute.Namespace).Get(ctx, httpRoute.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	for _, rule := range h.Spec.Rules {
		for _, f := range rule.BackendRefs {
			for _, filter := range f.Filters {
				for _, set := range filter.RequestHeaderModifier.Set {
					if set.Name == "X-Beamlit-Authorization" {
						set.Value = "Bearer " + o.tokenString
					}
				}
			}
		}
	}
	_, err = o.gatewayClient.GatewayV1().HTTPRoutes(httpRoute.Namespace).Update(ctx, h, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (o *gatewayAPIOffloader) Configure(ctx context.Context, model *modelv1alpha1.ModelDeployment, backendServiceRef *modelv1alpha1.ServiceReference, remoteServiceRef *modelv1alpha1.ServiceReference, backendWeight int) error {
	service, err := o.kubeClient.CoreV1().Services(model.Spec.OffloadingConfig.LocalServiceRef.Namespace).Get(ctx, model.Spec.OffloadingConfig.LocalServiceRef.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if model.Status.Workspace != "" {
		o.workspace = model.Status.Workspace
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
							WithWeight(int32(backendWeight)).
							WithFilters(
								v1.HTTPRouteFilter().
									WithType(gatewayv1.HTTPRouteFilterType("RequestHeaderModifier")).
									WithRequestHeaderModifier(v1.HTTPHeaderFilter().
										WithSet(
											v1.HTTPHeader().WithName("X-Beamlit-Workspace").WithValue(o.workspace),
											v1.HTTPHeader().WithName("X-Beamlit-Model").WithValue(model.Name),
											v1.HTTPHeader().WithName("X-Beamlit-Authorization").WithValue("Bearer "+o.tokenString),
										),
									),
							),
					).
					WithMatches(
						v1.HTTPRouteMatch().
							WithPath(
								v1.HTTPPathMatch().
									WithType(gatewayv1.PathMatchType("PathPrefix")).
									WithValue("/"),
							),
					),
			),
		)

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
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	delete(o.httpRoutes, model.Namespace)
	return nil
}
