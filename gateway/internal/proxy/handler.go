package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/beamlit/beamlit-controller/gateway/api/v1alpha1"
)

func (p *ProxyV1Alpha1) RewriteV1Alpha1(r *httputil.ProxyRequest) {
	routeName, ok := p.routesPerHost.Load(r.In.Host)
	if !ok {
		r.Out.Response.Status = http.StatusText(http.StatusNotFound)
		r.Out.Response.StatusCode = http.StatusNotFound
		r.Out.Response.Body = io.NopCloser(bytes.NewBufferString("not found"))
		return
	}
	slog.Info("route name", "routeName", routeName)
	route, err := p.persistenceV1Alpha1.GetRoute(r.In.Context(), routeName.(string))
	if err != nil {
		r.Out.Response.Status = http.StatusText(http.StatusNotFound)
		r.Out.Response.StatusCode = http.StatusNotFound
		r.Out.Response.Body = io.NopCloser(bytes.NewBufferString("not found"))
		return
	}
	slog.Info("route", "route", route)
	totalWeight := 0
	weightedBackends := []weightedBackend{}
	for _, backend := range route.Backends {
		slog.Info("backend", "backend", backend)
		totalWeight += backend.Weight
		weightedBackends = append(weightedBackends, weightedBackend{
			backend: backend,
			weight:  backend.Weight,
		})
	}
	if totalWeight == 0 {
		return
	}
	randomBackend := weightedRandomBackend(r.In.Context(), weightedBackends, totalWeight)
	if err := handleBackend(r.Out, randomBackend); err != nil {
		slog.Error("error handling the backend", "error", err)
	}
	r.SetXForwarded()
}

func handleBackend(r *http.Request, backend v1alpha1.Backend) error {
	if backend.Auth != nil {
		switch backend.Auth.Type {
		case v1alpha1.AuthTypeOAuth:
			token, err := handleOAuth(r.Context(), backend.Auth)
			if err != nil {
				return err
			}
			r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))
		}
	}
	r.URL.Scheme = backend.Scheme
	r.URL.Host = backend.Host

	r.Host = strings.Split(backend.Host, ":")[0]
	for key, value := range backend.HeadersToAdd {
		r.Header.Add(key, value)
	}
	if backend.PathPrefix != "" {
		r.URL.Path = backend.PathPrefix + r.URL.Path
	}
	return nil
}

func (p *ProxyV1Alpha1) ErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	slog.Info("error handler", "err", err)
	routeName, ok := p.backendHostToRoute.Load(r.Host)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte("not found")); err != nil {
			slog.Error("Error writing the response", "error", err)
		}
		return
	}
	for i := 0; i < 4; i++ {
		err := p.errorHandler(w, r, routeName.(string))
		if err == nil {
			return
		}
		time.Sleep(time.Duration(10^i) * time.Millisecond)
	}
	w.WriteHeader(http.StatusServiceUnavailable)
	if _, err := w.Write([]byte("service unavailable")); err != nil {
		slog.Error("Error writing the response", "error", err)
	}
}

func (p *ProxyV1Alpha1) errorHandler(w http.ResponseWriter, r *http.Request, routeName string) error {
	route, err := p.persistenceV1Alpha1.GetRoute(r.Context(), routeName)
	if err != nil {
		return fmt.Errorf("not found")
	}
	totalWeight := 0
	weightedBackends := []weightedBackend{}
	for _, backend := range route.Backends {
		if backend.Host == r.Host {
			continue
		}
		if backend.Weight == 0 {
			backend.Weight = 1
		}
		totalWeight += backend.Weight
		weightedBackends = append(weightedBackends, weightedBackend{
			backend: backend,
			weight:  backend.Weight,
		})
		r.URL.Path = removePathPrefix(r.URL.Path, backend.PathPrefix)
	}
	if totalWeight == 0 {
		return fmt.Errorf("no backends")
	}
	randomBackend := weightedRandomBackend(r.Context(), weightedBackends, totalWeight)
	if err := handleBackend(r, randomBackend); err != nil {
		return err
	}
	req, err := http.NewRequest(r.Method, r.URL.String(), r.Body)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}
	req.Header = r.Header
	req.Host = r.Host
	req.Proto = r.Proto
	req.ProtoMajor = r.ProtoMajor
	req.ProtoMinor = r.ProtoMinor
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error contacting backend: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("error closing response body", "error", err)
		}
	}()

	// Copy headers
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)

	// Stream the response body without any transfer encoding manipulation
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("error copying response: %w", err)
	}
	return nil
}

func (p *ProxyV1Alpha1) ModifyResponse(r *http.Response) error {
	if r.StatusCode < 399 {
		return nil
	}

	// Contact another backend
	routeName, ok := p.backendHostToRoute.Load(r.Request.Host)
	if !ok {
		r.StatusCode = http.StatusNotFound
		return nil
	}
	route, err := p.persistenceV1Alpha1.GetRoute(context.TODO(), routeName.(string))
	if err != nil {
		r.StatusCode = http.StatusNotFound
		return nil
	}
	totalWeight := 0
	weightedBackends := []weightedBackend{}
	for _, backend := range route.Backends {
		if backend.Host == r.Request.Host && backend.Scheme == r.Request.URL.Scheme {
			continue
		}
		if backend.Weight == 0 {
			backend.Weight = 1
		}
		totalWeight += backend.Weight
		weightedBackends = append(weightedBackends, weightedBackend{
			backend: backend,
			weight:  backend.Weight,
		})
		r.Request.URL.Path = removePathPrefix(r.Request.URL.Path, backend.PathPrefix)
	}
	slog.Info("weightedBackends", "weightedBackends", weightedBackends)
	if totalWeight == 0 {
		return nil
	}
	randomBackend := weightedRandomBackend(context.TODO(), weightedBackends, totalWeight)
	if err := handleBackend(r.Request, randomBackend); err != nil {
		return err
	}
	req, err := http.NewRequest(r.Request.Method, r.Request.URL.String(), r.Request.Body)
	if err != nil {
		return err
	}
	req.Header = r.Request.Header
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("error closing response body", "error", err)
		}
	}()

	// Copy headers and status
	for k, v := range resp.Header {
		r.Header[k] = v
	}
	r.StatusCode = resp.StatusCode
	r.Status = resp.Status
	r.Body = resp.Body

	return nil
}

func removePathPrefix(path, prefix string) string {
	if !strings.HasPrefix(path, "/") && strings.HasPrefix(prefix, "/") {
		prefix = prefix[1:]
	}
	if !strings.HasPrefix(path, prefix) {
		return path
	}
	return path[len(prefix):]
}
