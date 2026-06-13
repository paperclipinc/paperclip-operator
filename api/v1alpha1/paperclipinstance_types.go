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
	networkingv1 "k8s.io/api/networking/v1"
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

	// Connections configures third-party OAuth provider credentials for
	// the Paperclip connections system (GitHub, GitLab, Slack, etc.).
	// +optional
	Connections *ConnectionsSpec `json:"connections,omitempty"`

	// Plugins lists plugins to install.
	// +optional
	Plugins []PluginRef `json:"plugins,omitempty"`

	// SelfConfigure enables agents to modify their own Instance via
	// PaperclipSelfConfig resources, gated by an action allowlist.
	// +optional
	SelfConfigure SelfConfigureSpec `json:"selfConfigure,omitempty"`

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

	// ShareProcessNamespace enables PID namespace sharing between all containers
	// in the pod. When true, the infrastructure (pause) container becomes PID 1
	// and reaps zombie processes, which prevents accumulation of defunct helper
	// processes (git, plugins, shells) under a Node.js server that does not call
	// waitpid(). Defaults to true.
	//
	// Security note: enabling this lets every container in the pod see and signal
	// every other container's processes. A compromised sidecar could send signals
	// to the server and vice versa. Set to false to keep per-container PID
	// isolation; you are then responsible for reaping zombies (e.g. by baking
	// tini or dumb-init into the image).
	// +kubebuilder:default=true
	// +optional
	ShareProcessNamespace *bool `json:"shareProcessNamespace,omitempty"`

	// Suspended scales the workload to zero replicas when true. Non-runtime
	// resources (Service, ConfigMap, RBAC, NetworkPolicy, PVC) remain fully
	// managed. Set to false to resume normal operation.
	// +kubebuilder:default=false
	// +optional
	Suspended bool `json:"suspended,omitempty"`

	// Networking configures service, ingress, and WebSocket settings.
	// +optional
	Networking NetworkingSpec `json:"networking,omitempty"`

	// Observability configures metrics, logging, and monitoring.
	// +optional
	Observability ObservabilitySpec `json:"observability,omitempty"`

	// Availability configures scaling, PDB, and pod scheduling.
	// +optional
	Availability AvailabilitySpec `json:"availability,omitempty"`

	// Workload selects the server workload kind.
	//  - "StatefulSet" (default): per-instance PVC, stable identity. Required for
	//    embedded database mode and for persistence-backed instances.
	//  - "Deployment": stateless pods (ephemeral scratch); intended for
	//    database.mode external/managed with objectStorage when replicas > 1.
	//    Enables surge rollouts and avoids AZ-pinned per-ordinal PVCs under
	//    autoscaling. Requires storage.persistence.enabled=false.
	//  - "auto": Deployment when persistence is disabled AND database mode is
	//    not embedded; StatefulSet otherwise.
	// +kubebuilder:validation:Enum=StatefulSet;Deployment;auto
	// +kubebuilder:default=StatefulSet
	// +optional
	Workload string `json:"workload,omitempty"`

	// Probes configures liveness, readiness, and startup probes.
	// +optional
	Probes ProbesSpec `json:"probes,omitempty"`

	// Backup configures periodic backup to S3-compatible storage.
	// +optional
	Backup *BackupSpec `json:"backup,omitempty"`

	// RestoreFrom specifies a remote backup path to restore from on first boot.
	// +optional
	RestoreFrom string `json:"restoreFrom,omitempty"`

	// Tailscale configures an ephemeral Tailscale sidecar that Serves the
	// Paperclip app over the tailnet.
	// +optional
	Tailscale TailscaleSpec `json:"tailscale,omitempty"`

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
// +kubebuilder:validation:XValidation:rule="size(self.tag) > 0 || size(self.digest) > 0",message="spec.image: one of tag or digest must be set (pinning to :latest is not supported, pick a specific upstream release tag)"
type ImageSpec struct {
	// Repository is the container image repository.
	// +kubebuilder:default="ghcr.io/paperclipai/paperclip"
	// +optional
	Repository string `json:"repository,omitempty"`

	// Tag is the container image tag. Either tag or digest must be set; there is
	// no default, because pinning to a mutable tag like :latest can silently pull
	// a broken upstream build.
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

	// AutoUpdate enables automatic image updates by polling the registry for new digests.
	// +optional
	AutoUpdate *AutoUpdateSpec `json:"autoUpdate,omitempty"`
}

// AutoUpdateSpec configures automatic image update polling.
type AutoUpdateSpec struct {
	// Enabled controls whether auto-update polling is active.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Interval is the polling interval (e.g. "5m", "1h"). Minimum is 1m.
	// +kubebuilder:default="5m"
	// +kubebuilder:validation:Pattern=`^\d+(s|m|h)$`
	// +optional
	Interval string `json:"interval,omitempty"`
}

// DeploymentSpec controls deployment mode and exposure.
// +kubebuilder:validation:XValidation:rule="self.mode != 'local_trusted' || self.exposure == 'private'",message="spec.deployment.exposure must be 'private' when mode is 'local_trusted'"
type DeploymentSpec struct {
	// Mode sets the deployment mode: "local_trusted" (loopback / no auth) or
	// "authenticated" (login required). Matches Paperclip's DEPLOYMENT_MODES.
	// To run authenticated without public self-service sign-up, set
	// spec.auth.disableSignUp instead of a separate mode.
	// +kubebuilder:default="authenticated"
	// +kubebuilder:validation:Enum=local_trusted;authenticated
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

	// PlatformAdmin, when set, bootstraps the instance with a platform-managed
	// instance-admin so it is never left in the single-tenant "claim this
	// instance" state. The operator runs an idempotent seed init container
	// (`pnpm paperclipai auth seed-instance-admin`) after onboarding/migrations.
	// +optional
	PlatformAdmin *PlatformAdminSpec `json:"platformAdmin,omitempty"`
}

// PlatformAdminSpec describes the platform-managed instance-admin seeded into a
// freshly provisioned Paperclip instance for the shared cloud. It maps to the
// PAPERCLIP_SEED_ADMIN_* env vars consumed by the seed-instance-admin CLI.
type PlatformAdminSpec struct {
	// Email is the platform instance-admin's email address. Required.
	// Wired to PAPERCLIP_SEED_ADMIN_EMAIL.
	Email string `json:"email"`

	// Name is the optional display name for the platform instance-admin.
	// Wired to PAPERCLIP_SEED_ADMIN_NAME.
	// +optional
	Name string `json:"name,omitempty"`

	// UserID is the optional stable user ID to assign to the platform
	// instance-admin (e.g. the Paperclip ID). Wired to PAPERCLIP_SEED_ADMIN_USER_ID.
	// +optional
	UserID string `json:"userID,omitempty"`
}

// DatabaseSpec configures PostgreSQL.
// For high-availability production deployments, use mode "external" with a managed
// PostgreSQL service (e.g., Amazon RDS, Cloud SQL). The "managed" mode provides a
// single-instance PostgreSQL suitable for development and small deployments.
type DatabaseSpec struct {
	// Mode selects the database mode: "embedded" (PGlite), "external" (connection string), or "managed" (operator-managed StatefulSet).
	// +kubebuilder:default="managed"
	// +kubebuilder:validation:Enum=embedded;external;managed
	// +optional
	Mode string `json:"mode,omitempty"`

	// ExternalURL is the PostgreSQL connection string for external mode.
	// WARNING: This value is stored in plaintext in the CRD spec (etcd). If the URL contains
	// credentials, use ExternalURLSecretRef instead to reference a Secret.
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

	// AdminUser configures the initial admin user that is created automatically
	// when the instance is first deployed. If not set, the app's first-admin
	// (board-claim) flow lets the first human to authenticate claim ownership
	// in authenticated mode.
	// +optional
	AdminUser *AdminUserSpec `json:"adminUser,omitempty"`

	// DisableSignUp disables public self-service sign-up (the former
	// "single-tenant" behavior). Maps to PAPERCLIP_AUTH_DISABLE_SIGN_UP.
	// +optional
	DisableSignUp bool `json:"disableSignUp,omitempty"`

	// Email configures email sending for verification and password reset.
	// +optional
	Email *AuthEmailSpec `json:"email,omitempty"`

	// Google configures Google OAuth sign-in.
	// The referenced Secret must contain GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET keys.
	// +optional
	Google *OAuthProviderSpec `json:"google,omitempty"`

	// Apple configures Apple OAuth sign-in.
	// The referenced Secret must contain APPLE_CLIENT_ID and APPLE_CLIENT_SECRET keys.
	// +optional
	Apple *OAuthProviderSpec `json:"apple,omitempty"`
}

// AdminUserSpec configures the initial admin user for automatic bootstrap.
type AdminUserSpec struct {
	// Email is the admin user's email address (used as login).
	Email string `json:"email"`

	// Name is the admin user's display name.
	// +kubebuilder:default="Admin"
	// +optional
	Name string `json:"name,omitempty"`

	// PasswordSecretRef references a Secret containing the admin password.
	PasswordSecretRef corev1.SecretKeySelector `json:"passwordSecretRef"`
}

// AuthEmailSpec configures email delivery for auth flows.
type AuthEmailSpec struct {
	// ResendAPIKeySecretRef references a Secret key containing the Resend API key.
	// +optional
	ResendAPIKeySecretRef *corev1.SecretKeySelector `json:"resendAPIKeySecretRef,omitempty"`

	// From is the sender address for outbound emails (e.g. "Paperclip <noreply@example.com>").
	// +optional
	From string `json:"from,omitempty"`

	// VerificationRequired requires email verification before a session can be created.
	// +optional
	VerificationRequired bool `json:"verificationRequired,omitempty"`
}

// OAuthProviderSpec references credentials for a social OAuth provider.
type OAuthProviderSpec struct {
	// CredentialsSecretRef references a Secret containing provider-specific OAuth
	// client credentials (e.g. GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET).
	CredentialsSecretRef corev1.LocalObjectReference `json:"credentialsSecretRef"`
}

// SecretsSpec configures Paperclip's built-in secrets management.
// +kubebuilder:validation:XValidation:rule="self.provider != 'aws_secrets_manager' || has(self.aws)",message="spec.secrets.aws is required when provider is aws_secrets_manager"
type SecretsSpec struct {
	// Provider selects the secrets backend. "local_encrypted" (default) encrypts
	// secrets with the master key; "aws_secrets_manager" stores them in AWS
	// Secrets Manager. Maps to PAPERCLIP_SECRETS_PROVIDER.
	// +kubebuilder:default="local_encrypted"
	// +kubebuilder:validation:Enum=local_encrypted;aws_secrets_manager
	// +optional
	Provider string `json:"provider,omitempty"`

	// AWS configures the AWS Secrets Manager provider. AWS credentials are sourced
	// from the standard AWS SDK credential chain - use IRSA via
	// spec.security.rbac.serviceAccountAnnotations rather than injecting keys.
	// +optional
	AWS *AWSSecretsManagerSpec `json:"aws,omitempty"`

	// MasterKeySecretRef references a Secret containing the master encryption key.
	// +optional
	MasterKeySecretRef *corev1.SecretKeySelector `json:"masterKeySecretRef,omitempty"`

	// StrictMode requires all sensitive values to use encrypted references.
	// +optional
	StrictMode bool `json:"strictMode,omitempty"`
}

// AWSSecretsManagerSpec configures the AWS Secrets Manager secrets provider.
type AWSSecretsManagerSpec struct {
	// Region is the AWS region of the secrets and KMS key.
	Region string `json:"region"`

	// KMSKeyID is the KMS key (id or ARN) used to encrypt secrets.
	KMSKeyID string `json:"kmsKeyID"`

	// DeploymentID namespaces secrets for this deployment.
	DeploymentID string `json:"deploymentID"`

	// Prefix is the secret name prefix.
	// +kubebuilder:default="paperclip"
	// +optional
	Prefix string `json:"prefix,omitempty"`

	// Environment is an optional environment label applied to stored secrets.
	// +optional
	Environment string `json:"environment,omitempty"`

	// Endpoint overrides the AWS Secrets Manager endpoint (for VPC endpoints or testing).
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// DeleteRecoveryDays is the AWS recovery window (in days) for deleted secrets.
	// +kubebuilder:default=30
	// +optional
	DeleteRecoveryDays *int32 `json:"deleteRecoveryDays,omitempty"`
}

// StorageSpec configures persistent storage.
type StorageSpec struct {
	// Persistence configures the PVC for the Paperclip data directory (/paperclip).
	// +optional
	Persistence PersistenceSpec `json:"persistence,omitempty"`
}

// PersistenceSpec configures PVC settings.
type PersistenceSpec struct {
	// A pointer is required for round-tripping: with a plain bool plus
	// omitempty, enabled=false is dropped on marshal and the API server
	// re-defaults it to true on every controller update of the CR.

	// Enabled controls whether a PVC is created. Defaults to true.
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

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

	// ForcePathStyle forces path-style S3 addressing (bucket in the URL path
	// rather than the hostname). Defaults to true when provider is "minio"
	// (virtual-hosted addressing needs wildcard DNS that in-cluster MinIO
	// deployments don't have), false otherwise. Maps to
	// PAPERCLIP_STORAGE_S3_FORCE_PATH_STYLE.
	// +optional
	ForcePathStyle *bool `json:"forcePathStyle,omitempty"`
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

	// SchedulerGating selects how the heartbeat scheduler is pinned to a single
	// replica when replicas > 1.
	//  - "ordinal" (default): a shell wrapper enables the scheduler only on the
	//    StatefulSet's ordinal-0 pod. Works with every app version, but has no
	//    failover and requires the StatefulSet workload.
	//  - "lease": no env manipulation - every replica participates in the app's
	//    lease-based leader election with automatic failover. Requires an app
	//    version with scheduler leases.
	//  - "auto": currently behaves like "ordinal"; will default to "lease" once
	//    the minimum supported app version includes lease leadership.
	// +kubebuilder:validation:Enum=ordinal;lease;auto
	// +kubebuilder:default=ordinal
	// +optional
	SchedulerGating string `json:"schedulerGating,omitempty"`
}

// AdaptersSpec configures agent runtime adapters.
type AdaptersSpec struct {
	// APIKeys references Secrets containing LLM provider API keys.
	// The Secret should contain keys like ANTHROPIC_API_KEY, OPENAI_API_KEY, etc.
	// +optional
	APIKeysSecretRef *corev1.LocalObjectReference `json:"apiKeysSecretRef,omitempty"`

	// CloudSandbox configures cloud-based agent execution in isolated Kubernetes pods.
	// +optional
	CloudSandbox *CloudSandboxSpec `json:"cloudSandbox,omitempty"`

	// E2B supplies the API key for the E2B sandbox provider. The provider must be
	// enabled as a plugin and selected per-Environment in the Paperclip UI; the
	// operator only wires E2B_API_KEY. (Modal and Cloudflare sandbox credentials
	// are configured at runtime in the UI - see docs/deploy/runtime-configured-features.md.)
	// +optional
	E2B *E2BSpec `json:"e2b,omitempty"`

	// Execution configures the @paperclipai/plugin-kubernetes in-cluster sandbox
	// provider, which runs agents as per-tenant batch/v1 Jobs (or agent-sandbox
	// CRs) inside this cluster. This is the current k8s execution path and
	// supersedes the older in-tree CloudSandbox wiring (the two are independent;
	// do not enable both for the same execution surface).
	//
	// When Mode is "kubernetes" the operator additionally provisions a ClusterRole
	// + ClusterRoleBinding scoped to the resources the plugin creates per tenant
	// (namespaces, ServiceAccounts, Roles/RoleBindings, ResourceQuotas,
	// LimitRanges, NetworkPolicies, CiliumNetworkPolicies, Secrets, Jobs, Sandbox
	// CRs) and mounts the app ServiceAccount token so the app can reach the
	// in-cluster Kubernetes API.
	// +optional
	Execution *ExecutionSpec `json:"execution,omitempty"`

	// Registry declaratively defines which agent harnesses ("adapters") this
	// instance offers and how each is wired. Marshaled to the PAPERCLIP_ADAPTERS
	// env var consumed by the server's adapter-registry bootstrap. When empty,
	// the server uses its built-in defaults (no PAPERCLIP_ADAPTERS emitted).
	// +optional
	Registry []AdapterRegistryEntry `json:"registry,omitempty"`
}

// ExecutionSpec configures the agent execution policy enforced by the Paperclip
// server at boot. It maps to PAPERCLIP_EXECUTION_MODE and the PAPERCLIP_K8S_*
// env vars consumed by the fork's execution-policy bootstrap.
type ExecutionSpec struct {
	// Mode selects the forced execution policy. "kubernetes" forces all agent
	// runs onto the in-cluster Kubernetes sandbox provider and refuses local
	// execution; "any" leaves execution unrestricted (the operator emits no
	// PAPERCLIP_EXECUTION_MODE and provisions no execution RBAC).
	// Maps to PAPERCLIP_EXECUTION_MODE.
	// +kubebuilder:default="any"
	// +kubebuilder:validation:Enum=kubernetes;any
	// +optional
	Mode string `json:"mode,omitempty"`

	// Kubernetes configures the in-cluster Kubernetes sandbox backend. Honored
	// only when Mode is "kubernetes".
	// +optional
	Kubernetes *K8sExecutionSpec `json:"kubernetes,omitempty"`
}

// K8sExecutionSpec configures the @paperclipai/plugin-kubernetes backend. Each
// field maps to a PAPERCLIP_K8S_* env var read by the server at boot.
type K8sExecutionSpec struct {
	// Backend selects the per-run workload primitive. "job" runs each agent as a
	// fire-and-forget batch/v1 Job (log-scraped output); "sandbox-cr" creates a
	// long-lived agent-sandbox CR (agents.x-k8s.io) that the server execs into.
	// Maps to PAPERCLIP_K8S_BACKEND.
	// +kubebuilder:default="job"
	// +kubebuilder:validation:Enum=job;sandbox-cr
	// +optional
	Backend string `json:"backend,omitempty"`

	// RuntimeClassName is the RuntimeClass applied to agent pods (e.g. "gvisor")
	// for an extra kernel-isolation boundary. Maps to
	// PAPERCLIP_K8S_RUNTIME_CLASS_NAME. When set, the execution ClusterRole also
	// grants get/use on the named RuntimeClass.
	// +optional
	RuntimeClassName string `json:"runtimeClassName,omitempty"`

	// EgressMode selects how per-tenant egress is enforced. "standard" uses plain
	// NetworkPolicy (CIDR-only, cannot match FQDNs); "cilium" uses a
	// CiliumNetworkPolicy for exact FQDN allow-listing (requires the Cilium CNI).
	// Maps to PAPERCLIP_K8S_EGRESS_MODE.
	// +kubebuilder:validation:Enum=standard;cilium
	// +optional
	EgressMode string `json:"egressMode,omitempty"`

	// EgressAllowFQDNs is the list of fully-qualified domain names tenant agent
	// pods may reach (e.g. the LLM gateway and required APIs). Enforced exactly
	// only under EgressMode "cilium". Maps to PAPERCLIP_K8S_EGRESS_ALLOW_FQDNS
	// (comma-separated).
	// +optional
	// +listType=set
	EgressAllowFQDNs []string `json:"egressAllowFQDNs,omitempty"`

	// EgressAllowCIDRs is the list of CIDR blocks tenant agent pods may reach, in
	// addition to (or as the standard-mode substitute for) the FQDN allow-list.
	// Maps to PAPERCLIP_K8S_EGRESS_ALLOW_CIDRS (comma-separated).
	// +optional
	// +listType=set
	EgressAllowCIDRs []string `json:"egressAllowCIDRs,omitempty"`

	// NamespacePrefix is prepended to each derived per-tenant namespace name,
	// letting multiple instances share a cluster without namespace collisions.
	// Maps to PAPERCLIP_K8S_NAMESPACE_PREFIX.
	// +optional
	NamespacePrefix string `json:"namespacePrefix,omitempty"`
}

// AdapterRegistryEntry is one declarative agent-harness entry. Mirrors the
// server's shared AdapterRegistryEntry shape (packages/shared) and the plugin's
// local copy. Runtime fields are honored only for sandboxed (Kubernetes)
// execution. Keep the json tags in sync with the server zod schema.
type AdapterRegistryEntry struct {
	// AdapterType is the harness identifier, e.g. "opencode_local".
	// +kubebuilder:validation:MinLength=1
	AdapterType string `json:"adapterType"`

	// Enabled controls availability in the agent-creation picker. Defaults true.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// RuntimeImage is the container image the agent Job/Sandbox runs (k8s only).
	// +optional
	RuntimeImage string `json:"runtimeImage,omitempty"`

	// EnvKeys are process-env keys forwarded into the agent Job (k8s only),
	// e.g. ANTHROPIC_API_KEY.
	// +optional
	EnvKeys []string `json:"envKeys,omitempty"`

	// AllowFQDNs is the egress FQDN allow-list for the agent pod (k8s only).
	// +optional
	AllowFQDNs []string `json:"allowFqdns,omitempty"`

	// ProbeCommand is the adapter liveness/probe command (k8s only).
	// +optional
	ProbeCommand []string `json:"probeCommand,omitempty"`

	// DefaultEnv are NON-SECRET env defaults injected as the base layer for the
	// agent Job (process-env secrets override them), e.g. ANTHROPIC_BASE_URL set
	// to the in-cluster inference gateway. Never put secrets here.
	// +optional
	DefaultEnv map[string]string `json:"defaultEnv,omitempty"`
}

// E2BSpec configures the E2B sandbox provider API key.
type E2BSpec struct {
	// APIKeySecretRef references a Secret key holding the E2B API key.
	APIKeySecretRef corev1.SecretKeySelector `json:"apiKeySecretRef"`
}

// CloudSandboxSpec configures cloud sandbox execution for agent runtimes.
type CloudSandboxSpec struct {
	// Enabled controls whether cloud sandbox execution is available.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// DefaultImage is the default agent runtime container image.
	// +kubebuilder:default="ghcr.io/paperclipinc/agent-multi:latest"
	// +optional
	DefaultImage string `json:"defaultImage,omitempty"`

	// Namespace is the namespace for sandbox pods. Defaults to the instance namespace.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// IdleTimeoutMin is how long (in minutes) a sandbox pod can be idle before being reaped.
	// +kubebuilder:default=30
	// +optional
	IdleTimeoutMin int32 `json:"idleTimeoutMin,omitempty"`

	// Resources specifies default compute resources for sandbox pods.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Persistence configures PVC-backed persistent workspaces for sandbox pods.
	// +optional
	Persistence *CloudSandboxPersistenceSpec `json:"persistence,omitempty"`

	// MultiNamespace enables per-company namespace isolation for sandbox pods.
	// When enabled, each company's sandbox pods run in a dedicated namespace.
	// +optional
	MultiNamespace bool `json:"multiNamespace,omitempty"`

	// InferenceProxy configures the transparent inference metering proxy.
	// +optional
	InferenceProxy *InferenceProxySpec `json:"inferenceProxy,omitempty"`

	// ResourceTiers defines named resource presets for sandbox pods.
	// +optional
	ResourceTiers map[string]corev1.ResourceRequirements `json:"resourceTiers,omitempty"`
}

// CloudSandboxPersistenceSpec configures PVC-backed persistent workspaces.
type CloudSandboxPersistenceSpec struct {
	// Enabled enables PVC-backed workspaces instead of emptyDir.
	Enabled bool `json:"enabled,omitempty"`
	// StorageClass is the storage class for workspace PVCs.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`
	// Size is the storage size for workspace PVCs (e.g. "10Gi").
	// +kubebuilder:default="10Gi"
	// +optional
	Size string `json:"size,omitempty"`
}

// InferenceProxySpec configures the transparent inference metering proxy.
type InferenceProxySpec struct {
	// Enabled enables the inference proxy sidecar for metered API access.
	Enabled bool `json:"enabled,omitempty"`
	// Image is the inference proxy container image.
	// +optional
	Image string `json:"image,omitempty"`
	// Port is the port the proxy listens on.
	// +kubebuilder:default=8090
	// +optional
	Port int32 `json:"port,omitempty"`
}

// ConnectionsSpec configures third-party OAuth provider credentials.
// The operator injects credentials as PAPERCLIP_OAUTH_CREDENTIALS from
// the referenced Secret, enabling the Paperclip connections system to
// manage OAuth flows and token lifecycle for external services.
type ConnectionsSpec struct {
	// CredentialsSecretRef references a Secret containing OAuth client credentials.
	// The Secret must contain a key (default "PAPERCLIP_OAUTH_CREDENTIALS") whose
	// value is a JSON object mapping provider IDs to {clientId, clientSecret} pairs.
	// Example: {"github":{"clientId":"...","clientSecret":"..."},"slack":{"clientId":"...","clientSecret":"..."}}
	CredentialsSecretRef corev1.LocalObjectReference `json:"credentialsSecretRef"`

	// CredentialsKey is the key within the Secret that holds the JSON credentials.
	// Defaults to "PAPERCLIP_OAUTH_CREDENTIALS".
	// +kubebuilder:default="PAPERCLIP_OAUTH_CREDENTIALS"
	// +optional
	CredentialsKey string `json:"credentialsKey,omitempty"`

	// ProvidersConfigRef optionally references a ConfigMap containing a
	// PAPERCLIP_OAUTH_PROVIDERS key with a JSON provider catalog to extend
	// or override the built-in provider definitions at runtime.
	// +optional
	ProvidersConfigRef *corev1.LocalObjectReference `json:"providersConfigRef,omitempty"`
}

// TailscaleSpec configures an ephemeral Tailscale sidecar for secure tailnet
// access. When enabled, a userspace tailscaled sidecar runs alongside the
// Paperclip container and Serves the app (port 3100) over the tailnet via
// TS_SERVE_CONFIG. Use an ephemeral, reusable auth key from the Tailscale admin
// console so the node is automatically removed when the pod is deleted.
type TailscaleSpec struct {
	// Enabled enables the Tailscale sidecar.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Mode selects the Tailscale exposure mode.
	// "serve" exposes the instance to tailnet members only (default).
	// "funnel" exposes the instance to the public internet via Tailscale Funnel.
	// +kubebuilder:validation:Enum=serve;funnel
	// +kubebuilder:default="serve"
	// +optional
	Mode string `json:"mode,omitempty"`

	// Image configures the Tailscale sidecar container image.
	// +optional
	Image TailscaleImageSpec `json:"image,omitempty"`

	// AuthKey references a Secret containing the Tailscale auth key. The Secret
	// must have a key matching AuthKey.Key (default: "authkey"). Use an
	// ephemeral+reusable key from the Tailscale admin console.
	// +optional
	AuthKey *TailscaleAuthKeySpec `json:"authKey,omitempty"`

	// Hostname sets the Tailscale device name (defaults to the instance name).
	// +optional
	Hostname string `json:"hostname,omitempty"`

	// Resources specifies compute resources for the Tailscale sidecar container.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// TailscaleAuthKeySpec references a Secret key holding the Tailscale auth key.
type TailscaleAuthKeySpec struct {
	// SecretRef references the Secret containing the auth key.
	SecretRef corev1.LocalObjectReference `json:"secretRef"`

	// Key is the key within the referenced Secret. Defaults to "authkey".
	// +kubebuilder:default="authkey"
	// +optional
	Key string `json:"key,omitempty"`
}

// TailscaleImageSpec defines the Tailscale sidecar container image.
type TailscaleImageSpec struct {
	// Repository is the container image repository.
	// +kubebuilder:default="ghcr.io/tailscale/tailscale"
	// +optional
	Repository string `json:"repository,omitempty"`

	// Tag is the container image tag.
	// +kubebuilder:default="stable"
	// +optional
	Tag string `json:"tag,omitempty"`

	// Digest is the container image digest for supply-chain security.
	// +optional
	Digest string `json:"digest,omitempty"`
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
	// Pointer so an explicit `false` survives marshaling (a plain bool with
	// omitempty is dropped and re-defaulted to true by the API server on
	// every controller update — same bug class as PersistenceSpec.Enabled).
	// +kubebuilder:default=true
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// AllowIngressCIDRs specifies additional CIDR blocks allowed to reach the Paperclip service.
	// +optional
	// +listType=set
	// +kubebuilder:validation:items:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}$`
	AllowIngressCIDRs []string `json:"allowIngressCIDRs,omitempty"`

	// AllowEgressCIDRs specifies additional CIDR blocks the pod can reach.
	// +optional
	// +listType=set
	// +kubebuilder:validation:items:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}$`
	AllowEgressCIDRs []string `json:"allowEgressCIDRs,omitempty"`

	// ExtraEgress appends additional egress rules verbatim to the
	// operator-managed NetworkPolicy — e.g. to reach an in-cluster
	// object-storage endpoint (spec.objectStorage) or other services the
	// default rules don't cover. Required when networkPolicy is enabled and
	// objectStorage points at an in-cluster endpoint.
	// +optional
	ExtraEgress []networkingv1.NetworkPolicyEgressRule `json:"extraEgress,omitempty"`
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

	// HTTPRoute configures a Gateway API HTTPRoute.
	// This is an alternative to Ingress for clusters using the Gateway API.
	// +optional
	HTTPRoute *HTTPRouteSpec `json:"httpRoute,omitempty"`
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

// HTTPRouteSpec configures a Gateway API HTTPRoute.
type HTTPRouteSpec struct {
	// Enabled controls whether an HTTPRoute is created.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// ParentRefs specifies the Gateways this HTTPRoute attaches to.
	// Each ref identifies a Gateway by name (and optionally namespace and sectionName).
	// +optional
	ParentRefs []HTTPRouteParentRef `json:"parentRefs,omitempty"`

	// Hostnames specifies the hostnames matched by this HTTPRoute.
	// +optional
	Hostnames []string `json:"hostnames,omitempty"`

	// Annotations specifies additional annotations for the HTTPRoute.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// HTTPRouteParentRef identifies a Gateway (or other parent) that the HTTPRoute attaches to.
type HTTPRouteParentRef struct {
	// Name is the name of the Gateway.
	Name string `json:"name"`

	// Namespace is the namespace of the Gateway.
	// Defaults to the same namespace as the HTTPRoute.
	// +optional
	Namespace *string `json:"namespace,omitempty"`

	// SectionName is the name of a specific listener on the Gateway to attach to.
	// +optional
	SectionName *string `json:"sectionName,omitempty"`
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

	// PrometheusRule configures an auto-provisioned PrometheusRule with default
	// operator alerts (reconcile errors, instance not-ready, restart rate).
	// +optional
	PrometheusRule *PrometheusRuleSpec `json:"prometheusRule,omitempty"`

	// GrafanaDashboard configures auto-provisioned Grafana dashboard ConfigMaps.
	// +optional
	GrafanaDashboard *GrafanaDashboardSpec `json:"grafanaDashboard,omitempty"`
}

// PrometheusRuleSpec configures an auto-provisioned PrometheusRule.
type PrometheusRuleSpec struct {
	// Enabled enables PrometheusRule creation with operator alerts.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Labels to add to the PrometheusRule (e.g. for Prometheus rule selector matching).
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// RunbookBaseURL is the base URL for alert runbook links.
	// +kubebuilder:default="https://paperclip.inc/docs/operators/paperclip/runbooks"
	// +optional
	RunbookBaseURL string `json:"runbookBaseURL,omitempty"`
}

// GrafanaDashboardSpec configures auto-provisioned Grafana dashboard ConfigMaps.
type GrafanaDashboardSpec struct {
	// Enabled enables Grafana dashboard ConfigMap creation.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// Labels to add to the dashboard ConfigMaps (in addition to grafana_dashboard: "1").
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Folder is the Grafana folder to place the dashboards in.
	// +kubebuilder:default="Paperclip"
	// +optional
	Folder string `json:"folder,omitempty"`
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
	// Replicas is the desired number of Paperclip server pods. This field is
	// also the target of the scale subresource, so `kubectl scale` writes it.
	// Ignored when autoScaling is enabled (the HPA manages replicas), which
	// makes `kubectl scale` a no-op while the HPA is active.
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

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
	// Type specifies the probe mechanism: "auto" (default), "http", or "tcp".
	// "auto" uses HTTP probes in local_trusted mode and TCP probes in authenticated mode
	// (where /api/health returns 403 without credentials).
	// +kubebuilder:default="auto"
	// +kubebuilder:validation:Enum=auto;http;tcp
	// +optional
	Type string `json:"type,omitempty"`

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

// BackupSpec configures database backups. Two complementary mechanisms:
// the operator's pg_dump -> S3 CronJob (Schedule/S3, for managed/external
// PostgreSQL) and Paperclip's built-in local-dir backups (AppNative).
type BackupSpec struct {
	// Schedule is a cron expression for the operator pg_dump -> S3 backup CronJob.
	// Omit to use only app-native backups.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// S3 configures the S3 backup destination. Uses ObjectStorage config if not specified.
	// +optional
	S3 *BackupS3Spec `json:"s3,omitempty"`

	// RetentionDays specifies how many days to retain operator (S3) backups.
	// +kubebuilder:default=30
	// +optional
	RetentionDays *int32 `json:"retentionDays,omitempty"`

	// AppNative configures Paperclip's built-in database backups, written to a
	// local directory under the data PVC. Complementary to the operator's S3
	// CronJob; durable only when spec.storage.persistence is enabled.
	// +optional
	AppNative *AppNativeBackupSpec `json:"appNative,omitempty"`
}

// AppNativeBackupSpec configures Paperclip's built-in database backups
// (PAPERCLIP_DB_BACKUP_*).
type AppNativeBackupSpec struct {
	// Enabled toggles app-native backups. Maps to PAPERCLIP_DB_BACKUP_ENABLED.
	// When unset, the app default (enabled) applies.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// IntervalMinutes between backups. Maps to PAPERCLIP_DB_BACKUP_INTERVAL_MINUTES.
	// +optional
	IntervalMinutes int32 `json:"intervalMinutes,omitempty"`

	// RetentionDays for local backups. Maps to PAPERCLIP_DB_BACKUP_RETENTION_DAYS.
	// +optional
	RetentionDays int32 `json:"retentionDays,omitempty"`
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

	// ForcePathStyle forces path-style S3 addressing (bucket in the URL path
	// rather than the hostname). Defaults to true when provider is "minio"
	// (virtual-hosted addressing needs wildcard DNS that in-cluster MinIO
	// deployments don't have), false otherwise. Maps to
	// PAPERCLIP_STORAGE_S3_FORCE_PATH_STYLE.
	// +optional
	ForcePathStyle *bool `json:"forcePathStyle,omitempty"`
}

// --- Status types ---

// InstancePhase describes the phase of a Instance.
// +kubebuilder:validation:Enum=Pending;Provisioning;Running;Degraded;Failed;Terminating;BackingUp;Restoring;Updating;Suspended
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
	PhaseSuspended    InstancePhase = "Suspended"
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

	// AutoUpdate tracks the state of automatic image update checks.
	// +optional
	AutoUpdate *AutoUpdateStatus `json:"autoUpdate,omitempty"`

	// Replicas is the observed replica count of the active server workload (scale subresource).
	// +optional
	Replicas int32 `json:"replicas,omitempty"`

	// Selector is the label selector for the scale subresource (string form).
	// +optional
	Selector string `json:"selector,omitempty"`

	// SchedulerLeader is the pod currently holding the scheduler lease (lease gating only).
	// +optional
	SchedulerLeader string `json:"schedulerLeader,omitempty"`
}

// ManagedResources tracks the names of managed Kubernetes resources.
type ManagedResources struct {
	// +optional
	StatefulSet string `json:"statefulSet,omitempty"`
	// +optional
	Deployment string `json:"deployment,omitempty"`
	// +optional
	Service string `json:"service,omitempty"`
	// +optional
	ConfigMap string `json:"configMap,omitempty"`
	// +optional
	PersistentVolumeClaim string `json:"persistentVolumeClaim,omitempty"`
	// +optional
	Ingress string `json:"ingress,omitempty"`
	// +optional
	HTTPRoute string `json:"httpRoute,omitempty"`
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
	// +optional
	PrometheusRule string `json:"prometheusRule,omitempty"`
	// +optional
	GrafanaDashboardOperator string `json:"grafanaDashboardOperator,omitempty"`
	// +optional
	GrafanaDashboardInstance string `json:"grafanaDashboardInstance,omitempty"`
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

// AutoUpdateStatus tracks the state of automatic image update checks.
type AutoUpdateStatus struct {
	// LastCheckTime is when the operator last queried the registry.
	// +optional
	LastCheckTime *metav1.Time `json:"lastCheckTime,omitempty"`

	// ResolvedDigest is the most recently observed digest for the configured tag.
	// +optional
	ResolvedDigest string `json:"resolvedDigest,omitempty"`

	// LastUpdateTime is when the digest last changed and a rollout was triggered.
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// LastError records the most recent error from a registry check, if any.
	// +optional
	LastError string `json:"lastError,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:subresource:scale:specpath=.spec.availability.replicas,statuspath=.status.replicas,selectorpath=.status.selector
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
