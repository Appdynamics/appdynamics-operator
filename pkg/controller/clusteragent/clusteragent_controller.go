package clusteragent

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	"strconv"
	"strings"

	"time"

	appdynamicsv1alpha1 "github.com/Appdynamics/appdynamics-operator/pkg/apis/appdynamics/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_clusteragent")

const (
	AGENT_SECRET_NAME           string = "cluster-agent-secret"
	AGENT_PROXY_SECRET_NAME     string = "cluster-agent-proxy-secret"
	AGENT_CONFIG_NAME           string = "cluster-agent-config"
	AGENT_MON_CONFIG_NAME       string = "cluster-agent-mon"
	AGENT_LOG_CONFIG_NAME       string = "cluster-agent-log"
	INSTRUMENTATION_CONFIG_NAME string = "instrumentation-config"
	AGENT_SSL_CONFIG_NAME       string = "appd-agent-ssl-config"
	AGENT_SSL_CRED_STORE_NAME   string = "appd-agent-ssl-store"
	OLD_SPEC                    string = "cluster-agent-spec"
	ALLOWLISTED                 string = "allowlisted"
	BLOCKLISTED                 string = "blocklisted"

	ENV_INSTRUMENTATION     string = "Env"
	MATCH_ALL_REGEX         string = ".*"
	NO_INSTRUMENTATION             = "None"
	JAVA_LANGUAGE           string = "java"
	DOTNET_LANGUAGE         string = "dotnetcore"
	NODEJS_LANGUAGE         string = "nodejs"
	AppDJavaAttachImage            = "docker.io/appdynamics/java-agent:latest"
	AppDDotNetAttachImage          = "docker.io/appdynamics/dotnet-core-agent:latest"
	AppDNodeJSAttachImage          = "docker.io/appdynamics/nodejs-agent:20.8.0-stretch-slimv10"
	AGENT_MOUNT_PATH               = "/opt/appdynamics"
	IMAGE_PULL_POLICY              = "IfNotPresent"
	Deployment                     = "Deployment"
	JAVA_TOOL_OPTIONS              = "JAVA_TOOL_OPTIONS"
	FIRST                          = "first"
	MANUAL_APPNAME_STRATEGY        = "manual"
)

// Add creates a new Clusteragent Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileClusteragent{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("clusteragent-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Clusteragent
	err = c.Watch(&source.Kind{Type: &appdynamicsv1alpha1.Clusteragent{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Deployment and requeue the owner Clusteragent
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appdynamicsv1alpha1.Clusteragent{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileClusteragent{}

// ReconcileClusteragent reconciles a Clusteragent object
type ReconcileClusteragent struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

func (r *ReconcileClusteragent) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Clusteragent...")

	// Fetch the Clusteragent instance
	clusterAgent := &appdynamicsv1alpha1.Clusteragent{}
	err := r.client.Get(context.TODO(), request.NamespacedName, clusterAgent)
	reqLogger.Info("Retrieved cluster agent.", "Image", clusterAgent.Spec.Image)
	if err != nil {
		if errors.IsNotFound(err) {
			// Return and don't requeue
			reqLogger.Info("Cluster Agent resource not found. The object must be deleted")
			r.cleanUp(nil)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get Cluster Agent")
		return reconcile.Result{}, err
	}
	reqLogger.Info("Cluster agent spec exists. Checking the corresponding deployment...")
	// Check if the agent already exists in the namespace
	existingDeployment := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: clusterAgent.Name, Namespace: clusterAgent.Namespace}, existingDeployment)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Removing the old instance of the configMap...")
		r.cleanUp(clusterAgent)
		reqLogger.Info("Cluster agent deployment does not exist. Creating...")
		reqLogger.Info("Checking the secret...")
		secret, esecret := r.ensureSecret(clusterAgent)
		if esecret != nil {
			reqLogger.Error(esecret, "Failed to create new Cluster Agent due to secret", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
			return reconcile.Result{}, esecret
		}
		reqLogger.Info("Checking the config map")
		econfig := r.ensureConfigMap(clusterAgent, secret, true)
		if econfig != nil {
			reqLogger.Error(econfig, "Failed to create new Cluster Agent due to config map", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
			return reconcile.Result{}, econfig
		}
		// Define a new deployment for the cluster agent
		dep := r.newAgentDeployment(clusterAgent, secret)
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return reconcile.Result{}, err
		}
		reqLogger.Info("Deployment created successfully. Done")
		r.updateStatus(clusterAgent)
		return reconcile.Result{}, nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment")
		return reconcile.Result{}, err
	}

	reqLogger.Info("Cluster agent deployment exists. Checking for deltas with the current state...")
	// Ensure the deployment spec matches the new spec
	// Differentiate between breaking changes and benign updates
	// Check if secret has been recreated. if yes, restart pod
	secret, errsecret := r.ensureSecret(clusterAgent)
	if errsecret != nil {
		reqLogger.Error(errsecret, "Failed to get cluster agent config secret", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
		return reconcile.Result{}, errsecret
	}

	reqLogger.Info("Retrieving agent config map", "Deployment.Namespace", clusterAgent.Namespace)
	econfig := r.ensureConfigMap(clusterAgent, secret, false)
	if econfig != nil {
		reqLogger.Error(econfig, "Failed to obtain cluster agent config map", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
		return reconcile.Result{}, econfig
	}

	breaking, updateDeployment := r.hasBreakingChanges(clusterAgent, existingDeployment, secret)

	if breaking {
		fmt.Println("Breaking changes detected. Restarting the cluster agent pod...")

		saveOrUpdateClusterAgentSpecAnnotation(clusterAgent, existingDeployment)
		errUpdate := r.client.Update(context.TODO(), existingDeployment)
		if errUpdate != nil {
			reqLogger.Error(errUpdate, "Failed to update cluster agent", "clusterAgent.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
			return reconcile.Result{}, errUpdate
		}

		errRestart := r.restartAgent(clusterAgent)
		if errRestart != nil {
			reqLogger.Error(errRestart, "Failed to restart cluster agent", "clusterAgent.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
			return reconcile.Result{}, errRestart
		}
	} else if updateDeployment {
		fmt.Println("Breaking changes detected. Updating the the cluster agent deployment...")
		err = r.client.Update(context.TODO(), existingDeployment)
		if err != nil {
			reqLogger.Error(err, "Failed to update ClusterAgent Deployment", "Deployment.Namespace", existingDeployment.Namespace, "Deployment.Name", existingDeployment.Name)
			return reconcile.Result{}, err
		}
	} else {

		reqLogger.Info("No breaking changes.", "clusterAgent.Namespace", clusterAgent.Namespace)

		statusErr := r.updateStatus(clusterAgent)
		if statusErr == nil {
			reqLogger.Info("Status updated. Exiting reconciliation loop.")
		} else {
			reqLogger.Info("Status not updated. Exiting reconciliation loop.")
		}
		return reconcile.Result{}, nil

	}

	reqLogger.Info("Exiting reconciliation loop.")
	return reconcile.Result{RequeueAfter: 60 * time.Second}, nil
}

func (r *ReconcileClusteragent) updateStatus(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {
	clusterAgent.Status.LastUpdateTime = metav1.Now()

	if errInstance := r.client.Update(context.TODO(), clusterAgent); errInstance != nil {
		return fmt.Errorf("Unable to update clusteragent instance. %v", errInstance)
	}
	log.Info("ClusterAgent instance updated successfully", "clusterAgent.Namespace", clusterAgent.Namespace, "Date", clusterAgent.Status.LastUpdateTime)

	err := r.client.Status().Update(context.TODO(), clusterAgent)
	if err != nil {
		log.Error(err, "Failed to update cluster agent status", "clusterAgent.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
	} else {
		log.Info("ClusterAgent status updated successfully", "clusterAgent.Namespace", clusterAgent.Namespace, "Date", clusterAgent.Status.LastUpdateTime)
	}
	return err
}

func (r *ReconcileClusteragent) hasBreakingChanges(clusterAgent *appdynamicsv1alpha1.Clusteragent, existingDeployment *appsv1.Deployment, secret *corev1.Secret) (bool, bool) {
	breakingChanges := false
	updateDeployment := false

	fmt.Println("Checking for breaking changes...")

	if existingDeployment.Annotations != nil {
		if oldJson, ok := existingDeployment.Annotations[OLD_SPEC]; ok && oldJson != "" {
			var oldSpec appdynamicsv1alpha1.Clusteragent
			errJson := json.Unmarshal([]byte(oldJson), &oldSpec)
			if errJson != nil {
				log.Error(errJson, "Unable to retrieve the old spec from annotations", "clusterAgent.Namespace", clusterAgent.Namespace, "clusterAgent.Name", clusterAgent.Name)
			}

			if oldSpec.Spec.ControllerUrl != clusterAgent.Spec.ControllerUrl || oldSpec.Spec.Account != clusterAgent.Spec.Account {
				breakingChanges = true
			}
		}
	}

	if clusterAgent.Spec.Image != "" && existingDeployment.Spec.Template.Spec.Containers[0].Image != clusterAgent.Spec.Image {
		fmt.Printf("Image changed from has changed: %s	to	%s. Updating....\n", existingDeployment.Spec.Template.Spec.Containers[0].Image, clusterAgent.Spec.Image)
		existingDeployment.Spec.Template.Spec.Containers[0].Image = clusterAgent.Spec.Image
		return false, true
	}

	return breakingChanges, updateDeployment
}

func (r *ReconcileClusteragent) ensureSecret(clusterAgent *appdynamicsv1alpha1.Clusteragent) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	secretName := AGENT_SECRET_NAME
	if clusterAgent.Spec.AccessSecret != "" {
		secretName = clusterAgent.Spec.AccessSecret
	}

	key := client.ObjectKey{Namespace: clusterAgent.Namespace, Name: secretName}
	err := r.client.Get(context.TODO(), key, secret)
	if err != nil && errors.IsNotFound(err) {
		fmt.Printf("Required secret %s not found. An empty secret will be created, but the clusteragent will not start until at least the 'controller-key' key of the secret has a valid value", secretName)

		secret = &corev1.Secret{
			Type: corev1.SecretTypeOpaque,
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: clusterAgent.Namespace,
			},
		}

		secret.StringData = make(map[string]string)
		secret.StringData["controller-key"] = ""

		errCreate := r.client.Create(context.TODO(), secret)
		if errCreate != nil {
			fmt.Printf("Unable to create secret. %v\n", errCreate)
			return nil, fmt.Errorf("Unable to get secret for cluster-agent. %v", errCreate)
		} else {
			fmt.Printf("Secret created. %s\n", secretName)
			errLoad := r.client.Get(context.TODO(), key, secret)
			if errLoad != nil {
				fmt.Printf("Unable to reload secret. %v\n", errLoad)
				return nil, fmt.Errorf("Unable to get secret for cluster-agent. %v", err)
			}
		}
	} else if err != nil {
		return nil, fmt.Errorf("Unable to get secret for cluster-agent. %v", err)
	}

	return secret, nil
}

func (r *ReconcileClusteragent) cleanUp(clusterAgent *appdynamicsv1alpha1.Clusteragent) {
	namespace := "appdynamics"
	if clusterAgent != nil {
		namespace = clusterAgent.Namespace
	}
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: "cluster-agent-config", Namespace: namespace}, cm)
	if err != nil && errors.IsNotFound(err) {
		log.Info("The old instance of the configMap does not exist")
		return
	}

	if err == nil && cm != nil {
		err = r.client.Delete(context.TODO(), cm)
		if err != nil {
			log.Info("Unable to delete the old instance of the configMap", err)
		} else {
			log.Info("The old instance of the configMap deleted")
		}
	} else {
		log.Error(err, "Unable to retrieve the old instance of the configmap")
	}
}

func (r *ReconcileClusteragent) ensureConfigMap(clusterAgent *appdynamicsv1alpha1.Clusteragent, secret *corev1.Secret, create bool) error {
	setClusterAgentConfigDefaults(clusterAgent)
	setInstrumentationAgentDefaults(clusterAgent)

	err := r.ensureAgentMonConfig(clusterAgent)
	if err != nil {
		return err
	}

	err = r.ensureAgentConfig(clusterAgent)
	if err != nil {
		return err
	}

	err = r.ensureInstrumentationConfig(clusterAgent)
	if err != nil {
		return err
	}

	err = r.ensureLogConfig(clusterAgent)

	return err

}
func validateControllerUrl(controllerUrl string) (error, string, uint16, string) {
	if strings.Contains(controllerUrl, "http") {
		arr := strings.Split(controllerUrl, ":")
		if len(arr) > 3 || len(arr) < 2 {
			return fmt.Errorf("Controller Url is invalid. Use this format: protocol://url:port"), "", 0, ""
		}
		protocol := arr[0]
		controllerDns := strings.TrimLeft(arr[1], "//")
		controllerPort := 0
		if len(arr) != 3 {
			if strings.Contains(protocol, "s") {
				controllerPort = 443
			} else {
				controllerPort = 80
			}
		} else {
			port, errPort := strconv.Atoi(arr[2])
			if errPort != nil {
				return fmt.Errorf("Controller port is invalid. %v", errPort), "", 0, ""
			}
			controllerPort = port
		}

		ssl := "false"
		if strings.Contains(protocol, "s") {
			ssl = "true"
		}
		return nil, controllerDns, uint16(controllerPort), ssl
	} else {
		return fmt.Errorf("Controller Url is invalid. Use this format: protocol://dns:port"), "", 0, ""
	}
}

func (r *ReconcileClusteragent) ensureAgentConfig(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_CONFIG_NAME, Namespace: clusterAgent.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load Agent  configMap. %v", err)
	}

	errVal, controllerDns, port, sslEnabled := validateControllerUrl(clusterAgent.Spec.ControllerUrl)
	if errVal != nil {
		return errVal
	}

	portVal := strconv.Itoa(int(port))

	cm.Name = AGENT_CONFIG_NAME
	cm.Namespace = clusterAgent.Namespace
	cm.Data = make(map[string]string)
	cm.Data["APPDYNAMICS_AGENT_ACCOUNT_NAME"] = clusterAgent.Spec.Account
	cm.Data["APPDYNAMICS_CONTROLLER_HOST_NAME"] = controllerDns
	cm.Data["APPDYNAMICS_CONTROLLER_PORT"] = portVal
	cm.Data["APPDYNAMICS_CONTROLLER_SSL_ENABLED"] = sslEnabled
	cm.Data["APPDYNAMICS_CLUSTER_NAME"] = clusterAgent.Spec.AppName

	cm.Data["APPDYNAMICS_AGENT_PROXY_URL"] = clusterAgent.Spec.ProxyUrl
	cm.Data["APPDYNAMICS_AGENT_PROXY_USER"] = clusterAgent.Spec.ProxyUser

	cm.Data["APPDYNAMICS_CLUSTER_MONITORED_NAMESPACES"] = strings.Join(clusterAgent.Spec.NsToMonitor, ",")
	cm.Data["APPDYNAMICS_CLUSTER_EVENT_UPLOAD_INTERVAL"] = strconv.Itoa(clusterAgent.Spec.EventUploadInterval)
	cm.Data["APPDYNAMICS_CLUSTER_CONTAINER_REGISTRATION_INTERVAL"] = strconv.Itoa(clusterAgent.Spec.ContainerRegistrationInterval)
	cm.Data["APPDYNAMICS_CLUSTER_HTTP_CLIENT_TIMEOUT_INTERVAL"] = strconv.Itoa(clusterAgent.Spec.HttpClientTimeout)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap created")
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap updated")
	}

	return nil
}

func (r *ReconcileClusteragent) ensureAgentMonConfig(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {
	yml := fmt.Sprintf(`metric-collection-interval-seconds: %d
cluster-metric-collection-interval-seconds: %d
metadata-collection-interval-seconds: %d
container-registration-batch-size: %d
pod-registration-batch-size: %d
metric-upload-retry-count: %d
metric-upload-retry-interval-milliseconds: %d
max-pods-to-register-count: %d
max-pod-logs-tail-lines-count: %d
pod-filter: %s`, clusterAgent.Spec.MetricsSyncInterval, clusterAgent.Spec.ClusterMetricsSyncInterval, clusterAgent.Spec.MetadataSyncInterval,
		clusterAgent.Spec.ContainerBatchSize, clusterAgent.Spec.PodBatchSize,
		clusterAgent.Spec.MetricUploadRetryCount, clusterAgent.Spec.MetricUploadRetryIntervalMilliSeconds,
		clusterAgent.Spec.MaxPodsToRegisterCount, clusterAgent.Spec.MaxPodLogsTailLinesCount,
		createPodFilterString(clusterAgent))

	if clusterAgent.Spec.NsToMonitorRegex != "" {
		yml = fmt.Sprintf(`%s
ns-to-monitor-regex: %s`, yml, clusterAgent.Spec.NsToMonitorRegex)
	}
	if clusterAgent.Spec.NsToExcludeRegex != "" {
		yml = fmt.Sprintf(`%s
ns-to-exclude-regex: %s`, yml, clusterAgent.Spec.NsToExcludeRegex)
	}

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_MON_CONFIG_NAME, Namespace: clusterAgent.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load Agent  configMap. %v", err)
	}

	cm.Name = AGENT_MON_CONFIG_NAME
	cm.Namespace = clusterAgent.Namespace
	cm.Data = make(map[string]string)
	cm.Data["agent-monitoring.yml"] = string(yml)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap created")
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap updated")
	}

	return nil
}

func (r *ReconcileClusteragent) ensureInstrumentationConfig(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {
	yml := fmt.Sprintf(`instrumentation-method: %s
default-match-string: %s
default-label-match: %s
image-info: %v
default-language: %s
ns-to-instrument: %s
resources-to-instrument: %s
default-env: %s
default-custom-agent-config: %s
default-app-name: %s
app-name-label: %s
instrument-container: %s
container-match-string: %s
netviz-info: %v
run-as-user: %d
run-as-group: %d
app-name-strategy: %s
number-of-task-workers: %d
default-analytics-host: %s
default-analytics-port: %d
default-analytics-ssl-enabled: %t
instrumentation-rules: %v`, clusterAgent.Spec.InstrumentationMethod, clusterAgent.Spec.DefaultInstrumentMatchString,
		mapToJsonString(clusterAgent.Spec.DefaultLabelMatch), imageInfoMapToJsonString(clusterAgent.Spec.ImageInfoMap), clusterAgent.Spec.DefaultInstrumentationTech,
		clusterAgent.Spec.NsToInstrumentRegex, strings.Join(clusterAgent.Spec.ResourcesToInstrument, ","), clusterAgent.Spec.DefaultEnv,
		clusterAgent.Spec.DefaultCustomConfig, clusterAgent.Spec.DefaultAppName, clusterAgent.Spec.AppNameLabel, clusterAgent.Spec.InstrumentContainer,
		clusterAgent.Spec.DefaultContainerMatchString, netvizInfoToJsonString(clusterAgent.Spec.NetvizInfo), clusterAgent.Spec.RunAsUser, clusterAgent.Spec.RunAsGroup,
		clusterAgent.Spec.AppNameStrategy, clusterAgent.Spec.NumberOfTaskWorkers, clusterAgent.Spec.DefaultAnalyticsHost, clusterAgent.Spec.DefaultAnalyticsPort, clusterAgent.Spec.DefaultAnalyticsSslEnabled, instrumentationRulesToJsonString(clusterAgent.Spec.InstrumentationRules))

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: INSTRUMENTATION_CONFIG_NAME, Namespace: clusterAgent.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load Agent  configMap. %v", err)
	}

	cm.Name = INSTRUMENTATION_CONFIG_NAME
	cm.Namespace = clusterAgent.Namespace
	cm.Data = make(map[string]string)
	cm.Data["instrumentation.yml"] = string(yml)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap created")
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update Agent configMap. %v", e)
		}
		fmt.Println("Agent Configmap updated")
	}

	return nil
}

func (r *ReconcileClusteragent) ensureLogConfig(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {
	yml := fmt.Sprintf(`log-level: %s
max-filesize-mb: %d
max-backups: %d
write-to-stdout: %s`, clusterAgent.Spec.LogLevel, clusterAgent.Spec.LogFileSizeMb, clusterAgent.Spec.LogFileBackups,
		strings.ToLower(clusterAgent.Spec.StdoutLogging))

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_LOG_CONFIG_NAME, Namespace: clusterAgent.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)

	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load Agent Log configMap. %v", err)
	}

	cm.Name = AGENT_LOG_CONFIG_NAME
	cm.Namespace = clusterAgent.Namespace
	cm.Data = make(map[string]string)
	cm.Data["logger-config.yml"] = string(yml)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create Agent Log configMap. %v", e)
		}
		fmt.Println("Log Configmap created")
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update Agent Log configMap. %v", e)
		}
		fmt.Println("Log Configmap updated")
	}

	return nil
}

func (r *ReconcileClusteragent) ensureSSLConfig(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {

	if clusterAgent.Spec.AgentSSLStoreName == "" {
		return nil
	}

	//verify that AGENT_SSL_CRED_STORE_NAME config map exists.
	//it will be copied into the respecive namespace for instrumentation

	existing := &corev1.ConfigMap{}
	errCheck := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_SSL_CRED_STORE_NAME, Namespace: clusterAgent.Namespace}, existing)

	if errCheck != nil && errors.IsNotFound(errCheck) {
		return fmt.Errorf("Custom SSL store is requested, but the expected configMap %s with the trusted certificate store not found. Put the desired certificates into the cert store and create the configMap in the %s namespace", AGENT_SSL_CRED_STORE_NAME, clusterAgent.Namespace)
	} else if errCheck != nil {
		return fmt.Errorf("Unable to validate the expected configMap %s with the trusted certificate store. Put the desired certificates into the cert store and create the configMap in the %s namespace", AGENT_SSL_CRED_STORE_NAME, clusterAgent.Namespace)
	}

	return nil
}

func (r *ReconcileClusteragent) newAgentDeployment(clusterAgent *appdynamicsv1alpha1.Clusteragent, secret *corev1.Secret) *appsv1.Deployment {
	if clusterAgent.Spec.Image == "" {
		clusterAgent.Spec.Image = "appdynamics/cluster-agent:latest"
	}

	if clusterAgent.Spec.ServiceAccountName == "" {
		clusterAgent.Spec.ServiceAccountName = "appdynamics-operator"
	}

	secretName := AGENT_SECRET_NAME
	if clusterAgent.Spec.AccessSecret != "" {
		secretName = clusterAgent.Spec.AccessSecret
	}

	resLimit := corev1.ResourceList{}
	resLimit[corev1.ResourceCPU] = resource.MustParse("1250m")
	resLimit[corev1.ResourceMemory] = resource.MustParse("300Mi")

	resRequest := corev1.ResourceList{}
	resRequest[corev1.ResourceCPU] = resource.MustParse("750m")
	resRequest[corev1.ResourceMemory] = resource.MustParse("150Mi")

	resRequirements := corev1.ResourceRequirements{Requests: resRequest, Limits: resLimit}

	if len(clusterAgent.Spec.Resources.Limits) == 0 && len(clusterAgent.Spec.Resources.Requests) == 0 {
		clusterAgent.Spec.Resources = resRequirements
	}

	fmt.Printf("Building deployment spec for image %s\n", clusterAgent.Spec.Image)
	ls := labelsForClusteragent(clusterAgent)
	var replicas int32 = 1
	var optional = true
	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterAgent.Name,
			Namespace: clusterAgent.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					NodeSelector:       clusterAgent.Spec.NodeSelector,
					ServiceAccountName: clusterAgent.Spec.ServiceAccountName,
					Tolerations:        clusterAgent.Spec.Tolerations,
					Containers: []corev1.Container{{
						EnvFrom: []corev1.EnvFromSource{{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: AGENT_CONFIG_NAME}}}},
						Env: []corev1.EnvVar{
							{
								Name: "APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
										Key:                  "controller-key",
									},
								},
							},
							{
								Name: "APPDYNAMICS_USER_CREDENTIALS",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
										Key:                  "api-user",
										Optional:             &optional,
									},
								},
							},
							{
								Name: "APPDYNAMICS_AGENT_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "metadata.namespace",
									},
								},
							},
							{
								Name: "NODE_NAME",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										FieldPath: "spec.nodeName",
									},
								},
							},
						},
						Image:           clusterAgent.Spec.Image,
						ImagePullPolicy: corev1.PullAlways,
						Name:            "cluster-agent",
						Resources:       clusterAgent.Spec.Resources,
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "agent-mon-config",
							MountPath: "/opt/appdynamics/cluster-agent/config/agent-monitoring/",
						},
							{
								Name:      "agent-log",
								MountPath: "/opt/appdynamics/cluster-agent/config/logging/",
							},
							{
								Name:      "instrumentation-config",
								MountPath: "/opt/appdynamics/cluster-agent/config/instrumentation",
							}},
					}},
					Volumes: []corev1.Volume{{
						Name: "agent-mon-config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: AGENT_MON_CONFIG_NAME,
								},
							},
						},
					},
						{
							Name: "instrumentation-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: INSTRUMENTATION_CONFIG_NAME,
									},
								},
							},
						},
						{
							Name: "agent-log",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: AGENT_LOG_CONFIG_NAME,
									},
								},
							},
						}},
				},
			},
		},
	}

	//mount custom SSL cert if necessary (can contain controller and proxy SSL certs)
	if clusterAgent.Spec.CustomSSLSecret != "" {
		sslVol := corev1.Volume{
			Name: "agent-ssl-cert",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: clusterAgent.Spec.CustomSSLSecret,
				},
			},
		}
		dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, sslVol)

		sslMount := corev1.VolumeMount{
			Name:      "agent-ssl-cert",
			MountPath: "/opt/appdynamics/ssl",
		}
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = append(dep.Spec.Template.Spec.Containers[0].VolumeMounts, sslMount)
	}

	customSSLSecret := corev1.EnvVar{
		Name:  "APPDYNAMICS_CUSTOM_SSL_SECRET",
		Value: clusterAgent.Spec.CustomSSLSecret,
	}
	dep.Spec.Template.Spec.Containers[0].Env = append(dep.Spec.Template.Spec.Containers[0].Env, customSSLSecret)

	//security context override
	var podSec *corev1.PodSecurityContext = nil
	if clusterAgent.Spec.RunAsUser > 0 {
		podSec = &corev1.PodSecurityContext{RunAsUser: &clusterAgent.Spec.RunAsUser}
		if clusterAgent.Spec.RunAsGroup > 0 {
			podSec.RunAsGroup = &clusterAgent.Spec.RunAsGroup
		}
		if clusterAgent.Spec.FSGroup > 0 {
			podSec.FSGroup = &clusterAgent.Spec.FSGroup
		}
	}

	if podSec != nil {
		dep.Spec.Template.Spec.SecurityContext = podSec
	}

	//image pull secret
	if clusterAgent.Spec.ImagePullSecret != "" {
		dep.Spec.Template.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
			{Name: clusterAgent.Spec.ImagePullSecret},
		}
	}

	if clusterAgent.Spec.ProxyUser != "" {
		secret := &corev1.Secret{}
		key := client.ObjectKey{Namespace: clusterAgent.Namespace, Name: AGENT_PROXY_SECRET_NAME}
		err := r.client.Get(context.TODO(), key, secret)
		if err == nil {
			proxyPassword := corev1.EnvVar{
				Name: "APPDYNAMICS_AGENT_PROXY_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: AGENT_PROXY_SECRET_NAME},
						Key:                  "proxy-password",
					},
				},
			}
			dep.Spec.Template.Spec.Containers[0].Env = append(dep.Spec.Template.Spec.Containers[0].Env, proxyPassword)
		} else {
			log.Error(err, "Unable to load secret cluster-agent-proxy-secret for proxy password.")
		}
	}

	//save the new spec in annotations
	saveOrUpdateClusterAgentSpecAnnotation(clusterAgent, dep)

	// Set Cluster Agent instance as the owner and controller
	controllerutil.SetControllerReference(clusterAgent, dep, r.scheme)
	return dep
}

func saveOrUpdateClusterAgentSpecAnnotation(clusterAgent *appdynamicsv1alpha1.Clusteragent, dep *appsv1.Deployment) {
	jsonObj, e := json.Marshal(clusterAgent)
	if e != nil {
		log.Error(e, "Unable to serialize the current spec", "clusterAgent.Namespace", clusterAgent.Namespace, "clusterAgent.Name", clusterAgent.Name)
	} else {
		if dep.Annotations == nil {
			dep.Annotations = make(map[string]string)
		}
		dep.Annotations[OLD_SPEC] = string(jsonObj)
	}
}

func (r *ReconcileClusteragent) restartAgent(clusterAgent *appdynamicsv1alpha1.Clusteragent) error {
	podList := &corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForClusteragent(clusterAgent))
	listOps := &client.ListOptions{
		Namespace:     clusterAgent.Namespace,
		LabelSelector: labelSelector,
	}
	err := r.client.List(context.TODO(), listOps, podList)
	if err != nil || len(podList.Items) < 1 {
		return fmt.Errorf("Unable to retrieve cluster-agent pod. %v", err)
	}
	pod := podList.Items[0]
	//delete to force restart
	err = r.client.Delete(context.TODO(), &pod)
	if err != nil {
		return fmt.Errorf("Unable to delete cluster-agent pod. %v", err)
	}
	return nil
}

func labelsForClusteragent(clusterAgent *appdynamicsv1alpha1.Clusteragent) map[string]string {
	return map[string]string{"name": "clusterAgent", "clusterAgent_cr": clusterAgent.Name}
}

func setInstrumentationAgentDefaults(clusterAgent *appdynamicsv1alpha1.Clusteragent) {
	if clusterAgent.Spec.InstrumentationMethod == "" {
		clusterAgent.Spec.InstrumentationMethod = NO_INSTRUMENTATION
	}

	if clusterAgent.Spec.DefaultLabelMatch == nil {
		clusterAgent.Spec.DefaultLabelMatch = make([]map[string]string, 0)
	}

	defaultImageInfoMap := map[string]appdynamicsv1alpha1.ImageInfo{
		JAVA_LANGUAGE: {
			Image:           AppDJavaAttachImage,
			AgentMountPath:  AGENT_MOUNT_PATH,
			ImagePullPolicy: IMAGE_PULL_POLICY,
		},
		DOTNET_LANGUAGE: {
			Image:           AppDDotNetAttachImage,
			AgentMountPath:  AGENT_MOUNT_PATH,
			ImagePullPolicy: IMAGE_PULL_POLICY,
		},
		NODEJS_LANGUAGE: {
			Image:           AppDNodeJSAttachImage,
			AgentMountPath:  AGENT_MOUNT_PATH,
			ImagePullPolicy: IMAGE_PULL_POLICY,
		},
	}
	if clusterAgent.Spec.ImageInfoMap == nil {
		clusterAgent.Spec.ImageInfoMap = defaultImageInfoMap
	} else {
		for _, language := range []string{JAVA_LANGUAGE, DOTNET_LANGUAGE, NODEJS_LANGUAGE} {
			imageInfo, exists := clusterAgent.Spec.ImageInfoMap[language]
			if exists {
				if imageInfo.Image == "" {
					imageInfo.Image = defaultImageInfoMap[language].Image
				}
				if imageInfo.AgentMountPath == "" {
					imageInfo.AgentMountPath = defaultImageInfoMap[language].AgentMountPath
				}
				if imageInfo.ImagePullPolicy == "" {
					imageInfo.ImagePullPolicy = defaultImageInfoMap[language].ImagePullPolicy
				}
				clusterAgent.Spec.ImageInfoMap[language] = imageInfo
			} else {
				clusterAgent.Spec.ImageInfoMap[language] = defaultImageInfoMap[language]
			}
		}
	}

	if clusterAgent.Spec.DefaultInstrumentMatchString == "" {
		clusterAgent.Spec.DefaultInstrumentMatchString = MATCH_ALL_REGEX
	}

	if clusterAgent.Spec.DefaultInstrumentationTech == "" {
		clusterAgent.Spec.DefaultInstrumentationTech = JAVA_LANGUAGE
	}

	if clusterAgent.Spec.ResourcesToInstrument == nil {
		clusterAgent.Spec.ResourcesToInstrument = []string{Deployment}
	}

	if clusterAgent.Spec.DefaultEnv == "" {
		clusterAgent.Spec.DefaultEnv = JAVA_TOOL_OPTIONS
	}

	if clusterAgent.Spec.InstrumentContainer == "" {
		clusterAgent.Spec.InstrumentContainer = FIRST
	}

	if clusterAgent.Spec.NetvizInfo == (appdynamicsv1alpha1.NetvizInfo{}) {
		clusterAgent.Spec.NetvizInfo = appdynamicsv1alpha1.NetvizInfo{
			BciEnabled: true,
			Port:       3892,
		}
	}

	if clusterAgent.Spec.AppNameStrategy == "" {
		clusterAgent.Spec.AppNameStrategy = MANUAL_APPNAME_STRATEGY
	}

	setInstrumentationRuleDefault(clusterAgent)
}

func setInstrumentationRuleDefault(clusterAgent *appdynamicsv1alpha1.Clusteragent) {
	// Note: We do not set any default to appname. It will be determined at runtime
	for i := range clusterAgent.Spec.InstrumentationRules {
		if clusterAgent.Spec.InstrumentationRules[i].EnvToUse == "" {
			clusterAgent.Spec.InstrumentationRules[i].EnvToUse = clusterAgent.Spec.DefaultEnv
		}

		if len(clusterAgent.Spec.InstrumentationRules[i].LabelMatch) == 0 {
			clusterAgent.Spec.InstrumentationRules[i].LabelMatch = clusterAgent.Spec.DefaultLabelMatch
		}

		if clusterAgent.Spec.InstrumentationRules[i].MatchString == "" {
			clusterAgent.Spec.InstrumentationRules[i].MatchString = clusterAgent.Spec.DefaultInstrumentMatchString
		}

		if clusterAgent.Spec.InstrumentationRules[i].Language == "" {
			clusterAgent.Spec.InstrumentationRules[i].Language = clusterAgent.Spec.DefaultInstrumentationTech
		}

		if clusterAgent.Spec.InstrumentationRules[i].ImageInfo == (appdynamicsv1alpha1.ImageInfo{}) {
			clusterAgent.Spec.InstrumentationRules[i].ImageInfo = clusterAgent.Spec.ImageInfoMap[clusterAgent.Spec.InstrumentationRules[i].Language]
		} else {
			if clusterAgent.Spec.InstrumentationRules[i].ImageInfo.Image == "" {
				clusterAgent.Spec.InstrumentationRules[i].ImageInfo.Image = AppDJavaAttachImage
			}
			if clusterAgent.Spec.InstrumentationRules[i].ImageInfo.AgentMountPath == "" {
				clusterAgent.Spec.InstrumentationRules[i].ImageInfo.AgentMountPath = AGENT_MOUNT_PATH
			}
			if clusterAgent.Spec.InstrumentationRules[i].ImageInfo.ImagePullPolicy == "" {
				clusterAgent.Spec.InstrumentationRules[i].ImageInfo.ImagePullPolicy = IMAGE_PULL_POLICY
			}
		}

		if clusterAgent.Spec.InstrumentationRules[i].InstrumentContainer == "" {
			clusterAgent.Spec.InstrumentationRules[i].InstrumentContainer = clusterAgent.Spec.InstrumentContainer
		}

		if clusterAgent.Spec.InstrumentationRules[i].ContainerNameMatchString == "" {
			clusterAgent.Spec.InstrumentationRules[i].ContainerNameMatchString = clusterAgent.Spec.DefaultContainerMatchString
		}

		if clusterAgent.Spec.InstrumentationRules[i].CustomAgentConfig == "" {
			clusterAgent.Spec.InstrumentationRules[i].CustomAgentConfig = clusterAgent.Spec.DefaultCustomConfig
		}

		if clusterAgent.Spec.InstrumentationRules[i].NetvizInfo == (appdynamicsv1alpha1.NetvizInfo{}) {
			clusterAgent.Spec.InstrumentationRules[i].NetvizInfo = clusterAgent.Spec.NetvizInfo
		}

		if clusterAgent.Spec.InstrumentationRules[i].RunAsUser == 0 {
			clusterAgent.Spec.InstrumentationRules[i].RunAsUser = clusterAgent.Spec.RunAsUser
		}

		if clusterAgent.Spec.InstrumentationRules[i].RunAsGroup == 0 {
			clusterAgent.Spec.InstrumentationRules[i].RunAsGroup = clusterAgent.Spec.RunAsGroup
		}

		if clusterAgent.Spec.InstrumentationRules[i].AnalyticsHost == "" && clusterAgent.Spec.InstrumentationRules[i].AnalyticsPort == 0 {
			clusterAgent.Spec.InstrumentationRules[i].AnalyticsSslEnabled = clusterAgent.Spec.DefaultAnalyticsSslEnabled
		}

		if clusterAgent.Spec.InstrumentationRules[i].AnalyticsHost == "" {
			clusterAgent.Spec.InstrumentationRules[i].AnalyticsHost = clusterAgent.Spec.DefaultAnalyticsHost
		}

		if clusterAgent.Spec.InstrumentationRules[i].AnalyticsPort == 0 {
			clusterAgent.Spec.InstrumentationRules[i].AnalyticsPort = clusterAgent.Spec.DefaultAnalyticsPort
		}
	}
}

func setClusterAgentConfigDefaults(clusterAgent *appdynamicsv1alpha1.Clusteragent) {
	// bootstrap-config defaults
	if clusterAgent.Spec.NsToMonitor == nil {
		clusterAgent.Spec.NsToMonitor = []string{"default"}
	}

	if clusterAgent.Spec.EventUploadInterval == 0 {
		clusterAgent.Spec.EventUploadInterval = 10
	}

	if clusterAgent.Spec.NumberOfTaskWorkers == 0 {
		clusterAgent.Spec.NumberOfTaskWorkers = 2
	}

	if clusterAgent.Spec.ContainerRegistrationInterval == 0 {
		clusterAgent.Spec.ContainerRegistrationInterval = 120
	}

	if clusterAgent.Spec.HttpClientTimeout == 0 {
		clusterAgent.Spec.HttpClientTimeout = 30
	}

	// agent-monitoring defaults
	if clusterAgent.Spec.MetricsSyncInterval == 0 {
		clusterAgent.Spec.MetricsSyncInterval = 30
	}

	if clusterAgent.Spec.ClusterMetricsSyncInterval == 0 {
		clusterAgent.Spec.ClusterMetricsSyncInterval = 60
	}

	if clusterAgent.Spec.MetadataSyncInterval == 0 {
		clusterAgent.Spec.MetadataSyncInterval = 60
	}

	if clusterAgent.Spec.ContainerBatchSize == 0 {
		clusterAgent.Spec.ContainerBatchSize = 5
	}

	if clusterAgent.Spec.PodBatchSize == 0 {
		clusterAgent.Spec.PodBatchSize = 6
	}

	if clusterAgent.Spec.MetricUploadRetryCount == 0 {
		clusterAgent.Spec.MetricUploadRetryCount = 2
	}

	if clusterAgent.Spec.MetricUploadRetryIntervalMilliSeconds == 0 {
		clusterAgent.Spec.MetricUploadRetryIntervalMilliSeconds = 5
	}

	if clusterAgent.Spec.MaxPodsToRegisterCount == 0 {
		clusterAgent.Spec.MaxPodsToRegisterCount = 750
	}

	if clusterAgent.Spec.MaxPodLogsTailLinesCount == 0 {
		clusterAgent.Spec.MaxPodLogsTailLinesCount = 500
	}

	// logger defaults
	if clusterAgent.Spec.LogLevel == "" {
		clusterAgent.Spec.LogLevel = "INFO"
	}

	if clusterAgent.Spec.LogFileSizeMb == 0 {
		clusterAgent.Spec.LogFileSizeMb = 5
	}

	if clusterAgent.Spec.LogFileBackups == 0 {
		clusterAgent.Spec.LogFileBackups = 3
	}

	if clusterAgent.Spec.StdoutLogging == "" {
		clusterAgent.Spec.StdoutLogging = "true"
	}
}

func parseLabelField(labels []map[string]string, labelsType string) string {
	var stringBuilder strings.Builder
	stringBuilder.WriteString(" " + labelsType + "-labels: [")
	for _, label := range labels {
		for labelKey, labelValue := range label {
			stringBuilder.WriteString(fmt.Sprintf("{ %s: %s },", labelKey, labelValue))
		}
	}
	labelsString := strings.TrimRight(stringBuilder.String(), ",")
	return labelsString
}

func parseNameField(names []string, namesType string) string {
	var stringBuilder strings.Builder
	stringBuilder.WriteString(" " + namesType + "-names: [")
	for _, name := range names {
		stringBuilder.WriteString(name + ",")
	}
	namesString := strings.TrimRight(stringBuilder.String(), ",")
	return namesString
}

func createPodFilterString(clusterAgent *appdynamicsv1alpha1.Clusteragent) string {
	var podFilterString strings.Builder
	podFilterString.WriteString("{")
	if clusterAgent.Spec.PodFilter.BlocklistedLabels != nil {
		podFilterString.
			WriteString(parseLabelField(clusterAgent.Spec.PodFilter.BlocklistedLabels, BLOCKLISTED) + "],")
	}

	if clusterAgent.Spec.PodFilter.AllowlistedLabels != nil {
		podFilterString.
			WriteString(parseLabelField(clusterAgent.Spec.PodFilter.AllowlistedLabels, ALLOWLISTED) + "],")
	}

	if clusterAgent.Spec.PodFilter.BlocklistedNames != nil {
		podFilterString.
			WriteString(parseNameField(clusterAgent.Spec.PodFilter.BlocklistedNames, BLOCKLISTED) + "],")
	}

	if clusterAgent.Spec.PodFilter.AllowlistedNames != nil {
		podFilterString.
			WriteString(parseNameField(clusterAgent.Spec.PodFilter.AllowlistedNames, ALLOWLISTED) + "],")
	}
	return strings.TrimRight(podFilterString.String(), ",") + "}"
}

func netvizInfoToMap(netvizInfo appdynamicsv1alpha1.NetvizInfo) map[string]interface{} {
	return map[string]interface{}{
		"bci-enabled": netvizInfo.BciEnabled,
		"port":        netvizInfo.Port,
	}
}

func netvizInfoToJsonString(netvizInfo appdynamicsv1alpha1.NetvizInfo) string {
	netvizInfoMap := netvizInfoToMap(netvizInfo)
	json, err := json.Marshal(netvizInfoMap)
	if err != nil {
		fmt.Printf("Failed to marshal netviz info %v, %v", netvizInfoMap, err)
		return ""
	}
	return string(json)
}

func imageInfoMapToJsonString(imageInfo map[string]appdynamicsv1alpha1.ImageInfo) string {
	imageInfoMap := make(map[string]map[string]string)
	for language, info := range imageInfo {
		imageInfoMap[language] = imageInfoToMap(info)
	}
	json, err := json.Marshal(imageInfoMap)
	if err != nil {
		fmt.Printf("Failed to marshal image info %v, %v", imageInfoMap, err)
		return ""
	}
	return string(json)
}

func imageInfoToMap(imageInfo appdynamicsv1alpha1.ImageInfo) map[string]string {
	return map[string]string{
		"image":             imageInfo.Image,
		"agent-mount-path":  imageInfo.AgentMountPath,
		"image-pull-policy": imageInfo.ImagePullPolicy,
	}
}

func customAgentConfigsToArrayMap(customConfigs []appdynamicsv1alpha1.CustomConfigInfo) []map[string]string {
	out := make([]map[string]string, 0)
	for _, customConfig := range customConfigs {
		out = append(out, map[string]string{
			"config-map-name": customConfig.ConfigMapName,
			"sub-dir":         customConfig.SubDir,
		})
	}
	return out
}
func mapToJsonString(mapToConvert []map[string]string) string {
	valueToConvert := convertToMapOfArray(mapToConvert)
	json, err := json.Marshal(valueToConvert)
	if err != nil {
		fmt.Printf("Failed to marshal label info %v, %v", valueToConvert, err)
		return ""
	}
	return string(json)
}

func convertToMapOfArray(m []map[string]string) map[string][]string {
	result := make(map[string][]string)
	for _, label := range m {
		for key, val := range label {
			result[key] = append(result[key], val)
		}
	}
	return result
}

func instrumentationRulesToJsonString(rules []appdynamicsv1alpha1.InstrumentationRule) string {
	rulesOut := make([]map[string]interface{}, 0)
	for _, rule := range rules {
		ruleMap := map[string]interface{}{
			"namespaces":             rule.NamespaceRegex,
			"match-string":           rule.MatchString,
			"label-match":            convertToMapOfArray(rule.LabelMatch),
			"app-name":               rule.AppName,
			"tier-name":              rule.TierName,
			"language":               rule.Language,
			"instrument-container":   rule.InstrumentContainer,
			"container-match-string": rule.ContainerNameMatchString,
			"custom-agent-config":    rule.CustomAgentConfig,
			"env":                    rule.EnvToUse,
			"image-info":             imageInfoToMap(rule.ImageInfo),
			"netviz-info":            netvizInfoToMap(rule.NetvizInfo),
			"run-as-user":            rule.RunAsUser,
			"run-as-group":           rule.RunAsGroup,
			"app-name-label":         rule.AppNameLabel,
			"analytics-host":         rule.AnalyticsHost,
			"analytics-port":         rule.AnalyticsPort,
			"analytics-ssl-enabled":  rule.AnalyticsSslEnabled,
			"custom-settings-info":   customAgentConfigsToArrayMap(rule.CustomConfigInfo),
		}
		rulesOut = append(rulesOut, ruleMap)
	}
	json, err := json.Marshal(rulesOut)
	if err != nil {
		fmt.Printf("Failed to marshal ns rules %v, %v", rulesOut, err)
		return ""
	}
	return string(json)
}
