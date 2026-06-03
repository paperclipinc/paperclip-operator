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

package resources

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

const defaultRunbookBaseURL = "https://paperclip.inc/docs/operators/paperclip/runbooks"

// PrometheusRuleGVK returns the GroupVersionKind for PrometheusRule.
func PrometheusRuleGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "monitoring.coreos.com",
		Version: "v1",
		Kind:    "PrometheusRule",
	}
}

// PrometheusRuleName returns the name of the PrometheusRule.
func PrometheusRuleName(instance *paperclipv1alpha1.Instance) string {
	return instance.Name + "-alerts"
}

// BuildPrometheusRule creates an unstructured PrometheusRule for the Instance.
func BuildPrometheusRule(instance *paperclipv1alpha1.Instance) *unstructured.Unstructured {
	prLabels := make(map[string]string)
	for k, v := range Labels(instance) {
		prLabels[k] = v
	}
	runbookBase := defaultRunbookBaseURL
	if pr := instance.Spec.Observability.Metrics.PrometheusRule; pr != nil {
		for k, v := range pr.Labels {
			prLabels[k] = v
		}
		if pr.RunbookBaseURL != "" {
			runbookBase = pr.RunbookBaseURL
		}
	}

	alerts := buildAlerts(instance.Name, instance.Namespace, runbookBase)

	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "monitoring.coreos.com/v1",
			"kind":       "PrometheusRule",
			"metadata": map[string]interface{}{
				"name":      PrometheusRuleName(instance),
				"namespace": instance.Namespace,
				"labels":    toStringInterfaceMap(prLabels),
			},
			"spec": map[string]interface{}{
				"groups": []interface{}{
					map[string]interface{}{
						"name":  "paperclip-operator",
						"rules": alerts,
					},
				},
			},
		},
	}
}

func buildAlerts(name, ns, runbookBase string) []interface{} {
	// Helper to quote a label value in PromQL.
	q := func(s string) string { return `"` + s + `"` }

	return []interface{}{
		buildAlert(
			"PaperclipReconcileErrors",
			`sum(rate(paperclip_reconcile_total{result="error",instance=`+q(name)+`,namespace=`+q(ns)+`}[5m])) > 0`,
			"5m",
			"warning",
			"Paperclip instance {{ $labels.instance }} in {{ $labels.namespace }} has reconciliation errors.",
			runbookBase,
		),
		buildAlert(
			"PaperclipInstanceNotReady",
			`paperclip_instance_ready{instance=`+q(name)+`,namespace=`+q(ns)+`} == 0`,
			"10m",
			"critical",
			"Paperclip instance {{ $labels.instance }} in {{ $labels.namespace }} has been not-ready for 10m.",
			runbookBase,
		),
		buildAlert(
			"PaperclipInstanceDegraded",
			`paperclip_instance_phase{phase=~"Failed|Degraded",instance=`+q(name)+`,namespace=`+q(ns)+`} == 1`,
			"5m",
			"critical",
			"Paperclip instance {{ $labels.instance }} in {{ $labels.namespace }} is in {{ $labels.phase }} phase.",
			runbookBase,
		),
		buildAlert(
			"PaperclipSlowReconciliation",
			`histogram_quantile(0.99, sum(rate(paperclip_reconcile_duration_seconds_bucket{instance=`+q(name)+`,namespace=`+q(ns)+`}[5m])) by (le)) > 30`,
			"5m",
			"warning",
			"Paperclip instance {{ $labels.instance }} p99 reconciliation duration exceeds 30s.",
			runbookBase,
		),
		buildAlert(
			"PaperclipPodCrashLooping",
			`increase(kube_pod_container_status_restarts_total{namespace=`+q(ns)+`,pod=~`+q(name+"-.*")+`,container="paperclip"}[10m]) > 2`,
			"0m",
			"critical",
			"Paperclip pod {{ $labels.pod }} is crash-looping (>2 restarts in 10m).",
			runbookBase,
		),
		buildAlert(
			"PaperclipPodOOMKilled",
			`kube_pod_container_status_last_terminated_reason{reason="OOMKilled",namespace=`+q(ns)+`,pod=~`+q(name+"-.*")+`,container="paperclip"} == 1`,
			"0m",
			"warning",
			"Paperclip pod {{ $labels.pod }} was OOM killed. Consider increasing memory limits.",
			runbookBase,
		),
		buildAlert(
			"PaperclipPVCNearlyFull",
			`(kubelet_volume_stats_used_bytes{namespace=`+q(ns)+`,persistentvolumeclaim=~`+q(name+"-data.*")+`} / kubelet_volume_stats_capacity_bytes{namespace=`+q(ns)+`,persistentvolumeclaim=~`+q(name+"-data.*")+`}) > 0.80`,
			"5m",
			"warning",
			"PVC for Paperclip instance {{ $labels.persistentvolumeclaim }} is over 80% full.",
			runbookBase,
		),
	}
}

func buildAlert(alertName, expr, forDuration, severity, summary, runbookBase string) map[string]interface{} {
	return map[string]interface{}{
		"alert": alertName,
		"expr":  expr,
		"for":   forDuration,
		"labels": map[string]interface{}{
			"severity": severity,
		},
		"annotations": map[string]interface{}{
			"summary":     summary,
			"runbook_url": fmt.Sprintf("%s/%s", runbookBase, alertName),
		},
	}
}

// toStringInterfaceMap converts a string map to a map[string]interface{} for
// use in unstructured objects.
func toStringInterfaceMap(m map[string]string) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
