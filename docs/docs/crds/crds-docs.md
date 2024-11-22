# API Reference

## Packages
- [authorization.beamlit.com/v1alpha1](#authorizationbeamlitcomv1alpha1)
- [deployment.beamlit.com/v1alpha1](#deploymentbeamlitcomv1alpha1)


## authorization.beamlit.com/v1alpha1

Package v1alpha1 contains API Schema definitions for the model v1alpha1 API group

### Resource Types
- [Policy](#policy)
- [PolicyList](#policylist)



#### Policy



Policy is the Schema for the policies API



_Appears in:_
- [PolicyList](#policylist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `authorization.beamlit.com/v1alpha1` | | |
| `kind` _string_ | `Policy` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PolicySpec](#policyspec)_ |  |  |  |
| `status` _[PolicyStatus](#policystatus)_ |  |  |  |


#### PolicyFlavor







_Appears in:_
- [PolicySpec](#policyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | Type is the type of the flavor |  | Required: \{\} <br /> |
| `name` _string_ | Name is the name of the flavor |  | Required: \{\} <br /> |


#### PolicyList



PolicyList contains a list of Policy





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `authorization.beamlit.com/v1alpha1` | | |
| `kind` _string_ | `PolicyList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Policy](#policy) array_ |  |  |  |


#### PolicyLocation







_Appears in:_
- [PolicySpec](#policyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[PolicySubTypeLocation](#policysubtypelocation)_ | Type is the type of the location |  | Required: \{\} <br /> |
| `name` _string_ | Name is the name of the location |  | Required: \{\} <br /> |


#### PolicySpec



PolicySpec defines the desired state of Policy on Beamlit



_Appears in:_
- [Policy](#policy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `displayName` _string_ | DisplayName is the display name of the policy |  | Optional: \{\} <br /> |
| `type` _[PolicyType](#policytype)_ | Type is the type of the policy |  | Enum: [location flavor] <br />Required: \{\} <br /> |
| `locations` _[PolicyLocation](#policylocation) array_ |  |  |  |
| `flavors` _[PolicyFlavor](#policyflavor) array_ | Flavors is the list of flavors allowed by the policy<br />If not set, all flavors are allowed |  | Optional: \{\} <br /> |


#### PolicyStatus



PolicyStatus defines the observed state of Policy



_Appears in:_
- [Policy](#policy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `createdAtOnBeamlit` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta)_ | CreatedAtOnBeamlit is the time when the policy was created on Beamlit |  |  |
| `updatedAtOnBeamlit` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta)_ | UpdatedAtOnBeamlit is the time when the policy was updated on Beamlit |  |  |
| `workspace` _string_ | Workspace is the workspace of the policy |  |  |


#### PolicySubTypeLocation

_Underlying type:_ _string_





_Appears in:_
- [PolicyLocation](#policylocation)

| Field | Description |
| --- | --- |
| `location` |  |
| `country` |  |
| `continent` |  |


#### PolicyType

_Underlying type:_ _string_





_Appears in:_
- [PolicySpec](#policyspec)

| Field | Description |
| --- | --- |
| `location` |  |
| `flavor` |  |



## deployment.beamlit.com/v1alpha1

Package v1alpha1 contains API Schema definitions for the model v1alpha1 API group

### Resource Types
- [ModelDeployment](#modeldeployment)
- [ModelDeploymentList](#modeldeploymentlist)
- [ToolDeployment](#tooldeployment)
- [ToolDeploymentList](#tooldeploymentlist)



#### AuthConfig







_Appears in:_
- [RemoteBackend](#remotebackend)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[AuthType](#authtype)_ | Type is the type of the authentication |  | Enum: [oauth] <br />Required: \{\} <br /> |
| `oauthConfig` _[OAuthConfig](#oauthconfig)_ | OAuthConfig is the OAuth configuration for the remote backend |  | Optional: \{\} <br /> |


#### AuthType

_Underlying type:_ _string_





_Appears in:_
- [AuthConfig](#authconfig)

| Field | Description |
| --- | --- |
| `oauth` |  |


#### ModelDeployment



ModelDeployment is the Schema for the modeldeployments API



_Appears in:_
- [ModelDeploymentList](#modeldeploymentlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `deployment.beamlit.com/v1alpha1` | | |
| `kind` _string_ | `ModelDeployment` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ModelDeploymentSpec](#modeldeploymentspec)_ |  |  |  |
| `status` _[ModelDeploymentStatus](#modeldeploymentstatus)_ |  |  |  |


#### ModelDeploymentList



ModelDeploymentList contains a list of ModelDeployment





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `deployment.beamlit.com/v1alpha1` | | |
| `kind` _string_ | `ModelDeploymentList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ModelDeployment](#modeldeployment) array_ |  |  |  |


#### ModelDeploymentSpec



ModelDeploymentSpec defines the desired state of ModelDeployment



_Appears in:_
- [ModelDeployment](#modeldeployment)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `model` _string_ | Model is the name of the base model |  | Required: \{\} <br /> |
| `enabled` _boolean_ | Enabled is the flag to enable the model deployment on Beamlit | true | Optional: \{\} <br /> |
| `modelSourceRef` _[ObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectreference-v1-core)_ | ModelSourceRef is the reference to the model source<br />This is either a Deployment, StatefulSet... (anything that is a template for a pod) |  | Required: \{\} <br /> |
| `serviceRef` _[ServiceReference](#servicereference)_ | ServiceRef is the reference to the service exposing the model inside the cluster<br />If not specified, a local service will be created |  | Optional: \{\} <br /> |
| `metricServiceRef` _[ServiceReference](#servicereference)_ | MetricServiceRef is the reference to the service exposing the metrics inside the cluster<br />If not specified, the model deployment will not be offloaded |  | Optional: \{\} <br /> |
| `environment` _string_ | Environment is the environment attached to the model deployment<br />If not specified, the model deployment will be deployed in the "prod" environment | production | Optional: \{\} <br /> |
| `policies` _[PolicyRef](#policyref) array_ | Policies is the list of policies to apply to the model deployment | \{  \} | Optional: \{\} <br /> |
| `serverlessConfig` _[ServerlessConfig](#serverlessconfig)_ | ServerlessConfig is the serverless configuration for the model deployment<br />If not specified, the model deployment will be deployed with a default serverless configuration |  | Optional: \{\} <br /> |
| `offloadingConfig` _[OffloadingConfig](#offloadingconfig)_ | OffloadingConfig is the offloading configuration for the model deployment<br />If not specified, the model deployment will not be offloaded |  | Optional: \{\} <br /> |


#### ModelDeploymentStatus



ModelDeploymentStatus defines the observed state of ModelDeployment



_Appears in:_
- [ModelDeployment](#modeldeployment)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `offloadingStatus` _boolean_ | OffloadingStatus is the status of the offloading<br />True if the model deployment is offloaded |  |  |
| `servingPort` _integer_ | ServingPort is the port inside the pod that the model is served on |  |  |
| `metricPort` _integer_ | MetricPort is the port inside the pod that the metrics are exposed on |  |  |
| `workspace` _string_ | Workspace is the workspace of the model deployment |  |  |
| `createdAtOnBeamlit` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta)_ | CreatedAtOnBeamlit is the time when the model deployment was created on Beamlit |  |  |
| `updatedAtOnBeamlit` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta)_ | UpdatedAtOnBeamlit is the time when the model deployment was updated on Beamlit |  |  |


#### OAuthConfig







_Appears in:_
- [AuthConfig](#authconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clientId` _string_ | ClientID is the client ID for the OAuth configuration |  | Required: \{\} <br /> |
| `clientSecret` _string_ | ClientSecret is the client secret for the OAuth configuration |  | Required: \{\} <br /> |
| `tokenUrl` _string_ | TokenURL is the token URL for the OAuth configuration |  | Required: \{\} <br /> |


#### OffloadingBehavior







_Appears in:_
- [OffloadingConfig](#offloadingconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `percentage` _integer_ | Percentage is the percentage of the requests that will be offloaded | 100 | Maximum: 100 <br />Minimum: 0 <br />Optional: \{\} <br /> |


#### OffloadingConfig







_Appears in:_
- [ModelDeploymentSpec](#modeldeploymentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `remoteBackend` _[RemoteBackend](#remotebackend)_ | RemoteBackend is the reference to the remote backend<br />By default, the model deployment will be offloaded to the default backend |  | Optional: \{\} <br /> |
| `metrics` _[MetricSpec](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#metricspec-v2-autoscaling) array_ | Metrics is the list of metrics used for offloading | \{  \} | Optional: \{\} <br /> |
| `behavior` _[OffloadingBehavior](#offloadingbehavior)_ | Behavior is the behavior of the offloading | \{  \} | Optional: \{\} <br /> |


#### PolicyRef



PolicyRef is the reference to a policy



_Appears in:_
- [ModelDeploymentSpec](#modeldeploymentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `refType` _[PolicyRefType](#policyreftype)_ | RefType is the type of the policy reference | remotePolicy | Enum: [remotePolicy localPolicy] <br />Required: \{\} <br /> |
| `name` _string_ | Name is the name of the policy |  | Optional: \{\} <br /> |


#### PolicyRefType

_Underlying type:_ _string_





_Appears in:_
- [PolicyRef](#policyref)

| Field | Description |
| --- | --- |
| `remotePolicy` |  |
| `localPolicy` |  |


#### RemoteBackend







_Appears in:_
- [OffloadingConfig](#offloadingconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `host` _string_ | Host is the host of the remote backend |  | Required: \{\} <br /> |
| `authConfig` _[AuthConfig](#authconfig)_ | AuthConfig is the authentication configuration for the remote backend |  | Optional: \{\} <br /> |
| `pathPrefix` _string_ | PathPrefix is the path prefix for the remote backend |  |  |
| `headersToAdd` _object (keys:string, values:string)_ | HeadersToAdd is the list of headers to add to the requests |  | Optional: \{\} <br /> |
| `scheme` _[SupportedScheme](#supportedscheme)_ | Scheme is the scheme for the remote backend | http | Enum: [http https] <br />Optional: \{\} <br /> |


#### ServerlessConfig







_Appears in:_
- [ModelDeploymentSpec](#modeldeploymentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `minNumReplicas` _integer_ | MinNumReplicas is the minimum number of replicas | 0 | Minimum: 0 <br />Optional: \{\} <br /> |
| `maxNumReplicas` _integer_ | MaxNumReplicas is the maximum number of replicas | 10 | Minimum: 0 <br />Optional: \{\} <br /> |
| `metric` _string_ | Metric is the metric used for scaling |  | Optional: \{\} <br /> |
| `target` _string_ | Target is the target value for the metric |  | Optional: \{\} <br /> |
| `scaleUpMinimum` _integer_ | ScaleUpMinimum is the minimum number of replicas to scale up |  | Minimum: 2 <br />Optional: \{\} <br /> |
| `scaleDownDelay` _string_ | ScaleDownDelay is the delay between scaling down |  | Optional: \{\} <br /> |
| `stableWindow` _string_ | StableWindow is the window of time to consider the number of replicas stable |  | Optional: \{\} <br /> |
| `lastPodRetentionPeriod` _string_ | LastPodRetentionPeriod is the retention period for the last pod |  | Optional: \{\} <br /> |


#### ServiceReference







_Appears in:_
- [ModelDeploymentSpec](#modeldeploymentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `targetPort` _integer_ |  |  |  |


#### SupportedScheme

_Underlying type:_ _string_





_Appears in:_
- [RemoteBackend](#remotebackend)

| Field | Description |
| --- | --- |
| `http` |  |
| `https` |  |


#### ToolDeployment



ToolDeployment is the Schema for the tooldeployments API



_Appears in:_
- [ToolDeploymentList](#tooldeploymentlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `deployment.beamlit.com/v1alpha1` | | |
| `kind` _string_ | `ToolDeployment` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ToolDeploymentSpec](#tooldeploymentspec)_ |  |  |  |
| `status` _[ToolDeploymentStatus](#tooldeploymentstatus)_ |  |  |  |


#### ToolDeploymentList



ToolDeploymentList contains a list of ToolDeployment





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `deployment.beamlit.com/v1alpha1` | | |
| `kind` _string_ | `ToolDeploymentList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[ToolDeployment](#tooldeployment) array_ |  |  |  |


#### ToolDeploymentSpec



ToolDeploymentSpec defines the desired state of ToolDeployment



_Appears in:_
- [ToolDeployment](#tooldeployment)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `foo` _string_ | Foo is an example field of ToolDeployment. Edit tooldeployment_types.go to remove/update |  |  |


#### ToolDeploymentStatus



ToolDeploymentStatus defines the observed state of ToolDeployment



_Appears in:_
- [ToolDeployment](#tooldeployment)



