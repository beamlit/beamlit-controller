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

	"github.com/beamlit/operator/api/v1alpha1"
	"github.com/beamlit/operator/internal/beamlit"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Convert converts a ModelDeployment to a Beamlit ModelDeployment
// It is used by the controller to convert the Kubernetes resource to the Beamlit API resource
func ToBeamlitModelDeployment(ctx context.Context, kubernetesClient client.Client, modelDeployment *v1alpha1.ModelDeployment) (beamlit.ModelDeployment, error) {
	logger := log.FromContext(ctx)
	logger.V(2).Info("Converting ModelDeployment to Beamlit ModelDeployment", "Name", modelDeployment.Name)
	beamlitModelDeployment := beamlit.ModelDeployment{
		Model:       modelDeployment.Name,
		Labels:      toBeamlitLabels(modelDeployment),
		Environment: modelDeployment.Spec.Environment,
		ServerlessConfig: &beamlit.ServerlessConfig{
			MinNumReplicas:         toPtr(modelDeployment.Spec.ServerlessConfig.MinNumReplicas),
			MaxNumReplicas:         toPtr(modelDeployment.Spec.ServerlessConfig.MaxNumReplicas),
			Metric:                 modelDeployment.Spec.ServerlessConfig.Metric,
			Target:                 modelDeployment.Spec.ServerlessConfig.Target,
			ScaleUpMinimum:         modelDeployment.Spec.ServerlessConfig.ScaleUpMinimum,
			ScaleDownDelay:         modelDeployment.Spec.ServerlessConfig.ScaleDownDelay,
			StableWindow:           modelDeployment.Spec.ServerlessConfig.StableWindow,
			LastPodRetentionPeriod: modelDeployment.Spec.ServerlessConfig.LastPodRetentionPeriod,
		},
		Policies: toBeamlitPolicies(modelDeployment.Spec.Policies),
	}

	logger.V(2).Info("Converting pod template to Beamlit pod template", "Name", modelDeployment.Name)
	template, err := retrievePodTemplate(ctx, kubernetesClient, modelDeployment.Spec.ModelSourceRef.Kind, modelDeployment.Spec.ModelSourceRef.Name, modelDeployment.Spec.ModelSourceRef.Namespace)
	if err != nil {
		logger.V(0).Error(err, "Failed to convert pod template to Beamlit pod template", "Name", modelDeployment.Name)
		return beamlit.ModelDeployment{}, err
	}
	logger.V(2).Info("Successfully converted pod template to Beamlit pod template", "Name", modelDeployment.Name)
	beamlitModelDeployment.PodTemplate = toPtr(beamlit.PodTemplateSpec(template))
	logger.V(2).Info("Successfully converted ModelDeployment to Beamlit ModelDeployment", "Name", modelDeployment.Name)
	return beamlitModelDeployment, nil
}

func toBeamlitPolicies(policies []string) []beamlit.PolicyName {
	if len(policies) == 0 {
		return nil
	}
	beamlitPolicies := make([]beamlit.PolicyName, len(policies))
	for i, policy := range policies {
		beamlitPolicies[i] = beamlit.PolicyName(policy)
	}
	return beamlitPolicies
}

func toBeamlitLabels(model *v1alpha1.ModelDeployment) beamlit.Labels {
	labels := make(beamlit.Labels)
	for key, value := range model.ObjectMeta.Labels {
		labels[key] = value
	}
	labels["managed-by"] = "beamlit-operator"
	if model.Spec.OffloadingConfig != nil {
		labels["offloading-enabled"] = strconv.FormatBool(!model.Spec.OffloadingConfig.Disabled)
	}
	return labels
}
