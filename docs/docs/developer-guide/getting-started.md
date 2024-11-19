# Getting Started

In this guide, we'll go over the development process for the Beamlit Operator.

## Prerequisites

- Go 1.20+
- Docker
- kubectl
- kind
- controller-gen

## Setting up the development environment

1. Clone the repository:

```
git clone https://github.com/beamlit/beamlit-controller.git
```

2. Build the operator binary:

```
make dev-build
```

3. Run the operator locally:

```
./dev/manager
```

4. (Optional) Build the operator Docker image and use it to run the operator locally in KinD:

```
kind create cluster
make dev-docker-build
kind load docker-image beamlit/operator:dev
cd chart
helm install beamlit-operator . --set controllerManager.manager.repository=beamlit/operator,controllerManager.manager.tag=dev
```

## Testing

To run the tests, simply run:

```
make test
```

## Code generation

The operator uses controller-gen to generate the CRD and webhook manifests. To generate the manifests, run:

```
make generate
```
