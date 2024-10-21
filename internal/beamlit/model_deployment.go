package beamlit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	modelPath            = "models"
	modelDeploymentsPath = "deployments"
)

// CreateOrUpdateModelDeployment creates or updates a model deployment on Beamlit
// It returns the updated model deployment on Beamlit
// It returns an error if the request fails, or if the response status is not 200 - OK
func (c *Client) CreateOrUpdateModelDeployment(ctx context.Context, modelDeployment *ModelDeployment) (*ModelDeployment, error) {
	body := new(bytes.Buffer)
	logger := log.FromContext(ctx)
	logger.V(1).Info("Creating or updating model deployment on Beamlit", "ModelDeployment", modelDeployment.String())
	if err := json.NewEncoder(body).Encode(modelDeployment); err != nil {
		return nil, err
	}
	resp, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("%s/%s/%s/%s", modelPath, modelDeployment.Model, modelDeploymentsPath, modelDeployment.Environment), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create or update model deployment: %s, %s", resp.Status, resp.Body)
	}

	updatedModelDeployment := &ModelDeployment{}
	if err := json.NewDecoder(resp.Body).Decode(updatedModelDeployment); err != nil {
		return nil, err
	}
	return updatedModelDeployment, nil
}

// DeleteModelDeployment deletes a model deployment on Beamlit
// It returns an error if the request fails, or if the response status is not 200 - OK
// It returns nil if the model deployment is not found
func (c *Client) DeleteModelDeployment(ctx context.Context, model string, environment string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("%s/%s/%s/%s", modelPath, model, modelDeploymentsPath, environment), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete model deployment: %s", resp.Status)
	}
	return nil
}
