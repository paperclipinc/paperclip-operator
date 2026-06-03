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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SelfConfigAction names a category of mutation. Used by
// Instance.spec.selfConfigure.allowedActions to gate what the agent may
// request via PaperclipSelfConfig.
// +kubebuilder:validation:Enum=plugins;config;envVars
type SelfConfigAction string

const (
	// SelfConfigActionPlugins permits adding/removing plugins.
	SelfConfigActionPlugins SelfConfigAction = "plugins"
	// SelfConfigActionConfig permits patching the agent runtime config.
	SelfConfigActionConfig SelfConfigAction = "config"
	// SelfConfigActionEnvVars permits adding/removing plain-value env vars.
	SelfConfigActionEnvVars SelfConfigAction = "envVars"
)

// SelfConfigPhase represents the processing state of a self-config request.
// +kubebuilder:validation:Enum=Pending;Applied;Failed;Denied
type SelfConfigPhase string

const (
	// SelfConfigPhasePending is the initial, unprocessed state.
	SelfConfigPhasePending SelfConfigPhase = "Pending"
	// SelfConfigPhaseApplied means the request was applied via SSA.
	SelfConfigPhaseApplied SelfConfigPhase = "Applied"
	// SelfConfigPhaseFailed means the request could not be applied.
	SelfConfigPhaseFailed SelfConfigPhase = "Failed"
	// SelfConfigPhaseDenied means the request was rejected by policy.
	SelfConfigPhaseDenied SelfConfigPhase = "Denied"
)

// SelfConfigureSpec configures whether an agent can modify its own Instance.
type SelfConfigureSpec struct {
	// Enabled enables self-configuration for this instance. When true, the
	// agent can create PaperclipSelfConfig resources to modify its own spec.
	// +kubebuilder:default=false
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// AllowedActions restricts which action categories the agent can perform.
	// If empty and enabled is true, no actions are allowed (fail-safe).
	// +kubebuilder:validation:MaxItems=3
	// +optional
	AllowedActions []SelfConfigAction `json:"allowedActions,omitempty"`
}

// PaperclipSelfConfigSpec is an agent-driven, audited request to mutate the
// parent Instance. The operator validates against the parent's
// .spec.selfConfigure policy, then applies via Server-Side Apply with the
// dedicated field manager "paperclip-selfconfig" so GitOps controllers do not
// flap over the agent-owned fields.
type PaperclipSelfConfigSpec struct {
	// InstanceRef is the name of the parent Instance in the same namespace.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	InstanceRef string `json:"instanceRef"`

	// AddPlugins appends plugins to the parent's .spec.plugins.
	// +kubebuilder:validation:MaxItems=20
	// +optional
	AddPlugins []PluginRef `json:"addPlugins,omitempty"`

	// RemovePlugins is a list of plugin names to remove from the parent.
	// +kubebuilder:validation:MaxItems=20
	// +optional
	RemovePlugins []string `json:"removePlugins,omitempty"`

	// PatchConfig is a JSON object deep-merged into the agent runtime config
	// (exposed to the container as PAPERCLIP_CONFIG_PATCH). Protected keys are
	// rejected by the operator.
	// +kubebuilder:pruning:PreserveUnknownFields
	// +optional
	PatchConfig *apiextensionsv1.JSON `json:"patchConfig,omitempty"`

	// AddEnvVars is a list of environment variables to add (plain values only).
	// +kubebuilder:validation:MaxItems=20
	// +optional
	AddEnvVars []SelfConfigEnvVar `json:"addEnvVars,omitempty"`

	// RemoveEnvVars is a list of environment variable names to remove.
	// +kubebuilder:validation:MaxItems=20
	// +optional
	RemoveEnvVars []string `json:"removeEnvVars,omitempty"`
}

// SelfConfigEnvVar defines a plain-value environment variable (no secret refs).
type SelfConfigEnvVar struct {
	// Name of the environment variable. Must be a C_IDENTIFIER.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[A-Za-z_][A-Za-z0-9_]*$`
	Name string `json:"name"`

	// Value of the environment variable.
	Value string `json:"value"`
}

// PaperclipSelfConfigStatus defines the observed state of PaperclipSelfConfig.
type PaperclipSelfConfigStatus struct {
	// Phase is the processing state of this request.
	// +optional
	Phase SelfConfigPhase `json:"phase,omitempty"`

	// Message provides human-readable details about the current phase.
	// +optional
	Message string `json:"message,omitempty"`

	// ObservedGeneration is the spec generation the controller last processed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// CompletionTime is when the request reached a terminal phase.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Conditions surface fine-grained state.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=pcsc
// +kubebuilder:printcolumn:name="Instance",type=string,JSONPath=`.spec.instanceRef`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PaperclipSelfConfig is the Schema for the paperclipselfconfigs API. It
// represents a request from an agent to modify its own parent Instance spec,
// gated by the parent's .spec.selfConfigure allowlist and applied via
// Server-Side Apply with a dedicated field manager.
type PaperclipSelfConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PaperclipSelfConfigSpec   `json:"spec,omitempty"`
	Status PaperclipSelfConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PaperclipSelfConfigList contains a list of PaperclipSelfConfig.
type PaperclipSelfConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PaperclipSelfConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PaperclipSelfConfig{}, &PaperclipSelfConfigList{})
}
