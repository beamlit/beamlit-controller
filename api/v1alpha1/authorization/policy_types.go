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

package authorization

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PolicySpec defines the desired state of Policy on Beamlit
type PolicySpec struct {
	// DisplayName is the display name of the policy
	// +kubebuilder:validation:Optional
	DisplayName string `json:"displayName,omitempty"`

	// Type is the type of the policy
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=location;flavor
	Type PolicyType `json:"type"`

	Locations []PolicyLocation `json:"locations"`

	// Flavors is the list of flavors allowed by the policy
	// If not set, all flavors are allowed
	// +kubebuilder:validation:Optional
	Flavors []PolicyFlavor `json:"flavors"`
}

type PolicyType string

const (
	PolicyTypeLocation PolicyType = "location"
	PolicyTypeFlavor   PolicyType = "flavor"
)

type PolicySubTypeLocation string

const (
	PolicySubTypeLocationLocation  PolicySubTypeLocation = "location"
	PolicySubTypeLocationCountry   PolicySubTypeLocation = "country"
	PolicySubTypeLocationContinent PolicySubTypeLocation = "continent"
)

type PolicyLocation struct {
	// Type is the type of the location
	// +kubebuilder:validation:Required
	Type PolicySubTypeLocation `json:"type"`
	// Name is the name of the location
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

type PolicyFlavor struct {
	// Type is the type of the flavor
	// +kubebuilder:validation:Required
	Type string `json:"type"`
	// Name is the name of the flavor
	// +kubebuilder:validation:Required
	Name string `json:"name"`
}

// PolicyStatus defines the observed state of Policy
type PolicyStatus struct {
	// CreatedAtOnBeamlit is the time when the policy was created on Beamlit
	CreatedAtOnBeamlit metav1.Time `json:"createdAtOnBeamlit,omitempty"`
	// UpdatedAtOnBeamlit is the time when the policy was updated on Beamlit
	UpdatedAtOnBeamlit metav1.Time `json:"updatedAtOnBeamlit,omitempty"`
	// Workspace is the workspace of the policy
	Workspace string `json:"workspace"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Policy is the Schema for the policies API
type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicySpec   `json:"spec,omitempty"`
	Status PolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PolicyList contains a list of Policy
type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Policy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Policy{}, &PolicyList{})
}
