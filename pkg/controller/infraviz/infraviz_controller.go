package infraviz

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	appdynamicsv1alpha1 "github.com/Appdynamics/appdynamics-operator/pkg/apis/appdynamics/v1alpha1"
	"github.com/Appdynamics/appdynamics-operator/version"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
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

var log = logf.Log.WithName("controller_infraviz")

const (
	AGENT_SECRET_NAME         string = "cluster-agent-secret"
	AGENT_CONFIG_NAME         string = "ma-config"
	AGENT_LOG_CONFIG_NAME     string = "ma-log-config"
	AGENT_SSL_CONFIG_NAME     string = "ma-ssl-config"
	AGENT_SSL_CRED_STORE_NAME string = "appd-agent-ssl-store"
	AGENT_NETVIZ_CONFIG_NAME  string = "netviz-config"
	BIQPORT                   int32  = 9090
	OLD_SPEC                  string = "old-infraviz"
	OS_LINUX                  string = "linux"
	OS_WINDOWS                string = "windows"
	OS_ALL                    string = "all"
	UNIQUE_ID_FROM_HOST_IP    string = "status.hostIP"
	UNIQUE_ID_FROM_HOST_NAME  string = "spec.nodeName"

//	SYSLOG_PORT               int32  = 5144
)

// Add creates a new InfraViz Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileInfraViz{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("infraviz-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource InfraViz
	err = c.Watch(&source.Kind{Type: &appdynamicsv1alpha1.InfraViz{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner InfraViz
	err = c.Watch(&source.Kind{Type: &appsv1.DaemonSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appdynamicsv1alpha1.InfraViz{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileInfraViz{}

// ReconcileInfraViz reconciles a InfraViz object
type ReconcileInfraViz struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client          client.Client
	scheme          *runtime.Scheme
	addNodeSelector bool
}

func NewReconcileInfraViz(mgr manager.Manager) ReconcileInfraViz {
	riv := ReconcileInfraViz{client: mgr.GetClient(), scheme: mgr.GetScheme()}
	return riv
}

// Reconcile reads that state of the cluster for a InfraViz object and makes changes based on the state read
// and what is in the InfraViz.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileInfraViz) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling InfraViz")

	// Fetch the InfraViz instance
	infraViz := &appdynamicsv1alpha1.InfraViz{}
	err := r.client.Get(context.TODO(), request.NamespacedName, infraViz)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	r.scheme.Default(infraViz)

	r.addNodeSelector = infraViz.Spec.NodeOS != ""
	if infraViz.Spec.NodeOS == OS_ALL {
		//create 2 daemonsets, one for linux and another for windows nodes
		return r.ReconcileMixed(request, infraViz)
	}

	_, errReconcile := r.ReconcileDaemon(request, infraViz)
	if errReconcile != nil {
		return reconcile.Result{}, fmt.Errorf("Unable to reconcile daemonset. %v", errReconcile)
	}

	r.updateStatus(infraViz)

	return reconcile.Result{RequeueAfter: 60 * time.Second}, nil
}

func (r *ReconcileInfraViz) ReconcileMixed(request reconcile.Request, infraViz *appdynamicsv1alpha1.InfraViz) (reconcile.Result, error) {

	infravizLin := infraViz.DeepCopy()
	infravizLin.Spec.NodeOS = OS_LINUX

	_, err := r.ReconcileDaemon(request, infravizLin)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("Unable to reconcile daemonset for Linux nodes. %v", err)
	}

	infravizWin := infraViz.DeepCopy()
	infravizWin.Spec.NodeOS = OS_WINDOWS

	_, errWin := r.ReconcileDaemon(request, infravizWin)
	if errWin != nil {
		return reconcile.Result{}, fmt.Errorf("Unable to reconcile daemonset for Windows nodes. %v", errWin)
	}

	r.updateStatusMixed(infraViz, infravizLin, infravizWin)

	return reconcile.Result{}, nil
}

func (r *ReconcileInfraViz) ReconcileDaemon(request reconcile.Request, infraViz *appdynamicsv1alpha1.InfraViz) (reconcile.Result, error) {
	dsName := getDaemonName(infraViz)

	desiredDS := r.newInfraVizDaemonSet(infraViz)

	// Set InfraViz instance as the owner and controller
	if err := controllerutil.SetControllerReference(infraViz, desiredDS, r.scheme); err != nil {
		log.Error(err, "Unable to set owner reference on daemonset", "Namespace", infraViz.Namespace, "Name", dsName)
		return reconcile.Result{}, err
	}

	// Check if the Daemonset already exists
	existingDs := &appsv1.DaemonSet{}
	createDS := false
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: dsName, Namespace: infraViz.Namespace}, existingDs)
	if err != nil && errors.IsNotFound(err) {
		createDS = true
	} else if err != nil {
		log.Error(err, "Unable to load daemonset", "Namespace", infraViz.Namespace, "Name", dsName)
		return reconcile.Result{}, err
	}

	//if any breaking changes, restart ds
	hasBreakingChanges, errConf := r.validate(infraViz, existingDs, createDS)
	if errConf != nil {
		log.Error(errConf, "Unable to validate", "Namespace", infraViz.Namespace, "Name", dsName)
		return reconcile.Result{}, errConf
	}

	if hasBreakingChanges {
		log.Info("Breaking changes detected. Updating...")
		err = r.client.Update(context.TODO(), desiredDS)
		if err != nil {
			log.Error(err, "Unable to update breaking changes", "Namespace", infraViz.Namespace, "Name", dsName)
			return reconcile.Result{}, err
		}
		err := r.restartDaemonSet(infraViz)
		if err != nil {
			log.Error(err, "Unable to restart after breaking changes", "Namespace", infraViz.Namespace, "Name", dsName)
			return reconcile.Result{}, err
		}
	}

	if createDS {
		log.Info("Creating a new Daemon Set", "Namespace", infraViz.Namespace, "Name", dsName)
		err = r.client.Create(context.TODO(), desiredDS)
		if err != nil {
			log.Error(err, "Unable to create daemonset", "Namespace", infraViz.Namespace, "Name", dsName)
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileInfraViz) updateStatus(infraViz *appdynamicsv1alpha1.InfraViz) error {
	infraViz.Status.LastUpdateTime = metav1.Now()
	infraViz.Status.Version = version.Version

	podList, err := r.getInfraVizPods(infraViz)
	if err != nil {
		return fmt.Errorf("Unable to list pods of the  InfraViz daemonset. %v", err)
	}

	infraViz.Status.Nodes = make(map[string]string)
	for _, pod := range podList.Items {
		name := pod.Name
		status := pod.Status.Phase
		infraViz.Status.Nodes[name] = string(status)
	}

	updatedStatus := infraViz.Status

	infraViz.Status = appdynamicsv1alpha1.InfraVizStatus{}

	if errInstance := r.client.Update(context.TODO(), infraViz); errInstance != nil {
		return fmt.Errorf("Unable to update InfraViz instance. %v", errInstance)
	}
	log.Info("InfraViz instance updated successfully", "infraViz.Namespace", infraViz.Namespace, "Date", infraViz.Status.LastUpdateTime)

	infraViz.Status = updatedStatus
	err = r.client.Status().Update(context.TODO(), infraViz)
	if err != nil {
		log.Error(err, "Failed to update InfraViz status", "infraViz.Namespace", infraViz.Namespace, "infraViz.Name", infraViz.Name)
	} else {
		log.Info("InfraViz status updated successfully", "infraViz.Namespace", infraViz.Namespace, "Date", infraViz.Status.LastUpdateTime)
	}
	return err
}

func (r *ReconcileInfraViz) updateStatusMixed(infraViz *appdynamicsv1alpha1.InfraViz, infraVizLin *appdynamicsv1alpha1.InfraViz, infraVizWin *appdynamicsv1alpha1.InfraViz) error {
	infraViz.Status.LastUpdateTime = metav1.Now()
	infraViz.Status.Version = version.Version

	podListLin, errLin := r.getInfraVizPods(infraVizLin)
	if errLin != nil {
		return fmt.Errorf("Unable to list pods of the Linux Daemonset. %v", errLin)
	}

	podListWin, errWin := r.getInfraVizPods(infraVizWin)
	if errWin != nil {
		return fmt.Errorf("Unable to list pods of the Windows Daemonset. %v", errWin)
	}

	infraViz.Status.Nodes = make(map[string]string)
	for _, pod := range podListLin.Items {
		name := pod.Name
		status := pod.Status.Phase
		infraViz.Status.Nodes[name] = string(status)
	}

	for _, pod := range podListWin.Items {
		name := pod.Name
		status := pod.Status.Phase
		infraViz.Status.Nodes[name] = string(status)
	}

	updatedStatus := infraViz.Status

	infraViz.Status = appdynamicsv1alpha1.InfraVizStatus{}

	if errInstance := r.client.Update(context.TODO(), infraViz); errInstance != nil {
		return fmt.Errorf("Unable to update InfraViz instance. %v", errInstance)
	}
	log.Info("InfraViz instance updated successfully", "infraViz.Namespace", infraViz.Namespace, "Date", infraViz.Status.LastUpdateTime)

	infraViz.Status = updatedStatus
	err := r.client.Status().Update(context.TODO(), infraViz)
	if err != nil {
		log.Error(err, "Failed to update InfraViz status", "infraViz.Namespace", infraViz.Namespace, "infraViz.Name", infraViz.Name)
	} else {
		log.Info("InfraViz status updated successfully", "infraViz.Namespace", infraViz.Namespace, "Date", infraViz.Status.LastUpdateTime)
	}
	return err
}

func (r *ReconcileInfraViz) validate(infraViz *appdynamicsv1alpha1.InfraViz, existingDS *appsv1.DaemonSet, newDS bool) (bool, error) {
	breakingChanges := false
	if !newDS && existingDS.Annotations != nil {
		if oldJson, ok := existingDS.Annotations[OLD_SPEC]; ok && oldJson != "" {
			var oldSpec appdynamicsv1alpha1.InfraViz
			errJson := json.Unmarshal([]byte(oldJson), &oldSpec)
			if errJson != nil {
				log.Error(errJson, "Unable to retrieve the old spec from annotations", "infraViz.Namespace", infraViz.Namespace, "infraViz.Name", infraViz.Name)
			}
			if !reflect.DeepEqual(&oldSpec.Spec, &infraViz.Spec) {
				log.Info("Validate. Breaking changes detected compared to annotations")
				breakingChanges = true
			}
		}
	}

	logLevel := "info"

	//validate image for windows, as it is required
	if infraViz.Spec.NodeOS == OS_WINDOWS && infraViz.Spec.ImageWin == "" {
		return breakingChanges, fmt.Errorf("Image reference is required. For Windows and Mixed clusters set ImageWin property")
	}

	errVal, controllerDns, port, sslEnabled := validateControllerUrl(infraViz.Spec.ControllerUrl)
	if errVal != nil {
		return breakingChanges, errVal
	}

	eventUrl := infraViz.Spec.EventServiceUrl
	if eventUrl == "" {
		if strings.Contains(controllerDns, "appdynamics.com") {
			//saas
			eventUrl = "https://analytics.api.appdynamics.com"
		} else {
			protocol := "http"
			if sslEnabled == "true" {
				protocol = "https"
			}
			eventUrl = fmt.Sprintf("%s://%s:9080", protocol, controllerDns)
		}
	}
	var proxyHost, proxyPort, proxyUser, proxyPass string
	if infraViz.Spec.ProxyUrl != "" {
		arr := strings.Split(infraViz.Spec.ProxyUrl, ":")
		if len(arr) != 3 {
			fmt.Println("ProxyUrl is invalid. Use this format: protocol://domain:port")
			return breakingChanges, fmt.Errorf("ProxyUrl is invalid. Use this format: protocol://domain:port")
		} else {
			proxyHost = strings.TrimLeft(arr[1], "//")
			proxyPort = arr[2]
		}
	}

	if infraViz.Spec.ProxyUser != "" {
		arr := strings.Split(infraViz.Spec.ProxyUser, "@")
		if len(arr) != 2 {
			fmt.Println("ProxyUser is invalid. Use this format: user@pass")
			return breakingChanges, fmt.Errorf("ProxyUser is invalid. Use this format: user@pass")
		} else {
			proxyUser = arr[0]
			proxyPass = arr[1]
		}
	}

	if infraViz.Spec.LogLevel != "" {
		logLevel = infraViz.Spec.LogLevel
	}

	if infraViz.Spec.NetVizPort > 0 && infraViz.Spec.NodeOS != OS_WINDOWS {
		r.ensureNetVizConfig(infraViz)
	}

	isWindows := infraViz.Spec.NodeOS == OS_WINDOWS
	logConfigRequired := isWindows == false

	cm := &corev1.ConfigMap{}
	cmName := getConfigMapName(infraViz)

	err := r.client.Get(context.TODO(), types.NamespacedName{Name: cmName, Namespace: infraViz.Namespace}, cm)

	create := false
	if err != nil && errors.IsNotFound(err) {
		fmt.Printf("Config map %s not found. Creating...\n", cmName)
		//configMap does not exist. Create
		cm.Name = cmName
		cm.Namespace = infraViz.Namespace
		cm.Data = make(map[string]string)
		create = true
	} else if err != nil {
		return breakingChanges, fmt.Errorf("Failed to load configMap %s. %v", cmName, err)
	}

	if infraViz.Spec.EnableContainerHostId == "" {
		infraViz.Spec.EnableContainerHostId = "true"
	}

	if infraViz.Spec.EnableServerViz == "" {
		infraViz.Spec.EnableServerViz = "true"
	}

	if infraViz.Spec.EnableDockerViz == "" {
		infraViz.Spec.EnableDockerViz = "true"
	}

	if !create {
		if !newDS {
			if logConfigRequired {
				if cm.Data["APPDYNAMICS_LOG_LEVEL"] != logLevel ||
					cm.Data["APPDYNAMICS_LOG_STDOUT"] != strconv.FormatBool(infraViz.Spec.StdoutLogging) {
					breakingChanges = false
					if strings.Contains(infraViz.Spec.Image, "4.5") {
						e := r.ensureLogConfig(infraViz, logLevel)
						if e != nil {
							return breakingChanges, e
						}
					} else {
						e := r.ensureLogConfigCalendar(infraViz, logLevel)
						if e != nil {
							return breakingChanges, e
						}
					}
				}
			}

			//			if cm.Data["APPDYNAMICS_AGENT_ACCOUNT_NAME"] != infraViz.Spec.Account ||
			//				cm.Data["APPDYNAMICS_AGENT_GLOBAL_ACCOUNT_NAME"] != infraViz.Spec.GlobalAccount ||
			//				cm.Data["APPDYNAMICS_CONTROLLER_HOST_NAME"] != controllerDns ||
			//				cm.Data["APPDYNAMICS_CONTROLLER_PORT"] != strconv.Itoa(int(port)) ||
			//				cm.Data["APPDYNAMICS_SYSLOG_PORT"] != strconv.Itoa(int(infraViz.Spec.SyslogPort)) ||
			//				cm.Data["APPDYNAMICS_WIN_IMAGE"] != infraViz.Spec.ImageWin ||
			//				cm.Data["APPDYNAMICS_CONTROLLER_SSL_ENABLED"] != sslEnabled ||
			//				cm.Data["EVENT_ENDPOINT"] != eventUrl ||
			//				cm.Data["APPDYNAMICS_AGENT_PROXY_HOST"] != proxyHost ||
			//				cm.Data["APPDYNAMICS_AGENT_PROXY_PORT"] != proxyPort ||
			//				cm.Data["APPDYNAMICS_AGENT_PROXY_USER"] != proxyUser ||
			//				cm.Data["APPDYNAMICS_AGENT_PROXY_PASS"] != proxyPass ||
			//				cm.Data["APPDYNAMICS_AGENT_ENABLE_CONTAINERIDASHOSTID"] != infraViz.Spec.EnableContainerHostId ||
			//				cm.Data["APPDYNAMICS_SIM_ENABLED"] != infraViz.Spec.EnableServerViz ||
			//				cm.Data["APPDYNAMICS_DOCKER_ENABLED"] != infraViz.Spec.EnableDockerViz ||
			//				cm.Data["APPDYNAMICS_AGENT_METRIC_LIMIT"] != infraViz.Spec.MetricsLimit ||
			//				cm.Data["APPDYNAMICS_MA_PROPERTIES"] != infraViz.Spec.PropertyBag {
			//				log.Info("Validate. Breaking changes detected compared to configMap", "infraviz", existingDS.Name, "CM", cm.Name)
			//				breakingChanges = true
			//			}
		}

	} else {
		if logConfigRequired {
			if strings.Contains(infraViz.Spec.Image, "4.5") {
				e := r.ensureLogConfig(infraViz, logLevel)
				if e != nil {
					return breakingChanges, e
				}
			} else {
				e := r.ensureLogConfigCalendar(infraViz, logLevel)
				if e != nil {
					return breakingChanges, e
				}
			}
		}
	}

	cm.Data["APPDYNAMICS_AGENT_ACCOUNT_NAME"] = infraViz.Spec.Account
	cm.Data["APPDYNAMICS_AGENT_GLOBAL_ACCOUNT_NAME"] = infraViz.Spec.GlobalAccount
	cm.Data["APPDYNAMICS_CONTROLLER_HOST_NAME"] = controllerDns
	cm.Data["APPDYNAMICS_CONTROLLER_PORT"] = strconv.Itoa(int(port))
	cm.Data["APPDYNAMICS_CONTROLLER_SSL_ENABLED"] = string(sslEnabled)

	cm.Data["APPDYNAMICS_NETVIZ_AGENT_PORT"] = strconv.Itoa(int(infraViz.Spec.NetVizPort))
	cm.Data["APPDYNAMICS_SYSLOG_PORT"] = strconv.Itoa(int(infraViz.Spec.SyslogPort))
	cm.Data["APPDYNAMICS_WIN_IMAGE"] = infraViz.Spec.ImageWin

	cm.Data["APPDYNAMICS_AGENT_ENABLE_CONTAINERIDASHOSTID"] = infraViz.Spec.EnableContainerHostId

	cm.Data["APPDYNAMICS_SIM_ENABLED"] = infraViz.Spec.EnableServerViz

	cm.Data["APPDYNAMICS_DOCKER_ENABLED"] = infraViz.Spec.EnableDockerViz

	cm.Data["EVENT_ENDPOINT"] = eventUrl
	cm.Data["APPDYNAMICS_AGENT_PROXY_HOST"] = proxyHost
	cm.Data["APPDYNAMICS_AGENT_PROXY_PORT"] = proxyPort
	cm.Data["APPDYNAMICS_AGENT_PROXY_USER"] = proxyUser
	cm.Data["APPDYNAMICS_AGENT_PROXY_PASS"] = proxyPass
	cm.Data["APPDYNAMICS_AGENT_METRIC_LIMIT"] = infraViz.Spec.MetricsLimit
	cm.Data["APPDYNAMICS_LOG_LEVEL"] = logLevel
	cm.Data["APPDYNAMICS_LOG_STDOUT"] = strconv.FormatBool(infraViz.Spec.StdoutLogging)
	cm.Data["APPDYNAMICS_MA_PROPERTIES"] = infraViz.Spec.PropertyBag

	if errOwner := controllerutil.SetControllerReference(infraViz, cm, r.scheme); errOwner != nil {
		return breakingChanges, fmt.Errorf("Unable to set ownership to MA config map. %v", errOwner)
	}

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return breakingChanges, fmt.Errorf("Unable to create MA config map. %v", e)
		}
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return breakingChanges, fmt.Errorf("Unable to update MA config map. %v", e)
		}
	}

	if infraViz.Spec.AgentSSLStoreName != "" {
		r.ensureSSLConfig(infraViz)
	}

	return breakingChanges, nil
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

func (r *ReconcileInfraViz) restartDaemonSet(infraViz *appdynamicsv1alpha1.InfraViz) error {
	podList, err := r.getInfraVizPods(infraViz)

	if err != nil {
		return err
	}

	for _, p := range podList.Items {
		err = r.client.Delete(context.TODO(), &p)
		if err != nil && errors.IsNotFound(err) == false {
			return fmt.Errorf("Unable to delete InfraViz pod. %v", err)
		}
	}

	return nil
}

func (r *ReconcileInfraViz) getInfraVizPods(infraViz *appdynamicsv1alpha1.InfraViz) (*corev1.PodList, error) {
	podList := corev1.PodList{}
	labelSelector := labels.SelectorFromSet(labelsForInfraViz(infraViz))
	filter := &client.ListOptions{
		Namespace:     infraViz.Namespace,
		LabelSelector: labelSelector,
	}
	err := r.client.List(context.TODO(), filter, &podList)
	if err != nil {
		return nil, fmt.Errorf("Unable to load InfraViz pods. %v", err)
	}
	log.Info("Loaded InfraViz pods", "Number:", podList.Size(), "OS:", infraViz.Spec.NodeOS)

	return &podList, nil
}

func (r *ReconcileInfraViz) newInfraVizDaemonSet(infraViz *appdynamicsv1alpha1.InfraViz) *appsv1.DaemonSet {
	dsName := getDaemonName(infraViz)

	netviz := false
	if infraViz.Spec.NetVizPort > 0 {
		netviz = true
	}

	if infraViz.Spec.BiqPort == 0 {
		infraViz.Spec.BiqPort = BIQPORT
	}

	errSvc := r.ensureAgentService(infraViz)
	if errSvc != nil {
		fmt.Printf("Issues with InfraViz service: %v", errSvc)
	}
	r.ensureSecret(infraViz)

	selector := labelsForInfraViz(infraViz)
	podSpec := r.newPodSpecForCR(infraViz, netviz)

	ds := appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dsName,
			Namespace: infraViz.Namespace,
			Labels:    selector,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector},
				Spec:       podSpec,
			},
		},
	}

	//save the new spec in annotations
	jsonObj, e := json.Marshal(infraViz)
	if e != nil {
		log.Error(e, "Unable to serialize the current spec", "infraViz.Namespace", infraViz.Namespace, "infraViz.Name", infraViz.Name)
	} else {
		if ds.Annotations == nil {
			ds.Annotations = make(map[string]string)
		}
		ds.Annotations[OLD_SPEC] = string(jsonObj)
	}

	return &ds
}

func (r *ReconcileInfraViz) newPodSpecForCR(infraViz *appdynamicsv1alpha1.InfraViz, netviz bool) corev1.PodSpec {
	trueVar := true
	isWindows := infraViz.Spec.NodeOS == OS_WINDOWS

	imageName := infraViz.Spec.Image
	if imageName == "" {
		imageName = "appdynamics/machine-agent-analytics:latest"
	}

	if isWindows {
		imageName = infraViz.Spec.ImageWin
	}

	if infraViz.Spec.NetVizImage == "" {
		infraViz.Spec.NetVizImage = "appdynamics/machine-agent-netviz:latest"
	}

	if infraViz.Spec.EnableContainerHostId == "" {
		infraViz.Spec.EnableContainerHostId = "true"
	}

	if r.addNodeSelector {
		if infraViz.Spec.NodeSelector == nil {
			infraViz.Spec.NodeSelector = make(map[string]string)
		}
		if infraViz.Spec.NodeOS == OS_LINUX || infraViz.Spec.NodeOS == "" {
			infraViz.Spec.NodeSelector["kubernetes.io/os"] = OS_LINUX
		}
		if infraViz.Spec.NodeOS == OS_WINDOWS {
			infraViz.Spec.NodeSelector["kubernetes.io/os"] = OS_WINDOWS
		}
	}

	secretName := AGENT_SECRET_NAME
	if infraViz.Spec.AccessSecret != "" {
		secretName = infraViz.Spec.AccessSecret
	}

	accessKey := corev1.EnvVar{
		Name: "APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Key:                  "controller-key",
			},
		},
	}

	if infraViz.Spec.Env == nil || len(infraViz.Spec.Env) == 0 {
		infraViz.Spec.Env = []corev1.EnvVar{}
	}

	if !envVarExists("APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY", infraViz.Spec.Env) {
		infraViz.Spec.Env = append(infraViz.Spec.Env, accessKey)
	}

	if infraViz.Spec.UniqueHostId == "" {
		infraViz.Spec.UniqueHostId = UNIQUE_ID_FROM_HOST_NAME
		if infraViz.Spec.Pks {
			infraViz.Spec.UniqueHostId = UNIQUE_ID_FROM_HOST_IP
		}
	} else {
		if infraViz.Spec.UniqueHostId != UNIQUE_ID_FROM_HOST_NAME &&
			infraViz.Spec.UniqueHostId != UNIQUE_ID_FROM_HOST_IP {
			infraViz.Spec.UniqueHostId = ""
		}
	}

	if infraViz.Spec.UniqueHostId != "" {
		uniqueHostId := corev1.EnvVar{
			Name: "APPDYNAMICS_AGENT_UNIQUE_HOST_ID",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: infraViz.Spec.UniqueHostId,
				},
			},
		}
		if !envVarExists("APPDYNAMICS_AGENT_UNIQUE_HOST_ID", infraViz.Spec.Env) {
			infraViz.Spec.Env = append(infraViz.Spec.Env, uniqueHostId)
		}
	}

	dir := corev1.HostPathDirectory
	socket := corev1.HostPathSocket

	cm := corev1.EnvFromSource{}
	cm.ConfigMapRef = &corev1.ConfigMapEnvSource{}
	cmName := getConfigMapName(infraViz)
	cm.ConfigMapRef.Name = cmName

	ports := infraViz.Spec.Ports
	if ports == nil || len(ports) == 0 {
		ports = []corev1.ContainerPort{}
	}
	biqPort := corev1.ContainerPort{
		ContainerPort: infraViz.Spec.BiqPort,
		Protocol:      corev1.ProtocolTCP,
	}
	ports = append(ports, biqPort)

	if infraViz.Spec.SyslogPort > 0 {
		sysLogPort := corev1.ContainerPort{
			ContainerPort: infraViz.Spec.SyslogPort,
			Protocol:      corev1.ProtocolTCP,
			HostPort:      infraViz.Spec.SyslogPort,
		}
		ports = append(ports, sysLogPort)
	}

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{{
			Args: infraViz.Spec.Args,
			Env:  infraViz.Spec.Env,
			EnvFrom: []corev1.EnvFromSource{
				cm,
			},
			Image:           imageName,
			ImagePullPolicy: corev1.PullAlways,
			Name:            "appd-infra-agent",

			Resources: infraViz.Spec.Resources,
			Ports:     ports,
		}},
		NodeSelector:       infraViz.Spec.NodeSelector,
		ServiceAccountName: "appdynamics-infraviz",
		Tolerations:        infraViz.Spec.Tolerations,
	}

	//image pull secret
	if infraViz.Spec.ImagePullSecret != "" {
		podSpec.ImagePullSecrets = []corev1.LocalObjectReference{
			{Name: infraViz.Spec.ImagePullSecret},
		}
	}

	if isWindows == false {
		podSpec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      "proc",
				MountPath: "/hostroot/proc",
				ReadOnly:  true,
			},
			{
				Name:      "etc",
				MountPath: "/hostroot/etc",
				ReadOnly:  true,
			}, {
				Name:      "sys",
				MountPath: "/hostroot/sys",
				ReadOnly:  true,
			},
			{
				Name:      "ma-log-volume",
				MountPath: "/opt/appdynamics/conf/logging",
				ReadOnly:  true,
			}}
		podSpec.Containers[0].SecurityContext = &corev1.SecurityContext{
			Privileged: &trueVar,
		}
		podSpec.HostNetwork = true
		podSpec.HostPID = true
		podSpec.HostIPC = true
		podSpec.Volumes = []corev1.Volume{
			{
				Name: "proc",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/proc", Type: &dir,
					},
				},
			},
			{
				Name: "etc",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/etc", Type: &dir,
					},
				},
			},
			{
				Name: "sys",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/sys", Type: &dir,
					},
				},
			},
			{
				Name: "ma-log-volume",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: AGENT_LOG_CONFIG_NAME,
						},
					},
				},
			}}
	}

	if infraViz.Spec.PriorityClassName != "" {
		podSpec.PriorityClassName = infraViz.Spec.PriorityClassName
	}

	if isWindows == false && infraViz.Spec.EnableMasters {
		tolerationMasters := corev1.Toleration{Key: "node-role.kubernetes.io/master", Effect: corev1.TaintEffectNoSchedule, Operator: corev1.TolerationOpExists}
		if podSpec.Tolerations == nil {
			podSpec.Tolerations = []corev1.Toleration{tolerationMasters}
		} else {
			podSpec.Tolerations = append(podSpec.Tolerations, tolerationMasters)
		}
	}

	if isWindows == false && infraViz.Spec.EnableDockerViz == "" {
		infraViz.Spec.EnableDockerViz = "true"
	}

	var dockerVol corev1.Volume
	if isWindows == false && infraViz.Spec.EnableDockerViz == "true" {
		if infraViz.Spec.Pks {
			dockerVol = corev1.Volume{
				Name: "docker-sock",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/var/vcap/data/sys", Type: &dir,
					},
				},
			}
		} else {
			dockerVol = corev1.Volume{
				Name: "docker-sock",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: "/var/run/docker.sock", Type: &socket,
					},
				},
			}
		}
		podSpec.Volumes = append(podSpec.Volumes, dockerVol)

		volMountDocker := corev1.VolumeMount{
			Name:      "docker-sock",
			MountPath: "/var/run/docker.sock",
			ReadOnly:  true,
		}

		if infraViz.Spec.Pks {
			volMountDocker.MountPath = "/mnt"
			podSpec.Containers[0].Command = []string{"/bin/sh", "-c"}
			podSpec.Containers[0].Args = []string{"echo starting; mkdir -p /var/run/; ln -s /mnt/run/docker/docker.sock /var/run/docker.sock; /opt/appdynamics/startup.sh"}
		}

		podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, volMountDocker)
	}

	if isWindows == false && netviz {
		resRequest := corev1.ResourceList{}
		resRequest[corev1.ResourceCPU] = resource.MustParse("0.1")
		resRequest[corev1.ResourceMemory] = resource.MustParse("150Mi")

		resLimit := corev1.ResourceList{}
		resLimit[corev1.ResourceCPU] = resource.MustParse("0.2")
		resLimit[corev1.ResourceMemory] = resource.MustParse("300Mi")
		reqs := corev1.ResourceRequirements{Requests: resRequest, Limits: resLimit}

		netVizVolume := corev1.Volume{Name: "netviz-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: AGENT_NETVIZ_CONFIG_NAME,
					},
				},
			}}
		podSpec.Volumes = append(podSpec.Volumes, netVizVolume)

		netVizContainer := corev1.Container{
			Image:           infraViz.Spec.NetVizImage,
			ImagePullPolicy: corev1.PullAlways,
			Name:            "appd-netviz-agent",

			Resources: reqs,
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"}},
			},
			Ports: []corev1.ContainerPort{{
				ContainerPort: infraViz.Spec.NetVizPort,
				Protocol:      corev1.ProtocolTCP,
				HostPort:      infraViz.Spec.NetVizPort,
			}},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "netviz-volume",
				MountPath: "/netviz-agent/conf/agent_config.lua",
				SubPath:   "agent_config.lua",
			}},
		}
		podSpec.Containers = append(podSpec.Containers, netVizContainer)
	}

	if isWindows == false && infraViz.Spec.AgentSSLStoreName != "" {
		//custom SSL cert store
		volSSL := corev1.Volume{
			Name: "ssl-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: AGENT_SSL_CRED_STORE_NAME,
					},
				},
			},
		}
		volMountSSL := corev1.VolumeMount{
			Name:      "ssl-volume",
			MountPath: fmt.Sprintf("/opt/appdynamics/conf/%s", infraViz.Spec.AgentSSLStoreName),
			SubPath:   infraViz.Spec.AgentSSLStoreName,
		}

		//custom SSL config xml
		volSSLConfig := corev1.Volume{
			Name: "ssl-config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: AGENT_SSL_CONFIG_NAME,
					},
				},
			},
		}
		volMountConfig := corev1.VolumeMount{
			Name:      "ssl-config-volume",
			MountPath: "/opt/appdynamics/conf/controller-info.xml",
			SubPath:   "controller-info.xml",
		}

		podSpec.Volumes = append(podSpec.Volumes, volSSLConfig)
		podSpec.Volumes = append(podSpec.Volumes, volSSL)

		podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, volMountSSL)
		podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, volMountConfig)
	}

	return podSpec
}

func envVarExists(envVarName string, envs []corev1.EnvVar) bool {
	exists := false
	for _, envVar := range envs {
		if envVar.Name == envVarName {
			exists = true
			break
		}
	}
	return exists
}

func hasDSpecChanged(dsSpec *appsv1.DaemonSetSpec, newSpec *appsv1.DaemonSetSpec, ivSpec *appdynamicsv1alpha1.InfraVizSpec) bool {
	if len(dsSpec.Template.Spec.Containers) != len(newSpec.Template.Spec.Containers) {
		return true
	}
	if len(dsSpec.Template.Spec.Containers) == 2 && len(newSpec.Template.Spec.Containers) == 2 &&
		len(dsSpec.Template.Spec.Containers[1].Ports) > 0 && len(newSpec.Template.Spec.Containers[1].Ports) > 0 {
		if dsSpec.Template.Spec.Containers[1].Ports[0].ContainerPort != newSpec.Template.Spec.Containers[1].Ports[0].ContainerPort {
			return true
		}
	}
	currentSpecClone := ivSpec.DeepCopy()
	cloneCurrentSpec(dsSpec, currentSpecClone)
	if !reflect.DeepEqual(ivSpec, currentSpecClone) {
		return true
	}
	return false
}

func cloneCurrentSpec(dsSpec *appsv1.DaemonSetSpec, ivSpec *appdynamicsv1alpha1.InfraVizSpec) {

	ivSpec.Image = ""
	if len(dsSpec.Template.Spec.Containers) >= 1 {
		ivSpec.Image = dsSpec.Template.Spec.Containers[0].Image
	}

	ivSpec.Env = nil
	if len(dsSpec.Template.Spec.Containers) >= 1 && dsSpec.Template.Spec.Containers[0].Env != nil {
		in, out := &dsSpec.Template.Spec.Containers[0].Env, &ivSpec.Env
		*out = make([]corev1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}

	ivSpec.NodeSelector = nil
	if dsSpec.Template.Spec.NodeSelector != nil {
		in, out := &dsSpec.Template.Spec.NodeSelector, &ivSpec.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}

	ivSpec.Tolerations = nil
	if dsSpec.Template.Spec.Tolerations != nil {
		in, out := &dsSpec.Template.Spec.Tolerations, &ivSpec.Tolerations
		*out = make([]corev1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}

	ivSpec.Args = nil
	if len(dsSpec.Template.Spec.Containers) >= 1 && dsSpec.Template.Spec.Containers[0].Args != nil {
		in, out := &dsSpec.Template.Spec.Containers[0].Args, &ivSpec.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}

	ivSpec.Resources = corev1.ResourceRequirements{}
	if len(dsSpec.Template.Spec.Containers) >= 1 {
		dsSpec.Template.Spec.Containers[0].Resources.DeepCopyInto(&ivSpec.Resources)
	}
}

func (r *ReconcileInfraViz) ensureSSLConfig(infraViz *appdynamicsv1alpha1.InfraViz) error {

	if infraViz.Spec.AgentSSLStoreName == "" {
		return nil
	}

	//verify that AGENT_SSL_CRED_STORE_NAME map exists
	existing := &corev1.ConfigMap{}
	errCheck := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_SSL_CRED_STORE_NAME, Namespace: infraViz.Namespace}, existing)

	if errCheck != nil && errors.IsNotFound(errCheck) {
		return fmt.Errorf("Custom SSL store is requested, but the expected configMap %s with the trusted certificate store not found. Put the desired certificates into the cert store and create the configMap in the %s namespace", AGENT_SSL_CRED_STORE_NAME, infraViz.Namespace)
	} else if errCheck != nil {
		return fmt.Errorf("Unable to validate the expected configMap %s with the trusted certificate store. Put the desired certificates into the cert store and create the configMap in the %s namespace", AGENT_SSL_CRED_STORE_NAME, infraViz.Namespace)
	}

	//create controller config map for ssl store credentials
	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<controller-info>
    <controller-host></controller-host>
    <controller-port></controller-port>
    <controller-ssl-enabled>true</controller-ssl-enabled>
    <enable-orchestration>false</enable-orchestration>
    <unique-host-id></unique-host-id>
    <account-access-key></account-access-key>
    <account-name></account-name>
    <sim-enabled>true</sim-enabled>
    <machine-path></machine-path>
    <controller-keystore-password>/opt/appdynamics/conf/%s</controller-keystore-password>
    <controller-keystore-filename>%s</controller-keystore-filename>
</controller-info>`, infraViz.Spec.AgentSSLStoreName, infraViz.Spec.AgentSSLPassword)

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_SSL_CONFIG_NAME, Namespace: infraViz.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)
	if err == nil {
		e := r.client.Delete(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to delete the old MA SSL configMap. %v", e)
		}
	}
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load MA SSL configMap. %v", err)
	}

	fmt.Printf("Recreating MA SSL Config Map\n")

	cm.Name = AGENT_SSL_CONFIG_NAME
	cm.Namespace = infraViz.Namespace
	cm.Data = make(map[string]string)
	cm.Data[infraViz.Spec.AgentSSLStoreName] = string(xml)

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create MA SSL configMap. %v", e)
		}
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to re-create MA SSL configMap. %v", e)
		}
	}

	fmt.Println("Configmap re-created")
	return nil
}

func (r *ReconcileInfraViz) ensureLogConfigCalendar(infraViz *appdynamicsv1alpha1.InfraViz, logLevel string) error {

	appender := "FileAppender"
	if infraViz.Spec.StdoutLogging {
		appender = "ConsoleAppender"
	}

	xml := `<?xml version="1.0" encoding="UTF-8" ?>
<configuration status="Warn" monitorInterval="30">

    <Appenders>
        <Console name="ConsoleAppender" target="SYSTEM_OUT">
            <PatternLayout pattern="%d{ABSOLUTE} %5p [%t] %c{1} - %m%n"/>
        </Console>

        <RollingFile name="FileAppender" fileName="${log4j:configParentLocation}/../../logs/machine-agent.log"
                     filePattern="${log4j:configParentLocation}/../../logs/machine-agent.log.%i">
            <PatternLayout>
                <Pattern>[%t] %d{DATE} %5p %c{1} - %m%n</Pattern>
            </PatternLayout>
            <Policies>
                <SizeBasedTriggeringPolicy size="5000 KB"/>
            </Policies>
            <DefaultRolloverStrategy max="5"/>
        </RollingFile>
    </Appenders>` + fmt.Sprintf(`

    <Loggers>
        <Logger name="com.singularity" level="%s" additivity="false">
            <AppenderRef ref="%s"/>
        </Logger>
        <Logger name="com.appdynamics" level="%s" additivity="false">
            <AppenderRef ref="%s"/>
        </Logger>
        <Logger name="com.singularity.ee.agent.systemagent.task.sigar.SigarAppAgentMonitor" level="%s" additivity="false">
            <AppenderRef ref="%s"/>
        </Logger>
        <Root level="error">
            <AppenderRef ref="%s"/>
        </Root>
    </Loggers>

</configuration>`, logLevel, appender, logLevel, appender, logLevel, appender, appender)

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_LOG_CONFIG_NAME, Namespace: infraViz.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)
	//	if err == nil {
	//		e := r.client.Delete(context.TODO(), cm)
	//		if e != nil {
	//			return fmt.Errorf("Unable to delete the old MA Log configMap. %v", e)
	//		}
	//	}
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load MA Log configMap. %v", err)
	}

	//	fmt.Printf("Recreating MA Log Config Map\n")

	cm.Name = AGENT_LOG_CONFIG_NAME
	cm.Namespace = infraViz.Namespace
	cm.Data = make(map[string]string)
	cm.Data["log4j.xml"] = string(xml)

	// Set InfraViz instance as the owner and controller
	if err := controllerutil.SetControllerReference(infraViz, cm, r.scheme); err != nil {
		return fmt.Errorf("Unable to set Infraviz as owner of the infra agent log configMap. %v", err)
	}

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create MA Log configMap. %v", e)
		}
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update MA Log configMap. %v", e)
		}
	}

	fmt.Println("Logging configuration saved.")
	return nil

}

func (r *ReconcileInfraViz) ensureLogConfig(infraViz *appdynamicsv1alpha1.InfraViz, logLevel string) error {
	appender := "FileAppender"
	if infraViz.Spec.StdoutLogging {
		appender = "ConsoleAppender"
	}

	xml := `<?xml version="1.0" encoding="UTF-8" ?>
<!DOCTYPE log4j:configuration SYSTEM "log4j.dtd">
<log4j:configuration xmlns:log4j="http://jakarta.apache.org/log4j/">

    <appender name="ConsoleAppender" class="org.apache.log4j.ConsoleAppender">
        <layout class="org.apache.log4j.PatternLayout">
            <param name="ConversionPattern" value="%d{ABSOLUTE} %5p [%t] %c{1} - %m%n"/>
        </layout>
    </appender>

    <appender name="FileAppender" class="com.singularity.ee.agent.systemagent.SystemAgentLogAppender">
        <param name="File" value="logs/machine-agent.log"/>
        <param name="MaxFileSize" value="5000KB"/>
        <param name="MaxBackupIndex" value="5"/>
        <layout class="org.apache.log4j.PatternLayout">
            <param name="ConversionPattern" value="[%t] %d{DATE} %5p %c{1} - %m%n"/>
        </layout>
    </appender>` + fmt.Sprintf(`
    <logger name="com.singularity" additivity="false">
        <level value="%s"/>
        <appender-ref ref="%s"/>
    </logger>

    <logger name="com.appdynamics" additivity="false">
        <level value="%s"/>
        <appender-ref ref="%s"/>
    </logger>

    <logger name="com.singularity.ee.agent.systemagent.task.sigar.SigarAppAgentMonitor" additivity="false">
        <level value="%s"/>
        <appender-ref ref="%s"/>
    </logger>

    <root>
        <priority value="error"/>
        <appender-ref ref="%s"/>
    </root>

</log4j:configuration>
`, logLevel, appender, logLevel, appender, logLevel, appender, appender)

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_LOG_CONFIG_NAME, Namespace: infraViz.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)
	//	if err == nil {
	//		e := r.client.Delete(context.TODO(), cm)
	//		if e != nil {
	//			return fmt.Errorf("Unable to delete the old MA Log configMap. %v", e)
	//		}
	//	}
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load MA Log configMap. %v", err)
	}

	//	fmt.Printf("Recreating MA Log Config Map\n")

	cm.Name = AGENT_LOG_CONFIG_NAME
	cm.Namespace = infraViz.Namespace
	cm.Data = make(map[string]string)
	cm.Data["log4j.xml"] = string(xml)

	// Set InfraViz instance as the owner and controller
	if err := controllerutil.SetControllerReference(infraViz, cm, r.scheme); err != nil {
		return fmt.Errorf("Unable to set Infraviz as owner of the infra agent configMap. %v", err)
	}

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create MA Log configMap. %v", e)
		}
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to update MA Log configMap. %v", e)
		}
	}

	fmt.Println("Logging configuration saved.")
	return nil
}

func (r *ReconcileInfraViz) ensureSecret(infraViz *appdynamicsv1alpha1.InfraViz) error {
	secret := &corev1.Secret{}

	secretName := AGENT_SECRET_NAME
	if infraViz.Spec.AccessSecret != "" {
		secretName = infraViz.Spec.AccessSecret
	}

	key := client.ObjectKey{Namespace: infraViz.Namespace, Name: secretName}
	err := r.client.Get(context.TODO(), key, secret)
	if err != nil && errors.IsNotFound(err) {
		fmt.Printf("Required secret %s not found. An empty secret will be created, but the clusteragent will not start until at least the 'api-user' key of the secret has a valid value", secretName)

		secret = &corev1.Secret{
			Type: corev1.SecretTypeOpaque,
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: infraViz.Namespace,
			},
		}

		secret.StringData = make(map[string]string)
		secret.StringData["api-user"] = ""
		secret.StringData["controller-key"] = ""

		errCreate := r.client.Create(context.TODO(), secret)
		if errCreate != nil {
			fmt.Printf("Unable to create secret. %v\n", errCreate)
			return fmt.Errorf("Unable to get secret for cluster-agent. %v", errCreate)
		} else {
			fmt.Printf("Secret created. %s\n", secretName)
			errLoad := r.client.Get(context.TODO(), key, secret)
			if errLoad != nil {
				fmt.Printf("Unable to reload secret. %v\n", errLoad)
				return fmt.Errorf("Unable to get secret for cluster-agent. %v", err)
			}
		}
	} else if err != nil {
		return fmt.Errorf("Unable to get secret for cluster-agent. %v", err)
	}

	return nil
}

func (r *ReconcileInfraViz) ensureAgentService(infraViz *appdynamicsv1alpha1.InfraViz) error {
	dsName := getDaemonName(infraViz)
	selector := labelsForInfraViz(infraViz)
	svc := &corev1.Service{}
	key := client.ObjectKey{Namespace: infraViz.Namespace, Name: dsName}
	err := r.client.Get(context.TODO(), key, svc)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to get service for cluster-agent. %v\n", err)
	}

	if err != nil && errors.IsNotFound(err) {
		fmt.Printf("InfraViz service not found. %v\n", err)

		svc := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      dsName,
				Namespace: infraViz.Namespace,
				Labels:    selector,
			},
			Spec: corev1.ServiceSpec{
				Selector: selector,
				Ports: []corev1.ServicePort{
					{
						Name:     "biq-port",
						Protocol: corev1.ProtocolTCP,
						Port:     infraViz.Spec.BiqPort,
					},
				},
			},
		}

		if infraViz.Spec.NetVizPort > 0 {
			netVizPort := corev1.ServicePort{
				Name:     "netviz-port",
				Protocol: corev1.ProtocolTCP,
				Port:     infraViz.Spec.NetVizPort,
			}
			svc.Spec.Ports = append(svc.Spec.Ports, netVizPort)
		}

		if infraViz.Spec.SyslogPort > 0 {
			syslogPort := corev1.ServicePort{
				Name:     "syslog-port",
				Protocol: corev1.ProtocolTCP,
				Port:     infraViz.Spec.SyslogPort,
			}
			svc.Spec.Ports = append(svc.Spec.Ports, syslogPort)
		}

		if infraViz.Spec.Ports != nil && len(infraViz.Spec.Ports) > 0 {
			for _, p := range infraViz.Spec.Ports {
				customPort := corev1.ServicePort{
					Name:     p.Name,
					Protocol: corev1.ProtocolTCP,
					Port:     p.ContainerPort,
				}
				svc.Spec.Ports = append(svc.Spec.Ports, customPort)
			}
		}

		// Set InfraViz instance as the owner and controller
		if err := controllerutil.SetControllerReference(infraViz, svc, r.scheme); err != nil {
			return fmt.Errorf("Unable to set Infraviz as owner of the infra agent service. %v", err)
		}

		errCreate := r.client.Create(context.TODO(), svc)
		if errCreate != nil {
			return fmt.Errorf("Failed to create infraViz agent service: %v", errCreate)
		} else {
			fmt.Printf("Infraviz service created")
		}
	}
	return nil
}

func labelsForInfraViz(infraViz *appdynamicsv1alpha1.InfraViz) map[string]string {
	dsName := getDaemonName(infraViz)
	return map[string]string{"infraViz_cr": dsName}
}

func getDaemonName(infraViz *appdynamicsv1alpha1.InfraViz) string {
	dsName := infraViz.Name
	if infraViz.Spec.NodeOS == OS_WINDOWS {
		dsName = fmt.Sprintf("%s-win", dsName)
	}
	return dsName
}

func getConfigMapName(infraViz *appdynamicsv1alpha1.InfraViz) string {
	cmName := AGENT_CONFIG_NAME
	if infraViz.Spec.NodeOS == OS_WINDOWS {
		cmName = fmt.Sprintf("%s-%s", AGENT_CONFIG_NAME, OS_WINDOWS)
	}
	return cmName
}

func (r *ReconcileInfraViz) ensureNetVizConfig(infraViz *appdynamicsv1alpha1.InfraViz) error {
	netvizProps := fmt.Sprintf(`--
-- Copyright (c) 2019 AppDynamics Inc.
-- All rights reserved.
--
package.path = './?.lua;' .. package.path
require "config_helper"

ROOT_DIR="/opt/appdynamics/netviz"
INSTALL_DIR=ROOT_DIR
-- Define a unique hostname for identification on controller
UNIQUE_HOST_ID = ""

-- Define the ip of the interface where webservice is bound
WEBSERVICE_IP="0.0.0.0" 

--
-- NPM global configuration
-- Configurable params
-- {
--	enable_monitor = 0/1,	-- def:0, enable/disable monitoring
--	disable_filter = 0/1,	-- def:0, disable/enable language agent filtering
--	mode = KPI/Diagnostic/Advanced,	-- def:KPI
--	lua_scripts_path	-- Path to lua scripts.
--	enable_fqdn = 0/1	-- def:0, enable/disable fqdn resolution of ip
--	enable_netlib = 1/0	-- def:1 Disable/enable app filtering
--	app_filtering_channel	-- def:tcp://127.0.0.1:3898, Channel for app
--	filetering messages between App agents and Network Agent
--	backoff_enable = 0/1	-- def:1 Enable/Disable agent backoff module
--	backoff_time = [90 - 1200]	-- def:300, Agent auto backoff kick in period in secs
-- }
--
npm_config = {
	log_destination = "file",
	log_file = "appd-netagent.log",
	debug_log_file = "agent-debug.log",
	disable_filter = 1,
	mode = "Advanced",
	enable_netlib = %d,
	lua_scripts_path = ROOT_DIR .. "/scripts/netagent/lua",
	enable_fqdn = 1,
	backoff_enable = 1,
	backoff_time = 300,
}

--
-- Webserver configuration
-- Configurable params
-- {
--	host = ,	-- Ip on which webserver is listening on. Default set to
--			-- localhost. Set it to 0.0.0.0 to listen on all
--	port = ,		-- Port on which to open the webserver
--	request_timeout = , -- Request timeout in ms
--	threads = ,		-- Number of threads on the webserver
-- }
--
webserver_config = {
	host = WEBSERVICE_IP,
	port = %d,
	request_timeout = 10000,
	threads = 4,
}

--
-- Packet capture configurations (multiple captures can be configured)
-- Confiurable params, there can be multiple of these.
-- {
-- 	cap_module = "pcap",		-- def:"pcap", capture module
-- 	cap_type = "device"/"file",	-- def:"device", type of capture
-- 	ifname = "",		-- def:"any", interface name/pcap filename
-- 	enable_promisc = 0/1,	-- def:0, promiscuous mode pkt capture
-- 	thinktime = ,		-- def: 100, time in msec, to sleep if no pkts
-- 	snaplen = ,		-- def:1518. pkt capture len
-- 	buflen = ,		-- def:2. pcap buffer size in MB
-- 	ppi = ,			-- def:32. pcap ppi
-- },
--
capture = {
	-- first capture interface
	{
		cap_module = "pcap",
		cap_type = "device",
		ifname = "any",
		thinktime = 25,
		buflen = 48,
--		filter = "",
	},
--[[	{
		cap_module = "pcap",
		cap_type = "device",
		ifname = "en0",
	},
--]]
}

--
-- IP configuration
-- ip_config = {
--	expire_timeout = ,	-- Mins after which we expire ip metadata
--	retry_count = ,		-- No of tries to resolve fqdn for ip
-- }
ip_config = {
	expire_interval = 20,
	retry_count = 5,
}

--
-- DPI configuration
-- Configurable params
-- {
--	max_flows = ,	-- Max number of flows per fg to DPI at any given time.
--	max_data = ,	-- Max mega bytes to DPI per flow.
--	max_depth = ,	-- Max bytes to DPI in a packet
--	max_callchains = , -- Max callchains to store for a flowgroup
--	max_cc_perflow = , -- Max number of call chains to look for in each flow
-- }
--
dpi_config = {
	max_flows = 10,
	max_data = 4,
	max_depth = 4096,
	max_callchains_in_fg = 32,
	max_callchains_in_flow = 2,
}

-- Configurations for application service ports
-- {
--	ports = ,	-- Comma separated list of application service
--			   ports greater than 32000. Example
--			   ports = "40000, 41000, 42000"
-- }
--[[
application_service_ports = {
	ports = "",
}
--]]

--
-- Export data from network agent configuration/tunnables
-- Configurable params, there can be multiple of these.
-- {
-- 	exportype = "file"/"remote",	-- type of export mechanism
-- 	statsfile = "",			-- filename for stats export
-- 	metricsfile = "", 		-- filename for metrics export
-- 	serialization = "pb",		-- pb/capnp, serialization module
-- 	transport = "zmq", 		-- def:"zmq", transport module
-- 	zmqdest = "", 			-- dest peer for zmq
--  },
--
export_config = {
	-- file export
	{
		exporttype = "file",
		statsfile = "agent-stats.log",
		metricsfile = "agent-metrics.log",
		eventsfile =  "agent-events.log",
		snapshotsfile = "agent-snapshots.log",
		metadatafile = "agent-metadata.log",
	},
}

-- Plugin interface configuration.
-- List of interfaces to be monitored by supported plugins.
-- Configurable params, there can be multiple of these.
-- {
-- 	interface = "eth0",	-- def: "eth0", interface name
-- }
plugin_if_config = {
--[[
	{interface = "eth0"},
--]]
}

-- Plugin process configuration.
-- List of processes to be monitored by supported plugins.
-- Configurable params, there can be multiple of these.
-- {
--	process = "",		-- def: "appd-netagent", process name
-- }
plugin_proc_config = {
	{process = "appd-netagent"},
}

-- metadata to pass to pass the agent metadata specific params
system_metadata = {
	unique_host_id = UNIQUE_HOST_ID,
	install_dir = INSTALL_DIR,
	install_time = get_last_update_time(),
}`, infraViz.Spec.NetlibEnabled, infraViz.Spec.NetVizPort)

	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_NETVIZ_CONFIG_NAME, Namespace: infraViz.Namespace}, cm)

	create := err != nil && errors.IsNotFound(err)
	//	if err == nil {
	//		e := r.client.Delete(context.TODO(), cm)
	//		if e != nil {
	//			return fmt.Errorf("Unable to delete the old Netviz configMap. %v", e)
	//		}
	//	}
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to load Netviz configMap. %v", err)
	}

	fmt.Printf("Recreating Netviz Config Map\n")

	cm.Name = AGENT_NETVIZ_CONFIG_NAME
	cm.Namespace = infraViz.Namespace
	cm.Data = make(map[string]string)
	cm.Data["agent_config.lua"] = string(netvizProps)

	// Set InfraViz instance as the owner and controller
	if err := controllerutil.SetControllerReference(infraViz, cm, r.scheme); err != nil {
		return fmt.Errorf("Unable to set Infraviz as owner of Netviz configMap. %v", err)
	}

	if create {
		e := r.client.Create(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to create Netviz configMap. %v", e)
		}
	} else {
		e := r.client.Update(context.TODO(), cm)
		if e != nil {
			return fmt.Errorf("Unable to re-create Netviz configMap. %v", e)
		}
	}

	fmt.Println("Netviz Configmap re-created")
	return nil
}
