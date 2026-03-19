# Changelog

## [0.1.0](https://github.com/openclaw-rocks/paperclip-operator/releases/tag/v0.1.0) (2026-03-19)

### Features

* Initial release of the Paperclip Kubernetes Operator
* PaperclipInstance CRD with comprehensive configuration (image, database, auth, storage, networking, security, scaling, observability)
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
