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
	"fmt"

	"github.com/beamlit/operator/internal/informers"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

type k8sHealthWatcher struct {
	model           string
	watchTarget     v1.ObjectReference
	healthChan      chan<- HealthStatus
	errChan         chan<- informers.ErrWrapper
	informerFactory kubeinformers.SharedInformerFactory
	cancel          context.CancelFunc
}

func (h *k8sHealthWatcher) start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	h.cancel = cancel
	var informer cache.SharedIndexInformer
	switch h.watchTarget.Kind {
	case "Deployment":
		informer = h.informerFactory.Apps().V1().Deployments().Informer()
	case "StatefulSet":
		informer = h.informerFactory.Apps().V1().StatefulSets().Informer()
	case "DaemonSet":
		informer = h.informerFactory.Apps().V1().DaemonSets().Informer()
	case "ReplicaSet":
		informer = h.informerFactory.Apps().V1().ReplicaSets().Informer()
	default:
		h.errChan <- informers.ErrWrapper{
			ModelName: h.model,
			Err:       fmt.Errorf("unsupported resource kind: %s", h.watchTarget.Kind),
		}
		return
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			h.handleEvent(ctx, obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			h.handleEvent(ctx, newObj)
		},
		DeleteFunc: func(obj interface{}) {
			h.handleEvent(ctx, obj)
		},
	})

	h.informerFactory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		h.errChan <- informers.ErrWrapper{
			ModelName: h.model,
			Err:       fmt.Errorf("failed to sync informer cache"),
		}
		return
	}
	<-ctx.Done()
}

func (h *k8sHealthWatcher) handleEvent(ctx context.Context, obj interface{}) {
	switch h.watchTarget.Kind {
	case "Deployment":
		deployment, ok := obj.(*appsv1.Deployment)
		if !ok {
			return
		}
		if deployment.Namespace != h.watchTarget.Namespace || deployment.Name != h.watchTarget.Name {
			return
		}
		h.healthChan <- HealthStatus{
			ModelName: h.model,
			Healthy:   (deployment.Status.ReadyReplicas > 0),
		}
	case "StatefulSet":
		statefulSet, ok := obj.(*appsv1.StatefulSet)
		if !ok {
			return
		}
		if statefulSet.Namespace != h.watchTarget.Namespace || statefulSet.Name != h.watchTarget.Name {
			return
		}
		isHealthy := statefulSet.Status.ReadyReplicas > 0
		h.healthChan <- HealthStatus{
			ModelName: h.model,
			Healthy:   isHealthy,
		}
	case "DaemonSet":
		daemonSet, ok := obj.(*appsv1.DaemonSet)
		if !ok {
			return
		}
		if daemonSet.Namespace != h.watchTarget.Namespace || daemonSet.Name != h.watchTarget.Name {
			return
		}
		isHealthy := daemonSet.Status.NumberReady > 0
		h.healthChan <- HealthStatus{
			ModelName: h.model,
			Healthy:   isHealthy,
		}
	case "ReplicaSet":
		replicaSet, ok := obj.(*appsv1.ReplicaSet)
		if !ok {
			return
		}
		if replicaSet.Namespace != h.watchTarget.Namespace || replicaSet.Name != h.watchTarget.Name {
			return
		}
		isHealthy := replicaSet.Status.Replicas == replicaSet.Status.ReadyReplicas
		h.healthChan <- HealthStatus{
			ModelName: h.model,
			Healthy:   isHealthy,
		}
	default:
		h.errChan <- informers.ErrWrapper{
			ModelName: h.model,
			Err:       fmt.Errorf("unsupported resource kind: %s", h.watchTarget.Kind),
		}
	}
}
