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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
	"github.com/paperclipinc/paperclip-operator/internal/resources"
)

const (
	// LabelPodRole marks server pods with their scheduler role so traffic
	// tooling and humans can tell the lease holder apart from web replicas.
	LabelPodRole = "paperclip.inc/role"
	// PodRoleScheduler is the LabelPodRole value for the lease-holding pod.
	PodRoleScheduler = "scheduler"
	// PodRoleWeb is the LabelPodRole value for non-leader server pods.
	PodRoleWeb = "web"

	// AnnotationPodDeletionCost is the upstream pod-deletion-cost annotation.
	// The ReplicaSet controller scales down pods with lower cost first, so
	// tagging the scheduler leader with a high cost makes scale-down prefer
	// web replicas and avoids an unnecessary leader failover.
	AnnotationPodDeletionCost = "controller.kubernetes.io/pod-deletion-cost"
	// PodDeletionCostLeader is the deletion cost applied to the leader pod.
	PodDeletionCostLeader = "10000"

	// leaderVisibilityRequeueInterval is how often the operator re-polls the
	// pods' /api/health endpoints while lease gating is active.
	leaderVisibilityRequeueInterval = 30 * time.Second

	// healthProbeTimeout bounds each /api/health request.
	healthProbeTimeout = 2 * time.Second

	// healthBodyLimit bounds how much of the health response body is read.
	healthBodyLimit = 1 << 20
)

// schedulerHealth is the subset of the app's unauthenticated /api/health JSON
// response the operator cares about. A missing scheduler block decodes to the
// zero value, which simply means "not a candidate, not the leader".
type schedulerHealth struct {
	Scheduler struct {
		Candidate bool `json:"candidate"`
		IsLeader  bool `json:"isLeader"`
	} `json:"scheduler"`
}

// probeSchedulerHealth resolves the health probe implementation: the injected
// test fake when set, otherwise the real HTTP poll.
func (r *InstanceReconciler) probeSchedulerHealth(ctx context.Context, podIP string, port int32, hostHeader string) (schedulerHealth, error) {
	if r.healthProbe != nil {
		return r.healthProbe(podIP, port)
	}
	return r.httpHealthProbe(ctx, podIP, port, hostHeader)
}

// httpHealthProbe performs the real GET http://<podIP>:<port>/api/health and
// parses the scheduler block. The shared client is constructed once.
func (r *InstanceReconciler) httpHealthProbe(ctx context.Context, podIP string, port int32, hostHeader string) (schedulerHealth, error) {
	var health schedulerHealth

	r.healthClientOnce.Do(func() {
		r.healthClient = &http.Client{Timeout: healthProbeTimeout}
	})

	url := fmt.Sprintf("http://%s%s", net.JoinHostPort(podIP, strconv.Itoa(int(port))), resources.HealthPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return health, fmt.Errorf("building health request for %s: %w", url, err)
	}
	// The app enforces a Host-header allowlist (service DNS + loopback);
	// a pod-IP Host is rejected with 403, so present the service FQDN
	// while still dialing the specific pod's IP.
	req.Host = hostHeader
	resp, err := r.healthClient.Do(req)
	if err != nil {
		return health, fmt.Errorf("polling %s: %w", url, err)
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort close on a read-only response

	if resp.StatusCode != http.StatusOK {
		return health, fmt.Errorf("polling %s: unexpected status %d", url, resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, healthBodyLimit)).Decode(&health); err != nil {
		return health, fmt.Errorf("decoding health response from %s: %w", url, err)
	}
	return health, nil
}

// reconcileLeaderVisibility discovers which server pod holds the scheduler
// lease and surfaces it: pod role labels (scheduler/web), the leader's
// pod-deletion-cost annotation (Deployment workloads only), and
// status.SchedulerLeader. It is active only while lease gating is configured
// at replicas > 1; otherwise it strips everything it previously set.
//
// The returned result carries the polling requeue interval when active. The
// function never fails the reconcile: pod poll errors skip the pod, and
// list/patch errors are logged.
func (r *InstanceReconciler) reconcileLeaderVisibility(ctx context.Context, instance *paperclipv1alpha1.Instance) ctrl.Result {
	log := logf.FromContext(ctx)

	pods := &corev1.PodList{}
	if err := r.List(ctx, pods,
		client.InNamespace(instance.Namespace),
		client.MatchingLabels(resources.SelectorLabels(instance))); err != nil {
		log.Error(err, "Failed to list pods for scheduler leader visibility")
		return ctrl.Result{}
	}

	active := resources.SchedulerGatingMode(instance) == "lease" && resources.EffectiveReplicas(instance) > 1
	if !active {
		r.clearLeaderVisibility(ctx, instance, pods)
		return ctrl.Result{}
	}

	// Poll every running pod's unauthenticated /api/health for the scheduler
	// block. Per-pod errors skip the pod so a single crashed replica never
	// blocks leader discovery.
	port := resources.ServerPort(instance)
	leaderName := ""
	polled := make(map[string]bool, len(pods.Items))
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" || !pod.DeletionTimestamp.IsZero() {
			continue
		}
		health, err := r.probeSchedulerHealth(ctx, pod.Status.PodIP, port,
			fmt.Sprintf("%s.%s.svc.cluster.local", resources.ServiceName(instance), instance.Namespace))
		if err != nil {
			log.V(1).Info("Skipping pod for scheduler leader visibility: health poll failed",
				"pod", pod.Name, "reason", err.Error())
			continue
		}
		polled[pod.Name] = true
		if health.Scheduler.IsLeader {
			leaderName = pod.Name
		}
	}

	// Label the polled pods with their role and maintain the deletion-cost
	// annotation. Deletion cost is ReplicaSet-only semantics, so it is applied
	// solely when the active workload is a Deployment (and stripped otherwise,
	// e.g. after a Deployment -> StatefulSet migration).
	isDeployment := resources.EffectiveWorkloadIsDeployment(instance)
	for i := range pods.Items {
		pod := &pods.Items[i]
		if !polled[pod.Name] {
			continue
		}
		role := PodRoleWeb
		if pod.Name == leaderName {
			role = PodRoleScheduler
		}
		wantDeletionCost := isDeployment && pod.Name == leaderName
		r.patchPodLeaderMarkers(ctx, pod, role, wantDeletionCost)
	}

	// Record the leader without thrashing: when every poll failed this pass
	// the previous value is kept (we learned nothing), and the field is only
	// cleared once polls succeeded and none of the pods leads.
	if leaderName != "" {
		instance.Status.SchedulerLeader = leaderName
	} else if len(polled) > 0 {
		instance.Status.SchedulerLeader = ""
	}

	return ctrl.Result{RequeueAfter: leaderVisibilityRequeueInterval}
}

// clearLeaderVisibility removes the role labels and deletion-cost annotations
// the operator previously set and clears status.SchedulerLeader. It runs when
// lease gating is not active (ordinal gating or replicas <= 1).
func (r *InstanceReconciler) clearLeaderVisibility(ctx context.Context, instance *paperclipv1alpha1.Instance, pods *corev1.PodList) {
	instance.Status.SchedulerLeader = ""
	for i := range pods.Items {
		pod := &pods.Items[i]
		_, hasRole := pod.Labels[LabelPodRole]
		_, hasCost := pod.Annotations[AnnotationPodDeletionCost]
		if !hasRole && !hasCost {
			continue
		}
		r.patchPodLeaderMarkers(ctx, pod, "", false)
	}
}

// patchPodLeaderMarkers reconciles the role label and deletion-cost annotation
// on a single pod with a merge patch, patching only when something changed.
// An empty role removes the label.
func (r *InstanceReconciler) patchPodLeaderMarkers(ctx context.Context, pod *corev1.Pod, role string, wantDeletionCost bool) {
	base := pod.DeepCopy()
	changed := false

	if role == "" {
		if _, ok := pod.Labels[LabelPodRole]; ok {
			delete(pod.Labels, LabelPodRole)
			changed = true
		}
	} else if pod.Labels[LabelPodRole] != role {
		if pod.Labels == nil {
			pod.Labels = map[string]string{}
		}
		pod.Labels[LabelPodRole] = role
		changed = true
	}

	if wantDeletionCost {
		if pod.Annotations[AnnotationPodDeletionCost] != PodDeletionCostLeader {
			if pod.Annotations == nil {
				pod.Annotations = map[string]string{}
			}
			pod.Annotations[AnnotationPodDeletionCost] = PodDeletionCostLeader
			changed = true
		}
	} else if _, ok := pod.Annotations[AnnotationPodDeletionCost]; ok {
		delete(pod.Annotations, AnnotationPodDeletionCost)
		changed = true
	}

	if !changed {
		return
	}
	if err := r.Patch(ctx, pod, client.MergeFrom(base)); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to patch pod leader-visibility markers", "pod", pod.Name)
	}
}
