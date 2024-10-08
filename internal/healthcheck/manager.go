package healthcheck

import (
	"context"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
)

type Manager struct {
	informerFactory informers.SharedInformerFactory
	watchers        map[v1.ObjectReference]*Watcher
}

func NewManager(ctx context.Context, informerFactory informers.SharedInformerFactory) *Manager {
	return &Manager{
		informerFactory: informerFactory,
		watchers:        make(map[v1.ObjectReference]*Watcher),
	}
}

func (h *Manager) AddWatcher(ctx context.Context, watchTarget v1.ObjectReference, onHealthChange func(ctx context.Context, healthStatus bool) error) {
	watcher := newWatcher(ctx, watchTarget, h.informerFactory, onHealthChange)
	h.watchers[watchTarget] = watcher
	go watcher.Start(ctx)
}

func (h *Manager) RemoveWatcher(watchTarget v1.ObjectReference) {
	if watcher, ok := h.watchers[watchTarget]; ok {
		watcher.cancel()
	}
	delete(h.watchers, watchTarget)
}
