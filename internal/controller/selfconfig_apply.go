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

package controller

import (
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// SelfConfigFieldManager is the SSA field manager name for self-config changes.
// A dedicated manager keeps agent-owned fields separate from GitOps-owned fields
// so neither controller flaps the other's writes.
const SelfConfigFieldManager = "paperclip-selfconfig"

// protectedConfigKeys are config paths that cannot be modified via self-config.
// Modifying these could break authentication, secrets, or storage wiring.
var protectedConfigKeys = map[string]bool{
	"auth":     true,
	"secrets":  true,
	"database": true,
	"gateway":  true,
}

// protectedEnvVars are environment variable names that cannot be added or
// removed via self-config because they are operator-managed.
var protectedEnvVars = map[string]bool{
	"DATABASE_URL":         true,
	"BETTER_AUTH_SECRET":   true,
	"PAPERCLIP_MASTER_KEY": true,
	"NODE_ENV":             true,
	"PORT":                 true,
	"HOME":                 true,
	"PATH":                 true,
	EnvOAuthCredentials:    true,
	EnvOAuthProviders:      true,
}

// EnvOAuthCredentials mirrors resources.EnvOAuthCredentials without importing
// the resources package into the controller's protected-keys table.
const EnvOAuthCredentials = "PAPERCLIP_OAUTH_CREDENTIALS" // #nosec G101 -- env var name, not a credential

// EnvOAuthProviders mirrors resources.EnvOAuthProviders.
const EnvOAuthProviders = "PAPERCLIP_OAUTH_PROVIDERS"

// determineActions inspects which action categories a SelfConfig request uses.
func determineActions(sc *paperclipv1alpha1.PaperclipSelfConfig) []paperclipv1alpha1.SelfConfigAction {
	var actions []paperclipv1alpha1.SelfConfigAction
	if len(sc.Spec.AddPlugins) > 0 || len(sc.Spec.RemovePlugins) > 0 {
		actions = append(actions, paperclipv1alpha1.SelfConfigActionPlugins)
	}
	if sc.Spec.PatchConfig != nil {
		actions = append(actions, paperclipv1alpha1.SelfConfigActionConfig)
	}
	if len(sc.Spec.AddEnvVars) > 0 || len(sc.Spec.RemoveEnvVars) > 0 {
		actions = append(actions, paperclipv1alpha1.SelfConfigActionEnvVars)
	}
	return actions
}

// checkAllowedActions validates that all requested actions are in the allowed
// list. Returns the list of denied action names, or nil if all are allowed.
func checkAllowedActions(requested, allowed []paperclipv1alpha1.SelfConfigAction) []paperclipv1alpha1.SelfConfigAction {
	allowedSet := make(map[paperclipv1alpha1.SelfConfigAction]bool, len(allowed))
	for _, a := range allowed {
		allowedSet[a] = true
	}

	var denied []paperclipv1alpha1.SelfConfigAction
	for _, a := range requested {
		if !allowedSet[a] {
			denied = append(denied, a)
		}
	}
	return denied
}

// buildPluginsApply merges new plugins into the current list (deduplicated by
// name) and filters out removals.
func buildPluginsApply(current []paperclipv1alpha1.PluginRef, sc *paperclipv1alpha1.PaperclipSelfConfig) []paperclipv1alpha1.PluginRef {
	removeSet := make(map[string]bool, len(sc.Spec.RemovePlugins))
	for _, name := range sc.Spec.RemovePlugins {
		removeSet[name] = true
	}

	seen := make(map[string]bool, len(current)+len(sc.Spec.AddPlugins))
	result := make([]paperclipv1alpha1.PluginRef, 0, len(current)+len(sc.Spec.AddPlugins))
	for _, p := range current {
		if removeSet[p.Name] || seen[p.Name] {
			continue
		}
		result = append(result, p)
		seen[p.Name] = true
	}
	for _, p := range sc.Spec.AddPlugins {
		if removeSet[p.Name] || seen[p.Name] {
			continue
		}
		result = append(result, p)
		seen[p.Name] = true
	}
	return result
}

// buildConfigApply deep-merges the config patch into the current config JSON and
// returns the merged raw bytes. Protected top-level keys are rejected.
func buildConfigApply(current []byte, sc *paperclipv1alpha1.PaperclipSelfConfig) ([]byte, error) {
	if sc.Spec.PatchConfig == nil || len(sc.Spec.PatchConfig.Raw) == 0 {
		return nil, nil
	}

	var patch map[string]interface{}
	if err := json.Unmarshal(sc.Spec.PatchConfig.Raw, &patch); err != nil {
		return nil, fmt.Errorf("invalid config patch JSON: %w", err)
	}

	for key := range patch {
		if protectedConfigKeys[key] {
			return nil, fmt.Errorf("config key %q is protected and cannot be modified via self-config", key)
		}
	}

	base := make(map[string]interface{})
	if len(current) > 0 {
		if err := json.Unmarshal(current, &base); err != nil {
			return nil, fmt.Errorf("failed to parse existing config: %w", err)
		}
	}

	merged := deepMerge(base, patch)
	raw, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged config: %w", err)
	}
	return raw, nil
}

// deepMerge recursively merges src into dst. Arrays are replaced, not merged.
func deepMerge(dst, src map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(dst))
	for k, v := range dst {
		result[k] = v
	}
	for k, v := range src {
		if srcMap, ok := v.(map[string]interface{}); ok {
			if dstMap, ok := result[k].(map[string]interface{}); ok {
				result[k] = deepMerge(dstMap, srcMap)
				continue
			}
		}
		result[k] = v
	}
	return result
}

// buildEnvApply validates against protected env vars, then merges adds and
// filters removals against the current env list.
func buildEnvApply(current []corev1.EnvVar, sc *paperclipv1alpha1.PaperclipSelfConfig) ([]corev1.EnvVar, error) {
	for _, ev := range sc.Spec.AddEnvVars {
		if protectedEnvVars[ev.Name] {
			return nil, fmt.Errorf("environment variable %q is protected and cannot be modified via self-config", ev.Name)
		}
	}
	for _, name := range sc.Spec.RemoveEnvVars {
		if protectedEnvVars[name] {
			return nil, fmt.Errorf("environment variable %q is protected and cannot be removed via self-config", name)
		}
	}

	removeSet := make(map[string]bool, len(sc.Spec.RemoveEnvVars))
	for _, name := range sc.Spec.RemoveEnvVars {
		removeSet[name] = true
	}
	addByName := make(map[string]string, len(sc.Spec.AddEnvVars))
	for _, ev := range sc.Spec.AddEnvVars {
		addByName[ev.Name] = ev.Value
	}

	seen := make(map[string]bool, len(current)+len(sc.Spec.AddEnvVars))
	result := make([]corev1.EnvVar, 0, len(current)+len(sc.Spec.AddEnvVars))
	for _, ev := range current {
		if removeSet[ev.Name] {
			continue
		}
		if val, ok := addByName[ev.Name]; ok {
			result = append(result, corev1.EnvVar{Name: ev.Name, Value: val})
			seen[ev.Name] = true
			continue
		}
		result = append(result, ev)
		seen[ev.Name] = true
	}
	for _, ev := range sc.Spec.AddEnvVars {
		if !seen[ev.Name] {
			result = append(result, corev1.EnvVar{Name: ev.Name, Value: ev.Value})
			seen[ev.Name] = true
		}
	}
	return result, nil
}

// buildApplySpec constructs the partial InstanceSpec for an SSA apply based on
// the current instance state and the requested SelfConfig changes.
func buildApplySpec(
	instance *paperclipv1alpha1.Instance,
	sc *paperclipv1alpha1.PaperclipSelfConfig,
	actions []paperclipv1alpha1.SelfConfigAction,
) (*paperclipv1alpha1.InstanceSpec, error) {
	spec := &paperclipv1alpha1.InstanceSpec{}

	for _, action := range actions {
		switch action {
		case paperclipv1alpha1.SelfConfigActionPlugins:
			spec.Plugins = buildPluginsApply(instance.Spec.Plugins, sc)

		case paperclipv1alpha1.SelfConfigActionConfig:
			// Paperclip exposes the merged config patch to the container via the
			// PAPERCLIP_CONFIG_PATCH env var so the app can apply it at boot. We
			// merge into any existing patch carried on that env var.
			var currentRaw []byte
			for _, ev := range instance.Spec.Env {
				if ev.Name == EnvConfigPatch {
					currentRaw = []byte(ev.Value)
					break
				}
			}
			raw, err := buildConfigApply(currentRaw, sc)
			if err != nil {
				return nil, err
			}
			if raw != nil {
				spec.Env = upsertEnvVar(spec.Env, instance.Spec.Env, corev1.EnvVar{
					Name:  EnvConfigPatch,
					Value: string(raw),
				})
			}

		case paperclipv1alpha1.SelfConfigActionEnvVars:
			env, err := buildEnvApply(instance.Spec.Env, sc)
			if err != nil {
				return nil, err
			}
			spec.Env = env
		}
	}

	return spec, nil
}

// EnvConfigPatch is the env var the app reads for agent-applied config patches.
const EnvConfigPatch = "PAPERCLIP_CONFIG_PATCH"

// upsertEnvVar returns the env list with target set/replaced by Name. When the
// caller has already populated spec.Env (e.g. the envVars action ran first) it
// edits that slice; otherwise it seeds from the instance's current env so the
// SSA payload carries the full owned list.
func upsertEnvVar(specEnv, currentEnv []corev1.EnvVar, target corev1.EnvVar) []corev1.EnvVar {
	base := specEnv
	if base == nil {
		base = append([]corev1.EnvVar(nil), currentEnv...)
	}
	for i := range base {
		if base[i].Name == target.Name {
			base[i] = target
			return base
		}
	}
	return append(base, target)
}
