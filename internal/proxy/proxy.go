package proxy

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/beamlit/operator/internal/beamlit"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Proxy struct {
	beamlitGatewayURL *url.URL
	listenPort        int
	proxy             *httputil.ReverseProxy
	beamlitToken      *beamlit.BeamlitToken
}

func NewProxy(beamlitGatewayAddress string, listenPort int) (*Proxy, error) {
	parsedURL, err := url.Parse(beamlitGatewayAddress)
	if err != nil {
		return nil, err
	}

	beamlitToken, err := beamlit.NewBeamlitToken()
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(parsedURL)
	p := &Proxy{
		beamlitGatewayURL: parsedURL,
		listenPort:        listenPort,
		proxy:             proxy,
		beamlitToken:      beamlitToken,
	}
	return p, nil
}

func (p *Proxy) Serve(ctx context.Context) error {
	loggedProxy := func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, err := p.beamlitToken.Token(ctx)
			if err != nil {
				log.FromContext(ctx).Error(err, "Failed to get token")
				http.Error(w, "Failed to get token", http.StatusInternalServerError)
				return
			}
			r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
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
