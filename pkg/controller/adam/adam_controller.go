package adam

import (
	"context"
	"fmt"

	"encoding/json"
	"io/ioutil"
	"net/http"
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

var log = logf.Log.WithName("controller_adam")

const (
	AGENT_SECRET_NAME         string = "cluster-agent-secret"
	AGENt_CONFIG_NAME         string = "adam-config"
	AGENT_SSL_CONFIG_NAME     string = "appd-agent-ssl-config"
	AGENT_SSL_CRED_STORE_NAME string = "appd-agent-ssl-store"
)

// Add creates a new Adam Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileAdam{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("adam-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Adam
	err = c.Watch(&source.Kind{Type: &appdynamicsv1alpha1.Adam{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Adam
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appdynamicsv1alpha1.Adam{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileAdam{}

// ReconcileAdam reconciles a Adam object
type ReconcileAdam struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

func (r *ReconcileAdam) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Clusteragent...")

	// Fetch the Clusteragent instance
	clusterAgent := &appdynamicsv1alpha1.Adam{}
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
		_, updatedBag, econfig := r.ensureConfigMap(clusterAgent, secret, true)
		if econfig != nil {
			reqLogger.Error(econfig, "Failed to create new Cluster Agent due to config map", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
			return reconcile.Result{}, econfig
		}
		reqLogger.Info("Creating service...\n")
		_, esvc := r.ensureAgentService(clusterAgent, updatedBag)
		if esvc != nil {
			reqLogger.Error(esvc, "Failed to create new Cluster Agent due to service", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
			return reconcile.Result{}, esvc
		}
		// Define a new deployment for the cluster agent
		dep := r.newAgentDeployment(clusterAgent, secret, updatedBag)
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return reconcile.Result{}, err
		}
		reqLogger.Info("Deployment created successfully. Done")
		r.updateStatus(clusterAgent, updatedBag)
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
	cm, bag, econfig := r.ensureConfigMap(clusterAgent, secret, false)
	if econfig != nil {
		reqLogger.Error(econfig, "Failed to obtain cluster agent config map", "Deployment.Namespace", clusterAgent.Namespace, "Deployment.Name", clusterAgent.Name)
		return reconcile.Result{}, econfig
	}

	breaking, updateDeployment := r.hasBreakingChanges(clusterAgent, bag, existingDeployment, secret)

	//update the configMap
	reqLogger.Info("Reconciling the config map...", "clusterAgent.Namespace", clusterAgent.Namespace)
	updatedBag, errMap := r.updateMap(cm, clusterAgent, secret, false)
	if errMap != nil {
		reqLogger.Error(errMap, "Issues when reconciling the config map...", "clusterAgent.Namespace", clusterAgent.Namespace)
		return reconcile.Result{}, errMap
	}
	if breaking {
		fmt.Println("Breaking changes detected. Restarting the cluster agent pod...")
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

		statusErr := r.updateStatus(clusterAgent, updatedBag)
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

func (r *ReconcileAdam) updateStatus(clusterAgent *appdynamicsv1alpha1.Adam, updatedBag *appdynamicsv1alpha1.AppDBag) error {
	clusterAgent.Status.LastUpdateTime = metav1.Now()

	agentStatus, errState := r.getAgentState(clusterAgent, updatedBag)
	if errState != nil {
		log.Error(errState, "Failed to get agent state", "clusterAgent.Namespace", clusterAgent.Namespace, "Date", clusterAgent.Status.LastUpdateTime)
	} else {
		clusterAgent.Status.State = *agentStatus
	}

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

func (r *ReconcileAdam) getAgentState(clusterAgent *appdynamicsv1alpha1.Adam, updatedBag *appdynamicsv1alpha1.AppDBag) (*appdynamicsv1alpha1.AgentStatus, error) {
	url := fmt.Sprintf("http://%s:%d/status", clusterAgent.Name, updatedBag.AgentServerPort)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("Unable to get status from agent. %v\n", err)
	}

	client := &http.Client{}
	resp, errReq := client.Do(req)
	if errReq != nil {
		return nil, fmt.Errorf("Unable to get status from agent. %v\n", errReq)
	}

	defer resp.Body.Close()

	b, e := ioutil.ReadAll(resp.Body)

	if e != nil {
		return nil, fmt.Errorf("Unable to get status from agent. %v\n", e)
	}

	var agentStatus appdynamicsv1alpha1.AgentStatus

	err = json.Unmarshal(b, &agentStatus)
	if err != nil {
		return nil, fmt.Errorf("Unable to deserialize status from agent. %v\n", err)
	}

	return &agentStatus, nil

}

func (r *ReconcileAdam) hasBreakingChanges(clusterAgent *appdynamicsv1alpha1.Adam, bag *appdynamicsv1alpha1.AppDBag, existingDeployment *appsv1.Deployment, secret *corev1.Secret) (bool, bool) {
	killPod := false
	updateDeployment := false

	fmt.Println("Checking for breaking changes...")

	if clusterAgent.Spec.Image != "" && existingDeployment.Spec.Template.Spec.Containers[0].Image != clusterAgent.Spec.Image {
		fmt.Printf("Image changed from has changed: %s	to	%s. Updating....\n", existingDeployment.Spec.Template.Spec.Containers[0].Image, clusterAgent.Spec.Image)
		existingDeployment.Spec.Template.Spec.Containers[0].Image = clusterAgent.Spec.Image
		return false, true
	}

	if bag.SecretVersion != secret.ResourceVersion {
		fmt.Printf("SecretVersion has changed: %s		%s\n", bag.SecretVersion, secret.ResourceVersion)
		return true, updateDeployment
	}

	if clusterAgent.Spec.ControllerUrl != bag.ControllerUrl {
		fmt.Printf("ControllerUrl has changed: %s		%s\n", bag.ControllerUrl, clusterAgent.Spec.ControllerUrl)
		return true, updateDeployment
	}

	if clusterAgent.Spec.Account != "" && clusterAgent.Spec.Account != bag.Account {
		fmt.Printf("AccountName has changed: %s		%s\n", bag.Account, clusterAgent.Spec.Account)
		return true, updateDeployment
	}

	if clusterAgent.Spec.GlobalAccount != "" && clusterAgent.Spec.GlobalAccount != bag.GlobalAccount {
		fmt.Printf("GlobalAccountName has changed: %s		%s\n", bag.GlobalAccount, clusterAgent.Spec.GlobalAccount)
		return true, updateDeployment
	}
	if clusterAgent.Spec.AppName != "" && clusterAgent.Spec.AppName != bag.AppName {
		fmt.Printf("AppName has changed: %s		%s\n", bag.AppName, clusterAgent.Spec.AppName)
		return true, updateDeployment
	}

	if clusterAgent.Spec.EventServiceUrl != "" && clusterAgent.Spec.EventServiceUrl != bag.EventServiceUrl {
		fmt.Printf("EventServiceUrl has changed: %s		%s\n", bag.EventServiceUrl, clusterAgent.Spec.EventServiceUrl)
		return true, updateDeployment
	}
	if clusterAgent.Spec.SystemSSLCert != "" && clusterAgent.Spec.SystemSSLCert != bag.SystemSSLCert {
		fmt.Printf("SystemSSLCert has changed: %s		%s\n", bag.SystemSSLCert, clusterAgent.Spec.SystemSSLCert)
		return true, updateDeployment
	}

	if clusterAgent.Spec.AgentSSLCert != "" && clusterAgent.Spec.AgentSSLCert != bag.AgentSSLCert {
		fmt.Printf("AgentSSLCert has changed: %s		%s\n", bag.AgentSSLCert, clusterAgent.Spec.AgentSSLCert)
		return true, updateDeployment
	}

	return killPod, updateDeployment
}

func (r *ReconcileAdam) ensureSecret(clusterAgent *appdynamicsv1alpha1.Adam) (*corev1.Secret, error) {
	secret := &corev1.Secret{}

	key := client.ObjectKey{Namespace: clusterAgent.Namespace, Name: AGENT_SECRET_NAME}
	err := r.client.Get(context.TODO(), key, secret)
	if err != nil && errors.IsNotFound(err) {
		fmt.Printf("Required secret %s not found. An empty secret will be created, but the clusteragent will not start until at least the 'api-user' key of the secret has a valid value", AGENT_SECRET_NAME)

		secret = &corev1.Secret{
			Type: corev1.SecretTypeOpaque,
			ObjectMeta: metav1.ObjectMeta{
				Name:      AGENT_SECRET_NAME,
				Namespace: clusterAgent.Namespace,
			},
		}

		secret.StringData = make(map[string]string)
		secret.StringData["api-user"] = ""
		secret.StringData["controller-key"] = ""
		secret.StringData["event-key"] = ""

		errCreate := r.client.Create(context.TODO(), secret)
		if errCreate != nil {
			fmt.Printf("Unable to create secret. %v\n", errCreate)
			return nil, fmt.Errorf("Unable to get secret for cluster-agent. %v", errCreate)
		} else {
			fmt.Printf("Secret created. %s\n", AGENT_SECRET_NAME)
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

func (r *ReconcileAdam) ensureAgentService(clusterAgent *appdynamicsv1alpha1.Adam, bag *appdynamicsv1alpha1.AppDBag) (*corev1.Service, error) {
	selector := labelsForClusteragent(clusterAgent)
	svc := &corev1.Service{}
	key := client.ObjectKey{Namespace: clusterAgent.Namespace, Name: clusterAgent.Name}
	err := r.client.Get(context.TODO(), key, svc)
	if err != nil && !errors.IsNotFound(err) {
		return nil, fmt.Errorf("Unable to get service for cluster-agent. %v\n", err)
	}

	if err != nil && errors.IsNotFound(err) {
		svc := &corev1.Service{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Service",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterAgent.Name,
				Namespace: clusterAgent.Namespace,
				Labels:    selector,
			},
			Spec: corev1.ServiceSpec{
				Selector: selector,
				Ports: []corev1.ServicePort{
					{
						Name:     "web-port",
						Protocol: corev1.ProtocolTCP,
						Port:     bag.AgentServerPort,
					},
				},
			},
		}
		err = r.client.Create(context.TODO(), svc)
		if err != nil {
			return nil, fmt.Errorf("Failed to create cluster agent service: %v", err)
		}
	}
	return svc, nil
}

func (r *ReconcileAdam) cleanUp(clusterAgent *appdynamicsv1alpha1.Adam) {
	namespace := "appdynamics"
	if clusterAgent != nil {
		namespace = clusterAgent.Namespace
	}
	cm := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: "adam-config", Namespace: namespace}, cm)
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

func (r *ReconcileAdam) ensureConfigMap(clusterAgent *appdynamicsv1alpha1.Adam, secret *corev1.Secret, create bool) (*corev1.ConfigMap, *appdynamicsv1alpha1.AppDBag, error) {
	cm := &corev1.ConfigMap{}
	var bag *appdynamicsv1alpha1.AppDBag
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: "adam-config", Namespace: clusterAgent.Namespace}, cm)
	if err != nil && !errors.IsNotFound(err) {
		return nil, nil, fmt.Errorf("Failed to load configMap adam-config. %v", err)
	}
	if err != nil && errors.IsNotFound(err) {
		fmt.Printf("Config map not found. Creating...\n")
		//configMap does not exist. Create
		cm.Name = "adam-config"
		cm.Namespace = clusterAgent.Namespace
		bag, err = r.updateMap(cm, clusterAgent, secret, true)
		if err != nil {
			return nil, nil, err
		}
	}
	if err == nil {
		//deserialize the map into the property bag
		jsonData := cm.Data["adam-config.json"]
		jsonErr := json.Unmarshal([]byte(jsonData), &bag)
		if jsonErr != nil {
			return nil, nil, fmt.Errorf("Enable to retrieve the configMap. Cannot deserialize. %v", jsonErr)
		}
		bag.SecretVersion = secret.ResourceVersion
	}

	return cm, bag, nil

}

func (r *ReconcileAdam) ensureSSLConfig(clusterAgent *appdynamicsv1alpha1.Adam) error {

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

	//	//create controller config map for ssl store credentials to be used for app agent instrumentation
	//	xml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
	//<controller-info>
	//    <controller-host></controller-host>
	//    <controller-port></controller-port>
	//    <controller-ssl-enabled></controller-ssl-enabled>
	//    <use-simple-hostname>false</use-simple-hostname>
	//    <application-name></application-name>
	//    <tier-name></tier-name>
	//    <node-name></node-name>
	//    <agent-runtime-dir></agent-runtime-dir>
	//    <enable-orchestration>false</enable-orchestration>
	//    <use-encrypted-credentials></use-encrypted-credentials>
	//    <credential-store-filename></credential-store-filename>
	//    <credential-store-password></credential-store-password>
	//    <use-ssl-client-auth>false</use-ssl-client-auth>
	//    <asymmetric-keystore-filename></asymmetric-keystore-filename>
	//    <asymmetric-keystore-password></asymmetric-keystore-password>
	//    <asymmetric-key-password></asymmetric-key-password>
	//    <asymmetric-key-alias></asymmetric-key-alias>
	//    <account-name></account-name>
	//    <account-access-key></account-access-key>
	//    <force-agent-registration>false</force-agent-registration>
	//    <controller-keystore-password>../../conf/%s</controller-keystore-password>
	//    <controller-keystore-filename>%s</controller-keystore-filename>
	//</controller-info>`, clusterAgent.Spec.AgentSSLStoreName, clusterAgent.Spec.AgentSSLPassword)

	//	cm := &corev1.ConfigMap{}
	//	err := r.client.Get(context.TODO(), types.NamespacedName{Name: AGENT_SSL_CONFIG_NAME, Namespace: clusterAgent.Namespace}, cm)

	//	create := err != nil && errors.IsNotFound(err)
	//	if err == nil {
	//		e := r.client.Delete(context.TODO(), cm)
	//		if e != nil {
	//			return fmt.Errorf("Unable to delete the old agent SSL configMap. %v", e)
	//		}
	//	}
	//	if err != nil && !errors.IsNotFound(err) {
	//		return fmt.Errorf("Unable to load agent SSL configMap. %v", err)
	//	}

	//	fmt.Printf("Recreating agent SSL Config Map\n")

	//	cm.Name = AGENT_SSL_CONFIG_NAME
	//	cm.Namespace = clusterAgent.Namespace
	//	cm.Data = make(map[string]string)
	//	cm.Data[clusterAgent.Spec.AgentSSLStoreName] = string(xml)

	//	if create {
	//		e := r.client.Create(context.TODO(), cm)
	//		if e != nil {
	//			return fmt.Errorf("Unable to create agent SSL configMap. %v", e)
	//		}
	//	} else {
	//		e := r.client.Update(context.TODO(), cm)
	//		if e != nil {
	//			return fmt.Errorf("Unable to re-create agent SSL configMap. %v", e)
	//		}
	//	}

	fmt.Println("SSL Configmap verified")
	return nil
}

func (r *ReconcileAdam) updateMap(cm *corev1.ConfigMap, clusterAgent *appdynamicsv1alpha1.Adam, secret *corev1.Secret, create bool) (*appdynamicsv1alpha1.AppDBag, error) {
	bag := appdynamicsv1alpha1.GetDefaultProperties()

	reconcileBag(bag, clusterAgent, secret)
	if create {
		bag.InstrumentationUpdated = false
	}

	data, errJson := json.Marshal(bag)
	if errJson != nil {
		return nil, fmt.Errorf("Enable to create configMap. Cannot serialize the config Bag. %v", errJson)
	}
	cm.Data = make(map[string]string)
	cm.Data["adam-config.json"] = string(data)
	var e error
	if create {
		e = r.client.Create(context.TODO(), cm)
		fmt.Printf("Configmap created. Error = %v\n", e)
	} else {
		e = r.client.Update(context.TODO(), cm)
		fmt.Printf("Configmap updated. Error = %v\n", e)
	}

	if e != nil {
		return nil, fmt.Errorf("Failed to save configMap adam-config. %v", e)
	}
	return bag, nil
}

func (r *ReconcileAdam) newAgentDeployment(clusterAgent *appdynamicsv1alpha1.Adam, secret *corev1.Secret, bag *appdynamicsv1alpha1.AppDBag) *appsv1.Deployment {
	if clusterAgent.Spec.Image == "" {
		clusterAgent.Spec.Image = "appdynamics/cluster-agent:latest"
	}
	fmt.Printf("Building deployment spec for image %s\n", clusterAgent.Spec.Image)
	ls := labelsForClusteragent(clusterAgent)
	var replicas int32 = 1
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
					ServiceAccountName: "appdynamics-operator",
					Containers: []corev1.Container{{
						Env: []corev1.EnvVar{
							{
								Name: "APPDYNAMICS_REST_API_CREDENTIALS",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{Name: AGENT_SECRET_NAME},
										Key:                  "api-user",
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
						},
						Image:           clusterAgent.Spec.Image,
						ImagePullPolicy: corev1.PullAlways,
						Name:            "cluster-agent",
						Resources:       clusterAgent.Spec.Resources,
						Ports: []corev1.ContainerPort{{
							ContainerPort: bag.AgentServerPort,
							Protocol:      corev1.ProtocolTCP,
							Name:          "web-port",
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "agent-config",
							MountPath: "/opt/appdynamics/config/",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "agent-config",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "adam-config",
								},
							},
						},
					}},
				},
			},
		},
	}

	//add more env vars, if the secret has they keys
	if _, ok := secret.Data["controller-key"]; ok {
		controllerVar := corev1.EnvVar{
			Name: "APPDYNAMICS_AGENT_ACCOUNT_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: AGENT_SECRET_NAME},
					Key:                  "controller-key",
				},
			},
		}

		dep.Spec.Template.Spec.Containers[0].Env = append(dep.Spec.Template.Spec.Containers[0].Env, controllerVar)
	}

	if _, ok := secret.Data["event-key"]; ok {
		eventVar := corev1.EnvVar{
			Name: "APPDYNAMICS_EVENT_ACCESS_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: AGENT_SECRET_NAME},
					Key:                  "event-key",
				},
			},
		}
		dep.Spec.Template.Spec.Containers[0].Env = append(dep.Spec.Template.Spec.Containers[0].Env, eventVar)
	}

	//mount custom SSL cert if necessary
	if clusterAgent.Spec.AgentSSLCert != "" {
		if clusterAgent.Spec.CustomSSLConfigMap == "" {
			clusterAgent.Spec.CustomSSLConfigMap = "cluster-agent-ssl-config"
		}
		sslVol := corev1.Volume{
			Name: "agent-ssl-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: clusterAgent.Spec.CustomSSLConfigMap,
					},
				},
			},
		}
		dep.Spec.Template.Spec.Volumes = append(dep.Spec.Template.Spec.Volumes, sslVol)

		sslMount := corev1.VolumeMount{
			Name:      "agent-ssl-config",
			MountPath: fmt.Sprintf("/opt/appdynamics/ssl/%s", clusterAgent.Spec.AgentSSLCert),
			SubPath:   clusterAgent.Spec.AgentSSLCert,
		}
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = append(dep.Spec.Template.Spec.Containers[0].VolumeMounts, sslMount)
	}

	// Set Cluster Agent instance as the owner and controller
	controllerutil.SetControllerReference(clusterAgent, dep, r.scheme)
	return dep
}

func (r *ReconcileAdam) restartAgent(clusterAgent *appdynamicsv1alpha1.Adam) error {
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

func labelsForClusteragent(clusterAgent *appdynamicsv1alpha1.Adam) map[string]string {
	return map[string]string{"name": "clusterAgent", "clusterAgent_cr": clusterAgent.Name}
}
