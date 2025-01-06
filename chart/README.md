# beamlit-controller-chart

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

A Helm chart to deploy beamlit controller on your Kubernetes cluster

## Requirements

| Repository | Name | Version |
|------------|------|---------|
| file://../gateway/chart | beamlit-gateway-chart | 0.1.0 |
| https://kubernetes-sigs.github.io/metrics-server/ | metrics-server | 3.12.1 |

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| allowedNamespaces | list | `["default"]` | allowed namespaces |
| beamlitApiToken | string | `"REPLACE_ME"` | beamlit api token |
| beamlitBaseUrl | string | `"https://api.beamlit.com/v0"` | beamlit base url |
| config | object | `{"defaultRemoteBackend":{"authConfig":{"oauthConfig":{"clientId":"REPLACE_ME","clientSecret":"REPLACE_ME","tokenUrl":"https://api.beamlit.com/v0/oauth/token"},"type":"oauth"},"host":"run.beamlit.com","pathPrefix":"/$workspace/models/$model","scheme":"https"},"enableHTTP2":false,"namespaces":"default","proxyService":{"adminPort":8081,"name":"beamlit-gateway","namespace":"default","port":8080},"secureMetrics":false}` | config.yaml options |
| config.defaultRemoteBackend | object | `{"authConfig":{"oauthConfig":{"clientId":"REPLACE_ME","clientSecret":"REPLACE_ME","tokenUrl":"https://api.beamlit.com/v0/oauth/token"},"type":"oauth"},"host":"run.beamlit.com","pathPrefix":"/$workspace/models/$model","scheme":"https"}` | default-remote-backend |
| config.defaultRemoteBackend.authConfig | object | `{"oauthConfig":{"clientId":"REPLACE_ME","clientSecret":"REPLACE_ME","tokenUrl":"https://api.beamlit.com/v0/oauth/token"},"type":"oauth"}` | auth-config |
| config.defaultRemoteBackend.authConfig.oauthConfig | object | `{"clientId":"REPLACE_ME","clientSecret":"REPLACE_ME","tokenUrl":"https://api.beamlit.com/v0/oauth/token"}` | oauth2 |
| config.defaultRemoteBackend.authConfig.oauthConfig.clientId | string | `"REPLACE_ME"` | client-id |
| config.defaultRemoteBackend.authConfig.oauthConfig.clientSecret | string | `"REPLACE_ME"` | client-secret |
| config.defaultRemoteBackend.authConfig.oauthConfig.tokenUrl | string | `"https://api.beamlit.com/v0/oauth/token"` | token-url |
| config.defaultRemoteBackend.authConfig.type | string | `"oauth"` | type |
| config.defaultRemoteBackend.host | string | `"run.beamlit.com"` | host |
| config.defaultRemoteBackend.pathPrefix | string | `"/$workspace/models/$model"` | path-prefix |
| config.defaultRemoteBackend.scheme | string | `"https"` | scheme |
| config.enableHTTP2 | bool | `false` | enable-http2 |
| config.namespaces | string | `"default"` | namespaces |
| config.proxyService | object | `{"adminPort":8081,"name":"beamlit-gateway","namespace":"default","port":8080}` | proxy-service |
| config.proxyService.adminPort | int | `8081` | proxy-service.admin-port |
| config.proxyService.name | string | `"beamlit-gateway"` | proxy-service.name |
| config.proxyService.namespace | string | `"default"` | proxy-service.namespace |
| config.proxyService.port | int | `8080` | proxy-service.port |
| config.secureMetrics | bool | `false` | secure-metrics |
| controllerManager.kubeRbacProxy | object | `{"args":["--secure-listen-address=0.0.0.0:8443","--upstream=http://127.0.0.1:8080/","--logtostderr=true","--v=0"],"containerSecurityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}},"image":{"repository":"gcr.io/kubebuilder/kube-rbac-proxy","tag":"v0.16.0"},"resources":{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"5m","memory":"64Mi"}}}` | kube-rbac-proxy options |
| controllerManager.kubeRbacProxy.args | list | `["--secure-listen-address=0.0.0.0:8443","--upstream=http://127.0.0.1:8080/","--logtostderr=true","--v=0"]` | args to pass to the kube-rbac-proxy |
| controllerManager.kubeRbacProxy.containerSecurityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}` | container security context |
| controllerManager.kubeRbacProxy.containerSecurityContext.allowPrivilegeEscalation | bool | `false` | allowPrivilegeEscalation |
| controllerManager.kubeRbacProxy.containerSecurityContext.capabilities | object | `{"drop":["ALL"]}` | capabilities to drop |
| controllerManager.kubeRbacProxy.image | object | `{"repository":"gcr.io/kubebuilder/kube-rbac-proxy","tag":"v0.16.0"}` | image to use for the kube-rbac-proxy |
| controllerManager.kubeRbacProxy.resources | object | `{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"5m","memory":"64Mi"}}` | resources to request for the kube-rbac-proxy |
| controllerManager.kubeRbacProxy.resources.limits | object | `{"cpu":"500m","memory":"128Mi"}` | limits for the kube-rbac-proxy |
| controllerManager.kubeRbacProxy.resources.requests | object | `{"cpu":"5m","memory":"64Mi"}` | requests for the kube-rbac-proxy |
| controllerManager.manager | object | `{"containerSecurityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}},"image":{"pullPolicy":"IfNotPresent","repository":"ghcr.io/beamlit/beamlit-controller","tag":"latest"},"resources":{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"10m","memory":"64Mi"}}}` | manager options |
| controllerManager.manager.containerSecurityContext | object | `{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}` | container security context |
| controllerManager.manager.containerSecurityContext.allowPrivilegeEscalation | bool | `false` | allowPrivilegeEscalation |
| controllerManager.manager.image | object | `{"pullPolicy":"IfNotPresent","repository":"ghcr.io/beamlit/beamlit-controller","tag":"latest"}` | image to use for the manager |
| controllerManager.manager.resources | object | `{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"10m","memory":"64Mi"}}` | resources to request for the manager |
| controllerManager.manager.resources.limits | object | `{"cpu":"500m","memory":"128Mi"}` | limits for the manager |
| controllerManager.manager.resources.requests | object | `{"cpu":"10m","memory":"64Mi"}` | requests for the manager |
| controllerManager.podSecurityContext | object | `{"runAsNonRoot":true}` | pod security context |
| controllerManager.replicas | int | `1` | number of replicas |
| controllerManager.serviceAccount | object | `{"annotations":{}}` | service account |
| installBeamlitGateway | bool | `true` | installBeamlitGateway is a flag to install the beamlit gateway along with the controller |
| installMetricServer | bool | `false` | installMetricsServer is a flag to install the metrics-server along with the controller |
| kubernetesClusterDomain | string | `"cluster.local"` | kubernetes cluster domain |
| metrics-server | object | `{"args":["--kubelet-insecure-tls"]}` | metrics-server options |
| metrics-server.args | list | `["--kubelet-insecure-tls"]` | args to pass to the metrics-server |
| metricsService | object | `{"ports":[{"name":"https","port":8443,"protocol":"TCP","targetPort":"https"}],"type":"ClusterIP"}` | metrics service |
| metricsService.ports | list | `[{"name":"https","port":8443,"protocol":"TCP","targetPort":"https"}]` | ports for the metrics service |

