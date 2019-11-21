package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusteragentSpec defines the desired state of Clusteragent
type ClusteragentSpec struct {
	//account info
	ControllerUrl   string                      `json:"controllerUrl"`
	Account         string                      `json:"account,omitempty"`
	GlobalAccount   string                      `json:"globalAccount,omitempty"`
	EventServiceUrl string                      `json:"eventServiceUrl,omitempty"`
	Image           string                      `json:"image,omitempty"`
	Args            []string                    `json:"args,omitempty"`
	Env             []corev1.EnvVar             `json:"env,omitempty"`
	Resources       corev1.ResourceRequirements `json:"resources,omitempty"`
	AppName         string                      `json:"appName,omitempty"`
	AgentServerPort int32                       `json:"agentServerPort,omitempty"`
	SystemSSLCert   string                      `json:"systemSSLCert,omitempty"`
	AgentSSLCert    string                      `json:"agentSSLCert,omitempty"`
	CustomSSLSecret string                      `json:"customSSLSecret,omitempty"`
	ProxyUrl        string                      `json:"proxyUrl,omitempty"`
	ProxyUser       string                      `json:"proxyUser,omitempty"`
	ProxyPass       string                      `json:"proxyPass,omitempty"`

	//limits
	EventAPILimit                 int    `json:"eventAPILimit,omitempty"`
	MetricsSyncInterval           int    `json:"metricsSyncInterval,omitempty"`
	ClusterMetricsSyncInterval    int    `json:"clusterMetricsSyncInterval,omitempty"`
	MetadataSyncInterval          int    `json:"metadataSyncInterval,omitempty"`
	SnapshotSyncInterval          int    `json:"snapshotSyncInterval,omitempty"`
	EventUploadInterval           int    `json:"eventUploadInterval,omitempty"`
	ContainerRegistrationInterval int    `json:"containerRegistrationInterval,omitempty"`
	HttpClientTimeout             int    `json:"httpClientTimeout,omitempty"`
	ContainerBatchSize            int    `json:"containerBatchSize,omitempty"`
	ContainerParallelRequestLimit int    `json:"containerParallelRequestLimit,omitempty"`
	PodBatchSize                  int    `json:"podBatchSize,omitempty"`
	LogLines                      int    `json:"logLines,omitempty"`
	LogLevel                      string `json:"logLevel,omitempty"`
	LogFileSizeMb                 int    `json:"logFileSizeMb,omitempty"`
	LogFileBackups                int    `json:"logFileBackups,omitempty"`
	StdoutLogging                 string `json:"stdoutLogging,omitempty"`
	PodEventNumber                int    `json:"podEventNumber,omitempty"`
	OverconsumptionThreshold      int    `json:"overconsumptionThreshold,omitempty"`

	//instrumentation
	InstrumentationMethod       string                      `json:"instrumentationMethod,omitempty"`
	InstrumentMatchString       []string                    `json:"instrumentMatchString,omitempty"`
	DefaultInstrumentationTech  string                      `json:"defaultInstrumentationTech,omitempty"`
	NsToInstrument              []string                    `json:"nsToInstrument,omitempty"`
	NsToInstrumentExclude       []string                    `json:"nsToInstrumentExclude,omitempty"`
	NsToMonitor                 []string                    `json:"nsToMonitor,omitempty"`
	NsToMonitorExclude          []string                    `json:"nsToMonitorExclude,omitempty"`
	NodesToMonitor              []string                    `json:"nodesToMonitor,omitempty"`
	NodesToMonitorExclude       []string                    `json:"nodesToMonitorExclude,omitempty"`
	InstrumentRule              []AgentRequest              `json:"instrumentRule,omitempty"`
	AnalyticsAgentImage         string                      `json:"analyticsAgentImage,omitempty"`
	AppDJavaAttachImage         string                      `json:"appDJavaAttachImage,omitempty"`
	AppDDotNetAttachImage       string                      `json:"appDDotNetAttachImage,omitempty"`
	BiqService                  string                      `json:"biqService,omitempty"`
	InstrumentContainer         string                      `json:"instrumentContainer,omitempty"`
	InitContainerDir            string                      `json:"initContainerDir,omitempty"`
	AgentLabel                  string                      `json:"agentLabel,omitempty"`
	AgentLogOverride            string                      `json:"agentLogOverride,omitempty"`
	AgentUserOverride           string                      `json:"agentUserOverride,omitempty"`
	AgentEnvVar                 string                      `json:"agentEnvVar,omitempty"`
	AgentOpts                   string                      `json:"agentOpts,omitempty"`
	AppNameLiteral              string                      `json:"appNameLiteral,omitempty"`
	AppDAppLabel                string                      `json:"appDAppLabel,omitempty"`
	AppDTierLabel               string                      `json:"appDTierLabel,omitempty"`
	AppDAnalyticsLabel          string                      `json:"appDAnalyticsLabel,omitempty"`
	AgentMountName              string                      `json:"agentMountName,omitempty"`
	AgentMountPath              string                      `json:"agentMountPath,omitempty"`
	AppLogMountName             string                      `json:"appLogMountName,omitempty"`
	AppLogMountPath             string                      `json:"appLogMountPath,omitempty"`
	JDKMountName                string                      `json:"jDKMountName,omitempty"`
	JDKMountPath                string                      `json:"jDKMountPath,omitempty"`
	NodeNamePrefix              string                      `json:"nodeNamePrefix,omitempty"`
	AnalyticsAgentUrl           string                      `json:"analyticsAgentUrl,omitempty"`
	AnalyticsAgentContainerName string                      `json:"analyticsAgentContainerName,omitempty"`
	AppDInitContainerName       string                      `json:"appDInitContainerName,omitempty"`
	NetVizPort                  int                         `json:"netVizPort,omitempty"`
	InitRequestMem              string                      `json:"initRequestMem,omitempty"`
	InitRequestCpu              string                      `json:"initRequestCpu,omitempty"`
	BiqRequestMem               string                      `json:"biqRequestMem,omitempty"`
	BiqRequestCpu               string                      `json:"biqRequestCpu,omitempty"`
	UniqueHostID                string                      `json:"uniqueHostID,omitempty"`
	AgentSSLStoreName           string                      `json:"agentSSLStoreName,omitempty"`
	AgentSSLPassword            string                      `json:"agentSSLPassword,omitempty"`
	ContainerFilter             ClusteragentContainerFilter `json:"containerFilter,omitempty"`

	//snapshot schemas
	PodSchemaName       string `json:"podSchemaName,omitempty"`
	NodeSchemaName      string `json:"nodeSchemaName,omitempty"`
	EventSchemaName     string `json:"eventSchemaName,omitempty"`
	ContainerSchemaName string `json:"containerSchemaName,omitempty"`
	JobSchemaName       string `json:"jobSchemaName,omitempty"`
	LogSchemaName       string `json:"logSchemaName,omitempty"`
	EpSchemaName        string `json:"epSchemaName,omitempty"`
	NsSchemaName        string `json:"nsSchemaName,omitempty"`
	RqSchemaName        string `json:"rqSchemaName,omitempty"`
	DeploySchemaName    string `json:"deploySchemaName,omitempty"`
	RSSchemaName        string `json:"rSSchemaName,omitempty"`
	DaemonSchemaName    string `json:"daemonSchemaName,omitempty"`

	//dashboard
	DashboardSuffix       string `json:"dashboardSuffix,omitempty"`
	DashboardDelayMin     int    `json:"dashboardDelayMin,omitempty"`
	DashboardTemplatePath string `json:"dashboardTemplatePath,omitempty"`
}

type ClusteragentContainerFilter struct {
	WhitelistedNames  map[string][]string `json:"whitelistedNames,omitempty"`
	BlacklistedNames  map[string][]string `json:"blacklistedNames,omitempty"`
	BlacklistedLabels map[string]string   `json:"blacklistedLabels,omitempty"`
}

// ClusteragentStatus defines the observed state of Clusteragent
type ClusteragentStatus struct {
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	LastUpdateTime metav1.Time `json:"lastUpdateTime"`
	State          AgentStatus `json:"state"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Clusteragent is the Schema for the clusteragents API
// +k8s:openapi-gen=true
type Clusteragent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusteragentSpec   `json:"spec,omitempty"`
	Status ClusteragentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusteragentList contains a list of Clusteragent
type ClusteragentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Clusteragent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Clusteragent{}, &ClusteragentList{})
}
