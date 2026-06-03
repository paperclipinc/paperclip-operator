/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterDefaultsSingletonName is the only accepted name for the cluster-scoped
// PaperclipClusterDefaults singleton. A PaperclipClusterDefaults under any other
// name is ignored by the operator and reported as Invalid in its status.
const ClusterDefaultsSingletonName = "cluster"

// PaperclipClusterDefaultsSpec defines cluster-wide defaults that the operator
// merges into every Instance at reconcile time. Per-instance fields always win:
// a default is only applied when the corresponding instance field is unset.
type PaperclipClusterDefaultsSpec struct {
	// Image is the default container image configuration applied to instances
	// where the corresponding instance fields are unset. Each sub-field is
	// merged independently (e.g. a cluster-default tag still applies even when
	// the instance sets its own repository).
	// +optional
	Image ImageSpec `json:"image,omitempty"`

	// StorageClass is the default storage class applied to the Paperclip data
	// PVC, the managed PostgreSQL PVC, and the managed Redis PVC when those
	// fields are unset on the instance.
	// +optional
	StorageClass string `json:"storageClass,omitempty"`

	// DatabaseMode is the default database mode ("embedded", "external", or
	// "managed") applied to instances where spec.database.mode is unset.
	// +kubebuilder:validation:Enum=embedded;external;managed
	// +optional
	DatabaseMode string `json:"databaseMode,omitempty"`

	// Observability configures cluster-wide observability defaults that are
	// merged into instances where the corresponding fields are unset.
	// +optional
	Observability ObservabilitySpec `json:"observability,omitempty"`

	// Networking configures cluster-wide networking defaults. Currently only
	// the default Service type is merged when the instance leaves it unset.
	// +optional
	Networking NetworkingSpec `json:"networking,omitempty"`

	// Env is a list of default environment variables merged into every
	// instance's container env. Instance-level env entries with the same Name
	// override the cluster default for that name. Defaults appear first in the
	// resulting env list, followed by instance-only names.
	// +listType=map
	// +listMapKey=name
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// PaperclipClusterDefaultsStatus reports which singleton (if any) is currently
// being applied by the operator.
type PaperclipClusterDefaultsStatus struct {
	// Conditions describes the current state of the singleton, including
	// whether the name matches the expected "cluster" singleton.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration is the generation of the spec most recently
	// processed by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=pccd
// +kubebuilder:printcolumn:name="DatabaseMode",type=string,JSONPath=`.spec.databaseMode`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PaperclipClusterDefaults is a cluster-scoped singleton (name must be "cluster")
// that provides default values merged into every Instance at reconcile time. It
// gives platform operators a single source of truth for org-wide image, storage
// class, database mode, observability, networking, and shared environment-variable
// defaults without duplicating the same boilerplate in every Instance manifest.
//
// Precedence: per-instance fields always win over cluster defaults. A default is
// only applied when the corresponding instance field is unset. The merged values
// are used only for rendering owned resources; the user's stored spec in etcd is
// never overwritten.
type PaperclipClusterDefaults struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PaperclipClusterDefaultsSpec   `json:"spec,omitempty"`
	Status PaperclipClusterDefaultsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PaperclipClusterDefaultsList contains a list of PaperclipClusterDefaults.
type PaperclipClusterDefaultsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PaperclipClusterDefaults `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PaperclipClusterDefaults{}, &PaperclipClusterDefaultsList{})
}
