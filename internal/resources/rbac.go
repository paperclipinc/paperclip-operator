package resources

import (
	paperclipv1alpha1 "github.com/openclaw-rocks/paperclip-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// BuildServiceAccount constructs the ServiceAccount for a PaperclipInstance.
func BuildServiceAccount(instance *paperclipv1alpha1.PaperclipInstance) *corev1.ServiceAccount {
	sa := &corev1.ServiceAccount{
		ObjectMeta: ObjectMeta(instance, ServiceAccountName(instance)),
	}

	if instance.Spec.Security.RBAC.ServiceAccountAnnotations != nil {
		sa.Annotations = instance.Spec.Security.RBAC.ServiceAccountAnnotations
	}

	return sa
}
