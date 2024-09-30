package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Proxy struct {
	beamlitGatewayURL *url.URL
	listenPort        int
	proxy             *httputil.ReverseProxy
}

func NewProxy(beamlitGatewayAddress string, listenPort int) (*Proxy, error) {
	parsedURL, err := url.Parse(beamlitGatewayAddress)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(parsedURL)
	p := &Proxy{
		beamlitGatewayURL: parsedURL,
		listenPort:        listenPort,
		proxy:             proxy,
	}
	return p, nil
}

func (p *Proxy) Serve(ctx context.Context) error {
	loggedProxy := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.FromContext(ctx).Info("Proxying request to", "url", r.URL, "method", r.Method)
			h.ServeHTTP(w, r)
		})
	}
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", p.listenPort),
		Handler: loggedProxy(p.proxy),
	}
	return server.ListenAndServe()
}
