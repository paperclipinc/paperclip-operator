package resources

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	paperclipv1alpha1 "github.com/stubbi/paperclip-operator/api/v1alpha1"
)

// BuildPersistentVolumeClaim constructs the PVC for the Paperclip data directory.
func BuildPersistentVolumeClaim(instance *paperclipv1alpha1.Instance) *corev1.PersistentVolumeClaim {
	size := instance.Spec.Storage.Persistence.Size
	if size.IsZero() {
		size = resource.MustParse("5Gi")
	}

	accessModes := instance.Spec.Storage.Persistence.AccessModes
	if len(accessModes) == 0 {
		accessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: ObjectMeta(instance, PVCName(instance)),
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: accessModes,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: size,
				},
			},
		},
	}

	if instance.Spec.Storage.Persistence.StorageClass != nil {
		pvc.Spec.StorageClassName = instance.Spec.Storage.Persistence.StorageClass
	}

	return pvc
}

// BuildDatabasePVC constructs the PVC for the managed PostgreSQL database.
func BuildDatabasePVC(instance *paperclipv1alpha1.Instance) *corev1.PersistentVolumeClaim {
	size := instance.Spec.Database.Managed.StorageSize
	if size.IsZero() {
		size = resource.MustParse("10Gi")
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: ObjectMeta(instance, DatabasePVCName(instance)),
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: size,
				},
			},
		},
	}

	if instance.Spec.Database.Managed.StorageClass != nil {
		pvc.Spec.StorageClassName = instance.Spec.Database.Managed.StorageClass
	}

	pvc.Labels = LabelsWithComponent(instance, "database")

	return pvc
}
