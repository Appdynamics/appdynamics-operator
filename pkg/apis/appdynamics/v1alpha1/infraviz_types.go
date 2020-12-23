package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// InfraVizSpec defines the desired state of InfraViz
// +k8s:openapi-gen=true
type InfraVizSpec struct {
	ControllerUrl         string                      `json:"controllerUrl"`
	Account               string                      `json:"account,omitempty"`
	Image                 string                      `json:"image,omitempty"`
	ImageWin              string                      `json:"imageWin,omitempty"`
	AccessSecret          string                      `json:"accessSecret,omitempty"`
	GlobalAccount         string                      `json:"globalAccount,omitempty"`
	EventServiceUrl       string                      `json:"eventServiceUrl,omitempty"`
	EnableContainerHostId string                      `json:"enableContainerHostId,omitempty"`
	EnableDockerViz       string                      `json:"enableDockerViz,omitempty"`
	EnableServerViz       string                      `json:"enableServerViz,omitempty"`
	EnableMasters         bool                        `json:"enableMasters,omitempty"`
	UniqueHostId          string                      `json:"uniqueHostId,omitempty"`
	MetricsLimit          string                      `json:"metricsLimit,omitempty"`
	ProxyUrl              string                      `json:"proxyUrl,omitempty"`
	ProxyUser             string                      `json:"proxyUser,omitempty"`
	LogLevel              string                      `json:"logLevel,omitempty"`
	SyslogPort            int32                       `json:"syslogPort,omitempty"`
	Pks                   bool                        `json:"pks,omitempty"`
	NetVizPort            int32                       `json:"netVizPort,omitempty"`
	NetVizImage           string                      `json:"netVizImage,omitempty"`
	NetlibEnabled         int32                       `json:"netlibEnabled,omitempty"`
	BiqPort               int32                       `json:"biqPort,omitempty"`
	StdoutLogging         bool                        `json:"stdoutLogging,omitempty"`
	AgentSSLStoreName     string                      `json:"agentSSLStoreName,omitempty"`
	AgentSSLPassword      string                      `json:"agentSSLPassword,omitempty"`
	PropertyBag           string                      `json:"propertyBag,omitempty"`
	NodeSelector          map[string]string           `json:"nodeSelector,omitempty"`
	Tolerations           []corev1.Toleration         `json:"tolerations,omitempty"`
	Args                  []string                    `json:"args,omitempty"`
	Env                   []corev1.EnvVar             `json:"env,omitempty"`
	Ports                 []corev1.ContainerPort      `json:"ports,omitempty"`
	Resources             corev1.ResourceRequirements `json:"resources,omitempty"`
	PriorityClassName     string                      `json:"priorityClassName,omitempty"`
	ImagePullSecret       string                      `json:"imagePullSecret,omitempty"`
	NodeOS                string                      `json:"nodeOS,omitempty"`
}

// InfraVizStatus defines the observed state of InfraViz
// +k8s:openapi-gen=true
type InfraVizStatus struct {
	LastUpdateTime metav1.Time       `json:"lastUpdateTime"`
	Version        string            `json:"version"`
	Nodes          map[string]string `json:"nodes,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfraViz is the Schema for the infravizs API
// +k8s:openapi-gen=true
type InfraViz struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InfraVizSpec   `json:"spec,omitempty"`
	Status InfraVizStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// InfraVizList contains a list of InfraViz
type InfraVizList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []InfraViz `json:"items"`
}

func init() {
	SchemeBuilder.Register(&InfraViz{}, &InfraVizList{})
}
