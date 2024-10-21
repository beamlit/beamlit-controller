package beamlit

import (
	"bytes"
	"text/template"
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

func (m *ModelDeployment) String() string {
	tmpl := `
	Workspace: {{ .Workspace }}
	Model: {{ .Model }}
	Environment: {{ .Environment }}
	Labels: {{ .Labels }}
	ServerlessConfig: {{ .ServerlessConfig.String }}
	PodTemplate: {{ .PodTemplate }}
	Policies: {{ .Policies }}
	CreatedAt: {{ .CreatedAt }}
	UpdatedAt: {{ .UpdatedAt }}
	`

	buf := new(bytes.Buffer)
	if err := template.Must(template.New("model_deployment").Parse(tmpl)).Execute(buf, m); err != nil {
		return err.Error()
	}

	return buf.String()
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

func (s *ServerlessConfig) String() string {
	tmpl := `
	MinNumReplicas: {{ .MinNumReplicas }}
	MaxNumReplicas: {{ .MaxNumReplicas }}
	Metric: {{ .Metric }}
	Target: {{ .Target }}
	ScaleUpMinimum: {{ .ScaleUpMinimum }}
	ScaleDownDelay: {{ .ScaleDownDelay }}
	StableWindow: {{ .StableWindow }}
	LastPodRetentionPeriod: {{ .LastPodRetentionPeriod }}
	`

	buf := new(bytes.Buffer)
	if err := template.Must(template.New("serverless_config").Parse(tmpl)).Execute(buf, s); err != nil {
		return err.Error()
	}

	return buf.String()
}
