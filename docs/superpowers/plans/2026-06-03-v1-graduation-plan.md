# paperclip-operator: v1 API Graduation Migration Plan

- **Status:** Proposed (2026-06-03)
- **Owner:** stubbi (jannes@aqora.io)
- **Companion spec:** `docs/superpowers/specs/2026-06-03-v1-graduation-design.md`
- **Scope:** Implementation plan and checklists. NO code is implemented in this PR; this is the executable runbook for the graduation work that follows.

Each phase is a separate PR (Conventional Commits, one feature per commit, each ending with the required `Co-Authored-By` trailer). Phases that change API types MUST run `make generate && make manifests && make sync-chart-crds && make sync-bundle-crds` and commit regenerated files. New managed resources (the webhook Service, the migration Job RBAC) get `+kubebuilder:rbac` markers and a `make manifests` re-run so `config/rbac` and the Helm chart RBAC stay in sync.

Kind in scope: `Instance` (kind `Instance`, short `pci`, group `paperclip.inc`). This is the operator's only CRD and its FIRST webhook is introduced here.

## Guardrails (apply to every phase)

- No em/en dashes anywhere (ASCII only). Run `grep -rnP '[\x{2013}\x{2014}]'` on changed files; must be clean.
- Use `controllerutil.CreateOrUpdate` for any managed resource (Reconcile Guard CI forbids bare `r.Update`).
- Resource builders stay pure functions in `internal/resources/` with unit tests; controllers in `internal/controller/`.
- After any API type change: `make generate && make manifests && make sync-chart-crds && make sync-bundle-crds` (and `api-docs` if present); commit regenerated output.
- Add `+kubebuilder:rbac` markers for the migration Job's permissions; re-run `make manifests` (Helm RBAC Sync check).
- Validate before each PR: `go build ./...`; `go vet ./...`; `make lint` (best effort); `go test ./internal/resources/... ./api/...`; `make test` (envtest, best effort, CI runs it); helm sync hack scripts; `helm lint`; `operator-sdk bundle validate` if the CSV changed.

## Phase 0: groundwork (this PR, docs only)

- [x] Design spec committed (`specs/2026-06-03-v1-graduation-design.md`).
- [x] This plan committed.
- Outcome: agreed migration path. No code.

## Phase A: introduce v1 hub, v1alpha1 spoke, FIRST webhook (target v0.12.0)

Goal: `v1` served-not-storage; conversion webhook (the operator's first webhook) live; no data moves.

1. **Scaffold `api/v1`.**
   - Hand-add `api/v1/` mirroring `api/v1alpha1/` exactly, or `operator-sdk create api --group paperclip --version v1 --kind Instance`. Schema is a byte-for-byte clone of the frozen v0.11 `v1alpha1.Instance`.
   - `api/v1/groupversion_info.go`: `Version: "v1"`, `Group: "paperclip.inc"`.
   - Mark `api/v1.Instance` `+kubebuilder:object:root=true`, `+kubebuilder:subresource:status`, `+kubebuilder:resource:shortName=pci`, and copy printer columns (`Phase`, `Endpoint`, `Age`) verbatim.
   - Do NOT add `+kubebuilder:storageversion` to v1 yet.
2. **Conversion glue on the spoke.**
   - Implement `ConvertTo` / `ConvertFrom` on `api/v1alpha1.Instance` against the `api/v1` hub (field-by-field copy; identical schema).
   - Move the reconciler and `internal/resources/` builders to operate on `api/v1`. Builders stay pure; only the imported type version changes. Update `internal/resources/` unit tests.
3. **Stand up the webhook server (NEW for this operator).**
   - Add the controller-runtime webhook server to the manager in `cmd/`; expose the serving port; add a readiness probe.
   - Register the conversion webhook for `Instance` on that server.
   - Local cert bootstrap so `make run` brings up the webhook with a self-signed cert and no cluster cert infra.
4. **Conversion webhook plumbing.**
   - `config/crd/patches/webhook_in_paperclip_instances.yaml` (`spec.conversion.strategy: Webhook`).
   - `config/crd/patches/cainjection_in_paperclip_instances.yaml` (cert-manager CA injection).
   - Helm chart: NEW templates for cert-manager `Certificate`, `Issuer` (or shared selfsigned issuer), webhook `Service`, and the CRD `conversion.webhook.clientConfig.caBundle` wiring. Add `+kubebuilder:rbac` if the webhook needs any new verbs; re-run `make manifests` for Helm RBAC sync.
   - CSV: add the operator's first `webhookdefinitions` entry, `type: ConversionWebhook`, for `instances`. Ensure cert-manager `Certificate` objects are stripped from the OLM bundle (OLM owns conversion certs).
5. **Regenerate + sync.** `make generate manifests sync-chart-crds sync-bundle-crds`. Commit `config/crd/bases/*`, chart CRDs, bundle CRDs, CSV, new chart cert/Service templates.
6. **Tests.**
   - `api/` round-trip fuzz test `TestConvertRoundTrip` (`v1alpha1 -> v1 -> v1alpha1` lossless).
   - envtest: create a `v1alpha1` Instance, read it as `v1`, assert equality; and vice versa. Assert the webhook server comes up.
7. **Validate + PR.** Full gate list incl. `operator-sdk bundle validate`. PR title `feat(api): introduce paperclip.inc/v1 served-not-storage with conversion webhook`. Base `main`. Do NOT enable auto-merge.

Exit criteria: cluster serves both versions; `kubectl get pci.v1.paperclip.inc` works; etcd still stores `v1alpha1`; webhook server healthy; rollback = redeploy v0.11.

## Phase B: migrate stored CRs, then flip storage (target v0.12.x)

### B1: migration mechanism (ship first, do NOT flip yet)

1. **One-shot migrate Job (universal + local).**
   - Add a `migrate` subcommand to the operator binary (or a Job doing `list + no-op patch` across all namespaces for `instances`).
   - Job RBAC via `+kubebuilder:rbac:groups=paperclip.inc,resources=instances,verbs=get;list;patch`. Re-run `make manifests`; commit `config/rbac` and Helm RBAC.
   - Idempotent: gated by a CRD annotation `paperclip.inc/storage-migration-complete`.
2. **StorageVersionMigration CR (where supported).** Generate a `migration.k8s.io/v1alpha1` `StorageVersionMigration` for `instances.paperclip.inc`; document as the preferred path on GKE/OpenShift/OLM-managed clusters.
3. **OperatorHub hands-off path.** Operator runs the idempotent migration on startup (annotation-gated).

PR: `feat: storage-version migration for paperclip Instance CRs`. No storage flip in this PR.

### B2: storage flip (separate PR/release, gated)

1. Verify CRD `status.storedVersions` no longer contains `v1alpha1` after migration. If it lingers, STOP and re-run migration.
2. Move `+kubebuilder:storageversion` from `api/v1alpha1.Instance` to `api/v1.Instance`. Regenerate + sync all manifests/chart/bundle.
3. Validate; PR `feat(api)!: flip storage version to paperclip.inc/v1`.

Exit criteria: `v1` is storage; all objects stored as `v1`; `v1alpha1` still served.

POINT OF NO RETURN: after B2 the old schema is no longer the storage form. The gate in B2 step 1 is mandatory.

## Phase C: deprecate v1alpha1 + publish stability docs (target v0.13.0)

1. Mark `api/v1alpha1.Instance` `+kubebuilder:deprecatedversion` with a warning pointing at `docs/api-versioning.md`. Regenerate; CRD gains `deprecated: true` + `deprecationWarning`.
2. Author `docs/api-versioning.md` and `docs/conditions.md` for paperclip, modeled on the hermes references: non-breaking vs breaking change lists, the v2 + 6-month-overlap rule, decoupled API/image/chart semver, and the `Instance` conditions/status catalogue (`Phase`, `Endpoint`, conditions).
3. Release notes + OperatorHub description: announce `v1` as stable, `v1alpha1` deprecated with removal timeline.
4. Validate; PR `feat(api): deprecate paperclip.inc/v1alpha1; publish v1 stability contract`.

Exit criteria: `kubectl` warns on every `v1alpha1` op; v1 contract published.

## Phase D: stop serving v1alpha1 (>= Phase C + 6 months)

1. Set `api/v1alpha1.Instance` `served=false` (`+kubebuilder:unservedversion`). Keep it in the CRD for stored-version bookkeeping.
2. Regenerate + sync. PR `feat(api)!: stop serving paperclip.inc/v1alpha1`.

Exit criteria: applying `v1alpha1` returns a clear error; only `v1` served.

## Phase E: remove v1alpha1 (later, optional)

1. Delete `api/v1alpha1/` and its CRD `spec.versions` entry; drop conversion glue. Regenerate + sync.
2. PR `chore(api): remove paperclip.inc/v1alpha1`.

Exit criteria: CRD serves only `v1`; paperclip matches the hermes end state.

## Validation matrix (run per phase, capture into PR notes)

| Check | Command |
|---|---|
| Build | `go build ./...` |
| Vet | `go vet ./...` |
| Lint | `make lint` (best effort) |
| Unit | `go test ./internal/resources/... ./api/...` |
| Envtest | `make test` (best effort; CI runs it) |
| Helm CRD sync | hack sync scripts |
| Helm lint | `helm lint` |
| Bundle | `operator-sdk bundle validate ./bundle` (CSV changed in Phase A) |
| Dash scan | `grep -rnP '[\x{2013}\x{2014}]'` on changed files (must be clean) |

## Rollback summary

- Phases A, B1, C: redeploy prior operator image; `v1alpha1` remains storage (A/B1) or served (C). Fully reversible. Note Phase A also reverts the operator to webhook-free (v0.11) on rollback.
- Phase B2 (storage flip): NOT reversible once old schema gone. Gated on verified-complete migration; shipped as its own release so it can be held back if any cluster fails verification.
- Phases D, E: reversible by re-adding `served=true` / re-adding the version, since `v1` remains storage throughout.
