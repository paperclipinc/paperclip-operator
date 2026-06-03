package resources

import (
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

func TestGetTailscaleImage(t *testing.T) {
	inst := newTestInstance("pc")
	if got := GetTailscaleImage(inst); got != DefaultTailscaleImage+":"+DefaultTailscaleTag {
		t.Errorf("default image = %q", got)
	}

	inst.Spec.Tailscale.Image.Repository = "example.com/ts"
	inst.Spec.Tailscale.Image.Tag = "v1.2.3"
	if got := GetTailscaleImage(inst); got != "example.com/ts:v1.2.3" {
		t.Errorf("custom image = %q", got)
	}

	inst.Spec.Tailscale.Image.Digest = "sha256:deadbeef"
	if got := GetTailscaleImage(inst); got != "example.com/ts@sha256:deadbeef" {
		t.Errorf("digest image = %q", got)
	}
}

func TestTailscaleHostnameDefaultsToInstanceName(t *testing.T) {
	inst := newTestInstance("my-pc")
	if got := TailscaleHostname(inst); got != "my-pc" {
		t.Errorf("hostname = %q", got)
	}
	inst.Spec.Tailscale.Hostname = "custom"
	if got := TailscaleHostname(inst); got != "custom" {
		t.Errorf("hostname = %q", got)
	}
}

func TestBuildTailscaleServeConfig_ServeMode(t *testing.T) {
	inst := newTestInstance("pc")
	inst.Spec.Tailscale.Mode = TailscaleModeServe

	raw := BuildTailscaleServeConfig(inst)
	var cfg tailscaleServeConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("invalid serve config JSON: %v", err)
	}
	if cfg.TCP["443"] == nil || !cfg.TCP["443"].HTTPS {
		t.Error("expected HTTPS on 443")
	}
	web := cfg.Web["${TS_CERT_DOMAIN}:443"]
	if web == nil || web.Handlers["/"] == nil {
		t.Fatal("expected web handler for /")
	}
	if !strings.Contains(web.Handlers["/"].Proxy, "3100") {
		t.Errorf("proxy should target app port 3100, got %q", web.Handlers["/"].Proxy)
	}
	if len(cfg.AllowFunnel) != 0 {
		t.Error("serve mode should not enable funnel")
	}
}

func TestBuildTailscaleServeConfig_FunnelMode(t *testing.T) {
	inst := newTestInstance("pc")
	inst.Spec.Tailscale.Mode = TailscaleModeFunnel

	raw := BuildTailscaleServeConfig(inst)
	var cfg tailscaleServeConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatalf("invalid serve config JSON: %v", err)
	}
	if !cfg.AllowFunnel["${TS_CERT_DOMAIN}:443"] {
		t.Error("funnel mode should enable AllowFunnel")
	}
}

func TestBuildTailscaleContainer(t *testing.T) {
	inst := newTestInstance("pc")
	inst.Spec.Tailscale.Enabled = true
	inst.Spec.Tailscale.AuthKey = &paperclipv1alpha1.TailscaleAuthKeySpec{
		SecretRef: corev1.LocalObjectReference{Name: "ts-secret"},
	}

	c := BuildTailscaleContainer(inst)
	if c.Name != TailscaleContainerName {
		t.Errorf("name = %q", c.Name)
	}

	env := map[string]corev1.EnvVar{}
	for _, e := range c.Env {
		env[e.Name] = e
	}
	if env["TS_USERSPACE"].Value != "true" {
		t.Error("expected userspace mode")
	}
	if !strings.Contains(env["TS_EXTRA_ARGS"].Value, "--ephemeral") {
		t.Errorf("expected ephemeral node, got %q", env["TS_EXTRA_ARGS"].Value)
	}
	ak := env["TS_AUTHKEY"]
	if ak.ValueFrom == nil || ak.ValueFrom.SecretKeyRef == nil {
		t.Fatal("expected TS_AUTHKEY from secret")
	}
	if ak.ValueFrom.SecretKeyRef.Name != "ts-secret" || ak.ValueFrom.SecretKeyRef.Key != DefaultTailscaleAuthKeyKey {
		t.Errorf("auth key ref = %+v", ak.ValueFrom.SecretKeyRef)
	}
	if c.SecurityContext == nil || c.SecurityContext.ReadOnlyRootFilesystem == nil || !*c.SecurityContext.ReadOnlyRootFilesystem {
		t.Error("expected read-only root filesystem")
	}
}

func TestBuildTailscaleConfigMapAndVolumes(t *testing.T) {
	inst := newTestInstance("pc")
	cm := BuildTailscaleConfigMap(inst)
	if cm.Name != TailscaleConfigMapName(inst) {
		t.Errorf("configmap name = %q", cm.Name)
	}
	if _, ok := cm.Data[TailscaleServeConfigKey]; !ok {
		t.Error("configmap missing serve config key")
	}

	vols := TailscaleVolumes(inst)
	if len(vols) != 3 {
		t.Fatalf("expected 3 volumes, got %d", len(vols))
	}
	var hasConfig bool
	for _, v := range vols {
		if v.ConfigMap != nil && v.ConfigMap.Name == TailscaleConfigMapName(inst) {
			hasConfig = true
		}
	}
	if !hasConfig {
		t.Error("expected a configmap volume referencing the serve config")
	}
}
