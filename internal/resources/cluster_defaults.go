package resources

import (
	corev1 "k8s.io/api/core/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// ApplyClusterDefaults returns a deep copy of instance with unset fields filled
// from defaults. The original instance is never mutated so status/finalizer
// updates in the reconciler always write back the user's true spec, not a
// defaulted one.
//
// Precedence rules:
//   - Scalar fields (Image.Repository, Image.Tag, StorageClass, DatabaseMode,
//     Service.Type, log level, ...) inherit from the default only when the
//     instance value is the zero value.
//   - Env is the exception. Cluster defaults and instance env are merged by
//     Name: cluster entries appear first, and any instance entry with the same
//     Name overrides the cluster default's value in place.
//
// If defaults is nil, the instance is returned unchanged (still deep-copied so
// callers can safely mutate it for in-memory derivation).
func ApplyClusterDefaults(instance *paperclipv1alpha1.Instance, defaults *paperclipv1alpha1.PaperclipClusterDefaults) *paperclipv1alpha1.Instance {
	out := instance.DeepCopy()
	if defaults == nil {
		return out
	}

	d := defaults.Spec

	// Image: merge each sub-field independently.
	if out.Spec.Image.Repository == "" {
		out.Spec.Image.Repository = d.Image.Repository
	}
	// Only inherit the default Tag when the instance pinned neither a tag nor a
	// digest. Otherwise a cluster-default tag could silently override a
	// user-pinned digest resolution.
	if out.Spec.Image.Tag == "" && out.Spec.Image.Digest == "" {
		out.Spec.Image.Tag = d.Image.Tag
	}
	if out.Spec.Image.Digest == "" {
		out.Spec.Image.Digest = d.Image.Digest
	}
	if out.Spec.Image.PullPolicy == "" {
		out.Spec.Image.PullPolicy = d.Image.PullPolicy
	}
	if len(out.Spec.Image.PullSecrets) == 0 && len(d.Image.PullSecrets) > 0 {
		out.Spec.Image.PullSecrets = append([]corev1.LocalObjectReference(nil), d.Image.PullSecrets...)
	}

	// Storage class: applied to the Paperclip data PVC and the managed
	// PostgreSQL / Redis PVCs whenever the instance leaves the field unset.
	if d.StorageClass != "" {
		if out.Spec.Storage.Persistence.StorageClass == nil {
			out.Spec.Storage.Persistence.StorageClass = ptrTo(d.StorageClass)
		}
		if out.Spec.Database.Managed.StorageClass == nil {
			out.Spec.Database.Managed.StorageClass = ptrTo(d.StorageClass)
		}
		if out.Spec.Redis != nil && out.Spec.Redis.Managed.StorageClass == nil {
			out.Spec.Redis.Managed.StorageClass = ptrTo(d.StorageClass)
		}
	}

	// Database mode.
	if out.Spec.Database.Mode == "" {
		out.Spec.Database.Mode = d.DatabaseMode
	}

	// Observability: metrics + logging defaults only fill unset fields.
	if !out.Spec.Observability.Metrics.Enabled && d.Observability.Metrics.Enabled {
		out.Spec.Observability.Metrics.Enabled = true
	}
	if out.Spec.Observability.Logging.Level == "" {
		out.Spec.Observability.Logging.Level = d.Observability.Logging.Level
	}

	// Networking: default Service type.
	if out.Spec.Networking.Service.Type == "" {
		out.Spec.Networking.Service.Type = d.Networking.Service.Type
	}

	// Env: merge by Name (instance entries override cluster defaults).
	out.Spec.Env = mergeEnvVars(d.Env, out.Spec.Env)

	return out
}

// ptrTo returns a pointer to v. Kept local to avoid importing the controller
// helper into the pure resources package.
func ptrTo[T any](v T) *T {
	return &v
}

// mergeEnvVars returns a merged env list where cluster defaults appear first and
// any instance entry with a matching Name overrides the default's value in
// place. Instance-only names are appended in their original order.
func mergeEnvVars(defaults, instance []corev1.EnvVar) []corev1.EnvVar {
	if len(defaults) == 0 {
		if len(instance) == 0 {
			return nil
		}
		return append([]corev1.EnvVar(nil), instance...)
	}

	instanceByName := make(map[string]corev1.EnvVar, len(instance))
	for _, e := range instance {
		instanceByName[e.Name] = e
	}

	merged := make([]corev1.EnvVar, 0, len(defaults)+len(instance))
	seen := make(map[string]bool, len(defaults))
	for _, def := range defaults {
		seen[def.Name] = true
		if override, ok := instanceByName[def.Name]; ok {
			merged = append(merged, override)
			continue
		}
		merged = append(merged, def)
	}
	for _, e := range instance {
		if seen[e.Name] {
			continue
		}
		merged = append(merged, e)
	}
	return merged
}
