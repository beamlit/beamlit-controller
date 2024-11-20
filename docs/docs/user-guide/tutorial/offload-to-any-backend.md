# Offload your model to any destination

One key thing with Beamlit Controller, is that you can use it without any Beamlit subscription. Basically, if you'are already
running models replicas spread accross multiple clusters, Beamlit Controller can help you manage traffic and offload your models.

In the following tutorial will show you how to setup this architecture:

<span style="text-align: center">
  ```mermaid
  flowchart TD
  subgraph s1["Kubernetes A"]
          n1["Llama 3"]
          n2["API"]
          n3["Beamlit Gateway"]
    end
  subgraph s2["Kubernetes B"]
          n4["Llama 3"]
    end
      n2 -- Call Llama3 --> n3
      n3 --> n1
      n3 -- Oflload --> n4
  ```

</span>

## Requirements

- Two Kubernetes clusters
- Helm (version 3.8.0 or later is recommended).
- Beamlit controller installed on the first cluster (See [Getting Started](/index.html) for that)

## Let's dive in!

Lets assume you have a model deployment in your Kubernetes cluster. For testing purposes, this is a simple PHP-Apache deployment.

```yaml
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
```

Deploy this on the second cluster too, but make it reachable from the first cluster.

```yaml hl_lines="32"
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
  type: LoadBalancer
  ports:
  - port: 80
  selector:
    run: php-apache
```

Now, you want to offload your deployment on the first cluster. Just create a ModelDeployment resource.

```yaml hl_lines="18-20"
apiVersion: deployment.beamlit.com/v1alpha1
kind: ModelDeployment
metadata:
  name: my-model
spec:
  model: my-model
  environment: production
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

## Further reading

- Check the CRD definition for more details on the [ModelDeployment](/reference/crds/modeldeployment.html) resource and
availables fields in `spec.offloadingConfig.remoteBackend`.
