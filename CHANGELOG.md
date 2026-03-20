# Changelog

## [0.2.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.1.0...v0.2.0) (2026-03-20)


### Features

* migrate to paperclipinc org and add upstream image build workflow ([5eeb3d2](https://github.com/paperclipinc/paperclip-operator/commit/5eeb3d2cc9fc47b65b30bdd14d79b1ffcf8ee2c8))


### Bug Fixes

* correct Docker image name in release workflow ([551ee4e](https://github.com/paperclipinc/paperclip-operator/commit/551ee4ea18b7f30cdf2337877fa70dcb6c52dfbf))
* correct RBAC kustomization filenames for CRD roles ([1aa89b1](https://github.com/paperclipinc/paperclip-operator/commit/1aa89b113b82677eb5ab976703e272f7deb529d1))
* define DB_PASSWORD before DATABASE_URL for env var substitution ([ef07763](https://github.com/paperclipinc/paperclip-operator/commit/ef077637df138aa2dae7f0792c8393a500c6082a))
* propagate nodeSelector and tolerations to database StatefulSet ([7db4e83](https://github.com/paperclipinc/paperclip-operator/commit/7db4e83823e32690b85ded6ea1f2a5546dbdd9d6))

## [0.1.0](https://github.com/paperclipinc/paperclip-operator/releases/tag/v0.1.0) (2026-03-19)

### Features

* Initial release of the Paperclip Kubernetes Operator
* Instance CRD with comprehensive configuration (image, database, auth, storage, networking, security, scaling, observability)
* Managed PostgreSQL mode with auto-generated credentials
* External database support via connection string or Secret reference
* Persistent storage with configurable PVC
* S3-compatible object storage for multi-replica deployments
* Ingress with WebSocket support for real-time UI updates
* NetworkPolicy with deny-all baseline
* HPA and PDB for availability
* Health probes against /api/health
* LLM API key injection from Kubernetes Secrets
* Helm chart for operator deployment
* Prometheus metrics for reconciliation monitoring
