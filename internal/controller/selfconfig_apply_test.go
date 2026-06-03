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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

func TestDetermineActions(t *testing.T) {
	sc := &paperclipv1alpha1.PaperclipSelfConfig{
		Spec: paperclipv1alpha1.PaperclipSelfConfigSpec{
			AddPlugins: []paperclipv1alpha1.PluginRef{{Name: "p"}},
			AddEnvVars: []paperclipv1alpha1.SelfConfigEnvVar{{Name: "X", Value: "1"}},
		},
	}
	got := determineActions(sc)
	want := []paperclipv1alpha1.SelfConfigAction{
		paperclipv1alpha1.SelfConfigActionPlugins,
		paperclipv1alpha1.SelfConfigActionEnvVars,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("determineActions = %v, want %v", got, want)
	}
}

func TestCheckAllowedActions(t *testing.T) {
	requested := []paperclipv1alpha1.SelfConfigAction{
		paperclipv1alpha1.SelfConfigActionPlugins,
		paperclipv1alpha1.SelfConfigActionConfig,
	}
	allowed := []paperclipv1alpha1.SelfConfigAction{paperclipv1alpha1.SelfConfigActionPlugins}
	denied := checkAllowedActions(requested, allowed)
	if len(denied) != 1 || denied[0] != paperclipv1alpha1.SelfConfigActionConfig {
		t.Errorf("denied = %v", denied)
	}
}

func TestBuildPluginsApply(t *testing.T) {
	current := []paperclipv1alpha1.PluginRef{{Name: "keep"}, {Name: "drop"}}
	sc := &paperclipv1alpha1.PaperclipSelfConfig{
		Spec: paperclipv1alpha1.PaperclipSelfConfigSpec{
			AddPlugins:    []paperclipv1alpha1.PluginRef{{Name: "new"}, {Name: "keep"}},
			RemovePlugins: []string{"drop"},
		},
	}
	got := buildPluginsApply(current, sc)
	names := make([]string, 0, len(got))
	for _, p := range got {
		names = append(names, p.Name)
	}
	want := []string{"keep", "new"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("plugins = %v, want %v", names, want)
	}
}

func TestBuildEnvApply_ProtectedRejected(t *testing.T) {
	sc := &paperclipv1alpha1.PaperclipSelfConfig{
		Spec: paperclipv1alpha1.PaperclipSelfConfigSpec{
			AddEnvVars: []paperclipv1alpha1.SelfConfigEnvVar{{Name: "DATABASE_URL", Value: "x"}},
		},
	}
	if _, err := buildEnvApply(nil, sc); err == nil {
		t.Fatal("expected error for protected env var")
	}
}

func TestBuildEnvApply_MergeAndRemove(t *testing.T) {
	current := []corev1.EnvVar{
		{Name: "KEEP", Value: "1"},
		{Name: "OVERRIDE", Value: "old"},
		{Name: "DROP", Value: "x"},
	}
	sc := &paperclipv1alpha1.PaperclipSelfConfig{
		Spec: paperclipv1alpha1.PaperclipSelfConfigSpec{
			AddEnvVars: []paperclipv1alpha1.SelfConfigEnvVar{
				{Name: "OVERRIDE", Value: "new"},
				{Name: "ADDED", Value: "y"},
			},
			RemoveEnvVars: []string{"DROP"},
		},
	}
	got, err := buildEnvApply(current, sc)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]string{}
	for _, e := range got {
		byName[e.Name] = e.Value
	}
	if byName["KEEP"] != "1" || byName["OVERRIDE"] != "new" || byName["ADDED"] != "y" {
		t.Errorf("env result = %v", byName)
	}
	if _, ok := byName["DROP"]; ok {
		t.Error("DROP should have been removed")
	}
}

func TestBuildConfigApply_DeepMergeAndProtected(t *testing.T) {
	current := []byte(`{"feature":{"a":1},"keep":true}`)
	sc := &paperclipv1alpha1.PaperclipSelfConfig{
		Spec: paperclipv1alpha1.PaperclipSelfConfigSpec{
			PatchConfig: &apiextensionsv1.JSON{Raw: []byte(`{"feature":{"b":2}}`)},
		},
	}
	raw, err := buildConfigApply(current, sc)
	if err != nil {
		t.Fatal(err)
	}
	var merged map[string]interface{}
	if err := json.Unmarshal(raw, &merged); err != nil {
		t.Fatal(err)
	}
	feature := merged["feature"].(map[string]interface{})
	if feature["a"].(float64) != 1 || feature["b"].(float64) != 2 {
		t.Errorf("deep merge failed: %v", feature)
	}
	if merged["keep"] != true {
		t.Error("existing keys should be preserved")
	}

	// Protected key is rejected.
	sc.Spec.PatchConfig = &apiextensionsv1.JSON{Raw: []byte(`{"auth":{"x":1}}`)}
	if _, err := buildConfigApply(current, sc); err == nil {
		t.Fatal("expected error for protected config key")
	}
}

func TestBuildApplySpec_Plugins(t *testing.T) {
	inst := &paperclipv1alpha1.Instance{
		Spec: paperclipv1alpha1.InstanceSpec{
			Plugins: []paperclipv1alpha1.PluginRef{{Name: "existing"}},
		},
	}
	sc := &paperclipv1alpha1.PaperclipSelfConfig{
		Spec: paperclipv1alpha1.PaperclipSelfConfigSpec{
			AddPlugins: []paperclipv1alpha1.PluginRef{{Name: "new"}},
		},
	}
	spec, err := buildApplySpec(inst, sc, determineActions(sc))
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.Plugins) != 2 {
		t.Errorf("expected 2 plugins, got %d", len(spec.Plugins))
	}
}
