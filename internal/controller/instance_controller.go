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
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	paperclipv1alpha1 "github.com/stubbi/paperclip-operator/api/v1alpha1"
	"github.com/stubbi/paperclip-operator/internal/resources"
)

const (
	// FinalizerName is the finalizer added to Instances.
	FinalizerName = "paperclip.ai/finalizer"

	// ConditionReady indicates the instance is fully operational.
	ConditionReady = "Ready"
	// ConditionDatabaseReady indicates the database is ready.
	ConditionDatabaseReady = "DatabaseReady"
	// ConditionStatefulSetReady indicates the StatefulSet is ready.
	ConditionStatefulSetReady = "StatefulSetReady"
	// ConditionServiceReady indicates the Service is ready.
	ConditionServiceReady = "ServiceReady"
)

// InstanceReconciler reconciles a Instance object.
type InstanceReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

// +kubebuilder:rbac:groups=paperclip.inc,resources=instances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=paperclip.inc,resources=instances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=paperclip.inc,resources=instances/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete

// Reconcile moves the cluster state toward the desired state defined by the Instance CR.
//
//nolint:gocyclo // reconciliation loop is inherently complex
func (r *InstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	start := time.Now()

	// Fetch the Instance
	instance := &paperclipv1alpha1.Instance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Record metrics at the end of reconciliation
	defer func() {
		reconcileDuration.WithLabelValues(instance.Name, instance.Namespace).Observe(time.Since(start).Seconds())
		// Update phase metric
		for _, phase := range []string{"Pending", "Provisioning", "Running", "Degraded", "Failed", "Terminating"} {
			val := float64(0)
			if string(instance.Status.Phase) == phase {
				val = 1
			}
			instancePhase.WithLabelValues(instance.Name, instance.Namespace, phase).Set(val)
		}
		// Update info metric
		image := instance.Spec.Image.Repository + ":" + instance.Spec.Image.Tag
		instanceInfo.WithLabelValues(instance.Name, instance.Namespace, instance.Spec.Image.Tag, image).Set(1)
		// Update ready metric
		ready := float64(0)
		for _, cond := range instance.Status.Conditions {
			if cond.Type == ConditionReady && cond.Status == metav1.ConditionTrue {
				ready = 1
			}
		}
		instanceReady.WithLabelValues(instance.Name, instance.Namespace).Set(ready)
	}()

	// Handle deletion
	if !instance.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(instance, FinalizerName) {
			log.Info("Handling finalizer cleanup")
			r.setPhase(ctx, instance, paperclipv1alpha1.PhaseTerminating)
			controllerutil.RemoveFinalizer(instance, FinalizerName)
			if err := r.Update(ctx, instance); err != nil { // reconcile-guard:allow
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(instance, FinalizerName) {
		controllerutil.AddFinalizer(instance, FinalizerName)
		if err := r.Update(ctx, instance); err != nil { // reconcile-guard:allow
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Set initial phase
	if instance.Status.Phase == "" {
		r.setPhase(ctx, instance, paperclipv1alpha1.PhasePending)
	}

	// Reconcile all resources
	r.setPhase(ctx, instance, paperclipv1alpha1.PhaseProvisioning)

	// 1. ServiceAccount
	if instance.Spec.Security.RBAC.Create {
		if err := r.reconcileServiceAccount(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "ServiceAccount", err)
		}
	}

	// 2. Database (if managed)
	if instance.Spec.Database.Mode == "managed" || instance.Spec.Database.Mode == "" {
		if err := r.reconcileManagedDatabase(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "Database", err)
		}
	}

	// 3. PVC (if persistence enabled)
	if instance.Spec.Storage.Persistence.Enabled {
		if err := r.reconcilePVC(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "PVC", err)
		}
	}

	// 4. StatefulSet
	if err := r.reconcileStatefulSet(ctx, instance); err != nil {
		return r.handleError(ctx, instance, "StatefulSet", err)
	}

	// 5. Service
	if err := r.reconcileService(ctx, instance); err != nil {
		return r.handleError(ctx, instance, "Service", err)
	}

	// 6. Ingress (optional)
	if instance.Spec.Networking.Ingress != nil && instance.Spec.Networking.Ingress.Enabled {
		if err := r.reconcileIngress(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "Ingress", err)
		}
	}

	// 7. NetworkPolicy (optional)
	if instance.Spec.Security.NetworkPolicy.Enabled {
		if err := r.reconcileNetworkPolicy(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "NetworkPolicy", err)
		}
	}

	// 8. HPA (optional)
	if as := instance.Spec.Availability.AutoScaling; as != nil && as.Enabled {
		if err := r.reconcileHPA(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "HPA", err)
		}
	}

	// 9. PDB (optional)
	if pdb := instance.Spec.Availability.PodDisruptionBudget; pdb != nil && pdb.Enabled {
		if err := r.reconcilePDB(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "PDB", err)
		}
	}

	// Update status
	if err := r.updateStatus(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	reconcileTotal.WithLabelValues(instance.Name, instance.Namespace, "success").Inc()

	if r.Recorder != nil {
		r.Recorder.Event(instance, corev1.EventTypeNormal, "ReconcileSucceeded",
			"All managed resources reconciled successfully")
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *InstanceReconciler) reconcileServiceAccount(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	desired := resources.BuildServiceAccount(instance)
	obj := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desired.Name,
			Namespace: desired.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		obj.Labels = desired.Labels
		obj.Annotations = desired.Annotations
		return controllerutil.SetControllerReference(instance, obj, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling ServiceAccount: %w", err)
	}

	instance.Status.ManagedResources.ServiceAccount = obj.Name
	return nil
}

func (r *InstanceReconciler) reconcileManagedDatabase(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	// Ensure database credentials secret exists
	if err := r.ensureDatabaseSecret(ctx, instance); err != nil {
		return fmt.Errorf("reconciling database secret: %w", err)
	}

	// Database PVC
	pvc := &corev1.PersistentVolumeClaim{}
	pvcName := resources.DatabasePVCName(instance)
	err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: instance.Namespace}, pvc)
	if apierrors.IsNotFound(err) {
		desired := resources.BuildDatabasePVC(instance)
		if setErr := controllerutil.SetControllerReference(instance, desired, r.Scheme); setErr != nil {
			return fmt.Errorf("setting owner reference on database PVC: %w", setErr)
		}
		if createErr := r.Create(ctx, desired); createErr != nil {
			return fmt.Errorf("creating database PVC: %w", createErr)
		}
		instance.Status.ManagedResources.DatabasePVC = pvcName
	} else if err != nil {
		return fmt.Errorf("getting database PVC: %w", err)
	}

	// Database Service
	desiredSvc := resources.BuildDatabaseService(instance)
	svcObj := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desiredSvc.Name,
			Namespace: desiredSvc.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, svcObj, func() error {
		svcObj.Labels = desiredSvc.Labels
		svcObj.Spec.Selector = desiredSvc.Spec.Selector
		svcObj.Spec.Ports = desiredSvc.Spec.Ports
		svcObj.Spec.Type = desiredSvc.Spec.Type
		svcObj.Spec.SessionAffinity = desiredSvc.Spec.SessionAffinity
		return controllerutil.SetControllerReference(instance, svcObj, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling database Service: %w", err)
	}
	instance.Status.ManagedResources.DatabaseService = svcObj.Name

	// Database StatefulSet
	desiredSts := resources.BuildDatabaseStatefulSet(instance)
	stsObj := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desiredSts.Name,
			Namespace: desiredSts.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, stsObj, func() error {
		stsObj.Labels = desiredSts.Labels
		stsObj.Spec = desiredSts.Spec
		return controllerutil.SetControllerReference(instance, stsObj, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling database StatefulSet: %w", err)
	}
	instance.Status.ManagedResources.DatabaseStatefulSet = stsObj.Name

	// Set database condition
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               ConditionDatabaseReady,
		Status:             metav1.ConditionTrue,
		Reason:             "DatabaseProvisioned",
		Message:            "Managed PostgreSQL database is provisioned",
		ObservedGeneration: instance.Generation,
	})

	return nil
}

func (r *InstanceReconciler) ensureDatabaseSecret(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	secret := &corev1.Secret{}
	secretName := resources.DatabaseSecretName(instance)
	err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: instance.Namespace}, secret)
	if apierrors.IsNotFound(err) {
		password, genErr := generatePassword(32)
		if genErr != nil {
			return fmt.Errorf("generating database password: %w", genErr)
		}
		desired := resources.BuildDatabaseSecret(instance, password)
		if setErr := controllerutil.SetControllerReference(instance, desired, r.Scheme); setErr != nil {
			return fmt.Errorf("setting owner reference on database secret: %w", setErr)
		}
		if createErr := r.Create(ctx, desired); createErr != nil {
			return fmt.Errorf("creating database secret: %w", createErr)
		}
		return nil
	}
	return err
}

func (r *InstanceReconciler) reconcilePVC(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	pvc := &corev1.PersistentVolumeClaim{}
	pvcName := resources.PVCName(instance)
	err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: instance.Namespace}, pvc)
	if apierrors.IsNotFound(err) {
		desired := resources.BuildPersistentVolumeClaim(instance)
		if setErr := controllerutil.SetControllerReference(instance, desired, r.Scheme); setErr != nil {
			return fmt.Errorf("setting owner reference on PVC: %w", setErr)
		}
		if createErr := r.Create(ctx, desired); createErr != nil {
			return fmt.Errorf("creating PVC: %w", createErr)
		}
		instance.Status.ManagedResources.PersistentVolumeClaim = pvcName
		return nil
	}
	if err != nil {
		return fmt.Errorf("getting PVC: %w", err)
	}
	instance.Status.ManagedResources.PersistentVolumeClaim = pvcName
	return nil
}

func (r *InstanceReconciler) reconcileStatefulSet(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	desired := resources.BuildStatefulSet(instance)
	obj := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desired.Name,
			Namespace: desired.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		obj.Labels = desired.Labels
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(instance, obj, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling StatefulSet: %w", err)
	}

	instance.Status.ManagedResources.StatefulSet = obj.Name

	// Update StatefulSet condition
	ready := obj.Status.ReadyReplicas > 0
	status := metav1.ConditionFalse
	reason := "StatefulSetNotReady"
	message := "StatefulSet has no ready replicas"
	if ready {
		status = metav1.ConditionTrue
		reason = "StatefulSetReady"
		message = fmt.Sprintf("StatefulSet has %d ready replicas", obj.Status.ReadyReplicas)
	}
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               ConditionStatefulSetReady,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: instance.Generation,
	})

	return nil
}

func (r *InstanceReconciler) reconcileService(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	desired := resources.BuildService(instance)
	obj := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desired.Name,
			Namespace: desired.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		obj.Labels = desired.Labels
		obj.Annotations = desired.Annotations
		obj.Spec.Selector = desired.Spec.Selector
		obj.Spec.Ports = desired.Spec.Ports
		obj.Spec.Type = desired.Spec.Type
		obj.Spec.SessionAffinity = desired.Spec.SessionAffinity
		// Preserve ClusterIP (server-assigned)
		return controllerutil.SetControllerReference(instance, obj, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling Service: %w", err)
	}

	instance.Status.ManagedResources.Service = obj.Name

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               ConditionServiceReady,
		Status:             metav1.ConditionTrue,
		Reason:             "ServiceReady",
		Message:            "Service is provisioned",
		ObservedGeneration: instance.Generation,
	})

	return nil
}

func (r *InstanceReconciler) reconcileIngress(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	desired := resources.BuildIngress(instance)
	if desired == nil {
		return nil
	}

	obj := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desired.Name,
			Namespace: desired.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		obj.Labels = desired.Labels
		obj.Annotations = desired.Annotations
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(instance, obj, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling Ingress: %w", err)
	}

	instance.Status.ManagedResources.Ingress = obj.Name
	return nil
}

func (r *InstanceReconciler) reconcileNetworkPolicy(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	desired := resources.BuildNetworkPolicy(instance)
	obj := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desired.Name,
			Namespace: desired.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		obj.Labels = desired.Labels
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(instance, obj, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling NetworkPolicy: %w", err)
	}

	instance.Status.ManagedResources.NetworkPolicy = obj.Name
	return nil
}

func (r *InstanceReconciler) reconcileHPA(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	desired := resources.BuildHorizontalPodAutoscaler(instance)
	if desired == nil {
		return nil
	}

	obj := desired.DeepCopy()
	obj.Spec = desired.Spec

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		obj.Labels = desired.Labels
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(instance, obj, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling HPA: %w", err)
	}

	return nil
}

func (r *InstanceReconciler) reconcilePDB(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	desired := resources.BuildPodDisruptionBudget(instance)
	if desired == nil {
		return nil
	}

	obj := desired.DeepCopy()
	obj.Spec = desired.Spec

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		obj.Labels = desired.Labels
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(instance, obj, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling PDB: %w", err)
	}

	return nil
}

func (r *InstanceReconciler) updateStatus(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	instance.Status.ObservedGeneration = instance.Generation

	// Determine overall phase
	allReady := true
	for _, cond := range instance.Status.Conditions {
		if cond.Status != metav1.ConditionTrue {
			allReady = false
			break
		}
	}

	if allReady && len(instance.Status.Conditions) > 0 {
		instance.Status.Phase = paperclipv1alpha1.PhaseRunning
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             "AllResourcesReady",
			Message:            "All managed resources are ready",
			ObservedGeneration: instance.Generation,
		})
	} else {
		instance.Status.Phase = paperclipv1alpha1.PhaseProvisioning
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "ResourcesNotReady",
			Message:            "Some managed resources are not yet ready",
			ObservedGeneration: instance.Generation,
		})
	}

	// Set endpoint
	if instance.Spec.Deployment.PublicURL != "" {
		instance.Status.Endpoint = instance.Spec.Deployment.PublicURL
	} else {
		port := int32(3100)
		if instance.Spec.Networking.Service.Port > 0 {
			port = instance.Spec.Networking.Service.Port
		}
		instance.Status.Endpoint = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d",
			resources.ServiceName(instance), instance.Namespace, port)
	}

	return r.Status().Update(ctx, instance)
}

func (r *InstanceReconciler) setPhase(_ context.Context, instance *paperclipv1alpha1.Instance, phase paperclipv1alpha1.InstancePhase) {
	instance.Status.Phase = phase
}

//nolint:unparam // return signature matches controller-runtime convention
func (r *InstanceReconciler) handleError(ctx context.Context, instance *paperclipv1alpha1.Instance, resource string, err error) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Error(err, "Failed to reconcile resource", "resource", resource)

	reconcileTotal.WithLabelValues(instance.Name, instance.Namespace, "error").Inc()
	resourceCreationFailures.WithLabelValues(instance.Name, instance.Namespace, resource).Inc()

	instance.Status.Phase = paperclipv1alpha1.PhaseDegraded
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             "ReconcileError",
		Message:            fmt.Sprintf("Failed to reconcile %s: %v", resource, err),
		ObservedGeneration: instance.Generation,
	})

	if statusErr := r.Status().Update(ctx, instance); statusErr != nil {
		log.Error(statusErr, "Failed to update status after error")
	}

	if r.Recorder != nil {
		r.Recorder.Eventf(instance, corev1.EventTypeWarning, "ReconcileError",
			"Failed to reconcile %s: %v", resource, err)
	}

	return ctrl.Result{}, err
}

func generatePassword(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes)[:length], nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&paperclipv1alpha1.Instance{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Secret{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Named("instance").
		Complete(r)
}
