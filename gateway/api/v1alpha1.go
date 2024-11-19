package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/beamlit/beamlit-controller/gateway/api/v1alpha1"
	"gopkg.in/yaml.v2"
)

const (
	APIV1Alpha1       = "/v1alpha1"
	APIV1Alpha1Routes = APIV1Alpha1 + "/routes"
	ContentTypeJSON   = "application/json"
	ContentTypeYAML   = "application/yaml"
)

var (
	ErrRouteNotFound = errors.New("route not found")
)

//nolint:gocyclo,dupl
func RegisterRoutesV1Alpha1(mux *http.ServeMux, proxy v1alpha1.Proxy) {
	mux.Handle(fmt.Sprintf("%s/", APIV1Alpha1Routes), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			name := strings.TrimPrefix(r.URL.Path, APIV1Alpha1Routes+"/")
			fmt.Println(name)
			route, err := proxy.GetRoute(r.Context(), name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			switch r.Header.Get("Accept") {
			case ContentTypeJSON:
				if err := json.NewEncoder(w).Encode(route); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case ContentTypeYAML:
				if err := yaml.NewEncoder(w).Encode(route); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			default:
				if err := json.NewEncoder(w).Encode(route); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		case http.MethodPut: // PUT /v1alpha1/routes
			var route v1alpha1.Route
			switch r.Header.Get("Content-Type") {
			case ContentTypeJSON:
				if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				routeInDb, err := proxy.UpdateRoute(r.Context(), route)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if err := json.NewEncoder(w).Encode(routeInDb); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case ContentTypeYAML:
				if err := yaml.NewDecoder(r.Body).Decode(&route); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				routeInDb, err := proxy.UpdateRoute(r.Context(), route)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if err := yaml.NewEncoder(w).Encode(routeInDb); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			default:
				http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
				return
			}
		case http.MethodDelete: // DELETE /v1alpha1/routes/<name>
			route, err := proxy.DeleteRoute(r.Context(), strings.TrimPrefix(r.URL.Path, APIV1Alpha1Routes+"/"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			switch r.Header.Get("Accept") {
			case ContentTypeJSON:
				if err := json.NewEncoder(w).Encode(route); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case ContentTypeYAML:
				if err := yaml.NewEncoder(w).Encode(route); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			default:
				if err := json.NewEncoder(w).Encode(route); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	mux.Handle(APIV1Alpha1Routes, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost: // POST /v1alpha1/routes
			var route v1alpha1.Route
			switch r.Header.Get("Content-Type") {
			case ContentTypeJSON:
				if err := json.NewDecoder(r.Body).Decode(&route); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				routeInDb, err := proxy.RegisterRoute(r.Context(), route)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if err := json.NewEncoder(w).Encode(routeInDb); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			case ContentTypeYAML:
				if err := yaml.NewDecoder(r.Body).Decode(&route); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				routeInDb, err := proxy.RegisterRoute(r.Context(), route)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if err := yaml.NewEncoder(w).Encode(routeInDb); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			default:
				http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
				return
			}
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}))
}
