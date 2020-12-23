package clustercollector

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"strconv"
	"strings"
	"time"

	appdynamicsv1alpha1 "github.com/Appdynamics/appdynamics-operator/pkg/apis/appdynamics/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	OLD_SPEC                      string = "cluster-collector-spec"
	ClUSTER_MON_CONFIG_MAP_NAME   string = "cluster-collector-config"
	INFRA_AGENT_CONFIG_MAP_NAME   string = "infra-agent-config"
	INFRA_AGENT_NAME              string = "Infra Structure Agent"
	CLUSTER_COLLECTOR             string = "Cluster Monitor"
	TYPE_COLLECTOR                string = "Collector"
	CLUSTER_COLLECTOR_PATH        string = "./collectors/cluster-collector-linux-amd64"
	CONTAINER_MON                 string = "Container Monitor"
	CONTAINER_MON_CONFIG_MAP_NAME string = "container-collector-config"
	SERVER_MON                    string = "Server Monitor"
	SERVER_MON_CONFIG_MAP_NAME    string = "server-collector-config"
)

var log = logf.Log.WithName("controller_clustercollector")

// Add creates a new Clustercollector Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileClustercollector{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("clustercollector-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Clustercollector
	err = c.Watch(&source.Kind{Type: &appdynamicsv1alpha1.Clustercollector{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Deployment and requeue the owner Clustercollector
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appdynamicsv1alpha1.Clustercollector{},
	})
	if err != nil {
		return err
	}
	err = c.Watch(&source.Kind{Type: &appsv1.DaemonSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appdynamicsv1alpha1.Clustercollector{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileClustercollector{}

// ReconcileClustercollector reconciles a Clustercollector object
type ReconcileClustercollector struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

func (r *ReconcileClustercollector) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Clustercollector...")

	clusterCollector := &appdynamicsv1alpha1.Clustercollector{}
	err := r.client.Get(context.TODO(), request.NamespacedName, clusterCollector)
	reqLogger.Info("Retrieved cluster collector.", "Image", clusterCollector.Spec.Image)
	if err != nil {
		if errors.IsNotFound(err) {
			// Return and don't requeue
			reqLogger.Info("Cluster Collector resource not found. The object must be deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get Cluster Collector")
		return reconcile.Result{}, err
	}
	// updating configmaps
	reqLogger.Info("Ensuring and retrieving all required configMaps")
	econfig := r.ensureConfigMap(clusterCollector)
	if econfig != nil {
		reqLogger.Error(econfig, "Failed to obtain cluster agent config map", "Deployment.Namespace", clusterCollector.Namespace, "Deployment.Name", clusterCollector.Name)
		return reconcile.Result{}, econfig
	}

	reqLogger.Info("Cluster Collector spec exists. Checking the corresponding deployment...")
	clusterCollectorCtrl := NewClusterCollectorController(r.client, clusterCollector)
	reQueueClusterCollector, cErr := r.ensureClusterCollectors(clusterCollectorCtrl, clusterCollector, reqLogger)
	hostCollectorCtrl := NewHostCollectorController(r.client, clusterCollector)
	reQueueHostCollector, hErr := r.ensureClusterCollectors(hostCollectorCtrl, clusterCollector, reqLogger)
	if cErr != nil || hErr != nil {
		return reconcile.Result{}, fmt.Errorf("cluster-collector error: %v, host collector error: %s", cErr, hErr)
	} else if reQueueClusterCollector || reQueueHostCollector {
		return reconcile.Result{RequeueAfter: 60 * time.Second}, nil
	}
	reqLogger.Info("Exiting reconciliation loop.")
	return reconcile.Result{}, nil
}

func (r *ReconcileClustercollector) ensureClusterCollectors(clusterCollectorCtrl IClusterController, clusterCollector *appdynamicsv1alpha1.Clustercollector, reqLogger logr.Logger) (bool, error) {

	newDeploymentCreated, err := clusterCollectorCtrl.Init(reqLogger)
	dep := clusterCollectorCtrl.Get()
	controllerutil.SetControllerReference(clusterCollector, dep, r.scheme)
	// Check if the collector already exists in the namespace
	if err != nil {
		return false, err
	} else if newDeploymentCreated == true {
		err = clusterCollectorCtrl.Create(reqLogger)
		if err != nil {
			return false, err
		}
		return false, nil
	}
	switch dep.(type) {
	case *appsv1.Deployment:
		reqLogger.Info("Cluster Collector deployment exists. Checking for deltas with the current state...")
	case *appsv1.DaemonSet:
		reqLogger.Info("Host Collector daemonSet exists. Checking for deltas with the current state...")
	}

	reQueue, err := clusterCollectorCtrl.Update(reqLogger)
	if err != nil {
		return false, err
	}
	return reQueue, err
}

func (r *ReconcileClustercollector) ensureConfigMap(clusterCollector *appdynamicsv1alpha1.Clustercollector) error {
	setClusterCollectorConfigDefaults(clusterCollector)
	setInfraAgentConfigsDefaults(clusterCollector)
	setHostCollectorConfigDefaults(clusterCollector)
	err := r.ensureClusterCollectorConfig(clusterCollector)
	if err != nil {
		return err
	}
	err = r.ensureInfraAgentConfig(clusterCollector)
	if err != nil {
		return err
	}
	err = r.ensureContainerMonConfig(clusterCollector)
	if err != nil {
		return err
	}
	err = r.ensureServerMonConfig(clusterCollector)
	if err != nil {
		return err
	}
	return nil
}

func (r *ReconcileClustercollector) ensureInfraAgentConfig(clusterCollector *appdynamicsv1alpha1.Clustercollector) error {
	errVal, controllerDns, port, sslEnabled := validateUrl(clusterCollector.Spec.ControllerUrl)
	if errVal != nil {
		return errVal
	}
	portVal := strconv.Itoa(int(port))

	yml := fmt.Sprintf(`name: %s 
controller-host: %s
controller-port: %s
controller-account-name: %s
controller-ssl-enabled: %s
enabled: %t
controller-access-key: %s
controller-lib-socket-url: %s
collector-lib-port: %s
http-client-timeout: %d
http-client-basic-auth-enabled: %t
configuration-change-scan-period: %d
configuration-stale-grace-period: %d
debug-port: %s
client-lib-send-url: %s
client-lib-recv-url: %s
log-level: %s
debug-enabled: %t`, INFRA_AGENT_NAME, controllerDns, portVal, clusterCollector.Spec.Account, sslEnabled, true, clusterCollector.Spec.AccessSecret,
		clusterCollector.Spec.SystemConfigs.CollectorLibSocketUrl, clusterCollector.Spec.SystemConfigs.CollectorLibPort,
		clusterCollector.Spec.SystemConfigs.HttpClientTimeOut, clusterCollector.Spec.SystemConfigs.HttpBasicAuthEnabled,
		clusterCollector.Spec.SystemConfigs.ConfigChangeScanPeriod, clusterCollector.Spec.SystemConfigs.ConfigStaleGracePeriod,
		clusterCollector.Spec.SystemConfigs.DebugPort, clusterCollector.Spec.SystemConfigs.ClientLibSendUrl,
		clusterCollector.Spec.SystemConfigs.ClientLibRecvUrl, clusterCollector.Spec.SystemConfigs.LogLevel, clusterCollector.Spec.SystemConfigs.DebugEnabled)

	cm := &corev1.ConfigMap{}
	cm.Name = INFRA_AGENT_CONFIG_MAP_NAME
	cm.Namespace = clusterCollector.Namespace
	cm.Data = make(map[string]string)
	cm.Data[INFRA_AGENT_CONFIG_FILE_NAME] = string(yml)

	err := createConfigMap(r.client, cm)
	return err
}

func (r *ReconcileClustercollector) ensureClusterCollectorConfig(clusterCollector *appdynamicsv1alpha1.Clustercollector) error {
	version := "latest"
	if len(strings.Split(clusterCollector.Spec.Image, ":")) > 1 {
		version = strings.Split(clusterCollector.Spec.Image, ":")[1]
	}
	yml := fmt.Sprintf(`name: %s
type: %s
version: %s
clusterName: %s
nsToMonitor: %s
nsToExclude: %s
clusterMonitoringEnabled: %t
log-level: %s
path: %s
enabled: %t
exporter-address: %s
exporter-port: %d`, CLUSTER_COLLECTOR, TYPE_COLLECTOR, version, clusterCollector.Spec.ClusterName, clusterCollector.Spec.NsToMonitorRegex,
		clusterCollector.Spec.NsToExcludeRegex, clusterCollector.Spec.ClusterMonEnabled, clusterCollector.Spec.LogLevel,
		CLUSTER_COLLECTOR_PATH, true, clusterCollector.Spec.ExporterAddress, clusterCollector.Spec.ExporterPort)
	cm := &corev1.ConfigMap{}
	cm.Name = ClUSTER_MON_CONFIG_MAP_NAME
	cm.Namespace = clusterCollector.Namespace
	cm.Data = make(map[string]string)
	cm.Data["clustermon.conf"] = string(yml)

	err := createConfigMap(r.client, cm)
	return err
}

func (r *ReconcileClustercollector) ensureContainerMonConfig(clusterCollector *appdynamicsv1alpha1.Clustercollector) error {

	hostCollectorConfig := clusterCollector.Spec.HostCollector
	errVal, exporterAddr, port, _ := validateUrl(hostCollectorConfig.ContainerMetricExporterAddress)
	if errVal != nil {
		return errVal
	}
	portVal := strconv.Itoa(int(port))

	version := "latest"
	if len(strings.Split(clusterCollector.Spec.Image, ":")) > 1 {
		version = strings.Split(clusterCollector.Spec.Image, ":")[1]
	}
	yml := fmt.Sprintf(`name: %s
type: %s
version: %s
path: %s
enabled: %t
exporter-address: %s
exporter-port: %s
privileged: %t
dependency: %s
install-dependency: %t
log-level: %s`, CONTAINER_MON, TYPE_COLLECTOR, version, hostCollectorConfig.ContainerCollectorPath,
		true, exporterAddr, portVal, false, hostCollectorConfig.ContainerCollectorDependency, true, hostCollectorConfig.LogLevel)

	cm := &corev1.ConfigMap{}
	cm.Name = CONTAINER_MON_CONFIG_MAP_NAME
	cm.Namespace = clusterCollector.Namespace
	cm.Data = make(map[string]string)
	cm.Data[CONTAINER_MON_CONFIG_FILE_NAME] = string(yml)

	err := createConfigMap(r.client, cm)
	return err

}

func (r *ReconcileClustercollector) ensureServerMonConfig(clusterCollector *appdynamicsv1alpha1.Clustercollector) error {
	hostCollectorConfig := clusterCollector.Spec.HostCollector
	errVal, exporterAddr, port, _ := validateUrl(hostCollectorConfig.ContainerMetricExporterAddress)
	if errVal != nil {
		return errVal
	}
	portVal := strconv.Itoa(int(port))
	version := "latest"
	if len(strings.Split(clusterCollector.Spec.Image, ":")) > 1 {
		version = strings.Split(clusterCollector.Spec.Image, ":")[1]
	}

	yml := fmt.Sprintf(`name: %s
type: %s
version: %s
path: %s
enabled: %t
exporter-address: %s
exporter-port: %s
privileged: %t
dependency: %s
install-dependency: %t
log-level: %s`, SERVER_MON, TYPE_COLLECTOR, version, hostCollectorConfig.ServerCollectorPath,
		true, exporterAddr, portVal, false, hostCollectorConfig.ServerCollectorDependency, true, hostCollectorConfig.LogLevel)

	cm := &corev1.ConfigMap{}
	cm.Name = SERVER_MON_CONFIG_MAP_NAME
	cm.Namespace = clusterCollector.Namespace
	cm.Data = make(map[string]string)
	cm.Data[SERVER_MON_CONFIG_FILE_NAME] = string(yml)

	err := createConfigMap(r.client, cm)
	return err
}
