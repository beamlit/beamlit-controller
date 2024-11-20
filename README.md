# Beamlit Controller

<p align="center">
  <img src="./assets/beamlit-logo.png" alt="Beamlit Controller"/>
</p>

![GitHub License](https://img.shields.io/github/license/beamlit/beamlit-controller)
![Go Report Card](https://goreportcard.com/badge/github.com/beamlit/beamlit-controller)
![GitHub contributors](https://img.shields.io/github/contributors/beamlit/beamlit-controller)

Beamlit Controller is a Kubernetes controller for Beamlit, the global infrastructure for AI agents. With this controller, you can deploy and manage workloads (such as agents, models and functions) on Beamlit-managed or private clusters directly using Kubernetes.

A Beamlit gateway is also available to route inference requests to remote backends for improved resiliency and availability. Future-proof your existing AI deployments by offloading some or all traffic in case of usage surge or hardware failure.


## Table of Contents

- [Install](#install)
- [Get Started](#get-started)
- [Support](#support)
- [Contributing](#contributing)


## Install

For now, the controller requires you to have a Beamlit account. You can apply for private beta access [here](https://beamlit.com/beta-signup). After that, you will need to create a workspace and a [service account](https://docs.beamlit.com/Security/Service-accounts) in the workspace. Make sure to retrieve the service account's `client ID` and `client secret` as you will need it to install the controller.

#### Prerequisites

- A Kubernetes cluster (version 1.27 or later is recommended).
- Helm (version 3.8.0 or later is recommended).
- The `client ID` and `client secret` for a Beamlit service account, as explained above.

#### Full installation

Use this command for a complete installation of both the Beamlit controller and Beamlit gateway on your cluster, including all necessary dependencies for quick model offloading. Make sure to fill in the *CLIENT_ID* and *CLIENT_SECRET* values.

```sh
export CLIENT_ID="..."
export CLIENT_SECRET="..."
export API_KEY=`echo -n $CLIENT_ID:$CLIENT_SECRET | base64`
helm install beamlit-controller oci://ghcr.io/beamlit/beamlit-controller-chart \
    --set installMetricServer=true \
    --set beamlitApiToken=$API_KEY \
    --set config.defaultRemoteBackend.authConfig.oauthConfig.clientId=$CLIENT_ID \
    --set config.defaultRemoteBackend.authConfig.oauthConfig.clientSecret=$CLIENT_SECRET
```


## Get Started

With the Beamlit controller, you can deploy replicas of your AI applications on remote clusters —whether private or Beamlit-managed— facilitating hybrid deployments across multiple regions.

#### Deploy a model

Let's assume this is an AI model deployment in your Kubernetes cluster. For testing purposes, this is a simple PHP-Apache deployment.

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

You want to offload this deployment to Beamlit, to make sure that user traffic is served even in case of burst or failure. To do so, you need to create a model deployment resource.

#### Create a Beamlit model deployment

To create a model deployment, you need to create a ModelDeployment resource. Below is an example of a ModelDeployment resource for your PHP-Apache deployment. Here, offloading is scheduled to trigger when the average CPU usage of your deployment reaches 90%, in which case 50% of the requests will be routed to the remote cluster:

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

You can check the status of the model deployment by running:

```bash
kubectl get modeldeployment php-apache
```

The model is now deployed on Beamlit and the controller is in watching state for the offloading condition to be met. When it is met, the model will become active and some requests will start being routed to Beamlit, making sure all your consumers are served. If your own deployment is completely down, all traffic will be routed to Beamlit.

#### Offload on total failure

In a terminal, simulate some load on your deployment:

```bash
kubectl run curl-check --rm -it --image=curlimages/curl -- sh -c "while true; do response=\$(curl -D - http://php-apache); echo \"\$response\"; echo \$response | grep -q 'Cf-Ray' && echo 'Route: beamlit' || echo 'Route: local'; sleep 0.1; done"
```

In another terminal, scale down your deployment to simulate a total failure:
```bash
kubectl scale deployment php-apache --replicas=0
```
You should see the output of the first terminal changing to `Route: beamlit` after scaling down the deployment. You've experienced no downtime and no error in the first terminal.


## Support

If you need assistance with installing or using either the Beamlit controller or Beamlit gateway, please [open an issue](https://github.com/beamlit/controller/issues/new) for support.


## Contributing

Contributions are welcome! Please use the [Github flow](https://guides.github.com/introduction/flow/) to contribute to beamlit-controller. Create a branch, add commits, and [open a pull request](https://github.com/beamlit/beamlit-controller/compare/).
