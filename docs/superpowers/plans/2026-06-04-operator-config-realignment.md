# Operator Config Realignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Realign the operator with Paperclip's real config contract: fix the deployment-mode enum, modernize bind, remove the dead Redis and fabricated managed-inference config, and add the genuinely env-configurable features (AWS Secrets Manager vault, E2B key, app-native DB backup). Verify with a full app boot on kind.

**Architecture:** Resource construction stays in pure builder functions in `internal/resources/` (unit-tested without envtest); the controller only orchestrates. CRD changes drive regeneration of `config/crd/bases`, deepcopy, Helm chart CRD templates, OLM bundle, and `docs/api-reference.md`. This is a breaking change set (`feat!:`).

**Tech Stack:** Go 1.25, controller-runtime/kubebuilder, envtest, Ginkgo (e2e), kind.

**Spec:** `docs/superpowers/specs/2026-06-04-operator-config-realignment-design.md`

**Branch:** `feat/config-realignment` (already checked out; image-org fix already committed).

## Conventions for every task

- After any change to `api/v1alpha1/*_types.go`, run in order:
  `make generate` then `make manifests` then `make sync-chart-crds`, and stage the generated diffs with the task's commit.
- Fast unit tests: `go test ./internal/resources/ -run <Name> -v`. Full resources suite: `go test ./internal/resources/`.
- Use `Ptr[T]` from `internal/resources/common.go` for pointer values. Octal literals as `0o644`. No em/en dashes.
- Commit at the end of each task with a conventional-commit message. Breaking commits use `!` (e.g. `feat!:`).

## File Structure

- `api/v1alpha1/paperclipinstance_types.go` - field add/remove + CEL validation markers.
- `internal/resources/statefulset.go` - env builders (`buildEnvVars` and helpers).
- `internal/resources/common.go` - `UseTCPProbes` probe selection.
- `internal/resources/redis.go` - DELETED.
- `internal/resources/redis_test.go` - DELETED (if present).
- `internal/resources/networkpolicy.go` - remove redis egress.
- `internal/controller/instance_controller.go` - remove redis reconcile call.
- `internal/controller/selfconfig_apply.go` - remove `REDIS_URL` allowlist entry.
- `internal/resources/resources_test.go` - unit tests for all builder changes.
- `test/conformance/` - CEL validation cases.
- `test/e2e/e2e_test.go` - full-boot + feature-render cases.
- `config/samples/*.yaml`, `README.md`, `docs/` - update + new `docs/deploy/runtime-configured-features.md`.

---

### Task 1: Deployment mode enum + disableSignUp + probe selection

**Files:**
- Modify: `api/v1alpha1/paperclipinstance_types.go` (DeploymentSpec ~216-238, AuthSpec ~287-313)
- Modify: `internal/resources/common.go:85-98` (UseTCPProbes)
- Modify: `internal/resources/statefulset.go:205` (mode env), `:272-280` region (add disableSignUp)
- Test: `internal/resources/resources_test.go`

- [ ] **Step 1: Write failing tests**

Add to `resources_test.go`:

```go
func TestUseTCPProbes_ModeSelection(t *testing.T) {
	cases := []struct {
		mode string
		want bool
	}{
		{"authenticated", true},
		{"local_trusted", false},
	}
	for _, tc := range cases {
		inst := &paperclipv1alpha1.Instance{}
		inst.Spec.Deployment.Mode = tc.mode
		if got := resources.UseTCPProbes(inst); got != tc.want {
			t.Errorf("mode %q: UseTCPProbes=%v want %v", tc.mode, got, tc.want)
		}
	}
}

func TestBuildStatefulSet_DisableSignUpEnv(t *testing.T) {
	inst := newMinimalInstance() // helper already used by existing tests; if absent, construct inline
	inst.Spec.Deployment.Mode = "authenticated"
	inst.Spec.Auth.DisableSignUp = true
	ss := resources.BuildStatefulSet(inst)
	if !hasEnvValue(ss, "PAPERCLIP_AUTH_DISABLE_SIGN_UP", "true") {
		t.Error("expected PAPERCLIP_AUTH_DISABLE_SIGN_UP=true")
	}
	if !hasEnvValue(ss, "PAPERCLIP_DEPLOYMENT_MODE", "authenticated") {
		t.Error("expected PAPERCLIP_DEPLOYMENT_MODE=authenticated")
	}
}
```

If `hasEnvValue`/`newMinimalInstance` helpers do not exist, add small local helpers near the top of the test file:

```go
func hasEnvValue(ss *appsv1.StatefulSet, name, val string) bool {
	for _, e := range ss.Spec.Template.Spec.Containers[0].Env {
		if e.Name == name {
			return e.Value == val
		}
	}
	return false
}
func hasEnvName(ss *appsv1.StatefulSet, name string) bool {
	for _, e := range ss.Spec.Template.Spec.Containers[0].Env {
		if e.Name == name {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run tests, verify fail**

Run: `go test ./internal/resources/ -run 'TestUseTCPProbes_ModeSelection|TestBuildStatefulSet_DisableSignUpEnv' -v`
Expected: FAIL (`local_trusted` not handled; `DisableSignUp` field undefined).

- [ ] **Step 3: Update API type**

In `DeploymentSpec.Mode`:

```go
	// Mode sets the deployment mode: "local_trusted" (loopback/no-auth) or
	// "authenticated" (login required). Matches Paperclip's DEPLOYMENT_MODES.
	// +kubebuilder:default="authenticated"
	// +kubebuilder:validation:Enum=local_trusted;authenticated
	// +optional
	Mode string `json:"mode,omitempty"`
```

Add CEL to the Instance spec or DeploymentSpec (place on the type that has both mode and exposure - DeploymentSpec):

```go
// +kubebuilder:validation:XValidation:rule="self.mode != 'local_trusted' || self.exposure == 'private'",message="spec.deployment.exposure must be 'private' when mode is 'local_trusted'"
type DeploymentSpec struct {
```

In `AuthSpec`, add:

```go
	// DisableSignUp disables public self-service sign-up (the former "single-tenant"
	// behavior). Maps to PAPERCLIP_AUTH_DISABLE_SIGN_UP.
	// +optional
	DisableSignUp bool `json:"disableSignUp,omitempty"`
```

- [ ] **Step 4: Update builders**

In `common.go` `UseTCPProbes`, replace the trailing comment+return:

```go
	// "auto" or empty: authenticated mode returns 403 from /api/health, so use TCP.
	return instance.Spec.Deployment.Mode == "authenticated"
```
Also update the function doc comment to drop "single-tenant".

In `statefulset.go` `buildEnvVars`, after the BETTER_AUTH_SECRET block (~280), add:

```go
	if instance.Spec.Auth.DisableSignUp {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_AUTH_DISABLE_SIGN_UP", Value: "true"})
	}
```

- [ ] **Step 5: Run tests, verify pass**

Run: `go test ./internal/resources/ -run 'TestUseTCPProbes_ModeSelection|TestBuildStatefulSet_DisableSignUpEnv' -v`
Expected: PASS

- [ ] **Step 6: Regenerate + full resources test**

Run: `make generate && make manifests && make sync-chart-crds && go test ./internal/resources/`
Expected: generation succeeds; resources tests PASS (fix any existing test/sample still using `mode: open`/`single-tenant` by switching to `local_trusted`/`authenticated`).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat!: align deployment mode enum with app (local_trusted|authenticated)

Replaces invalid open/single-tenant modes; adds spec.auth.disableSignUp for
the no-public-signup case. Probe auto-selection now uses TCP only for
authenticated mode."
```

---

### Task 2: Bind modernization (HOST -> PAPERCLIP_BIND)

**Files:**
- Modify: `internal/resources/statefulset.go:200-207` (base env list)
- Test: `internal/resources/resources_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestBuildStatefulSet_BindEnv(t *testing.T) {
	inst := newMinimalInstance()
	ss := resources.BuildStatefulSet(inst)
	if !hasEnvValue(ss, "PAPERCLIP_BIND", "custom") {
		t.Error("expected PAPERCLIP_BIND=custom")
	}
	if !hasEnvValue(ss, "PAPERCLIP_BIND_HOST", "0.0.0.0") {
		t.Error("expected PAPERCLIP_BIND_HOST=0.0.0.0")
	}
	if hasEnvName(ss, "HOST") {
		t.Error("legacy HOST env should no longer be set")
	}
}
```

- [ ] **Step 2: Run, verify fail**

Run: `go test ./internal/resources/ -run TestBuildStatefulSet_BindEnv -v`
Expected: FAIL (HOST still set; PAPERCLIP_BIND absent).

- [ ] **Step 3: Implement**

In `buildEnvVars` base list, replace `{Name: "HOST", Value: "0.0.0.0"},` with:

```go
		{Name: "PAPERCLIP_BIND", Value: "custom"},
		{Name: "PAPERCLIP_BIND_HOST", Value: "0.0.0.0"},
```

- [ ] **Step 4: Run, verify pass**

Run: `go test ./internal/resources/ -run TestBuildStatefulSet_BindEnv -v`
Expected: PASS. Then `go test ./internal/resources/` and fix any existing test asserting `HOST`.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat!: emit PAPERCLIP_BIND/PAPERCLIP_BIND_HOST instead of legacy HOST"
```

---

### Task 3: Remove dead Redis

**Files:**
- Delete: `internal/resources/redis.go` and `internal/resources/redis_test.go` (if it exists)
- Modify: `api/v1alpha1/paperclipinstance_types.go` (remove `Redis *RedisSpec` field on InstanceSpec + `RedisSpec`/`ManagedRedisSpec` types ~412-454)
- Modify: `internal/resources/statefulset.go:361-382` (remove redis env block)
- Modify: `internal/resources/networkpolicy.go` (remove redis egress rule)
- Modify: `internal/controller/instance_controller.go` (remove redis reconcile call + any redis import usage)
- Modify: `internal/controller/selfconfig_apply.go:46` (remove `"REDIS_URL": true,`)
- Test: `internal/resources/resources_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestBuildStatefulSet_NoRedisEnv(t *testing.T) {
	inst := newMinimalInstance()
	ss := resources.BuildStatefulSet(inst)
	if hasEnvName(ss, "PAPERCLIP_RATE_LIMIT_REDIS_URL") {
		t.Error("Redis env must not be emitted; app has no Redis")
	}
}
```

- [ ] **Step 2: Run, verify fail or compile error**

Run: `go test ./internal/resources/ -run TestBuildStatefulSet_NoRedisEnv -v`
Expected: FAIL (env still present).

- [ ] **Step 3: Remove redis usages**

- `git rm internal/resources/redis.go` (and `internal/resources/redis_test.go` if present).
- In `statefulset.go`, delete the entire `// Redis` block (`if instance.Spec.Redis != nil { ... }`, ~361-382).
- In `networkpolicy.go`, remove the redis-port egress rule (search for `RedisPort` / `6379` / `Spec.Redis`).
- In `instance_controller.go`, remove the redis reconcile call (search `reconcileRedis` / `Redis`) and any now-unused helper/import.
- In `selfconfig_apply.go`, delete the line `"REDIS_URL": true,`.
- In `api/v1alpha1/paperclipinstance_types.go`, remove the `Redis *RedisSpec` field from `InstanceSpec` and delete the `RedisSpec` and `ManagedRedisSpec` type blocks. Remove any `RedisPort` const in `common.go` if unused.

Verify nothing references redis:

Run: `grep -rni "redis" api/ internal/ | grep -v _test`
Expected: no matches (except none). Fix any stragglers.

- [ ] **Step 4: Build, regenerate, test**

Run: `go build ./... && make generate && make manifests && make sync-chart-crds && go test ./internal/resources/`
Expected: builds; `TestBuildStatefulSet_NoRedisEnv` PASS. Remove redis from samples/conformance testdata if referenced (`grep -rni redis config/ test/`).

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat!: remove managed Redis (app has no Redis rate limiter)

The app exposes no REDIS_URL/PAPERCLIP_RATE_LIMIT_REDIS_URL and its only rate
limiter is in-process. Removes spec.redis, the Redis StatefulSet/Service/PVC
builders, the redis NetworkPolicy egress, and the dead env var."
```

---

### Task 4: Remove fabricated managed-inference

**Files:**
- Modify: `api/v1alpha1/paperclipinstance_types.go` (AdaptersSpec ~480-499: remove ManagedInferenceSecretRef, ManagedInferenceProvider, ManagedInferenceModel)
- Modify: `internal/resources/statefulset.go` (remove `buildManagedInferenceEnvVars` 530-585 + its call at 409)
- Test: `internal/resources/resources_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestBuildStatefulSet_NoManagedInferenceEnv(t *testing.T) {
	inst := newMinimalInstance()
	ss := resources.BuildStatefulSet(inst)
	for _, name := range []string{
		"PAPERCLIP_MANAGED_ANTHROPIC_API_KEY",
		"PAPERCLIP_MANAGED_OPENAI_API_KEY",
		"PAPERCLIP_MANAGED_INFERENCE_API_KEY",
		"PAPERCLIP_MANAGED_INFERENCE_PROVIDER",
		"PAPERCLIP_MANAGED_INFERENCE_MODEL",
	} {
		if hasEnvName(ss, name) {
			t.Errorf("fabricated env %s must not be emitted", name)
		}
	}
}
```

- [ ] **Step 2: Run, verify fail**

Run: `go test ./internal/resources/ -run TestBuildStatefulSet_NoManagedInferenceEnv -v`
Expected: FAIL.

- [ ] **Step 3: Implement**

- Delete `func buildManagedInferenceEnvVars(...)` and the line `vars = append(vars, buildManagedInferenceEnvVars(instance)...)` (with its `// Managed inference` comment) in `statefulset.go`.
- Remove the three managed-inference fields from `AdaptersSpec`.
- `grep -rni "managedinference\|MANAGED_INFERENCE\|MANAGED_ANTHROPIC\|MANAGED_OPENAI\|MANAGED_GEMINI\|MANAGED_OPENROUTER" api/ internal/ | grep -v _test` -> expect no matches.

- [ ] **Step 4: Build, regenerate, test**

Run: `go build ./... && make generate && make manifests && make sync-chart-crds && go test ./internal/resources/`
Expected: PASS. Update any sample/conformance testdata that set `managedInference*`.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat!: remove fabricated managed-inference config

PAPERCLIP_MANAGED_INFERENCE_* env vars do not exist in the app and were no-ops.
LLM keys are set via spec.adapters.apiKeysSecretRef (ANTHROPIC_API_KEY/OPENAI_API_KEY);
model discovery is automatic."
```

---

### Task 5: AWS Secrets Manager vault

**Files:**
- Modify: `api/v1alpha1/paperclipinstance_types.go` (SecretsSpec ~351-360: add provider + aws struct)
- Modify: `internal/resources/statefulset.go` (new `buildSecretsProviderEnvVars` helper + call after StrictMode block ~307)
- Test: `internal/resources/resources_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestBuildStatefulSet_AWSSecretsVaultEnv(t *testing.T) {
	inst := newMinimalInstance()
	inst.Spec.Secrets.Provider = "aws_secrets_manager"
	inst.Spec.Secrets.AWS = &paperclipv1alpha1.AWSSecretsManagerSpec{
		Region:       "eu-central-1",
		KMSKeyID:     "arn:aws:kms:eu-central-1:123:key/abc",
		DeploymentID: "prod-1",
		Prefix:       "paperclip",
	}
	ss := resources.BuildStatefulSet(inst)
	checks := map[string]string{
		"PAPERCLIP_SECRETS_PROVIDER":          "aws_secrets_manager",
		"PAPERCLIP_SECRETS_AWS_REGION":        "eu-central-1",
		"PAPERCLIP_SECRETS_AWS_KMS_KEY_ID":    "arn:aws:kms:eu-central-1:123:key/abc",
		"PAPERCLIP_SECRETS_AWS_DEPLOYMENT_ID": "prod-1",
		"PAPERCLIP_SECRETS_AWS_PREFIX":        "paperclip",
	}
	for k, v := range checks {
		if !hasEnvValue(ss, k, v) {
			t.Errorf("expected %s=%s", k, v)
		}
	}
}

func TestBuildStatefulSet_LocalEncryptedDefaultNoAWSEnv(t *testing.T) {
	inst := newMinimalInstance() // provider defaults to local_encrypted
	ss := resources.BuildStatefulSet(inst)
	if hasEnvName(ss, "PAPERCLIP_SECRETS_AWS_REGION") {
		t.Error("AWS env must not appear for local_encrypted provider")
	}
}
```

- [ ] **Step 2: Run, verify fail**

Run: `go test ./internal/resources/ -run 'AWSSecretsVault|LocalEncryptedDefault' -v`
Expected: FAIL (types/fields undefined).

- [ ] **Step 3: Update API type**

In `SecretsSpec` add:

```go
	// Provider selects the secrets backend. "local_encrypted" (default) uses the
	// master key; "aws_secrets_manager" stores secrets in AWS Secrets Manager.
	// +kubebuilder:default="local_encrypted"
	// +kubebuilder:validation:Enum=local_encrypted;aws_secrets_manager
	// +optional
	Provider string `json:"provider,omitempty"`

	// AWS configures the AWS Secrets Manager provider. Credentials are sourced from
	// the AWS SDK credential chain (use IRSA via spec.security.rbac.serviceAccountAnnotations).
	// +optional
	AWS *AWSSecretsManagerSpec `json:"aws,omitempty"`
```

Add the type with CEL requiring the three mandatory fields:

```go
// AWSSecretsManagerSpec configures the AWS Secrets Manager secrets provider.
type AWSSecretsManagerSpec struct {
	// Region is the AWS region of the secrets/KMS key.
	Region string `json:"region"`
	// KMSKeyID is the KMS key (id or ARN) used to encrypt secrets.
	KMSKeyID string `json:"kmsKeyID"`
	// DeploymentID namespaces secrets for this deployment.
	DeploymentID string `json:"deploymentID"`
	// Prefix is the secret name prefix.
	// +kubebuilder:default="paperclip"
	// +optional
	Prefix string `json:"prefix,omitempty"`
	// Environment is an optional environment label.
	// +optional
	Environment string `json:"environment,omitempty"`
	// Endpoint overrides the AWS Secrets Manager endpoint (for testing/VPC endpoints).
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// DeleteRecoveryDays is the AWS recovery window for deleted secrets.
	// +kubebuilder:default=30
	// +optional
	DeleteRecoveryDays *int32 `json:"deleteRecoveryDays,omitempty"`
}
```

Add CEL on SecretsSpec requiring aws fields when provider is aws:

```go
// +kubebuilder:validation:XValidation:rule="self.provider != 'aws_secrets_manager' || has(self.aws)",message="spec.secrets.aws is required when provider is aws_secrets_manager"
type SecretsSpec struct {
```

- [ ] **Step 4: Implement env helper**

In `statefulset.go`, add a call after the StrictMode block:

```go
	vars = append(vars, buildSecretsProviderEnvVars(instance)...)
```

And the helper:

```go
func buildSecretsProviderEnvVars(instance *paperclipv1alpha1.Instance) []corev1.EnvVar {
	provider := instance.Spec.Secrets.Provider
	if provider == "" || provider == "local_encrypted" {
		return nil
	}
	vars := []corev1.EnvVar{{Name: "PAPERCLIP_SECRETS_PROVIDER", Value: provider}}
	if provider == "aws_secrets_manager" && instance.Spec.Secrets.AWS != nil {
		aws := instance.Spec.Secrets.AWS
		vars = append(vars,
			corev1.EnvVar{Name: "PAPERCLIP_SECRETS_AWS_REGION", Value: aws.Region},
			corev1.EnvVar{Name: "PAPERCLIP_SECRETS_AWS_KMS_KEY_ID", Value: aws.KMSKeyID},
			corev1.EnvVar{Name: "PAPERCLIP_SECRETS_AWS_DEPLOYMENT_ID", Value: aws.DeploymentID},
		)
		if aws.Prefix != "" {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_SECRETS_AWS_PREFIX", Value: aws.Prefix})
		}
		if aws.Environment != "" {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_SECRETS_AWS_ENVIRONMENT", Value: aws.Environment})
		}
		if aws.Endpoint != "" {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_SECRETS_AWS_ENDPOINT", Value: aws.Endpoint})
		}
		if aws.DeleteRecoveryDays != nil {
			vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_SECRETS_AWS_DELETE_RECOVERY_DAYS", Value: fmt.Sprintf("%d", *aws.DeleteRecoveryDays)})
		}
	}
	return vars
}
```

- [ ] **Step 5: Run, verify pass + regenerate**

Run: `go test ./internal/resources/ -run 'AWSSecretsVault|LocalEncryptedDefault' -v && make generate && make manifests && make sync-chart-crds && go test ./internal/resources/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: add AWS Secrets Manager secrets provider

spec.secrets.provider + spec.secrets.aws emit PAPERCLIP_SECRETS_PROVIDER and
PAPERCLIP_SECRETS_AWS_*. AWS creds come from the SDK chain (IRSA via
serviceAccountAnnotations); no keys are injected."
```

---

### Task 6: E2B sandbox key

**Files:**
- Modify: `api/v1alpha1/paperclipinstance_types.go` (AdaptersSpec: add `E2B *E2BSpec`)
- Modify: `internal/resources/statefulset.go` (emit `E2B_API_KEY` in `buildEnvVars`, near cloud sandbox)
- Test: `internal/resources/resources_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestBuildStatefulSet_E2BKeyEnv(t *testing.T) {
	inst := newMinimalInstance()
	inst.Spec.Adapters.E2B = &paperclipv1alpha1.E2BSpec{
		APIKeySecretRef: corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "e2b-secret"},
			Key:                  "api-key",
		},
	}
	ss := resources.BuildStatefulSet(inst)
	for _, e := range ss.Spec.Template.Spec.Containers[0].Env {
		if e.Name == "E2B_API_KEY" {
			if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil ||
				e.ValueFrom.SecretKeyRef.Name != "e2b-secret" || e.ValueFrom.SecretKeyRef.Key != "api-key" {
				t.Fatalf("E2B_API_KEY not wired to secret ref: %+v", e)
			}
			return
		}
	}
	t.Error("expected E2B_API_KEY env from secret ref")
}
```

- [ ] **Step 2: Run, verify fail**

Run: `go test ./internal/resources/ -run TestBuildStatefulSet_E2BKeyEnv -v`
Expected: FAIL (type undefined).

- [ ] **Step 3: Implement**

In `AdaptersSpec` add:

```go
	// E2B configures the E2B sandbox provider API key. The provider itself must be
	// enabled as a plugin and selected per-Environment in the Paperclip UI; the
	// operator only supplies E2B_API_KEY.
	// +optional
	E2B *E2BSpec `json:"e2b,omitempty"`
```

Add the type:

```go
// E2BSpec configures the E2B sandbox provider.
type E2BSpec struct {
	// APIKeySecretRef references a Secret key holding the E2B API key.
	APIKeySecretRef corev1.SecretKeySelector `json:"apiKeySecretRef"`
}
```

In `buildEnvVars`, after the LLM API keys block (~406), add:

```go
	if instance.Spec.Adapters.E2B != nil {
		ref := instance.Spec.Adapters.E2B.APIKeySecretRef
		vars = append(vars, corev1.EnvVar{
			Name:      "E2B_API_KEY",
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &ref},
		})
	}
```

- [ ] **Step 4: Run, regenerate, test**

Run: `go test ./internal/resources/ -run TestBuildStatefulSet_E2BKeyEnv -v && make generate && make manifests && make sync-chart-crds && go test ./internal/resources/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "feat: add E2B sandbox API key (spec.adapters.e2b -> E2B_API_KEY)"
```

---

### Task 7: App-native DB backup

**Files:**
- Modify: `api/v1alpha1/paperclipinstance_types.go` (add `AppNative *AppNativeBackupSpec` on InstanceSpec.Backup or a new top-level `spec.backup.appNative`)
- Modify: `internal/resources/statefulset.go` (new `buildAppNativeBackupEnvVars` + call)
- Test: `internal/resources/resources_test.go`

Note: `spec.backup` (BackupSpec) currently REQUIRES `schedule` (no omitempty). To allow app-native config without the pg_dump CronJob, make `Schedule` optional (`+optional`, `omitempty`) and only build the CronJob when `Schedule != ""` (verify the controller already guards on schedule; if it guards on `Backup != nil`, change it to guard on `Schedule != ""`). Add the appNative struct to BackupSpec.

- [ ] **Step 1: Write failing test**

```go
func TestBuildStatefulSet_AppNativeBackupEnv(t *testing.T) {
	inst := newMinimalInstance()
	inst.Spec.Backup = &paperclipv1alpha1.BackupSpec{
		AppNative: &paperclipv1alpha1.AppNativeBackupSpec{
			Enabled:         resources.Ptr(true),
			IntervalMinutes: 120,
			RetentionDays:   14,
		},
	}
	ss := resources.BuildStatefulSet(inst)
	checks := map[string]string{
		"PAPERCLIP_DB_BACKUP_ENABLED":          "true",
		"PAPERCLIP_DB_BACKUP_INTERVAL_MINUTES": "120",
		"PAPERCLIP_DB_BACKUP_RETENTION_DAYS":   "14",
	}
	for k, v := range checks {
		if !hasEnvValue(ss, k, v) {
			t.Errorf("expected %s=%s", k, v)
		}
	}
	if !hasEnvName(ss, "PAPERCLIP_DB_BACKUP_DIR") {
		t.Error("expected PAPERCLIP_DB_BACKUP_DIR to be set under the data PVC")
	}
}

func TestBuildStatefulSet_AppNativeBackupDisabled(t *testing.T) {
	inst := newMinimalInstance()
	inst.Spec.Backup = &paperclipv1alpha1.BackupSpec{
		AppNative: &paperclipv1alpha1.AppNativeBackupSpec{Enabled: resources.Ptr(false)},
	}
	ss := resources.BuildStatefulSet(inst)
	if !hasEnvValue(ss, "PAPERCLIP_DB_BACKUP_ENABLED", "false") {
		t.Error("expected PAPERCLIP_DB_BACKUP_ENABLED=false")
	}
}
```

- [ ] **Step 2: Run, verify fail**

Run: `go test ./internal/resources/ -run AppNativeBackup -v`
Expected: FAIL.

- [ ] **Step 3: Implement API**

Make `BackupSpec.Schedule` optional and add the appNative field:

```go
	// Schedule is a cron expression for the operator pg_dump -> S3 backup CronJob.
	// Optional; omit to use only app-native backups.
	// +optional
	Schedule string `json:"schedule,omitempty"`

	// AppNative configures Paperclip's built-in DB backups (local dir under the
	// data PVC). Complementary to the operator pg_dump -> S3 CronJob.
	// +optional
	AppNative *AppNativeBackupSpec `json:"appNative,omitempty"`
```

Add the type:

```go
// AppNativeBackupSpec configures Paperclip's built-in database backups.
type AppNativeBackupSpec struct {
	// Enabled toggles app-native backups. Defaults to true (the app's own default).
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
	// IntervalMinutes between backups.
	// +optional
	IntervalMinutes int32 `json:"intervalMinutes,omitempty"`
	// RetentionDays for local backups.
	// +optional
	RetentionDays int32 `json:"retentionDays,omitempty"`
}
```

- [ ] **Step 4: Implement env helper**

In `statefulset.go`, add a call in `buildEnvVars` and the helper:

```go
	vars = append(vars, buildAppNativeBackupEnvVars(instance)...)
```

```go
func buildAppNativeBackupEnvVars(instance *paperclipv1alpha1.Instance) []corev1.EnvVar {
	if instance.Spec.Backup == nil || instance.Spec.Backup.AppNative == nil {
		return nil
	}
	an := instance.Spec.Backup.AppNative
	var vars []corev1.EnvVar
	if an.Enabled != nil {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_DB_BACKUP_ENABLED", Value: fmt.Sprintf("%t", *an.Enabled)})
	}
	if an.IntervalMinutes > 0 {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_DB_BACKUP_INTERVAL_MINUTES", Value: fmt.Sprintf("%d", an.IntervalMinutes)})
	}
	if an.RetentionDays > 0 {
		vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_DB_BACKUP_RETENTION_DAYS", Value: fmt.Sprintf("%d", an.RetentionDays)})
	}
	// Keep backups on the persistent data volume.
	vars = append(vars, corev1.EnvVar{Name: "PAPERCLIP_DB_BACKUP_DIR", Value: DataMountPath + "/backups"})
	return vars
}
```

Verify the controller only builds the pg_dump CronJob when `instance.Spec.Backup != nil && instance.Spec.Backup.Schedule != ""` (adjust the guard if it currently keys on `Backup != nil`).

- [ ] **Step 5: Run, regenerate, test**

Run: `go test ./internal/resources/ -run AppNativeBackup -v && make generate && make manifests && make sync-chart-crds && go test ./...`
Expected: PASS (run full `go test ./...` minus e2e; envtest may be needed - if envtest unavailable run `go test ./internal/resources/ ./test/conformance/`).

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "feat: add app-native DB backup config (PAPERCLIP_DB_BACKUP_*)

Makes spec.backup.schedule optional and adds spec.backup.appNative for the
app's built-in local-dir backups, stored under the data PVC. The operator
pg_dump -> S3 CronJob remains the offsite path."
```

---

### Task 8: Conformance (CEL) tests

**Files:**
- Modify: `test/conformance/` (add negative cases; follow existing `negative_test.go` style)

- [ ] **Step 1: Add failing-by-design validation cases**

Add cases asserting the API server rejects:
- `spec.deployment.mode: local_trusted` with `spec.deployment.exposure: public`.
- `spec.deployment.mode: open` (no longer in enum).
- `spec.secrets.provider: aws_secrets_manager` without `spec.secrets.aws`.

Match the existing harness in `test/conformance/negative_test.go` (it applies YAML against an apiserver and expects an error). Reuse its helper for "expect apply error containing message".

- [ ] **Step 2: Run conformance**

Run: `go test ./test/conformance/ -v`
Expected: PASS (the apiserver rejects each invalid manifest with the CEL message).

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "test(conformance): cover new CEL validation (mode/exposure, aws secrets)"
```

---

### Task 9: Docs, samples, CHANGELOG/upgrade notes

**Files:**
- Create: `docs/deploy/runtime-configured-features.md`
- Modify: `config/samples/*.yaml`, `README.md`, `docs/api-reference.md` (regen), `CHANGELOG.md` (or a docs/UPGRADING note)

- [ ] **Step 1: Write runtime-configured-features doc**

Create `docs/deploy/runtime-configured-features.md` covering, in prose with examples:
- MCP server: `@paperclipai/mcp-server` is a stdio binary configured with `PAPERCLIP_API_URL` + `PAPERCLIP_API_KEY`; not exposed by the operator. Show a sample stdio client invocation pointing at the in-cluster Service URL.
- Modal/Cloudflare sandbox creds: set in plugin config / company secrets in the UI (operator only wires `E2B_API_KEY`).
- SSH Environments: runtime DB records created in the UI/API; list the fields (host/port/username/remoteWorkspacePath/privateKeySecretRef).
- First-admin: with `spec.auth.adminUser` the operator bootstraps; without it, the app board-claim flow grants the first human ownership in authenticated mode.

- [ ] **Step 2: Update samples + README + api-reference**

- Update `config/samples/*` to use `mode: authenticated` (already valid) and remove any `redis:`/`managedInference*` blocks; add commented examples of `secrets.provider: aws_secrets_manager`, `adapters.e2b`, and `backup.appNative`.
- Update README env/feature tables: remove Redis + managed-inference rows; add Secrets vault, E2B, app-native backup, PAPERCLIP_BIND.
- Regenerate api-reference: run the api-docs target if present (`make api-docs` or the documented command); otherwise edit `docs/api-reference.md` to match.

- [ ] **Step 3: Write upgrade notes**

Add an "Upgrading to <next> (breaking)" section to `CHANGELOG.md` (or `docs/UPGRADING.md`) listing the three migrations from the spec (redis removed; managedInference removed; mode open/single-tenant -> local_trusted / authenticated+disableSignUp).

- [ ] **Step 4: Verify links + lint**

Run: `make lint` (and `make fmt` first). Expected: clean. Fix any double-hyphen usage flagged.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "docs: realignment - runtime-configured features, samples, upgrade notes"
```

---

### Task 10: Full unit/integration gate before e2e

- [ ] **Step 1: Run the full non-e2e suite**

Run: `make fmt && make vet && make lint && make test`
Expected: all PASS. `make test` runs unit + envtest integration; if envtest binaries are missing, install via `make envtest` / the documented setup, then re-run. Fix any controller integration test referencing removed fields.

- [ ] **Step 2: Commit any fixups**

```bash
git add -A && git commit -m "test: fix up integration tests after realignment" || echo "nothing to commit"
```

---

### Task 11: E2E full app boot on kind (authenticated + managed DB)

**Files:**
- Modify: `test/e2e/e2e_test.go` (+ `e2e_suite_test.go` if needed)

- [ ] **Step 1: Resolve a real app image tag**

Run: `gh api 'repos/paperclipai/paperclip/tags' --jq '.[0:5][].name'`
Pick the newest `vYYYY.MMM.N` tag (e.g. `v2026.529.0`). Use it as the Instance `spec.image.tag` in the e2e case (CRD forbids `:latest`).

- [ ] **Step 2: Add the happy-path boot case**

In `test/e2e/e2e_test.go`, add a Ginkgo `It` that:
- Creates a namespace, a `BETTER_AUTH_SECRET` Secret, and an admin-password Secret.
- Applies an Instance: `image.repository=ghcr.io/paperclipai/paperclip`, `image.tag=<resolved>`, `database.mode=managed`, `deployment.mode=authenticated`, `auth.secretRef` -> the BETTER_AUTH_SECRET secret, `auth.adminUser` -> email + passwordSecretRef, `storage.persistence.enabled=true`.
- `Eventually` (timeout 5m, poll 10s): managed Postgres StatefulSet `-db` has 1 ready replica; then the app StatefulSet has 1 ready replica (readiness via TCP probe).
- Asserts via `kubectl get statefulset <name> -o jsonpath` that the app pod env contains `PAPERCLIP_DEPLOYMENT_MODE=authenticated`, `PAPERCLIP_BIND=custom`, and does NOT contain `PAPERCLIP_RATE_LIMIT_REDIS_URL` or any `PAPERCLIP_MANAGED_`.
- Asserts no Service/StatefulSet named `<name>-redis` exists.
- `Eventually` the bootstrap Job `<name>-bootstrap` reaches `Complete`.

Follow the existing helpers/patterns in `e2e_test.go` (it already builds+loads the operator image and deploys via the manager manifests).

- [ ] **Step 3: Add the feature-render case**

A second `It` applies an Instance with `secrets.provider=aws_secrets_manager` (+ dummy aws fields), `adapters.e2b.apiKeySecretRef`, and `backup.appNative`, and asserts (without waiting for Ready) that the rendered StatefulSet env contains `PAPERCLIP_SECRETS_AWS_REGION`, `E2B_API_KEY`, and `PAPERCLIP_DB_BACKUP_ENABLED`. This case can use `database.mode=embedded` to avoid a second Postgres.

- [ ] **Step 4: Run e2e on kind**

Run: `make test-e2e`
Expected: creates the kind cluster, builds+loads the operator image, both `It`s PASS, tears down. If the app image needs a config knob to boot in authenticated mode without LLM keys, capture pod logs (`kubectl logs`) in the failure path and adjust the Instance (e.g. ensure `auth.secretRef` present, exposure private). Iterate until green.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "test(e2e): full app boot on kind + feature env render"
```

---

## Self-review notes

- Spec coverage: A (Task 1-2), B (Task 3), C (Task 4), D (Task 5), E (Task 6), F (Task 7), G (Task 9 docs), validation (Task 8), verification (Task 10-11). All spec sections mapped.
- Open verification points for the executor (do not skip): exact line of the redis reconcile call in `instance_controller.go`; whether `BackupSpec` CronJob guard keys on `Schedule` vs `Backup != nil`; whether `newMinimalInstance`/`hasEnvValue` helpers already exist in `resources_test.go` (reuse if present, else add once).
- Breaking commits use `feat!:`; release-please will compute the version bump.
