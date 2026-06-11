package resources

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// BuildServerPodTemplate constructs the Paperclip server pod template shared by
// the StatefulSet and Deployment workload builders. This is a mechanical
// extraction of the template construction that previously lived inline in
// BuildStatefulSet; the produced template is identical.
func BuildServerPodTemplate(instance *paperclipv1alpha1.Instance, extraPodAnnotations map[string]string) corev1.PodTemplateSpec {
	labels := LabelsWithComponent(instance, "server")

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

	// The app only needs to reach the in-cluster Kubernetes API when it is forced
	// onto the Kubernetes sandbox provider; mount the SA token only then. Otherwise
	// leave it UNSET (the prior default) so non-k8s instances are unchanged — setting
	// it to false here broke minimal-instance readiness in conformance.
	if IsKubernetesExecution(instance) {
		podSpec.AutomountServiceAccountToken = Ptr(true)
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

	// Platform instance-admin seed init container: idempotently seeds a
	// platform-managed instance-admin so the instance is never left in the
	// single-tenant "claim this instance" state. Must run AFTER onboard, which
	// applies the DB migrations the seed depends on (init containers run in
	// array order).
	if instance.Spec.Deployment.PlatformAdmin != nil && instance.Spec.Deployment.PlatformAdmin.Email != "" {
		podSpec.InitContainers = append(podSpec.InitContainers, buildSeedInstanceAdminInitContainer(instance))
	}

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

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      labels,
			Annotations: podAnnotations,
		},
		Spec: podSpec,
	}
}
