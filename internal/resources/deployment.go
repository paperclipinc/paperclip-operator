package resources

import (
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// BuildDeployment constructs the Paperclip server Deployment for stateless
// multi-replica instances (spec.workload=Deployment or auto). It shares the
// pod template, name, labels, and selector with BuildStatefulSet so the
// Service (which selects on labels, not workload kind) keeps matching and the
// workload name stays stable across a StatefulSet<->Deployment switch.
//
// Rollouts surge by one pod (maxSurge=1, maxUnavailable=0) so capacity never
// drops below the desired replica count during updates.
func BuildDeployment(instance *paperclipv1alpha1.Instance, extraPodAnnotations map[string]string) *appsv1.Deployment {
	selectorLabels := SelectorLabels(instance)

	replicas := WorkloadReplicas(instance)

	return &appsv1.Deployment{
		ObjectMeta: ObjectMeta(instance, DeploymentName(instance)),
		Spec: appsv1.DeploymentSpec{
			Replicas:                &replicas,
			RevisionHistoryLimit:    Ptr(int32(10)),
			ProgressDeadlineSeconds: Ptr(int32(600)),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: BuildServerPodTemplate(instance, extraPodAnnotations),
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       Ptr(intstr.FromInt32(1)),
					MaxUnavailable: Ptr(intstr.FromInt32(0)),
				},
			},
		},
	}
}
