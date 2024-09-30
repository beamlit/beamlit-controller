package offloading

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	v1 "k8s.io/api/core/v1"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

func (o *Offloader) keepInterceptorsInSync(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second): // TODO: make this configurable
			err := o.refreshInterceptors(ctx)
			if err != nil {
				return err
			}
		}
	}
}

func (o *Offloader) refreshInterceptors(ctx context.Context) error {
	service := &v1.Service{}
	err := o.kubeClient.Get(ctx, client.ObjectKey{
		Namespace: o.gatewayNamespace,
		Name:      o.interceptorServiceName,
	}, service)
	if err != nil {
		return err
	}

	service.Spec.Ports[0].Name = fmt.Sprintf("%d-%s-interceptor-%d-%d", rand.Intn(10), o.gatewayName, o.gatewayPort, o.gatewayPort) // Update port name to keep it fresh
	err = o.kubeClient.Update(ctx, service)
	if err != nil {
		return err
	}
	return nil
}
