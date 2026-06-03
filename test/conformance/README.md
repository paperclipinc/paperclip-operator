# Conformance suite

Conformance categories for the Paperclip `Instance` CR. Each category is a
Ginkgo spec file; the whole suite is gated on `KUBECONFIG` and skipped when it
is unset, so it never breaks the unit or envtest jobs.

| Category            | File                          | Needs a running operator? |
| ------------------- | ----------------------------- | ------------------------- |
| Negative (CEL/schema) | `negative_test.go`          | No (API-server-only, CRD only) |
| Idempotency         | `idempotency_test.go`         | Yes |
| GitOps coexistence  | `gitops_coexistence_test.go`  | Yes |
| Failure modes       | `failure_modes_test.go`       | Yes |
| Upgrade path        | `upgrade_test.go`             | Yes |

The operator-readiness gate (`waitForOperatorAvailable` in `helpers.go`) is
called from the `BeforeAll` of each operator-dependent spec, NOT the global
`BeforeSuite`, so the Negative category runs against a cluster with only the CRD
installed.

TODO: the live-kind / operator-dependent conformance jobs (idempotency,
gitops-coexistence, failure-modes, upgrade) are advisory-on-PR
(`continue-on-error: true`) pending harness hardening for paperclip's
managed-DB workload; flip `continue-on-error` off in
`.github/workflows/conformance.yaml` once they are reliably green. The Negative
job stays required/blocking.
