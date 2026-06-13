package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// newBenchInstance creates a minimal instance for benchmarking.
func newBenchInstance() *paperclipv1alpha1.Instance {
	return &paperclipv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bench",
			Namespace: "bench-ns",
		},
		Spec: paperclipv1alpha1.InstanceSpec{
			Image: paperclipv1alpha1.ImageSpec{
				Repository: "ghcr.io/paperclipai/paperclip",
				Tag:        "v1.0.0",
			},
			Database: paperclipv1alpha1.DatabaseSpec{Mode: "managed"},
		},
	}
}

// newFullBenchInstance creates a fully-loaded instance for benchmarking.
func newFullBenchInstance() *paperclipv1alpha1.Instance {
	return &paperclipv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bench-full",
			Namespace: "bench-ns",
		},
		Spec: paperclipv1alpha1.InstanceSpec{
			Image: paperclipv1alpha1.ImageSpec{
				Repository: "ghcr.io/paperclipai/paperclip",
				Tag:        "v1.0.0",
			},
			Deployment: paperclipv1alpha1.DeploymentSpec{
				Mode:      "authenticated",
				Exposure:  "public",
				PublicURL: "https://bench.example.com",
			},
			Database: paperclipv1alpha1.DatabaseSpec{Mode: "managed"},
			Storage: paperclipv1alpha1.StorageSpec{
				Persistence: paperclipv1alpha1.PersistenceSpec{
					Enabled: Ptr(true),
					Size:    resource.MustParse("10Gi"),
				},
			},
			Heartbeat: paperclipv1alpha1.HeartbeatSpec{Enabled: true, IntervalMS: 60000},
			Env: []corev1.EnvVar{
				{Name: "ENV1", Value: "val1"},
				{Name: "ENV2", Value: "val2"},
			},
			EnvFrom: []corev1.EnvFromSource{
				{SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "api-keys"},
				}},
			},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
			Availability: paperclipv1alpha1.AvailabilitySpec{
				Replicas:     Ptr(int32(3)),
				NodeSelector: map[string]string{"node-type": "compute"},
				Tolerations: []corev1.Toleration{
					{Key: "dedicated", Operator: corev1.TolerationOpEqual, Value: "true", Effect: corev1.TaintEffectNoSchedule},
				},
				PodDisruptionBudget: &paperclipv1alpha1.PDBSpec{
					Enabled:        true,
					MaxUnavailable: Ptr(int32(1)),
				},
				AutoScaling: &paperclipv1alpha1.AutoScalingSpec{
					Enabled:                        true,
					MinReplicas:                    Ptr(int32(2)),
					MaxReplicas:                    10,
					TargetCPUUtilizationPercentage: Ptr(int32(80)),
				},
			},
			Security: paperclipv1alpha1.SecuritySpec{
				RBAC: paperclipv1alpha1.RBACSpec{Create: true},
				NetworkPolicy: paperclipv1alpha1.NetworkPolicySpec{
					Enabled:           true,
					AllowIngressCIDRs: []string{"10.0.0.0/8"},
					AllowEgressCIDRs:  []string{"10.0.0.0/8"},
				},
			},
			Networking: paperclipv1alpha1.NetworkingSpec{
				Ingress: &paperclipv1alpha1.IngressSpec{
					Enabled: true,
					Hosts:   []string{"bench.example.com"},
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// StatefulSet benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBuildStatefulSet_Minimal(b *testing.B) {
	instance := newBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildStatefulSet(instance, nil)
	}
}

func BenchmarkBuildStatefulSet_FullyLoaded(b *testing.B) {
	instance := newFullBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildStatefulSet(instance, nil)
	}
}

func BenchmarkBuildDatabaseStatefulSet(b *testing.B) {
	instance := newFullBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildDatabaseStatefulSet(instance)
	}
}

// ---------------------------------------------------------------------------
// Service benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBuildService_Minimal(b *testing.B) {
	instance := newBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildService(instance)
	}
}

func BenchmarkBuildService_FullyLoaded(b *testing.B) {
	instance := newFullBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildService(instance)
	}
}

// ---------------------------------------------------------------------------
// NetworkPolicy benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBuildNetworkPolicy_Minimal(b *testing.B) {
	instance := newBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildNetworkPolicy(instance)
	}
}

func BenchmarkBuildNetworkPolicy_FullyLoaded(b *testing.B) {
	instance := newFullBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildNetworkPolicy(instance)
	}
}

// ---------------------------------------------------------------------------
// Ingress benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBuildIngress(b *testing.B) {
	instance := newFullBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildIngress(instance)
	}
}

// ---------------------------------------------------------------------------
// PDB benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBuildPodDisruptionBudget(b *testing.B) {
	instance := newFullBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildPodDisruptionBudget(instance)
	}
}

// ---------------------------------------------------------------------------
// HPA benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBuildHorizontalPodAutoscaler(b *testing.B) {
	instance := newFullBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildHorizontalPodAutoscaler(instance)
	}
}

// ---------------------------------------------------------------------------
// PVC benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBuildPersistentVolumeClaim(b *testing.B) {
	instance := newFullBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildPersistentVolumeClaim(instance)
	}
}

// ---------------------------------------------------------------------------
// ServiceAccount benchmarks
// ---------------------------------------------------------------------------

func BenchmarkBuildServiceAccount(b *testing.B) {
	instance := newFullBenchInstance()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BuildServiceAccount(instance)
	}
}
