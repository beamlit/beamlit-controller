package helper

import (
	authorizationv1alpha1 "github.com/beamlit/beamlit-controller/api/v1alpha1/authorization"
	beamlit "github.com/beamlit/toolkit/sdk"
)

func ToBeamlitPolicy(policy *authorizationv1alpha1.Policy) *beamlit.Policy {
	beamlitPolicy := &beamlit.Policy{}
	beamlitPolicy.Metadata.Name = &policy.Name
	beamlitPolicy.Metadata.DisplayName = &policy.Spec.DisplayName
	//beamlitPolicy.Labels = toBeamlitLabels(policy.Labels) // TODO: Add this back when we have a way to convert labels to Beamlit labels
	switch policy.Spec.Type {
	case authorizationv1alpha1.PolicyTypeFlavor:
		beamlitPolicy.Spec.Type = toPtr("flavor")
		beamlitPolicy.Spec.Flavors = toBeamlitFlavors(policy.Spec.Flavors)
	case authorizationv1alpha1.PolicyTypeLocation:
		beamlitPolicy.Spec.Type = toPtr("location")
		beamlitPolicy.Spec.Locations = toBeamlitLocations(policy.Spec.Locations)
	}
	return beamlitPolicy
}

func toBeamlitFlavors(flavors []authorizationv1alpha1.PolicyFlavor) *[]beamlit.Flavor {
	beamlitFlavors := make([]beamlit.Flavor, len(flavors))
	for i, flavor := range flavors {
		beamlitFlavors[i] = beamlit.Flavor{
			Name: &flavor.Name, //nolint:exportloopref
			Type: &flavor.Type, //nolint:exportloopref
		}
	}
	return &beamlitFlavors
}

func toBeamlitLocations(locations []authorizationv1alpha1.PolicyLocation) *[]beamlit.PolicyLocation {
	beamlitLocations := make([]beamlit.PolicyLocation, len(locations))
	for i, location := range locations {
		typeStr := string(location.Type)
		beamlitLocations[i] = beamlit.PolicyLocation{
			Name: &location.Name, //nolint:exportloopref
			Type: &typeStr,
		}
	}
	return &beamlitLocations
}
