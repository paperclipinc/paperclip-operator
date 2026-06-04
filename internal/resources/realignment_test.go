package resources

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// containerEnv returns the env slice of the primary paperclip container.
func containerEnv(inst *paperclipv1alpha1.Instance) []corev1.EnvVar {
	return BuildStatefulSet(inst, nil).Spec.Template.Spec.Containers[0].Env
}

func envValue(env []corev1.EnvVar, name string) (string, bool) {
	for _, e := range env {
		if e.Name == name {
			return e.Value, true
		}
	}
	return "", false
}

func envEntry(env []corev1.EnvVar, name string) (corev1.EnvVar, bool) {
	for _, e := range env {
		if e.Name == name {
			return e, true
		}
	}
	return corev1.EnvVar{}, false
}

func hasEnvName(env []corev1.EnvVar, name string) bool {
	_, ok := envEntry(env, name)
	return ok
}

func wantEnvValue(t *testing.T, env []corev1.EnvVar, name, want string) {
	t.Helper()
	got, ok := envValue(env, name)
	if !ok {
		t.Errorf("expected env %s to be set", name)
		return
	}
	if got != want {
		t.Errorf("env %s = %q, want %q", name, got, want)
	}
}

// --- Task 1: deployment mode + disableSignUp + probe selection ---

func TestUseTCPProbes_ModeSelection(t *testing.T) {
	cases := []struct {
		mode string
		want bool
	}{
		{"authenticated", true},
		{"local_trusted", false},
	}
	for _, tc := range cases {
		inst := newTestInstance("p")
		inst.Spec.Deployment.Mode = tc.mode
		if tc.mode == "local_trusted" {
			inst.Spec.Deployment.Exposure = "private"
		}
		if got := UseTCPProbes(inst); got != tc.want {
			t.Errorf("mode %q: UseTCPProbes=%v want %v", tc.mode, got, tc.want)
		}
	}
}

func TestBuildStatefulSet_DisableSignUpEnv(t *testing.T) {
	inst := newTestInstance("p")
	inst.Spec.Deployment.Mode = "authenticated"
	inst.Spec.Auth.DisableSignUp = true
	env := containerEnv(inst)
	wantEnvValue(t, env, "PAPERCLIP_AUTH_DISABLE_SIGN_UP", "true")
	wantEnvValue(t, env, "PAPERCLIP_DEPLOYMENT_MODE", "authenticated")
}

func TestBuildStatefulSet_DisableSignUpOmittedByDefault(t *testing.T) {
	inst := newTestInstance("p")
	if hasEnvName(containerEnv(inst), "PAPERCLIP_AUTH_DISABLE_SIGN_UP") {
		t.Error("PAPERCLIP_AUTH_DISABLE_SIGN_UP must be omitted when DisableSignUp is false")
	}
}
