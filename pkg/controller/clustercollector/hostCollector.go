package clustercollector

import (
	"context"
	"fmt"
	appdynamicsv1alpha1 "github.com/Appdynamics/appdynamics-operator/pkg/apis/appdynamics/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	HOST_COLLECTOR_NAME            string = "k8s-host-collector"
	INFRA_AGENT_VOLUME_NAME        string = "infraagent-config"
	CONTAINER_MON_VOLUME_NAME      string = "containermon-config"
	SERVER_MON_VOLUME_NAME         string = "servermon-config"
	INFRA_AGENT_HOME_PATH          string = "/opt/appdynamics/InfraAgent"
	INFRA_AGENT_CONFIG_FILE_NAME   string = "agent.conf"
	COLLECTOR_PATH                 string = "/opt/appdynamics/InfraAgent/collectors"
	CONTAINER_MON_CONFIG_FILE_NAME string = "containermon.conf"
	SERVER_MON_CONFIG_FILE_NAME    string = "servermon.conf"
)

type hostCollectorController struct {
	client           client.Client
	clusterCollector *appdynamicsv1alpha1.Clustercollector
	daemonset        *appsv1.DaemonSet
}

func NewHostCollectorController(client client.Client, clusterCollector *appdynamicsv1alpha1.Clustercollector) *hostCollectorController {
	return &hostCollectorController{
		client:           client,
		clusterCollector: clusterCollector,
		daemonset:        &appsv1.DaemonSet{},
	}
}
func (h *hostCollectorController) Get() metav1.Object {
	return h.daemonset
}

func (h *hostCollectorController) Init(reqLogger logr.Logger) (bool, error) {
	err := h.initialiseDaemonSet()
	newDaemonSetCreated := false
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Cluster Collector daemonset does not exist. Creating...")
		newDaemonSetCreated = true
		// Define a new deployment for the cluster collector
		err = h.newCollectorDaemonSet()
		return newDaemonSetCreated, err
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Daemonset")
		return newDaemonSetCreated, err
	}
	return newDaemonSetCreated, err
}

func (h *hostCollectorController) Create(reqLogger logr.Logger) error {
	reqLogger.Info("Creating a new Daemonset", "Daemonset.Namespace", h.daemonset.Namespace, "Daemonset.Name", h.daemonset.Name)
	err := h.client.Create(context.TODO(), h.daemonset)
	if err != nil {
		reqLogger.Error(err, "Failed to create new Daemonset", "Daemonset.Namespace", h.daemonset.Namespace, "Daemonset.Name", h.daemonset.Name)
		return err
	}
	reqLogger.Info("Daemonset created successfully. Done")
	updateStatus(h.clusterCollector, h.client)
	return nil
}

func (h *hostCollectorController) Update(reqLogger logr.Logger) (bool, error) {
	breaking, updateDeployment := hasBreakingChanges(h.clusterCollector, h.daemonset, &h.daemonset.Spec.Template)
	reQueue := false
	existingDaemonSet := h.daemonset
	clusterCollector := h.clusterCollector
	if breaking {
		fmt.Println("Breaking changes detected. Restarting the host collector pod...")

		saveOrUpdateCollectorSpecAnnotation(h.daemonset, h.clusterCollector)

		errUpdate := h.client.Update(context.TODO(), existingDaemonSet)
		if errUpdate != nil {
			reqLogger.Error(errUpdate, "Failed to update host collector", "hostCollector.Namespace", clusterCollector.Namespace, "Daemonset.Name", clusterCollector.Name)
			return reQueue, errUpdate
		}

		errRestart := h.RestartCollector()
		if errRestart != nil {
			reqLogger.Error(errRestart, "Failed to restart host collector", "hostCollector.Namespace", clusterCollector.Namespace, "Daemonset.Name", clusterCollector.Name)
			return reQueue, errRestart
		}
	} else if updateDeployment {
		fmt.Println("Non-Breaking changes detected. Updating the the host collector deployment...")
		err := h.client.Update(context.TODO(), existingDaemonSet)
		if err != nil {
			reqLogger.Error(err, "Failed to update Hostcollector Daemonset", "Daemonset.Namespace", existingDaemonSet.Namespace, "Daemonset.Name", existingDaemonSet.Name)
			return reQueue, err
		}
	} else {
		reqLogger.Info("No breaking changes.", "HostCollector.Namespace", clusterCollector.Namespace)
		statusErr := updateStatus(clusterCollector, h.client)
		if statusErr == nil {
			reqLogger.Info("Status updated. Exiting reconciliation loop.")
		} else {
			reqLogger.Info("Status not updated. Exiting reconciliation loop.")
		}
		return reQueue, nil
	}
	reQueue = true
	return reQueue, nil
}

func (h *hostCollectorController) RestartCollector() error {
	podList := &corev1.PodList{}
	clusterCollector := h.clusterCollector
	labelSelector := labels.SelectorFromSet(labelsForHostCollector(clusterCollector))
	listOps := &client.ListOptions{
		Namespace:     clusterCollector.Namespace,
		LabelSelector: labelSelector,
	}
	err := h.client.List(context.TODO(), listOps, podList)
	if err != nil || len(podList.Items) < 1 {
		return fmt.Errorf("Unable to retrieve host-collector pod. %v", err)
	}
	pod := podList.Items[0]
	//delete to force restart
	err = h.client.Delete(context.TODO(), &pod)
	if err != nil {
		return fmt.Errorf("Unable to delete host-collector pod. %v", err)
	}
	return nil
}
func (h *hostCollectorController) initialiseDaemonSet() error {
	err := h.client.Get(context.TODO(), types.NamespacedName{Name: HOST_COLLECTOR_NAME,
		Namespace: h.clusterCollector.Namespace}, h.daemonset)
	return err
}

func (h *hostCollectorController) newCollectorDaemonSet() error {
	trueVal := true
	clusterCollector := h.clusterCollector

	fmt.Printf("Building DaemonSet spec for image %s\n", clusterCollector.Spec.Image)
	ls := labelsForHostCollector(clusterCollector)
	ds := &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterCollector.Spec.HostCollector.Name,
			Namespace: clusterCollector.Namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: clusterCollector.Spec.HostCollector.ServiceAccountName,
					Containers: []corev1.Container{{
						Image:           clusterCollector.Spec.HostCollector.Image,
						ImagePullPolicy: corev1.PullAlways,
						Name:            "host-collector",
						Resources:       clusterCollector.Spec.HostCollector.Resources,
						SecurityContext: &corev1.SecurityContext{
							Privileged: &trueVal,
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "proc",
							MountPath: "/host/proc",
							ReadOnly:  true,
						}, {
							Name:      "var-run",
							MountPath: "/var/run",
						}, {
							Name:      "sys",
							MountPath: "/sys",
							ReadOnly:  true,
						}, {
							Name:      "root",
							MountPath: "/rootfs",
							ReadOnly:  true,
						}, {
							Name:      "var-lib-docker",
							MountPath: "/var/lib/docker/",
							ReadOnly:  true,
						}, {
							Name:      INFRA_AGENT_VOLUME_NAME,
							MountPath: fmt.Sprintf("%s/%s", INFRA_AGENT_HOME_PATH, INFRA_AGENT_CONFIG_FILE_NAME),
							SubPath:   INFRA_AGENT_CONFIG_FILE_NAME,
						}, {
							Name:      CONTAINER_MON_VOLUME_NAME,
							MountPath: fmt.Sprintf("%s/%s", COLLECTOR_PATH, CONTAINER_MON_CONFIG_FILE_NAME),
							SubPath:   CONTAINER_MON_CONFIG_FILE_NAME,
						}, {
							Name:      SERVER_MON_VOLUME_NAME,
							MountPath: fmt.Sprintf("%s/%s", COLLECTOR_PATH, SERVER_MON_CONFIG_FILE_NAME),
							SubPath:   SERVER_MON_CONFIG_FILE_NAME,
						},
						},
					}},
					Volumes: []corev1.Volume{{
						Name: "proc",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{Path: "/proc"},
						},
					}, {
						Name: "var-run",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{Path: "/var/run"},
						},
					}, {
						Name: "sys",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{Path: "/sys"},
						},
					}, {
						Name: "root",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{Path: "/"},
						},
					}, {
						Name: "var-lib-docker",
						VolumeSource: corev1.VolumeSource{
							HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/docker/"},
						},
					}, {
						Name: INFRA_AGENT_VOLUME_NAME,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: INFRA_AGENT_CONFIG_MAP_NAME,
								},
							},
						},
					}, {
						Name: CONTAINER_MON_VOLUME_NAME,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: CONTAINER_MON_CONFIG_MAP_NAME,
								},
							},
						},
					}, {
						Name: SERVER_MON_VOLUME_NAME,
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: SERVER_MON_CONFIG_MAP_NAME,
								},
							},
						},
					}},
				},
			},
		},
	}

	//save the new spec in annotations
	saveOrUpdateCollectorSpecAnnotation(h.daemonset, h.clusterCollector)
	h.daemonset = ds
	// Set Host collector instance as the owner and controller
	return nil
}

func labelsForHostCollector(hostCollector *appdynamicsv1alpha1.Clustercollector) map[string]string {
	return map[string]string{"name": "hostCollector", "hostCollector_cr": hostCollector.Spec.HostCollector.Name}
}
