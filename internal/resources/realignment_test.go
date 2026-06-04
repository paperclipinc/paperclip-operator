package resources

import (
	"strings"
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

// --- Task 2: bind modernization ---

func TestBuildStatefulSet_BindEnv(t *testing.T) {
	env := containerEnv(newTestInstance("p"))
	wantEnvValue(t, env, "PAPERCLIP_BIND", "custom")
	wantEnvValue(t, env, "PAPERCLIP_BIND_HOST", "0.0.0.0")
	if hasEnvName(env, "HOST") {
		t.Error("legacy HOST env should no longer be set")
	}
}

// --- allowed hostnames always include in-cluster Service DNS ---
// In authenticated mode the app rejects auth requests from hostnames not in its
// allowlist. The operator's own bootstrap Job calls the Service DNS, so those
// names must always be allowed or admin bootstrap fails with HTTP 403.

func TestBuildStatefulSet_AllowedHostnamesIncludesServiceDNS(t *testing.T) {
	inst := newTestInstance("p") // namespace test-ns
	inst.Spec.Deployment.AllowedHostnames = []string{"paperclip.example.com"}
	v, ok := envValue(containerEnv(inst), "PAPERCLIP_ALLOWED_HOSTNAMES")
	if !ok {
		t.Fatal("expected PAPERCLIP_ALLOWED_HOSTNAMES to always be set")
	}
	for _, want := range []string{
		"p.test-ns.svc.cluster.local", "p.test-ns", "p", "localhost", "paperclip.example.com",
	} {
		if !strings.Contains(","+v+",", ","+want+",") {
			t.Errorf("allowed hostnames %q missing %q", v, want)
		}
	}
}

func TestBuildStatefulSet_AllowedHostnamesSetWithoutUserValues(t *testing.T) {
	v, ok := envValue(containerEnv(newTestInstance("p")), "PAPERCLIP_ALLOWED_HOSTNAMES")
	if !ok || !strings.Contains(v, "p.test-ns.svc.cluster.local") {
		t.Errorf("expected Service DNS in allowed hostnames even without user values, got %q", v)
	}
}

// --- main container bypasses the image gosu entrypoint ---
// The app image ENTRYPOINT (docker-entrypoint.sh) gosu-drops from root to "node",
// which fails under runAsNonRoot/drop-ALL ("failed switching to node: operation
// not permitted"). The operator must override Command to exec node directly.

func TestBuildStatefulSet_MainContainerBypassesImageEntrypoint(t *testing.T) {
	c := BuildStatefulSet(newTestInstance("p"), nil).Spec.Template.Spec.Containers[0]
	if len(c.Command) == 0 {
		t.Fatal("main container must override Command to bypass the image gosu entrypoint")
	}
	if c.Command[0] != "/bin/sh" {
		t.Errorf("expected /bin/sh command wrapper, got %v", c.Command)
	}
	joined := strings.Join(c.Args, " ")
	if !strings.Contains(joined, "exec ") || !strings.Contains(joined, "server/dist/index.js") {
		t.Errorf("expected args to exec the node entrypoint, got %q", joined)
	}
}

func TestBuildStatefulSet_MultiReplicaKeepsHeartbeatGating(t *testing.T) {
	inst := newTestInstance("p")
	inst.Spec.Availability.Replicas = Ptr(int32(3))
	inst.Spec.Heartbeat.Enabled = true
	c := BuildStatefulSet(inst, nil).Spec.Template.Spec.Containers[0]
	joined := strings.Join(c.Args, " ")
	if !strings.Contains(joined, "HEARTBEAT_SCHEDULER_ENABLED") {
		t.Errorf("multi-replica should keep pod-0 heartbeat gating, got %q", joined)
	}
}

// --- OTEL instrumentation gating ---
// The operator injects NODE_OPTIONS=--import ./server/dist/instrumentation.js for
// OTEL. That file only exists in the app image when instrumentation is built in,
// and forcing it unconditionally crashes the app (ERR_MODULE_NOT_FOUND) on images
// that do not ship it. Gate the whole OTEL block on observability.metrics.enabled.

func TestBuildStatefulSet_NoOTELByDefault(t *testing.T) {
	env := containerEnv(newTestInstance("p")) // metrics disabled by default
	for _, name := range []string{"NODE_OPTIONS", "OTEL_EXPORTER_OTLP_ENDPOINT", "OTEL_SERVICE_NAME"} {
		if hasEnvName(env, name) {
			t.Errorf("%s must not be injected when observability.metrics is disabled", name)
		}
	}
}

func TestBuildStatefulSet_OTELWhenMetricsEnabled(t *testing.T) {
	inst := newTestInstance("p")
	inst.Spec.Observability.Metrics.Enabled = true
	env := containerEnv(inst)
	wantEnvValue(t, env, "NODE_OPTIONS", "--import ./server/dist/instrumentation.js")
	wantEnvValue(t, env, "OTEL_SERVICE_NAME", "p")
	if !hasEnvName(env, "OTEL_EXPORTER_OTLP_ENDPOINT") {
		t.Error("expected OTEL_EXPORTER_OTLP_ENDPOINT when metrics enabled")
	}
}

// --- Task 3: Redis removed ---

func TestBuildStatefulSet_NoRedisEnv(t *testing.T) {
	if hasEnvName(containerEnv(newTestInstance("p")), "PAPERCLIP_RATE_LIMIT_REDIS_URL") {
		t.Error("Redis env must not be emitted; the app has no Redis rate limiter")
	}
}

// --- Task 4: managed-inference removed ---

func TestBuildStatefulSet_NoManagedInferenceEnv(t *testing.T) {
	env := containerEnv(newTestInstance("p"))
	for _, name := range []string{
		"PAPERCLIP_MANAGED_ANTHROPIC_API_KEY",
		"PAPERCLIP_MANAGED_OPENAI_API_KEY",
		"PAPERCLIP_MANAGED_GEMINI_API_KEY",
		"PAPERCLIP_MANAGED_OPENROUTER_API_KEY",
		"PAPERCLIP_MANAGED_INFERENCE_API_KEY",
		"PAPERCLIP_MANAGED_INFERENCE_PROVIDER",
		"PAPERCLIP_MANAGED_INFERENCE_MODEL",
	} {
		if hasEnvName(env, name) {
			t.Errorf("fabricated env %s must not be emitted", name)
		}
	}
}

// --- Task 5: AWS Secrets Manager vault ---

func TestBuildStatefulSet_AWSSecretsVaultEnv(t *testing.T) {
	inst := newTestInstance("p")
	inst.Spec.Secrets.Provider = "aws_secrets_manager"
	inst.Spec.Secrets.AWS = &paperclipv1alpha1.AWSSecretsManagerSpec{
		Region:       "eu-central-1",
		KMSKeyID:     "arn:aws:kms:eu-central-1:123:key/abc",
		DeploymentID: "prod-1",
		Prefix:       "paperclip",
		Environment:  "production",
		Endpoint:     "https://secretsmanager.eu-central-1.amazonaws.com",
	}
	env := containerEnv(inst)
	wantEnvValue(t, env, "PAPERCLIP_SECRETS_PROVIDER", "aws_secrets_manager")
	wantEnvValue(t, env, "PAPERCLIP_SECRETS_AWS_REGION", "eu-central-1")
	wantEnvValue(t, env, "PAPERCLIP_SECRETS_AWS_KMS_KEY_ID", "arn:aws:kms:eu-central-1:123:key/abc")
	wantEnvValue(t, env, "PAPERCLIP_SECRETS_AWS_DEPLOYMENT_ID", "prod-1")
	wantEnvValue(t, env, "PAPERCLIP_SECRETS_AWS_PREFIX", "paperclip")
	wantEnvValue(t, env, "PAPERCLIP_SECRETS_AWS_ENVIRONMENT", "production")
	wantEnvValue(t, env, "PAPERCLIP_SECRETS_AWS_ENDPOINT", "https://secretsmanager.eu-central-1.amazonaws.com")
}

func TestBuildStatefulSet_LocalEncryptedDefaultNoAWSEnv(t *testing.T) {
	env := containerEnv(newTestInstance("p")) // provider unset -> local_encrypted
	if hasEnvName(env, "PAPERCLIP_SECRETS_PROVIDER") {
		t.Error("PAPERCLIP_SECRETS_PROVIDER must be omitted for the default local provider")
	}
	if hasEnvName(env, "PAPERCLIP_SECRETS_AWS_REGION") {
		t.Error("AWS env must not appear for the default local provider")
	}
}

// --- Task 6: E2B sandbox key ---

func TestBuildStatefulSet_E2BKeyEnv(t *testing.T) {
	inst := newTestInstance("p")
	inst.Spec.Adapters.E2B = &paperclipv1alpha1.E2BSpec{
		APIKeySecretRef: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "e2b-secret"},
			Key:                  "api-key",
		},
	}
	e, ok := envEntry(containerEnv(inst), "E2B_API_KEY")
	if !ok {
		t.Fatal("expected E2B_API_KEY env from secret ref")
	}
	if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil ||
		e.ValueFrom.SecretKeyRef.Name != "e2b-secret" || e.ValueFrom.SecretKeyRef.Key != "api-key" {
		t.Fatalf("E2B_API_KEY not wired to secret ref: %+v", e)
	}
}

func TestBuildStatefulSet_E2BOmittedByDefault(t *testing.T) {
	if hasEnvName(containerEnv(newTestInstance("p")), "E2B_API_KEY") {
		t.Error("E2B_API_KEY must be omitted when spec.adapters.e2b is unset")
	}
}

// --- Task 7: app-native DB backup ---

func TestBuildStatefulSet_AppNativeBackupEnv(t *testing.T) {
	inst := newTestInstance("p")
	inst.Spec.Backup = &paperclipv1alpha1.BackupSpec{
		AppNative: &paperclipv1alpha1.AppNativeBackupSpec{
			Enabled:         Ptr(true),
			IntervalMinutes: 120,
			RetentionDays:   14,
		},
	}
	env := containerEnv(inst)
	wantEnvValue(t, env, "PAPERCLIP_DB_BACKUP_ENABLED", "true")
	wantEnvValue(t, env, "PAPERCLIP_DB_BACKUP_INTERVAL_MINUTES", "120")
	wantEnvValue(t, env, "PAPERCLIP_DB_BACKUP_RETENTION_DAYS", "14")
	wantEnvValue(t, env, "PAPERCLIP_DB_BACKUP_DIR", DataMountPath+"/backups")
}

func TestBuildStatefulSet_AppNativeBackupDisabled(t *testing.T) {
	inst := newTestInstance("p")
	inst.Spec.Backup = &paperclipv1alpha1.BackupSpec{
		AppNative: &paperclipv1alpha1.AppNativeBackupSpec{Enabled: Ptr(false)},
	}
	wantEnvValue(t, containerEnv(inst), "PAPERCLIP_DB_BACKUP_ENABLED", "false")
}

func TestBuildStatefulSet_NoAppNativeBackupByDefault(t *testing.T) {
	if hasEnvName(containerEnv(newTestInstance("p")), "PAPERCLIP_DB_BACKUP_ENABLED") {
		t.Error("PAPERCLIP_DB_BACKUP_* must be omitted when spec.backup.appNative is unset")
	}
}
