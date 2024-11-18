package persistence

import (
	"context"

	"github.com/beamlit/beamlit-controller/gateway/api/v1alpha1"
)

type PersistenceV1Alpha1 interface {
	RegisterRoute(ctx context.Context, route v1alpha1.Route) (v1alpha1.Route, error)
	GetRoute(ctx context.Context, name string) (v1alpha1.Route, error)
	UpdateRoute(ctx context.Context, route v1alpha1.Route) (v1alpha1.Route, error)
	DeleteRoute(ctx context.Context, name string) (v1alpha1.Route, error)
}
