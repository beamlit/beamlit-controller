//nolint:errcheck
package clientset

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/beamlit/beamlit-controller/gateway/api/v1alpha1"
)

const (
	v1Alpha1RoutePath = "v1alpha1/routes"
)

type V1Alpha1Client interface {
	GetRoute(ctx context.Context, name string) (*v1alpha1.Route, error)
	RegisterRoute(ctx context.Context, route v1alpha1.Route) (*v1alpha1.Route, error)
	DeleteRoute(ctx context.Context, name string) (*v1alpha1.Route, error)
	UpdateRoute(ctx context.Context, route v1alpha1.Route) (*v1alpha1.Route, error)
}

func (c *ClientSet) GetRoute(ctx context.Context, name string) (*v1alpha1.Route, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("http://%s/%s/%s", c.apiAddr, v1Alpha1RoutePath, name),
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var route v1alpha1.Route
	if err := json.NewDecoder(resp.Body).Decode(&route); err != nil {
		return nil, err
	}
	return &route, nil
}

func (c *ClientSet) RegisterRoute(ctx context.Context, route v1alpha1.Route) (*v1alpha1.Route, error) {
	body, err := json.Marshal(route)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		fmt.Sprintf("http://%s/%s", c.apiAddr, v1Alpha1RoutePath),
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var registeredRoute v1alpha1.Route
	if err := json.NewDecoder(resp.Body).Decode(&registeredRoute); err != nil {
		return nil, err
	}
	return &registeredRoute, nil
}

func (c *ClientSet) DeleteRoute(ctx context.Context, name string) (*v1alpha1.Route, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("http://%s/%s/%s", c.apiAddr, v1Alpha1RoutePath, name),
		nil,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var deletedRoute v1alpha1.Route
	if err := json.NewDecoder(resp.Body).Decode(&deletedRoute); err != nil {
		return nil, err
	}
	return &deletedRoute, nil
}

func (c *ClientSet) UpdateRoute(ctx context.Context, route v1alpha1.Route) (*v1alpha1.Route, error) {
	body, err := json.Marshal(route)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPut,
		fmt.Sprintf("http://%s/%s/%s", c.apiAddr, v1Alpha1RoutePath, route.Name),
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var updatedRoute v1alpha1.Route
	if err := json.NewDecoder(resp.Body).Decode(&updatedRoute); err != nil {
		return nil, err
	}
	return &updatedRoute, nil
}
