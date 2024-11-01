package beamlit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	beamlit "github.com/tmp-moon/toolkit/sdk"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateOrUpdateModelDeployment creates or updates a model deployment on Beamlit
// It returns the updated model deployment on Beamlit
// It returns an error if the request fails, or if the response status is not 200 - OK
func (c *Client) CreateOrUpdateModelDeployment(ctx context.Context, modelDeployment beamlit.ModelDeployment) (*beamlit.ModelDeployment, error) {
	if modelDeployment.Model == nil || modelDeployment.Environment == nil {
		return nil, fmt.Errorf("model and environment are required")
	}
	resp, err := c.client.PutModelDeployment(ctx, *modelDeployment.Model, *modelDeployment.Environment, modelDeployment)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 299 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to update ModelDeployment, status code: %d, body: %s", resp.StatusCode, string(body))
	}
	updatedModelDeployment := &beamlit.ModelDeployment{}
	if err := json.NewDecoder(resp.Body).Decode(updatedModelDeployment); err != nil {
		return nil, err
	}
	return updatedModelDeployment, nil
}

// DeleteModelDeployment deletes a model deployment on Beamlit
// It returns an error if the request fails, or if the response status is not 200 - OK
// It returns nil if the model deployment is not found
func (c *Client) DeleteModelDeployment(ctx context.Context, model string, environment string) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Deleting ModelDeployment", "Model", model, "Environment", environment)
	resp, err := c.client.DeleteModelDeployment(ctx, model, environment)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	logger.V(1).Info("ModelDeployment deleted", "Status", resp.StatusCode)
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete ModelDeployment, status code: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) NotifyOnModelOffloading(ctx context.Context, model string, environment string, offloading bool) error {
	resp, err := c.client.GetModelDeployment(ctx, model, environment)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to get ModelDeployment, status code: %d, body: %s", resp.StatusCode, string(body))
	}
	modelDeployment := &beamlit.ModelDeployment{}
	if err := json.NewDecoder(resp.Body).Decode(modelDeployment); err != nil {
		return err
	}
	var labels beamlit.Labels
	if modelDeployment.Labels != nil {
		labels = *modelDeployment.Labels
	}
	labels["offloading"] = strconv.FormatBool(offloading)
	modelDeployment.Labels = &labels
	if _, err := c.client.PutModelDeployment(ctx, model, environment, *modelDeployment); err != nil {
		return err
	}
	return nil
}
