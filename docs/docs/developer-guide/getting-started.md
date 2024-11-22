# Getting Started

In this guide, we'll go over the development process for the Beamlit Controller.

## Prerequisites

- Go 1.23+
- Docker
- kubectl
- helm 3.8.0+
- kind with kubernetes 1.31+ (for local development)

## Setting up the development environment

1. Clone the repository:
  ```bash
  git clone https://github.com/beamlit/beamlit-controller.git
  cd beamlit-controller
  ```

2. Build the gateway Docker image:
  ```bash
  docker build -t beamlit/gateway:dev -f Dockerfile.gateway .
  ```

3. Build the controller Docker image:
  ```bash
  docker build -t beamlit/controller:dev -f Dockerfile .
  ```

4. Create a local Kubernetes cluster using KinD:
  ```bash
  kind create cluster
  ```

5. Load the Docker images into the KinD cluster:
  ```bash
  kind load docker-image beamlit/gateway:dev
  kind load docker-image beamlit/controller:dev
  ```

6. Modify the Helm chart to use the local images and run:
  ```bash
  cd chart
  helm dependency build
  helm install beamlit-controller .
  ```

## Testing

To run the tests, simply run:

```bash
make test
```

## Code generation

The controller uses controller-gen to generate the CRD and webhook manifests. To generate the manifests, run:

```bash
make generate
```

## Contributing

Please read our [contributing guidelines](https://github.com/beamlit/beamlit-controller/blob/main/CONTRIBUTING.md) before submitting a pull request.
