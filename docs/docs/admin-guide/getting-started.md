# Beamlit Controller

!!! warning test

    Please read the [Getting Started](/index.html) before reading this section.

The Beamlit Controller that comes with its own CRDs to manage Beamlit AI agents directly in your Kubernetes cluster. It is built using the Operator SDK and the Kubernetes Go client library.

It is composed of two components:

- A controller which is responsible for watching the resources and reconciling the desired state with the actual state.
- A gateway which is responsible for handling the incoming inference requests and routing them to the appropriate backend.

## Configuring Controller

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

In the config file, you can specify the metric source for offloading. Below is an example of how to configure the metric source for offloading:

```yaml
config:
  # ...
  metricInformer:
    type: prometheus
    prometheus:
      address: my-local-prom:9090
```
