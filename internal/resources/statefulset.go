package resources

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// BuildStatefulSet constructs the Paperclip server StatefulSet.
func BuildStatefulSet(instance *paperclipv1alpha1.Instance) *appsv1.StatefulSet {
	labels := LabelsWithComponent(instance, "server")
	selectorLabels := SelectorLabels(instance)

	replicas := int32(1)

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
	if instance.Spec.Security.ContainerSecurityContext != nil {
		container.SecurityContext = instance.Spec.Security.ContainerSecurityContext
	} else {
		container.SecurityContext = &corev1.SecurityContext{
			AllowPrivilegeEscalation: Ptr(false),
			ReadOnlyRootFilesystem:   Ptr(false), // Paperclip needs writable filesystem for node_modules, etc.
			RunAsNonRoot:             Ptr(true),
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
	case "external":
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
		vars = append(vars, corev1.EnvVar{
			Name: "DATABASE_URL",
			Value: fmt.Sprintf("postgresql://paperclip:$(DB_PASSWORD)@%s-db.%s.svc.cluster.local:%d/paperclip",
				instance.Name, instance.Namespace, PostgreSQLPort),
		})
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
	}

	if instance.Spec.Secrets.StrictMode {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_SECRETS_STRICT_MODE", Value: "true"})
	}

	// Heartbeat scheduler
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
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_S3_BUCKET", Value: os.Bucket})
		if os.Region != "" {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_S3_REGION", Value: os.Region})
		}
		if os.Endpoint != "" {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_S3_ENDPOINT", Value: os.Endpoint})
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

	// Logging
	if instance.Spec.Observability.Logging.Level != "" {
		vars = append(vars, corev1.EnvVar{Name: "LOG_LEVEL", Value: instance.Spec.Observability.Logging.Level})
	}

	// User-supplied env vars (last, so they can override defaults)
	vars = append(vars, instance.Spec.Env...)

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
	mounts := []corev1.VolumeMount{
		{
			Name:      DataVolumeName,
			MountPath: DataMountPath,
		},
	}

	// Extra volume mounts
	mounts = append(mounts, instance.Spec.ExtraVolumeMounts...)

	return mounts
}

func buildLivenessProbe(instance *paperclipv1alpha1.Instance, port int32) *corev1.Probe {
	probe := &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   HealthPath,
				Port:   intstr.FromInt32(port),
				Scheme: corev1.URISchemeHTTP,
			},
		},
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
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   HealthPath,
				Port:   intstr.FromInt32(port),
				Scheme: corev1.URISchemeHTTP,
			},
		},
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
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path:   HealthPath,
				Port:   intstr.FromInt32(port),
				Scheme: corev1.URISchemeHTTP,
			},
		},
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

func containerImage(instance *paperclipv1alpha1.Instance) string {
	repo := instance.Spec.Image.Repository
	if repo == "" {
		repo = "ghcr.io/paperclipinc/paperclip"
	}

	if instance.Spec.Image.Digest != "" {
		return repo + "@" + instance.Spec.Image.Digest
	}

	tag := instance.Spec.Image.Tag
	if tag == "" {
		tag = "latest"
	}
	return repo + ":" + tag
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
