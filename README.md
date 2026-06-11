# Paperclip Kubernetes Operator

[![CI](https://github.com/paperclipinc/paperclip-operator/actions/workflows/ci.yaml/badge.svg?branch=main)](https://github.com/paperclipinc/paperclip-operator/actions/workflows/ci.yaml?query=branch%3Amain)
[![Go Report Card](https://goreportcard.com/badge/github.com/paperclipinc/paperclip-operator)](https://goreportcard.com/report/github.com/paperclipinc/paperclip-operator)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.28%2B-326CE5?logo=kubernetes&logoColor=white)](https://kubernetes.io)
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8?logo=go&logoColor=white)](https://go.dev)

**Deploy and manage [Paperclip](https://github.com/paperclipai/paperclip) AI agent orchestration instances on Kubernetes with production-grade security, observability, and lifecycle management.**

Paperclip is an open-source AI agent orchestration platform. While you can deploy it manually, production Kubernetes deployments involve more than a Deployment and a Service -- you need database provisioning, secret management, persistent storage, health monitoring, network isolation, scaling, backup, and config rollouts, all wired correctly. This operator encodes those concerns into a single `Instance` custom resource so you can go from zero to production in minutes:

```yaml
apiVersion: paperclip.inc/v1alpha1
kind: Instance
metadata:
  name: my-paperclip
spec:
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

The operator reconciles this into a fully managed stack of Kubernetes resources: secured, monitored, and self-healing.

---

## Features

| | Feature | Details |
|---|---|---|
| **Declarative** | Single CRD | One resource defines the entire stack: StatefulSet, Service, ConfigMap, PVC, ServiceAccount, NetworkPolicy, Ingress, HPA, PDB, and more |
| **Database** | Managed PostgreSQL | Provisions PostgreSQL 17 with auto-generated credentials, data checksums, and graceful shutdown -- or connect to an external database, or use embedded PGlite |
| **Auth** | Full auth lifecycle | Better Auth with OAuth providers (Google, Apple), email verification via Resend, and automatic admin user bootstrap |
| **Secure** | Hardened by default | Non-root, all capabilities dropped, seccomp RuntimeDefault, default-deny NetworkPolicy, minimal RBAC |
| **Observable** | Built-in metrics | 7 Prometheus metrics, ServiceMonitor integration, configurable log levels |
| **Scalable** | Auto-scaling | HPA with CPU/memory targets, PodDisruptionBudgets, topology spread constraints |
| **Smart Probes** | Mode-aware health checks | Automatically uses TCP probes in authenticated mode (where `/api/health` returns 403) |
| **Storage** | S3 object storage | S3/MinIO/R2 for multi-replica file storage |
| **Backup** | S3-backed snapshots | Scheduled backups with configurable retention, point-in-time restore into new instances |
| **Secrets** | Encrypted secrets | Paperclip's built-in secrets management with master key support and strict mode |
| **Connections** | OAuth integrations | GitHub, GitLab, Slack, and more via the Paperclip connections system |
| **Cloud Sandbox** | Isolated execution | Agent runtimes in isolated Kubernetes pods with persistent workspaces, inference metering proxy, resource tiers, and multi-namespace isolation |
| **Extensible** | Sidecars & init containers | Add custom sidecar containers, init containers, extra volumes, and volume mounts |
| **Auto-Update** | Registry polling | Opt-in digest-based image update detection with automatic rollouts |
| **Plugins** | Declarative install | Install Paperclip plugins via `spec.plugins` |

## Architecture

```
+--------------------------------------------------------------+
|  Instance CR                                                  |
|  (your declarative config)                                    |
+--------------+-----------------------------------------------+
               | watch
               v
+--------------------------------------------------------------+
|  Paperclip Operator                                          |
|  +----------+  +-----------+  +---------------------------+  |
|  | Reconciler|  | Finalizer |  |   Prometheus Metrics      |  |
|  |           |  | (backup   |  |  (reconcile count,        |  |
|  | creates  -->  |  on delete)|  |   duration, phases)      |  |
|  +----------+  +-----------+  +---------------------------+  |
+--------------+-----------------------------------------------+
               | manages
               v
+--------------------------------------------------------------+
|  Managed Resources (per instance)                            |
|                                                              |
|  ServiceAccount    ConfigMap       NetworkPolicy             |
|  PVC               Ingress         PDB                       |
|  HPA               ServiceMonitor  CronJob (backup)          |
|                                                              |
|  StatefulSet                                                 |
|  +--------------------------------------------------------+  |
|  | Paperclip Container (Node.js, port 3100)               |  |
|  +--------------------------------------------------------+  |
|  + custom init containers + custom sidecars                  |
|                                                              |
|  Service (ClusterIP/LoadBalancer/NodePort)                   |
|                                                              |
|  [Managed PostgreSQL StatefulSet + Service + PVC] (optional) |
+--------------------------------------------------------------+
```

## Quick Start

### Prerequisites

- Kubernetes 1.28+
- Helm 3 (recommended) or kubectl

### 1. Install the operator

```bash
# Via Helm (recommended)
helm install paperclip-operator \
  oci://ghcr.io/paperclipinc/charts/paperclip-operator \
  --namespace paperclip-operator-system \
  --create-namespace
```

<details>
<summary>Alternative: install with kubectl</summary>

```bash
kubectl apply -f https://github.com/paperclipinc/paperclip-operator/releases/latest/download/install.yaml
```

</details>

<details>
<summary>Alternative: install with Kustomize</summary>

```bash
make install   # Install CRDs
make deploy IMG=ghcr.io/paperclipinc/paperclip-operator:latest
```

</details>

### 2. Create required Secrets

```bash
# Auth secret (required for authenticated mode)
kubectl create secret generic paperclip-auth \
  --from-literal=BETTER_AUTH_SECRET="$(openssl rand -hex 32)"

# LLM API keys (optional)
kubectl create secret generic paperclip-api-keys \
  --from-literal=ANTHROPIC_API_KEY="sk-ant-..." \
  --from-literal=OPENAI_API_KEY="sk-..."
```

### 3. Deploy a Paperclip instance

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

### 4. Verify

```bash
kubectl get instances
# or use the shorthand:
kubectl get pci
```

```
NAME           PHASE     ENDPOINT                                              AGE
my-paperclip   Running   http://my-paperclip.default.svc.cluster.local:3100    5m
```

```bash
kubectl get pods
# NAME              READY   STATUS    AGE
# my-paperclip-0    1/1     Running   5m
# my-paperclip-db-0 1/1     Running   5m   (managed PostgreSQL)
```

---

## Configuration

### Deployment Modes

Control authentication and network exposure:

```yaml
spec:
  deployment:
    mode: authenticated        # "local_trusted" or "authenticated"
    exposure: private          # "private" (ClusterIP) or "public" (Ingress/LB)
    publicURL: https://paperclip.example.com   # required when exposure is "public"
    allowedHostnames:
      - paperclip.example.com  # CORS allowed hostnames
```

| Mode | Description |
|------|-------------|
| `authenticated` (default) | Login required via Better Auth. Requires `BETTER_AUTH_SECRET`. To run authenticated without a public sign-up page, set `spec.auth.disableSignUp: true` (maps to `PAPERCLIP_AUTH_DISABLE_SIGN_UP`). |
| `local_trusted` | No authentication, intended for trusted local/loopback access. Requires `exposure: private`. |

| Exposure | Description |
|----------|-------------|
| `private` (default) | ClusterIP Service only. Access via port-forward or internal DNS. |
| `public` | Enables Ingress/LoadBalancer. Set `publicURL` for the external-facing URL. |

### Database

Three database modes for different deployment scenarios:

```yaml
spec:
  database:
    mode: managed   # "embedded", "external", or "managed"
```

| Mode | Use Case |
|------|----------|
| `managed` (default) | Operator provisions PostgreSQL 17 as a StatefulSet with PVC and auto-generated credentials. Suitable for development and small deployments. |
| `external` | Connect to an existing PostgreSQL instance. Recommended for production HA deployments (e.g., Amazon RDS, Cloud SQL, Azure Database for PostgreSQL). |
| `embedded` | Uses PGlite (in-process SQLite-compatible storage). Single-node only, good for local development and testing. |

#### Managed PostgreSQL

```yaml
spec:
  database:
    mode: managed
    managed:
      image: postgres:17-alpine   # default
      storageSize: 10Gi           # default
      storageClass: gp3           # optional
      resources:
        requests:
          cpu: 250m
          memory: 256Mi
        limits:
          cpu: "1"
          memory: 1Gi
```

The operator provisions a dedicated PostgreSQL StatefulSet, Service, and PVC. Credentials are auto-generated and stored in a managed Secret. Data checksums are enabled and `stop_mode` is set to `fast` for graceful shutdown.

#### External database

```yaml
spec:
  database:
    mode: external
    # Option 1: connection string (stored in etcd -- avoid if it contains credentials)
    externalURL: "postgresql://user:pass@host:5432/paperclip?sslmode=require"
    # Option 2: Secret reference (recommended for credentials)
    externalURLSecretRef:
      name: paperclip-database
      key: DATABASE_URL
```

> **Security:** Prefer `externalURLSecretRef` over `externalURL`. The CRD spec is stored in etcd -- plaintext connection strings containing passwords are visible to anyone with read access to the custom resource.

### Authentication

#### Better Auth secret

Required for `authenticated` mode:

```yaml
spec:
  auth:
    secretRef:
      name: paperclip-auth
      key: BETTER_AUTH_SECRET
```

#### Disabling public sign-up

To run in `authenticated` mode without a public registration page, disable sign-up:

```yaml
spec:
  deployment:
    mode: authenticated
  auth:
    disableSignUp: true   # maps to PAPERCLIP_AUTH_DISABLE_SIGN_UP, default false
```

This is the recommended replacement for the previous `single-tenant` mode. Combine it with `adminUser` bootstrap to provision the only account.

#### Automatic admin user bootstrap

Skip the manual setup screen by configuring an initial admin user. The operator creates a bootstrap Job that registers the admin account on first deployment:

```yaml
spec:
  auth:
    adminUser:
      email: admin@example.com
      name: Admin                     # default: "Admin"
      passwordSecretRef:
        name: paperclip-admin
        key: password
```

#### OAuth providers

Enable social sign-in via Google or Apple. Each provider's Secret must contain the corresponding client ID and client secret keys:

```yaml
spec:
  auth:
    google:
      credentialsSecretRef:
        name: google-oauth
        # Secret must contain GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET
    apple:
      credentialsSecretRef:
        name: apple-oauth
        # Secret must contain APPLE_CLIENT_ID and APPLE_CLIENT_SECRET
```

#### Email verification

Configure email delivery for verification and password reset via [Resend](https://resend.com):

```yaml
spec:
  auth:
    email:
      resendAPIKeySecretRef:
        name: resend-api-key
        key: RESEND_API_KEY
      from: "Paperclip <noreply@example.com>"
      verificationRequired: true
```

### Secrets Management

Paperclip includes a built-in encrypted secrets system. The operator injects the master encryption key:

```yaml
spec:
  secrets:
    masterKeySecretRef:
      name: paperclip-secrets
      key: MASTER_KEY
    strictMode: true    # require all sensitive values to use encrypted references
```

#### Secrets provider

The secrets vault backend is selectable via `spec.secrets.provider`. The default is `local_encrypted` (the built-in encrypted store above). To store secrets in AWS Secrets Manager instead, set the provider to `aws_secrets_manager` and configure `spec.secrets.aws`:

```yaml
spec:
  secrets:
    provider: aws_secrets_manager   # "local_encrypted" (default) or "aws_secrets_manager"
    aws:
      region: us-east-1             # required for AWS
      kmsKeyID: alias/paperclip     # required, KMS key for encryption
      deploymentID: prod            # required, isolates secrets per deployment
      prefix: paperclip             # optional, default "paperclip"
      environment: production       # optional
      endpoint: ""                  # optional, custom endpoint
      deleteRecoveryDays: 30        # optional, default 30
```

These map to `PAPERCLIP_SECRETS_PROVIDER` and the `PAPERCLIP_SECRETS_AWS_*` environment variables. AWS credentials are not injected by the operator; they are resolved through the AWS SDK credential chain, so use IRSA by adding the role annotation under `spec.security.rbac.serviceAccountAnnotations` (for example `eks.amazonaws.com/role-arn`).

### LLM API Keys

Inject LLM provider API keys from a Kubernetes Secret via `spec.adapters.apiKeysSecretRef`:

```yaml
spec:
  adapters:
    apiKeysSecretRef:
      name: paperclip-api-keys
      # Secret should contain: ANTHROPIC_API_KEY, OPENAI_API_KEY, etc.
```

The keys (for example `ANTHROPIC_API_KEY` and `OPENAI_API_KEY`) are passed straight through to the app. Paperclip discovers the available models for each provider automatically from the provider's API, so no model or provider needs to be configured on the operator.

### E2B Sandbox

Supply an [E2B](https://e2b.dev) API key so agents can use E2B cloud sandboxes:

```yaml
spec:
  adapters:
    e2b:
      apiKeySecretRef:
        name: paperclip-e2b
        key: E2B_API_KEY
```

This maps to the `E2B_API_KEY` environment variable. Other sandbox environments (Modal, Cloudflare, SSH) are not operator-configurable; they are set up at runtime in the Paperclip UI. See [Runtime-configured features](docs/deploy/runtime-configured-features.md) for details.

### Cloud Sandbox

Run agent runtimes in isolated Kubernetes pods with resource limits, persistent workspaces, and an optional inference metering proxy:

```yaml
spec:
  adapters:
    cloudSandbox:
      enabled: true
      defaultImage: ghcr.io/paperclipinc/agent-multi:latest
      namespace: paperclip-sandboxes   # defaults to instance namespace
      idleTimeoutMin: 30               # reap idle pods after 30 minutes
      multiNamespace: true             # per-company namespace isolation
      resources:
        requests:
          cpu: 500m
          memory: 512Mi
        limits:
          cpu: "2"
          memory: 2Gi
      persistence:
        enabled: true
        storageClass: gp3
        size: 10Gi
      resourceTiers:
        small:
          requests:
            cpu: 250m
            memory: 256Mi
        large:
          requests:
            cpu: "2"
            memory: 4Gi
      inferenceProxy:
        enabled: true
        image: ghcr.io/paperclipinc/inference-proxy:latest
        port: 8090
```

| Feature | Description |
|---------|-------------|
| **Persistent workspaces** | PVC-backed workspaces that survive pod restarts |
| **Multi-namespace** | Per-company namespace isolation for sandbox pods |
| **Resource tiers** | Named presets (small, medium, large) for sandbox resource limits |
| **Inference proxy** | Transparent metering proxy sidecar for API usage tracking |
| **Idle reaping** | Automatic cleanup of idle sandbox pods |

### Connections (OAuth Integrations)

Enable Paperclip's connections system for third-party OAuth integrations (GitHub, GitLab, Slack, etc.):

```yaml
spec:
  connections:
    credentialsSecretRef:
      name: paperclip-oauth-credentials
    credentialsKey: PAPERCLIP_OAUTH_CREDENTIALS   # default key name
    providersConfigRef:
      name: custom-providers   # optional: extend built-in provider catalog
```

The credentials Secret must contain a JSON object mapping provider IDs to OAuth client credentials:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: paperclip-oauth-credentials
type: Opaque
stringData:
  PAPERCLIP_OAUTH_CREDENTIALS: |
    {
      "github": {
        "clientId": "Iv1.xxxxxxxxxxxxxxxx",
        "clientSecret": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
      },
      "slack": {
        "clientId": "1234567890.1234567890",
        "clientSecret": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
      }
    }
```

Set the OAuth callback URL to `https://<your-domain>/api/connections/callback`.

### Plugins

Install Paperclip plugins declaratively:

```yaml
spec:
  plugins:
    - name: "@paperclip/analytics"
      version: "1.2.0"
    - name: "some-other-plugin"
```

### S3 Object Storage

Required for multi-replica deployments where all replicas need access to the same files. Supports AWS S3, MinIO, and Cloudflare R2:

```yaml
spec:
  objectStorage:
    provider: s3           # "s3", "minio", or "r2"
    bucket: my-paperclip-storage
    region: us-east-1      # optional for S3
    endpoint: ""           # required for MinIO/R2
    credentialsSecretRef:
      name: paperclip-s3
      # Secret must contain S3_ACCESS_KEY_ID and S3_SECRET_ACCESS_KEY
```

> **Horizontal scaling:** Paperclip does not use Redis. Scaling out relies on a shared PostgreSQL database, shared object storage (S3/MinIO/R2) for files, and pod-0 heartbeat gating so only one replica runs the scheduler. Configure `database.mode: external` and `objectStorage` when running multiple replicas. The in-process rate limiter is per-pod by design.

### Heartbeat Scheduler

Paperclip runs a heartbeat scheduler for periodic agent tasks. In multi-replica deployments, only pod-0 (ordinal 0) runs the scheduler to prevent duplicate execution:

```yaml
spec:
  heartbeat:
    enabled: true        # default: true
    intervalMS: 60000    # default: 60000 (1 minute)
```

### Persistent Storage

By default, the operator creates a 5Gi PVC mounted at `/paperclip`:

```yaml
spec:
  storage:
    persistence:
      enabled: true          # default: true
      size: 5Gi              # default
      storageClass: gp3      # optional
      accessModes:
        - ReadWriteOnce      # optional
```

### Networking

#### Service

```yaml
spec:
  networking:
    service:
      type: ClusterIP          # "ClusterIP", "LoadBalancer", or "NodePort"
      port: 3100               # default: 3100
      annotations:
        service.beta.kubernetes.io/aws-load-balancer-type: nlb
```

#### Ingress

Full Ingress support with TLS and WebSocket annotations:

```yaml
spec:
  networking:
    ingress:
      enabled: true
      ingressClassName: nginx
      hosts:
        - paperclip.example.com
      tls:
        - hosts:
            - paperclip.example.com
          secretName: paperclip-tls
      annotations:
        nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
        nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
        nginx.ingress.kubernetes.io/proxy-http-version: "1.1"
        nginx.ingress.kubernetes.io/proxy-set-headers: "Upgrade"
```

> **WebSocket support:** Paperclip uses WebSockets for real-time UI updates. Add appropriate timeout annotations for your ingress controller to prevent WebSocket disconnections.

### Scaling

#### Workload profiles

`spec.workload` selects how the server runs:

```yaml
spec:
  workload: auto   # "StatefulSet" (default), "Deployment", or "auto"
```

| Profile | Use for | Behavior |
|---------|---------|----------|
| `StatefulSet` (default) | Single replica, persistence, or embedded database | Stable pod identity with a per-instance PVC; rolling updates replace pods in place |
| `Deployment` | Stateless multi-replica (external/managed database + `objectStorage`, persistence off) | Surge rollouts (`maxSurge: 1`, `maxUnavailable: 0`) so capacity never drops, no AZ-pinned per-ordinal PVCs, HPA-friendly scale-in |
| `auto` | Let the operator decide | Deployment when persistence is disabled and the database is not embedded; StatefulSet otherwise |

> **PVC safety:** `workload: Deployment` requires `storage.persistence.enabled: false` -- the ReadWriteOnce data PVC cannot be shared by surging Deployment pods. If persistence is still enabled, the operator keeps the StatefulSet and reports the `WorkloadProfileValid: False` condition.

#### Manual replicas

```yaml
spec:
  availability:
    replicas: 3
```

When running multiple replicas, use `database.mode: external` (or `managed`) with a production-grade PostgreSQL service and configure `objectStorage` for shared file access -- the operator surfaces a `MultiReplicaPreconditions: False` condition (plus a Warning event) at `replicas > 1` until both are in place. The operator ensures only one pod runs the heartbeat scheduler.

The Instance CRD exposes the scale subresource (`status.replicas` / `status.selector` track the active workload), so standard tooling works:

```bash
kubectl scale instance/my-paperclip --replicas=3
```

External autoscalers like KEDA can target the instance directly:

```yaml
scaleTargetRef:
  apiVersion: paperclip.inc/v1alpha1
  kind: Instance
  name: my-paperclip
```

#### Horizontal Pod Autoscaler

```yaml
spec:
  availability:
    autoScaling:
      enabled: true
      minReplicas: 1              # default: 1
      maxReplicas: 3              # default: 3
      targetCPUUtilizationPercentage: 80          # default: 80
      targetMemoryUtilizationPercentage: 70       # optional
```

When auto-scaling is enabled, the HPA owns the replica count: the operator preserves the workload's current `replicas` on every reconcile, and `spec.availability.replicas` (including writes via `kubectl scale`) is ignored.

#### Pod Disruption Budget

```yaml
spec:
  availability:
    podDisruptionBudget:
      enabled: true
      minAvailable: 1
      # or: maxUnavailable: 1
```

Keep `minAvailable` strictly below `autoScaling.minReplicas`: when they are equal, the PDB allows zero disruptions at minimum scale and node drains stall (the operator emits a `PDBMayBlockDrains` warning event). If unhealthy pods blocking drains is a concern, manage your own PDB instead of the operator's and set the eviction policy:

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: my-paperclip-pdb
spec:
  minAvailable: 1
  unhealthyPodEvictionPolicy: AlwaysAllow   # evict crash-looping pods during drains
  selector:
    matchLabels:
      app.kubernetes.io/name: paperclip
      app.kubernetes.io/instance: my-paperclip
      app.kubernetes.io/component: server
```

#### Topology Spread Constraints

Spread pods across zones or nodes for improved availability:

```yaml
spec:
  availability:
    topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: topology.kubernetes.io/zone
        whenUnsatisfiable: DoNotSchedule
        labelSelector:
          matchLabels:
            app.kubernetes.io/instance: my-paperclip
```

#### Node Scheduling

```yaml
spec:
  availability:
    nodeSelector:
      kubernetes.io/arch: amd64
    tolerations:
      - key: dedicated
        operator: Equal
        value: paperclip
        effect: NoSchedule
    affinity:
      nodeAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          nodeSelectorTerms:
            - matchExpressions:
                - key: node-type
                  operator: In
                  values: [compute]
```

### Health Probes

The operator configures liveness, readiness, and startup probes automatically:

```yaml
spec:
  probes:
    type: auto   # "auto" (default), "http", or "tcp"
    liveness:
      initialDelaySeconds: 30
      periodSeconds: 10
      timeoutSeconds: 5
      failureThreshold: 3
    readiness:
      periodSeconds: 5
    startup:
      failureThreshold: 60
      periodSeconds: 5
```

| Probe Type | Behavior |
|------------|----------|
| `auto` (default) | HTTP probes (`GET /api/health`) in `local_trusted` mode, TCP probes (port 3100) in `authenticated` mode |
| `http` | Always use HTTP probes against `/api/health` |
| `tcp` | Always use TCP probes against port 3100 |

> **Why auto mode?** In authenticated mode, `/api/health` returns 403 without credentials, causing HTTP probes to fail. The operator automatically switches to TCP probes in these modes.

### Image Configuration

```yaml
spec:
  image:
    repository: ghcr.io/paperclipai/paperclip   # default
    tag: latest                                   # default
    digest: sha256:abc123...                      # optional, overrides tag
    pullPolicy: IfNotPresent                      # "Always", "Never", or "IfNotPresent"
    pullSecrets:
      - name: my-registry-secret
    autoUpdate:
      enabled: true
      interval: 5m    # polling interval (minimum: 1m)
```

When `autoUpdate` is enabled, the operator polls the container registry for new digests matching the configured tag and triggers a rolling update when a new digest is detected. Auto-update is a no-op for digest-pinned images.

### Backup and Restore

#### Scheduled backups

```yaml
spec:
  backup:
    schedule: "0 2 * * *"    # cron expression (daily at 2 AM UTC)
    retentionDays: 30        # default: 30
    s3:
      bucket: my-paperclip-backups
      path: backups/my-instance
      region: us-east-1
      endpoint: ""           # for MinIO/R2
      credentialsSecretRef:
        name: backup-s3-credentials
        # Secret must contain AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY
```

If `backup.s3` is not set, the operator falls back to the `objectStorage` configuration. The operator's `pg_dump` to S3 CronJob only runs when `spec.backup.schedule` is set.

#### App-native database backup

Paperclip can also run its own periodic database backups inside the app process. These write to a local directory under the `/paperclip` data PVC and are complementary to the operator's offsite `pg_dump` to S3 CronJob above:

```yaml
spec:
  backup:
    appNative:
      enabled: true          # default: true, maps to PAPERCLIP_DB_BACKUP_ENABLED
      intervalMinutes: 60    # default: 60, maps to PAPERCLIP_DB_BACKUP_INTERVAL_MINUTES
      retentionDays: 7       # default: 7, maps to PAPERCLIP_DB_BACKUP_RETENTION_DAYS
```

App-native backups are local-dir only (no offsite copy) and are only durable when `spec.storage.persistence.enabled` is true. Use the S3 CronJob for offsite snapshots.

#### Restore from backup

```yaml
spec:
  restoreFrom: "backups/my-instance/2026-01-15T10:30:00Z"
```

The operator runs a restore Job to populate the PVC before starting the StatefulSet, then clears `restoreFrom` automatically. This works on both existing and brand-new instances -- you can clone an instance by creating a new `Instance` CR with `restoreFrom` pointing to an existing backup.

### Custom Sidecars and Init Containers

```yaml
spec:
  sidecars:
    - name: cloud-sql-proxy
      image: gcr.io/cloud-sql-connectors/cloud-sql-proxy:2.14.3
      args: ["--structured-logs", "my-project:us-central1:my-db"]
      ports:
        - containerPort: 5432
  initContainers:
    - name: fetch-models
      image: curlimages/curl:8.5.0
      command: ["sh", "-c", "curl -o /data/model.bin https://..."]
      volumeMounts:
        - name: data
          mountPath: /data
```

### Extra Volumes and Volume Mounts

Mount additional ConfigMaps, Secrets, or PVCs into the Paperclip container:

```yaml
spec:
  extraVolumes:
    - name: shared-data
      persistentVolumeClaim:
        claimName: shared-pvc
  extraVolumeMounts:
    - name: shared-data
      mountPath: /shared
```

### Environment Variables

Inject additional environment variables directly or from ConfigMaps/Secrets:

```yaml
spec:
  env:
    - name: MY_CUSTOM_VAR
      value: "my-value"
    - name: SECRET_VAR
      valueFrom:
        secretKeyRef:
          name: my-secret
          key: secret-key
  envFrom:
    - configMapRef:
        name: my-configmap
    - secretRef:
        name: my-secret
```

### Compute Resources

```yaml
spec:
  resources:
    requests:
      cpu: 500m
      memory: 512Mi
    limits:
      cpu: "2"
      memory: 2Gi
```

### Pod Annotations

```yaml
spec:
  podAnnotations:
    cluster-autoscaler.kubernetes.io/safe-to-evict: "false"
    prometheus.io/scrape: "true"
```

---

## Security

The operator follows a **secure-by-default** philosophy. Every instance ships with hardened settings out of the box.

### Defaults

- **Non-root execution**: containers run as non-root by default
- **All capabilities dropped**: no ambient Linux capabilities
- **Seccomp RuntimeDefault**: syscall filtering enabled
- **Read-only root filesystem**: writable only at the PVC mount point (`/paperclip`) and `/tmp`
- **Default-deny NetworkPolicy**: only DNS (53) and HTTPS (443) egress allowed; ingress limited to the service port from the same namespace
- **Minimal RBAC**: each instance gets its own ServiceAccount; `automountServiceAccountToken` is disabled
- **No wildcard RBAC**: operator uses minimum required verbs with no wildcards

### Network Policies

```yaml
spec:
  security:
    networkPolicy:
      enabled: true          # default: true
      allowIngressCIDRs:     # additional CIDR blocks allowed to reach the service
        - 10.0.0.0/8
      allowEgressCIDRs:      # additional CIDR blocks the pod can reach
        - 172.16.0.0/12
```

When enabled, the operator creates a NetworkPolicy with a deny-all baseline and selective allow rules for DNS, HTTPS egress, and same-namespace ingress on the service port. The managed PostgreSQL pods get their own allow rules.

### Pod and Container Security Context

```yaml
spec:
  security:
    podSecurityContext:
      runAsNonRoot: true
      fsGroup: 1000
    containerSecurityContext:
      readOnlyRootFilesystem: true
      allowPrivilegeEscalation: false
      capabilities:
        drop: [ALL]
```

### RBAC and ServiceAccount

```yaml
spec:
  security:
    rbac:
      create: true   # default: true
      serviceAccountAnnotations:
        # AWS IRSA
        eks.amazonaws.com/role-arn: "arn:aws:iam::123456789012:role/paperclip"
        # GCP Workload Identity
        # iam.gke.io/gcp-service-account: "paperclip@project.iam.gserviceaccount.com"
```

---

## Observability

### Prometheus Metrics

The operator exposes 7 Prometheus metrics:

| Metric | Type | Description |
|--------|------|-------------|
| `paperclip_reconcile_total` | Counter | Total reconciliations by instance, namespace, and result (success/error) |
| `paperclip_reconcile_duration_seconds` | Histogram | Reconciliation latency in seconds |
| `paperclip_instance_phase` | Gauge | Current phase per instance (1 = active for given phase) |
| `paperclip_instance_info` | Gauge | Instance metadata (always 1, use for PromQL joins); labels: version, image |
| `paperclip_instance_ready` | Gauge | Whether the instance pod is ready (1/0) |
| `paperclip_managed_instances` | Gauge | Total number of managed instances across the cluster |
| `paperclip_resource_creation_failures_total` | Counter | Resource creation failures by resource type |

### ServiceMonitor

```yaml
spec:
  observability:
    metrics:
      enabled: true
      serviceMonitor:
        enabled: true
        interval: 30s       # default: 30s
```

### Logging

```yaml
spec:
  observability:
    logging:
      level: info   # "debug", "info", "warn", or "error"
```

---

## Status and Lifecycle

### Phases

| Phase | Description |
|-------|-------------|
| `Pending` | CR accepted, reconciliation not yet started |
| `Provisioning` | Creating managed resources (StatefulSet, Service, database, etc.) |
| `Running` | All resources healthy, pods ready |
| `Updating` | Rolling update in progress |
| `BackingUp` | Backup operation in progress |
| `Restoring` | Restore operation in progress |
| `Degraded` | Some resources unhealthy but recoverable |
| `Failed` | Unrecoverable error |
| `Terminating` | Finalizer running, cleaning up resources |

### Inspecting status

```bash
# Check phase and endpoint
kubectl get pci my-paperclip

# View conditions
kubectl get instance my-paperclip -o jsonpath='{.status.conditions}' | jq .

# View managed resources
kubectl get instance my-paperclip -o jsonpath='{.status.managedResources}' | jq .

# View auto-update status
kubectl get instance my-paperclip -o jsonpath='{.status.autoUpdate}' | jq .

# View backup status
kubectl get instance my-paperclip -o jsonpath='{.status.backup}' | jq .
```

### What the operator manages automatically

These behaviors are always applied -- no configuration needed:

| Behavior | Details |
|----------|---------|
| `PAPERCLIP_BIND=custom` + `PAPERCLIP_BIND_HOST=0.0.0.0` | Always set so Paperclip binds to all interfaces in the container (replaces the legacy `HOST` variable) |
| `SERVE_UI=true` | Always set so the web UI is served |
| Heartbeat leader election | Only pod-0 runs the heartbeat scheduler in multi-replica deployments |
| Config hash rollouts | Environment/config changes trigger rolling updates via SHA-256 hash annotation |
| Owner references | All managed resources have owner references for automatic garbage collection |
| Finalizer | Runs backup (if configured) and cleanup on CR deletion |
| Status tracking | Phase, conditions, endpoint, and managed resource names are continuously updated |

---

## Production Deployment Example

A full production deployment with external database, S3 storage, OAuth, Ingress with TLS, and monitoring:

```yaml
apiVersion: paperclip.inc/v1alpha1
kind: Instance
metadata:
  name: paperclip-prod
  namespace: paperclip
spec:
  image:
    tag: v1.2.3
    pullPolicy: IfNotPresent

  deployment:
    mode: authenticated
    exposure: public
    publicURL: https://paperclip.example.com
    allowedHostnames:
      - paperclip.example.com

  database:
    mode: external
    externalURLSecretRef:
      name: paperclip-database
      key: DATABASE_URL

  auth:
    secretRef:
      name: paperclip-auth
      key: BETTER_AUTH_SECRET
    adminUser:
      email: admin@example.com
      passwordSecretRef:
        name: paperclip-admin
        key: password
    google:
      credentialsSecretRef:
        name: google-oauth
    email:
      resendAPIKeySecretRef:
        name: resend-key
        key: RESEND_API_KEY
      from: "Paperclip <noreply@example.com>"
      verificationRequired: true

  secrets:
    masterKeySecretRef:
      name: paperclip-secrets
      key: MASTER_KEY
    strictMode: true

  storage:
    persistence:
      enabled: true
      size: 20Gi
      storageClass: gp3

  objectStorage:
    provider: s3
    bucket: paperclip-storage
    region: us-east-1
    credentialsSecretRef:
      name: paperclip-s3

  adapters:
    apiKeysSecretRef:
      name: paperclip-api-keys

  connections:
    credentialsSecretRef:
      name: paperclip-oauth-credentials

  security:
    networkPolicy:
      enabled: true
    rbac:
      create: true
      serviceAccountAnnotations:
        eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/paperclip

  networking:
    service:
      type: ClusterIP
      port: 3100
    ingress:
      enabled: true
      ingressClassName: nginx
      hosts:
        - paperclip.example.com
      tls:
        - hosts:
            - paperclip.example.com
          secretName: paperclip-tls
      annotations:
        nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
        nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"

  observability:
    metrics:
      enabled: true
      serviceMonitor:
        enabled: true
        interval: 30s
    logging:
      level: info

  availability:
    replicas: 3
    podDisruptionBudget:
      enabled: true
      minAvailable: 1
    topologySpreadConstraints:
      - maxSkew: 1
        topologyKey: topology.kubernetes.io/zone
        whenUnsatisfiable: DoNotSchedule

  probes:
    startup:
      failureThreshold: 60
      periodSeconds: 5

  backup:
    schedule: "0 2 * * *"
    retentionDays: 30
    s3:
      bucket: paperclip-backups
      path: backups/prod
      region: us-east-1
      credentialsSecretRef:
        name: backup-s3-credentials

  resources:
    requests:
      cpu: "1"
      memory: 1Gi
    limits:
      cpu: "4"
      memory: 4Gi
```

---

## Full CRD Specification

For the complete list of configurable fields, see the [Instance CRD types](api/v1alpha1/paperclipinstance_types.go) or run:

```bash
kubectl explain instance.spec
kubectl explain instance.spec.database
kubectl explain instance.spec.auth
```

See [config/samples/](config/samples/) for additional examples.

---

## Development

### Prerequisites

- Go 1.24+
- Docker
- kubectl
- A Kubernetes cluster (Kind, minikube, or remote)

### Build and run locally

```bash
git clone https://github.com/paperclipinc/paperclip-operator.git
cd paperclip-operator
go mod download

make install      # Install CRDs into current cluster
make run          # Run operator locally against current kubeconfig
```

### Run tests

```bash
make test                              # Unit + integration tests (envtest)
go test ./internal/resources/ -v       # Fast unit tests (no envtest needed)
make bench                             # Benchmarks for resource builders
make test-e2e                          # E2E tests (requires Kind cluster)
make scorecard                         # Operator SDK scorecard tests
```

### Lint and vet

```bash
make lint          # golangci-lint
go vet ./...       # Go vet
```

### After changing CRD types

```bash
make generate          # Regenerate deepcopy methods
make manifests         # Regenerate CRD YAML and RBAC
make sync-chart-crds   # Sync CRDs into Helm chart
```

### Build Docker image

```bash
make docker-build IMG=my-registry/paperclip-operator:dev
```

### Project structure

```
api/v1alpha1/          CRD types (Instance)
internal/controller/   Reconciliation logic (single controller + metrics)
internal/resources/    Pure resource builder functions (StatefulSet, Service, etc.)
config/crd/bases/      Generated CRD YAML (committed to git)
config/samples/        Example Instance CRs
charts/                Helm chart (CRDs as templates in templates/crds/)
bundle/                OLM bundle for OperatorHub submissions
hack/                  Build/sync scripts
.github/workflows/     CI/CD pipelines
```

The operator follows a clean separation of concerns: the controller orchestrates reconciliation, while all Kubernetes resource construction happens in pure functions inside `internal/resources/`. This makes builders easy to unit test without envtest.

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/my-feature`)
3. Commit using [conventional commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, etc.)
4. Push and open a pull request

All PRs require passing CI checks (lint, test, security scan, reconcile guard, Helm sync, E2E) and one approval.

## License

[Apache License 2.0](LICENSE)
