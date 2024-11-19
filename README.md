# Beamlit Controller

<p align="center">
  <img src="./docs/assets/beamlit-logo.png" alt="Beamlit Controller"/>
</p>

![GitHub License](https://img.shields.io/github/license/beamlit/beamlit-controller)
![Go Report Card](https://goreportcard.com/badge/github.com/beamlit/beamlit-controller)
![GitHub contributors](https://img.shields.io/github/contributors/beamlit/beamlit-controller)

Beamlit Controller is a Kubernetes controller for Beamlit. It allows you to manage your Beamlit model deployments directly on Kubernetes.
One key benefit is that it allows you to offload your model deployments to Beamlit or any other Kubernetes cluster.

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [Support](#support)
- [Contributing](#contributing)

## Installation

At the moment, the controller needs you to have a beamlit account. You can create one [here](https://beamlit.com/). Then,
create a service account [here](https://app.beamlit.dev/mjoffre/workspace/settings/service-accounts).
You will need the client-id and client-secret of the service account to setup the controller.

### Prerequisites

- Kubernetes cluster (version 1.27 or later is recommended)
- Helm (version 3.8.0 or later is recommended)

### Full Installation

> Use this if you want to have a full installation of the controller on your cluster with all the dependencies needed to offload your model in a snap.

```sh
export CLIENT_ID="..."
export CLIENT_SECRET="..."
export API_KEY=`echo -n $CLIENT_ID:$CLIENT_SECRET | base64`
helm install beamlit-controller oci://ghcr.io/beamlit/beamlit-controller-chart \
    --set installMetricServer=true \
    --set beamlitApiToken=$API_KEY \
    --set controllerManager.manager.image.repository=sugate/operator \
    --set controllerManager.manager.image.tag=latest-mac \
    --set config.defaultRemoteBackend.authConfig.oauthConfig.clientId=$CLIENT_ID \
    --set config.defaultRemoteBackend.authConfig.oauthConfig.clientSecret=$CLIENT_SECRET
```

## Getting Started

### Prerequisites

- Kubernetes cluster with the controller installed
- A deployed model on your Kubernetes cluster

### Deploy a model

Let's assume this is your model deployment, a simple PHP-Apache deployment for testing purposes:
```bash
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: php-apache
spec:
  selector:
    matchLabels:
      run: php-apache
  template:
    metadata:
      labels:
        run: php-apache
    spec:
      containers:
      - name: php-apache
        image: registry.k8s.io/hpa-example
        ports:
        - containerPort: 80
        resources:
          limits:
            cpu: 500m
          requests:
            cpu: 200m
---
apiVersion: v1
kind: Service
metadata:
  name: php-apache
  labels:
    run: php-apache
spec:
  ports:
  - port: 80
  selector:
    run: php-apache
EOF
```
You want to offload this deployment to Beamlit. To do so, you need to create a model deployment resource.

### Create a Beamlit model deployment

To create a model deployment, you need to create a ModelDeployment resource. Here is an example of a ModelDeployment resource for the PHP-Apache deployment:
```bash
kubectl apply -f - <<EOF
apiVersion: deployment.beamlit.com/v1alpha1
kind: ModelDeployment
metadata:
  name: php-apache
spec:
  model: "php-apache"
  environment: "production"
  modelSourceRef:
    apiVersion: apps/v1
    kind: Deployment
    name: php-apache
    namespace: default
  serviceRef:
    name: php-apache
    namespace: default
    targetPort: 80
  offloadingConfig:
    behavior:
      percentage: 50
    metrics:
      - type: Resource
        resource:
          name: cpu
          target:
            type: Utilization
            averageUtilization: 90
EOF
```
Right now, your model deployment is not offloaded to Beamlit. The controller will take care of that for you.
When the controller will be notified that metric is above the threshold, it will offload the model to Beamlit.
When there will be a failure in the model deployment, the controller will offload the model to Beamlit.

### Check the status of the model deployment

You can check the status of the model deployment by running:
```bash
kubectl get modeldeployment php-apache
```

### Offload on failure

In a terminal, start running some load:
```bash
kubectl run curl-check --rm -it --image=curlimages/curl -- sh -c "while true; do response=\$(curl -D - http://php-apache); echo \"\$response\"; echo \$response | grep -q 'Cf-Ray' && echo 'Route: beamlit' || echo 'Route: local'; sleep 0.1; done"
```

In another terminal, scale down the deployment:
```bash
kubectl scale deployment php-apache --replicas=0
```
You should see the output of the first terminal changing to `Route: beamlit` after scaling down the deployment.
You had experienced no downtime and no error in the first terminal. This is Beamlit.

## Support

Please [open an issue](https://github.com/beamlit/controller/issues/new) for support.

## Contributing

Please contribute using [Github Flow](https://guides.github.com/introduction/flow/). Create a branch, add commits, and [open a pull request](https://github.com/beamlit/controller/compare/).
