# Paperclip Kubernetes Operator

A Kubernetes operator for deploying and managing [Paperclip](https://github.com/paperclipai/paperclip) instances -- the open-source AI agent orchestration platform.

## Overview

The Paperclip Operator automates the deployment and lifecycle management of Paperclip instances on Kubernetes. It manages:

- **Paperclip server** as a StatefulSet with health probes and persistent storage
- **PostgreSQL database** (operator-managed, external, or embedded PGlite)
- **Networking** -- Service, Ingress (with WebSocket support), NetworkPolicy
- **Security** -- Pod/container security contexts, RBAC, network isolation
- **Scaling** -- HPA, PDB, node scheduling constraints
- **Secrets** -- Auto-generated database credentials, LLM API key injection

## Quick Start

### Install the operator

```bash
# Via Helm
helm install paperclip-operator oci://ghcr.io/paperclipinc/charts/paperclip-operator

# Or via kubectl
kubectl apply -f https://github.com/paperclipinc/paperclip-operator/releases/latest/download/install.yaml
```

### Create a Paperclip instance

```yaml
apiVersion: paperclip.inc/v1alpha1
kind: Instance
metadata:
  name: my-paperclip
spec:
  image:
    tag: latest
  deployment:
    mode: authenticated
  database:
    mode: managed
  auth:
    secretRef:
      name: paperclip-auth
      key: BETTER_AUTH_SECRET
  adapters:
    apiKeysSecretRef:
      name: paperclip-api-keys
  storage:
    persistence:
      enabled: true
      size: 5Gi
```

```bash
kubectl apply -f my-paperclip.yaml
```

### Check status

```bash
kubectl get instances
# or shorthand:
kubectl get pci
```

## Features

| Feature | Description |
|---------|-------------|
| **Managed PostgreSQL** | Operator provisions and manages PostgreSQL 17 StatefulSet |
| **External Database** | Connect to existing PostgreSQL via connection string or Secret reference |
| **Persistent Storage** | PVC for `/paperclip` data directory |
| **S3 Object Storage** | S3/MinIO/R2 for multi-replica deployments |
| **Ingress + WebSocket** | Full Ingress support with WebSocket annotations |
| **Network Policies** | Deny-all baseline with selective allow rules |
| **Auto-scaling** | HPA with CPU/memory targets |
| **Pod Disruption Budget** | Availability protection during maintenance |
| **Health Probes** | Liveness, readiness, and startup probes against `/api/health` |
| **LLM API Keys** | Inject Anthropic, OpenAI keys from Kubernetes Secrets |
| **Secrets Management** | Support for Paperclip's encrypted secrets with master key |
| **Custom Sidecars** | Add sidecar containers and init containers |

## Configuration Reference

See [config/samples/](config/samples/) for complete examples.

### Database Modes

| Mode | Description |
|------|-------------|
| `managed` (default) | Operator creates PostgreSQL 17 StatefulSet + PVC + credentials Secret |
| `external` | Provide `DATABASE_URL` directly or via Secret reference |
| `embedded` | Uses PGlite (single-node only, good for development) |

### Deployment Modes

| Mode | Description |
|------|-------------|
| `authenticated` (default) | Requires login via Better Auth |
| `open` | No authentication required |
| `single-tenant` | Single-user mode |

## Development

### Prerequisites

- Go 1.25+
- Docker
- kubectl
- A Kubernetes cluster (kind, minikube, or remote)

### Build and run locally

```bash
make install      # Install CRDs
make run          # Run operator against current kubeconfig
```

### Run tests

```bash
make test                              # All tests
go test ./internal/resources/ -v       # Fast unit tests only
```

### Build Docker image

```bash
make docker-build IMG=my-registry/paperclip-operator:dev
```

## License

Apache License 2.0
