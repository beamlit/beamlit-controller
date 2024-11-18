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

	"github.com/beamlit/beamlit-controller/internal/informers"
	v1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// k8sHealthInformer is a health informer that uses the Kubernetes API to check the health of the model deployment.
// When a model has no available replicas, it is considered unhealthy.
// Not suitable for serverless environments.
type k8sHealthInformer struct {
	healthChan chan HealthStatus
	errChan    chan informers.ErrWrapper
	clientset  kubernetes.Interface
	watchers   map[string]*k8sHealthWatcher // model: watcher
}

func newK8SHealthInformer(ctx context.Context, restConfig *rest.Config) (HealthInformer, error) {
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}
	return &k8sHealthInformer{
		healthChan: make(chan HealthStatus),
		clientset:  clientset,
		watchers:   make(map[string]*k8sHealthWatcher),
		errChan:    make(chan informers.ErrWrapper),
	}, nil
}

func (k *k8sHealthInformer) Start(ctx context.Context) <-chan HealthStatus {
	logger := log.FromContext(ctx)
	go func() {
		for {
			select {
			case <-ctx.Done():
				k.Stop()
				return
			case err := <-k.errChan:
				logger.Error(err.Err, "error in health informer", "modelName", err.ModelName)
				k.removeWatcher(err.ModelName)
			}
		}
	}()
	return k.healthChan
}

func (k *k8sHealthInformer) Register(ctx context.Context, model string, resource v1.ObjectReference) {
	if _, ok := k.watchers[model]; ok {
		k.removeWatcher(model)
	}
	k.watchers[model] = &k8sHealthWatcher{
		model:           model,
		watchTarget:     resource,
		healthChan:      k.healthChan,
		errChan:         k.errChan,
		informerFactory: kubeinformers.NewSharedInformerFactoryWithOptions(k.clientset, 0, kubeinformers.WithNamespace(resource.Namespace)),
	}
	go k.watchers[model].start(ctx)
}

func (k *k8sHealthInformer) Unregister(ctx context.Context, model string) {
	if watcher, ok := k.watchers[model]; ok {
		watcher.cancel()
		delete(k.watchers, model)
	}
}

func (k *k8sHealthInformer) Stop() {
	for _, watcher := range k.watchers {
		watcher.cancel()
		delete(k.watchers, watcher.model)
	}
	close(k.healthChan)
}

func (h *k8sHealthInformer) removeWatcher(model string) {
	if watcher, ok := h.watchers[model]; ok {
		watcher.cancel()
		delete(h.watchers, model)
	}
}
