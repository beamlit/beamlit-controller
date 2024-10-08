package service

import (
	"context"
	"fmt"

	modelv1alpha1 "github.com/beamlit/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

// watchService watches the service for changes and calls the appropriate methods on the service controller
func (s *serviceController) watchService(ctx context.Context, serviceRef *modelv1alpha1.ServiceReference) error {
	serviceWatcher, err := s.kubeClient.CoreV1().Services(serviceRef.Namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", serviceRef.Name),
	})
	if err != nil {
		return err
	}

	defer serviceWatcher.Stop()

	for event := range serviceWatcher.ResultChan() {
		if event.Type == watch.Error {
			return fmt.Errorf("error watching service: %v", event.Object)
		}

		_, ok := event.Object.(*corev1.Service)
		if !ok {
			return fmt.Errorf("unexpected object type: %T", event.Object)
		}

		switch event.Type {
		case watch.Added, watch.Modified:
			s.Register(ctx, serviceRef)
		case watch.Deleted:
			s.Unregister(ctx, serviceRef)
			return nil // we don't need to watch the service anymore
		}
	}

	return nil
}
