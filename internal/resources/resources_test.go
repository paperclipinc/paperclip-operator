package resources

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

func newTestInstance(name string) *paperclipv1alpha1.Instance {
	return &paperclipv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test-ns",
		},
		Spec: paperclipv1alpha1.InstanceSpec{
			Image: paperclipv1alpha1.ImageSpec{
				Repository: "ghcr.io/paperclipai/paperclip",
				Tag:        "v1.0.0",
			},
			Deployment: paperclipv1alpha1.DeploymentSpec{
				Mode:     "authenticated",
				Exposure: "private",
			},
			Database: paperclipv1alpha1.DatabaseSpec{
				Mode: "managed",
			},
			Storage: paperclipv1alpha1.StorageSpec{
				Persistence: paperclipv1alpha1.PersistenceSpec{
					Enabled: true,
					Size:    resource.MustParse("5Gi"),
				},
			},
			Security: paperclipv1alpha1.SecuritySpec{
				RBAC: paperclipv1alpha1.RBACSpec{
					Create: true,
				},
				NetworkPolicy: paperclipv1alpha1.NetworkPolicySpec{
					Enabled: true,
				},
			},
			Heartbeat: paperclipv1alpha1.HeartbeatSpec{
				Enabled:    true,
				IntervalMS: 60000,
			},
		},
	}
}

func TestBuildStatefulSet(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	sts := BuildStatefulSet(instance, nil)

	if sts.Name != "my-paperclip" {
		t.Errorf("expected StatefulSet name 'my-paperclip', got %q", sts.Name)
	}
	if sts.Namespace != "test-ns" {
		t.Errorf("expected namespace 'test-ns', got %q", sts.Namespace)
	}
	if *sts.Spec.Replicas != 1 {
		t.Errorf("expected 1 replica, got %d", *sts.Spec.Replicas)
	}
	if len(sts.Spec.Template.Spec.Containers) < 1 {
		t.Fatal("expected at least 1 container")
	}

	container := sts.Spec.Template.Spec.Containers[0]
	if container.Name != ContainerName {
		t.Errorf("expected container name %q, got %q", ContainerName, container.Name)
	}
	if container.Image != "ghcr.io/paperclipai/paperclip:v1.0.0" {
		t.Errorf("expected image 'ghcr.io/paperclipai/paperclip:v1.0.0', got %q", container.Image)
	}

	// Verify port
	if len(container.Ports) == 0 {
		t.Fatal("expected at least 1 port")
	}
	if container.Ports[0].ContainerPort != DefaultPort {
		t.Errorf("expected port %d, got %d", DefaultPort, container.Ports[0].ContainerPort)
	}

	// Verify probes exist
	if container.LivenessProbe == nil {
		t.Error("expected liveness probe")
	}
	if container.ReadinessProbe == nil {
		t.Error("expected readiness probe")
	}
	if container.StartupProbe == nil {
		t.Error("expected startup probe")
	}

	// Verify probe type: authenticated mode should use TCP probes
	if container.LivenessProbe.TCPSocket == nil {
		t.Error("expected TCP liveness probe for authenticated mode")
	}
	if container.ReadinessProbe.TCPSocket == nil {
		t.Error("expected TCP readiness probe for authenticated mode")
	}

	// Verify volume mounts
	if len(container.VolumeMounts) == 0 {
		t.Fatal("expected at least 1 volume mount")
	}
	if container.VolumeMounts[0].MountPath != DataMountPath {
		t.Errorf("expected mount path %q, got %q", DataMountPath, container.VolumeMounts[0].MountPath)
	}

	// Verify pod security context
	if sts.Spec.Template.Spec.SecurityContext == nil {
		t.Fatal("expected pod security context")
	}
	if *sts.Spec.Template.Spec.SecurityContext.RunAsNonRoot != true {
		t.Error("expected RunAsNonRoot=true")
	}
}

func TestBuildStatefulSetWithDigest(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Image.Digest = "sha256:abc123"
	sts := BuildStatefulSet(instance, nil)

	container := sts.Spec.Template.Spec.Containers[0]
	expected := "ghcr.io/paperclipai/paperclip@sha256:abc123"
	if container.Image != expected {
		t.Errorf("expected image %q, got %q", expected, container.Image)
	}
}

func TestBuildStatefulSetEnvVars(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Deployment.PublicURL = "https://paperclip.example.com"
	instance.Spec.Deployment.AllowedHostnames = []string{"paperclip.example.com"}
	sts := BuildStatefulSet(instance, nil)

	container := sts.Spec.Template.Spec.Containers[0]

	envMap := make(map[string]string)
	for _, env := range container.Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}

	if envMap["PAPERCLIP_BIND"] != "custom" {
		t.Error("expected PAPERCLIP_BIND=custom")
	}
	if envMap["PAPERCLIP_BIND_HOST"] != "0.0.0.0" {
		t.Error("expected PAPERCLIP_BIND_HOST=0.0.0.0")
	}
	if envMap["SERVE_UI"] != "true" {
		t.Error("expected SERVE_UI=true")
	}
	if envMap["PAPERCLIP_HOME"] != DataMountPath {
		t.Errorf("expected PAPERCLIP_HOME=%s", DataMountPath)
	}
	if envMap["PAPERCLIP_PUBLIC_URL"] != "https://paperclip.example.com" {
		t.Error("expected PAPERCLIP_PUBLIC_URL=https://paperclip.example.com")
	}
	// Always includes in-cluster Service DNS plus the user-specified hostname.
	if !strings.Contains(envMap["PAPERCLIP_ALLOWED_HOSTNAMES"], "paperclip.example.com") {
		t.Errorf("expected PAPERCLIP_ALLOWED_HOSTNAMES to include paperclip.example.com, got %q", envMap["PAPERCLIP_ALLOWED_HOSTNAMES"])
	}
}

func TestBuildService(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	svc := BuildService(instance)

	if svc.Name != "my-paperclip" {
		t.Errorf("expected Service name 'my-paperclip', got %q", svc.Name)
	}
	if svc.Spec.Type != corev1.ServiceTypeClusterIP {
		t.Errorf("expected ClusterIP, got %s", svc.Spec.Type)
	}
	if svc.Spec.SessionAffinity != corev1.ServiceAffinityNone {
		t.Errorf("expected SessionAffinity None, got %s", svc.Spec.SessionAffinity)
	}
	if len(svc.Spec.Ports) == 0 {
		t.Fatal("expected at least 1 port")
	}
	if svc.Spec.Ports[0].Port != DefaultPort {
		t.Errorf("expected port %d, got %d", DefaultPort, svc.Spec.Ports[0].Port)
	}
}

func TestBuildServiceLoadBalancer(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Networking.Service.Type = corev1.ServiceTypeLoadBalancer
	svc := BuildService(instance)

	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		t.Errorf("expected LoadBalancer, got %s", svc.Spec.Type)
	}
}

func TestBuildDatabaseStatefulSet(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	sts := BuildDatabaseStatefulSet(instance)

	if sts.Name != "my-paperclip-db" {
		t.Errorf("expected name 'my-paperclip-db', got %q", sts.Name)
	}
	if *sts.Spec.Replicas != 1 {
		t.Errorf("expected 1 replica, got %d", *sts.Spec.Replicas)
	}

	container := sts.Spec.Template.Spec.Containers[0]
	if container.Image != "postgres:17-alpine" {
		t.Errorf("expected image 'postgres:17-alpine', got %q", container.Image)
	}
	if container.LivenessProbe == nil {
		t.Error("expected liveness probe")
	}
	if container.ReadinessProbe == nil {
		t.Error("expected readiness probe")
	}
}

func TestBuildDatabaseService(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	svc := BuildDatabaseService(instance)

	if svc.Name != "my-paperclip-db" {
		t.Errorf("expected name 'my-paperclip-db', got %q", svc.Name)
	}
	if svc.Spec.Ports[0].Port != PostgreSQLPort {
		t.Errorf("expected port %d, got %d", PostgreSQLPort, svc.Spec.Ports[0].Port)
	}
}

func TestBuildPersistentVolumeClaim(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	pvc := BuildPersistentVolumeClaim(instance)

	if pvc.Name != "my-paperclip-data" {
		t.Errorf("expected name 'my-paperclip-data', got %q", pvc.Name)
	}

	storage := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	if storage.Cmp(resource.MustParse("5Gi")) != 0 {
		t.Errorf("expected 5Gi storage, got %s", storage.String())
	}
}

func TestBuildPVCCustomStorageClass(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	sc := "fast-ssd"
	instance.Spec.Storage.Persistence.StorageClass = &sc
	pvc := BuildPersistentVolumeClaim(instance)

	if pvc.Spec.StorageClassName == nil || *pvc.Spec.StorageClassName != "fast-ssd" {
		t.Error("expected storage class 'fast-ssd'")
	}
}

func TestBuildNetworkPolicy(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	np := BuildNetworkPolicy(instance)

	if np.Name != "my-paperclip" {
		t.Errorf("expected name 'my-paperclip', got %q", np.Name)
	}
	if len(np.Spec.PolicyTypes) != 2 {
		t.Errorf("expected 2 policy types, got %d", len(np.Spec.PolicyTypes))
	}

	// Should have ingress rule for the service port
	if len(np.Spec.Ingress) == 0 {
		t.Fatal("expected at least 1 ingress rule")
	}

	// Should have egress rules for DNS, HTTPS, and database
	if len(np.Spec.Egress) < 3 {
		t.Errorf("expected at least 3 egress rules (DNS, HTTPS, database), got %d", len(np.Spec.Egress))
	}
}

func TestBuildNetworkPolicyCloudSandboxK8sAPIEgress(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Adapters.CloudSandbox = &paperclipv1alpha1.CloudSandboxSpec{
		Enabled: true,
	}
	np := BuildNetworkPolicy(instance)

	found := false
	for _, rule := range np.Spec.Egress {
		for _, port := range rule.Ports {
			if port.Port != nil && port.Port.IntValue() == 6443 {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected egress rule for K8s API port 6443 when cloud sandbox enabled")
	}
}

func TestBuildNetworkPolicyNoK8sAPIEgressWithoutSandbox(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	np := BuildNetworkPolicy(instance)

	for _, rule := range np.Spec.Egress {
		for _, port := range rule.Ports {
			if port.Port != nil && port.Port.IntValue() == 6443 {
				t.Error("should not have K8s API egress rule when cloud sandbox is not enabled")
			}
		}
	}
}

func TestBuildIngress(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Networking.Ingress = &paperclipv1alpha1.IngressSpec{
		Enabled:          true,
		IngressClassName: Ptr("nginx"),
		Hosts:            []string{"paperclip.example.com"},
		TLS: []paperclipv1alpha1.IngressTLSSpec{
			{
				Hosts:      []string{"paperclip.example.com"},
				SecretName: "paperclip-tls",
			},
		},
		Annotations: map[string]string{
			"nginx.ingress.kubernetes.io/proxy-read-timeout": "3600",
		},
	}

	ing := BuildIngress(instance)
	if ing == nil {
		t.Fatal("expected non-nil Ingress")
	}

	if *ing.Spec.IngressClassName != "nginx" {
		t.Errorf("expected ingress class 'nginx', got %q", *ing.Spec.IngressClassName)
	}
	if len(ing.Spec.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(ing.Spec.Rules))
	}
	if ing.Spec.Rules[0].Host != "paperclip.example.com" {
		t.Errorf("expected host 'paperclip.example.com', got %q", ing.Spec.Rules[0].Host)
	}
	if len(ing.Spec.TLS) != 1 {
		t.Fatalf("expected 1 TLS entry, got %d", len(ing.Spec.TLS))
	}
	if ing.Annotations["nginx.ingress.kubernetes.io/proxy-read-timeout"] != "3600" {
		t.Error("expected WebSocket annotation")
	}
}

func TestBuildIngressNil(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Networking.Ingress = nil
	ing := BuildIngress(instance)
	if ing != nil {
		t.Error("expected nil Ingress when spec is nil")
	}
}

func TestBuildHTTPRoute(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Networking.HTTPRoute = &paperclipv1alpha1.HTTPRouteSpec{
		Enabled: true,
		ParentRefs: []paperclipv1alpha1.HTTPRouteParentRef{
			{
				Name:        "my-gateway",
				SectionName: Ptr("https"),
			},
		},
		Hostnames: []string{"paperclip.example.com"},
		Annotations: map[string]string{
			"example.com/custom": "value",
		},
	}

	route := BuildHTTPRoute(instance)
	if route == nil {
		t.Fatal("expected non-nil HTTPRoute")
	}

	if route.Name != "my-paperclip" {
		t.Errorf("expected name 'my-paperclip', got %q", route.Name)
	}

	if len(route.Spec.ParentRefs) != 1 {
		t.Fatalf("expected 1 parentRef, got %d", len(route.Spec.ParentRefs))
	}
	if string(route.Spec.ParentRefs[0].Name) != "my-gateway" {
		t.Errorf("expected parentRef name 'my-gateway', got %q", route.Spec.ParentRefs[0].Name)
	}
	if route.Spec.ParentRefs[0].SectionName == nil || string(*route.Spec.ParentRefs[0].SectionName) != "https" {
		t.Error("expected parentRef sectionName 'https'")
	}

	if len(route.Spec.Hostnames) != 1 {
		t.Fatalf("expected 1 hostname, got %d", len(route.Spec.Hostnames))
	}
	if string(route.Spec.Hostnames[0]) != "paperclip.example.com" {
		t.Errorf("expected hostname 'paperclip.example.com', got %q", route.Spec.Hostnames[0])
	}

	if len(route.Spec.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(route.Spec.Rules))
	}
	rule := route.Spec.Rules[0]
	if len(rule.BackendRefs) != 1 {
		t.Fatalf("expected 1 backendRef, got %d", len(rule.BackendRefs))
	}
	if string(rule.BackendRefs[0].Name) != "my-paperclip" {
		t.Errorf("expected backendRef name 'my-paperclip', got %q", rule.BackendRefs[0].Name)
	}
	if rule.BackendRefs[0].Port == nil || int32(*rule.BackendRefs[0].Port) != 3100 {
		t.Error("expected backendRef port 3100")
	}

	if route.Annotations["example.com/custom"] != "value" {
		t.Error("expected custom annotation")
	}
}

func TestBuildHTTPRouteNil(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Networking.HTTPRoute = nil
	route := BuildHTTPRoute(instance)
	if route != nil {
		t.Error("expected nil HTTPRoute when spec is nil")
	}
}

func TestBuildHTTPRouteWithNamespace(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Networking.HTTPRoute = &paperclipv1alpha1.HTTPRouteSpec{
		Enabled: true,
		ParentRefs: []paperclipv1alpha1.HTTPRouteParentRef{
			{
				Name:      "shared-gateway",
				Namespace: Ptr("gateway-system"),
			},
		},
		Hostnames: []string{"app.example.com"},
	}

	route := BuildHTTPRoute(instance)
	if route == nil {
		t.Fatal("expected non-nil HTTPRoute")
	}

	if route.Spec.ParentRefs[0].Namespace == nil {
		t.Fatal("expected parentRef namespace to be set")
	}
	if string(*route.Spec.ParentRefs[0].Namespace) != "gateway-system" {
		t.Errorf("expected namespace 'gateway-system', got %q", *route.Spec.ParentRefs[0].Namespace)
	}
}

func TestBuildServiceAccount(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Security.RBAC.ServiceAccountAnnotations = map[string]string{
		"eks.amazonaws.com/role-arn": "arn:aws:iam::role/paperclip",
	}
	sa := BuildServiceAccount(instance)

	if sa.Name != "my-paperclip" {
		t.Errorf("expected name 'my-paperclip', got %q", sa.Name)
	}
	if sa.Annotations["eks.amazonaws.com/role-arn"] != "arn:aws:iam::role/paperclip" {
		t.Error("expected IRSA annotation")
	}
}

func TestBuildHPA(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Availability.AutoScaling = &paperclipv1alpha1.AutoScalingSpec{
		Enabled:                        true,
		MinReplicas:                    Ptr(int32(2)),
		MaxReplicas:                    5,
		TargetCPUUtilizationPercentage: Ptr(int32(75)),
	}

	hpa := BuildHorizontalPodAutoscaler(instance)
	if hpa == nil {
		t.Fatal("expected non-nil HPA")
	}
	if *hpa.Spec.MinReplicas != 2 {
		t.Errorf("expected minReplicas 2, got %d", *hpa.Spec.MinReplicas)
	}
	if hpa.Spec.MaxReplicas != 5 {
		t.Errorf("expected maxReplicas 5, got %d", hpa.Spec.MaxReplicas)
	}
	if len(hpa.Spec.Metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(hpa.Spec.Metrics))
	}
}

func TestBuildHPANil(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Availability.AutoScaling = nil
	hpa := BuildHorizontalPodAutoscaler(instance)
	if hpa != nil {
		t.Error("expected nil HPA when spec is nil")
	}
}

func TestBuildPDB(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Availability.PodDisruptionBudget = &paperclipv1alpha1.PDBSpec{
		Enabled:      true,
		MinAvailable: Ptr(int32(1)),
	}

	pdb := BuildPodDisruptionBudget(instance)
	if pdb == nil {
		t.Fatal("expected non-nil PDB")
	}
	if pdb.Name != "my-paperclip" {
		t.Errorf("expected name 'my-paperclip', got %q", pdb.Name)
	}
}

func TestBuildStatefulSetConnectionsEnvVars(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Connections = &paperclipv1alpha1.ConnectionsSpec{
		CredentialsSecretRef: corev1.LocalObjectReference{Name: "oauth-creds"},
	}
	sts := BuildStatefulSet(instance, nil)
	container := sts.Spec.Template.Spec.Containers[0]

	var found bool
	for _, env := range container.Env {
		if env.Name == EnvOAuthCredentials {
			found = true
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Fatal("expected SecretKeyRef for PAPERCLIP_OAUTH_CREDENTIALS")
			}
			if env.ValueFrom.SecretKeyRef.Name != "oauth-creds" {
				t.Errorf("expected secret name 'oauth-creds', got %q", env.ValueFrom.SecretKeyRef.Name)
			}
			if env.ValueFrom.SecretKeyRef.Key != EnvOAuthCredentials {
				t.Errorf("expected default key 'PAPERCLIP_OAUTH_CREDENTIALS', got %q", env.ValueFrom.SecretKeyRef.Key)
			}
		}
	}
	if !found {
		t.Error("expected PAPERCLIP_OAUTH_CREDENTIALS env var")
	}
}

func TestBuildStatefulSetConnectionsCustomKey(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Connections = &paperclipv1alpha1.ConnectionsSpec{
		CredentialsSecretRef: corev1.LocalObjectReference{Name: "oauth-creds"},
		CredentialsKey:       "custom-key",
	}
	sts := BuildStatefulSet(instance, nil)
	container := sts.Spec.Template.Spec.Containers[0]

	for _, env := range container.Env {
		if env.Name == EnvOAuthCredentials {
			if env.ValueFrom.SecretKeyRef.Key != "custom-key" {
				t.Errorf("expected key 'custom-key', got %q", env.ValueFrom.SecretKeyRef.Key)
			}
			return
		}
	}
	t.Error("expected PAPERCLIP_OAUTH_CREDENTIALS env var")
}

func TestBuildStatefulSetConnectionsWithProvidersCatalog(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Connections = &paperclipv1alpha1.ConnectionsSpec{
		CredentialsSecretRef: corev1.LocalObjectReference{Name: "oauth-creds"},
		ProvidersConfigRef:   &corev1.LocalObjectReference{Name: "custom-providers"},
	}
	sts := BuildStatefulSet(instance, nil)
	container := sts.Spec.Template.Spec.Containers[0]

	var foundCreds, foundProviders bool
	for _, env := range container.Env {
		if env.Name == EnvOAuthCredentials {
			foundCreds = true
		}
		if env.Name == EnvOAuthProviders {
			foundProviders = true
			if env.ValueFrom == nil || env.ValueFrom.ConfigMapKeyRef == nil {
				t.Fatal("expected ConfigMapKeyRef for PAPERCLIP_OAUTH_PROVIDERS")
			}
			if env.ValueFrom.ConfigMapKeyRef.Name != "custom-providers" {
				t.Errorf("expected configmap name 'custom-providers', got %q", env.ValueFrom.ConfigMapKeyRef.Name)
			}
		}
	}
	if !foundCreds {
		t.Error("expected PAPERCLIP_OAUTH_CREDENTIALS env var")
	}
	if !foundProviders {
		t.Error("expected PAPERCLIP_OAUTH_PROVIDERS env var")
	}
}

func TestBuildStatefulSetNoConnections(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	// Connections is nil by default
	sts := BuildStatefulSet(instance, nil)
	container := sts.Spec.Template.Spec.Containers[0]

	for _, env := range container.Env {
		if env.Name == EnvOAuthCredentials || env.Name == EnvOAuthProviders {
			t.Errorf("unexpected OAuth env var %q when connections is nil", env.Name)
		}
	}
}

func TestBuildStatefulSetAutoUpdateAnnotation(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	extraAnnotations := map[string]string{
		"paperclip.inc/resolved-digest": "sha256:abc123",
	}
	sts := BuildStatefulSet(instance, extraAnnotations)

	got := sts.Spec.Template.Annotations["paperclip.inc/resolved-digest"]
	if got != "sha256:abc123" {
		t.Errorf("expected digest annotation 'sha256:abc123', got %q", got)
	}
}

func TestBuildStatefulSetCloudSandboxEnvVars(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Adapters.CloudSandbox = &paperclipv1alpha1.CloudSandboxSpec{
		Enabled:        true,
		DefaultImage:   "ghcr.io/paperclipinc/agent-multi:v1.0",
		IdleTimeoutMin: 15,
	}
	sts := BuildStatefulSet(instance, nil)
	container := sts.Spec.Template.Spec.Containers[0]

	envMap := make(map[string]string)
	for _, env := range container.Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}

	if envMap["PAPERCLIP_CLOUD_SANDBOX_ENABLED"] != "true" {
		t.Error("expected PAPERCLIP_CLOUD_SANDBOX_ENABLED=true")
	}
	if envMap["PAPERCLIP_CLOUD_SANDBOX_NAMESPACE"] != "test-ns" {
		t.Errorf("expected namespace test-ns, got %q", envMap["PAPERCLIP_CLOUD_SANDBOX_NAMESPACE"])
	}
	if envMap["PAPERCLIP_CLOUD_SANDBOX_DEFAULT_IMAGE"] != "ghcr.io/paperclipinc/agent-multi:v1.0" {
		t.Error("expected default image")
	}
	if envMap["PAPERCLIP_CLOUD_SANDBOX_IDLE_TIMEOUT_MIN"] != "15" {
		t.Error("expected idle timeout 15")
	}
}

func TestBuildStatefulSetCloudSandboxPersistenceEnvVars(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Adapters.CloudSandbox = &paperclipv1alpha1.CloudSandboxSpec{
		Enabled: true,
		Persistence: &paperclipv1alpha1.CloudSandboxPersistenceSpec{
			Enabled:      true,
			StorageClass: "fast-ssd",
			Size:         "20Gi",
		},
	}
	sts := BuildStatefulSet(instance, nil)
	container := sts.Spec.Template.Spec.Containers[0]

	envMap := make(map[string]string)
	for _, env := range container.Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}

	if envMap["PAPERCLIP_CLOUD_SANDBOX_PERSISTENCE_ENABLED"] != "true" {
		t.Error("expected PAPERCLIP_CLOUD_SANDBOX_PERSISTENCE_ENABLED=true")
	}
	if envMap["PAPERCLIP_CLOUD_SANDBOX_PERSISTENCE_STORAGE_CLASS"] != "fast-ssd" {
		t.Errorf("expected storage class 'fast-ssd', got %q", envMap["PAPERCLIP_CLOUD_SANDBOX_PERSISTENCE_STORAGE_CLASS"])
	}
	if envMap["PAPERCLIP_CLOUD_SANDBOX_PERSISTENCE_SIZE"] != "20Gi" {
		t.Errorf("expected size '20Gi', got %q", envMap["PAPERCLIP_CLOUD_SANDBOX_PERSISTENCE_SIZE"])
	}
}

func TestBuildStatefulSetCloudSandboxMultiNamespaceEnvVars(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Adapters.CloudSandbox = &paperclipv1alpha1.CloudSandboxSpec{
		Enabled:        true,
		MultiNamespace: true,
	}
	sts := BuildStatefulSet(instance, nil)
	container := sts.Spec.Template.Spec.Containers[0]

	envMap := make(map[string]string)
	for _, env := range container.Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}

	if envMap["PAPERCLIP_CLOUD_SANDBOX_MULTI_NAMESPACE"] != "true" {
		t.Error("expected PAPERCLIP_CLOUD_SANDBOX_MULTI_NAMESPACE=true")
	}
}

func TestBuildStatefulSetCloudSandboxNoPersistenceEnvVars(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Adapters.CloudSandbox = &paperclipv1alpha1.CloudSandboxSpec{
		Enabled: true,
	}
	sts := BuildStatefulSet(instance, nil)
	container := sts.Spec.Template.Spec.Containers[0]

	for _, env := range container.Env {
		if env.Name == "PAPERCLIP_CLOUD_SANDBOX_PERSISTENCE_ENABLED" ||
			env.Name == "PAPERCLIP_CLOUD_SANDBOX_PERSISTENCE_STORAGE_CLASS" ||
			env.Name == "PAPERCLIP_CLOUD_SANDBOX_PERSISTENCE_SIZE" ||
			env.Name == "PAPERCLIP_CLOUD_SANDBOX_MULTI_NAMESPACE" {
			t.Errorf("unexpected env var %q when persistence/multi-namespace not configured", env.Name)
		}
	}
}

func TestBuildStatefulSetNoCloudSandbox(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	sts := BuildStatefulSet(instance, nil)
	container := sts.Spec.Template.Spec.Containers[0]

	for _, env := range container.Env {
		if env.Name == "PAPERCLIP_CLOUD_SANDBOX_ENABLED" {
			t.Error("unexpected cloud sandbox env var when not configured")
		}
	}
}

func TestBuildStatefulSetCloudSandboxSchedulingEnvVars(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Adapters.CloudSandbox = &paperclipv1alpha1.CloudSandboxSpec{
		Enabled: true,
	}
	instance.Spec.Availability.NodeSelector = map[string]string{
		"cloud.google.com/gke-nodepool": "sandbox",
	}
	instance.Spec.Availability.Tolerations = []corev1.Toleration{
		{
			Key:      "sandbox",
			Operator: corev1.TolerationOpEqual,
			Value:    "true",
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}

	sts := BuildStatefulSet(instance, nil)
	container := sts.Spec.Template.Spec.Containers[0]

	envMap := make(map[string]string)
	for _, env := range container.Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}

	// Verify nodeSelector env var
	nsVal, ok := envMap["PAPERCLIP_CLOUD_SANDBOX_NODE_SELECTOR"]
	if !ok {
		t.Fatal("expected PAPERCLIP_CLOUD_SANDBOX_NODE_SELECTOR to be set")
	}
	if nsVal != `{"cloud.google.com/gke-nodepool":"sandbox"}` {
		t.Errorf("unexpected nodeSelector JSON: %s", nsVal)
	}

	// Verify tolerations env var
	tolVal, ok := envMap["PAPERCLIP_CLOUD_SANDBOX_TOLERATIONS"]
	if !ok {
		t.Fatal("expected PAPERCLIP_CLOUD_SANDBOX_TOLERATIONS to be set")
	}
	if tolVal != `[{"key":"sandbox","operator":"Equal","value":"true","effect":"NoSchedule"}]` {
		t.Errorf("unexpected tolerations JSON: %s", tolVal)
	}

	// Verify these are NOT set when availability scheduling is empty
	instance2 := newTestInstance("my-paperclip-2")
	instance2.Spec.Adapters.CloudSandbox = &paperclipv1alpha1.CloudSandboxSpec{
		Enabled: true,
	}
	sts2 := BuildStatefulSet(instance2, nil)
	container2 := sts2.Spec.Template.Spec.Containers[0]
	for _, env := range container2.Env {
		if env.Name == "PAPERCLIP_CLOUD_SANDBOX_NODE_SELECTOR" {
			t.Error("unexpected PAPERCLIP_CLOUD_SANDBOX_NODE_SELECTOR when nodeSelector is empty")
		}
		if env.Name == "PAPERCLIP_CLOUD_SANDBOX_TOLERATIONS" {
			t.Error("unexpected PAPERCLIP_CLOUD_SANDBOX_TOLERATIONS when tolerations is empty")
		}
	}
}

func TestBuildSandboxRole(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	role := BuildSandboxRole(instance, "test-ns")

	if role.Name != "my-paperclip-sandbox" {
		t.Errorf("expected name my-paperclip-sandbox, got %q", role.Name)
	}
	if role.Namespace != "test-ns" {
		t.Errorf("expected namespace test-ns, got %q", role.Namespace)
	}
	if len(role.Rules) != 4 {
		t.Errorf("expected 4 rules (pods, pods/exec, pods/log, networkpolicies), got %d", len(role.Rules))
	}
	// Verify pods rule has all required verbs
	podsRule := role.Rules[0]
	expectedVerbs := []string{"create", "get", "list", "watch", "delete", "patch"}
	if len(podsRule.Verbs) != len(expectedVerbs) {
		t.Errorf("expected %d verbs for pods, got %d", len(expectedVerbs), len(podsRule.Verbs))
	}
}

func TestBuildSandboxRolePersistencePVC(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Adapters.CloudSandbox = &paperclipv1alpha1.CloudSandboxSpec{
		Enabled: true,
		Persistence: &paperclipv1alpha1.CloudSandboxPersistenceSpec{
			Enabled:      true,
			StorageClass: "fast-ssd",
			Size:         "20Gi",
		},
	}
	role := BuildSandboxRole(instance, "test-ns")

	if len(role.Rules) != 5 {
		t.Errorf("expected 5 rules (pods, pods/exec, pods/log, networkpolicies, pvcs), got %d", len(role.Rules))
	}
	// The PVC rule should be the last one
	pvcRule := role.Rules[4]
	if len(pvcRule.Resources) != 1 || pvcRule.Resources[0] != "persistentvolumeclaims" {
		t.Errorf("expected PVC resource, got %v", pvcRule.Resources)
	}
	expectedVerbs := []string{"create", "get", "list", "delete"}
	if len(pvcRule.Verbs) != len(expectedVerbs) {
		t.Errorf("expected %d verbs for PVCs, got %d", len(expectedVerbs), len(pvcRule.Verbs))
	}
}

func TestBuildSandboxRoleNoPersistence(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Adapters.CloudSandbox = &paperclipv1alpha1.CloudSandboxSpec{
		Enabled: true,
	}
	role := BuildSandboxRole(instance, "test-ns")

	// Should have only the base 4 rules without PVC
	if len(role.Rules) != 4 {
		t.Errorf("expected 4 rules without persistence, got %d", len(role.Rules))
	}
	for _, rule := range role.Rules {
		for _, res := range rule.Resources {
			if res == "persistentvolumeclaims" {
				t.Error("unexpected PVC rule when persistence is not enabled")
			}
		}
	}
}

func TestBuildSandboxClusterRole(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Adapters.CloudSandbox = &paperclipv1alpha1.CloudSandboxSpec{
		Enabled:        true,
		MultiNamespace: true,
	}
	cr := BuildSandboxClusterRole(instance)

	if cr.Name != "test-ns-my-paperclip-sandbox" {
		t.Errorf("expected name test-ns-my-paperclip-sandbox, got %q", cr.Name)
	}
	// Should have base 4 rules + namespaces + services/serviceaccounts + roles/rolebindings = 7
	if len(cr.Rules) != 7 {
		t.Errorf("expected 7 rules (pods, pods/exec, pods/log, networkpolicies, namespaces, services/serviceaccounts, roles/rolebindings), got %d", len(cr.Rules))
	}
	// Namespace rule
	nsRule := cr.Rules[4]
	if len(nsRule.Resources) != 1 || nsRule.Resources[0] != "namespaces" {
		t.Errorf("expected namespaces resource, got %v", nsRule.Resources)
	}
	expectedVerbs := []string{"create", "get", "list", "delete"}
	if len(nsRule.Verbs) != len(expectedVerbs) {
		t.Errorf("expected %d verbs for namespaces, got %d", len(expectedVerbs), len(nsRule.Verbs))
	}
}

func TestBuildSandboxClusterRoleWithPersistence(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Adapters.CloudSandbox = &paperclipv1alpha1.CloudSandboxSpec{
		Enabled:        true,
		MultiNamespace: true,
		Persistence: &paperclipv1alpha1.CloudSandboxPersistenceSpec{
			Enabled: true,
		},
	}
	cr := BuildSandboxClusterRole(instance)

	// Should have base 4 rules + PVC rule + namespaces + services/serviceaccounts + roles/rolebindings = 8
	if len(cr.Rules) != 8 {
		t.Errorf("expected 8 rules, got %d", len(cr.Rules))
	}
}

func TestBuildSandboxClusterRoleBinding(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Adapters.CloudSandbox = &paperclipv1alpha1.CloudSandboxSpec{
		Enabled:        true,
		MultiNamespace: true,
	}
	binding := BuildSandboxClusterRoleBinding(instance)

	if binding.Name != "test-ns-my-paperclip-sandbox" {
		t.Errorf("expected name test-ns-my-paperclip-sandbox, got %q", binding.Name)
	}
	if binding.RoleRef.Kind != "ClusterRole" {
		t.Errorf("expected ClusterRole kind, got %q", binding.RoleRef.Kind)
	}
	if binding.RoleRef.Name != "test-ns-my-paperclip-sandbox" {
		t.Error("expected role ref to match sandbox cluster role name")
	}
	if len(binding.Subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(binding.Subjects))
	}
	if binding.Subjects[0].Name != "my-paperclip" {
		t.Error("expected subject to be instance service account")
	}
	if binding.Subjects[0].Namespace != "test-ns" {
		t.Errorf("expected subject namespace test-ns, got %q", binding.Subjects[0].Namespace)
	}
}

func TestBuildSandboxRoleBinding(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	binding := BuildSandboxRoleBinding(instance, "test-ns")

	if binding.Name != "my-paperclip-sandbox" {
		t.Errorf("expected name my-paperclip-sandbox, got %q", binding.Name)
	}
	if binding.RoleRef.Name != "my-paperclip-sandbox" {
		t.Error("expected role ref to match sandbox role name")
	}
	if len(binding.Subjects) != 1 {
		t.Fatalf("expected 1 subject, got %d", len(binding.Subjects))
	}
	if binding.Subjects[0].Name != "my-paperclip" {
		t.Error("expected subject to be instance service account")
	}
}

func TestLabels(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	labels := Labels(instance)

	if labels[LabelApp] != AppName {
		t.Errorf("expected app label %q, got %q", AppName, labels[LabelApp])
	}
	if labels[LabelInstance] != "my-paperclip" {
		t.Errorf("expected instance label 'my-paperclip', got %q", labels[LabelInstance])
	}
	if labels[LabelManagedBy] != ManagedBy {
		t.Errorf("expected managed-by label %q, got %q", ManagedBy, labels[LabelManagedBy])
	}
}

func TestBuildSecretsMasterKeySecret(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	secret := BuildSecretsMasterKeySecret(instance, "test-master-key-value")

	if secret.Name != "my-paperclip-secrets-master-key" {
		t.Errorf("expected name 'my-paperclip-secrets-master-key', got %q", secret.Name)
	}
	if secret.Namespace != "test-ns" {
		t.Errorf("expected namespace 'test-ns', got %q", secret.Namespace)
	}
	if secret.Type != corev1.SecretTypeOpaque {
		t.Errorf("expected Opaque type, got %s", secret.Type)
	}
	if secret.Labels[LabelApp] != AppName {
		t.Errorf("expected app label %q, got %q", AppName, secret.Labels[LabelApp])
	}
	if secret.Labels[LabelManagedBy] != ManagedBy {
		t.Errorf("expected managed-by label %q, got %q", ManagedBy, secret.Labels[LabelManagedBy])
	}
	val, ok := secret.Data["master-key"]
	if !ok {
		t.Fatal("expected 'master-key' key in secret data")
	}
	if string(val) != "test-master-key-value" {
		t.Errorf("expected master key value 'test-master-key-value', got %q", string(val))
	}
}

func TestBuildSandboxClusterRoleMultiNamespacePermissions(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Adapters.CloudSandbox = &paperclipv1alpha1.CloudSandboxSpec{
		Enabled:        true,
		MultiNamespace: true,
	}
	cr := BuildSandboxClusterRole(instance)

	// Collect all resources across rules into a map for easy lookup
	resourceVerbs := make(map[string][]string)
	for _, rule := range cr.Rules {
		for _, res := range rule.Resources {
			resourceVerbs[res] = rule.Verbs
		}
	}

	// Verify namespaces rule includes "delete"
	nsVerbs, ok := resourceVerbs["namespaces"]
	if !ok {
		t.Fatal("expected namespaces resource in ClusterRole rules")
	}
	foundDelete := false
	for _, v := range nsVerbs {
		if v == "delete" {
			foundDelete = true
		}
	}
	if !foundDelete {
		t.Error("expected 'delete' verb for namespaces resource")
	}

	// Verify services resource
	if _, ok := resourceVerbs["services"]; !ok {
		t.Error("expected services resource in ClusterRole rules")
	}

	// Verify serviceaccounts resource
	if _, ok := resourceVerbs["serviceaccounts"]; !ok {
		t.Error("expected serviceaccounts resource in ClusterRole rules")
	}

	// Verify roles resource
	if _, ok := resourceVerbs["roles"]; !ok {
		t.Error("expected roles resource in ClusterRole rules")
	}

	// Verify rolebindings resource
	if _, ok := resourceVerbs["rolebindings"]; !ok {
		t.Error("expected rolebindings resource in ClusterRole rules")
	}
}

func TestBuildStatefulSetAutoGeneratedSecretsMasterKey(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	// masterKeySecretRef is nil by default - should use auto-generated secret
	sts := BuildStatefulSet(instance, nil)
	container := sts.Spec.Template.Spec.Containers[0]

	var found bool
	for _, env := range container.Env {
		if env.Name == "PAPERCLIP_SECRETS_MASTER_KEY" {
			found = true
			if env.ValueFrom == nil || env.ValueFrom.SecretKeyRef == nil {
				t.Fatal("expected SecretKeyRef for PAPERCLIP_SECRETS_MASTER_KEY")
			}
			if env.ValueFrom.SecretKeyRef.Name != "my-paperclip-secrets-master-key" {
				t.Errorf("expected auto-generated secret name 'my-paperclip-secrets-master-key', got %q",
					env.ValueFrom.SecretKeyRef.Name)
			}
			if env.ValueFrom.SecretKeyRef.Key != "master-key" {
				t.Errorf("expected key 'master-key', got %q", env.ValueFrom.SecretKeyRef.Key)
			}
		}
	}
	if !found {
		t.Error("expected PAPERCLIP_SECRETS_MASTER_KEY env var from auto-generated secret")
	}
}

func TestNamingConventions(t *testing.T) {
	instance := newTestInstance("my-paperclip")

	tests := []struct {
		name     string
		fn       func(*paperclipv1alpha1.Instance) string
		expected string
	}{
		{"StatefulSetName", StatefulSetName, "my-paperclip"},
		{"ServiceName", ServiceName, "my-paperclip"},
		{"ConfigMapName", ConfigMapName, "my-paperclip-config"},
		{"PVCName", PVCName, "my-paperclip-data"},
		{"IngressName", IngressName, "my-paperclip"},
		{"ServiceAccountName", ServiceAccountName, "my-paperclip"},
		{"NetworkPolicyName", NetworkPolicyName, "my-paperclip"},
		{"DatabaseStatefulSetName", DatabaseStatefulSetName, "my-paperclip-db"},
		{"DatabaseServiceName", DatabaseServiceName, "my-paperclip-db"},
		{"DatabasePVCName", DatabasePVCName, "my-paperclip-db-data"},
		{"HPAName", HPAName, "my-paperclip"},
		{"PDBName", PDBName, "my-paperclip"},
		{"DatabaseSecretName", DatabaseSecretName, "my-paperclip-db-credentials"},
		{"SecretsMasterKeySecretName", SecretsMasterKeySecretName, "my-paperclip-secrets-master-key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(instance)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestContainerSecurityContextOverrideAppliesToAllPaperclipContainers(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Security.ContainerSecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: Ptr(false),
		RunAsNonRoot:             Ptr(false),
		RunAsUser:                Ptr(int64(0)),
		SeccompProfile: &corev1.SeccompProfile{
			Type: corev1.SeccompProfileTypeRuntimeDefault,
		},
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
			Add:  []corev1.Capability{"SETUID", "SETGID"},
		},
	}

	// Main container
	sts := BuildStatefulSet(instance, nil)
	mainContainer := sts.Spec.Template.Spec.Containers[0]
	if *mainContainer.SecurityContext.RunAsNonRoot != false {
		t.Error("main container: expected RunAsNonRoot=false from CRD override")
	}
	if *mainContainer.SecurityContext.RunAsUser != 0 {
		t.Error("main container: expected RunAsUser=0 from CRD override")
	}

	// Onboard init container - should also use the CRD override
	var onboard *corev1.Container
	for i := range sts.Spec.Template.Spec.InitContainers {
		if sts.Spec.Template.Spec.InitContainers[i].Name == "onboard" {
			onboard = &sts.Spec.Template.Spec.InitContainers[i]
			break
		}
	}
	if onboard == nil {
		t.Fatal("expected onboard init container")
	}
	if *onboard.SecurityContext.RunAsNonRoot != false {
		t.Error("onboard: expected RunAsNonRoot=false from CRD override")
	}

	// Bootstrap job
	instance.Spec.Auth.AdminUser = &paperclipv1alpha1.AdminUserSpec{
		Email: "admin@test.com",
		PasswordSecretRef: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "admin-secret"},
			Key:                  "password",
		},
	}
	job := BuildBootstrapJob(instance)
	bootstrapContainer := job.Spec.Template.Spec.Containers[0]
	if *bootstrapContainer.SecurityContext.RunAsNonRoot != false {
		t.Error("bootstrap: expected RunAsNonRoot=false from CRD override")
	}
}

func TestDefaultSecurityContextOnOnboardAndBootstrap(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	// No CRD override - should get restricted PSS defaults

	sts := BuildStatefulSet(instance, nil)
	var onboard *corev1.Container
	for i := range sts.Spec.Template.Spec.InitContainers {
		if sts.Spec.Template.Spec.InitContainers[i].Name == "onboard" {
			onboard = &sts.Spec.Template.Spec.InitContainers[i]
			break
		}
	}
	if onboard == nil {
		t.Fatal("expected onboard init container")
	}
	sc := onboard.SecurityContext
	if *sc.RunAsNonRoot != true {
		t.Error("onboard: expected default RunAsNonRoot=true")
	}
	if *sc.AllowPrivilegeEscalation != false {
		t.Error("onboard: expected default AllowPrivilegeEscalation=false")
	}
	if sc.Capabilities == nil || sc.Capabilities.Drop[0] != "ALL" {
		t.Error("onboard: expected default drop ALL capabilities")
	}

	instance.Spec.Auth.AdminUser = &paperclipv1alpha1.AdminUserSpec{
		Email: "admin@test.com",
		PasswordSecretRef: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "admin-secret"},
			Key:                  "password",
		},
	}
	job := BuildBootstrapJob(instance)
	bsc := job.Spec.Template.Spec.Containers[0].SecurityContext
	if *bsc.RunAsNonRoot != true {
		t.Error("bootstrap: expected default RunAsNonRoot=true")
	}
	if *bsc.AllowPrivilegeEscalation != false {
		t.Error("bootstrap: expected default AllowPrivilegeEscalation=false")
	}
}

func TestBuildStatefulSetShareProcessNamespace(t *testing.T) {
	// Default: nil spec value yields ShareProcessNamespace=true for zombie reaping.
	instance := newTestInstance("share-default")
	sts := BuildStatefulSet(instance, nil)
	if sps := sts.Spec.Template.Spec.ShareProcessNamespace; sps == nil || *sps != true {
		t.Fatalf("expected default ShareProcessNamespace=true, got %v", sps)
	}

	// Opt-out: explicit false is honored.
	instance.Spec.ShareProcessNamespace = Ptr(false)
	sts = BuildStatefulSet(instance, nil)
	if sps := sts.Spec.Template.Spec.ShareProcessNamespace; sps == nil || *sps != false {
		t.Fatalf("expected ShareProcessNamespace=false when opted out, got %v", sps)
	}

	// Explicit true is honored.
	instance.Spec.ShareProcessNamespace = Ptr(true)
	sts = BuildStatefulSet(instance, nil)
	if sps := sts.Spec.Template.Spec.ShareProcessNamespace; sps == nil || *sps != true {
		t.Fatalf("expected ShareProcessNamespace=true, got %v", sps)
	}
}

func TestBuildStatefulSetSuspended(t *testing.T) {
	instance := newTestInstance("suspend")
	instance.Spec.Availability.Replicas = Ptr(int32(3))

	// Not suspended: replicas reflect the effective count.
	sts := BuildStatefulSet(instance, nil)
	if sts.Spec.Replicas == nil || *sts.Spec.Replicas != 3 {
		t.Fatalf("expected 3 replicas when not suspended, got %v", sts.Spec.Replicas)
	}

	// Suspended: replicas forced to 0 (scale-to-zero).
	instance.Spec.Suspended = true
	sts = BuildStatefulSet(instance, nil)
	if sts.Spec.Replicas == nil || *sts.Spec.Replicas != 0 {
		t.Fatalf("expected 0 replicas when suspended, got %v", sts.Spec.Replicas)
	}
}

func TestStatefulSetReplicas(t *testing.T) {
	instance := newTestInstance("repl")
	if got := StatefulSetReplicas(instance); got != 1 {
		t.Errorf("expected default 1 replica, got %d", got)
	}
	instance.Spec.Availability.Replicas = Ptr(int32(5))
	if got := StatefulSetReplicas(instance); got != 5 {
		t.Errorf("expected 5 replicas, got %d", got)
	}
	instance.Spec.Suspended = true
	if got := StatefulSetReplicas(instance); got != 0 {
		t.Errorf("expected 0 replicas when suspended, got %d", got)
	}
}
