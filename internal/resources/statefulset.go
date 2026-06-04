package resources

import (
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// BuildStatefulSet constructs the Paperclip server StatefulSet.
func BuildStatefulSet(instance *paperclipv1alpha1.Instance, extraPodAnnotations map[string]string) *appsv1.StatefulSet {
	labels := LabelsWithComponent(instance, "server")
	selectorLabels := SelectorLabels(instance)

	replicas := StatefulSetReplicas(instance)

	container := buildMainContainer(instance)
	volumes := buildVolumes(instance)

	podSpec := corev1.PodSpec{
		Containers:                    []corev1.Container{container},
		Volumes:                       volumes,
		RestartPolicy:                 corev1.RestartPolicyAlways,
		DNSPolicy:                     corev1.DNSClusterFirst,
		SchedulerName:                 "default-scheduler",
		TerminationGracePeriodSeconds: Ptr(int64(30)),
		ServiceAccountName:            ServiceAccountName(instance),
		ShareProcessNamespace:         shareProcessNamespace(instance),
	}

	// Pod security context
	if instance.Spec.Security.PodSecurityContext != nil {
		podSpec.SecurityContext = instance.Spec.Security.PodSecurityContext
	} else {
		podSpec.SecurityContext = &corev1.PodSecurityContext{
			RunAsNonRoot: Ptr(true),
			RunAsUser:    Ptr(int64(1000)),
			RunAsGroup:   Ptr(int64(1000)),
			FSGroup:      Ptr(int64(1000)),
		}
	}

	// Image pull secrets
	if len(instance.Spec.Image.PullSecrets) > 0 {
		podSpec.ImagePullSecrets = instance.Spec.Image.PullSecrets
	}

	// Node scheduling
	if instance.Spec.Availability.NodeSelector != nil {
		podSpec.NodeSelector = instance.Spec.Availability.NodeSelector
	}
	if len(instance.Spec.Availability.Tolerations) > 0 {
		podSpec.Tolerations = instance.Spec.Availability.Tolerations
	}
	if instance.Spec.Availability.Affinity != nil {
		podSpec.Affinity = instance.Spec.Availability.Affinity
	}
	if len(instance.Spec.Availability.TopologySpreadConstraints) > 0 {
		podSpec.TopologySpreadConstraints = instance.Spec.Availability.TopologySpreadConstraints
	}

	// SELinux relabel init container: ensures the data volume labels match
	// the pod's SELinux context. Required because Kubernetes may assign MCS
	// categories to the volume that differ from the pod's level, making
	// the data inaccessible. Runs as privileged to perform chcon.
	if instance.Spec.Storage.Persistence.Enabled {
		seLevel := "s0"
		if instance.Spec.Security.PodSecurityContext != nil &&
			instance.Spec.Security.PodSecurityContext.SELinuxOptions != nil &&
			instance.Spec.Security.PodSecurityContext.SELinuxOptions.Level != "" {
			seLevel = instance.Spec.Security.PodSecurityContext.SELinuxOptions.Level
		}
		podSpec.InitContainers = append(podSpec.InitContainers, corev1.Container{
			Name:    "selinux-relabel",
			Image:   "fedora:latest",
			Command: []string{"chcon", "-R", "system_u:object_r:container_file_t:" + seLevel, DataMountPath},
			VolumeMounts: []corev1.VolumeMount{
				{Name: DataVolumeName, MountPath: DataMountPath},
			},
			SecurityContext: &corev1.SecurityContext{
				Privileged:   Ptr(true),
				RunAsUser:    Ptr(int64(0)),
				RunAsNonRoot: Ptr(false),
			},
		})
	}

	// Onboarding init container: runs non-interactive setup and admin bootstrap
	// before the server starts. Only runs when config doesn't exist yet.
	podSpec.InitContainers = append(podSpec.InitContainers, buildOnboardInitContainer(instance))

	// Tailscale sidecar (ephemeral node that Serves the app over the tailnet)
	if instance.Spec.Tailscale.Enabled {
		podSpec.Containers = append(podSpec.Containers, BuildTailscaleContainer(instance))
		podSpec.Volumes = append(podSpec.Volumes, TailscaleVolumes(instance)...)
	}

	// Custom sidecars
	podSpec.Containers = append(podSpec.Containers, instance.Spec.Sidecars...)

	// Custom init containers
	podSpec.InitContainers = append(podSpec.InitContainers, instance.Spec.InitContainers...)

	// Extra volumes
	podSpec.Volumes = append(podSpec.Volumes, instance.Spec.ExtraVolumes...)

	// Pod annotations
	podAnnotations := make(map[string]string)
	for k, v := range instance.Spec.PodAnnotations {
		podAnnotations[k] = v
	}
	for k, v := range extraPodAnnotations {
		podAnnotations[k] = v
	}

	// Prometheus scrape annotations
	podAnnotations["prometheus.io/scrape"] = "true"
	podAnnotations["prometheus.io/port"] = fmt.Sprintf("%d", servicePort(instance))
	podAnnotations["prometheus.io/path"] = "/metrics"

	sts := &appsv1.StatefulSet{
		ObjectMeta: ObjectMeta(instance, StatefulSetName(instance)),
		Spec: appsv1.StatefulSetSpec{
			Replicas:             &replicas,
			ServiceName:          ServiceName(instance),
			RevisionHistoryLimit: Ptr(int32(10)),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: podAnnotations,
				},
				Spec: podSpec,
			},
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

	// Multi-replica heartbeat gating: only pod-0 runs the scheduler.
	// Uses a shell wrapper that checks the StatefulSet ordinal in $HOSTNAME.
	if instance.Spec.Heartbeat.Enabled && EffectiveReplicas(instance) > 1 {
		container.Command = []string{"/bin/sh", "-c"}
		container.Args = []string{
			`case "$HOSTNAME" in *-0) export HEARTBEAT_SCHEDULER_ENABLED=true ;; *) export HEARTBEAT_SCHEDULER_ENABLED=false ;; esac; exec ` + DefaultPaperclipEntrypoint,
		}
	}

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
		{Name: "HOST", Value: "0.0.0.0"},
		{Name: "PAPERCLIP_HOME", Value: DataMountPath},
		{Name: "SERVE_UI", Value: "true"},
		{Name: "PAPERCLIP_DEPLOYMENT_MODE", Value: instance.Spec.Deployment.Mode},
		{Name: "PAPERCLIP_DEPLOYMENT_EXPOSURE", Value: instance.Spec.Deployment.Exposure},
	}

	// OpenTelemetry - load instrumentation before the app so OTEL can
	// hook into HTTP/Express/pg modules at require time.
	vars = append(vars,
		corev1.EnvVar{Name: "NODE_OPTIONS", Value: "--import ./server/dist/instrumentation.js"},
		corev1.EnvVar{Name: "OTEL_EXPORTER_OTLP_ENDPOINT", Value: "http://otel-collector.observability.svc.cluster.local:4317"},
		corev1.EnvVar{Name: "OTEL_SERVICE_NAME", Value: instance.Name},
		corev1.EnvVar{
			Name:  "OTEL_RESOURCE_ATTRIBUTES",
			Value: fmt.Sprintf("k8s.namespace.name=%s,k8s.statefulset.name=%s", instance.Namespace, StatefulSetName(instance)),
		},
	)

	// Public URL
	if instance.Spec.Deployment.PublicURL != "" {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_PUBLIC_URL", Value: instance.Spec.Deployment.PublicURL})
	}

	// Allowed hostnames
	if len(instance.Spec.Deployment.AllowedHostnames) > 0 {
		hostnamesStr := ""
		for i, h := range instance.Spec.Deployment.AllowedHostnames {
			if i > 0 {
				hostnamesStr += ","
			}
			hostnamesStr += h
		}
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_ALLOWED_HOSTNAMES", Value: hostnamesStr})
	}

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

	// Redis
	if instance.Spec.Redis != nil {
		redis := instance.Spec.Redis
		switch redis.Mode {
		case ModeExternal:
			if redis.ExternalURLSecretRef != nil {
				vars = append(vars, corev1.EnvVar{
					Name: "PAPERCLIP_RATE_LIMIT_REDIS_URL",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: redis.ExternalURLSecretRef,
					},
				})
			} else if redis.ExternalURL != "" {
				vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_RATE_LIMIT_REDIS_URL", Value: redis.ExternalURL})
			}
		default: // "managed" or empty
			vars = append(vars, corev1.EnvVar{
				Name:  "PAPERCLIP_RATE_LIMIT_REDIS_URL",
				Value: RedisURL(instance),
			})
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

	// Managed inference
	vars = append(vars, buildManagedInferenceEnvVars(instance)...)

	// Cloud sandbox
	vars = append(vars, buildCloudSandboxEnvVars(instance)...)

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

func buildManagedInferenceEnvVars(instance *paperclipv1alpha1.Instance) []corev1.EnvVar {
	if instance.Spec.Adapters.ManagedInferenceSecretRef == nil {
		return nil
	}

	secretRef := *instance.Spec.Adapters.ManagedInferenceSecretRef

	// Per-provider keys - each is optional in the Secret
	providerKeys := []string{
		"PAPERCLIP_MANAGED_ANTHROPIC_API_KEY",
		"PAPERCLIP_MANAGED_OPENAI_API_KEY",
		"PAPERCLIP_MANAGED_GEMINI_API_KEY",
		"PAPERCLIP_MANAGED_OPENROUTER_API_KEY",
	}

	vars := make([]corev1.EnvVar, 0, len(providerKeys)+3)
	for _, key := range providerKeys {
		vars = append(vars, corev1.EnvVar{
			Name: key,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: secretRef,
					Key:                  key,
					Optional:             Ptr(true),
				},
			},
		})
	}

	// Legacy single-key for backward compatibility
	vars = append(vars, corev1.EnvVar{
		Name: "PAPERCLIP_MANAGED_INFERENCE_API_KEY",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: secretRef,
				Key:                  "PAPERCLIP_MANAGED_INFERENCE_API_KEY",
				Optional:             Ptr(true),
			},
		},
	})

	if instance.Spec.Adapters.ManagedInferenceProvider != "" {
		vars = append(vars, corev1.EnvVar{
			Name:  "PAPERCLIP_MANAGED_INFERENCE_PROVIDER",
			Value: instance.Spec.Adapters.ManagedInferenceProvider,
		})
	}
	if instance.Spec.Adapters.ManagedInferenceModel != "" {
		vars = append(vars, corev1.EnvVar{
			Name:  "PAPERCLIP_MANAGED_INFERENCE_MODEL",
			Value: instance.Spec.Adapters.ManagedInferenceModel,
		})
	}

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

func buildVolumes(instance *paperclipv1alpha1.Instance) []corev1.Volume {
	var volumes []corev1.Volume

	if instance.Spec.Storage.Persistence.Enabled {
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
	if instance.Spec.Networking.Service.Port > 0 {
		return instance.Spec.Networking.Service.Port
	}
	return DefaultPort
}
