package resources

import (
	"fmt"

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
	// BrandVolumeName is the name of the optional brand-assets volume.
	BrandVolumeName = "paperclip-branding"
	// BrandMountPath is the read-only mount path for the brand-assets ConfigMap.
	// The server serves this directory under /branding (PAPERCLIP_BRAND_DIR).
	BrandMountPath = "/etc/paperclip/branding"
	// EnvBrandDir is the environment variable pointing the server at the brand dir.
	EnvBrandDir = "PAPERCLIP_BRAND_DIR"
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

	// DefaultServerTerminationGracePeriodSeconds is the default pod termination
	// grace period for the Paperclip server pod when the instance does not set
	// spec.availability.terminationGracePeriodSeconds. It is deliberately high (30
	// minutes) so a rollout, node drain, or deploy never SIGKILLs an in-flight
	// agent run: agent runs are frequently multi-minute (a single autonomous build
	// runs as one long sandbox exec), and the hosted deployment already allows a
	// 30-minute run window (PAPERCLIP_HEARTBEAT_REAP_STALE_MS=1800000). Matching
	// that window here means the grace period is never the limiting factor for a
	// run finishing across a pod replacement. The grace period is a ceiling, not a
	// fixed delay -- the kubelet terminates the pod as soon as the server process
	// exits, so this high value only extends how long a pod that still has work may
	// keep running before SIGKILL; it does not slow down teardown of an idle pod
	// whose server exits promptly on SIGTERM.
	DefaultServerTerminationGracePeriodSeconds int64 = 1800

	// DefaultServerDrainTimeoutSeconds is the default preStop sleep applied to the
	// server container when spec.availability.serverDrain is enabled (the default).
	// The preStop hook holds the container briefly BEFORE the kubelet delivers
	// SIGTERM, which gives the Endpoints/EndpointSlice controllers time to remove
	// the terminating pod from the Service so no NEW request or scheduler work is
	// routed to a pod that is about to shut down (the well-known kube-proxy/CNI
	// endpoint-deregistration race). It is intentionally short so it consumes only
	// a small slice of the termination grace period, leaving the rest for the
	// server's own SIGTERM handling.
	DefaultServerDrainTimeoutSeconds int64 = 15
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

// SELinuxRelabelEnabled resolves spec.security.seLinuxRelabel (*bool, nil =
// default true). When true (or unset) and persistence is enabled the operator
// adds the privileged selinux-relabel init container. An explicit false lets
// operators opt out on clusters where chcon fails permanently (NFS storage or
// non-SELinux-enforcing nodes).
func SELinuxRelabelEnabled(instance *paperclipv1alpha1.Instance) bool {
	if instance.Spec.Security.SELinuxRelabel == nil {
		return true
	}
	return *instance.Spec.Security.SELinuxRelabel
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

// ServerTerminationGracePeriodSeconds resolves the pod termination grace period
// for the Paperclip server pod: the explicit
// spec.availability.terminationGracePeriodSeconds when set, otherwise the high
// default (DefaultServerTerminationGracePeriodSeconds) so a rollout does not
// SIGKILL in-flight agent runs.
func ServerTerminationGracePeriodSeconds(instance *paperclipv1alpha1.Instance) int64 {
	if g := instance.Spec.Availability.TerminationGracePeriodSeconds; g != nil {
		return *g
	}
	return DefaultServerTerminationGracePeriodSeconds
}

// ServerDrainEnabled reports whether the server container should get the preStop
// endpoint-deregistration hook. Defaults to true (nil spec, or an explicit
// serverDrain block with Enabled unset).
func ServerDrainEnabled(instance *paperclipv1alpha1.Instance) bool {
	d := instance.Spec.Availability.ServerDrain
	if d == nil || d.Enabled == nil {
		return true
	}
	return *d.Enabled
}

// ServerDrainTimeoutSeconds resolves the preStop sleep duration, defaulting to
// DefaultServerDrainTimeoutSeconds. It is clamped so the preStop can never
// consume the ENTIRE termination grace period: the kubelet counts the preStop
// hook against terminationGracePeriodSeconds and then still needs time to deliver
// SIGTERM and let the server exit, so a preStop as long as (or longer than) the
// grace would leave zero time for graceful shutdown and force an immediate
// SIGKILL. We cap it at grace-1 (with a floor of 0) to always leave at least one
// second for the server's SIGTERM path.
func ServerDrainTimeoutSeconds(instance *paperclipv1alpha1.Instance) int64 {
	timeout := DefaultServerDrainTimeoutSeconds
	if d := instance.Spec.Availability.ServerDrain; d != nil && d.TimeoutSeconds != nil {
		timeout = *d.TimeoutSeconds
	}
	if timeout < 0 {
		timeout = 0
	}
	grace := ServerTerminationGracePeriodSeconds(instance)
	if max := grace - 1; timeout > max {
		if max < 0 {
			max = 0
		}
		timeout = max
	}
	return timeout
}

// buildServerLifecycle returns the server container Lifecycle, or nil. When the
// serverDrain hook is enabled (default) it installs a preStop that sleeps for the
// resolved drain timeout BEFORE the kubelet delivers SIGTERM. This deregisters
// the terminating pod from the Service (closing the endpoint-removal race) so new
// traffic stops before shutdown; the server's own SIGTERM handling then runs
// within the remaining, deliberately-high, termination grace period.
//
// This hook does NOT itself make in-flight agent runs finish -- that requires the
// server image to soft-drain on SIGTERM. Its job is the traffic-cutover half:
// keep the pod serving while it is still reachable, then hand off to SIGTERM with
// the pod already out of rotation.
func buildServerLifecycle(instance *paperclipv1alpha1.Instance) *corev1.Lifecycle {
	if !ServerDrainEnabled(instance) {
		return nil
	}
	timeout := ServerDrainTimeoutSeconds(instance)
	if timeout <= 0 {
		return nil
	}
	return &corev1.Lifecycle{
		PreStop: &corev1.LifecycleHandler{
			Exec: &corev1.ExecAction{
				Command: []string{"/bin/sh", "-c", fmt.Sprintf("sleep %d", timeout)},
			},
		},
	}
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

// paperclipPodSecurityContext returns the pod-level security context for pods
// running the Paperclip image. If the user has provided a custom pod security
// context via spec.security.podSecurityContext it is used verbatim; otherwise
// the restricted-PSS-compliant default is returned.
//
// Every pod the operator renders from the Paperclip image must go through this
// helper. Hard-coding the UID/GID 1000 default (as the bootstrap Job used to do)
// makes the pod unschedulable on OpenShift, where the namespace's dynamically
// allocated UID/GID range is the only one the restricted-v2 SCC admits, so the
// Job fails admission with ".spec.securityContext.fsGroup: Invalid value: 1000"
// and the Instance never leaves Provisioning (issue #111).
func paperclipPodSecurityContext(instance *paperclipv1alpha1.Instance) *corev1.PodSecurityContext {
	if instance.Spec.Security.PodSecurityContext != nil {
		return instance.Spec.Security.PodSecurityContext
	}
	return &corev1.PodSecurityContext{
		RunAsNonRoot: Ptr(true),
		RunAsUser:    Ptr(int64(1000)),
		RunAsGroup:   Ptr(int64(1000)),
		FSGroup:      Ptr(int64(1000)),
	}
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
