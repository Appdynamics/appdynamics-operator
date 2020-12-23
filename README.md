# AppDynamics Operator

AppDynamics Operator simplifies the configuration and lifecycle management of the AppDynamics ClusterAgent and the AppDynamics Machine Agent on different Kubernetes distributions and OpenShift. The Operator encapsulates key operational knowledge on how to configure and upgrade the ClusterAgent and the Machine Agent. It knows, for example, which configuration changes are benign and do not require restart of the ClusterAgent, which minimizes unnecesary load on the cluster API server.

The Operator is implemented using [OperatorSDK](https://github.com/operator-framework/operator-sdk) and uses Kubernetes API to maintain the desired state of the custom resources that represent the ClusterAgent and the Machine Agent.
When the Operator is deployed, it creates custom resource definitions (CRDs) for 2 custom resources:

* [clusteragent](https://github.com/Appdynamics/appdynamics-operator#clusteragent-deployment), which represents the ClusterAgent.
* [infraviz](https://github.com/Appdynamics/appdynamics-operator#the-machine-agent-deployment), which represents the Machine Agent bundled with netviz and analytics.


This level of abstraction further simplifies the management of monitoring and imstrumentation and ensures granular security policy of the ClusterAgent and the Machine Agent.



## Operator deployment
Create namespace for the operator and the ClusterAgent

* Create namespace for AppDynamics components
  * Kubernetes
   `kubectl create namespace appdynamics`
  * OpenShift
   `oc new-project appdynamics --description="AppDynamics Infrastructure"`

* Create Secret `cluster-agent-secret` with the following key
  * The "controller-key" - the access key to the AppDynamics controller.

```
kubectl -n appdynamics create secret generic cluster-agent-secret \
--from-literal=controller-key="<controller-access-key>" \
```

* Update the image reference in the Operator deployment spec (deploy/cluster-agent-operator.yaml), if necessary.

The default is "docker.io/appdynamics/cluster-agent-operator:latest".


* Deploy the Operator
```
kubectl apply -f deploy/cluster-agent-operator.yaml
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

The AppDynamics Cluster Agent is a new, lightweight agent written in Golang providing APM monitoring for Kubernetes. The Cluster Agent is designed to help you understand how Kubernetes infrastructure affects your applications and business performance. With the AppDynamics Cluster Agent, you can collect metadata, metrics, and events about a Kubernetes cluster. You can query for the components in the cluster, and report metrics for those components. The Cluster Agent works across cloud platforms such as Kubernetes on AWS EKS, AKS on Azure, PKS on Pivotal and many others.
The `clusteragent` is the custom resource that the Operator works with to deploy an instance of the ClusterAgent. When a clusteragent spec is provided, the Operator will create a single replica deployment and the necessary additional resources (a configMap and a service) to support the ClusterAgent.

Here is an example of a minimalistic spec of the ClusterAgent custom resource:

```
apiVersion: appdynamics.com/v1alpha1
kind: Clusteragent
metadata:
  name: k8s-cluster-agent
  namespace: appdynamics
spec:
  appName: "<app-name>"
  controllerUrl: "<protocol>://<appdynamics-controller-host>:<port>"
  account: appdynamics-cluster-agent
  image: "<your-docker-registry>/appdynamics/cluster-agent:tag"
```

Update [the provided spec](https://github.com/Appdynamics/appdynamics-operator/blob/master/deploy/cluster-agent.yaml) with the AppDynamics account information and deploy:

```
kubectl apply -f deploy/cluster-agent.yaml
```


### Clusteragent Configuration Settings


| Parameter                 | Description                                                  | Default                    |
| ------------------------- | ------------------------------------------------------------ | -------------------------- |
| `controllerUrl`           |  Full AppDynamics Controller URL including protocol and port |       Required             |
| `account`                 |  AppDynamics Account Name                                    |       Required             |
| `image` | Cluster Agent image reference | Required |
| `imagePullSecret` | Name of the image pull secret | Optional
| `resources` |  Definitions of resources and limits for the Cluster Agent | See resource recommendations below |
| `appName` |  Name of the cluster. Displayed in the Controller UI as your cluster name. | Required |
| `nsToMonitor` | List of namespaces the Cluster Agent should initially monitor | Default is `default` |
| `eventUploadInterval` | Interval in seconds at which Kubernetes warning and state-change events are uploaded to the Controller | Default 10 sec |
| `containerRegistrationInterval` | Interval in seconds at which the Cluster Agent checks for containers and registers them with the Controller. You should only modify the default value if you want to discover running containers more frequently. The default value should be used in most environments. | Default 120 sec |
| `httpClientTimeout` | Number of seconds after which the server call is terminated if no response is received from the Controller | Default 30 sec |
| `customSSLSecret` | Provides the certificates to the Cluster Agent | Not set by default |
| `proxyUrl` | Publicly accessible hostname of the proxy (`protocol://domain:port`) | Not set by default |
| `proxyUser` | Proxy username associated with the basic authentication credentials | Not set by default |
| `metricsSyncInterval` | Interval in seconds between sending container metrics to the Controller | Default 30 sec |
| `clusterMetricsSyncInterval` | Interval in seconds between sending cluster-level metrics to the Controller | Default 60 sec |
| `metadataSyncInterval` | Interval in seconds at which metadata is collected for containers and pods | Default 60 sec |
| `podFilter` | Definitions of allowlisted/blocklisted names and labels to filter pods | Not set by default |
| `containerBatchSize` |The Cluster Agent checks for containers and registers them with the Controller. This process is known as a container registration cycle. The containers are sent to the Controller in batches, and the containerBatchSize is the maximum number of containers per batch in one cycle. | Default 5 containers |
| `podBatchSize` | The Cluster Agent checks for pods and registers them with the Controller. This process is known as a pod registration cycle. The pods are sent to the Controller in batches, and the podBatchSize is the maximum number of pods per batch in one registration cycle. | Default 6 pods |
| `metricUploadRetryCount` | Number of times metric upload action to be attempted if unsuccessful the first time | Default is 3 |
| `metricUploadRetryIntervalMilliSeconds` | Interval between consecutive metric upload retries, in milliseconds | Default is 5 |
| `logLevel` | Logging level (`INFO`, `DEBUG`, `WARN`, `TRACE`) | Default `INFO` |
| `logFileSizeMb` | Maximum file size of the log in MB | Default is 5 |
| `logFileBackups` | Maximum number of backups the log saves. When the maximum number of backups is reached, the oldest log file after the initial log file is deleted. | Default is 3 |
| `stdoutLogging` | By default, the Cluster Agent writes to a log file in the logs directory. The stdoutLogging parameter is provided so you can send logs to the container stdout as well. | Default is `"true"` |
| `runAsUser` | ID of the user that will be associated with the cluster agent pod. | By default, the agent container runs as user *appdynamics*
| `runAsGroup` | ID of the group that will be associated with the cluster agent pod. | By default, the group is *appdynamics*
| `nodeSelector` | Labels that identify nodes for scheduling of the deployment pods | Optional

Example resource limits:

```
   resources:
    limits:
      cpu: 300m
      memory: "200M"
    requests:
      cpu: 200m
      memory: "100M"
```

Here is an example of the entire spec of the Clusteragent custom resource:
```
    apiVersion: appdynamics.com/v1alpha1
    kind: Clusteragent
    metadata:
      name: k8s-cluster-agent
      namespace: appdynamics
    spec:
      appName: "<app-name>"
      controllerUrl: "<protocol>://<appdynamics-controller-host>:<port>"
      account: "<account-name>"
      # docker image info
      image: "<your-docker-registry>/appdynamics/cluster-agent:tag"
      nsToMonitor:
        - "default"
      eventUploadInterval: 10
      containerRegistrationInterval: 120
      httpClientTimeout: 30
      customSSLSecret: "<secret-name>"
      proxyUrl: "<protocol>://<domain>:<port>"
      proxyUser: "<proxy-user>"
      metricsSyncInterval: 30
      clusterMetricsSyncInterval: 60
      metadataSyncInterval: 60
      containerBatchSize: 25
      podBatchSize: 30
      metricUploadRetryCount: 3
      metricUploadRetryIntervalMilliSeconds: 5
      nodeSelector:
        kubernetes.io/os: linux
      podFilter:
        blocklistedLabels:
          - label1: value1
        allowlistedLabels:
          - label1: value1
          - label2: value2
        allowlistedNames:
          - name1
        blocklistedNames:
          - name1
          - name2
      logLevel: "INFO"
      logFileSizeMb: 5
      logFileBackups: 3
      stdoutLogging: "true"
```


## The Machine Agent deployment

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
  globalAccount: "<your-global-account-name>"
 

```

 The controller URL must be in the following format:
` <protocol>://<controller-domain>:<port> `


Use the provided specs for [Kubernetes](https://github.com/Appdynamics/appdynamics-operator/blob/master/deploy/infraviz.yaml) and [OpenShift](https://github.com/Appdynamics/appdynamics-operator/blob/master/deploy/infraviz-openshift.yaml) to deploy the Machine Agent. Make sure to update the spec with the AppDynamics account information prior to deployment.

```
kubectl apply -f deploy/infraviz.yaml
```
On OpenShift:

```
oc apply -f deploy/infraviz-openshift.yaml
```

### Mixed OS Clusters
As of v 0.5.0, the operator can deploy Machine Agent daemonsets to both Linux and Windows nodes. The deployment startegy in mixed OS clusters is determined by the value of the `nodeOS` property. This property has the following values:

* linux
* windows
* all

To deploy the Machine Agent to both OSs you have several options:

1. A single Infraviz resource. You may use a single custom resource of the type Infraviz to deploy the Machine Agent across all nodes regardless of the operating system. Set the `nodeOS ` property to `all`. You also have to provide the windows image reference in the `imageWin` property of the InfraViz spec. When the `nodeOS ` property is set to `all`, the operator will create 2 daemonsets, one for Linux nodes and the other one for Windows nodes.

2. Separate Infraviz resources. You can create 2 independent custom resources of the type Infraviz, one for the Linux nodes and the other one for the Windows nodes. In the Linux spec set the value of `nodeOS ` to `linux`. In the Windows spec, set the value of the `nodeOS ` to `windows`. In the Windows spec you also have to provide the image reference in the `imageWin` property. Make sure that the custom resource names are different.
 

The choice of the strategy depends on your customization needs. The first strategy is convenient when both Linux and Windows daemonsets share all Infraviz properties (resources, ports, etc). 
If Linux and Windows daemonsets need to be customized independently, the secod strategy is more practical.

In both secanrios the placement is driven by the `nodeSelector` value. The operator will generate a nodeSelector, if not specified by the user, and set the value of `"kubernetes.io/os"` to `linux` or `windows` depending on the value of the `nodeOS` property. You can provide additional nodeSelector values as necessary using the `nodeSelector` property of the Infraviz spec.

If `nodeOS` is not set, the operator will attempt to deploy the daemonset to all available worker nodes in the cluster, regardless of the OS. If you have a mixed OS cluster, you will need to set the `nodeSelector` property to `kubernetes.io/os: linux` or a similar depending on the labels used on the cluster nodes.
The `enableMasters` directive will be honored in all cases except when `nodeOS` property is set to `windows`. 

Updates to the infraViz with different `nodeOS` values is transparent. For example, if you have an existing Infraviz instance with `nodeOS` set to "", "linux", or "windows" and you change the `nodeOS` property to 'all', the operator will automatically create the second daemonset and update the existing one if required. There is one excpetion, when going on reverse, from 2 daemonsets down to 1 daemonset. In this case you need to delete the Infraviz resource with `nodeOS` set to `all` and deploy another Infraviz instance targeted for Linux or Windows

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
| `uniqueHostId` | Unique host ID in AppDynamics. | Optional. If not provided, the operator will set the value to `spec.nodeName`. Valid options: `spec.nodeName`, `status.hostIP` |
| `metricsLimit` | Number of metrics that the Machine Agent is allowed to post to the controller | Optional |
| `logLevel`	| Logging level (`info` or `debug`) | `info` |
| `stdoutLogging` | Determines if the logs are saved to a file or redirected to the console | "false" |
| `syslogPort` | The embedded analytics agent uses this host port to ingest syslog messages. The port is not set by default. When required, the recommended value is 5144 or based on the port availability on the host | Optional |
| `netVizPort` | When > 0, the network visibility agent will be deployed in a sidecar along with the machine agent. By default the network visibility agent works with port `3892` | Not set by default |
| `netVizImage` | Reference of the Network Agent image | "appdynamics/machine-agent-netviz:latest"
| `proxyUrl` | Url of the proxy server (protocol://domain:port") | Optional |
| `proxyUser` | Proxy user credentials (user@password) | Optional |
| `propertyBag` | A string with any other machine agent parameters | Optional
| `pks` | PKS deployment flag. Must be used on PKS environments|  Optional |
| `enableMasters` | When set to **true** server visibility will be provided for Master nodes. By default only Worker nodes are monitored. On managed Kubernetes providers the flag has no effect, as the Master plane is not accessible | Optional
| `image` | The Machine Agent image | "appdynamics/machine-agent-analytics:latest" |
| `imageWin` | The Machine Agent image for Windows nodes | Required when `nodeOS` is set to `all` or `windows` |
| `imagePullSecret` | Name of the image pull secret | Optional
| `nodeSelector` | Labels that identify nodes for scheduling of the daemonset pods | Optional |
| `tolerations` | A list of tolerations | Optional |
| `env` | List of environment variables | Optional |
| `args` | List of command arguments | Optional
| `ports` | List of ports. All declared ports will be added to the service that fronts the Machine Agent pods | Optional
| `resources` | Definitions of resources and limits for the machine agent  | See example below |
| `priorityClassName` | Name of the priority class, e.g. `system-node-critical`  | Optional. If set, the infraviz resource and all dependencies including RBAC must be deployed to kube-system namespace |

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

### Examples

* Server and Docker visibility

```
apiVersion: appdynamics.com/v1alpha1
kind: InfraViz
metadata:
  name: appd-infraviz
  namespace: appdynamics
spec:
  controllerUrl: http://saas.appdynamics.com
  image: docker.io/appdynamics/machine-agent-analytics:latest  //default
  account: customer1
  globalAccount: customer1_f1d654a0-5
  enableDockerViz: "true"
  enableMasters: true                                       // will deploy to master nodes too, if possible
  stdoutLogging: true                                       // log to console
  nodeSelector:
    kubernetes.io/os: linux                           
  resources:
    limits:
      cpu: 500m
      memory: "1G"
    requests:
      cpu: 200m
      memory: "800M"

```

* Server and Network visibility

```
apiVersion: appdynamics.com/v1alpha1
kind: InfraViz
metadata:
  name: appd-infraviz
  namespace: appdynamics
spec:
  controllerUrl: http://saas.appdynamics.com
  image: appdynamics/machine-agent-analytics:latest         
  account: customer1
  globalAccount: customer1_f1d654a0-5
  enableDockerViz: "true"
  enableMasters: true                                       
  stdoutLogging: true                                       
  netVizImage: appdynamics/machine-agent-netviz:latest      // by default
  netVizPort: 3892                                          //setting the port enables NetViz
  nodeSelector:
    kubernetes.io/os: linux
  resources:
    limits:
      cpu: 500m
      memory: "1G"
    requests:
      cpu: 200m
      memory: "800M"

```

* Customizing ports. Syslog, Analytics

```
apiVersion: appdynamics.com/v1alpha1
kind: InfraViz
metadata:
  name: appd-infraviz
  namespace: appdynamics
spec:
  controllerUrl: http://saas.appdynamics.com
  image: appdynamics/machine-agent-analytics:latest         
  account: customer1
  globalAccount: customer1_f1d654a0-5
  enableDockerViz: "true"
  enableMasters: true                                       
  stdoutLogging: true                                       
  syslogPort: 5144
  biqPort: 9090                                             // by default
  nodeSelector:
    kubernetes.io/os: linux
  resources:
    limits:
      cpu: 500m
      memory: "1G"
    requests:
      cpu: 200m
      memory: "800M"

```

* Deploy to Windows nodes

```
apiVersion: appdynamics.com/v1alpha1
kind: InfraViz
metadata:
  name: appd-infraviz
  namespace: appdynamics
spec:
  controllerUrl: http://saas.appdynamics.com
  account: customer1
  globalAccount: customer1_f1d654a0-5
  imageWin: docker.io/appdynamics/machine-agent-analytics:20.6.0-win-ltsc2019  
  nodeOS: windows
  enableMasters: false                                 
  stdoutLogging: true                                 
  resources:
    limits:
      cpu: 500m
      memory: "1G"
    requests:
      cpu: 200m
      memory: "800M"

```

* Deploy to Mixed OS Clusters

```
apiVersion: appdynamics.com/v1alpha1
kind: InfraViz
metadata:
  name: appd-infraviz
  namespace: appdynamics
spec:
  controllerUrl: http://saas.appdynamics.com
  account: customer1
  globalAccount: customer1_f1d654a0-5
  image: "docker.io/appdynamics/machine-agent-analytics:latest"
  imageWin: docker.io/appdynamics/machine-agent-analytics:20.6.0-win-ltsc2019  
  nodeOS: all
  enableMasters: true                                 
  stdoutLogging: true                                 
  resources:
    limits:
      cpu: 500m
      memory: "1G"
    requests:
      cpu: 200m
      memory: "800M"

```

* Deploy to Infra nodes 

```
apiVersion: appdynamics.com/v1alpha1
kind: InfraViz
metadata:
  name: appd-infraviz
  namespace: appdynamics
spec:
  controllerUrl: http://saas.appdynamics.com
  image: docker.io/appdynamics/machine-agent-analytics:latest
  account: customer1
  globalAccount: customer1_f1d654a0-5
  enableDockerViz: "true"
  enableMasters: true                                 
  stdoutLogging: true                                 
  nodeSelector:
    kubernetes.io/os: linux 
   tolerations:
   - effect: NoSchedule
     key: node-role.kubernetes.io/infra
     operator: Exists                          
  resources:
    limits:
      cpu: 500m
      memory: "1G"
    requests:
      cpu: 200m
      memory: "800M"

```

