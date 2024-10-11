package beamlit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
)

const (
	modelDeploymentsPath = "model_deployments"
)

type ModelDeployment struct {
	Name              string                 `json:"name"`
	DisplayName       *string                `json:"display_name"`
	EnabledLocations  []Location             `json:"enabled_locations"`
	SupportedGPUTypes []string               `json:"supported_gpu_types"`
	Environment       string                 `json:"environment"`
	ScalingConfig     *ScalingConfig         `json:"scaling_config"`
	PodTemplate       corev1.PodTemplateSpec `json:"pod_template"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

type ScalingConfig struct {
	MinNumReplicas          *int                                       `json:"min_num_replicas"`
	MaxNumReplicas          *int                                       `json:"max_num_replicas"`
	MetricPort              *int                                       `json:"metric_port"`
	MetricPath              *string                                    `json:"metric_path"`
	HorizontalPodAutoscaler *autoscalingv2.HorizontalPodAutoscalerSpec `json:"horizontal_pod_autoscaler"`
}

type Location struct {
	Location       string `json:"location"`
	MinNumReplicas *int   `json:"min_num_replicas"`
	MaxNumReplicas *int   `json:"max_num_replicas"`
}

// CreateOrUpdateModelDeployment creates or updates a model deployment on Beamlit
// It returns the updated model deployment on Beamlit
// It returns an error if the request fails, or if the response status is not 200 - OK
func (c *Client) CreateOrUpdateModelDeployment(ctx context.Context, modelDeployment *ModelDeployment) (*ModelDeployment, error) {
	body := new(bytes.Buffer)
	if err := json.NewEncoder(body).Encode(modelDeployment); err != nil {
		return nil, err
	}
	resp, err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("%s/%s", modelDeploymentsPath, modelDeployment.Name), body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create or update model deployment: %s", resp.Status)
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
func (c *Client) DeleteModelDeployment(ctx context.Context, name string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("%s/%s", modelDeploymentsPath, name), nil)
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
