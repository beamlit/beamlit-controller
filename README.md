# Beamlit Operator

Beamlit Operator is a Kubernetes operator for Beamlit. It allows you to manage your Beamlit model deployments directly on Kubernetes.
One key benefit is that it allows you to offload your model deployments to Beamlit or any other Kubernetes cluster.

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [Support](#support)
- [Contributing](#contributing)

## Installation

At the moment, the operator only supports Gateway API.

### Prerequisites

- Kubernetes 1.22+
- Helm 3.8.0+

### Full Installation

> Use this if you want to have a full installation of the operator on your cluster with all the dependencies needed to offload your model.

```sh
git clone https://github.com/beamlit/operator.git
cd operator/chart
helm dependency update
helm install beamlit-operator . -n beamlit --create-namespace --full-install --
```

## Usage

### Prerequisites

- Beamlit API key
- Kubernetes cluster with the operator installed
- A deployed model on your Kubernetes cluster

### Deploy a model

```sh
kubectl apply -f - << EOF
apiVersion: model.beamlit.io/v1alpha1
kind: ModelDeployment
metadata:
  labels:
    app.kubernetes.io/name: operator
    app.kubernetes.io/managed-by: kustomize
  name: modeldeployment-sample
spec:
  displayName: "Model Deployment Sample"
  enabledLocations:
    - location: fr-east-lcl
  environment: prod
  supportedGPUTypes:
    - T4
  modelSourceRef:
    apiVersion: apps/v1 # mandatory
    kind: Deployment # mandatory
    name: php-apache # mandatory
    namespace: default # remove
EOF
```

### Offload a model

```sh
kubectl apply -f - << EOF
apiVersion: model.beamlit.io/v1alpha1
kind: ModelDeployment
metadata:
  labels:
    app.kubernetes.io/name: operator
    app.kubernetes.io/managed-by: kustomize
  name: modeldeployment-sample
spec:
  displayName: "Model Deployment Sample"
  enabledLocations:
    - location: fr-east-lcl
  environment: prod
  supportedGPUTypes:
    - T4
  modelSourceRef:
    apiVersion: apps/v1 # mandatory
    kind: Deployment # mandatory
    name: php-apache # mandatory
    namespace: default # remove
  offloadingConfig:
    disabled: false
    behavior:
      percentage: 50
    localServiceRef:
      name: php-apache
      namespace: default
      targetPort: 80
    metrics:
      - type: Resource
        resource:
          name: cpu
          target:
            type: Utilization
            averageUtilization: 50
EOF
```

## Support

Please [open an issue](https://github.com/beamlit/operator/issues/new) for support.

## Contributing

Please contribute using [Github Flow](https://guides.github.com/introduction/flow/). Create a branch, add commits, and [open a pull request](https://github.com/beamlit/operator/compare/).
