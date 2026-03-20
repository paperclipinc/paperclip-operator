package resources

import (
	corev1 "k8s.io/api/core/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// BuildServiceAccount constructs the ServiceAccount for a Instance.
func BuildServiceAccount(instance *paperclipv1alpha1.Instance) *corev1.ServiceAccount {
	sa := &corev1.ServiceAccount{
		ObjectMeta: ObjectMeta(instance, ServiceAccountName(instance)),
	}

	if instance.Spec.Security.RBAC.ServiceAccountAnnotations != nil {
		sa.Annotations = instance.Spec.Security.RBAC.ServiceAccountAnnotations
	}

	return sa
}
