# Offloading Metric

The offloading metric is a customizable infrastructure metric used by the Beamlit controller to trigger [model offloading](https://www.notion.so/Model-overflow-13847e47b1ea817f82a8c8bda522126f?pvs=21) when it hits a certain threshold.

Currently, Beamlit supports two ways to retrieve metrics:

- metrics from a **self-managed [Prometheus](https://github.com/prometheus/prometheus)**, evaluated through a [Prometheus query (PromQL)](https://prometheus.io/docs/prometheus/latest/querying/basics/).
- metrics from a Kubernetes **metrics-server**

## Overview

Setting up the offloading metric is made via the following parameters in the `ModelDeployment` custom resource:

```yaml
spec:
  # ...
  offloadingConfig:
    remoteBackend:
      host: my-model-on-another-cluster:80
      scheme: http
    behavior:
      percentage: 50
    metrics:
      - type: Resource
        resource:
          name: cpu
          target:
            type: Utilization
            averageUtilization: 50
```

where:

- `remoteBackend` is the reference to the remote cluster/backend where your traffic will be offloaded to
- `behavior` is the **percentage** of requests to offload to the remote backend when the offloading metric reaches its threshold
- `metrics` is the **offloading metric**, based on which the controller will decide whether to trigger traffic offloading

## Set up metric using Prometheus

### Prerequisites

- A [Prometheus](https://github.com/prometheus/prometheus) server that is either in your Kubernetes cluster, or accessible in your network via URL without authentication.
- Set up the [Beamlit controller](https://www.notion.so/Install-Beamlit-operator-13847e47b1ea81dab2eefb5694f621ef?pvs=21) to monitor your Prometheus service by adding the following configuration in the controller’s `values.yaml` :

```yaml
config:
  metricInformer:
    type: prometheus
    prometheus:
      address: my-local-prom:9090
```

### Metric overview

The Beamlit controller can use any metric that is saved in the Prometheus database, or use any computation on such metrics using PromQL.

Metrics are specified by an _External_ metric ([MetricSpec from Kubernetes](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/#autoscaling-on-metrics-not-related-to-kubernetes-objects)). For example:

```yaml
offloadingConfig:
  remoteBackend:
    host: my-model-on-another-cluster:80
    scheme: http
  behavior:
    percentage: 50
  metrics:
    - type: External
      external:
        metric:
          name: my_custom_metric_in_prom # Metric name, or PromQL query
          selector:
            # Any label you want to match in your metric.
            # This only works for a single metric (not a PromQL query).
            matchLabels:
            my_label: "value"
          target:
            type: Value
            value: 10
```

## Set up metric using Kubernetes metrics-server

### Prerequisites

- Kubernetes [metrics-server](https://github.com/kubernetes-sigs/metrics-server) must be installed on your Kubernetes cluster.

<Tip>Using metrics-server is the default mode of the Beamlit controller.</Tip>

### Metric overview

The Beamlit controller can use any metric that's compatible with the Kubernetes [HorizontalPodAutoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/#autoscaling-on-multiple-metrics-and-custom-metrics) (HPA) and accessible through _metrics-server_.

Metrics are defined using the [Horizontal Pod Autoscaler (HPA) format](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/#autoscaling-on-multiple-metrics-and-custom-metrics) for Resource, Pod, Object, and External metrics. For example, with [Resource metrics](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#support-for-resource-metrics):

```yaml
metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 50
```

This will trigger offloading when the average CPU usage gets higher than 50%.

<Tip>Read more about the available parameters for defining metrics and targets on the [Kubernetes HPA documentation](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/#autoscaling-on-multiple-metrics-and-custom-metrics).</Tip>
