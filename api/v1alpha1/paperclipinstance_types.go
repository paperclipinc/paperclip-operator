/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InstanceSpec defines the desired state of a Paperclip instance.
type InstanceSpec struct {
	// Image specifies the Paperclip container image to deploy.
	// +optional
	Image ImageSpec `json:"image,omitempty"`

	// Deployment controls the deployment mode and exposure settings.
	// +optional
	Deployment DeploymentSpec `json:"deployment,omitempty"`

	// Database configures the PostgreSQL connection.
	// +optional
	Database DatabaseSpec `json:"database,omitempty"`

	// Auth configures authentication settings.
	// +optional
	Auth AuthSpec `json:"auth,omitempty"`

	// Secrets configures the Paperclip secrets management system.
	// +optional
	Secrets SecretsSpec `json:"secrets,omitempty"`

	// Storage configures persistent storage for the Paperclip data directory.
	// +optional
	Storage StorageSpec `json:"storage,omitempty"`

	// ObjectStorage configures S3-compatible object storage for multi-replica deployments.
	// +optional
	ObjectStorage *ObjectStorageSpec `json:"objectStorage,omitempty"`

	// Heartbeat configures the agent heartbeat scheduler.
	// +optional
	Heartbeat HeartbeatSpec `json:"heartbeat,omitempty"`

	// Adapters configures agent runtime adapters.
	// +optional
	Adapters AdaptersSpec `json:"adapters,omitempty"`

	// Plugins lists plugins to install.
	// +optional
	Plugins []PluginRef `json:"plugins,omitempty"`

	// Env specifies additional environment variables for the Paperclip container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// EnvFrom specifies additional environment variable sources for the Paperclip container.
	// +optional
	EnvFrom []corev1.EnvFromSource `json:"envFrom,omitempty"`

	// Resources specifies the compute resources for the Paperclip container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Security configures pod and container security settings.
	// +optional
	Security SecuritySpec `json:"security,omitempty"`

	// Networking configures service, ingress, and WebSocket settings.
	// +optional
	Networking NetworkingSpec `json:"networking,omitempty"`

	// Observability configures metrics, logging, and monitoring.
	// +optional
	Observability ObservabilitySpec `json:"observability,omitempty"`

	// Availability configures scaling, PDB, and pod scheduling.
	// +optional
	Availability AvailabilitySpec `json:"availability,omitempty"`

	// Probes configures liveness, readiness, and startup probes.
	// +optional
	Probes ProbesSpec `json:"probes,omitempty"`

	// Backup configures periodic backup to S3-compatible storage.
	// +optional
	Backup *BackupSpec `json:"backup,omitempty"`

	// RestoreFrom specifies a remote backup path to restore from on first boot.
	// +optional
	RestoreFrom string `json:"restoreFrom,omitempty"`

	// Sidecars specifies additional sidecar containers.
	// +optional
	Sidecars []corev1.Container `json:"sidecars,omitempty"`

	// InitContainers specifies additional init containers.
	// +optional
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// ExtraVolumes specifies additional volumes to add to the pod.
	// +optional
	ExtraVolumes []corev1.Volume `json:"extraVolumes,omitempty"`

	// ExtraVolumeMounts specifies additional volume mounts for the Paperclip container.
	// +optional
	ExtraVolumeMounts []corev1.VolumeMount `json:"extraVolumeMounts,omitempty"`

	// PodAnnotations specifies additional annotations for the pod template.
	// +optional
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
}

// ImageSpec configures the container image.
type ImageSpec struct {
	// Repository is the container image repository.
	// +kubebuilder:default="ghcr.io/paperclipinc/paperclip"
	// +optional
	Repository string `json:"repository,omitempty"`

	// Tag is the container image tag.
	// +kubebuilder:default="latest"
	// +optional
	Tag string `json:"tag,omitempty"`

	// Digest overrides the tag with an image digest (e.g. sha256:abc...).
	// +optional
	Digest string `json:"digest,omitempty"`

	// PullPolicy specifies the image pull policy.
	// +kubebuilder:default="IfNotPresent"
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	// +optional
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`

	// PullSecrets specifies image pull secrets.
	// +optional
	PullSecrets []corev1.LocalObjectReference `json:"pullSecrets,omitempty"`
}

// DeploymentSpec controls deployment mode and exposure.
type DeploymentSpec struct {
	// Mode sets the deployment mode: "open" (no auth), "authenticated" (login required), or "single-tenant".
	// +kubebuilder:default="authenticated"
	// +kubebuilder:validation:Enum=open;authenticated;"single-tenant"
	// +optional
	Mode string `json:"mode,omitempty"`

	// Exposure controls network exposure: "private" (ClusterIP only) or "public" (Ingress/LoadBalancer).
	// +kubebuilder:default="private"
	// +kubebuilder:validation:Enum=private;public
	// +optional
	Exposure string `json:"exposure,omitempty"`

	// PublicURL is the externally-reachable URL for the Paperclip instance.
	// Required when exposure is "public".
	// +optional
	PublicURL string `json:"publicURL,omitempty"`

	// AllowedHostnames is a list of allowed hostnames for CORS.
	// +optional
	AllowedHostnames []string `json:"allowedHostnames,omitempty"`
}

// DatabaseSpec configures PostgreSQL.
type DatabaseSpec struct {
	// Mode selects the database mode: "embedded" (PGlite), "external" (connection string), or "managed" (operator-managed StatefulSet).
	// +kubebuilder:default="managed"
	// +kubebuilder:validation:Enum=embedded;external;managed
	// +optional
	Mode string `json:"mode,omitempty"`

	// ExternalURL is the PostgreSQL connection string for external mode.
	// +optional
	ExternalURL string `json:"externalURL,omitempty"`

	// ExternalURLSecretRef references a Secret containing the DATABASE_URL key.
	// +optional
	ExternalURLSecretRef *corev1.SecretKeySelector `json:"externalURLSecretRef,omitempty"`

	// Managed configures the operator-managed PostgreSQL StatefulSet.
	// +optional
	Managed ManagedDatabaseSpec `json:"managed,omitempty"`
}

// ManagedDatabaseSpec configures the operator-managed PostgreSQL instance.
type ManagedDatabaseSpec struct {
	// Image is the PostgreSQL container image.
	// +kubebuilder:default="postgres:17-alpine"
	// +optional
	Image string `json:"image,omitempty"`

	// StorageSize is the PVC size for PostgreSQL data.
	// +kubebuilder:default="10Gi"
	// +optional
	StorageSize resource.Quantity `json:"storageSize,omitempty"`

	// StorageClass is the storage class for the PostgreSQL PVC.
	// +optional
	StorageClass *string `json:"storageClass,omitempty"`

	// Resources specifies compute resources for the PostgreSQL container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// AuthSpec configures authentication.
type AuthSpec struct {
	// SecretRef references a Secret containing the BETTER_AUTH_SECRET key.
	// Required when deployment mode is "authenticated".
	// +optional
	SecretRef *corev1.SecretKeySelector `json:"secretRef,omitempty"`
}

// SecretsSpec configures Paperclip's built-in secrets management.
type SecretsSpec struct {
	// MasterKeySecretRef references a Secret containing the master encryption key.
	// +optional
	MasterKeySecretRef *corev1.SecretKeySelector `json:"masterKeySecretRef,omitempty"`

	// StrictMode requires all sensitive values to use encrypted references.
	// +optional
	StrictMode bool `json:"strictMode,omitempty"`
}

// StorageSpec configures persistent storage.
type StorageSpec struct {
	// Persistence configures the PVC for the Paperclip data directory (/paperclip).
	// +optional
	Persistence PersistenceSpec `json:"persistence,omitempty"`
}

// PersistenceSpec configures PVC settings.
type PersistenceSpec struct {
	// Enabled controls whether a PVC is created. Defaults to true.
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Size is the PVC storage size.
	// +kubebuilder:default="5Gi"
	// +optional
	Size resource.Quantity `json:"size,omitempty"`

	// StorageClass is the storage class for the PVC.
	// +optional
	StorageClass *string `json:"storageClass,omitempty"`

	// AccessModes specifies the PVC access modes.
	// +optional
	AccessModes []corev1.PersistentVolumeAccessMode `json:"accessModes,omitempty"`
}

// ObjectStorageSpec configures S3-compatible object storage.
type ObjectStorageSpec struct {
	// Provider is the S3-compatible provider: "s3", "minio", "r2".
	// +kubebuilder:validation:Enum=s3;minio;r2
	Provider string `json:"provider"`

	// Bucket is the S3 bucket name.
	Bucket string `json:"bucket"`

	// Region is the S3 region.
	// +optional
	Region string `json:"region,omitempty"`

	// Endpoint is the S3-compatible endpoint URL (for MinIO/R2).
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// CredentialsSecretRef references a Secret containing AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY.
	// +optional
	CredentialsSecretRef *corev1.LocalObjectReference `json:"credentialsSecretRef,omitempty"`
}

// HeartbeatSpec configures the agent heartbeat scheduler.
type HeartbeatSpec struct {
	// Enabled controls whether the heartbeat scheduler runs. Defaults to true.
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// IntervalMS sets the heartbeat interval in milliseconds.
	// +kubebuilder:default=60000
	// +optional
	IntervalMS int32 `json:"intervalMS,omitempty"`
}

// AdaptersSpec configures agent runtime adapters.
type AdaptersSpec struct {
	// APIKeys references Secrets containing LLM provider API keys.
	// The Secret should contain keys like ANTHROPIC_API_KEY, OPENAI_API_KEY, etc.
	// +optional
	APIKeysSecretRef *corev1.LocalObjectReference `json:"apiKeysSecretRef,omitempty"`
}

// PluginRef references a Paperclip plugin.
type PluginRef struct {
	// Name is the plugin package name.
	Name string `json:"name"`

	// Version is the plugin version.
	// +optional
	Version string `json:"version,omitempty"`
}

// SecuritySpec configures security settings.
type SecuritySpec struct {
	// PodSecurityContext specifies security settings for the pod.
	// +optional
	PodSecurityContext *corev1.PodSecurityContext `json:"podSecurityContext,omitempty"`

	// ContainerSecurityContext specifies security settings for the Paperclip container.
	// +optional
	ContainerSecurityContext *corev1.SecurityContext `json:"containerSecurityContext,omitempty"`

	// NetworkPolicy configures network isolation.
	// +optional
	NetworkPolicy NetworkPolicySpec `json:"networkPolicy,omitempty"`

	// RBAC configures ServiceAccount and RBAC settings.
	// +optional
	RBAC RBACSpec `json:"rbac,omitempty"`
}

// NetworkPolicySpec configures network isolation.
type NetworkPolicySpec struct {
	// Enabled controls whether a NetworkPolicy is created. Defaults to true.
	// +kubebuilder:default=true
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// AllowIngressCIDRs specifies additional CIDR blocks allowed to reach the Paperclip service.
	// +optional
	AllowIngressCIDRs []string `json:"allowIngressCIDRs,omitempty"`

	// AllowEgressCIDRs specifies additional CIDR blocks the pod can reach.
	// +optional
	AllowEgressCIDRs []string `json:"allowEgressCIDRs,omitempty"`
}

// RBACSpec configures ServiceAccount and RBAC.
type RBACSpec struct {
	// Create controls whether a ServiceAccount is created. Defaults to true.
	// +kubebuilder:default=true
	// +optional
	Create bool `json:"create,omitempty"`

	// ServiceAccountAnnotations specifies additional annotations for the ServiceAccount.
	// +optional
	ServiceAccountAnnotations map[string]string `json:"serviceAccountAnnotations,omitempty"`
}

// NetworkingSpec configures service and ingress.
type NetworkingSpec struct {
	// Service configures the Kubernetes Service.
	// +optional
	Service ServiceSpec `json:"service,omitempty"`

	// Ingress configures the Kubernetes Ingress.
	// +optional
	Ingress *IngressSpec `json:"ingress,omitempty"`
}

// ServiceSpec configures the Kubernetes Service.
type ServiceSpec struct {
	// Type is the Kubernetes Service type.
	// +kubebuilder:default="ClusterIP"
	// +kubebuilder:validation:Enum=ClusterIP;LoadBalancer;NodePort
	// +optional
	Type corev1.ServiceType `json:"type,omitempty"`

	// Port is the service port. Defaults to 3100 (Paperclip's default).
	// +kubebuilder:default=3100
	// +optional
	Port int32 `json:"port,omitempty"`

	// Annotations specifies additional annotations for the Service.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// IngressSpec configures the Kubernetes Ingress.
type IngressSpec struct {
	// Enabled controls whether an Ingress is created.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// IngressClassName specifies the Ingress class name.
	// +optional
	IngressClassName *string `json:"ingressClassName,omitempty"`

	// Hosts specifies the Ingress hostnames.
	// +optional
	Hosts []string `json:"hosts,omitempty"`

	// TLS configures TLS for the Ingress.
	// +optional
	TLS []IngressTLSSpec `json:"tls,omitempty"`

	// Annotations specifies additional annotations for the Ingress.
	// Tip: Add WebSocket support annotations for your ingress controller here
	// (e.g., nginx.ingress.kubernetes.io/proxy-read-timeout: "3600").
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// IngressTLSSpec configures TLS for an Ingress host.
type IngressTLSSpec struct {
	// Hosts specifies the TLS hostnames.
	Hosts []string `json:"hosts"`

	// SecretName is the name of the TLS secret.
	SecretName string `json:"secretName"`
}

// ObservabilitySpec configures monitoring and logging.
type ObservabilitySpec struct {
	// Metrics configures Prometheus metrics.
	// +optional
	Metrics MetricsSpec `json:"metrics,omitempty"`

	// Logging configures log level and format.
	// +optional
	Logging LoggingSpec `json:"logging,omitempty"`
}

// MetricsSpec configures Prometheus metrics.
type MetricsSpec struct {
	// Enabled controls whether metrics are exposed.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// ServiceMonitor enables creating a Prometheus ServiceMonitor.
	// +optional
	ServiceMonitor *ServiceMonitorSpec `json:"serviceMonitor,omitempty"`
}

// ServiceMonitorSpec configures a Prometheus ServiceMonitor.
type ServiceMonitorSpec struct {
	// Enabled controls whether a ServiceMonitor is created.
	Enabled bool `json:"enabled"`

	// Interval specifies the scrape interval.
	// +kubebuilder:default="30s"
	// +optional
	Interval string `json:"interval,omitempty"`
}

// LoggingSpec configures logging.
type LoggingSpec struct {
	// Level sets the log level: "debug", "info", "warn", "error".
	// +kubebuilder:default="info"
	// +kubebuilder:validation:Enum=debug;info;warn;error
	// +optional
	Level string `json:"level,omitempty"`
}

// AvailabilitySpec configures scaling and pod scheduling.
type AvailabilitySpec struct {
	// PodDisruptionBudget configures the PDB.
	// +optional
	PodDisruptionBudget *PDBSpec `json:"podDisruptionBudget,omitempty"`

	// AutoScaling configures the HorizontalPodAutoscaler.
	// +optional
	AutoScaling *AutoScalingSpec `json:"autoScaling,omitempty"`

	// NodeSelector specifies node selection constraints.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations specifies pod tolerations.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity specifies pod affinity rules.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`

	// TopologySpreadConstraints specifies topology spread constraints.
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
}

// PDBSpec configures a PodDisruptionBudget.
type PDBSpec struct {
	// Enabled controls whether a PDB is created.
	Enabled bool `json:"enabled"`

	// MinAvailable specifies the minimum number of pods that must be available.
	// +optional
	MinAvailable *int32 `json:"minAvailable,omitempty"`

	// MaxUnavailable specifies the maximum number of pods that can be unavailable.
	// +optional
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`
}

// AutoScalingSpec configures a HorizontalPodAutoscaler.
type AutoScalingSpec struct {
	// Enabled controls whether an HPA is created.
	Enabled bool `json:"enabled"`

	// MinReplicas is the minimum number of replicas.
	// +kubebuilder:default=1
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the maximum number of replicas.
	// +kubebuilder:default=3
	// +optional
	MaxReplicas int32 `json:"maxReplicas,omitempty"`

	// TargetCPUUtilizationPercentage is the target CPU utilization for scaling.
	// +kubebuilder:default=80
	// +optional
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`

	// TargetMemoryUtilizationPercentage is the target memory utilization for scaling.
	// +optional
	TargetMemoryUtilizationPercentage *int32 `json:"targetMemoryUtilizationPercentage,omitempty"`
}

// ProbesSpec configures health probes.
type ProbesSpec struct {
	// Liveness configures the liveness probe against /api/health.
	// +optional
	Liveness *ProbeSpec `json:"liveness,omitempty"`

	// Readiness configures the readiness probe against /api/health.
	// +optional
	Readiness *ProbeSpec `json:"readiness,omitempty"`

	// Startup configures the startup probe against /api/health.
	// +optional
	Startup *ProbeSpec `json:"startup,omitempty"`
}

// ProbeSpec configures an individual probe.
type ProbeSpec struct {
	// InitialDelaySeconds is the number of seconds after the container starts before the probe is initiated.
	// +optional
	InitialDelaySeconds *int32 `json:"initialDelaySeconds,omitempty"`

	// PeriodSeconds is how often (in seconds) to perform the probe.
	// +optional
	PeriodSeconds *int32 `json:"periodSeconds,omitempty"`

	// TimeoutSeconds is the number of seconds after which the probe times out.
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`

	// FailureThreshold is the number of consecutive failures before the probe is considered failed.
	// +optional
	FailureThreshold *int32 `json:"failureThreshold,omitempty"`

	// SuccessThreshold is the number of consecutive successes before the probe is considered successful.
	// +optional
	SuccessThreshold *int32 `json:"successThreshold,omitempty"`
}

// BackupSpec configures periodic backup to S3.
type BackupSpec struct {
	// Schedule is a cron expression for backup scheduling.
	Schedule string `json:"schedule"`

	// S3 configures the S3 backup destination. Uses ObjectStorage config if not specified.
	// +optional
	S3 *BackupS3Spec `json:"s3,omitempty"`

	// RetentionDays specifies how many days to retain backups.
	// +kubebuilder:default=30
	// +optional
	RetentionDays *int32 `json:"retentionDays,omitempty"`
}

// BackupS3Spec configures S3 backup destination.
type BackupS3Spec struct {
	// Bucket is the S3 bucket name.
	Bucket string `json:"bucket"`

	// Path is the S3 key prefix for backups.
	// +optional
	Path string `json:"path,omitempty"`

	// Region is the S3 region.
	// +optional
	Region string `json:"region,omitempty"`

	// Endpoint is the S3-compatible endpoint URL.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// CredentialsSecretRef references a Secret containing AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY.
	// +optional
	CredentialsSecretRef *corev1.LocalObjectReference `json:"credentialsSecretRef,omitempty"`
}

// --- Status types ---

// InstancePhase describes the phase of a Instance.
// +kubebuilder:validation:Enum=Pending;Provisioning;Running;Degraded;Failed;Terminating;BackingUp;Restoring;Updating
type InstancePhase string

const (
	PhasePending      InstancePhase = "Pending"
	PhaseProvisioning InstancePhase = "Provisioning"
	PhaseRunning      InstancePhase = "Running"
	PhaseDegraded     InstancePhase = "Degraded"
	PhaseFailed       InstancePhase = "Failed"
	PhaseTerminating  InstancePhase = "Terminating"
	PhaseBackingUp    InstancePhase = "BackingUp"
	PhaseRestoring    InstancePhase = "Restoring"
	PhaseUpdating     InstancePhase = "Updating"
)

// InstanceStatus defines the observed state of Instance.
type InstanceStatus struct {
	// Phase is the current phase of the instance.
	// +optional
	Phase InstancePhase `json:"phase,omitempty"`

	// ObservedGeneration is the most recent generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the instance state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Endpoint is the primary service endpoint URL.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// ManagedResources lists the names of resources managed by this instance.
	// +optional
	ManagedResources ManagedResources `json:"managedResources,omitempty"`

	// Backup tracks the state of the latest backup operation.
	// +optional
	Backup *BackupStatus `json:"backup,omitempty"`

	// Restore tracks the state of the latest restore operation.
	// +optional
	Restore *RestoreStatus `json:"restore,omitempty"`
}

// ManagedResources tracks the names of managed Kubernetes resources.
type ManagedResources struct {
	// +optional
	StatefulSet string `json:"statefulSet,omitempty"`
	// +optional
	Service string `json:"service,omitempty"`
	// +optional
	ConfigMap string `json:"configMap,omitempty"`
	// +optional
	PersistentVolumeClaim string `json:"persistentVolumeClaim,omitempty"`
	// +optional
	Ingress string `json:"ingress,omitempty"`
	// +optional
	ServiceAccount string `json:"serviceAccount,omitempty"`
	// +optional
	NetworkPolicy string `json:"networkPolicy,omitempty"`
	// +optional
	DatabaseStatefulSet string `json:"databaseStatefulSet,omitempty"`
	// +optional
	DatabaseService string `json:"databaseService,omitempty"`
	// +optional
	DatabasePVC string `json:"databasePVC,omitempty"`
}

// BackupStatus tracks the state of a backup operation.
type BackupStatus struct {
	// LastBackupTime is the time of the last successful backup.
	// +optional
	LastBackupTime *metav1.Time `json:"lastBackupTime,omitempty"`

	// LastBackupResult is the result of the last backup.
	// +optional
	LastBackupResult string `json:"lastBackupResult,omitempty"`
}

// RestoreStatus tracks the state of a restore operation.
type RestoreStatus struct {
	// CompletionTime is when the restore completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Result is the result of the restore.
	// +optional
	Result string `json:"result,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=pci
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.status.endpoint`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Instance is the Schema for the instances API.
type Instance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InstanceSpec   `json:"spec,omitempty"`
	Status InstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// InstanceList contains a list of Instance.
type InstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Instance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Instance{}, &InstanceList{})
}
