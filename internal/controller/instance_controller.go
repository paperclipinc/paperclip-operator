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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
	"github.com/paperclipinc/paperclip-operator/internal/registry"
	"github.com/paperclipinc/paperclip-operator/internal/resources"
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
	// ConditionRedisReady indicates the managed Redis is ready.
	ConditionRedisReady = "RedisReady"
	// ConditionNetworkPolicyReady indicates the NetworkPolicy is reconciled.
	ConditionNetworkPolicyReady = "NetworkPolicyReady"
	// ConditionRBACReady indicates the ServiceAccount and RBAC are reconciled.
	ConditionRBACReady = "RBACReady"
	// ConditionIngressReady indicates the Ingress is reconciled.
	ConditionIngressReady = "IngressReady"
	// ConditionHTTPRouteReady indicates the HTTPRoute is reconciled.
	ConditionHTTPRouteReady = "HTTPRouteReady"
	// ConditionPDBReady indicates the PodDisruptionBudget is reconciled.
	ConditionPDBReady = "PDBReady"
	// ConditionHPAReady indicates the HorizontalPodAutoscaler is reconciled.
	ConditionHPAReady = "HPAReady"
	// ConditionBackupReady indicates the backup CronJob is reconciled.
	ConditionBackupReady = "BackupReady"
	// ConditionSuspended indicates the instance is suspended (scaled to zero).
	ConditionSuspended = "Suspended"

	// ModeManaged is the value for managed resource modes (database, redis).
	ModeManaged = "managed"

	// AnnotationResolvedDigest is the pod template annotation that records the current resolved digest.
	// Changing this annotation triggers a rolling restart.
	AnnotationResolvedDigest = "paperclip.inc/resolved-digest"
)

// InstanceReconciler reconciles a Instance object.
type InstanceReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	RegistryClient *registry.Client
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
// +kubebuilder:rbac:groups="",resources=pods,verbs=create;delete;get;list;patch;watch
// +kubebuilder:rbac:groups="",resources=pods/exec,verbs=create;get
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=create;get;list;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=policy,resources=poddisruptionbudgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

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
		for _, phase := range []string{"Pending", "Provisioning", "Running", "Degraded", "Failed", "Terminating", "BackingUp", "Restoring", "Updating", "Suspended"} {
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

			// Clean up cluster-scoped resources (can't use owner references)
			if err := r.cleanupClusterScopedResources(ctx, instance); err != nil {
				log.Error(err, "Failed to cleanup cluster-scoped resources")
				return ctrl.Result{}, err
			}

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

	// 0. Ensure shared secrets master key exists (before StatefulSet needs it)
	if err := r.ensureSecretsMasterKey(ctx, instance); err != nil {
		return r.handleError(ctx, instance, "SecretsMasterKey", err)
	}

	// 1. ServiceAccount
	if instance.Spec.Security.RBAC.Create {
		if err := r.reconcileServiceAccount(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "ServiceAccount", err)
		}
	}

	// 1.5. Sandbox RBAC (if cloud sandbox enabled)
	if cs := instance.Spec.Adapters.CloudSandbox; cs != nil && cs.Enabled {
		if err := r.reconcileSandboxRBAC(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "SandboxRBAC", err)
		}
	}

	// 2. Database (if managed)
	if instance.Spec.Database.Mode == ModeManaged || instance.Spec.Database.Mode == "" {
		if err := r.reconcileManagedDatabase(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "Database", err)
		}
	}

	// 2.5. Redis (if managed)
	if instance.Spec.Redis != nil && (instance.Spec.Redis.Mode == ModeManaged || instance.Spec.Redis.Mode == "") {
		if err := r.reconcileManagedRedis(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "Redis", err)
		}
	}

	// 3. PVC (if persistence enabled)
	if instance.Spec.Storage.Persistence.Enabled {
		if err := r.reconcilePVC(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "PVC", err)
		}
	}

	// 3.5. Auto-update: check registry for new digest
	autoUpdateRequeue := ctrl.Result{}
	if r.RegistryClient != nil {
		autoUpdateRequeue = r.reconcileAutoUpdate(ctx, instance)
	}
	var extraPodAnnotations map[string]string
	if instance.Status.AutoUpdate != nil && instance.Status.AutoUpdate.ResolvedDigest != "" {
		extraPodAnnotations = map[string]string{
			AnnotationResolvedDigest: instance.Status.AutoUpdate.ResolvedDigest,
		}
	}

	// 4. StatefulSet
	if err := r.reconcileStatefulSet(ctx, instance, extraPodAnnotations); err != nil {
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

	// 6b. HTTPRoute (optional, alternative to Ingress for Gateway API)
	if instance.Spec.Networking.HTTPRoute != nil && instance.Spec.Networking.HTTPRoute.Enabled {
		if err := r.reconcileHTTPRoute(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "HTTPRoute", err)
		}
	}

	// 7. NetworkPolicy (optional)
	if instance.Spec.Security.NetworkPolicy.Enabled {
		if err := r.reconcileNetworkPolicy(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "NetworkPolicy", err)
		}
		// Database NetworkPolicy (when managed)
		if instance.Spec.Database.Mode == ModeManaged || instance.Spec.Database.Mode == "" {
			if err := r.reconcileDatabaseNetworkPolicy(ctx, instance); err != nil {
				return r.handleError(ctx, instance, "DatabaseNetworkPolicy", err)
			}
		}
		// Redis NetworkPolicy (when managed)
		if instance.Spec.Redis != nil && (instance.Spec.Redis.Mode == ModeManaged || instance.Spec.Redis.Mode == "") {
			if err := r.reconcileRedisNetworkPolicy(ctx, instance); err != nil {
				return r.handleError(ctx, instance, "RedisNetworkPolicy", err)
			}
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

	// 10. Admin bootstrap Job (optional, runs once)
	if instance.Spec.Auth.AdminUser != nil {
		if err := r.reconcileBootstrapJob(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "BootstrapJob", err)
		}
	}

	// 11. Backup CronJob (optional)
	if instance.Spec.Backup != nil {
		if err := r.reconcileBackupCronJob(ctx, instance); err != nil {
			return r.handleError(ctx, instance, "BackupCronJob", err)
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

	requeueAfter := 5 * time.Minute
	if autoUpdateRequeue.RequeueAfter > 0 && autoUpdateRequeue.RequeueAfter < requeueAfter {
		requeueAfter = autoUpdateRequeue.RequeueAfter
	}
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
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

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               ConditionRBACReady,
		Status:             metav1.ConditionTrue,
		Reason:             "RBACProvisioned",
		Message:            "ServiceAccount and RBAC are provisioned",
		ObservedGeneration: instance.Generation,
	})
	return nil
}

func (r *InstanceReconciler) reconcileSandboxRBAC(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	cs := instance.Spec.Adapters.CloudSandbox
	namespace := cs.Namespace
	if namespace == "" {
		namespace = instance.Namespace
	}

	// Role
	desiredRole := resources.BuildSandboxRole(instance, namespace)
	roleObj := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desiredRole.Name,
			Namespace: desiredRole.Namespace,
		},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, roleObj, func() error {
		roleObj.Labels = desiredRole.Labels
		roleObj.Rules = desiredRole.Rules
		return controllerutil.SetControllerReference(instance, roleObj, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling sandbox Role: %w", err)
	}

	// RoleBinding
	desiredBinding := resources.BuildSandboxRoleBinding(instance, namespace)
	bindingObj := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desiredBinding.Name,
			Namespace: desiredBinding.Namespace,
		},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, bindingObj, func() error {
		bindingObj.Labels = desiredBinding.Labels
		bindingObj.RoleRef = desiredBinding.RoleRef
		bindingObj.Subjects = desiredBinding.Subjects
		return controllerutil.SetControllerReference(instance, bindingObj, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling sandbox RoleBinding: %w", err)
	}

	// ClusterRole + ClusterRoleBinding for multi-namespace sandbox isolation
	if cs.MultiNamespace {
		desiredCR := resources.BuildSandboxClusterRole(instance)
		crObj := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: desiredCR.Name,
			},
		}
		_, err = controllerutil.CreateOrUpdate(ctx, r.Client, crObj, func() error {
			crObj.Labels = desiredCR.Labels
			crObj.Rules = desiredCR.Rules
			return nil // no owner reference for cluster-scoped resources
		})
		if err != nil {
			return fmt.Errorf("reconciling sandbox ClusterRole: %w", err)
		}

		desiredCRB := resources.BuildSandboxClusterRoleBinding(instance)
		crbObj := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: desiredCRB.Name,
			},
		}
		_, err = controllerutil.CreateOrUpdate(ctx, r.Client, crbObj, func() error {
			crbObj.Labels = desiredCRB.Labels
			crbObj.RoleRef = desiredCRB.RoleRef
			crbObj.Subjects = desiredCRB.Subjects
			return nil
		})
		if err != nil {
			return fmt.Errorf("reconciling sandbox ClusterRoleBinding: %w", err)
		}
	}

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

func (r *InstanceReconciler) reconcileManagedRedis(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	// Redis PVC
	pvc := &corev1.PersistentVolumeClaim{}
	pvcName := resources.RedisPVCName(instance)
	err := r.Get(ctx, types.NamespacedName{Name: pvcName, Namespace: instance.Namespace}, pvc)
	if apierrors.IsNotFound(err) {
		desired := resources.BuildRedisPVC(instance)
		if setErr := controllerutil.SetControllerReference(instance, desired, r.Scheme); setErr != nil {
			return fmt.Errorf("setting owner reference on Redis PVC: %w", setErr)
		}
		if createErr := r.Create(ctx, desired); createErr != nil {
			return fmt.Errorf("creating Redis PVC: %w", createErr)
		}
		instance.Status.ManagedResources.RedisPVC = pvcName
	} else if err != nil {
		return fmt.Errorf("getting Redis PVC: %w", err)
	}

	// Redis Service
	desiredSvc := resources.BuildRedisService(instance)
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
		return fmt.Errorf("reconciling Redis Service: %w", err)
	}
	instance.Status.ManagedResources.RedisService = svcObj.Name

	// Redis StatefulSet
	desiredSts := resources.BuildRedisStatefulSet(instance)
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
		return fmt.Errorf("reconciling Redis StatefulSet: %w", err)
	}
	instance.Status.ManagedResources.RedisStatefulSet = stsObj.Name

	// Set Redis condition
	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               ConditionRedisReady,
		Status:             metav1.ConditionTrue,
		Reason:             "RedisProvisioned",
		Message:            "Managed Redis is provisioned",
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

func (r *InstanceReconciler) ensureSecretsMasterKey(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	// Skip if the user provided their own master key secret reference
	if instance.Spec.Secrets.MasterKeySecretRef != nil {
		return nil
	}

	secret := &corev1.Secret{}
	secretName := resources.SecretsMasterKeySecretName(instance)
	err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: instance.Namespace}, secret)
	if apierrors.IsNotFound(err) {
		key, genErr := generatePassword(32)
		if genErr != nil {
			return fmt.Errorf("generating secrets master key: %w", genErr)
		}
		desired := resources.BuildSecretsMasterKeySecret(instance, key)
		if setErr := controllerutil.SetControllerReference(instance, desired, r.Scheme); setErr != nil {
			return fmt.Errorf("setting owner reference on secrets master key: %w", setErr)
		}
		if createErr := r.Create(ctx, desired); createErr != nil {
			return fmt.Errorf("creating secrets master key: %w", createErr)
		}
		return nil
	}
	return err
}

func (r *InstanceReconciler) cleanupClusterScopedResources(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	// Clean up sandbox ClusterRole
	cr := &rbacv1.ClusterRole{}
	crName := resources.SandboxClusterRoleName(instance)
	if err := r.Get(ctx, types.NamespacedName{Name: crName}, cr); err == nil {
		if err := r.Delete(ctx, cr); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting sandbox ClusterRole: %w", err)
		}
	}

	// Clean up sandbox ClusterRoleBinding
	crb := &rbacv1.ClusterRoleBinding{}
	if err := r.Get(ctx, types.NamespacedName{Name: crName}, crb); err == nil {
		if err := r.Delete(ctx, crb); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting sandbox ClusterRoleBinding: %w", err)
		}
	}

	return nil
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

func (r *InstanceReconciler) reconcileStatefulSet(ctx context.Context, instance *paperclipv1alpha1.Instance, extraPodAnnotations map[string]string) error {
	desired := resources.BuildStatefulSet(instance, extraPodAnnotations)
	obj := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desired.Name,
			Namespace: desired.Namespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		obj.Labels = desired.Labels
		// When HPA is enabled, preserve the current replica count to avoid
		// fighting the autoscaler on every reconcile. Suspended takes priority:
		// scale-to-zero must win over the autoscaler's last-known count.
		if as := instance.Spec.Availability.AutoScaling; as != nil && as.Enabled && obj.Spec.Replicas != nil && !instance.Spec.Suspended {
			desired.Spec.Replicas = obj.Spec.Replicas
		}
		obj.Spec = desired.Spec
		return controllerutil.SetControllerReference(instance, obj, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("reconciling StatefulSet: %w", err)
	}

	instance.Status.ManagedResources.StatefulSet = obj.Name

	// Update StatefulSet condition
	status := metav1.ConditionFalse
	reason := "StatefulSetNotReady"
	message := "StatefulSet has no ready replicas"
	if instance.Spec.Suspended {
		// Suspended: desired is 0 replicas. Ready once all pods are terminated.
		if obj.Status.Replicas == 0 {
			status = metav1.ConditionTrue
			reason = "StatefulSetSuspended"
			message = "StatefulSet scaled to zero (instance suspended)"
		} else {
			reason = "StatefulSetSuspending"
			message = fmt.Sprintf("StatefulSet draining %d replicas (instance suspended)", obj.Status.Replicas)
		}
	} else if obj.Status.ReadyReplicas > 0 {
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

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               ConditionIngressReady,
		Status:             metav1.ConditionTrue,
		Reason:             "IngressProvisioned",
		Message:            "Ingress is provisioned",
		ObservedGeneration: instance.Generation,
	})
	return nil
}

func (r *InstanceReconciler) reconcileHTTPRoute(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	desired := resources.BuildHTTPRoute(instance)
	if desired == nil {
		return nil
	}

	obj := &gatewayapiv1.HTTPRoute{
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
		return fmt.Errorf("reconciling HTTPRoute: %w", err)
	}

	instance.Status.ManagedResources.HTTPRoute = obj.Name

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               ConditionHTTPRouteReady,
		Status:             metav1.ConditionTrue,
		Reason:             "HTTPRouteProvisioned",
		Message:            "HTTPRoute is provisioned",
		ObservedGeneration: instance.Generation,
	})
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

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               ConditionNetworkPolicyReady,
		Status:             metav1.ConditionTrue,
		Reason:             "NetworkPolicyProvisioned",
		Message:            "NetworkPolicy is provisioned",
		ObservedGeneration: instance.Generation,
	})
	return nil
}

func (r *InstanceReconciler) reconcileDatabaseNetworkPolicy(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	desired := resources.BuildDatabaseNetworkPolicy(instance)
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
		return fmt.Errorf("reconciling database NetworkPolicy: %w", err)
	}
	return nil
}

func (r *InstanceReconciler) reconcileRedisNetworkPolicy(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	desired := resources.BuildRedisNetworkPolicy(instance)
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
		return fmt.Errorf("reconciling Redis NetworkPolicy: %w", err)
	}
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

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               ConditionHPAReady,
		Status:             metav1.ConditionTrue,
		Reason:             "HPAProvisioned",
		Message:            "HorizontalPodAutoscaler is provisioned",
		ObservedGeneration: instance.Generation,
	})
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

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               ConditionPDBReady,
		Status:             metav1.ConditionTrue,
		Reason:             "PDBProvisioned",
		Message:            "PodDisruptionBudget is provisioned",
		ObservedGeneration: instance.Generation,
	})
	return nil
}

func (r *InstanceReconciler) reconcileBootstrapJob(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	desired := resources.BuildBootstrapJob(instance)
	if desired == nil {
		return nil
	}

	// Check if Job already exists (it should only run once)
	existing := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, existing)
	if err == nil {
		// Job already exists, nothing to do
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("checking bootstrap Job: %w", err)
	}

	// Job does not exist, create it
	if err := controllerutil.SetControllerReference(instance, desired, r.Scheme); err != nil {
		return fmt.Errorf("setting owner reference on bootstrap Job: %w", err)
	}
	if err := r.Create(ctx, desired); err != nil { // reconcile-guard:allow
		return fmt.Errorf("creating bootstrap Job: %w", err)
	}

	return nil
}

func (r *InstanceReconciler) reconcileBackupCronJob(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	desired := resources.BuildBackupCronJob(instance)
	if desired == nil {
		// Backup not fully configured (e.g., no S3 bucket) — clean up any existing CronJob.
		existing := &batchv1.CronJob{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      resources.BackupCronJobName(instance),
			Namespace: instance.Namespace,
		}, existing)
		if err == nil {
			return r.Delete(ctx, existing)
		}
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("checking backup CronJob: %w", err)
	}

	obj := &batchv1.CronJob{
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
		return fmt.Errorf("reconciling backup CronJob: %w", err)
	}

	meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
		Type:               ConditionBackupReady,
		Status:             metav1.ConditionTrue,
		Reason:             "BackupProvisioned",
		Message:            "Backup CronJob is provisioned",
		ObservedGeneration: instance.Generation,
	})
	return nil
}

func (r *InstanceReconciler) updateStatus(ctx context.Context, instance *paperclipv1alpha1.Instance) error {
	instance.Status.ObservedGeneration = instance.Generation

	// Suspended: override phase and readiness. The workload is scaled to zero
	// but all non-runtime resources remain managed.
	if instance.Spec.Suspended {
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionSuspended,
			Status:             metav1.ConditionTrue,
			Reason:             "Suspended",
			Message:            "Instance is suspended (spec.suspended=true), workload scaled to zero",
			ObservedGeneration: instance.Generation,
		})
		meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
			Type:               ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             "Suspended",
			Message:            "Instance is suspended, not serving traffic",
			ObservedGeneration: instance.Generation,
		})
		instance.Status.Phase = paperclipv1alpha1.PhaseSuspended
		r.setEndpoint(instance)
		return r.Status().Update(ctx, instance)
	}

	// Not suspended: clear any stale Suspended condition.
	meta.RemoveStatusCondition(&instance.Status.Conditions, ConditionSuspended)

	// Determine overall phase
	allReady := allSubConditionsReady(instance.Status.Conditions)

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

	r.setEndpoint(instance)

	return r.Status().Update(ctx, instance)
}

// setEndpoint records the primary service endpoint URL in status.
func (r *InstanceReconciler) setEndpoint(instance *paperclipv1alpha1.Instance) {
	if instance.Spec.Deployment.PublicURL != "" {
		instance.Status.Endpoint = instance.Spec.Deployment.PublicURL
		return
	}
	port := int32(3100)
	if instance.Spec.Networking.Service.Port > 0 {
		port = instance.Spec.Networking.Service.Port
	}
	instance.Status.Endpoint = fmt.Sprintf("http://%s.%s.svc.cluster.local:%d",
		resources.ServiceName(instance), instance.Namespace, port)
}

// allSubConditionsReady returns true when every condition except ConditionReady
// has Status=True. The Ready condition itself is excluded to avoid a
// self-referential loop where a previous Ready=False prevents the aggregate
// from ever becoming true.
func allSubConditionsReady(conditions []metav1.Condition) bool {
	for _, cond := range conditions {
		if cond.Type == ConditionReady || cond.Type == ConditionSuspended {
			continue
		}
		if cond.Status != metav1.ConditionTrue {
			return false
		}
	}
	return true
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

func (r *InstanceReconciler) reconcileAutoUpdate(ctx context.Context, instance *paperclipv1alpha1.Instance) ctrl.Result {
	log := logf.FromContext(ctx)
	autoUpdate := instance.Spec.Image.AutoUpdate

	if autoUpdate == nil || !autoUpdate.Enabled {
		instance.Status.AutoUpdate = nil
		return ctrl.Result{}
	}

	interval, err := time.ParseDuration(autoUpdate.Interval)
	if err != nil || interval < time.Minute {
		interval = 5 * time.Minute
	}

	if instance.Status.AutoUpdate == nil {
		instance.Status.AutoUpdate = &paperclipv1alpha1.AutoUpdateStatus{}
	}

	now := metav1.Now()
	if instance.Status.AutoUpdate.LastCheckTime != nil {
		elapsed := now.Sub(instance.Status.AutoUpdate.LastCheckTime.Time)
		if elapsed < interval {
			return ctrl.Result{RequeueAfter: interval - elapsed}
		}
	}

	// Resolve credentials from imagePullSecrets
	var dockerConfigJSON []byte
	if len(instance.Spec.Image.PullSecrets) > 0 {
		secret := &corev1.Secret{}
		getErr := r.Get(ctx, types.NamespacedName{
			Name:      instance.Spec.Image.PullSecrets[0].Name,
			Namespace: instance.Namespace,
		}, secret)
		if getErr != nil {
			log.Error(getErr, "Failed to get imagePullSecret for auto-update")
			instance.Status.AutoUpdate.LastError = getErr.Error()
			instance.Status.AutoUpdate.LastCheckTime = &now
			return ctrl.Result{RequeueAfter: interval}
		}
		dockerConfigJSON = secret.Data[".dockerconfigjson"]
	}

	repo := instance.Spec.Image.Repository
	if repo == "" {
		repo = "ghcr.io/paperclipinc/paperclip"
	}
	tag := instance.Spec.Image.Tag
	if tag == "" {
		instance.Status.AutoUpdate.LastError = "auto-update requires spec.image.tag to be set; cannot poll a digest-pinned image"
		instance.Status.AutoUpdate.LastCheckTime = &now
		return ctrl.Result{RequeueAfter: interval}
	}

	digest, err := r.RegistryClient.ResolveDigest(ctx, repo, tag, dockerConfigJSON)
	instance.Status.AutoUpdate.LastCheckTime = &now

	if err != nil {
		log.Error(err, "Failed to resolve image digest", "repo", repo, "tag", tag)
		instance.Status.AutoUpdate.LastError = err.Error()
		if r.Recorder != nil {
			r.Recorder.Eventf(instance, corev1.EventTypeWarning, "AutoUpdateCheckFailed",
				"Failed to check registry for %s:%s: %v", repo, tag, err)
		}
		return ctrl.Result{RequeueAfter: interval}
	}

	instance.Status.AutoUpdate.LastError = ""
	previousDigest := instance.Status.AutoUpdate.ResolvedDigest
	if digest != previousDigest {
		log.Info("New image digest detected", "repo", repo, "tag", tag,
			"previousDigest", previousDigest, "newDigest", digest)
		instance.Status.AutoUpdate.ResolvedDigest = digest
		instance.Status.AutoUpdate.LastUpdateTime = &now
		if r.Recorder != nil {
			r.Recorder.Eventf(instance, corev1.EventTypeNormal, "AutoUpdateDigestChanged",
				"New digest detected for %s:%s: %s", repo, tag, digest)
		}
	}

	return ctrl.Result{RequeueAfter: interval}
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
		Owns(&gatewayapiv1.HTTPRoute{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Named("instance").
		Complete(r)
}
