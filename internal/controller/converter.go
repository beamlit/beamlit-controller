package controller

import (
	"context"

	"github.com/beamlit/operator/api/v1alpha1"
	"github.com/beamlit/operator/internal/beamlit"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourceutil "github.com/beamlit/operator/internal/k8s_resource_utils"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
)

func Convert(ctx context.Context, kubernetesClient client.Client, modelDeployment *v1alpha1.ModelDeployment) (beamlit.ModelDeployment, error) {
	beamlitModelDeployment := beamlit.ModelDeployment{
		Name:              modelDeployment.Name,
		DisplayName:       &modelDeployment.Spec.DisplayName,
		EnabledLocations:  toBeamlitLocations(modelDeployment.Spec.EnabledLocations),
		SupportedGPUTypes: modelDeployment.Spec.SupportedGPUTypes,
		Environment:       modelDeployment.Spec.Environment,
		ScalingConfig: &beamlit.ScalingConfig{
			MinNumReplicas: toIntPtr(int(modelDeployment.Spec.MinNumReplicasPerLocation)),
			MaxNumReplicas: toIntPtr(int(modelDeployment.Spec.MaxNumReplicasPerLocation)),
			MetricPort:     toIntPtr(int(modelDeployment.Spec.ScalingConfig.MetricPort)),
			MetricPath:     &modelDeployment.Spec.ScalingConfig.MetricPath,
		},
	}

	template, err := resourceutil.PodTemplate(ctx, kubernetesClient, &modelDeployment.Spec.ModelSourceRef)
	if err != nil {
		return beamlit.ModelDeployment{}, err
	}
	beamlitModelDeployment.PodTemplate = template

	hpaSpec, err := toBeamlitHPAConfig(ctx, kubernetesClient, modelDeployment.Spec.ScalingConfig)
	if err != nil {
		return beamlit.ModelDeployment{}, err
	}
	beamlitModelDeployment.ScalingConfig.HorizontalPodAutoscaler = &hpaSpec

	return beamlitModelDeployment, nil
}

func toBeamlitLocations(locations []v1alpha1.Location) []beamlit.Location {
	beamlitLocations := make([]beamlit.Location, 0, len(locations))
	for _, location := range locations {
		beamlitLocations = append(beamlitLocations, beamlit.Location{
			Location:       location.Location,
			MinNumReplicas: toIntPtr(int(location.MinNumReplicas)),
			MaxNumReplicas: toIntPtr(int(location.MaxNumReplicas)),
		})
	}
	return beamlitLocations
}

func toBeamlitHPAConfig(ctx context.Context, kubernetesClient client.Client, hpaConfig *v1alpha1.ScalingConfig) (autoscalingv2.HorizontalPodAutoscalerSpec, error) {
	if hpaConfig.MetricPort == 0 {
		// We don't need to send a scaling instruction if there is no metric port
		return autoscalingv2.HorizontalPodAutoscalerSpec{}, nil
	}

	if hpaConfig.HPARef != nil {
		return resourceutil.HPA(ctx, kubernetesClient, hpaConfig.HPARef.Namespace, hpaConfig.HPARef.Name)
	}

	return autoscalingv2.HorizontalPodAutoscalerSpec{
		Metrics:  hpaConfig.Metrics,
		Behavior: hpaConfig.Behavior,
	}, nil
}
