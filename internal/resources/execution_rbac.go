package resources

import (
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// ExecutionClusterRoleName returns the name of the cluster-scoped Role granting
// the app the permissions the @paperclipai/plugin-kubernetes sandbox provider
// needs to provision and run per-tenant agent workloads in-cluster. It is
// namespaced into the name so multiple instances sharing a cluster don't
// collide.
func ExecutionClusterRoleName(instance *paperclipv1alpha1.Instance) string {
	return instance.Namespace + "-" + instance.Name + "-execution"
}

// executionPolicyRules returns the EXACT RBAC the plugin's orchestrators use.
// Every rule is justified by a concrete Kubernetes API call in
// packages/plugins/sandbox-providers/kubernetes/src/. Scope is deliberately
// minimal: the app creates the per-tenant scaffolding once (first-write-wins)
// and then creates/reads/deletes per-run workloads. We grant a ClusterRole (not
// a namespaced Role) because the app creates these resources across MANY tenant
// namespaces it derives at runtime, and because namespace create/delete is
// itself cluster-scoped.
func executionPolicyRules(k *paperclipv1alpha1.K8sExecutionSpec) []rbacv1.PolicyRule {
	rules := []rbacv1.PolicyRule{
		// tenant-orchestrator: ensureNamespace (readNamespace / createNamespace).
		// Namespaces are cluster-scoped; the app creates one per tenant.
		{
			APIGroups: []string{""},
			Resources: []string{"namespaces"},
			Verbs:     []string{"get", "create"},
		},
		// tenant-orchestrator: ensureServiceAccount (read/create the tenant SA
		// that agent pods run as).
		{
			APIGroups: []string{""},
			Resources: []string{"serviceaccounts"},
			Verbs:     []string{"get", "create"},
		},
		// tenant-orchestrator: ensureRole + ensureRoleBinding (the in-namespace
		// pods/log:get grant for the tenant SA).
		{
			APIGroups: []string{"rbac.authorization.k8s.io"},
			Resources: []string{"roles", "rolebindings"},
			Verbs:     []string{"get", "create"},
		},
		// tenant-orchestrator: ensureResourceQuota.
		{
			APIGroups: []string{""},
			Resources: []string{"resourcequotas"},
			Verbs:     []string{"get", "create"},
		},
		// tenant-orchestrator: ensureLimitRange.
		{
			APIGroups: []string{""},
			Resources: []string{"limitranges"},
			Verbs:     []string{"get", "create"},
		},
		// secret-manager: createPerRunSecret writes the per-run Opaque Secret
		// (BOOTSTRAP_TOKEN + adapter env). Deletion is via ownerReference cascade
		// (the Job/Sandbox owns it), so no delete verb is required.
		{
			APIGroups: []string{""},
			Resources: []string{"secrets"},
			Verbs:     []string{"get", "create"},
		},
		// tenant-orchestrator: ensureNetworkPolicies (deny-all + standard egress
		// NetworkPolicy). read-on-404-then-create.
		{
			APIGroups: []string{"networking.k8s.io"},
			Resources: []string{"networkpolicies"},
			Verbs:     []string{"get", "create"},
		},
		// job-orchestrator / sandbox-cr-orchestrator / plugin reachability check:
		// findPodForJob + findPodForSandbox list pods (job-name / managed-by
		// label selectors) and plugin.ts lists pods to confirm namespace
		// reachability; some paths read a pod by name.
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list"},
		},
		// job-orchestrator.streamPodLogs / sandbox streamSandboxLogs:
		// readNamespacedPodLog.
		{
			APIGroups: []string{""},
			Resources: []string{"pods/log"},
			Verbs:     []string{"get"},
		},
		// pod-exec.execInPod: the @kubernetes/client-node Exec class opens the
		// pods/exec subresource (a "create" on the exec subresource). Used for
		// multi-command adapter workflows and the sandbox-cr backend.
		{
			APIGroups: []string{""},
			Resources: []string{"pods/exec"},
			Verbs:     []string{"create", "get"},
		},
		// job-orchestrator: createJob / readNamespacedJobStatus / deleteJob (the
		// default "job" backend runs each agent as a batch/v1 Job).
		{
			APIGroups: []string{"batch"},
			Resources: []string{"jobs"},
			Verbs:     []string{"get", "list", "create", "delete"},
		},
	}

	// sandbox-cr-orchestrator: create/get/delete agents.x-k8s.io Sandbox CRs.
	// Only the "sandbox-cr" backend touches these; grant when that backend is
	// selected (or when the backend is unspecified, since the plugin can fall
	// back to it per-lease via lease metadata).
	if k == nil || k.Backend != "job" {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{"agents.x-k8s.io"},
			Resources: []string{"sandboxes"},
			Verbs:     []string{"get", "create", "delete"},
		})
	}

	// tenant-orchestrator: ensureCiliumNetworkPolicy creates/reads a
	// cilium.io/v2 CiliumNetworkPolicy. Only the "cilium" egress mode uses it.
	if k != nil && k.EgressMode == "cilium" {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{"cilium.io"},
			Resources: []string{"ciliumnetworkpolicies"},
			Verbs:     []string{"get", "create"},
		})
	}

	// pod-spec-builder sets runtimeClassName on agent pods. When a RuntimeClass
	// (e.g. gvisor) is configured, the app needs get/use on that RuntimeClass so
	// the apiserver admits pods referencing it. Scope to the named class only.
	if k != nil && k.RuntimeClassName != "" {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups:     []string{"node.k8s.io"},
			Resources:     []string{"runtimeclasses"},
			ResourceNames: []string{k.RuntimeClassName},
			Verbs:         []string{"get", "use"},
		})
	}

	return rules
}

// BuildExecutionClusterRole creates the cluster-scoped Role granting the app the
// permissions the Kubernetes sandbox provider needs. Returns nil when the
// instance is not in Kubernetes execution mode.
func BuildExecutionClusterRole(instance *paperclipv1alpha1.Instance) *rbacv1.ClusterRole {
	if !IsKubernetesExecution(instance) {
		return nil
	}
	var k *paperclipv1alpha1.K8sExecutionSpec
	if instance.Spec.Adapters.Execution != nil {
		k = instance.Spec.Adapters.Execution.Kubernetes
	}
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   ExecutionClusterRoleName(instance),
			Labels: Labels(instance),
		},
		Rules: executionPolicyRules(k),
	}
}

// BuildExecutionClusterRoleBinding binds the execution ClusterRole to the app
// ServiceAccount. Returns nil when the instance is not in Kubernetes execution
// mode.
func BuildExecutionClusterRoleBinding(instance *paperclipv1alpha1.Instance) *rbacv1.ClusterRoleBinding {
	if !IsKubernetesExecution(instance) {
		return nil
	}
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   ExecutionClusterRoleName(instance),
			Labels: Labels(instance),
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     ExecutionClusterRoleName(instance),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      ServiceAccountName(instance),
				Namespace: instance.Namespace,
			},
		},
	}
}
