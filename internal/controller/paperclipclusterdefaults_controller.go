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
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// ConditionClusterDefaultsReady is the condition type reported on a
// PaperclipClusterDefaults singleton.
const ConditionClusterDefaultsReady = "Ready"

// PaperclipClusterDefaultsReconciler enforces the singleton invariant on the
// cluster-scoped PaperclipClusterDefaults resource and surfaces Ready status.
// The actual defaulting happens in-memory inside the Instance reconciler; this
// controller only validates the singleton name and reports status.
type PaperclipClusterDefaultsReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=paperclip.inc,resources=paperclipclusterdefaults,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=paperclip.inc,resources=paperclipclusterdefaults/status,verbs=get;update;patch

// Reconcile validates the singleton name and reports a Ready condition. A
// PaperclipClusterDefaults under any name other than "cluster" is reported as
// Invalid and ignored by the Instance reconciler.
func (r *PaperclipClusterDefaultsReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	pccd := &paperclipv1alpha1.PaperclipClusterDefaults{}
	if err := r.Get(ctx, req.NamespacedName, pccd); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	cond := metav1.Condition{
		Type:               ConditionClusterDefaultsReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Singleton",
		Message:            "PaperclipClusterDefaults singleton is healthy and applied to Instances",
		ObservedGeneration: pccd.Generation,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}

	if pccd.Name != paperclipv1alpha1.ClusterDefaultsSingletonName {
		cond.Status = metav1.ConditionFalse
		cond.Reason = "InvalidName"
		cond.Message = fmt.Sprintf("name must be %q (got %q); this resource is ignored by the operator",
			paperclipv1alpha1.ClusterDefaultsSingletonName, pccd.Name)
		if r.Recorder != nil {
			r.Recorder.Event(pccd, "Warning", "InvalidName", cond.Message)
		}
	}

	meta.SetStatusCondition(&pccd.Status.Conditions, cond)
	pccd.Status.ObservedGeneration = pccd.Generation
	if err := r.Status().Update(ctx, pccd); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status: %w", err)
	}
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// SetupWithManager wires the controller.
func (r *PaperclipClusterDefaultsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&paperclipv1alpha1.PaperclipClusterDefaults{}).
		Named("paperclipclusterdefaults").
		Complete(r)
}
