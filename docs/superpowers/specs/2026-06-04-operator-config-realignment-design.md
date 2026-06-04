# Operator Config Realignment Design

Date: 2026-06-04
Status: Approved (design), pending spec review

## Problem

The operator was built partly against Paperclip release notes rather than the
app's actual configuration contract. Auditing `paperclipai/paperclip@master`
(`server/src/config.ts`, `packages/shared/src/config-schema.ts`,
`packages/shared/src/constants.ts`, `docs/deploy/environment-variables.md`)
revealed the operator emits configuration the app ignores or rejects, and that
several "missing features" are not configurable by an operator at all.

This design realigns the operator with the real contract via a clean break
(no backward-compatible shims), and adds the features that are genuinely
env-configurable.

### Verified findings (source of truth: app `@master`)

- `DEPLOYMENT_MODES = ["local_trusted", "authenticated"]`. The operator's CRD
  enum is `open|authenticated|single-tenant` and passes the raw value as
  `PAPERCLIP_DEPLOYMENT_MODE`, so `open` and `single-tenant` are invalid.
  The app also requires `exposure = private` when mode is `local_trusted`.
- The app has **no Redis**. There is no `REDIS_URL` / `PAPERCLIP_RATE_LIMIT_REDIS_URL`
  in `server/src/config.ts`; the only rate limiter is in-process/in-memory. The
  operator's managed Redis (StatefulSet/Service/PVC) and `PAPERCLIP_RATE_LIMIT_REDIS_URL`
  are dead weight.
- `PAPERCLIP_MANAGED_INFERENCE_*` and `PAPERCLIP_MANAGED_ANTHROPIC_API_KEY` do
  not exist in the app. Real LLM keys are `ANTHROPIC_API_KEY` / `OPENAI_API_KEY`;
  model lists are discovered live from the provider (`packages/adapters/claude-local/src/server/models.ts`).
- `HOST` works but is documented legacy; preferred bind is `PAPERCLIP_BIND`
  (`loopback|lan|tailnet|custom`) + `PAPERCLIP_BIND_HOST` (required when `custom`).
- Secrets vault IS env-configurable: `PAPERCLIP_SECRETS_PROVIDER`
  (`local_encrypted|aws_secrets_manager|gcp_secret_manager|vault`; only the first
  two are runtime-functional) + `PAPERCLIP_SECRETS_AWS_*` (region, kms key id,
  deployment id, prefix, environment, endpoint, delete recovery days). AWS creds
  come from the AWS SDK credential chain (IRSA), not Paperclip env.
- Sandbox provider is a per-Environment DB/UI record. Only E2B has an env
  fallback (`E2B_API_KEY`). Modal (`tokenId`/`tokenSecret`) and Cloudflare
  (`bridgeBaseUrl`/`bridgeAuthToken`) are plugin config / company secrets only;
  plugin workers do not inherit pod env.
- App-native DB backup is env-configurable but writes to a local dir only (no
  S3): `PAPERCLIP_DB_BACKUP_ENABLED` (default true), `PAPERCLIP_DB_BACKUP_INTERVAL_MINUTES`
  (default 60), `PAPERCLIP_DB_BACKUP_RETENTION_DAYS` (default 7), `PAPERCLIP_DB_BACKUP_DIR`.
- The standalone MCP server (`@paperclipai/mcp-server`) is a stdio binary with no
  port/HTTP listener; it connects to an instance via `PAPERCLIP_API_URL` +
  `PAPERCLIP_API_KEY`. Nothing for the operator to expose as a Service.
- SSH "Environments" are runtime DB records (`sshEnvironmentConfigSchema`); no env
  configures them.
- First-admin: no env flag. `authenticated` mode + DB state triggers a board-claim
  flow (`server/src/board-claim.ts`); the CLI `bootstrap-ceo` is the credentialed path.

## Scope decisions (confirmed with user)

- Pivot to the corrected scope (fix drift + add env-configurable features).
- Clean break now: remove Redis, managed-inference, and old mode enum values
  outright. Existing CRs (including the production SaaS deployment) must be
  migrated before upgrade.
- Kind verification: full app boot with `authenticated` mode + managed Postgres.

## Changes

### A. Deployment mode + bind (breaking)

- `spec.deployment.mode`: enum becomes `local_trusted|authenticated`,
  default `authenticated`. Emit `PAPERCLIP_DEPLOYMENT_MODE` = the value verbatim.
- New `spec.auth.disableSignUp` (bool, default false) -> `PAPERCLIP_AUTH_DISABLE_SIGN_UP`.
  Replaces the old `single-tenant` mode intent (authenticated + no public sign-up).
- CEL validation on the Instance: `spec.deployment.exposure` must be `private`
  when `spec.deployment.mode == "local_trusted"`.
- Bind: stop emitting `HOST`. Emit `PAPERCLIP_BIND=custom` and
  `PAPERCLIP_BIND_HOST=0.0.0.0`.
- Probe auto-selection (`internal/resources/common.go`): HTTP probe for
  `local_trusted`, TCP probe for `authenticated` (because `/api/health` returns
  403 without credentials in authenticated mode). Remove the `single-tenant` branch.
- `PaperclipClusterDefaults.databaseMode` enum is unaffected (embedded/external/managed),
  but any cluster-defaults mode references must use the new values where applicable.

### B. Remove dead Redis (breaking)

- Delete `internal/resources/redis.go` and its tests.
- Delete `spec.redis` (`RedisSpec`, `ManagedRedisSpec`) from the API type.
- Delete `RedisURL` (`internal/resources/redis.go`), the `PAPERCLIP_RATE_LIMIT_REDIS_URL` env block in
  `statefulset.go`, the redis reconcile call in `instance_controller.go`, the
  redis egress rule in `networkpolicy.go`, and the `REDIS_URL` entry in the
  selfconfig env allowlist (`internal/controller/selfconfig_apply.go`).
- Regenerate CRDs/deepcopy; update charts, bundle, samples, docs, tests.

### C. Remove fabricated managed-inference (breaking)

- Delete `spec.adapters.managedInferenceSecretRef`, `managedInferenceProvider`,
  `managedInferenceModel` from the API type.
- Delete `buildManagedInferenceEnvVars` and its call in `statefulset.go`.
- Keep `spec.adapters.apiKeysSecretRef`, which maps to the real `ANTHROPIC_API_KEY`
  / `OPENAI_API_KEY`. No model default is set (live discovery handles it).

### D. AWS Secrets Manager vault (additive)

- Extend `spec.secrets`:
  - `provider` enum `local_encrypted|aws_secrets_manager`, default `local_encrypted`.
  - `aws` struct: `region` (req when aws), `kmsKeyID` (req), `deploymentID` (req),
    `prefix` (opt, default `paperclip`), `environment` (opt), `endpoint` (opt),
    `deleteRecoveryDays` (opt, default 30).
- Emit when provider is aws: `PAPERCLIP_SECRETS_PROVIDER=aws_secrets_manager`,
  `PAPERCLIP_SECRETS_AWS_REGION`, `PAPERCLIP_SECRETS_AWS_KMS_KEY_ID`,
  `PAPERCLIP_SECRETS_AWS_DEPLOYMENT_ID`, `PAPERCLIP_SECRETS_AWS_PREFIX`,
  `PAPERCLIP_SECRETS_AWS_ENVIRONMENT`, `PAPERCLIP_SECRETS_AWS_ENDPOINT`,
  `PAPERCLIP_SECRETS_AWS_DELETE_RECOVERY_DAYS`.
- AWS credentials via IRSA: documented via `spec.security.rbac.serviceAccountAnnotations`
  (e.g. `eks.amazonaws.com/role-arn`). The operator injects no AWS keys.
- CEL validation: when `provider == aws_secrets_manager`, `aws.region`,
  `aws.kmsKeyID`, and `aws.deploymentID` are required.
- NetworkPolicy: existing rule already allows 443 egress, which covers AWS
  Secrets Manager. No change required (verify during implementation).

### E. E2B sandbox key (additive, minimal)

- New `spec.adapters.e2b.apiKeySecretRef` (Secret name + key) -> `E2B_API_KEY`.
- The existing K8s `spec.adapters.cloudSandbox` provider is untouched.
- Docs note: the user must also enable the `@paperclipai/plugin-e2b` plugin and
  create the E2B Environment in the Paperclip UI; the operator only supplies the key.

### F. App-native DB backup (additive + reconcile)

- New `spec.backup.appNative` struct: `enabled` (default true), `intervalMinutes`
  (default 60), `retentionDays` (default 7).
- Emit `PAPERCLIP_DB_BACKUP_ENABLED`, `PAPERCLIP_DB_BACKUP_INTERVAL_MINUTES`,
  `PAPERCLIP_DB_BACKUP_RETENTION_DAYS`, and `PAPERCLIP_DB_BACKUP_DIR` pointing
  under the `/paperclip` data PVC.
- The existing operator `pg_dump` -> S3 CronJob (`spec.backup.schedule`/`spec.backup.s3`)
  remains as the offsite path for managed/external Postgres; document that the
  app-native backup is local-dir only and that the two are complementary.
- Docs note: app-native backups are only durable when `spec.storage.persistence.enabled`.

### G. Docs-only (not operator-configurable)

- New `docs/deploy/runtime-configured-features.md` covering: MCP server (stdio
  usage snippet with `PAPERCLIP_API_URL`/`PAPERCLIP_API_KEY`), Modal/Cloudflare
  sandbox credentials (plugin config / company secrets), and SSH Environments
  (UI/API runtime records). No CRD surface for these.
- First-admin: keep `spec.auth.adminUser` bootstrap Job (credentialed path);
  document that omitting it lets the app board-claim flow grant the first human
  ownership in authenticated mode.

## Components and boundaries

- API types (`api/v1alpha1/paperclipinstance_types.go`): field add/remove + CEL.
- Resource builders (`internal/resources/`): env mapping, deletion of `redis.go`,
  edits to `statefulset.go`, `common.go`, `networkpolicy.go`. New builder logic
  for secrets-vault and e2b/backup env lives in `statefulset.go` env helpers
  (pure functions, unit-tested in `resources_test.go`).
- Controller (`internal/controller/`): remove redis reconcile + selfconfig
  allowlist entry. No new reconcile resources (vault/e2b/backup are env-only).
- Generated artifacts: CRDs (`config/crd/bases`), deepcopy, Helm chart CRD
  templates, OLM bundle, `docs/api-reference.md`, samples.

## Testing

- Unit tests (`internal/resources/resources_test.go`): for every changed env
  helper - deployment mode + bind, disableSignUp, removal of redis/managed env,
  AWS vault env (all fields + omitted optionals), E2B key, app-native backup env.
- Conformance (`test/conformance/`): CEL validation cases - `local_trusted`
  with `public` exposure rejected; aws provider missing required fields rejected;
  old enum values (`open`, `single-tenant`) rejected.
- E2E on kind (`test/e2e/`, full app boot), authenticated + managed DB:
  1. Happy path: managed Postgres + `authenticated` + `auth.secretRef`
     (BETTER_AUTH_SECRET) + `auth.adminUser`, real image
     `ghcr.io/paperclipai/paperclip:<latest CalVer tag>`. Assert managed PG
     Ready -> app pod Ready (TCP probe) -> bootstrap Job completes. Assert no
     Redis resources exist and no `PAPERCLIP_MANAGED_*` / `PAPERCLIP_RATE_LIMIT_REDIS_URL`
     env on the pod; assert `PAPERCLIP_BIND`/`PAPERCLIP_BIND_HOST` and
     `PAPERCLIP_DEPLOYMENT_MODE=authenticated` present.
  2. Feature render: an Instance with `secrets.provider=aws_secrets_manager`
     renders `PAPERCLIP_SECRETS_AWS_*`; `adapters.e2b.apiKeySecretRef` renders
     `E2B_API_KEY` from the secret; `backup.appNative` renders `PAPERCLIP_DB_BACKUP_*`.
     (No live AWS/E2B connection asserted.)

## Migration / release

- Breaking: `feat!:` commits. Add an "Upgrading" section to CHANGELOG/docs:
  - `spec.redis` removed - delete it from existing CRs.
  - `spec.adapters.managedInference*` removed - use `spec.adapters.apiKeysSecretRef`.
  - `spec.deployment.mode: open` -> `local_trusted`; `single-tenant` ->
    `authenticated` + `spec.auth.disableSignUp: true`.
- Version bump driven by release-please from the `feat!:` commits.

## Out of scope

- Modal/Cloudflare/SSH/MCP CRD surfaces (not env-configurable).
- gcp_secret_manager / vault secret providers (not runtime-functional upstream).
- Changing the operator's own image or module path.
