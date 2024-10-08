package healthcheck

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Watcher struct {
	watchTarget     v1.ObjectReference
	onHealthChange  func(ctx context.Context, healthStatus bool) error
	informerFactory informers.SharedInformerFactory
	cancel          context.CancelFunc
}

func newWatcher(ctx context.Context, watchTarget v1.ObjectReference, informerFactory informers.SharedInformerFactory, onHealthChange func(ctx context.Context, healthStatus bool) error) *Watcher {
	return &Watcher{
		watchTarget:     watchTarget,
		onHealthChange:  onHealthChange,
		informerFactory: informerFactory,
	}
}

func (h *Watcher) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	h.cancel = cancel
	logger := log.FromContext(ctx)
	var informer cache.SharedIndexInformer
	switch h.watchTarget.Kind {
	case "Deployment":
		informer = h.informerFactory.Apps().V1().Deployments().Informer()
	case "StatefulSet":
		informer = h.informerFactory.Apps().V1().StatefulSets().Informer()
	case "DaemonSet":
		informer = h.informerFactory.Apps().V1().DaemonSets().Informer()
	default:
		logger.Error(nil, "unsupported resource kind", "kind", h.watchTarget.Kind)
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
		logger.Error(nil, "failed to sync informer cache")
	}
	logger.Info("healthcheck watcher started")

	<-ctx.Done()
}

func (h *Watcher) handleEvent(ctx context.Context, obj interface{}) {
	switch h.watchTarget.Kind {
	case "Deployment":
		deployment, ok := obj.(*appsv1.Deployment)
		if !ok {
			return
		}
		if deployment.Namespace != h.watchTarget.Namespace || deployment.Name != h.watchTarget.Name {
			return
		}
		logger := log.FromContext(ctx)
		logger.Info("deployment event", "status", deployment.Status)
		isHealthy := deployment.Status.ReadyReplicas > 0
		logger.Info("deployment event", "isHealthy", isHealthy)
		h.onHealthChange(ctx, isHealthy)
	case "StatefulSet":
		statefulSet, ok := obj.(*appsv1.StatefulSet)
		if !ok {
			return
		}
		if statefulSet.Namespace != h.watchTarget.Namespace || statefulSet.Name != h.watchTarget.Name {
			return
		}
		logger := log.FromContext(ctx)
		logger.Info("statefulset event", "status", statefulSet.Status)
		isHealthy := statefulSet.Status.ReadyReplicas > 0
		logger.Info("statefulset event", "isHealthy", isHealthy)
		h.onHealthChange(ctx, isHealthy)
	case "DaemonSet":
		daemonSet, ok := obj.(*appsv1.DaemonSet)
		if !ok {
			return
		}
		if daemonSet.Namespace != h.watchTarget.Namespace || daemonSet.Name != h.watchTarget.Name {
			return
		}
		logger := log.FromContext(ctx)
		logger.Info("daemonset event", "status", daemonSet.Status)
		isHealthy := daemonSet.Status.NumberReady > 0
		logger.Info("daemonset event", "isHealthy", isHealthy)
		h.onHealthChange(ctx, isHealthy)
	}
}
