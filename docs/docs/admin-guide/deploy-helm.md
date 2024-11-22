# Helm Deployment

You can deploy the Beamlit Controller using Helm. Below is an example of how to deploy the Beamlit Controller using Helm:

```sh
export CLIENT_ID=REPLACE_ME
export CLIENT_SECRET=REPLACE_ME
helm install beamlit-controller oci://ghcr.io/beamlit/beamlit-controller-chart \
    --set installMetricServer=true \ # If you want to install the metric server along with the controller to allow offloading models
    --set beamlitApiToken=`echo -n $CLIENT_ID:$CLIENT_SECRET | base64` \
    --set config.defaultRemoteBackend.authConfig.oauthConfig.clientId=$CLIENT_ID \
    --set config.defaultRemoteBackend.authConfig.oauthConfig.clientSecret=$CLIENT_SECRET
```

{!chart/README.md!lines=14-66}
