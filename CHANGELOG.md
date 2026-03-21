# Changelog

## [0.2.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.1.0...v0.2.0) (2026-03-21)


### Features

* add automatic admin user bootstrap via spec.auth.adminUser ([daf5731](https://github.com/paperclipinc/paperclip-operator/commit/daf57311362c9fa75269381b604620992d7b6865))
* add onboarding init container for automatic admin bootstrap ([2680aee](https://github.com/paperclipinc/paperclip-operator/commit/2680aee15f9116556c97f32cf8fd8fe3468a70db))
* migrate to paperclipinc org and add upstream image build workflow ([5eeb3d2](https://github.com/paperclipinc/paperclip-operator/commit/5eeb3d2cc9fc47b65b30bdd14d79b1ffcf8ee2c8))
* production-ready horizontal scaling and multi-replica support ([2e9065d](https://github.com/paperclipinc/paperclip-operator/commit/2e9065d5441bf72eeba617f183874993b880bd47))


### Bug Fixes

* bootstrap job health check for authenticated mode ([41654d3](https://github.com/paperclipinc/paperclip-operator/commit/41654d3dd7244ed4a3c3683f2050dad523bfbeb3))
* correct Docker image name in release workflow ([551ee4e](https://github.com/paperclipinc/paperclip-operator/commit/551ee4ea18b7f30cdf2337877fa70dcb6c52dfbf))
* correct gofmt formatting in database.go ([c5b707a](https://github.com/paperclipinc/paperclip-operator/commit/c5b707a9cbe222e6106242550ae1e3582bd967a3))
* correct RBAC kustomization filenames for CRD roles ([1aa89b1](https://github.com/paperclipinc/paperclip-operator/commit/1aa89b113b82677eb5ab976703e272f7deb529d1))
* define DB_PASSWORD before DATABASE_URL for env var substitution ([ef07763](https://github.com/paperclipinc/paperclip-operator/commit/ef077637df138aa2dae7f0792c8393a500c6082a))
* implement correct Paperclip admin bootstrap flow ([3c63d3d](https://github.com/paperclipinc/paperclip-operator/commit/3c63d3d43f563f48b04f33d92917244ec1333c3d))
* kill onboard server process after config creation ([c47b5de](https://github.com/paperclipinc/paperclip-operator/commit/c47b5de384b666d46ed921e77bf080e1048333be))
* prevent onboard init container from starting the server ([c269fc8](https://github.com/paperclipinc/paperclip-operator/commit/c269fc8c75b7a664b618eea941c747752ce85551))
* propagate nodeSelector and tolerations to database StatefulSet ([7db4e83](https://github.com/paperclipinc/paperclip-operator/commit/7db4e83823e32690b85ded6ea1f2a5546dbdd9d6))
* use curl instead of wget in bootstrap job ([1b1a117](https://github.com/paperclipinc/paperclip-operator/commit/1b1a117964b2a1f8f416d33cb6e3d529ab4f5897))
* use kill -9 and pkill to terminate onboard process tree ([e496aaa](https://github.com/paperclipinc/paperclip-operator/commit/e496aaa5937d6554a5749cc3436bbf95913f304a))
* use public URL for all bootstrap API calls ([03bc4f2](https://github.com/paperclipinc/paperclip-operator/commit/03bc4f26e7081010cf49f9d7681bc3f6010066aa))
* use server-side apply for CRD installation ([99b767d](https://github.com/paperclipinc/paperclip-operator/commit/99b767d47d62485e59643d77fdccc4b555afac63))

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
