package offloading

import (
	"context"
	"fmt"
	"log"

	modelv1alpha1 "github.com/beamlit/operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	clientsetgatewayv1 "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/typed/apis/v1"
)

const (
	GatewayServiceName = "beamlit-gateway-service"
)

type Offloader struct {
	gatewayName               string
	gatewayNamespace          string
	gatewayPort               int
	gatewaySelectors          map[string]string
	kubeClient                client.Client
	gatewayClient             *clientsetgatewayv1.GatewayV1Client
	watchList                 *cache.ListWatch
	interceptorServiceName    string
	frontendServices          []*v1.Service
	serviceBackendPerFrontend map[string]*v1.Service
	HTTPRoutePerLocalService  map[string]*gatewayv1.HTTPRoute // key is the local service name (namespace/name)
}

func NewOffloader(ctx context.Context, kubeClient client.Client, cacheClient *kubernetes.Clientset, gatewayClient *clientsetgatewayv1.GatewayV1Client, gatewaySelectors map[string]string, gatewayNamespace, gatewayName string, gatewayPort int) (*Offloader, error) {
	// TODO: Fix this default namespace
	watchList := cache.NewListWatchFromClient(cacheClient.CoreV1().RESTClient(), "services", v1.NamespaceAll, fields.Everything())
	interceptorService := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-interceptor", gatewayName),
			Namespace: gatewayNamespace,
		},
		Spec: v1.ServiceSpec{
			Selector: gatewaySelectors,
			Type:     v1.ServiceTypeClusterIP,
			Ports: []v1.ServicePort{
				{
					Port:       int32(gatewayPort),
					Name:       fmt.Sprintf("%s-interceptor-%d-%d", gatewayName, gatewayPort, gatewayPort),
					TargetPort: intstr.FromInt(gatewayPort),
				},
			},
			ExternalIPs: []string{},
		},
	}
	err := kubeClient.Create(ctx, interceptorService)
	if err != nil {
		return nil, err
	}
	o := &Offloader{
		gatewayName:               gatewayName,
		gatewayNamespace:          gatewayNamespace,
		gatewayPort:               gatewayPort,
		gatewayClient:             gatewayClient,
		kubeClient:                kubeClient,
		watchList:                 watchList,
		frontendServices:          make([]*v1.Service, 0),
		interceptorServiceName:    interceptorService.Name,
		gatewaySelectors:          gatewaySelectors,
		serviceBackendPerFrontend: make(map[string]*v1.Service),
		HTTPRoutePerLocalService:  make(map[string]*gatewayv1.HTTPRoute),
	}
	go o.watchServices(ctx)
	go o.keepInterceptorsInSync(ctx)
	return o, nil
}

func (o *Offloader) Register(ctx context.Context, localServiceRef *modelv1alpha1.ServiceReference, remoteServiceRef *modelv1alpha1.ServiceReference, modelName string) error {
	// Create ServiceBackend InTheSameNamespace pointing to the localServiceRef
	log.Println("Registering model deployment", modelName, "for local service", localServiceRef.Name, "and remote service", remoteServiceRef.Name)
	localServiceDefinition := &v1.Service{}
	err := o.kubeClient.Get(ctx, client.ObjectKey{
		Namespace: localServiceRef.Namespace,
		Name:      localServiceRef.Name,
	}, localServiceDefinition)
	if err != nil {
		return err
	}
	if localServiceDefinition.Name == "" {
		return fmt.Errorf("local service %s/%s not found", localServiceRef.Namespace, localServiceRef.Name)
	}
	// Check if the service backend already exists
	serviceBackend := &v1.Service{}
	err = o.kubeClient.Get(ctx, client.ObjectKey{
		Namespace: localServiceRef.Namespace,
		Name:      fmt.Sprintf("%s-backend-beamlit", localServiceDefinition.Name),
	}, serviceBackend)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if serviceBackend.Name == fmt.Sprintf("%s-backend-beamlit", localServiceDefinition.Name) {
		return nil
	}
	serviceBackend = &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-backend-beamlit", localServiceDefinition.Name),
			Namespace: localServiceRef.Namespace,
		},
		Spec: v1.ServiceSpec{
			Selector: localServiceDefinition.Spec.Selector,
			Type:     v1.ServiceTypeClusterIP,
			Ports:    localServiceDefinition.Spec.Ports,
		},
	}
	err = o.kubeClient.Create(ctx, serviceBackend)
	if err != nil {
		return err
	}
	o.serviceBackendPerFrontend[fmt.Sprintf("%s/%s", localServiceRef.Namespace, localServiceRef.Name)] = serviceBackend
	// Add localServiceRef clusterIP to the backendService ExternalIPs and port
	log.Println("Adding local service clusterIP to the interceptor service", localServiceDefinition.Spec.ClusterIP)
	interceptorService := &v1.Service{}
	err = o.kubeClient.Get(ctx, client.ObjectKey{
		Namespace: o.gatewayNamespace,
		Name:      o.interceptorServiceName,
	}, interceptorService)
	if err != nil || interceptorService.Name == "" {
		return err
	}
	interceptorService.Spec.ExternalIPs = append(interceptorService.Spec.ExternalIPs, localServiceDefinition.Spec.ClusterIP)
	var isPortPresent bool
	for _, port := range interceptorService.Spec.Ports {
		if port.Port == int32(localServiceRef.TargetPort) {
			isPortPresent = true
			break
		}
	}
	if !isPortPresent {
		// TODO: Implement clean up on deletion of the service
		interceptorService.Spec.Ports = append(interceptorService.Spec.Ports, v1.ServicePort{Port: int32(localServiceRef.TargetPort), TargetPort: intstr.FromInt(o.gatewayPort), Name: fmt.Sprintf("%s-interceptor-%d-%d", o.gatewayName, localServiceRef.TargetPort, o.gatewayPort)})
	}
	err = o.kubeClient.Update(ctx, interceptorService)
	if err != nil {
		return err
	}
	// Create HTTPRoute pointing to the backendService and the remoteServiceRef
	log.Println("Creating HTTPRoute for model deployment", modelName, "for local service", localServiceRef.Name, "and remote service", remoteServiceRef.Name)
	httpRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-http-route", modelName),
			Namespace: localServiceRef.Namespace,
		},
		Spec: gatewayv1.HTTPRouteSpec{
			CommonRouteSpec: gatewayv1.CommonRouteSpec{
				ParentRefs: []gatewayv1.ParentReference{
					{
						Name:      gatewayv1.ObjectName(o.gatewayName),
						Kind:      createKind("Gateway"),
						Group:     createGroup("gateway.networking.k8s.io"),
						Namespace: createNamespace(o.gatewayNamespace),
					},
				},
			},
			Hostnames: []gatewayv1.Hostname{
				// TODO: Handle update of the service
				gatewayv1.Hostname(localServiceDefinition.Spec.ClusterIP),                                                   // CluterIP
				gatewayv1.Hostname(localServiceRef.Name),                                                                    // test
				gatewayv1.Hostname(fmt.Sprintf("%s.%s", localServiceRef.Name, localServiceRef.Namespace)),                   // test.default
				gatewayv1.Hostname(fmt.Sprintf("%s.%s.svc", localServiceRef.Name, localServiceRef.Namespace)),               // test.default.svc
				gatewayv1.Hostname(fmt.Sprintf("%s.%s.svc.cluster.local", localServiceRef.Name, localServiceRef.Namespace)), // test.default.svc.cluster.local
			},
			Rules: []gatewayv1.HTTPRouteRule{
				{
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Kind:      createServiceKind(),
									Name:      gatewayv1.ObjectName(serviceBackend.Name),
									Namespace: createNamespace(serviceBackend.Namespace),
									Port:      createPortNumber(localServiceRef.TargetPort),
								},
								Weight: createWeight(100),
							},
						},
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Kind:      createServiceKind(),
									Name:      gatewayv1.ObjectName(remoteServiceRef.Name),
									Namespace: createNamespace(remoteServiceRef.Namespace),
									Port:      createPortNumber(remoteServiceRef.TargetPort),
								},
								Weight: createWeight(0),
							},
						},
					},
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  createPathMatchType("PathPrefix"),
								Value: createPathValue("/"),
							},
						},
					},
					Filters: []gatewayv1.HTTPRouteFilter{
						{
							Type: gatewayv1.HTTPRouteFilterRequestHeaderModifier,
							RequestHeaderModifier: &gatewayv1.HTTPHeaderFilter{
								Set: []gatewayv1.HTTPHeader{
									{
										Name:  "X-Beamlit-Model",
										Value: modelName,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	_, err = o.gatewayClient.HTTPRoutes(localServiceRef.Namespace).Create(ctx, httpRoute, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	o.HTTPRoutePerLocalService[fmt.Sprintf("%s/%s", localServiceRef.Namespace, localServiceRef.Name)] = httpRoute
	return nil
}

func (o *Offloader) Offload(ctx context.Context, localServiceRef *modelv1alpha1.ServiceReference, percentage int) error {
	// Offload the localServiceRef to the remoteServiceRef by updating the HTTPRoute weight
	httpRoute, ok := o.HTTPRoutePerLocalService[fmt.Sprintf("%s/%s", localServiceRef.Namespace, localServiceRef.Name)]
	if !ok {
		return fmt.Errorf("HTTPRoute not found for local service %s", localServiceRef.Name)
	}
	httpRoute, err := o.gatewayClient.HTTPRoutes(localServiceRef.Namespace).Get(ctx, httpRoute.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	httpRoute.Spec.Rules[0].BackendRefs[0].Weight = createWeight(100 - int32(percentage))
	httpRoute.Spec.Rules[0].BackendRefs[1].Weight = createWeight(int32(percentage))
	httpRoute, err = o.gatewayClient.HTTPRoutes(localServiceRef.Namespace).Update(ctx, httpRoute, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	o.HTTPRoutePerLocalService[fmt.Sprintf("%s/%s", localServiceRef.Namespace, localServiceRef.Name)] = httpRoute
	return nil
}

func (o *Offloader) Unregister(ctx context.Context, localServiceRef *modelv1alpha1.ServiceReference) error {
	httpRoute, ok := o.HTTPRoutePerLocalService[fmt.Sprintf("%s/%s", localServiceRef.Namespace, localServiceRef.Name)]
	if !ok {
		return fmt.Errorf("HTTPRoute not found for local service %s", localServiceRef.Name)
	}
	err := o.gatewayClient.HTTPRoutes(localServiceRef.Namespace).Delete(ctx, httpRoute.Name, metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	delete(o.HTTPRoutePerLocalService, fmt.Sprintf("%s/%s", localServiceRef.Namespace, localServiceRef.Name))
	err = o.kubeClient.Delete(ctx, o.serviceBackendPerFrontend[fmt.Sprintf("%s/%s", localServiceRef.Namespace, localServiceRef.Name)])
	if err != nil {
		return err
	}
	delete(o.serviceBackendPerFrontend, fmt.Sprintf("%s/%s", localServiceRef.Namespace, localServiceRef.Name))
	var localServiceDefinition *v1.Service
	err = o.kubeClient.Get(ctx, client.ObjectKey{
		Namespace: localServiceRef.Namespace,
		Name:      localServiceRef.Name,
	}, localServiceDefinition)
	if err != nil {
		return err
	}
	if localServiceDefinition == nil {
		return fmt.Errorf("local service %s/%s not found", localServiceRef.Namespace, localServiceRef.Name)
	}
	interceptorService := &v1.Service{}
	err = o.kubeClient.Get(ctx, client.ObjectKey{
		Namespace: o.gatewayNamespace,
		Name:      o.interceptorServiceName,
	}, interceptorService)
	if err != nil || interceptorService.Name == "" {
		return err
	}
	for i, externalIP := range interceptorService.Spec.ExternalIPs {
		if externalIP == localServiceDefinition.Spec.ClusterIP {
			interceptorService.Spec.ExternalIPs = append(interceptorService.Spec.ExternalIPs[:i], interceptorService.Spec.ExternalIPs[i+1:]...)
			err = o.kubeClient.Update(ctx, interceptorService)
			if err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func createKind(kind string) *gatewayv1.Kind {
	k := gatewayv1.Kind(kind)
	return &k
}

func createGroup(group string) *gatewayv1.Group {
	g := gatewayv1.Group(group)
	return &g
}
