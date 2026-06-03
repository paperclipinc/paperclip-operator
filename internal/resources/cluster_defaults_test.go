package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

func TestApplyClusterDefaults_NilDefaultsReturnsCopy(t *testing.T) {
	inst := newTestInstance("pc")
	out := ApplyClusterDefaults(inst, nil)
	if out == inst {
		t.Fatal("expected a deep copy, got the same pointer")
	}
	if out.Spec.Image.Tag != inst.Spec.Image.Tag {
		t.Fatalf("expected unchanged tag, got %q", out.Spec.Image.Tag)
	}
}

func TestApplyClusterDefaults_FillsUnsetScalars(t *testing.T) {
	inst := newTestInstance("pc")
	// Clear fields the defaults should fill.
	inst.Spec.Image.Repository = ""
	inst.Spec.Image.Tag = ""
	inst.Spec.Database.Mode = ""
	inst.Spec.Storage.Persistence.StorageClass = nil
	inst.Spec.Observability.Logging.Level = ""
	inst.Spec.Networking.Service.Type = ""

	defaults := &paperclipv1alpha1.PaperclipClusterDefaults{
		Spec: paperclipv1alpha1.PaperclipClusterDefaultsSpec{
			Image: paperclipv1alpha1.ImageSpec{
				Repository: "mirror.example.com/paperclip",
				Tag:        "2026.0403",
			},
			StorageClass: "fast-ssd",
			DatabaseMode: "managed",
			Observability: paperclipv1alpha1.ObservabilitySpec{
				Metrics: paperclipv1alpha1.MetricsSpec{Enabled: true},
				Logging: paperclipv1alpha1.LoggingSpec{Level: "debug"},
			},
			Networking: paperclipv1alpha1.NetworkingSpec{
				Service: paperclipv1alpha1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
			},
		},
	}

	out := ApplyClusterDefaults(inst, defaults)

	if out.Spec.Image.Repository != "mirror.example.com/paperclip" {
		t.Errorf("repository = %q", out.Spec.Image.Repository)
	}
	if out.Spec.Image.Tag != "2026.0403" {
		t.Errorf("tag = %q", out.Spec.Image.Tag)
	}
	if out.Spec.Database.Mode != "managed" {
		t.Errorf("database mode = %q", out.Spec.Database.Mode)
	}
	if out.Spec.Storage.Persistence.StorageClass == nil || *out.Spec.Storage.Persistence.StorageClass != "fast-ssd" {
		t.Errorf("storage class = %v", out.Spec.Storage.Persistence.StorageClass)
	}
	if out.Spec.Database.Managed.StorageClass == nil || *out.Spec.Database.Managed.StorageClass != "fast-ssd" {
		t.Errorf("db storage class = %v", out.Spec.Database.Managed.StorageClass)
	}
	if !out.Spec.Observability.Metrics.Enabled {
		t.Error("metrics should be enabled by default")
	}
	if out.Spec.Observability.Logging.Level != "debug" {
		t.Errorf("log level = %q", out.Spec.Observability.Logging.Level)
	}
	if out.Spec.Networking.Service.Type != corev1.ServiceTypeLoadBalancer {
		t.Errorf("service type = %q", out.Spec.Networking.Service.Type)
	}
}

func TestApplyClusterDefaults_InstanceWins(t *testing.T) {
	inst := newTestInstance("pc")
	inst.Spec.Image.Repository = "my-repo/paperclip"
	inst.Spec.Image.Tag = "v9.9.9"
	inst.Spec.Database.Mode = "external"

	defaults := &paperclipv1alpha1.PaperclipClusterDefaults{
		Spec: paperclipv1alpha1.PaperclipClusterDefaultsSpec{
			Image:        paperclipv1alpha1.ImageSpec{Repository: "mirror/paperclip", Tag: "1.0.0"},
			DatabaseMode: "managed",
		},
	}

	out := ApplyClusterDefaults(inst, defaults)
	if out.Spec.Image.Repository != "my-repo/paperclip" {
		t.Errorf("instance repository should win, got %q", out.Spec.Image.Repository)
	}
	if out.Spec.Image.Tag != "v9.9.9" {
		t.Errorf("instance tag should win, got %q", out.Spec.Image.Tag)
	}
	if out.Spec.Database.Mode != "external" {
		t.Errorf("instance database mode should win, got %q", out.Spec.Database.Mode)
	}
	// The original instance must not be mutated.
	if inst.Spec.Image.Repository != "my-repo/paperclip" {
		t.Error("original instance was mutated")
	}
}

func TestApplyClusterDefaults_DigestPreventsTagInheritance(t *testing.T) {
	inst := newTestInstance("pc")
	inst.Spec.Image.Tag = ""
	inst.Spec.Image.Digest = "sha256:abc"

	defaults := &paperclipv1alpha1.PaperclipClusterDefaults{
		Spec: paperclipv1alpha1.PaperclipClusterDefaultsSpec{
			Image: paperclipv1alpha1.ImageSpec{Tag: "1.0.0"},
		},
	}

	out := ApplyClusterDefaults(inst, defaults)
	if out.Spec.Image.Tag != "" {
		t.Errorf("tag should not be inherited when a digest is pinned, got %q", out.Spec.Image.Tag)
	}
}

func TestApplyClusterDefaults_MergeEnvByName(t *testing.T) {
	inst := newTestInstance("pc")
	inst.Spec.Env = []corev1.EnvVar{
		{Name: "NPM_CONFIG_REGISTRY", Value: "https://instance.example"},
		{Name: "INSTANCE_ONLY", Value: "x"},
	}

	defaults := &paperclipv1alpha1.PaperclipClusterDefaults{
		Spec: paperclipv1alpha1.PaperclipClusterDefaultsSpec{
			Env: []corev1.EnvVar{
				{Name: "NPM_CONFIG_REGISTRY", Value: "https://default.example"},
				{Name: "DEFAULT_ONLY", Value: "y"},
			},
		},
	}

	out := ApplyClusterDefaults(inst, defaults)
	byName := map[string]string{}
	for _, e := range out.Spec.Env {
		byName[e.Name] = e.Value
	}
	if byName["NPM_CONFIG_REGISTRY"] != "https://instance.example" {
		t.Errorf("instance env should override default, got %q", byName["NPM_CONFIG_REGISTRY"])
	}
	if byName["DEFAULT_ONLY"] != "y" {
		t.Errorf("default-only env missing, got %q", byName["DEFAULT_ONLY"])
	}
	if byName["INSTANCE_ONLY"] != "x" {
		t.Errorf("instance-only env missing, got %q", byName["INSTANCE_ONLY"])
	}
	// Defaults appear first.
	if out.Spec.Env[0].Name != "NPM_CONFIG_REGISTRY" {
		t.Errorf("expected default name first, got %q", out.Spec.Env[0].Name)
	}
}

func TestApplyClusterDefaults_RedisStorageClass(t *testing.T) {
	inst := newTestInstance("pc")
	inst.Spec.Redis = &paperclipv1alpha1.RedisSpec{Mode: "managed"}

	defaults := &paperclipv1alpha1.PaperclipClusterDefaults{
		ObjectMeta: metav1.ObjectMeta{Name: paperclipv1alpha1.ClusterDefaultsSingletonName},
		Spec:       paperclipv1alpha1.PaperclipClusterDefaultsSpec{StorageClass: "fast-ssd"},
	}

	out := ApplyClusterDefaults(inst, defaults)
	if out.Spec.Redis.Managed.StorageClass == nil || *out.Spec.Redis.Managed.StorageClass != "fast-ssd" {
		t.Errorf("redis storage class = %v", out.Spec.Redis.Managed.StorageClass)
	}
}
