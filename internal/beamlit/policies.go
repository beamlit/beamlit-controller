package beamlit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	beamlit "github.com/tmp-moon/toolkit/sdk"
)

func (c *Client) CreateOrUpdatePolicy(ctx context.Context, policy beamlit.Policy) (*beamlit.Policy, error) {
	if policy.Name == nil {
		return nil, fmt.Errorf("policy name is required")
	}
	resp, err := c.client.PutPolicy(ctx, *policy.Name, policy)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
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
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete Policy, status code: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}
