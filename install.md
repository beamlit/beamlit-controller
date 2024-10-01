# Install

```
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
kubectl patch -n kube-system deployment metrics-server --type=json \
  -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'
helm install eg oci://docker.io/envoyproxy/gateway-helm --version v0.0.0-latest -n envoy-gateway-system --create-namespace
kubectl wait --timeout=5m -n envoy-gateway-system deployment/envoy-gateway --for=condition=Available
cd chart
helm install controller .
```

# Test

```
kubectl apply -f https://k8s.io/examples/application/php-apache.yaml
kubectl apply -f ./config/samples/model_v1alpha1_modeldeployment.yaml
kubectl run -i --tty load-generator --rm --image=busybox:1.28 --restart=Never -- /bin/sh -c "while sleep 0.01; do wget -q -O- http://php-apache.default/; done"
```

# Cleanup test

```
kubectl delete -f ./config/samples/model_v1alpha1_modeldeployment.yaml
kubectl delete -f https://k8s.io/examples/application/php-apache.yaml
helm uninstall controller
helm uninstall eg
```
