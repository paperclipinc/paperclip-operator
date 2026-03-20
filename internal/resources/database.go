package resources

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// BuildDatabaseStatefulSet constructs the PostgreSQL StatefulSet for managed database mode.
func BuildDatabaseStatefulSet(instance *paperclipv1alpha1.Instance) *appsv1.StatefulSet {
	labels := LabelsWithComponent(instance, "database")
	selectorLabels := DatabaseSelectorLabels(instance)

	image := instance.Spec.Database.Managed.Image
	if image == "" {
		image = "postgres:17-alpine"
	}

	storageSize := instance.Spec.Database.Managed.StorageSize
	if storageSize.IsZero() {
		storageSize = resource.MustParse("10Gi")
	}

	container := corev1.Container{
		Name:  DatabaseContainerName,
		Image: image,
		Ports: []corev1.ContainerPort{
			{
				Name:          "postgres",
				ContainerPort: PostgreSQLPort,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		Env: []corev1.EnvVar{
			{Name: "POSTGRES_DB", Value: "paperclip"},
			{Name: "POSTGRES_USER", Value: "paperclip"},
			{
				Name: "POSTGRES_PASSWORD",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: DatabaseSecretName(instance),
						},
						Key: "password",
					},
				},
			},
			{Name: "PGDATA", Value: DatabaseMountPath + "/pgdata"},
			{Name: "POSTGRES_INITDB_ARGS", Value: "--data-checksums"},
		},
		Resources:                databaseResources(instance),
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"/bin/sh", "-c", "pg_ctl stop -m fast -D $PGDATA"},
				},
			},
		},
		ImagePullPolicy:          corev1.PullIfNotPresent,
		TerminationMessagePath:   "/dev/termination-log",
		TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      DatabaseVolumeName,
				MountPath: DatabaseMountPath,
			},
		},
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"pg_isready", "-U", "paperclip"},
				},
			},
			InitialDelaySeconds: 15,
			PeriodSeconds:       20,
			TimeoutSeconds:      5,
			FailureThreshold:    6,
			SuccessThreshold:    1,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"pg_isready", "-U", "paperclip"},
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
			TimeoutSeconds:      3,
			FailureThreshold:    3,
			SuccessThreshold:    1,
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: Ptr(false),
			RunAsNonRoot:             Ptr(true),
			RunAsUser:                Ptr(int64(70)), // postgres user
			RunAsGroup:               Ptr(int64(70)),
		},
	}

	replicas := int32(1)

	sts := &appsv1.StatefulSet{
		ObjectMeta: ObjectMeta(instance, DatabaseStatefulSetName(instance)),
		Spec: appsv1.StatefulSetSpec{
			Replicas:             &replicas,
			ServiceName:          DatabaseServiceName(instance),
			RevisionHistoryLimit: Ptr(int32(10)),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers:                    []corev1.Container{container},
					RestartPolicy:                 corev1.RestartPolicyAlways,
					DNSPolicy:                     corev1.DNSClusterFirst,
					SchedulerName:                 "default-scheduler",
					TerminationGracePeriodSeconds: Ptr(int64(30)),
					NodeSelector:                  instance.Spec.Availability.NodeSelector,
					Tolerations:                   instance.Spec.Availability.Tolerations,
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: Ptr(true),
						RunAsUser:    Ptr(int64(70)),
						RunAsGroup:   Ptr(int64(70)),
						FSGroup:      Ptr(int64(70)),
					},
					Volumes: []corev1.Volume{
						{
							Name: DatabaseVolumeName,
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: DatabasePVCName(instance),
								},
							},
						},
					},
				},
			},
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
		},
	}

	sts.Labels = labels

	return sts
}

func databaseResources(instance *paperclipv1alpha1.Instance) corev1.ResourceRequirements {
	r := instance.Spec.Database.Managed.Resources
	if len(r.Requests) == 0 && len(r.Limits) == 0 {
		return corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("250m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("1"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		}
	}
	return r
}

// BuildDatabaseSecret constructs the auto-generated database credentials Secret.
func BuildDatabaseSecret(instance *paperclipv1alpha1.Instance, password string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: ObjectMeta(instance, DatabaseSecretName(instance)),
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{
			"password": password,
		},
	}
}
