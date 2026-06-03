# Paperclip Operator

The Paperclip Operator is a Kubernetes operator for deploying and managing
[Paperclip](https://paperclip.inc) instances with production-grade security,
observability, and lifecycle management.

It manages a single custom resource, `Instance` (short name `pci`), in the
`paperclip.inc` API group. From one `Instance` the operator reconciles the
server StatefulSet, Service, optional managed PostgreSQL and Redis,
persistence, networking (Ingress or Gateway API HTTPRoute), RBAC,
NetworkPolicy, autoscaling, disruption budgets, backups, and observability
resources.

## Features

- Single-CR deployment of the Paperclip Node.js application (port 3100,
  health endpoint `/api/health`).
- Managed or external PostgreSQL and Redis.
- Persistent storage with optional S3-compatible object storage and scheduled
  backups.
- Networking via Service plus optional Ingress or Gateway API HTTPRoute.
- Security defaults: non-root pod, dropped capabilities, NetworkPolicy
  isolation, and PID-namespace sharing for zombie-process reaping.
- Scale-to-zero suspend via `spec.suspended`.
- Observability: Prometheus metrics, optional ServiceMonitor, PrometheusRule
  alerts, and Grafana dashboard ConfigMaps.
- High availability: HorizontalPodAutoscaler, PodDisruptionBudget, topology
  spread, and pod scheduling controls.

## Quick start

```yaml
apiVersion: paperclip.inc/v1alpha1
kind: Instance
metadata:
  name: paperclip
spec:
  image:
    tag: v1.0.0
  deployment:
    mode: authenticated
    exposure: private
  database:
    mode: managed
  storage:
    persistence:
      enabled: true
      size: 5Gi
```

Apply it to a namespace where the operator is installed:

```sh
kubectl apply -f instance.yaml
kubectl get pci
```

## API reference

The full, auto-generated API reference for every field of the `Instance`
custom resource is available in the [API Reference](api-reference.md). It is
regenerated from the Go API types with `make api-docs`.
