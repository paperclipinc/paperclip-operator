package resources

import (
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// statelessTestInstance returns a test instance configured for the Deployment
// workload: external database, no persistence.
func statelessTestInstance() *paperclipv1alpha1.Instance {
	instance := newTestInstance("my-paperclip")
	instance.Spec.Workload = "Deployment"
	instance.Spec.Storage.Persistence.Enabled = false
	instance.Spec.Database.Mode = "external"
	instance.Spec.Database.ExternalURL = "postgresql://user:pass@db.example.com:5432/paperclip"
	return instance
}

func TestBuildDeployment(t *testing.T) {
	instance := statelessTestInstance()
	deploy := BuildDeployment(instance, nil)

	if deploy.Name != "my-paperclip" {
		t.Errorf("expected Deployment name 'my-paperclip', got %q", deploy.Name)
	}
	if deploy.Namespace != "test-ns" {
		t.Errorf("expected namespace 'test-ns', got %q", deploy.Namespace)
	}
	if deploy.Spec.Replicas == nil || *deploy.Spec.Replicas != 1 {
		t.Errorf("expected 1 replica, got %v", deploy.Spec.Replicas)
	}

	// Surge rollout strategy: never drop below desired capacity.
	if deploy.Spec.Strategy.Type != appsv1.RollingUpdateDeploymentStrategyType {
		t.Errorf("expected RollingUpdate strategy, got %q", deploy.Spec.Strategy.Type)
	}
	ru := deploy.Spec.Strategy.RollingUpdate
	if ru == nil {
		t.Fatal("expected RollingUpdate strategy parameters")
	}
	if ru.MaxSurge == nil || ru.MaxSurge.IntValue() != 1 {
		t.Errorf("expected maxSurge=1, got %v", ru.MaxSurge)
	}
	if ru.MaxUnavailable == nil || ru.MaxUnavailable.IntValue() != 0 {
		t.Errorf("expected maxUnavailable=0, got %v", ru.MaxUnavailable)
	}

	if len(deploy.Spec.Template.Spec.Containers) < 1 {
		t.Fatal("expected at least 1 container")
	}
	if deploy.Spec.Template.Spec.Containers[0].Name != ContainerName {
		t.Errorf("expected container name %q, got %q", ContainerName, deploy.Spec.Template.Spec.Containers[0].Name)
	}
}

func TestBuildDeploymentNameMatchesStatefulSet(t *testing.T) {
	instance := statelessTestInstance()
	deploy := BuildDeployment(instance, nil)
	sts := BuildStatefulSet(instance, nil)

	if deploy.Name != sts.Name {
		t.Errorf("expected Deployment name %q to equal StatefulSet name %q", deploy.Name, sts.Name)
	}
}

func TestBuildDeploymentSelectorMatchesService(t *testing.T) {
	instance := statelessTestInstance()
	deploy := BuildDeployment(instance, nil)
	svc := BuildService(instance)

	if !reflect.DeepEqual(deploy.Spec.Selector.MatchLabels, svc.Spec.Selector) {
		t.Errorf("expected Deployment selector %v to match Service selector %v",
			deploy.Spec.Selector.MatchLabels, svc.Spec.Selector)
	}
	// The pod template labels must satisfy the selector.
	for k, v := range deploy.Spec.Selector.MatchLabels {
		if deploy.Spec.Template.Labels[k] != v {
			t.Errorf("expected pod template label %s=%s, got %q", k, v, deploy.Spec.Template.Labels[k])
		}
	}
}

func TestBuildDeploymentEmptyDirVolume(t *testing.T) {
	instance := statelessTestInstance()
	deploy := BuildDeployment(instance, nil)

	var found bool
	for _, vol := range deploy.Spec.Template.Spec.Volumes {
		if vol.Name == DataVolumeName {
			found = true
			if vol.EmptyDir == nil {
				t.Error("expected emptyDir data volume when persistence is disabled")
			}
			if vol.PersistentVolumeClaim != nil {
				t.Error("expected no PVC data volume when persistence is disabled")
			}
		}
	}
	if !found {
		t.Fatalf("expected data volume %q", DataVolumeName)
	}
}

func TestBuildDeploymentSuspendedReplicas(t *testing.T) {
	instance := statelessTestInstance()
	instance.Spec.Suspended = true
	deploy := BuildDeployment(instance, nil)
	if deploy.Spec.Replicas == nil || *deploy.Spec.Replicas != 0 {
		t.Errorf("expected 0 replicas when suspended, got %v", deploy.Spec.Replicas)
	}
}

func TestUseDeploymentWorkload(t *testing.T) {
	tests := []struct {
		name        string
		workload    string
		persistence bool
		dbMode      string
		want        bool
	}{
		{"explicit StatefulSet", "StatefulSet", false, "external", false},
		{"explicit Deployment", "Deployment", false, "external", true},
		{"explicit Deployment with persistence", "Deployment", true, "external", true},
		{"empty defaults to StatefulSet", "", false, "external", false},
		{"auto no persistence external db", "auto", false, "external", true},
		{"auto no persistence managed db", "auto", false, "managed", true},
		{"auto no persistence embedded db", "auto", false, "embedded", false},
		{"auto persistence external db", "auto", true, "external", false},
		{"auto persistence managed db", "auto", true, "managed", false},
		{"auto persistence embedded db", "auto", true, "embedded", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance := newTestInstance("workload")
			instance.Spec.Workload = tt.workload
			instance.Spec.Storage.Persistence.Enabled = tt.persistence
			instance.Spec.Database.Mode = tt.dbMode
			if got := UseDeploymentWorkload(instance); got != tt.want {
				t.Errorf("UseDeploymentWorkload(workload=%q, persistence=%t, dbMode=%q) = %t, want %t",
					tt.workload, tt.persistence, tt.dbMode, got, tt.want)
			}
		})
	}
}
