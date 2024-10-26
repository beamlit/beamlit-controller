package beamlit

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

type Labels map[string]string

type PolicyName string

// PodTemplateSpec is a local type that wraps corev1.PodTemplateSpec
type PodTemplateSpec corev1.PodTemplateSpec

type ModelDeployment struct {
	Workspace   string `json:"workspace"`
	Model       string `json:"model"`
	Environment string `json:"environment"`
	Labels      Labels `json:"labels"`

	ServerlessConfig *ServerlessConfig `json:"serverless_config"`

	PodTemplate *PodTemplateSpec `json:"pod_template"`
	// ModelProviderRef     *ModelProviderRef     `json:"model_provider_ref"`
	// RuntimeConfiguration *RuntimeConfiguration `json:"runtime_configuration"`
	Policies []PolicyName `json:"policies"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ServerlessConfig struct {
	MinNumReplicas         *int32  `json:"min_num_replicas"`
	MaxNumReplicas         *int32  `json:"max_num_replicas"`
	Metric                 *string `json:"metric"`
	Target                 *string `json:"target"`
	ScaleUpMinimum         *int32  `json:"scale_up_minimum"`
	ScaleDownDelay         *string `json:"scale_down_delay"`
	StableWindow           *string `json:"stable_window"`
	LastPodRetentionPeriod *string `json:"last_pod_retention_period"`
}
