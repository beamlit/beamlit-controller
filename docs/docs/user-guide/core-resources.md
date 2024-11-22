# Core Resources

With the Beamlit Controller you can interact directly inside your Kubernetes cluster with the following resources hosted on Beamlit:

- **Models** (using the [`ModelDeployment`](#modeldeployment) custom resource)
- **Policies** (using the [`Policy`](#policy) custom resource)
- More to come

The key benefits of using Beamlit resources in your cluster are:

- **Seamless Offloading**: Effortlessly offload your AI agents to remote clusters for hybrid deployments. Beamlit guarantees uninterrupted user traffic during bursts or failures and allows you to load balance between your on-premises and Beamlit-managed clusters.
- **Unified Management**: Centrally manage your AI agents and policies from a single, intuitive interface. Oversee all your AI models and policies with ease.
- **Automated Deployments**: Streamline the deployment of your AI agents using your existing tools. Beamlit is fully compatible with GitOps, CI/CD, and other DevOps tools, ensuring smooth integration into your workflow.

## ModelDeployment

To manage your models, you need to create a `ModelDeployment` resource.
Below is an example of a `ModelDeployment` resource for a model you have deployed in your Kubernetes cluster.
In this example, offloading is scheduled to trigger when the average CPU usage of the deployment reaches 90%,
at which point 50% of the requests will be routed to the remote cluster:

```yaml
apiVersion: deployment.beamlit.com/v1alpha1
kind: ModelDeployment
metadata:
  name: my-model
spec:
  model: my-model-on-Beamlit
  environment: production
  modelSourceRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-model
    namespace: default
  serviceRef:
    name: my-model
    namespace: default
    targetPort: 8080
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
```

Let's break down the fields in the `ModelDeployment` resource:

- `model`: The name of the model on Beamlit. If it exists, the environment of the model will be updated; otherwise, it will be created.
- `environment`: The environment of the model on Beamlit. By default, it is set to `production`. Yet, we only support `production` and `development` environments.
- `modelSourceRef`: The reference to the Kubernetes deployment, statefulset, daemonset, ..., that hosts the model.
- `serviceRef`: The reference to the Kubernetes service that exposes the model. The `targetPort` field specifies the port on which the model is listening for incoming inference requests.
- `offloadingConfig`: The configuration for offloading the model. It specifies the behavior of the offloading and the metrics that trigger the offloading. Note, you can disable offloading by omitting this field.

For further details on the `ModelDeployment` resource, refer to the [ModelDeployment API reference](/crds/crds-docs.html#modeldeployment).

## Policy

A `Policy` resource allows you to define rules that govern the deployment of your model on Beamlit, thus the behavior of the offloading.
There is two types of policies: one for the location of the offloading and one for the flavor of the offloading.

Here is an example of a `Policy` resource that specifies a location constraint (only offload to the US and North America).

```yaml
apiVersion: authorization.beamlit.com/v1alpha1
kind: Policy
metadata:
  name: my-policy
spec:
  type: location
  locations:
    - type: country
      name: "us"
    - type: continent
      name: "na"
```

To attach this policy to a model, you need to reference it in the `ModelDeployment` resource:

```yaml hl_lines="17-21"
apiVersion: deployment.beamlit.com/v1alpha1
kind: ModelDeployment
metadata:
  name: my-model
spec:
  model: my-model-on-Beamlit
  environment: production
  modelSourceRef:
    apiVersion: apps/v1
    kind: Deployment
    name: my-model
    namespace: default
  serviceRef:
    name: my-model
    namespace: default
    targetPort: 8080
  policies:
    - refType: localPolicy
      name: my-policy
    - refType: remotePolicy
      name: my-policy-on-beamlit
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
```

In this example, the `Policy` resource `my-policy` is attached to the `ModelDeployment` resource `my-model`.
Along with the location policy, a flavor policy `my-policy-on-beamlit` is also attached to the model, this is a policy living on Beamlit.

For further details on the `Policy` resource, refer to the [Policy API reference](/crds/crds-docs.html#policy).

## Next Steps

- [Learn about offloading metrics](offloading-metric.md)
