package proxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"

	"github.com/beamlit/beamlit-controller/gateway/api"
	"github.com/beamlit/beamlit-controller/gateway/api/v1alpha1"
	"github.com/beamlit/beamlit-controller/gateway/internal/persistence"
)

type ProxyV1Alpha1 struct {
	persistenceV1Alpha1 persistence.PersistenceV1Alpha1
	proxy               *httputil.ReverseProxy
	routesPerHost       sync.Map // key: host, value: []route name
	backendHostToRoute  sync.Map // key: host, value: route name
}

func New() api.Proxy {
	v1alpha1Proxy := &ProxyV1Alpha1{
		routesPerHost:       sync.Map{},
		backendHostToRoute:  sync.Map{},
		persistenceV1Alpha1: persistence.NewInMemV1Alpha1(),
	}
	v1alpha1Proxy.proxy = &httputil.ReverseProxy{
		Rewrite:        v1alpha1Proxy.RewriteV1Alpha1,
		ModifyResponse: v1alpha1Proxy.ModifyResponse,
		ErrorHandler:   v1alpha1Proxy.ErrorHandler,
	}
	return v1alpha1Proxy
}

func (p *ProxyV1Alpha1) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.proxy.ServeHTTP(w, r)
}

func (p *ProxyV1Alpha1) RegisterRoute(ctx context.Context, route v1alpha1.Route) (v1alpha1.Route, error) {
	for _, hostname := range route.Hostnames {
		p.routesPerHost.Store(hostname, route.Name)
	}
	for _, b := range route.Backends {
		p.backendHostToRoute.Store(strings.Split(b.Host, ":")[0], route.Name)
	}
	return p.persistenceV1Alpha1.RegisterRoute(ctx, route)
}

func (p *ProxyV1Alpha1) GetRoute(ctx context.Context, name string) (v1alpha1.Route, error) {
	return p.persistenceV1Alpha1.GetRoute(ctx, name)
}

func (p *ProxyV1Alpha1) UpdateRoute(ctx context.Context, route v1alpha1.Route) (v1alpha1.Route, error) {
	for _, hostname := range route.Hostnames {
		p.routesPerHost.Store(hostname, route.Name)
	}
	for _, b := range route.Backends {
		p.backendHostToRoute.Store(strings.Split(b.Host, ":")[0], route.Name)
	}
	return p.persistenceV1Alpha1.UpdateRoute(ctx, route)
}

func (p *ProxyV1Alpha1) DeleteRoute(ctx context.Context, name string) (v1alpha1.Route, error) {
	return p.persistenceV1Alpha1.DeleteRoute(ctx, name)
}
