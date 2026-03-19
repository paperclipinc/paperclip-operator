package resources

import (
	paperclipv1alpha1 "github.com/openclaw-rocks/paperclip-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// BuildService constructs the Paperclip Service.
func BuildService(instance *paperclipv1alpha1.PaperclipInstance) *corev1.Service {
	port := servicePort(instance)
	svcType := corev1.ServiceTypeClusterIP
	if instance.Spec.Networking.Service.Type != "" {
		svcType = instance.Spec.Networking.Service.Type
	}

	svc := &corev1.Service{
		ObjectMeta: ObjectMeta(instance, ServiceName(instance)),
		Spec: corev1.ServiceSpec{
			Type:            svcType,
			Selector:        SelectorLabels(instance),
			SessionAffinity: corev1.ServiceAffinityNone,
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Port:     port,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}

	if instance.Spec.Networking.Service.Annotations != nil {
		svc.Annotations = instance.Spec.Networking.Service.Annotations
	}

	return svc
}

// BuildDatabaseService constructs the PostgreSQL Service for managed database mode.
func BuildDatabaseService(instance *paperclipv1alpha1.PaperclipInstance) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: ObjectMeta(instance, DatabaseServiceName(instance)),
		Spec: corev1.ServiceSpec{
			Type:            corev1.ServiceTypeClusterIP,
			Selector:        DatabaseSelectorLabels(instance),
			SessionAffinity: corev1.ServiceAffinityNone,
			Ports: []corev1.ServicePort{
				{
					Name:     "postgres",
					Port:     PostgreSQLPort,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}
}
