package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClustercollectorSpec defines the desired state of Clustercollector
type ClustercollectorSpec struct {
	Image              string                      `json:"image,omitempty"`
	ServiceAccountName string                      `json:"serviceAccountName,omitempty"`
	Resources          corev1.ResourceRequirements `json:"resources,omitempty"`
	ControllerUrl      string                      `json:"controllerUrl"`
	Account            string                      `json:"account,omitempty"`
	ClusterName        string                      `json:"clusterName,omitempty"`
	AccessSecret       string                      `json:"accessSecret,omitempty"`
	NsToMonitorRegex   string                      `json:"nsToMonitorRegex,omitempty"`
	NsToExcludeRegex   string                      `json:"nsToExcludeRegex,omitempty"`
	LogLevel           string                      `json:"logLevel,omitempty"`
	ClusterMonEnabled  bool                        `json:"clusterMonEnabled,omitempty"`
	SystemConfigs      InfraAgentConfig            `json:"systemConfig,omitempty"`
	ExporterAddress    string                      `json:"kubeExporterAddress,omitempty"`
	ExporterPort       int                         `json:"kubeExporterPort,omitempty"`
	HostCollector      HostCollectorConfig         `json:"hostCollectorConfig,omitempty"`
}

type InfraAgentConfig struct {
	CollectorLibSocketUrl  string `json:"collectorLibSocketUrl,omitempty"`
	CollectorLibPort       string `json:"collectorLibPort,omitempty"`
	HttpClientTimeOut      int    `json:"httpClientTimeOut,omitempty"`
	HttpBasicAuthEnabled   bool   `json:"httpBasicAuthEnabled,omitempty"`
	ConfigChangeScanPeriod int    `json:"configChangeScanPeriod,omitempty"`
	ConfigStaleGracePeriod int    `json:"configStaleGracePeriod,omitempty"`
	DebugEnabled           bool   `json:"debugEnabled,omitempty"`
	DebugPort              string `json:"debugPort,omitempty"`
	LogLevel               string `json:"logLevel,omitempty"`
	ClientLibSendUrl       string `json:"clientLibSendUrl,omitempty"`
	ClientLibRecvUrl       string `json:"clientLibRecvUrl,omitempty"`
}

type HostCollectorConfig struct {
	Name                           string                      `json:"name,omitempty"`
	Image                          string                      `json:"image,omitempty"`
	ServiceAccountName             string                      `json:"serviceAccountName,omitempty"`
	Resources                      corev1.ResourceRequirements `json:"resources,omitempty"`
	ContainerMetricExporterAddress string                      `json:"containerMetricExporterAddr,omitempty"`
	ServerMetricExporterAddress    string                      `json:"serverMetricExporterAddr,omitempty"`
	LogLevel                       string                      `json:"logLevel,omitempty"`
	ContainerCollectorPath         string                      `json:"containerCollectorPath,omitempty"`
	ServerCollectorPath            string                      `json:"serverCollectorPath,omitempty"`
	ContainerCollectorDependency   string                      `json:"containerCollectorDependency,omitempty"`
	ServerCollectorDependency      string                      `json:"serverCollectorDependency,omitempty"`
	ContainerMonEnabled            string                      `json:"containerMonEnabled,omitempty"`
	ServerMonEnabled               string                      `json:"serverMonEnabled,omitempty"`
}

// ClustercollectorStatus defines the observed state of Clustercollector
type ClustercollectorStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
	State          AgentStatus `json:"state"`
}

// Clustercollector is the Schema for the clusteragents API
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Clustercollector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClustercollectorSpec   `json:"spec,omitempty"`
	Status ClustercollectorStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// ClustercollectorList contains a list of Clustercollector
type ClustercollectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Clustercollector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Clustercollector{}, &ClustercollectorList{})
}
