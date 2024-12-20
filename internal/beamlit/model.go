package beamlit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	beamlit "github.com/beamlit/toolkit/sdk"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// CreateOrUpdateModel creates or updates a model on Beamlit
// It returns the updated model on Beamlit
// It returns an error if the request fails, or if the response status is not 200 - OK
func (c *Client) CreateOrUpdateModel(ctx context.Context, model beamlit.Model) (*beamlit.Model, error) {
	if model.Metadata.Name == nil || model.Metadata.Environment == nil {
		return nil, fmt.Errorf("name and environment are required")
	}
	resp, err := c.client.GetModel(ctx, *model.Metadata.Name, &beamlit.GetModelParams{
		Environment: model.Metadata.Environment,
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		return c.createModel(ctx, model)
	}
	return c.updateModel(ctx, model)

}

func (c *Client) createModel(ctx context.Context, model beamlit.Model) (*beamlit.Model, error) {
	resp, err := c.client.CreateModel(ctx, model)
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
		return nil, fmt.Errorf("failed to update Model, status code: %d, body: %s", resp.StatusCode, string(body))
	}
	modelResp := &beamlit.Model{}
	if err := json.NewDecoder(resp.Body).Decode(modelResp); err != nil {
		return nil, err
	}
	return modelResp, nil
}

func (c *Client) updateModel(ctx context.Context, model beamlit.Model) (*beamlit.Model, error) {
	resp, err := c.client.UpdateModel(ctx, *model.Metadata.Name, model)
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
		return nil, fmt.Errorf("failed to update Model, status code: %d, body: %s", resp.StatusCode, string(body))
	}
	modelResp := &beamlit.Model{}
	if err := json.NewDecoder(resp.Body).Decode(modelResp); err != nil {
		return nil, err
	}
	return modelResp, nil
}

// DeleteModelDeployment deletes a model deployment on Beamlit
// It returns an error if the request fails, or if the response status is not 200 - OK
// It returns nil if the model deployment is not found
func (c *Client) DeleteModelDeployment(ctx context.Context, model string, environment string) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Deleting Model", "Model", model, "Environment", environment)
	resp, err := c.client.DeleteModel(ctx, model, &beamlit.DeleteModelParams{
		Environment: environment,
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Error(err, "failed to close response body")
		}
	}()
	logger.V(1).Info("Model deleted", "Status", resp.StatusCode)
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete Model, status code: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) NotifyOnModelOffloading(ctx context.Context, model string, environment string, offloading bool) error {
	resp, err := c.client.GetModel(ctx, model, &beamlit.GetModelParams{
		Environment: &environment,
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.FromContext(ctx).Error(err, "failed to close response body")
		}
	}()
	if resp.StatusCode >= 299 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to get ModelDeployment, status code: %d, body: %s", resp.StatusCode, string(body))
	}
	modelDeployment := &beamlit.Model{}
	if err := json.NewDecoder(resp.Body).Decode(modelDeployment); err != nil {
		return err
	}
	var labels beamlit.MetadataLabels
	if modelDeployment.Metadata.Labels != nil {
		labels = *modelDeployment.Metadata.Labels
	}
	labels["offloading"] = strconv.FormatBool(offloading)
	modelDeployment.Metadata.Labels = &labels
	_, err = c.client.UpdateModel(ctx, model, *modelDeployment)
	return err
}
