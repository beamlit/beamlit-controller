package persistence

import (
	"context"
	"sync"

	"github.com/beamlit/beamlit-controller/gateway/api/v1alpha1"
)

type InMem struct {
	sync.Map // key: route name, value: route
}

func NewInMemV1Alpha1() PersistenceV1Alpha1 {
	return &InMem{}
}

func (i *InMem) RegisterRoute(ctx context.Context, route v1alpha1.Route) (v1alpha1.Route, error) {
	i.Store(route.Name, route)
	return route, nil
}

func (i *InMem) GetRoute(ctx context.Context, name string) (v1alpha1.Route, error) {
	route, ok := i.Load(name)
	if !ok {
		return v1alpha1.Route{}, v1alpha1.ErrRouteNotFound
	}
	return route.(v1alpha1.Route), nil
}

func (i *InMem) UpdateRoute(ctx context.Context, route v1alpha1.Route) (v1alpha1.Route, error) {
	i.Store(route.Name, route)
	return route, nil
}

func (i *InMem) DeleteRoute(ctx context.Context, name string) (v1alpha1.Route, error) {
	route, ok := i.Load(name)
	if !ok {
		return v1alpha1.Route{}, v1alpha1.ErrRouteNotFound
	}
	i.Delete(name)
	return route.(v1alpha1.Route), nil
}
