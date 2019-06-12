# AppDynamics Operator

AppDynamics Operator simplifies the configuration and lifecycle management of the AppDynamics ClusterAgent and the AppDynamics Machine Agent on different Kubernetes distributions and OpenShift. The Operator encapsulates key operational knowledge on how to configure and upgrade the ClusterAgent and the Machine Agent. It knows, for example, which configuration changes are benign and do not require restart of the ClusterAgent, which minimizes unnecesary load on the cluster API server.

The Operator is implemented using [OperatorSDK](https://github.com/operator-framework/operator-sdk) and uses Kubernetes API to maintain the desired state of the custom resources that represent the ClusterAgent and the Machine Agent. 
When the Operator is deployed, it creates custom resource definitions (CRDs) for 2 custom resources: 

* `clusteragent`, which represents the ClusterAgent. 
*  `infraviz`, which represents the Machine Agent bundled with netviz and analytics.


This level of abstraction further simplifies the management of monitoring and imstrumentation and ensures granular security policy of the ClusterAgent and the Machine Agent.



## Operator deployment
Create namespace for the operator and the ClusterAgent

* Create namespace for AppDynamics components
  * Kubernetes
   `kubectl create namespace appdynamics`
  * OpenShift
   `oc new-project appdynamics --description="AppDynamics Infrastructure"`

* Create Secret `cluster-agent-secret` (deploy/cluster-agent/cluster-agent-secret.yaml). 
  * The "api-user" key with the AppDynamics user account information is required. It needs to be in the following format `<username>@<account>:<password>`, e.g ` user@customer1:123 `. 
  * The "controller-key" is required for the deployments of the `infraviz` custom resource (The Machine Agent). It is the access key to the AppDynamics controller, which can be obtained from the controller UI. For `clusteragent` deployments this key is optional.
  * The "event-key" is optional. It is only used by the `clusteragent` resource. This value represents an Event API key, which allows posting resource snapshots to the event analytics.  If not specified, it will be automatically generated by the ClusterAgent

```
kubectl -n appdynamics create secret generic cluster-agent-secret \
--from-literal=api-user="" \
--from-literal=controller-key="" \
--from-literal=event-key="" \
```

* Update the image reference in the Operator deployment spec (deploy/cluster-agent-operator.yaml), if necessary. 

The default is "docker.io/appdynamics/cluster-agent-operator:latest".


* Deploy the ClusterAgent
```
kubectl create -f deploy/cluster-agent-operator.yaml
```


### Images

By default "docker.io/appdynamics/cluster-agent-operator:latest" is used.

[AppDynamics images](https://access.redhat.com/containers/#/product/f5e13e601dc05eaa) are also available from [Red Hat Container Catalog](https://access.redhat.com/containers/). 

To enable pulling,  create a secret in the ClusterAgent namespace. In this example, namespace **appdynamics** is used and appdynamics-operator account is linked to the secret.

```
$ oc -n appdynamics create secret docker-registry redhat-connect 
--docker-server=registry.connect.redhat.com 
--docker-username=REDHAT_CONNECT_USERNAME 
--docker-password=REDHAT_CONNECT_PASSWORD --docker-email=unused
$ oc -n appdynamics secrets link appdynamics-operator redhat-connect 
--for=pull 
```


## ClusterAgent deployment

The `clusteragent` is the custom resource that the Operator works with to deploy an instance of the ClusterAgent. When a clusteragent spec is provided, the Operator will create a single replica deployment and the necessary additional resources (a configMap and a service) to support the ClusterAgent.

Here is an example of a minimalistic spec of the ClusterAgent custom resource:

```
apiVersion: appdynamics.com/v1alpha1
kind: Clusteragent
metadata:
  name: K8s-Cluster-Agent
spec:
  controllerUrl: "<protocol>://<controller-url>:<port>"
```
Update controller URL in the configMap (deploy/cluster-agent/cluster-agent-config.yaml). 
The controller URL must be in the following format:
` <protocol>://<controller-domain>:<port> `


Here is another example with imstrumentation rules:

```
apiVersion: appdynamics.com/v1alpha1
kind: Clusteragent
metadata:
  name: local-k8s
  namespace: appdynamics
spec:
  controllerUrl: "https://appd-controller.com"
  appName: MyClusterAgent
  appDJavaAttachImage: "appdynamics/java-agent:latest"
  appDDotNetAttachImage: "appdynamics/dotnet-core-agent:latest"
  instrumentationMethod: "mountEnv" 
  nsToInstrument:
  - dev
  - ad-devops
  instrumentRule:
  - matchString: 
    - "client-api" # only deployments with matching name or metadata will be instrumented
    namespaces:
    - ns1
	appDAppLabel: "appName"  # this value will be used as Application name
	appDTierLabel: "tierName" # this value will be used as Tier name
	version: "appdynamics/java-agent:4.5.6" # version override
	biq: "sidecar" # add analytics agent in the sidecar
```

For other instrumentation examples, refer to this [documentation](https://github.com/Appdynamics/cluster-agent/blob/master/docs/instrumentation.md)

## The MachineAgent deployment

Appdynamics operator can be used to enable server and network visibility with AppDynamics Machine agent. 
The operator works with custom resource `infraviz` to deploy the AppDynamics Machine Agent daemon set.

Here is an example of a minimalistic `infraviz` spec with the required parameters:

```
apiVersion: appdynamics.com/v1alpha1
kind: InfraViz
metadata:
  name: appd-infraviz
  namespace: appdynamics
spec:
  controllerUrl: "https://appd-controller.com"
  image: "docker.io/appdynamics/machine-agent-analytics:latest"
  account: "<your-account-name>"
  globalAccount: "<your-global-account-name"
 
```
 
 The controller URL must be in the following format:
` <protocol>://<controller-domain>:<port> `

### Infraviz Configuration Settings


| Parameter                 | Description                                                  | Default                    |
| ------------------------- | ------------------------------------------------------------ | -------------------------- |
| `controllerUrl`           |  Url of the AppDynamics controller                            |       Required             |
| `account`                 |  AppDynamics Account Name                                    |       Required             |
| `globalAccount`          |  Global Account Name                            |     Required  |
| `eventServiceUrl`   | Event Service Endpoint | Optional |
| `enableContainerHostId` | Flag that determines how container names are derived (pod name vs container id) | "true" |
| `enableServerViz` |  Enable Server Visibility | "true" |
| `enableDockerViz` | Enable Docker Container Visibiltiy | "true" |
| `uniqieHostId` | Unique host ID in AppDynamics. | Optional |
| `metricsLimit` | Number of metrics that the Machine Agent is allowed to post to the controller | Optional |
| `logLevel`	| Logging level (`info` or `debug`) | `info` |
| `stdoutLogging` | Determines if the logs are saved to a file or redirected to the console | "false" |
| `proxyUrl` | Url of the proxy server (protocol://domain:port") | Optional |
| `proxyUser` | Proxy user credentials (user@password) | Optional |
| `propertyBag` | A string with any other machine agent parameters | Optional
| `image` | The Machine Agent image | "appdynamics/machine-agent-analytics:latest" |
| `nodeSelector` | A set of nodes to deploy the daemon set pods to | Optional |
| `tolerations` | A list of tolerations | Optional |
| `env` | List of environment variables | Optional |
| `args` | List of command arguments | Optional
| `resources` | Definitions of resources and limits for the machine agent  | See example below |

Example resource limits:

```
   resources:
    limits:
      cpu: 600m
      memory: "1G"
    requests: 
      cpu: 300m
      memory: "800M"
```


## Deploy ClusterAgent and Machine Agent together

An [example](https://github.com/Appdynamics/appdynamics-operator/blob/master/deploy/crds/bundle.yaml) spec to deploy both custom resources.