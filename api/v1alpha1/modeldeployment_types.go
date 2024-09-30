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
	// DisplayName is the name of the model deployment displayed on Beamlit UI
	// Note: The DisplayName is not used for any logic and is only for display purposes. The name of the model deployment is the name of the ModelDeployment object.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=""
	DisplayName string `json:"displayName,omitempty"`

	// EnabledLocations is the list of locations where the model can be deployed
	// If not specified, the model deployment can be deployed in all locations
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	EnabledLocations []Location `json:"enabledLocations,omitempty"`

	// SupportedGPUTypes is the list of GPU types supported by the model deployment
	// If not specified, the model deployment can be deployed on all GPU types
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={A100_40GB, T4}
	SupportedGPUTypes []string `json:"supportedGPUTypes,omitempty"`

	// Environment is the environment attached to the model deployment
	// If not specified, the model deployment will be deployed in the "prod" environment
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="prod"
	Environment string `json:"environment,omitempty"`

	// MinNumReplicasPerLocation is the minimum number of replicas per location
	// If not specified, the model deployment will be deployed with 0 replicas
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=0
	MinNumReplicasPerLocation int32 `json:"minNumReplicasPerLocation,omitempty"`

	// MaxNumReplicasPerLocation is the maximum number of replicas per location
	// If not specified, the model deployment will be deployed with no limit
	// 0 means no limit
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=0
	MaxNumReplicasPerLocation int32 `json:"maxNumReplicasPerLocation,omitempty"`

	// ModelSourceRef is the reference to the model source
	// This is either a Deployment, StatefulSet... (anything that is a template for a pod)
	// +kubebuilder:validation:Required
	ModelSourceRef corev1.ObjectReference `json:"modelSourceRef"`

	// ScalingConfig is the scaling configuration for the model deployment
	// If not specified, the model deployment will be scaled automatically on Beamlit based on the number of requests
	// You can specify either HPA or metrics, but not both.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	ScalingConfig *ScalingConfig `json:"scalingConfig,omitempty"`

	// OffloadingConfig is the offloading configuration for the model deployment
	// If not specified, the model deployment will not be offloaded
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	OffloadingConfig *OffloadingConfig `json:"offloadingConfig,omitempty"`
}

type Location struct {
	// Location is the location of the model deployment
	// +kubebuilder:validation:Required
	Location string `json:"location"`

	// MinNumReplicas is the minimum number of replicas for the location
	// Note: it supersedes the MinNumReplicas in the ModelDeploymentSpec
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=0
	MinNumReplicas int32 `json:"minNumReplicas,omitempty"`

	// MaxNumReplicas is the maximum number of replicas for the location
	// Note: it supersedes the MaxNumReplicas in the ModelDeploymentSpec
	// 0 means no limit
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=0
	MaxNumReplicas int32 `json:"maxNumReplicas,omitempty"`
}

type ScalingConfig struct {
	// MetricPort is the port to get the metric for the model deployment in a prometheus-compatible format (https://prometheus.io/docs/instrumenting/exposition_formats/)
	// +kubebuilder:validation:Optional
	MetricPort int32 `json:"metricPort,omitempty"`

	// MetricPath is the path to get the metric for the model deployment in a prometheus-compatible format
	// +kubebuilder:validation:Optional
	MetricPath string `json:"metricPath,omitempty"`

	// Behavior is the behavior of the autoscaler
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	Behavior *autoscalingv2.HorizontalPodAutoscalerBehavior `json:"behavior,omitempty"`

	// Metrics is the list of metrics used for autoscaling
	// +kubebuilder:validation:Optional
	// +kubebuilder:default={}
	Metrics []autoscalingv2.MetricSpec `json:"metrics,omitempty"`

	// HPARef is the reference to the current HorizontalPodAutoscaler
	// +kubebuilder:validation:Optional
	HPARef *corev1.ObjectReference `json:"hpaRef,omitempty"`
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
	CurrentReplicas   int32              `json:"currentReplicas,omitempty"`
	DesiredReplicas   int32              `json:"desiredReplicas,omitempty"`
	OffloadingStatus  *OffloadingStatus  `json:"offloadingStatus,omitempty"`
	ScalingStatus     *ScalingStatus     `json:"scalingStatus,omitempty"`
}

type ScalingStatus struct {
	Status string                  `json:"status,omitempty"`
	HPARef *corev1.ObjectReference `json:"hpaRef,omitempty"`
}

type OffloadingStatus struct {
	Status          string                     `json:"status,omitempty"`
	LocalServiceRef *corev1.ObjectReference    `json:"localServiceRef,omitempty"`
	Behavior        *OffloadingBehavior        `json:"behavior,omitempty"`
	Metrics         []autoscalingv2.MetricSpec `json:"metrics,omitempty"`
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
