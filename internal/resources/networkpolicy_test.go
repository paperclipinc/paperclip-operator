package resources

import (
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

func netpolTestInstance() *paperclipv1alpha1.Instance {
	return &paperclipv1alpha1.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "np", Namespace: "np-ns"},
	}
}

func TestNetworkPolicyExtraEgress(t *testing.T) {
	inst := netpolTestInstance()
	inst.Spec.Security.NetworkPolicy.ExtraEgress = []networkingv1.NetworkPolicyEgressRule{
		{To: []networkingv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "minio"}}}}},
	}
	np := BuildNetworkPolicy(inst)
	last := np.Spec.Egress[len(np.Spec.Egress)-1]
	if len(last.To) != 1 || last.To[0].PodSelector == nil || last.To[0].PodSelector.MatchLabels["app"] != "minio" {
		t.Fatalf("extraEgress rule not appended verbatim: %+v", last)
	}
}

func TestNetworkPolicyEnabledPointerSemantics(t *testing.T) {
	inst := netpolTestInstance()
	if !NetworkPolicyEnabled(inst) {
		t.Fatal("nil Enabled must default to true")
	}
	f := false
	inst.Spec.Security.NetworkPolicy.Enabled = &f
	if NetworkPolicyEnabled(inst) {
		t.Fatal("explicit false must disable")
	}
}

func TestObjectStorageForcePathStyle(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		explicit *bool
		want     bool
	}{
		{name: "minio defaults to path style", provider: "minio", want: true},
		{name: "s3 defaults to virtual-hosted", provider: "s3", want: false},
		{name: "explicit false wins over minio default", provider: "minio", explicit: boolPtr(false), want: false},
		{name: "explicit true wins for s3", provider: "s3", explicit: boolPtr(true), want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inst := netpolTestInstance()
			inst.Spec.ObjectStorage = &paperclipv1alpha1.ObjectStorageSpec{
				Provider:       tc.provider,
				Bucket:         "b",
				ForcePathStyle: tc.explicit,
			}
			tmpl := BuildServerPodTemplate(inst, nil)
			got := false
			for _, env := range tmpl.Spec.Containers[0].Env {
				if env.Name == "PAPERCLIP_STORAGE_S3_FORCE_PATH_STYLE" && env.Value == "true" {
					got = true
				}
			}
			if got != tc.want {
				t.Fatalf("forcePathStyle env: got %v want %v", got, tc.want)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }
