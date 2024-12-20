package beamlit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	beamlit "github.com/beamlit/toolkit/sdk"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func (c *Client) CreateOrUpdatePolicy(ctx context.Context, policy beamlit.Policy) (*beamlit.Policy, error) {
	if policy.Metadata.Name == nil {
		return nil, fmt.Errorf("policy name is required")
	}
	policyResp, err := c.client.GetPolicy(ctx, *policy.Metadata.Name)
	if err != nil {
		return nil, err
	}
	var resp *http.Response
	switch policyResp.StatusCode {
	case http.StatusNotFound:
		resp, err = c.client.CreatePolicy(ctx, policy)
	case http.StatusOK:
		resp, err = c.client.UpdatePolicy(ctx, *policy.Metadata.Name, policy)
	default:
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.FromContext(ctx).Error(err, "failed to close response body")
		}
	}()
	if resp.StatusCode >= 299 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to update Policy, status code: %d, body: %s", resp.StatusCode, string(body))
	}
	updatedPolicy := &beamlit.Policy{}
	if err := json.NewDecoder(resp.Body).Decode(updatedPolicy); err != nil {
		return nil, err
	}
	return updatedPolicy, nil
}

func (c *Client) DeletePolicy(ctx context.Context, name string) error {
	resp, err := c.client.DeletePolicy(ctx, name)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.FromContext(ctx).Error(err, "failed to close response body")
		}
	}()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete Policy, status code: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}
