package api

import (
	"context"
	"net/http"
)

type Router struct {
	apiAddr string
	proxy   Proxy
}

func NewRouter(ctx context.Context, apiAddr string, proxy Proxy) *Router {
	return &Router{
		apiAddr: apiAddr,
		proxy:   proxy,
	}
}

func (r *Router) Run(ctx context.Context) error {
	mux := http.NewServeMux()
	RegisterRoutesV1Alpha1(mux, r.proxy)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return http.ListenAndServe(r.apiAddr, mux)
}
