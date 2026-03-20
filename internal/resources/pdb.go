package resources

import (
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// BuildPodDisruptionBudget constructs the PDB for a Instance.
func BuildPodDisruptionBudget(instance *paperclipv1alpha1.Instance) *policyv1.PodDisruptionBudget {
	pdbSpec := instance.Spec.Availability.PodDisruptionBudget
	if pdbSpec == nil {
		return nil
	}

	pdb := &policyv1.PodDisruptionBudget{
		ObjectMeta: ObjectMeta(instance, PDBName(instance)),
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: SelectorLabels(instance),
			},
		},
	}

	if pdbSpec.MinAvailable != nil {
		pdb.Spec.MinAvailable = Ptr(intstr.FromInt32(*pdbSpec.MinAvailable))
	}
	if pdbSpec.MaxUnavailable != nil {
		pdb.Spec.MaxUnavailable = Ptr(intstr.FromInt32(*pdbSpec.MaxUnavailable))
	}

	return pdb
}
