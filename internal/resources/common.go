package resources

import (
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

	// HealthPath is the HTTP health check path.
	HealthPath = "/api/health"

	// DefaultPaperclipEntrypoint is the default Paperclip container entrypoint.
	// Used when the operator needs to inject a shell wrapper (e.g., heartbeat leader election).
	DefaultPaperclipEntrypoint = `node --import ./server/node_modules/tsx/dist/loader.mjs server/dist/index.js`
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

// UseTCPProbes returns true when probes should use TCP instead of HTTP.
// This is needed in authenticated/single-tenant mode where /api/health returns 403.
func UseTCPProbes(instance *paperclipv1alpha1.Instance) bool {
	probeType := instance.Spec.Probes.Type
	if probeType == "tcp" {
		return true
	}
	if probeType == "http" {
		return false
	}
	// "auto" or empty: use TCP for authenticated/single-tenant modes
	mode := instance.Spec.Deployment.Mode
	return mode == "authenticated" || mode == "single-tenant"
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
