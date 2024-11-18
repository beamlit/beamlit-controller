package offloader

import (
	"context"
	"fmt"
	"strings"
	"sync"

	modelv1alpha1 "github.com/beamlit/beamlit-controller/api/v1alpha1/deployment"
	proxyv1alpha1 "github.com/beamlit/beamlit-controller/gateway/api/v1alpha1"
	beamlitclientset "github.com/beamlit/beamlit-controller/gateway/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type beamlitGatewayOffloader struct {
	managedRoutes    sync.Map // key: model name, value: bool
	kubeClient       kubernetes.Interface
	managementClient *beamlitclientset.ClientSet
	workspace        string // TODO: remove this
}

func newBeamlitGatewayOffloader(ctx context.Context, kubeClient kubernetes.Interface, managementClient *beamlitclientset.ClientSet) (Offloader, error) {
	return &beamlitGatewayOffloader{kubeClient: kubeClient, managementClient: managementClient, managedRoutes: sync.Map{}}, nil
}

func (o *beamlitGatewayOffloader) Configure(ctx context.Context, model *modelv1alpha1.ModelDeployment, localBackend *modelv1alpha1.ServiceReference, remoteBackend *modelv1alpha1.RemoteBackend, remoteBackendWeight int) error {
	service, err := o.kubeClient.CoreV1().Services(model.Spec.ServiceRef.Namespace).Get(ctx, model.Spec.ServiceRef.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if o.workspace == "" && model.Status.Workspace != "" {
		o.workspace = model.Status.Workspace
	}
	route := proxyv1alpha1.Route{
		Name: model.Name,
		Hostnames: []string{
			service.Spec.ClusterIP,
			service.Name,
			fmt.Sprintf("%s.%s", service.Name, service.Namespace),
			fmt.Sprintf("%s.%s.svc", service.Name, service.Namespace),
			fmt.Sprintf("%s.%s.svc.cluster.local", service.Name, service.Namespace),
		},
		Backends: []proxyv1alpha1.Backend{
			{
				Host:   fmt.Sprintf("%s.%s.svc.cluster.local:%d", localBackend.Name, localBackend.Namespace, localBackend.TargetPort),
				Weight: 100 - remoteBackendWeight,
				Scheme: "http", // TODO: support HTTPS
			},
			{
				Host:         remoteBackend.Host,
				Weight:       remoteBackendWeight,
				Scheme:       string(remoteBackend.Scheme),
				HeadersToAdd: remoteBackend.HeadersToAdd,
				PathPrefix:   o.variableReplace(remoteBackend.PathPrefix, model),
			},
		},
	}
	if authConfig := remoteBackend.AuthConfig; authConfig != nil {
		var authType proxyv1alpha1.AuthType
		if authConfig.Type == modelv1alpha1.AuthTypeOAuth {
			authType = proxyv1alpha1.AuthTypeOAuth
		}
		route.Backends[1].Auth = &proxyv1alpha1.Auth{
			Type: authType,
		}
		if authConfig.OAuthConfig != nil {
			route.Backends[1].Auth.OAuth = &proxyv1alpha1.OAuth{
				ClientID:     authConfig.OAuthConfig.ClientID,
				ClientSecret: authConfig.OAuthConfig.ClientSecret,
				TokenURL:     authConfig.OAuthConfig.TokenURL,
			}
		}
	}
	if _, ok := o.managedRoutes.Load(model.Name); ok {
		_, err = o.managementClient.UpdateRoute(ctx, route)
	} else {
		_, err = o.managementClient.RegisterRoute(ctx, route)
	}
	if err != nil {
		return err
	}
	o.managedRoutes.Store(model.Name, true)
	return nil
}

func (o *beamlitGatewayOffloader) Cleanup(ctx context.Context, model *modelv1alpha1.ModelDeployment) error {
	if _, ok := o.managedRoutes.Load(model.Name); !ok {
		return nil
	}
	_, err := o.managementClient.DeleteRoute(ctx, model.Name)
	o.managedRoutes.Delete(model.Name)
	return err
}

func (o *beamlitGatewayOffloader) variableReplace(pathPrefix string, model *modelv1alpha1.ModelDeployment) string {
	pathPrefix = strings.ReplaceAll(pathPrefix, "$workspace", o.workspace)
	pathPrefix = strings.ReplaceAll(pathPrefix, "$model", model.Spec.Model)
	return pathPrefix
}
