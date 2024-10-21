/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelDeploymentSpec defines the desired state of ModelDeployment
type ModelDeploymentSpec struct {
	// Model is the name of the base model
	// +kubebuilder:validation:Required
	Model string `json:"model"`

	// ModelSourceRef is the reference to the model source
	// This is either a Deployment, StatefulSet... (anything that is a template for a pod)
	// +kubebuilder:validation:Required
	ModelSourceRef corev1.ObjectReference `json:"modelSourceRef"`

	// Environment is the environment attached to the model deployment
	// If not specified, the model deployment will be deployed in the "prod" environment
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="production"
	Environment string `json:"environment,omitempty"`

	// Policies is the list of policies to apply to the model deployment
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	Policies []string `json:"policies,omitempty"`

	// ServerlessConfig is the serverless configuration for the model deployment
	// If not specified, the model deployment will be deployed with a default serverless configuration
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	ServerlessConfig *ServerlessConfig `json:"serverlessConfig,omitempty"`

	// OffloadingConfig is the offloading configuration for the model deployment
	// If not specified, the model deployment will not be offloaded
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	OffloadingConfig *OffloadingConfig `json:"offloadingConfig,omitempty"`
}

type ServerlessConfig struct {
	// MinNumReplicas is the minimum number of replicas
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=0
	MinNumReplicas int32 `json:"minNumReplicas,omitempty"`

	// MaxNumReplicas is the maximum number of replicas
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=10
	MaxNumReplicas int32 `json:"maxNumReplicas,omitempty"`

	// Metric is the metric used for scaling
	// +kubebuilder:validation:Optional
	Metric *string `json:"metric,omitempty"`

	// Target is the target value for the metric
	// +kubebuilder:validation:Optional
	Target *string `json:"target,omitempty"`

	// ScaleUpMinimum is the minimum number of replicas to scale up
	// +kubebuilder:validation:Minimum=2
	// +kubebuilder:validation:Optional
	ScaleUpMinimum *int32 `json:"scaleUpMinimum,omitempty"`

	// ScaleDownDelay is the delay between scaling down
	// +kubebuilder:validation:Optional
	ScaleDownDelay *string `json:"scaleDownDelay,omitempty"`

	// StableWindow is the window of time to consider the number of replicas stable
	// +kubebuilder:validation:Optional
	StableWindow *string `json:"stableWindow,omitempty"`

	// LastPodRetentionPeriod is the retention period for the last pod
	// +kubebuilder:validation:Optional
	LastPodRetentionPeriod *string `json:"lastPodRetentionPeriod,omitempty"`
}

type OffloadingConfig struct {
	// Disabled is the flag to disable offloading
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	Disabled bool `json:"disabled,omitempty"`

	// LocalServiceRef is the reference to the local service exposing the model
	// If not specified, a local service will be created
	// +kubebuilder:validation:Optional
	LocalServiceRef *ServiceReference `json:"localServiceRef,omitempty"`

	// RemoteServiceRef is the reference to the remote service exposing the model
	// If not specified, we will use Beamlit's default service
	// +kubebuilder:validation:Optional
	RemoteServiceRef *ServiceReference `json:"remoteServiceRef,omitempty"`

	// Metrics is the list of metrics used for offloading
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	Metrics []autoscalingv2.MetricSpec `json:"metrics,omitempty"`

	// Behavior is the behavior of the offloading
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	Behavior *OffloadingBehavior `json:"behavior,omitempty"`
}

type ServiceReference struct {
	corev1.ObjectReference `json:",inline"`
	TargetPort             int32 `json:"targetPort,omitempty"`
}

type OffloadingBehavior struct {
	// Percentage is the percentage of the requests that will be offloaded
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=100
	Percentage int32 `json:"percentage,omitempty"`
}

// ModelDeploymentStatus defines the observed state of ModelDeployment
type ModelDeploymentStatus struct {
	Conditions        []metav1.Condition `json:"conditions,omitempty"`
	AvailableReplicas int32              `json:"availableReplicas,omitempty"`
	// OffloadingStatus is the status of the offloading
	// True if the model deployment is offloaded
	OffloadingStatus bool `json:"offloadingStatus,omitempty"`
	// Workspace is the workspace of the model deployment
	Workspace          string      `json:"workspace,omitempty"`
	CreatedAtOnBeamlit metav1.Time `json:"createdAtOnBeamlit,omitempty"`
	UpdatedAtOnBeamlit metav1.Time `json:"updatedAtOnBeamlit,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:subresource:scale:specpath=.spec.minNumReplicasPerLocation,statuspath=.status.availableReplicas,selectorpath=.status.conditions

// ModelDeployment is the Schema for the modeldeployments API
type ModelDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ModelDeploymentSpec   `json:"spec,omitempty"`
	Status ModelDeploymentStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ModelDeploymentList contains a list of ModelDeployment
type ModelDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ModelDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ModelDeployment{}, &ModelDeploymentList{})
}
