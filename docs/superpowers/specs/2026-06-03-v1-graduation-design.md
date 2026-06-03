# paperclip-operator: v1 API Graduation Design

- **Status:** Proposed (2026-06-03)
- **Owner:** stubbi (jannes@aqora.io)
- **Repo:** `paperclipinc/paperclip-operator` (Go module `github.com/stubbi/paperclip-operator`)
- **API group:** `paperclip.inc`
- **Current state:** `v1alpha1` only, operator at v0.11, live on OperatorHub with real users
- **Reference:** hermes-operator `docs/api-versioning.md` and `docs/conditions.md` (v1-from-day-one stability contract)
- **Scope:** DESIGN ONLY. This document and the companion plan describe how to graduate the API to a stable `v1`. No conversion or controller code is implemented here.

## 0. TL;DR

paperclip-operator has shipped to v0.11 on `paperclip.inc/v1alpha1` and is in production via OperatorHub. The single managed CRD, `Instance` (kind `Instance`, short `pci`), is mature and its schema has stabilized. This design graduates it to a stable `paperclip.inc/v1` using the standard Kubernetes hub-and-spoke multi-version pattern:

1. Introduce `v1` alongside `v1alpha1`. `v1` becomes the hub type.
2. The v0.11 schema is the shape we want to freeze, so the initial `v1` schema is identical to `v1alpha1`: conversion is a **trivial structural round-trip** (no field renames). A conversion webhook is still required by Kubernetes when a CRD serves more than one version with differing storage, but the conversion functions are mechanical copies.
3. Ship a release (`vX`, targeted v0.12.0) where `v1` is **served but not storage**; `v1alpha1` stays storage. No data moves yet.
4. Run a **storage-version migration** of all existing `Instance` CRs in the field, then flip the CRD storage version to `v1`.
5. Mark `v1alpha1` **deprecated** (served, warned), then eventually **served=false**, observing the minimum overlap window.

Note: paperclip-operator currently ships NO admission webhooks (the PROJECT file declares the `Instance` API with `controller: true` and no `webhooks:` block). Graduating to v1 introduces the operator's FIRST webhook (a conversion webhook). The cert and OLM implications below account for this being a green-field webhook server, which is the main way this graduation differs from openclaw's.

## 1. Context and goals

### Current API surface

| Kind | Scope | Short | Storage version today | Webhooks today |
|---|---|---|---|---|
| `Instance` | Namespaced | `pci` | `v1alpha1` | NONE |

`api/v1alpha1/groupversion_info.go` declares `GroupVersion = {Group: "paperclip.inc", Version: "v1alpha1"}`. The `Instance` type carries `+kubebuilder:subresource:status`, `+kubebuilder:resource:shortName=pci`, and printer columns `Phase`, `Endpoint`, `Age`. There is no second served version and no webhook server today.

### Goals

- **G1:** Promote the public contract to `paperclip.inc/v1` so the hermes API-stability guarantees apply (no breaking changes for the life of v1.x; breaking changes require v2 + conversion + 6 month overlap).
- **G2:** Zero downtime, zero data loss for existing `Instance` objects already stored as `v1alpha1`, including OperatorHub field installs.
- **G3:** No forced user action at upgrade time. `kubectl get pci` keeps working; GitOps manifests pinned to `v1alpha1` keep applying through the deprecation window.
- **G4:** Operator stays runnable locally (in-process, `make run` / single binary) AND under OLM, without bifurcating the conversion or migration design. The new webhook server must come up cleanly in both modes.
- **G5:** Adopt the hermes stability documents: publish a paperclip `docs/api-versioning.md` and `docs/conditions.md` analog when v1 becomes storage.

### Non-goals

- **NG1:** No schema redesign. v1 is a byte-for-byte clone of the frozen v0.11 `v1alpha1` `Instance` shape.
- **NG2:** No new CRD kinds.
- **NG3:** No new admission (validating/defaulting) webhooks as part of this work. Only the conversion webhook is introduced, because the multi-version CRD requires it. Adding admission webhooks is a separate, later decision.
- **NG4:** This is not a v2. No breaking field change. A genuine break follows the hermes v2 path, not this graduation.

## 2. Why graduate, and why now

- v0.11 is mature; the `v1alpha1` label deters conservative adopters and trips corporate policies that refuse alpha CRDs.
- The `Instance` schema has stabilized across releases; freezing it is low risk.
- OperatorHub presence already makes breaking changes expensive, so locking in a stable contract now and routing all future breaks through a real v2 process is strictly better than mutating a depended-on "alpha."

## 3. Hub-and-spoke model

Exactly one **hub** (the controller's working type and the storage type) and any number of **spokes** that convert to/from the hub.

Decision: **`v1` is the hub.** `v1alpha1` becomes a spoke implementing `ConvertTo(hub)` / `ConvertFrom(hub)`.

Rationale: the controller and `internal/resources/` builders should operate on the long-lived type; the alpha type is slated for removal, so it carries the conversion glue, not the controller's working type. The end state matches hermes: `v1` hub+storage, with a legacy `v1alpha1` spoke awaiting retirement.

Concretely (described, not implemented here):
- New `api/v1/` package: `paperclipinstance_types.go`, `groupversion_info.go` (`Version: "v1"`), generated deepcopy.
- `api/v1.Instance` carries `+kubebuilder:object:root=true`, `+kubebuilder:subresource:status`, `+kubebuilder:resource:shortName=pci`, and the same printer columns (`Phase`, `Endpoint`, `Age`). It gains `+kubebuilder:storageversion` only at the storage-flip phase.
- `api/v1alpha1.Instance` implements `conversion.Convertible` against the `v1` hub: a field-by-field copy (mechanical; identical schema). A round-trip fuzz test (`TestConvertRoundTrip`) asserts `v1alpha1 -> v1 -> v1alpha1` is lossless.
- The controller's manager registers both schemes; the `Instance` reconciler switches its working type to `api/v1`.

## 4. Storage version: which, and when to flip

| Phase | Served versions | Storage version | Notes |
|---|---|---|---|
| Today (v0.11) | `v1alpha1` | `v1alpha1` | single version, no webhook |
| Phase A (v0.12.0) | `v1alpha1`, `v1` | `v1alpha1` | `v1` served-not-storage; FIRST webhook (conversion) live; no data moves |
| Phase B (v0.12.x) | `v1alpha1`, `v1` | `v1alpha1` -> migrate -> `v1` | migrate existing CRs, THEN flip `storage: true` to `v1` |
| Phase C (v0.13.0) | `v1alpha1` (deprecated), `v1` | `v1` | `v1alpha1` `deprecated: true` + `deprecationWarning` |
| Phase D (>= Phase C + 6 months) | `v1` (`v1alpha1` served=false) | `v1` | apply against `v1alpha1` rejected with a pointer to conversion |
| Phase E (later) | `v1` | `v1` | `v1alpha1` removed from the CRD |

The storage flip in Phase B is the most delicate step. Rule: **never flip the storage version until every stored object has been re-encoded at the new version.** Migrate, confirm the CRD's `status.storedVersions` no longer lists `v1alpha1`, then flip. Flipping first risks unrecoverable objects whose stored bytes are the old version after the old schema is gone.

## 5. Conversion webhook and OLM cert implications

paperclip-operator has no webhook server today, so this section introduces one. This is the largest single delta of the graduation.

### Conversion webhook

A conversion webhook is mandatory once the CRD serves two versions that can differ in stored form. Even with a trivial round-trip the API server calls the webhook to translate between served and stored versions (for example, `kubectl get pci.v1` while the stored object is still `v1alpha1` during Phase A/B).

- Stand up a webhook server in the operator Deployment (controller-runtime's webhook server; add the serving port and a readiness probe). This is new infrastructure for paperclip-operator.
- `config/crd/patches/webhook_in_paperclip_instances.yaml`: `spec.conversion.strategy: Webhook` referencing the operator service.
- `config/crd/patches/cainjection_in_paperclip_instances.yaml`: cert-manager CA injection into `conversion.webhook.clientConfig.caBundle`.
- Add `cert-manager` `Certificate` + `Issuer` templates to the Helm chart (new, since there was no webhook before) and a `Service` for the webhook port.

### Local vs OLM cert story (G4)

- **Local / `make run`:** controller-runtime generates a self-signed serving cert into a temp dir for the local webhook server; the local kustomize overlay injects that CA into the CRD conversion block. `make run` must now bring up the webhook server; the design adds a local cert bootstrap so `make run` works with no cluster cert infra.
- **Plain manifests / Helm:** cert-manager issues the serving cert and injects the CA into the CRD `conversion` block. New chart templates: `Certificate`, `Issuer` (or reuse a shared selfsigned issuer), webhook `Service`. Helm RBAC Sync and chart cert wiring stay in sync via `make manifests` + `make sync-chart-crds`.
- **OLM / OperatorHub:** OLM does NOT use cert-manager. OLM provisions and rotates the serving cert and injects the CA into webhook configs declared in the CSV `spec.webhookdefinitions`. For conversion specifically, the CSV gains a `type: ConversionWebhook` definition listing `instances` and the conversion path/port; OLM owns the cert and patches the CRD `conversion.webhook.clientConfig.caBundle`. Therefore:
  - We must NOT ship a cert-manager `Certificate` for the conversion webhook inside the OLM bundle; OLM and cert-manager would fight over the caBundle. `make bundle` strips cert-manager objects; OLM injects its own.
  - OLM requires the conversion webhook's CRD to be **owned** by the CSV. `instances.paperclip.inc` is already an owned CRD, so this is additive.
  - This is paperclip-operator's first `webhookdefinitions` entry; `operator-sdk bundle validate` and the deployment's webhook port/Service wiring must be exercised end to end (the plan gates on this).

### Failure-mode note

A conversion webhook sits on the read/write path of every `Instance`. If down, `kubectl get pci` fails for the affected versions. Mitigations: the handler is a pure in-process struct copy (no external deps); the webhook server shares the operator Deployment with a readiness probe; `v1alpha1` stays storage until migration completes, so a webhook outage in Phase A degrades the `v1` read view but never blocks writes to native `v1alpha1` storage.

## 6. Storage-version migration for existing CRs

Existing clusters have `Instance` objects stored as `v1alpha1`. After Phase A the CRD still stores `v1alpha1`; before flipping to `v1` (Phase B) every stored object must be re-encoded as `v1`.

Two mechanisms (the plan recommends StorageVersionMigration where available and the one-shot Job as the universal fallback, satisfying G4):

1. **`StorageVersionMigration` (storage-version-migrator).** On clusters running kube-storage-version-migrator (GKE, OpenShift, OLM-managed clusters where available), create a `migration.k8s.io/v1alpha1` `StorageVersionMigration` for `instances.paperclip.inc`. The controller lists and no-op-updates every object, forcing re-encode at the current storage version. Sequence AFTER the webhook is live (Phase A), BEFORE the storage flip.
2. **One-shot migrate Job (universal + local).** A `Job` (operator binary `migrate` subcommand, or `kubectl get instances -A -o yaml | kubectl replace`) listing all `Instance` CRs across namespaces and issuing a no-op patch to force re-encode. Works on any cluster including local kind/minikube, and is the `make run` / single-binary local path. Its RBAC (`get`, `list`, `patch` on `instances`) is added via `+kubebuilder:rbac` markers so `config/rbac` and the Helm chart RBAC stay in sync.

After either path, verify on the CRD: `status.storedVersions` lists only `v1` (the API server prunes `v1alpha1` once no stored object uses it). If `v1alpha1` lingers, an object was missed: do not flip.

For OperatorHub installs, the migration runs idempotently on operator startup (gated by a CRD annotation marking completion) so field installs migrate hands-off (G3).

## 7. CRD served / deprecated version lifecycle

Per-version markers on the CRD (driven by kubebuilder markers on the Go types; regenerated via `make manifests` + `make sync-chart-crds` + `make sync-bundle-crds`):

- Phase A: `v1alpha1 {served: true, storage: true}`, `v1 {served: true, storage: false}`.
- Phase B: after migration, `v1alpha1 {served: true, storage: false}`, `v1 {served: true, storage: true}`.
- Phase C: `v1alpha1 {served: true, storage: false, deprecated: true, deprecationWarning: "paperclip.inc/v1alpha1 is deprecated; use paperclip.inc/v1. See https://github.com/paperclipinc/paperclip-operator/blob/main/docs/api-versioning.md"}`.
- Phase D: `v1alpha1 {served: false, storage: false, deprecated: true}`.
- Phase E: `v1alpha1` removed from `spec.versions`.

`deprecated`/`deprecationWarning` make `kubectl` print a warning on every `v1alpha1` op in Phase C, a loud non-fatal nudge before Phase D makes it fatal.

## 8. OperatorHub bundle and channel implications

- **Owned CRD in the CSV:** `instances.paperclip.inc` is already owned; the CSV `spec.customresourcedefinitions.owned` entry gains `version: v1` and the new conversion `webhookdefinitions` entry (Section 5).
- **`replaces` / channel:** the v0.12.0 bundle `replaces` v0.11.x as usual. paperclip-operator uses a single `stable` channel (confirmed in `bundle/metadata/annotations.yaml`). Graduating the API version does NOT require a new channel; operator semver is independent of API group version (hermes policy decouples API version, image semver, chart semver).
- **Upgrade ordering on OperatorHub:** OLM upgrades the operator Deployment first (bringing the conversion webhook online), then applies the CRD with both versions. Phase A keeps `v1alpha1` as storage, so OLM's CRD-update safety check (refuses dropping the current storage version) passes. The storage flip ships in a later bundle only after migration.
- **First webhook on OperatorHub:** because this is paperclip-operator's first webhook, the v0.12.0 CSV is the first to declare `webhookdefinitions`. Validate that OLM provisions the cert and the deployment exposes the webhook Service/port. `operator-sdk bundle validate` gates this.
- **community-operators submission:** the existing CI that submits the OLM bundle to RedHat (see repo history, e.g. #64) carries the updated bundle; no structural pipeline change.

## 9. Backward-compatibility guarantees

- **Reads:** `kubectl get pci`, `kubectl get instances.v1alpha1.paperclip.inc`, and `kubectl get instances.v1.paperclip.inc` all return the same object, transparently converted, through Phase D.
- **Writes:** manifests/GitOps pinned to `apiVersion: paperclip.inc/v1alpha1` keep applying through Phase C (with a deprecation warning) and stop only at Phase D, after >= 6 months overlap.
- **No field semantics change.** v1 is a schema clone; a `v1alpha1` object and its `v1` projection are field-identical.
- **Conditions preserved.** The `Instance` status (`Phase`, `Endpoint`, and any conditions surfaced in printer columns) carries over unchanged; once on v1 it becomes part of the published stability contract (paperclip `docs/conditions.md`).
- **Short name `pci` preserved.** Removing it would be breaking per the hermes policy.

## 10. Phased rollout summary

1. **vX = v0.12.0 (additive):** add `api/v1` hub type, `v1alpha1` spoke conversion, FIRST webhook server + conversion plumbing (kustomize patches + Helm cert templates + Service + CSV ConversionWebhook), CSV owned-CRD `v1` entry. `v1` served-not-storage. Round-trip + envtest conversion tests green. No data moves.
2. **Migrate (v0.12.x):** ship the storage-version migration (StorageVersionMigration where available; idempotent startup migration for OperatorHub; one-shot Job for local/manual). Confirm no stored `v1alpha1` objects remain.
3. **Flip storage (v0.12.x or v0.13.0):** set `v1` `storage: true`, only after migration confirms no stored `v1alpha1` objects.
4. **Deprecate v1alpha1 (v0.13.0):** `deprecated: true` + warning. Publish `docs/api-versioning.md` and `docs/conditions.md` (the paperclip analogs of the hermes references).
5. **Stop serving v1alpha1 (>= step 4 + 6 months):** `served: false`.
6. **Remove v1alpha1 (later):** drop from `spec.versions`.

## 11. Risks and rollback

| Risk | Likelihood | Impact | Mitigation / rollback |
|---|---|---|---|
| First-ever webhook server fails to come up (no prior webhook infra) | Medium | High | Add the webhook server + readiness probe; exercise in envtest and on local kind before release. Phase A keeps `v1alpha1` as storage so writes do not depend on conversion. Rollback: redeploy v0.11 (no webhook), CRD reverts to single version. |
| Conversion webhook down blocks CR reads | Low | High | Pure in-process copy, no external deps; readiness-gated; `v1alpha1` stays storage through Phase A. Rollback: redeploy prior image. |
| Storage flipped before migration complete (data loss) | Low | Critical | Hard gate: verify CRD `status.storedVersions` has no `v1alpha1` before flipping; flip is a separate PR/release from the migration. Not reversible once old schema gone, hence the gate. |
| OLM and cert-manager both manage conversion caBundle | Medium | Medium | Bundle build strips cert-manager certs; OLM owns conversion certs via `ConversionWebhook` definition; Helm/manifest install uses cert-manager only. Documented divergence. |
| OperatorHub refuses CRD update (drops current storage version) | Low | Medium | Phase A keeps `v1alpha1` as storage; storage flip ships only after migration. |
| Round-trip conversion lossy (field not copied) | Low | High | Identical schemas by construction; `TestConvertRoundTrip` fuzz test in `api/` gates every change; CI conversion test in envtest. |
| Users hard-pin `v1alpha1` and miss deprecation | Medium | Low | 6 month overlap, `kubectl` deprecation warnings in Phase C, release notes, OperatorHub description note. |
| Local installs lack storage-version-migrator | Medium | Low | One-shot migrate Job + idempotent startup migration both work with no cluster add-on (G4). |

**Rollback posture:** every phase before the storage flip (Phase B) is reversible by redeploying the prior operator image, because `v1alpha1` stays the storage version and no data has moved. The storage flip is the point of no return; gated on a verified-complete migration and shipped as its own release so it can be held back independently if verification fails on any cluster.

## 12. Reference

- hermes-operator `docs/api-versioning.md`: the stability contract paperclip adopts on v1 (non-breaking vs breaking lists, v2 + 6-month overlap rule, decoupled API/image/chart semver).
- hermes-operator `docs/conditions.md`: the conditions-catalogue shape paperclip publishes at Phase C.
- Kubernetes CRD versioning: https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/
- Kubebuilder multiversion / conversion: https://book.kubebuilder.io/multiversion-tutorial/conversion.html
- kube-storage-version-migrator: https://github.com/kubernetes-sigs/kube-storage-version-migrator
- OLM webhook (incl. ConversionWebhook) support: https://olm.operatorframework.io/docs/advanced-tasks/adding-admission-and-conversion-webhooks/
