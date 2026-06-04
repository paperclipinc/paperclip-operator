---
title: Upgrading
summary: Breaking changes and migration steps between operator releases
---

# Upgrading

## Config realignment (breaking)

This release realigns the operator with the Paperclip app's real configuration
contract. It removes configuration the app never consumed and corrects an
invalid enum. Migrate existing `Instance` resources **before** upgrading the
operator.

### 1. `spec.redis` removed

The Paperclip app bundles no Redis client and reads no Redis environment
variable; the operator's managed Redis was inert. Remove any `spec.redis` block
from your `Instance` resources. Existing managed-Redis `StatefulSet`/`Service`/
`PVC` objects are owned by the `Instance` and are garbage-collected once the
field is gone and the operator reconciles.

Horizontal scaling is unaffected: it relies on shared PostgreSQL
(`spec.database` managed/external), object storage (`spec.objectStorage`), the
HPA/PodDisruptionBudget/topology-spread settings, and pod-0 heartbeat gating.

### 2. `spec.adapters.managedInference*` removed

`managedInferenceSecretRef`, `managedInferenceProvider`, and
`managedInferenceModel` produced `PAPERCLIP_MANAGED_*` env vars that the app
does not read. Remove them. Supply LLM provider keys via
`spec.adapters.apiKeysSecretRef` (a Secret with `ANTHROPIC_API_KEY` and/or
`OPENAI_API_KEY`); the app discovers available models automatically.

### 3. `spec.deployment.mode` enum changed

The app only accepts `local_trusted` and `authenticated`. Migrate:

| Old value | New value |
|-----------|-----------|
| `open` | `local_trusted` (requires `spec.deployment.exposure: private`) |
| `authenticated` | `authenticated` (unchanged) |
| `single-tenant` | `authenticated` plus `spec.auth.disableSignUp: true` |

### 4. Bind env modernized (no action required)

The operator now sets `PAPERCLIP_BIND=custom` + `PAPERCLIP_BIND_HOST=0.0.0.0`
instead of the legacy `HOST`. This is internal to the generated pod spec; no CR
change is needed.

### New, optional features

- `spec.secrets.provider: aws_secrets_manager` + `spec.secrets.aws` - store
  secrets in AWS Secrets Manager (credentials via IRSA).
- `spec.adapters.e2b.apiKeySecretRef` - E2B sandbox provider API key.
- `spec.backup.appNative` - Paperclip's built-in local-dir DB backups.

See also `docs/deploy/runtime-configured-features.md` for capabilities
configured in the app UI rather than the operator.
