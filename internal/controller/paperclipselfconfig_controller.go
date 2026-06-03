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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// SelfConfigTTL is how long completed requests are kept before auto-deletion.
const SelfConfigTTL = 1 * time.Hour

// PaperclipSelfConfigReconciler reconciles PaperclipSelfConfig objects. It
// validates each request against the parent Instance's selfConfigure allowlist
// and applies approved changes to the Instance via Server-Side Apply using a
// dedicated field manager.
type PaperclipSelfConfigReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=paperclip.inc,resources=paperclipselfconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=paperclip.inc,resources=paperclipselfconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=paperclip.inc,resources=paperclipselfconfigs/finalizers,verbs=update

// Reconcile processes a PaperclipSelfConfig request.
func (r *PaperclipSelfConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	sc := &paperclipv1alpha1.PaperclipSelfConfig{}
	if err := r.Get(ctx, req.NamespacedName, sc); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Terminal phases: check TTL for cleanup.
	if sc.Status.Phase == paperclipv1alpha1.SelfConfigPhaseApplied ||
		sc.Status.Phase == paperclipv1alpha1.SelfConfigPhaseFailed ||
		sc.Status.Phase == paperclipv1alpha1.SelfConfigPhaseDenied {
		if sc.Status.CompletionTime != nil {
			age := time.Since(sc.Status.CompletionTime.Time)
			if age >= SelfConfigTTL {
				log.Info("deleting expired self-config request", "name", sc.Name, "age", age)
				if err := r.Delete(ctx, sc); err != nil && !apierrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			}
			return ctrl.Result{RequeueAfter: SelfConfigTTL - age}, nil
		}
		return ctrl.Result{}, nil
	}

	// Fetch parent instance.
	instance := &paperclipv1alpha1.Instance{}
	if err := r.Get(ctx, types.NamespacedName{Name: sc.Spec.InstanceRef, Namespace: sc.Namespace}, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return r.setTerminalStatus(ctx, sc, paperclipv1alpha1.SelfConfigPhaseFailed,
				fmt.Sprintf("instance %q not found", sc.Spec.InstanceRef))
		}
		return ctrl.Result{}, err
	}

	// Validate self-configure is enabled.
	if !instance.Spec.SelfConfigure.Enabled {
		return r.setTerminalStatus(ctx, sc, paperclipv1alpha1.SelfConfigPhaseDenied,
			"self-configure is not enabled on the target instance")
	}

	// Determine which actions the request uses.
	requestedActions := determineActions(sc)
	if len(requestedActions) == 0 {
		return r.setTerminalStatus(ctx, sc, paperclipv1alpha1.SelfConfigPhaseFailed,
			"request contains no actions")
	}

	// Check against the allowlist.
	denied := checkAllowedActions(requestedActions, instance.Spec.SelfConfigure.AllowedActions)
	if len(denied) > 0 {
		msg := fmt.Sprintf("denied actions: %v", denied)
		if r.Recorder != nil {
			r.Recorder.Event(instance, "Warning", "SelfConfigDenied", msg)
		}
		return r.setTerminalStatus(ctx, sc, paperclipv1alpha1.SelfConfigPhaseDenied, msg)
	}

	// Build the partial spec for the SSA apply.
	applySpec, err := buildApplySpec(instance, sc, requestedActions)
	if err != nil {
		log.Error(err, "failed to build apply spec")
		return r.setTerminalStatus(ctx, sc, paperclipv1alpha1.SelfConfigPhaseFailed,
			fmt.Sprintf("failed to build apply spec: %v", err))
	}

	// Apply changes using Server-Side Apply with a dedicated field manager.
	// Use unstructured to preserve empty slices in the JSON payload, which
	// typed objects with omitempty would drop, so the field manager correctly
	// releases ownership when all items are removed.
	typedObj := &paperclipv1alpha1.Instance{
		TypeMeta: metav1.TypeMeta{
			APIVersion: paperclipv1alpha1.GroupVersion.String(),
			Kind:       "Instance",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: *applySpec,
	}

	rawMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(typedObj)
	if err != nil {
		log.Error(err, "failed to convert apply spec to unstructured")
		return r.setTerminalStatus(ctx, sc, paperclipv1alpha1.SelfConfigPhaseFailed,
			fmt.Sprintf("failed to convert apply spec: %v", err))
	}

	applyObj := &unstructured.Unstructured{Object: rawMap}
	if err := r.Patch(ctx, applyObj,
		client.Apply,
		client.FieldOwner(SelfConfigFieldManager),
		client.ForceOwnership,
	); err != nil {
		log.Error(err, "failed to apply self-config changes via SSA")
		return r.setTerminalStatus(ctx, sc, paperclipv1alpha1.SelfConfigPhaseFailed,
			fmt.Sprintf("failed to apply changes: %v", err))
	}

	// Set owner reference to the parent instance (GC on instance deletion).
	if err := controllerutil.SetOwnerReference(instance, sc, r.Scheme); err != nil {
		log.Error(err, "failed to set owner reference")
	} else if err := r.Update(ctx, sc); err != nil { // reconcile-guard:allow
		log.Error(err, "failed to update owner reference")
	}

	if r.Recorder != nil {
		r.Recorder.Event(sc, "Normal", "Applied", "self-config request applied successfully")
		r.Recorder.Event(instance, "Normal", "SelfConfigApplied",
			fmt.Sprintf("self-config request %q applied", sc.Name))
	}

	return r.setTerminalStatus(ctx, sc, paperclipv1alpha1.SelfConfigPhaseApplied, "changes applied successfully")
}

// setTerminalStatus updates the SelfConfig status to a terminal phase.
func (r *PaperclipSelfConfigReconciler) setTerminalStatus(
	ctx context.Context,
	sc *paperclipv1alpha1.PaperclipSelfConfig,
	phase paperclipv1alpha1.SelfConfigPhase,
	message string,
) (ctrl.Result, error) {
	now := metav1.Now()
	sc.Status.Phase = phase
	sc.Status.Message = message
	sc.Status.ObservedGeneration = sc.Generation
	sc.Status.CompletionTime = &now

	if err := r.Status().Update(ctx, sc); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: SelfConfigTTL}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PaperclipSelfConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&paperclipv1alpha1.PaperclipSelfConfig{}).
		Named("paperclipselfconfig").
		Complete(r)
}
