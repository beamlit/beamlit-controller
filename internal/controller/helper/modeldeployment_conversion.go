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

package helper

import (
	"context"
	"strconv"

	beamlit "github.com/beamlit/toolkit/sdk"
	"github.com/mitchellh/mapstructure"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	modelv1alpha1 "github.com/beamlit/beamlit-controller/api/v1alpha1/deployment"
)

// Convert converts a ModelDeployment to a Beamlit ModelDeployment
// It is used by the controller to convert the Kubernetes resource to the Beamlit API resource
func ToBeamlitModelDeployment(ctx context.Context, kubernetesClient client.Client, modelDeployment *modelv1alpha1.ModelDeployment) (beamlit.ModelDeployment, error) {
	logger := log.FromContext(ctx)
	logger.V(2).Info("Converting ModelDeployment to Beamlit ModelDeployment", "Name", modelDeployment.Name)

	labelOpts := []func(labels map[string]string){}
	if modelDeployment.Spec.OffloadingConfig != nil && modelDeployment.Spec.Enabled {
		labelOpts = append(labelOpts, withOffloadingEnabled)
	}

	beamlitModelDeployment := beamlit.ModelDeployment{
		Model:       &modelDeployment.Spec.Model,
		Labels:      toPtr(toBeamlitLabels(modelDeployment.Labels, labelOpts...)),
		Environment: &modelDeployment.Spec.Environment,
		Enabled:     toPtr(modelDeployment.Spec.Enabled),
		MetricPort:  toPtr(int(modelDeployment.Status.MetricPort)),
		ServingPort: toPtr(int(modelDeployment.Status.ServingPort)),
		Policies:    toBeamlitPolicies(modelDeployment.Spec.Policies),
	}

	if modelDeployment.Spec.ServerlessConfig != nil {
		var scaleUpMinimum *int
		if modelDeployment.Spec.ServerlessConfig.ScaleUpMinimum != nil {
			scaleUpMinimum = toPtr(int(*modelDeployment.Spec.ServerlessConfig.ScaleUpMinimum))
		}
		beamlitModelDeployment.ServerlessConfig = &beamlit.DeploymentServerlessConfig{
			MinNumReplicas:         toPtr(int(modelDeployment.Spec.ServerlessConfig.MinNumReplicas)),
			MaxNumReplicas:         toPtr(int(modelDeployment.Spec.ServerlessConfig.MaxNumReplicas)),
			Metric:                 modelDeployment.Spec.ServerlessConfig.Metric,
			Target:                 modelDeployment.Spec.ServerlessConfig.Target,
			ScaleDownDelay:         modelDeployment.Spec.ServerlessConfig.ScaleDownDelay,
			ScaleUpMinimum:         scaleUpMinimum,
			StableWindow:           modelDeployment.Spec.ServerlessConfig.StableWindow,
			LastPodRetentionPeriod: modelDeployment.Spec.ServerlessConfig.LastPodRetentionPeriod,
		}
	}

	logger.V(2).Info("Converting pod template to Beamlit pod template", "Name", modelDeployment.Name)
	template, err := retrievePodTemplate(ctx, kubernetesClient, modelDeployment.Spec.ModelSourceRef.Kind, modelDeployment.Spec.ModelSourceRef.Name, modelDeployment.Spec.ModelSourceRef.Namespace)
	if err != nil {
		logger.V(0).Error(err, "Failed to convert pod template to Beamlit pod template", "Name", modelDeployment.Name)
		return beamlit.ModelDeployment{}, err
	}
	logger.V(2).Info("Successfully converted pod template to Beamlit pod template", "Name", modelDeployment.Name)
	var podTemplate map[string]interface{} // TODO: Use a better type
	if err := mapstructure.Decode(template, &podTemplate); err != nil {
		logger.V(0).Error(err, "Failed to convert pod template to Beamlit pod template", "Name", modelDeployment.Name)
		return beamlit.ModelDeployment{}, err
	}
	beamlitModelDeployment.PodTemplate = &podTemplate
	logger.V(2).Info("Successfully converted ModelDeployment to Beamlit ModelDeployment", "Name", modelDeployment.Name)
	return beamlitModelDeployment, nil
}

func withOffloadingEnabled(labels map[string]string) {
	labels["offloading-enabled"] = strconv.FormatBool(true)
}

func toBeamlitLabels(labels map[string]string, opts ...func(labels map[string]string)) beamlit.Labels {
	beamlitLabels := make(beamlit.Labels)
	for key, value := range labels {
		beamlitLabels[key] = value
	}
	beamlitLabels["managed-by"] = "beamlit-operator"
	for _, opt := range opts {
		opt(beamlitLabels)
	}
	return beamlitLabels
}

func toBeamlitPolicies(policies []modelv1alpha1.PolicyRef) *[]string {
	beamlitPolicies := make([]string, len(policies))
	for i, policy := range policies {
		switch policy.RefType {
		case modelv1alpha1.PolicyRefTypeRemotePolicy:
			beamlitPolicies[i] = policy.Name
		case modelv1alpha1.PolicyRefTypeLocalPolicy:
			beamlitPolicies[i] = policy.Ref.Name
		}
	}
	return &beamlitPolicies
}
