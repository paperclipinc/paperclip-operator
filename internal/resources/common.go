package resources

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

const (
	// LabelApp is the standard app label key.
	LabelApp = "app.kubernetes.io/name"
	// LabelInstance is the instance label key.
	LabelInstance = "app.kubernetes.io/instance"
	// LabelManagedBy is the managed-by label key.
	LabelManagedBy = "app.kubernetes.io/managed-by"
	// LabelComponent is the component label key.
	LabelComponent = "app.kubernetes.io/component"

	// AppName is the application name used in labels.
	AppName = "paperclip"
	// ManagedBy is the manager name used in labels.
	ManagedBy = "paperclip-operator"

	// ContainerName is the name of the main Paperclip container.
	ContainerName = "paperclip"
	// DatabaseContainerName is the name of the PostgreSQL container.
	DatabaseContainerName = "postgres"

	// DefaultPort is the default Paperclip server port.
	DefaultPort int32 = 3100
	// PostgreSQLPort is the default PostgreSQL port.
	PostgreSQLPort int32 = 5432

	// DataVolumeName is the name of the Paperclip data volume.
	DataVolumeName = "paperclip-data"
	// DataMountPath is the mount path for the Paperclip data volume.
	DataMountPath = "/paperclip"
	// DatabaseVolumeName is the name of the PostgreSQL data volume.
	DatabaseVolumeName = "pgdata"
	// DatabaseMountPath is the mount path for the PostgreSQL data volume.
	DatabaseMountPath = "/var/lib/postgresql/data"

	// ModeExternal is the value for external resource modes (database, redis).
	ModeExternal = "external"

	// HealthPath is the HTTP health check path.
	HealthPath = "/api/health"

	// DefaultPaperclipEntrypoint is the default Paperclip container entrypoint.
	// Used when the operator needs to inject a shell wrapper (e.g., heartbeat leader election).
	DefaultPaperclipEntrypoint = `node --import ./server/node_modules/tsx/dist/loader.mjs server/dist/index.js`

	// EnvOAuthCredentials is the environment variable for OAuth provider credentials JSON.
	EnvOAuthCredentials = "PAPERCLIP_OAUTH_CREDENTIALS" // #nosec G101 -- env var name, not a credential //nolint:gosec
	// EnvOAuthProviders is the environment variable for custom OAuth provider definitions.
	EnvOAuthProviders = "PAPERCLIP_OAUTH_PROVIDERS"
)

// Ptr returns a pointer to the given value.
func Ptr[T any](v T) *T {
	return &v
}

// EffectiveReplicas returns the configured replica count, defaulting to 1.
func EffectiveReplicas(instance *paperclipv1alpha1.Instance) int32 {
	if instance.Spec.Availability.Replicas != nil {
		return *instance.Spec.Availability.Replicas
	}
	return 1
}

// WorkloadReplicas returns the replica count for the server workload
// (StatefulSet or Deployment). When the instance is suspended, replicas is
// forced to 0 (scale-to-zero). Otherwise it returns the effective replica
// count. When HPA is enabled the controller preserves the current replica
// count on update so it does not fight the autoscaler.
func WorkloadReplicas(instance *paperclipv1alpha1.Instance) int32 {
	if instance.Spec.Suspended {
		return 0
	}
	return EffectiveReplicas(instance)
}

// UseDeploymentWorkload returns true when the server should run as a
// Deployment: explicit spec.workload=Deployment, or auto with no
// persistence and a non-embedded database.
func UseDeploymentWorkload(instance *paperclipv1alpha1.Instance) bool {
	switch instance.Spec.Workload {
	case "Deployment":
		return true
	case "auto":
		return !PersistenceEnabled(instance) && instance.Spec.Database.Mode != "embedded"
	default:
		return false
	}
}

// PersistenceEnabled reports whether the data PVC is enabled (defaults to
// true when unset).
// NetworkPolicyEnabled resolves the *bool (nil = default true).
func NetworkPolicyEnabled(instance *paperclipv1alpha1.Instance) bool {
	if instance.Spec.Security.NetworkPolicy.Enabled == nil {
		return true
	}
	return *instance.Spec.Security.NetworkPolicy.Enabled
}

func PersistenceEnabled(instance *paperclipv1alpha1.Instance) bool {
	if instance.Spec.Storage.Persistence.Enabled == nil {
		return true
	}
	return *instance.Spec.Storage.Persistence.Enabled
}

// SchedulerGatingMode resolves spec.heartbeat.schedulerGating to the mode the
// operator actually applies: "ordinal" or "lease". "auto" currently resolves
// to "ordinal" (it will flip to "lease" once the minimum supported app version
// includes lease-based scheduler leadership), and an empty value defaults to
// "ordinal".
func SchedulerGatingMode(instance *paperclipv1alpha1.Instance) string {
	if instance.Spec.Heartbeat.SchedulerGating == "lease" {
		return "lease"
	}
	return "ordinal"
}

// EffectiveWorkloadIsDeployment reports whether the server workload the
// controller actually reconciles is a Deployment. It applies the PVC-safety
// override on top of UseDeploymentWorkload: an explicit spec.workload=
// Deployment with persistence enabled falls back to a StatefulSet (the
// ReadWriteOnce data PVC cannot be shared by surging Deployment pods), and the
// HPA scaleTargetRef must follow that fallback.
func EffectiveWorkloadIsDeployment(instance *paperclipv1alpha1.Instance) bool {
	return UseDeploymentWorkload(instance) && !PersistenceEnabled(instance)
}

// ServerPort returns the Paperclip server container port: the configured
// service port (spec.networking.service.port) or the default. It is the port
// the container listens on, used by probes and by the operator's own
// /api/health polling for scheduler leader discovery.
func ServerPort(instance *paperclipv1alpha1.Instance) int32 {
	if instance.Spec.Networking.Service.Port > 0 {
		return instance.Spec.Networking.Service.Port
	}
	return DefaultPort
}

// UseTCPProbes returns true when probes should use TCP instead of HTTP.
// This is needed in authenticated mode where /api/health returns 403.
func UseTCPProbes(instance *paperclipv1alpha1.Instance) bool {
	probeType := instance.Spec.Probes.Type
	if probeType == "tcp" {
		return true
	}
	if probeType == "http" {
		return false
	}
	// "auto" or empty: authenticated mode returns 403 from /api/health, so use TCP.
	return instance.Spec.Deployment.Mode == "authenticated"
}

// Labels returns the standard labels for a Instance resource.
func Labels(instance *paperclipv1alpha1.Instance) map[string]string {
	return map[string]string{
		LabelApp:       AppName,
		LabelInstance:  instance.Name,
		LabelManagedBy: ManagedBy,
	}
}

// LabelsWithComponent returns standard labels plus a component label.
func LabelsWithComponent(instance *paperclipv1alpha1.Instance, component string) map[string]string {
	labels := Labels(instance)
	labels[LabelComponent] = component
	return labels
}

// SelectorLabels returns the minimal labels used for pod selectors.
// Includes component=server to distinguish from database pods.
func SelectorLabels(instance *paperclipv1alpha1.Instance) map[string]string {
	return map[string]string{
		LabelApp:       AppName,
		LabelInstance:  instance.Name,
		LabelComponent: "server",
	}
}

// DatabaseSelectorLabels returns the labels used for the database pod selector.
func DatabaseSelectorLabels(instance *paperclipv1alpha1.Instance) map[string]string {
	return map[string]string{
		LabelApp:       AppName,
		LabelInstance:  instance.Name,
		LabelComponent: "database",
	}
}

// ObjectMeta returns a standard ObjectMeta for a managed resource.
func ObjectMeta(instance *paperclipv1alpha1.Instance, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      name,
		Namespace: instance.Namespace,
		Labels:    Labels(instance),
	}
}

// --- Naming conventions ---

// StatefulSetName returns the StatefulSet name for a Instance.
func StatefulSetName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name
}

// DeploymentName returns the Deployment name for a Instance. It must equal
// StatefulSetName so the server workload keeps its name (and the Service its
// label-based selection) when switching workload kinds.
func DeploymentName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name
}

// ServiceName returns the Service name for a Instance.
func ServiceName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name
}

// ConfigMapName returns the ConfigMap name for a Instance.
func ConfigMapName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name + "-config"
}

// PVCName returns the PVC name for a Instance.
func PVCName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name + "-data"
}

// IngressName returns the Ingress name for a Instance.
func IngressName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name
}

// HTTPRouteName returns the HTTPRoute name for a Instance.
func HTTPRouteName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name
}

// ServiceAccountName returns the ServiceAccount name for a Instance.
func ServiceAccountName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name
}

// NetworkPolicyName returns the NetworkPolicy name for a Instance.
func NetworkPolicyName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name
}

// DatabaseStatefulSetName returns the database StatefulSet name.
func DatabaseStatefulSetName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name + "-db"
}

// DatabaseServiceName returns the database Service name.
func DatabaseServiceName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name + "-db"
}

// DatabasePVCName returns the database PVC name.
func DatabasePVCName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name + "-db-data"
}

// HPAName returns the HPA name for a Instance.
func HPAName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name
}

// PDBName returns the PDB name for a Instance.
func PDBName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name
}

// DatabaseSecretName returns the auto-generated database credentials secret name.
func DatabaseSecretName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name + "-db-credentials"
}

// SecretsMasterKeySecretName returns the auto-generated secrets master key secret name.
func SecretsMasterKeySecretName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name + "-secrets-master-key"
}

// paperclipContainerSecurityContext returns the security context for containers
// running the Paperclip image. If the user has provided a custom security context
// via the CRD, it is used; otherwise the restricted-PSS-compliant default is returned.
func paperclipContainerSecurityContext(instance *paperclipv1alpha1.Instance) *corev1.SecurityContext {
	if instance.Spec.Security.ContainerSecurityContext != nil {
		return instance.Spec.Security.ContainerSecurityContext
	}
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: Ptr(false),
		RunAsNonRoot:             Ptr(true),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}
