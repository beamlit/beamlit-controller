# Beamlit Controller

## TL;DR

You can apply for private beta access [here](https://beamlit.com/beta-signup).
After that, you will need to create a workspace and a [service account](https://docs.beamlit.com/Security/Service-accounts) in the workspace.
Make sure to retrieve the service account's `client ID` and `client secret` as you will need it to install the controller.
Then you can install the Beamlit Controller using Helm:

```sh
export CLIENT_ID=REPLACE_ME
export CLIENT_SECRET=REPLACE_ME
export API_KEY=`echo -n $CLIENT_ID:$CLIENT_SECRET | base64`
helm install beamlit-controller oci://ghcr.io/beamlit/beamlit-controller-chart \
    --set installMetricServer=true \ # If you want to install the metric server along with the controller to allow offloading models
    --set beamlitApiToken=$API_KEY \
    --set config.defaultRemoteBackend.authConfig.oauthConfig.clientId=$CLIENT_ID \
    --set config.defaultRemoteBackend.authConfig.oauthConfig.clientSecret=$CLIENT_SECRET
```

## Introduction

!!! note "Before you begin"
    Please read the [Getting Started](/index.html) before reading this section.

The Beamlit Controller that comes with its own CRDs to manage Beamlit AI agents directly in your Kubernetes cluster. It is built using the Operator SDK and the Kubernetes Go client library.

It is composed of two components:

- A controller which is responsible for watching the resources and reconciling the desired state with the actual state.
- A gateway which is responsible for handling the incoming inference requests and routing them to the appropriate backend.

## Configuring Controller

!!! warning "Environment variable"
    You need to set `BEAMLIT_TOKEN` env variable to be able to use the Beamlit Controller.
    This token is used to authenticate with the Beamlit API. It's you `clientId` and `clientSecret` concatenated with a colon and then base64 encoded.
    Example: `echo -n $CLIENT_ID:$CLIENT_SECRET | base64`


You can configure the Beamlit Controller by creating a `config.yaml` file. Below is an example of a `config.yaml` file:

```yaml
# -- enable-http2
enableHTTP2: false
# -- secure-metrics
secureMetrics: false
# -- namespaces
namespaces: default
# -- default-remote-backend
defaultRemoteBackend:
  # -- host
  host: "run.beamlit.dev"
  # -- path-prefix
  pathPrefix: "/$workspace/$model"
  # -- auth-config
  authConfig:
    # -- type
    type: oauth
    # -- oauth2
    oauthConfig:
      # -- client-id
      clientId: "REPLACE_ME"
      # -- client-secret
      clientSecret: "REPLACE_ME"
      # -- token-url
      tokenUrl: "https://api.beamlit.dev/v0/oauth/token"
  # -- scheme
  scheme: https
```

Please refer to the [Configuration Reference](https://github.com/beamlit/beamlit-controller/blob/89ca8aaed7d77d523f1dbec806755154c0a4a8b6/internal/config/config.go#L22) for more information on the configuration options.

## Deploying Controller

We advise you to deploy the Beamlit Controller using Helm. Below is an example of how to deploy the Beamlit Controller using Helm:

```sh
export CLIENT_ID=REPLACE_ME
export CLIENT_SECRET=REPLACE_ME
export API_KEY=`echo -n $CLIENT_ID:$CLIENT_SECRET | base64`
helm install beamlit-controller oci://ghcr.io/beamlit/beamlit-controller-chart \
    --set installMetricServer=true \ # If you want to install the metric server along with the controller to allow offloading models
    --set beamlitApiToken=$API_KEY \
    --set config.defaultRemoteBackend.authConfig.oauthConfig.clientId=$CLIENT_ID \
    --set config.defaultRemoteBackend.authConfig.oauthConfig.clientSecret=$CLIENT_SECRET
```

You can find the default values for the Beamlit Controller Helm chart and more information [here](deploy-helm.md)

## Configuring Metric Source for Offloading

Currently, the Beamlit Controller supports offloading models to a remote backend based on two metric source:

- Kubernetes Metrics Server (Default)
- Prometheus Compatible servers

In the config file, you can specify the metric source for offloading. Below is an example of how to configure the metric source for offloading to
use Prometheus:

```yaml
config:
  # ...
  metricInformer:
    type: prometheus
    prometheus:
      address: my-local-prom:9090
```
