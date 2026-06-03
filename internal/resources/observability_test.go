package resources

import (
	"encoding/json"
	"strings"
	"testing"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

func TestBuildPrometheusRule(t *testing.T) {
	instance := newTestInstance("alerts-test")
	instance.Spec.Observability.Metrics.PrometheusRule = &paperclipv1alpha1.PrometheusRuleSpec{
		Enabled: true,
		Labels:  map[string]string{"release": "kube-prometheus-stack"},
	}

	pr := BuildPrometheusRule(instance)
	if pr.GetName() != "alerts-test-alerts" {
		t.Errorf("expected name alerts-test-alerts, got %q", pr.GetName())
	}
	if pr.GetNamespace() != "test-ns" {
		t.Errorf("expected namespace test-ns, got %q", pr.GetNamespace())
	}
	if pr.GetLabels()["release"] != "kube-prometheus-stack" {
		t.Errorf("expected custom label to be merged, got %v", pr.GetLabels())
	}

	spec, ok := pr.Object["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("expected spec map")
	}
	groups, ok := spec["groups"].([]interface{})
	if !ok || len(groups) != 1 {
		t.Fatalf("expected 1 rule group, got %v", spec["groups"])
	}
	group := groups[0].(map[string]interface{})
	rules, ok := group["rules"].([]interface{})
	if !ok || len(rules) == 0 {
		t.Fatal("expected at least one alert rule")
	}

	// Verify expected default alerts exist and reference paperclip_ metrics.
	wantAlerts := map[string]bool{
		"PaperclipReconcileErrors":  false,
		"PaperclipInstanceNotReady": false,
		"PaperclipInstanceDegraded": false,
		"PaperclipPodCrashLooping":  false,
		"PaperclipPVCNearlyFull":    false,
	}
	for _, r := range rules {
		rule := r.(map[string]interface{})
		name, _ := rule["alert"].(string)
		if _, tracked := wantAlerts[name]; tracked {
			wantAlerts[name] = true
		}
		expr, _ := rule["expr"].(string)
		if strings.Contains(expr, "openclaw_") || strings.Contains(expr, "hermes_") {
			t.Errorf("alert %q expr references a sibling operator metric: %s", name, expr)
		}
	}
	for name, found := range wantAlerts {
		if !found {
			t.Errorf("expected alert %q to be present", name)
		}
	}
}

func TestBuildGrafanaDashboards(t *testing.T) {
	instance := newTestInstance("dash-test")
	instance.Spec.Observability.Metrics.GrafanaDashboard = &paperclipv1alpha1.GrafanaDashboardSpec{
		Enabled: true,
		Folder:  "Custom",
		Labels:  map[string]string{"team": "platform"},
	}

	for _, cm := range []struct {
		name string
		obj  func() (string, map[string]string, map[string]string)
	}{
		{"operator", func() (string, map[string]string, map[string]string) {
			c := BuildGrafanaDashboardOperator(instance)
			return c.Name, c.Labels, c.Annotations
		}},
		{"instance", func() (string, map[string]string, map[string]string) {
			c := BuildGrafanaDashboardInstance(instance)
			return c.Name, c.Labels, c.Annotations
		}},
	} {
		name, labels, annotations := cm.obj()
		if name == "" {
			t.Fatalf("%s: expected a ConfigMap name", cm.name)
		}
		if labels["grafana_dashboard"] != "1" {
			t.Errorf("%s: expected grafana_dashboard=1 label", cm.name)
		}
		if labels["team"] != "platform" {
			t.Errorf("%s: expected custom label team=platform", cm.name)
		}
		if annotations["grafana_folder"] != "Custom" {
			t.Errorf("%s: expected grafana_folder=Custom, got %q", cm.name, annotations["grafana_folder"])
		}
	}

	// Verify the operator dashboard JSON is valid and references paperclip metrics.
	op := BuildGrafanaDashboardOperator(instance)
	data := op.Data["paperclip-operator.json"]
	if data == "" {
		t.Fatal("expected operator dashboard JSON")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		t.Fatalf("operator dashboard JSON is invalid: %v", err)
	}
	if !strings.Contains(data, "paperclip_reconcile_total") {
		t.Error("expected operator dashboard to reference paperclip_reconcile_total")
	}
	if strings.Contains(data, "openclaw_") {
		t.Error("operator dashboard references openclaw_ metrics")
	}
}
