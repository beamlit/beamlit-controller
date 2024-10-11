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

	"github.com/beamlit/operator/api/v1alpha1"
	"github.com/beamlit/operator/internal/beamlit"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
)

// Convert converts a ModelDeployment to a Beamlit ModelDeployment
// It is used by the controller to convert the Kubernetes resource to the Beamlit API resource
func ToBeamlitModelDeployment(ctx context.Context, kubernetesClient client.Client, modelDeployment *v1alpha1.ModelDeployment) (beamlit.ModelDeployment, error) {
	logger := log.FromContext(ctx)
	logger.V(2).Info("Converting ModelDeployment to Beamlit ModelDeployment", "Name", modelDeployment.Name)
	beamlitModelDeployment := beamlit.ModelDeployment{
		Name:              modelDeployment.Name,
		DisplayName:       &modelDeployment.Spec.DisplayName,
		EnabledLocations:  toBeamlitLocations(ctx, modelDeployment.Spec.EnabledLocations),
		SupportedGPUTypes: modelDeployment.Spec.SupportedGPUTypes,
		Environment:       modelDeployment.Spec.Environment,
		ScalingConfig: &beamlit.ScalingConfig{
			MinNumReplicas: toIntPtr(int(modelDeployment.Spec.MinNumReplicasPerLocation)),
			MaxNumReplicas: toIntPtr(int(modelDeployment.Spec.MaxNumReplicasPerLocation)),
			MetricPort:     toIntPtr(int(modelDeployment.Spec.ScalingConfig.MetricPort)),
			MetricPath:     &modelDeployment.Spec.ScalingConfig.MetricPath,
		},
	}

	logger.V(2).Info("Converting pod template to Beamlit pod template", "Name", modelDeployment.Name)
	template, err := retrievePodTemplate(ctx, kubernetesClient, modelDeployment.Spec.ModelSourceRef.Kind, modelDeployment.Spec.ModelSourceRef.Name, modelDeployment.Spec.ModelSourceRef.Namespace)
	if err != nil {
		logger.V(0).Error(err, "Failed to convert pod template to Beamlit pod template", "Name", modelDeployment.Name)
		return beamlit.ModelDeployment{}, err
	}
	logger.V(2).Info("Successfully converted pod template to Beamlit pod template", "Name", modelDeployment.Name)
	beamlitModelDeployment.PodTemplate = template
	logger.V(2).Info("Converting HPA config to Beamlit HPA config", "Name", modelDeployment.Name)
	hpaSpec, err := toBeamlitHPAConfig(ctx, kubernetesClient, modelDeployment.Spec.ScalingConfig)
	if err != nil {
		logger.V(0).Error(err, "Failed to convert HPA config to Beamlit HPA config", "Name", modelDeployment.Name)
		return beamlit.ModelDeployment{}, err
	}
	logger.V(2).Info("Successfully converted HPA config to Beamlit HPA config", "Name", modelDeployment.Name)
	beamlitModelDeployment.ScalingConfig.HorizontalPodAutoscaler = &hpaSpec
	logger.V(2).Info("Successfully converted ModelDeployment to Beamlit ModelDeployment", "Name", modelDeployment.Name)
	return beamlitModelDeployment, nil
}

func toBeamlitLocations(ctx context.Context, locations []v1alpha1.Location) []beamlit.Location {
	logger := log.FromContext(ctx)
	logger.V(2).Info("Converting locations to Beamlit locations", "Locations", locations)
	beamlitLocations := make([]beamlit.Location, 0, len(locations))
	for _, location := range locations {
		beamlitLocations = append(beamlitLocations, beamlit.Location{
			Location:       location.Location,
			MinNumReplicas: toIntPtr(int(location.MinNumReplicas)),
			MaxNumReplicas: toIntPtr(int(location.MaxNumReplicas)),
		})
	}
	logger.V(2).Info("Successfully converted locations to Beamlit locations", "Locations", beamlitLocations)
	return beamlitLocations
}

func toBeamlitHPAConfig(ctx context.Context, kubernetesClient client.Client, hpaConfig *v1alpha1.ScalingConfig) (autoscalingv2.HorizontalPodAutoscalerSpec, error) {
	logger := log.FromContext(ctx)
	logger.V(2).Info("Converting HPA config to Beamlit HPA config", "HPA Config", hpaConfig)
	if hpaConfig.MetricPort == 0 {
		logger.V(2).Info("No HPA config found, skipping", "HPA Config", hpaConfig)
		// We don't need to send a scaling instruction if there is no metric port
		return autoscalingv2.HorizontalPodAutoscalerSpec{}, nil
	}

	if hpaConfig.HPARef != nil {
		logger.V(2).Info("Retrieving HPA", "HPA Ref", hpaConfig.HPARef)
		hpaSpec, err := retrieveHPA(ctx, kubernetesClient, hpaConfig.HPARef.Name, hpaConfig.HPARef.Namespace)
		if err != nil {
			logger.V(0).Error(err, "Failed to retrieve HPA", "HPA Ref", hpaConfig.HPARef)
			return autoscalingv2.HorizontalPodAutoscalerSpec{}, err
		}
		logger.V(2).Info("Successfully retrieved HPA", "HPA Spec", hpaSpec)
		return hpaSpec, nil
	}

	logger.V(2).Info("Creating HPA spec from config", "HPA Config", hpaConfig)
	return autoscalingv2.HorizontalPodAutoscalerSpec{
		Metrics:  hpaConfig.Metrics,
		Behavior: hpaConfig.Behavior,
	}, nil
}
