package resources

import (
	"encoding/json"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// BuildStatefulSet constructs the Paperclip server StatefulSet.
func BuildStatefulSet(instance *paperclipv1alpha1.Instance, extraPodAnnotations map[string]string) *appsv1.StatefulSet {
	selectorLabels := SelectorLabels(instance)

	replicas := WorkloadReplicas(instance)

	sts := &appsv1.StatefulSet{
		ObjectMeta: ObjectMeta(instance, StatefulSetName(instance)),
		Spec: appsv1.StatefulSetSpec{
			Replicas:             &replicas,
			ServiceName:          ServiceName(instance),
			RevisionHistoryLimit: Ptr(int32(10)),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: BuildServerPodTemplate(instance, extraPodAnnotations),
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
		},
	}

	return sts
}

func buildMainContainer(instance *paperclipv1alpha1.Instance) corev1.Container {
	image := containerImage(instance)
	port := servicePort(instance)

	container := corev1.Container{
		Name:  ContainerName,
		Image: image,
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: port,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env:                      buildEnvVars(instance),
		EnvFrom:                  instance.Spec.EnvFrom,
		Resources:                instance.Spec.Resources,
		ImagePullPolicy:          imagePullPolicy(instance),
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		VolumeMounts:             buildVolumeMounts(instance),
	}

	// Container security context
	container.SecurityContext = paperclipContainerSecurityContext(instance)
	if container.SecurityContext.ReadOnlyRootFilesystem == nil {
		container.SecurityContext = container.SecurityContext.DeepCopy()
		container.SecurityContext.ReadOnlyRootFilesystem = Ptr(false) // Paperclip needs writable filesystem for node_modules, etc.
	}

	// Always override the image ENTRYPOINT (docker-entrypoint.sh) and exec the
	// server directly. The image entrypoint gosu-drops from root to "node", which
	// fails under our runAsNonRoot / runAsUser:1000 / drop-ALL securityContext
	// ("failed switching to node: operation not permitted"). We already run as the
	// node uid, so there is nothing to drop to.
	container.Command = []string{"/bin/sh", "-c"}
	script := "exec " + DefaultPaperclipEntrypoint
	// Multi-replica heartbeat gating: only pod-0 runs the scheduler. Uses a shell
	// wrapper that checks the StatefulSet ordinal in $HOSTNAME. Applied only for
	// the "ordinal" gating mode on the StatefulSet workload: "lease" delegates
	// leadership to the app's lease-based leader election, and Deployment pods
	// have no stable ordinals for the wrapper to match (the controller surfaces
	// that combination via the SchedulerGatingValid condition).
	if instance.Spec.Heartbeat.Enabled && EffectiveReplicas(instance) > 1 &&
		SchedulerGatingMode(instance) == "ordinal" && !EffectiveWorkloadIsDeployment(instance) {
		script = `case "$HOSTNAME" in *-0) export HEARTBEAT_SCHEDULER_ENABLED=true ;; *) export HEARTBEAT_SCHEDULER_ENABLED=false ;; esac; ` + script
	}
	container.Args = []string{script}

	// Probes
	container.LivenessProbe = buildLivenessProbe(instance, port)
	container.ReadinessProbe = buildReadinessProbe(instance, port)
	container.StartupProbe = buildStartupProbe(instance, port)

	return container
}

func buildEnvVars(instance *paperclipv1alpha1.Instance) []corev1.EnvVar {
	port := servicePort(instance)
	vars := []corev1.EnvVar{
		{Name: "PORT", Value: fmt.Sprintf("%d", port)},
		{Name: "PAPERCLIP_BIND", Value: "custom"},
		{Name: "PAPERCLIP_BIND_HOST", Value: "0.0.0.0"},
		{Name: "PAPERCLIP_HOME", Value: DataMountPath},
		{Name: "SERVE_UI", Value: "true"},
		{Name: "PAPERCLIP_DEPLOYMENT_MODE", Value: instance.Spec.Deployment.Mode},
		{Name: "PAPERCLIP_DEPLOYMENT_EXPOSURE", Value: instance.Spec.Deployment.Exposure},
	}

	// OpenTelemetry - load instrumentation before the app so OTEL can hook into
	// HTTP/Express/pg modules at require time. Only inject this when observability
	// is enabled: the --import preload references ./server/dist/instrumentation.js,
	// which is only present in app images built with instrumentation; forcing it
	// unconditionally crashes the app (ERR_MODULE_NOT_FOUND) on images without it.
	if instance.Spec.Observability.Metrics.Enabled {
		vars = append(vars,
			corev1.EnvVar{Name: "NODE_OPTIONS", Value: "--import ./server/dist/instrumentation.js"},
			corev1.EnvVar{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: "http://otel-collector.observability.svc.cluster.local:4317"},
			corev1.EnvVar{Name: "OTEL_SERVICE_NAME", Value: instance.Name},
			corev1.EnvVar{
				Name:  "OTEL_RESOURCE_ATTRIBUTES",
				Value: fmt.Sprintf("k8s.namespace.name=%s,k8s.statefulset.name=%s", instance.Namespace, StatefulSetName(instance)),
			},
		)
	}

	// Public URL
	if instance.Spec.Deployment.PublicURL != "" {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_PUBLIC_URL", Value: instance.Spec.Deployment.PublicURL})
	}

	// Allowed hostnames: always include the in-cluster Service DNS names (plus
	// loopback) so the operator's own bootstrap Job and other in-cluster clients
	// pass the app's authenticated-mode hostname allowlist. User-specified
	// hostnames (e.g. the public ingress host) are appended.
	vars = append(vars, corev1.EnvVar{
		Name:  "PAPERCLIP_ALLOWED_HOSTNAMES",
		Value: strings.Join(allowedHostnames(instance), ","),
	})

	// Database URL
	switch instance.Spec.Database.Mode {
	case ModeExternal:
		if instance.Spec.Database.ExternalURLSecretRef != nil {
			vars = append(vars, corev1.EnvVar{
				Name: "DATABASE_URL",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: instance.Spec.Database.ExternalURLSecretRef,
				},
			})
		} else if instance.Spec.Database.ExternalURL != "" {
			vars = append(vars, corev1.EnvVar{Name: "DATABASE_URL", Value: instance.Spec.Database.ExternalURL})
		}
	case "managed":
		// DB_PASSWORD must be defined before DATABASE_URL for $(DB_PASSWORD) substitution to work
		vars = append(vars, corev1.EnvVar{
			Name: "DB_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: DatabaseSecretName(instance),
					},
					Key: "password",
				},
			},
		})
		vars = append(vars, corev1.EnvVar{
			Name: "DATABASE_URL",
			Value: fmt.Sprintf("postgresql://paperclip:$(DB_PASSWORD)@%s-db.%s.svc.cluster.local:%d/paperclip",
				instance.Name, instance.Namespace, PostgreSQLPort),
		})
		// "embedded" mode uses PGlite - no DATABASE_URL needed
	}

	// Auth secret
	if instance.Spec.Auth.SecretRef != nil {
		vars = append(vars, corev1.EnvVar{
			Name: "BETTER_AUTH_SECRET",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: instance.Spec.Auth.SecretRef,
			},
		})
	}

	// Disable public self-service sign-up (former single-tenant behavior)
	if instance.Spec.Auth.DisableSignUp {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_AUTH_DISABLE_SIGN_UP", Value: "true"})
	}

	// Secrets management master key
	if instance.Spec.Secrets.MasterKeySecretRef != nil {
		vars = append(vars, corev1.EnvVar{
			Name: "PAPERCLIP_SECRETS_MASTER_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: instance.Spec.Secrets.MasterKeySecretRef,
			},
		})
	} else {
		// Always inject from auto-generated secret to ensure all replicas share the same key
		vars = append(vars, corev1.EnvVar{
			Name: "PAPERCLIP_SECRETS_MASTER_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: SecretsMasterKeySecretName(instance),
					},
					Key: "master-key",
				},
			},
		})
	}

	if instance.Spec.Secrets.StrictMode {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_SECRETS_STRICT_MODE", Value: "true"})
	}

	// Secrets provider (e.g. AWS Secrets Manager)
	vars = append(vars, buildSecretsProviderEnvVars(instance)...)

	// App-native database backups
	vars = append(vars, buildAppNativeBackupEnvVars(instance)...)

	// Auth: email delivery and OAuth providers
	vars = append(vars, buildAuthEmailAndOAuthEnvVars(instance)...)

	// Heartbeat scheduler
	// When heartbeat is disabled, explicitly disable it on all pods.
	// When enabled with multiple replicas, the command wrapper handles per-pod gating
	// (only pod-0 runs the scheduler), so we skip the static env var here.
	if !instance.Spec.Heartbeat.Enabled {
		vars = append(vars, corev1.EnvVar{Name: "HEARTBEAT_SCHEDULER_ENABLED", Value: "false"})
	}
	if instance.Spec.Heartbeat.IntervalMS > 0 {
		vars = append(vars, corev1.EnvVar{
			Name:  "HEARTBEAT_SCHEDULER_INTERVAL_MS",
			Value: fmt.Sprintf("%d", instance.Spec.Heartbeat.IntervalMS),
		})
	}

	// Object storage
	if instance.Spec.ObjectStorage != nil {
		os := instance.Spec.ObjectStorage
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_STORAGE_PROVIDER", Value: "s3"})
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_STORAGE_S3_BUCKET", Value: os.Bucket})
		if os.Region != "" {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_STORAGE_S3_REGION", Value: os.Region})
		}
		if os.Endpoint != "" {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_STORAGE_S3_ENDPOINT", Value: os.Endpoint})
		}
		// Path-style addressing: explicit value wins; nil defaults to true for
		// MinIO, whose in-cluster deployments lack the wildcard DNS that
		// virtual-hosted bucket addressing requires.
		forcePathStyle := os.Provider == "minio"
		if os.ForcePathStyle != nil {
			forcePathStyle = *os.ForcePathStyle
		}
		if forcePathStyle {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_STORAGE_S3_FORCE_PATH_STYLE", Value: "true"})
		}
		if os.CredentialsSecretRef != nil {
			vars = append(vars,
				corev1.EnvVar{
					Name: "AWS_ACCESS_KEY_ID",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: *os.CredentialsSecretRef,
							Key:                  "AWS_ACCESS_KEY_ID",
						},
					},
				},
				corev1.EnvVar{
					Name: "AWS_SECRET_ACCESS_KEY",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: *os.CredentialsSecretRef,
							Key:                  "AWS_SECRET_ACCESS_KEY",
						},
					},
				},
			)
		}
	}

	// LLM API keys
	if instance.Spec.Adapters.APIKeysSecretRef != nil {
		vars = append(vars, corev1.EnvVar{
			Name: "ANTHROPIC_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: *instance.Spec.Adapters.APIKeysSecretRef,
					Key:                  "ANTHROPIC_API_KEY",
					Optional:             Ptr(true),
				},
			},
		})
		vars = append(vars, corev1.EnvVar{
			Name: "OPENAI_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: *instance.Spec.Adapters.APIKeysSecretRef,
					Key:                  "OPENAI_API_KEY",
					Optional:             Ptr(true),
				},
			},
		})
	}

	// E2B sandbox provider API key
	if instance.Spec.Adapters.E2B != nil {
		ref := instance.Spec.Adapters.E2B.APIKeySecretRef
		vars = append(vars, corev1.EnvVar{
			Name:      "E2B_API_KEY",
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &ref},
		})
	}

	// Cloud sandbox
	vars = append(vars, buildCloudSandboxEnvVars(instance)...)

	// In-cluster Kubernetes execution policy (@paperclipai/plugin-kubernetes)
	vars = append(vars, buildExecutionEnvVars(instance)...)

	// Declarative adapter registry (PAPERCLIP_ADAPTERS) consumed by the server's
	// adapter-registry bootstrap (picker availability + k8s runtime wiring).
	vars = append(vars, buildAdapterRegistryEnvVars(instance)...)

	// OAuth connections
	if instance.Spec.Connections != nil {
		conn := instance.Spec.Connections
		key := conn.CredentialsKey
		if key == "" {
			key = EnvOAuthCredentials
		}
		vars = append(vars, corev1.EnvVar{
			Name: EnvOAuthCredentials,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: conn.CredentialsSecretRef,
					Key:                  key,
				},
			},
		})
		if conn.ProvidersConfigRef != nil {
			vars = append(vars, corev1.EnvVar{
				Name: EnvOAuthProviders,
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: *conn.ProvidersConfigRef,
						Key:                  EnvOAuthProviders,
						Optional:             Ptr(true),
					},
				},
			})
		}
	}

	// Logging
	if instance.Spec.Observability.Logging.Level != "" {
		vars = append(vars, corev1.EnvVar{Name: "LOG_LEVEL", Value: instance.Spec.Observability.Logging.Level})
	}

	// User-supplied env vars (last, so they can override defaults)
	vars = append(vars, instance.Spec.Env...)

	return vars
}

func buildAuthEmailAndOAuthEnvVars(instance *paperclipv1alpha1.Instance) []corev1.EnvVar {
	var vars []corev1.EnvVar

	// Email (Resend)
	if instance.Spec.Auth.Email != nil {
		email := instance.Spec.Auth.Email
		if email.ResendAPIKeySecretRef != nil {
			vars = append(vars, corev1.EnvVar{
				Name: "RESEND_API_KEY",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: email.ResendAPIKeySecretRef,
				},
			})
		}
		if email.From != "" {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_EMAIL_FROM", Value: email.From})
		}
		if email.VerificationRequired {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_EMAIL_VERIFICATION_REQUIRED", Value: "true"})
		}
	}

	// Google OAuth
	if instance.Spec.Auth.Google != nil {
		secretRef := instance.Spec.Auth.Google.CredentialsSecretRef
		vars = append(vars,
			corev1.EnvVar{
				Name: "GOOGLE_CLIENT_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: secretRef,
						Key:                  "GOOGLE_CLIENT_ID",
					},
				},
			},
			corev1.EnvVar{
				Name: "GOOGLE_CLIENT_SECRET",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: secretRef,
						Key:                  "GOOGLE_CLIENT_SECRET",
					},
				},
			},
		)
	}

	// Apple OAuth
	if instance.Spec.Auth.Apple != nil {
		secretRef := instance.Spec.Auth.Apple.CredentialsSecretRef
		vars = append(vars,
			corev1.EnvVar{
				Name: "APPLE_CLIENT_ID",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: secretRef,
						Key:                  "APPLE_CLIENT_ID",
					},
				},
			},
			corev1.EnvVar{
				Name: "APPLE_CLIENT_SECRET",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: secretRef,
						Key:                  "APPLE_CLIENT_SECRET",
					},
				},
			},
		)
	}

	return vars
}

// allowedHostnames returns the hostname allowlist for PAPERCLIP_ALLOWED_HOSTNAMES:
// always the in-cluster Service DNS names plus loopback, then any user-specified
// hostnames, deduplicated and order-preserving.
func allowedHostnames(instance *paperclipv1alpha1.Instance) []string {
	svc := ServiceName(instance)
	ns := instance.Namespace
	extra := instance.Spec.Deployment.AllowedHostnames
	hosts := make([]string, 0, 6+len(extra))
	hosts = append(hosts,
		"localhost",
		"127.0.0.1",
		svc,
		svc+"."+ns,
		svc+"."+ns+".svc",
		svc+"."+ns+".svc.cluster.local",
	)
	hosts = append(hosts, extra...)

	seen := make(map[string]bool, len(hosts))
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		out = append(out, h)
	}
	return out
}

// buildSecretsProviderEnvVars emits the env vars selecting an external secrets
// provider. AWS credentials are intentionally not injected - the app uses the
// AWS SDK credential chain (IRSA via serviceAccountAnnotations).
func buildSecretsProviderEnvVars(instance *paperclipv1alpha1.Instance) []corev1.EnvVar {
	provider := instance.Spec.Secrets.Provider
	if provider == "" || provider == "local_encrypted" {
		return nil
	}

	vars := []corev1.EnvVar{{Name: "PAPERCLIP_SECRETS_PROVIDER", Value: provider}}
	if provider == "aws_secrets_manager" && instance.Spec.Secrets.AWS != nil {
		aws := instance.Spec.Secrets.AWS
		vars = append(vars,
			corev1.EnvVar{Name: "PAPERCLIP_SECRETS_AWS_REGION", Value: aws.Region},
			corev1.EnvVar{Name: "PAPERCLIP_SECRETS_AWS_KMS_KEY_ID", Value: aws.KMSKeyID},
			corev1.EnvVar{Name: "PAPERCLIP_SECRETS_AWS_DEPLOYMENT_ID", Value: aws.DeploymentID},
		)
		if aws.Prefix != "" {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_SECRETS_AWS_PREFIX", Value: aws.Prefix})
		}
		if aws.Environment != "" {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_SECRETS_AWS_ENVIRONMENT", Value: aws.Environment})
		}
		if aws.Endpoint != "" {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_SECRETS_AWS_ENDPOINT", Value: aws.Endpoint})
		}
		if aws.DeleteRecoveryDays != nil {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_SECRETS_AWS_DELETE_RECOVERY_DAYS", Value: fmt.Sprintf("%d", *aws.DeleteRecoveryDays)})
		}
	}
	return vars
}

// buildAppNativeBackupEnvVars emits Paperclip's built-in DB backup env vars,
// keeping the backup directory on the persistent data volume.
func buildAppNativeBackupEnvVars(instance *paperclipv1alpha1.Instance) []corev1.EnvVar {
	if instance.Spec.Backup == nil || instance.Spec.Backup.AppNative == nil {
		return nil
	}
	an := instance.Spec.Backup.AppNative
	var vars []corev1.EnvVar
	if an.Enabled != nil {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_DB_BACKUP_ENABLED", Value: fmt.Sprintf("%t", *an.Enabled)})
	}
	if an.IntervalMinutes > 0 {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_DB_BACKUP_INTERVAL_MINUTES", Value: fmt.Sprintf("%d", an.IntervalMinutes)})
	}
	if an.RetentionDays > 0 {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_DB_BACKUP_RETENTION_DAYS", Value: fmt.Sprintf("%d", an.RetentionDays)})
	}
	vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_DB_BACKUP_DIR", Value: DataMountPath + "/backups"})
	return vars
}

func buildCloudSandboxEnvVars(instance *paperclipv1alpha1.Instance) []corev1.EnvVar {
	cs := instance.Spec.Adapters.CloudSandbox
	if cs == nil || !cs.Enabled {
		return nil
	}

	vars := []corev1.EnvVar{
		{Name: "PAPERCLIP_CLOUD_SANDBOX_ENABLED", Value: "true"},
	}

	ns := cs.Namespace
	if ns == "" {
		ns = instance.Namespace
	}
	vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_CLOUD_SANDBOX_NAMESPACE", Value: ns})

	if cs.DefaultImage != "" {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_CLOUD_SANDBOX_DEFAULT_IMAGE", Value: cs.DefaultImage})
	}
	if cs.IdleTimeoutMin > 0 {
		vars = append(vars, corev1.EnvVar{
			Name:  "PAPERCLIP_CLOUD_SANDBOX_IDLE_TIMEOUT_MIN",
			Value: fmt.Sprintf("%d", cs.IdleTimeoutMin),
		})
	}

	// Phase 4: persistence
	if cs.Persistence != nil && cs.Persistence.Enabled {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_CLOUD_SANDBOX_PERSISTENCE_ENABLED", Value: "true"})
		if cs.Persistence.StorageClass != "" {
			vars = append(vars, corev1.EnvVar{
				Name:  "PAPERCLIP_CLOUD_SANDBOX_PERSISTENCE_STORAGE_CLASS",
				Value: cs.Persistence.StorageClass,
			})
		}
		if cs.Persistence.Size != "" {
			vars = append(vars, corev1.EnvVar{
				Name:  "PAPERCLIP_CLOUD_SANDBOX_PERSISTENCE_SIZE",
				Value: cs.Persistence.Size,
			})
		}
	}

	// Phase 4: multi-namespace isolation
	if cs.MultiNamespace {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_CLOUD_SANDBOX_MULTI_NAMESPACE", Value: "true"})
	}

	// Node scheduling: pass the instance's scheduling constraints so the
	// Paperclip server can apply them to sandbox pods it creates.
	if len(instance.Spec.Availability.NodeSelector) > 0 {
		if b, err := json.Marshal(instance.Spec.Availability.NodeSelector); err == nil {
			vars = append(vars, corev1.EnvVar{
				Name:  "PAPERCLIP_CLOUD_SANDBOX_NODE_SELECTOR",
				Value: string(b),
			})
		}
	}
	if len(instance.Spec.Availability.Tolerations) > 0 {
		if b, err := json.Marshal(instance.Spec.Availability.Tolerations); err == nil {
			vars = append(vars, corev1.EnvVar{
				Name:  "PAPERCLIP_CLOUD_SANDBOX_TOLERATIONS",
				Value: string(b),
			})
		}
	}

	return vars
}

// IsKubernetesExecution reports whether the instance is configured to force
// agent execution onto the in-cluster Kubernetes sandbox provider. This gates
// the execution RBAC and the app ServiceAccount-token mount.
func IsKubernetesExecution(instance *paperclipv1alpha1.Instance) bool {
	ex := instance.Spec.Adapters.Execution
	return ex != nil && ex.Mode == "kubernetes"
}

// buildExecutionEnvVars translates spec.adapters.execution into the
// PAPERCLIP_EXECUTION_MODE / PAPERCLIP_K8S_* env vars consumed by the fork's
// execution-policy bootstrap. Only emitted when execution is configured; when
// Mode is "any" (or the block is nil) the bootstrap stays unrestricted, so we
// emit nothing.
func buildExecutionEnvVars(instance *paperclipv1alpha1.Instance) []corev1.EnvVar {
	ex := instance.Spec.Adapters.Execution
	if ex == nil || ex.Mode != "kubernetes" {
		return nil
	}

	vars := []corev1.EnvVar{
		{Name: "PAPERCLIP_EXECUTION_MODE", Value: "kubernetes"},
		// The operator only ever wires the in-cluster path: the app talks to the
		// local kube-apiserver via its mounted ServiceAccount token.
		{Name: "PAPERCLIP_K8S_IN_CLUSTER", Value: "true"},
	}

	k := ex.Kubernetes
	if k == nil {
		return vars
	}

	if k.Backend != "" {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_K8S_BACKEND", Value: k.Backend})
	}
	if k.RuntimeClassName != "" {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_K8S_RUNTIME_CLASS_NAME", Value: k.RuntimeClassName})
	}
	if k.EgressMode != "" {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_K8S_EGRESS_MODE", Value: k.EgressMode})
	}
	if len(k.EgressAllowFQDNs) > 0 {
		vars = append(vars, corev1.EnvVar{
			Name:  "PAPERCLIP_K8S_EGRESS_ALLOW_FQDNS",
			Value: strings.Join(k.EgressAllowFQDNs, ","),
		})
	}
	if len(k.EgressAllowCIDRs) > 0 {
		vars = append(vars, corev1.EnvVar{
			Name:  "PAPERCLIP_K8S_EGRESS_ALLOW_CIDRS",
			Value: strings.Join(k.EgressAllowCIDRs, ","),
		})
	}
	if k.NamespacePrefix != "" {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_K8S_NAMESPACE_PREFIX", Value: k.NamespacePrefix})
	}

	return vars
}

// buildAdapterRegistryEnvVars marshals spec.adapters.registry into the
// PAPERCLIP_ADAPTERS env var. Emits nothing when the registry is empty so
// unconfigured instances keep their built-in adapter defaults.
func buildAdapterRegistryEnvVars(instance *paperclipv1alpha1.Instance) []corev1.EnvVar {
	registry := instance.Spec.Adapters.Registry
	if len(registry) == 0 {
		return nil
	}
	encoded, err := json.Marshal(registry)
	if err != nil {
		// Marshaling typed structs cannot fail in practice; emit nothing rather
		// than produce a malformed env var that would fail the server bootstrap.
		return nil
	}
	return []corev1.EnvVar{{Name: "PAPERCLIP_ADAPTERS", Value: string(encoded)}}
}

func buildVolumes(instance *paperclipv1alpha1.Instance) []corev1.Volume {
	var volumes []corev1.Volume

	if PersistenceEnabled(instance) {
		volumes = append(volumes, corev1.Volume{
			Name: DataVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: PVCName(instance),
				},
			},
		})
	} else {
		volumes = append(volumes, corev1.Volume{
			Name: DataVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	return volumes
}

func buildVolumeMounts(instance *paperclipv1alpha1.Instance) []corev1.VolumeMount {
	mounts := make([]corev1.VolumeMount, 0, 1+len(instance.Spec.ExtraVolumeMounts))
	mounts = append(mounts, corev1.VolumeMount{
		Name:      DataVolumeName,
		MountPath: DataMountPath,
	})
	mounts = append(mounts, instance.Spec.ExtraVolumeMounts...)
	return mounts
}

func probeHandler(instance *paperclipv1alpha1.Instance, port int32) corev1.ProbeHandler {
	if UseTCPProbes(instance) {
		return corev1.ProbeHandler{
			TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(port),
			},
		}
	}
	return corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{
			Path:   HealthPath,
			Port:   intstr.FromInt32(port),
			Scheme: corev1.URISchemeHTTP,
		},
	}
}

func buildLivenessProbe(instance *paperclipv1alpha1.Instance, port int32) *corev1.Probe {
	probe := &corev1.Probe{
		ProbeHandler:        probeHandler(instance, port),
		InitialDelaySeconds: 15,
		PeriodSeconds:       20,
		TimeoutSeconds:      5,
		FailureThreshold:    6,
		SuccessThreshold:    1,
	}

	if p := instance.Spec.Probes.Liveness; p != nil {
		if p.InitialDelaySeconds != nil {
			probe.InitialDelaySeconds = *p.InitialDelaySeconds
		}
		if p.PeriodSeconds != nil {
			probe.PeriodSeconds = *p.PeriodSeconds
		}
		if p.TimeoutSeconds != nil {
			probe.TimeoutSeconds = *p.TimeoutSeconds
		}
		if p.FailureThreshold != nil {
			probe.FailureThreshold = *p.FailureThreshold
		}
		if p.SuccessThreshold != nil {
			probe.SuccessThreshold = *p.SuccessThreshold
		}
	}

	return probe
}

func buildReadinessProbe(instance *paperclipv1alpha1.Instance, port int32) *corev1.Probe {
	probe := &corev1.Probe{
		ProbeHandler:        probeHandler(instance, port),
		InitialDelaySeconds: 5,
		PeriodSeconds:       10,
		TimeoutSeconds:      3,
		FailureThreshold:    3,
		SuccessThreshold:    1,
	}

	if p := instance.Spec.Probes.Readiness; p != nil {
		if p.InitialDelaySeconds != nil {
			probe.InitialDelaySeconds = *p.InitialDelaySeconds
		}
		if p.PeriodSeconds != nil {
			probe.PeriodSeconds = *p.PeriodSeconds
		}
		if p.TimeoutSeconds != nil {
			probe.TimeoutSeconds = *p.TimeoutSeconds
		}
		if p.FailureThreshold != nil {
			probe.FailureThreshold = *p.FailureThreshold
		}
		if p.SuccessThreshold != nil {
			probe.SuccessThreshold = *p.SuccessThreshold
		}
	}

	return probe
}

func buildStartupProbe(instance *paperclipv1alpha1.Instance, port int32) *corev1.Probe {
	probe := &corev1.Probe{
		ProbeHandler:        probeHandler(instance, port),
		InitialDelaySeconds: 0,
		PeriodSeconds:       5,
		TimeoutSeconds:      3,
		FailureThreshold:    30,
		SuccessThreshold:    1,
	}

	if p := instance.Spec.Probes.Startup; p != nil {
		if p.InitialDelaySeconds != nil {
			probe.InitialDelaySeconds = *p.InitialDelaySeconds
		}
		if p.PeriodSeconds != nil {
			probe.PeriodSeconds = *p.PeriodSeconds
		}
		if p.TimeoutSeconds != nil {
			probe.TimeoutSeconds = *p.TimeoutSeconds
		}
		if p.FailureThreshold != nil {
			probe.FailureThreshold = *p.FailureThreshold
		}
		if p.SuccessThreshold != nil {
			probe.SuccessThreshold = *p.SuccessThreshold
		}
	}

	return probe
}

func buildOnboardInitContainer(instance *paperclipv1alpha1.Instance) corev1.Container {
	image := containerImage(instance)

	// Create the Paperclip config file if it doesn't exist yet.
	// Uses `onboard --yes` which accepts quickstart defaults. Unfortunately this also
	// starts the server, so we run it in a subshell and kill the entire process group
	// once the config file appears.
	script := `
CONFIG="/paperclip/instances/default/config.json"
if [ -f "$CONFIG" ]; then
  echo "Config already exists, skipping onboard."
  exit 0
fi
echo "Running initial onboarding..."
# Run onboard in a separate process group so we can kill the whole tree
sh -c 'exec pnpm paperclipai onboard --yes' &
ONBOARD_PID=$!
# Wait for the config file to appear (onboard creates it before starting the server)
for i in $(seq 1 120); do
  if [ -f "$CONFIG" ]; then
    echo "Config created successfully."
    # Kill the entire process tree (onboard + server + node children)
    kill -9 $ONBOARD_PID 2>/dev/null || true
    # Also kill any remaining node processes started by onboard
    pkill -9 -f "paperclipai" 2>/dev/null || true
    pkill -9 -f "server/dist/index" 2>/dev/null || true
    exit 0
  fi
  sleep 1
done
echo "Timed out waiting for config file."
kill -9 $ONBOARD_PID 2>/dev/null || true
exit 1
`

	return corev1.Container{
		Name:            "onboard",
		Image:           image,
		ImagePullPolicy: imagePullPolicy(instance),
		Command:         []string{"/bin/sh", "-c"},
		Args:            []string{script},
		Env:             buildEnvVars(instance),
		EnvFrom:         instance.Spec.EnvFrom,
		VolumeMounts:    buildVolumeMounts(instance),
		SecurityContext: paperclipContainerSecurityContext(instance),
	}
}

// buildSeedInstanceAdminInitContainer runs the product CLI command that
// idempotently seeds a platform-managed instance-admin. It mirrors the onboard
// init container's invocation (pnpm paperclipai ...), securityContext, and
// volume mounts. It must be scheduled AFTER the onboard init container because
// onboard applies the DB migrations this command relies on.
//
// The CLI reads DATABASE_URL (reused from the same external-database secret/uri
// source the main app container uses, via buildBackupDBEnvVars) plus the
// PAPERCLIP_SEED_ADMIN_* env vars sourced from spec.deployment.platformAdmin.
func buildSeedInstanceAdminInitContainer(instance *paperclipv1alpha1.Instance) corev1.Container {
	image := containerImage(instance)
	admin := instance.Spec.Deployment.PlatformAdmin

	// DATABASE_URL (+ DB_PASSWORD for managed mode) from the same source the
	// main app container resolves its DB connection from.
	env := buildBackupDBEnvVars(instance)
	env = append(env, corev1.EnvVar{Name: "PAPERCLIP_SEED_ADMIN_EMAIL", Value: admin.Email})
	if admin.Name != "" {
		env = append(env, corev1.EnvVar{Name: "PAPERCLIP_SEED_ADMIN_NAME", Value: admin.Name})
	}
	if admin.UserID != "" {
		env = append(env, corev1.EnvVar{Name: "PAPERCLIP_SEED_ADMIN_USER_ID", Value: admin.UserID})
	}

	return corev1.Container{
		Name:            "seed-instance-admin",
		Image:           image,
		ImagePullPolicy: imagePullPolicy(instance),
		Command:         []string{"/bin/sh", "-c"},
		Args:            []string{"exec pnpm paperclipai auth seed-instance-admin"},
		Env:             env,
		EnvFrom:         instance.Spec.EnvFrom,
		VolumeMounts:    buildVolumeMounts(instance),
		SecurityContext: paperclipContainerSecurityContext(instance),
	}
}

// shareProcessNamespace returns the effective ShareProcessNamespace value,
// defaulting to true so a pause container reaps zombie processes left by the
// Node.js server (which does not call waitpid()). Users may opt out by setting
// spec.shareProcessNamespace to false.
func shareProcessNamespace(instance *paperclipv1alpha1.Instance) *bool {
	if instance.Spec.ShareProcessNamespace != nil {
		return instance.Spec.ShareProcessNamespace
	}
	return Ptr(true)
}

func containerImage(instance *paperclipv1alpha1.Instance) string {
	repo := instance.Spec.Image.Repository
	if repo == "" {
		repo = "ghcr.io/paperclipai/paperclip"
	}

	if instance.Spec.Image.Digest != "" {
		return repo + "@" + instance.Spec.Image.Digest
	}

	return repo + ":" + instance.Spec.Image.Tag
}

func imagePullPolicy(instance *paperclipv1alpha1.Instance) corev1.PullPolicy {
	if instance.Spec.Image.PullPolicy != "" {
		return instance.Spec.Image.PullPolicy
	}
	return corev1.PullIfNotPresent
}

func servicePort(instance *paperclipv1alpha1.Instance) int32 {
	return ServerPort(instance)
}
