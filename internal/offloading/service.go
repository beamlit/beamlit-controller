package offloading

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

func (o *Offloader) watchServices(ctx context.Context) {
	informer := cache.NewSharedIndexInformer(
		o.watchList,
		&v1.Service{},
		time.Second*0,
		cache.Indexers{},
	)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// Do nothing
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			o.handleServiceUpdate(ctx, oldObj.(*v1.Service), newObj.(*v1.Service))
		},
		DeleteFunc: func(obj interface{}) {
			o.handleServiceDelete(ctx, obj.(*v1.Service))
		},
	})

	informer.Run(ctx.Done())
}

func (o *Offloader) handleServiceUpdate(ctx context.Context, oldObj, newObj *v1.Service) {
	if oldObj.Spec.ClusterIP != newObj.Spec.ClusterIP {
		o.updateInterceptorService(ctx, oldObj.Spec.ClusterIP, newObj.Spec.ClusterIP)
	}
}

func (o *Offloader) handleServiceDelete(ctx context.Context, service *v1.Service) {
	interceptorService := &v1.Service{}
	err := o.kubeClient.Get(ctx, client.ObjectKey{
		Namespace: o.gatewayNamespace,
		Name:      o.interceptorServiceName,
	}, interceptorService)
	if err != nil {
		return
	}
	for i, externalIP := range interceptorService.Spec.ExternalIPs {
		if externalIP == service.Spec.ClusterIP {
			interceptorService.Spec.ExternalIPs = append(interceptorService.Spec.ExternalIPs[:i], interceptorService.Spec.ExternalIPs[i+1:]...)
			o.kubeClient.Update(ctx, interceptorService)
			break
		}
	}
}

func (o *Offloader) updateInterceptorService(ctx context.Context, oldClusterIP, newClusterIP string) {
	interceptorService := &v1.Service{}
	err := o.kubeClient.Get(ctx, client.ObjectKey{
		Namespace: o.gatewayNamespace,
		Name:      o.interceptorServiceName,
	}, interceptorService)
	if err != nil {
		return
	}
	for i, externalIP := range interceptorService.Spec.ExternalIPs {
		if externalIP == oldClusterIP {
			interceptorService.Spec.ExternalIPs[i] = newClusterIP
			break
		}
	}
	o.kubeClient.Update(ctx, interceptorService)
}
