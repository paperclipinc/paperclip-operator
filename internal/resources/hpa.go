package resources

import (
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"

	paperclipv1alpha1 "github.com/stubbi/paperclip-operator/api/v1alpha1"
)

// BuildHorizontalPodAutoscaler constructs the HPA for a Instance.
func BuildHorizontalPodAutoscaler(instance *paperclipv1alpha1.Instance) *autoscalingv2.HorizontalPodAutoscaler {
	as := instance.Spec.Availability.AutoScaling
	if as == nil {
		return nil
	}

	minReplicas := int32(1)
	if as.MinReplicas != nil {
		minReplicas = *as.MinReplicas
	}

	maxReplicas := as.MaxReplicas
	if maxReplicas == 0 {
		maxReplicas = 3
	}

	var metrics []autoscalingv2.MetricSpec

	if as.TargetCPUUtilizationPercentage != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: as.TargetCPUUtilizationPercentage,
				},
			},
		})
	}

	if as.TargetMemoryUtilizationPercentage != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: as.TargetMemoryUtilizationPercentage,
				},
			},
		})
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: ObjectMeta(instance, HPAName(instance)),
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "StatefulSet",
				Name:       StatefulSetName(instance),
			},
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			Metrics:     metrics,
		},
	}

	return hpa
}
