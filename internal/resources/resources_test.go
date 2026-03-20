package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

//nolint:unparam // test helper kept flexible for future test cases
func newTestInstance(name string) *paperclipv1alpha1.Instance {
	return &paperclipv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "test-ns",
		},
		Spec: paperclipv1alpha1.InstanceSpec{
			Image: paperclipv1alpha1.ImageSpec{
				Repository: "ghcr.io/paperclipinc/paperclip",
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
	sts := BuildStatefulSet(instance)

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
	if container.Image != "ghcr.io/paperclipinc/paperclip:v1.0.0" {
		t.Errorf("expected image 'ghcr.io/paperclipinc/paperclip:v1.0.0', got %q", container.Image)
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

	// Verify health check path
	if container.LivenessProbe.HTTPGet.Path != HealthPath {
		t.Errorf("expected liveness probe path %q, got %q", HealthPath, container.LivenessProbe.HTTPGet.Path)
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
	sts := BuildStatefulSet(instance)

	container := sts.Spec.Template.Spec.Containers[0]
	expected := "ghcr.io/paperclipinc/paperclip@sha256:abc123"
	if container.Image != expected {
		t.Errorf("expected image %q, got %q", expected, container.Image)
	}
}

func TestBuildStatefulSetEnvVars(t *testing.T) {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Deployment.PublicURL = "https://paperclip.example.com"
	instance.Spec.Deployment.AllowedHostnames = []string{"paperclip.example.com"}
	sts := BuildStatefulSet(instance)

	container := sts.Spec.Template.Spec.Containers[0]

	envMap := make(map[string]string)
	for _, env := range container.Env {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		}
	}

	if envMap["HOST"] != "0.0.0.0" {
		t.Error("expected HOST=0.0.0.0")
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
	if envMap["PAPERCLIP_ALLOWED_HOSTNAMES"] != "paperclip.example.com" {
		t.Error("expected PAPERCLIP_ALLOWED_HOSTNAMES=paperclip.example.com")
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
