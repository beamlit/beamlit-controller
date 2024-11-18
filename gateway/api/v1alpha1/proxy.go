package v1alpha1

import "context"

type Proxy interface {
	RegisterRoute(ctx context.Context, route Route) (Route, error)
	GetRoute(ctx context.Context, name string) (Route, error)
	UpdateRoute(ctx context.Context, route Route) (Route, error)
	DeleteRoute(ctx context.Context, name string) (Route, error)
}
