# API Reference

## Packages
- [paperclip.inc/v1alpha1](#paperclipincv1alpha1)


## paperclip.inc/v1alpha1

Package v1alpha1 contains API Schema definitions for the paperclip v1alpha1 API group.

### Resource Types
- [Instance](#instance)
- [PaperclipClusterDefaults](#paperclipclusterdefaults)
- [PaperclipSelfConfig](#paperclipselfconfig)



#### AWSSecretsManagerSpec



AWSSecretsManagerSpec configures the AWS Secrets Manager secrets provider.



_Appears in:_
- [SecretsSpec](#secretsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `region` _string_ | Region is the AWS region of the secrets and KMS key. |  |  |
| `kmsKeyID` _string_ | KMSKeyID is the KMS key (id or ARN) used to encrypt secrets. |  |  |
| `deploymentID` _string_ | DeploymentID namespaces secrets for this deployment. |  |  |
| `prefix` _string_ | Prefix is the secret name prefix. | paperclip | Optional: \{\} <br /> |
| `environment` _string_ | Environment is an optional environment label applied to stored secrets. |  | Optional: \{\} <br /> |
| `endpoint` _string_ | Endpoint overrides the AWS Secrets Manager endpoint (for VPC endpoints or testing). |  | Optional: \{\} <br /> |
| `deleteRecoveryDays` _integer_ | DeleteRecoveryDays is the AWS recovery window (in days) for deleted secrets. | 30 | Optional: \{\} <br /> |


#### AdapterRegistryEntry



AdapterRegistryEntry is one declarative agent-harness entry. Mirrors the
server's shared AdapterRegistryEntry shape (packages/shared) and the plugin's
local copy. Runtime fields are honored only for sandboxed (Kubernetes)
execution. Keep the json tags in sync with the server zod schema.



_Appears in:_
- [AdaptersSpec](#adaptersspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `adapterType` _string_ | AdapterType is the harness identifier, e.g. "opencode_local". |  | MinLength: 1 <br /> |
| `enabled` _boolean_ | Enabled controls availability in the agent-creation picker. Defaults true. |  | Optional: \{\} <br /> |
| `runtimeImage` _string_ | RuntimeImage is the container image the agent Job/Sandbox runs (k8s only). |  | Optional: \{\} <br /> |
| `envKeys` _string array_ | EnvKeys are process-env keys forwarded into the agent Job (k8s only),<br />e.g. ANTHROPIC_API_KEY. |  | Optional: \{\} <br /> |
| `allowFqdns` _string array_ | AllowFQDNs is the egress FQDN allow-list for the agent pod (k8s only). |  | Optional: \{\} <br /> |
| `probeCommand` _string array_ | ProbeCommand is the adapter liveness/probe command (k8s only). |  | Optional: \{\} <br /> |
| `defaultEnv` _object (keys:string, values:string)_ | DefaultEnv are NON-SECRET env defaults injected as the base layer for the<br />agent Job (process-env secrets override them), e.g. ANTHROPIC_BASE_URL set<br />to the in-cluster inference gateway. Never put secrets here. |  | Optional: \{\} <br /> |


#### AdaptersSpec



AdaptersSpec configures agent runtime adapters.



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiKeysSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core)_ | APIKeys references Secrets containing LLM provider API keys.<br />The Secret should contain keys like ANTHROPIC_API_KEY, OPENAI_API_KEY, etc. |  | Optional: \{\} <br /> |
| `cloudSandbox` _[CloudSandboxSpec](#cloudsandboxspec)_ | CloudSandbox configures cloud-based agent execution in isolated Kubernetes pods. |  | Optional: \{\} <br /> |
| `e2b` _[E2BSpec](#e2bspec)_ | E2B supplies the API key for the E2B sandbox provider. The provider must be<br />enabled as a plugin and selected per-Environment in the Paperclip UI; the<br />operator only wires E2B_API_KEY. (Modal and Cloudflare sandbox credentials<br />are configured at runtime in the UI - see docs/deploy/runtime-configured-features.md.) |  | Optional: \{\} <br /> |
| `execution` _[ExecutionSpec](#executionspec)_ | Execution configures the @paperclipai/plugin-kubernetes in-cluster sandbox<br />provider, which runs agents as per-tenant batch/v1 Jobs (or agent-sandbox<br />CRs) inside this cluster. This is the current k8s execution path and<br />supersedes the older in-tree CloudSandbox wiring (the two are independent;<br />do not enable both for the same execution surface).<br />When Mode is "kubernetes" the operator additionally provisions a ClusterRole<br />(namespaces, ServiceAccounts, Roles/RoleBindings, ResourceQuotas,<br />LimitRanges, NetworkPolicies, CiliumNetworkPolicies, Secrets, Jobs, Sandbox<br />CRs) and mounts the app ServiceAccount token so the app can reach the<br />in-cluster Kubernetes API. |  | Optional: \{\} <br /> |
| `registry` _[AdapterRegistryEntry](#adapterregistryentry) array_ | Registry declaratively defines which agent harnesses ("adapters") this<br />instance offers and how each is wired. Marshaled to the PAPERCLIP_ADAPTERS<br />env var consumed by the server's adapter-registry bootstrap. When empty,<br />the server uses its built-in defaults (no PAPERCLIP_ADAPTERS emitted). |  | Optional: \{\} <br /> |


#### AdminUserSpec



AdminUserSpec configures the initial admin user for automatic bootstrap.



_Appears in:_
- [AuthSpec](#authspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `email` _string_ | Email is the admin user's email address (used as login). |  |  |
| `name` _string_ | Name is the admin user's display name. | Admin | Optional: \{\} <br /> |
| `passwordSecretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretkeyselector-v1-core)_ | PasswordSecretRef references a Secret containing the admin password. |  |  |


#### AppNativeBackupSpec



AppNativeBackupSpec configures Paperclip's built-in database backups
(PAPERCLIP_DB_BACKUP_*).



_Appears in:_
- [BackupSpec](#backupspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled toggles app-native backups. Maps to PAPERCLIP_DB_BACKUP_ENABLED.<br />When unset, the app default (enabled) applies. |  | Optional: \{\} <br /> |
| `intervalMinutes` _integer_ | IntervalMinutes between backups. Maps to PAPERCLIP_DB_BACKUP_INTERVAL_MINUTES. |  | Optional: \{\} <br /> |
| `retentionDays` _integer_ | RetentionDays for local backups. Maps to PAPERCLIP_DB_BACKUP_RETENTION_DAYS. |  | Optional: \{\} <br /> |


#### AuthEmailSpec



AuthEmailSpec configures email delivery for auth flows.



_Appears in:_
- [AuthSpec](#authspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resendAPIKeySecretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretkeyselector-v1-core)_ | ResendAPIKeySecretRef references a Secret key containing the Resend API key. |  | Optional: \{\} <br /> |
| `from` _string_ | From is the sender address for outbound emails (e.g. "Paperclip <noreply@example.com>"). |  | Optional: \{\} <br /> |
| `verificationRequired` _boolean_ | VerificationRequired requires email verification before a session can be created. |  | Optional: \{\} <br /> |


#### AuthSpec



AuthSpec configures authentication.



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretkeyselector-v1-core)_ | SecretRef references a Secret containing the BETTER_AUTH_SECRET key.<br />Required when deployment mode is "authenticated". |  | Optional: \{\} <br /> |
| `adminUser` _[AdminUserSpec](#adminuserspec)_ | AdminUser configures the initial admin user that is created automatically<br />when the instance is first deployed. If not set, the app's first-admin<br />(board-claim) flow lets the first human to authenticate claim ownership<br />in authenticated mode. |  | Optional: \{\} <br /> |
| `disableSignUp` _boolean_ | DisableSignUp disables public self-service sign-up (the former<br />"single-tenant" behavior). Maps to PAPERCLIP_AUTH_DISABLE_SIGN_UP. |  | Optional: \{\} <br /> |
| `email` _[AuthEmailSpec](#authemailspec)_ | Email configures email sending for verification and password reset. |  | Optional: \{\} <br /> |
| `google` _[OAuthProviderSpec](#oauthproviderspec)_ | Google configures Google OAuth sign-in.<br />The referenced Secret must contain GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET keys. |  | Optional: \{\} <br /> |
| `apple` _[OAuthProviderSpec](#oauthproviderspec)_ | Apple configures Apple OAuth sign-in.<br />The referenced Secret must contain APPLE_CLIENT_ID and APPLE_CLIENT_SECRET keys. |  | Optional: \{\} <br /> |


#### AutoScalingSpec



AutoScalingSpec configures a HorizontalPodAutoscaler.



_Appears in:_
- [AvailabilitySpec](#availabilityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether an HPA is created. |  |  |
| `minReplicas` _integer_ | MinReplicas is the minimum number of replicas. | 1 | Optional: \{\} <br /> |
| `maxReplicas` _integer_ | MaxReplicas is the maximum number of replicas. | 3 | Optional: \{\} <br /> |
| `targetCPUUtilizationPercentage` _integer_ | TargetCPUUtilizationPercentage is the target CPU utilization for scaling. | 80 | Optional: \{\} <br /> |
| `targetMemoryUtilizationPercentage` _integer_ | TargetMemoryUtilizationPercentage is the target memory utilization for scaling. |  | Optional: \{\} <br /> |


#### AutoUpdateSpec



AutoUpdateSpec configures automatic image update polling.



_Appears in:_
- [ImageSpec](#imagespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether auto-update polling is active. | false | Optional: \{\} <br /> |
| `interval` _string_ | Interval is the polling interval (e.g. "5m", "1h"). Minimum is 1m. | 5m | Pattern: `^\d+(s\|m\|h)$` <br />Optional: \{\} <br /> |


#### AvailabilitySpec



AvailabilitySpec configures scaling and pod scheduling.



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ | Replicas is the desired number of Paperclip server pods.<br />Ignored when autoScaling is enabled (the HPA manages replicas). | 1 | Minimum: 1 <br />Optional: \{\} <br /> |
| `podDisruptionBudget` _[PDBSpec](#pdbspec)_ | PodDisruptionBudget configures the PDB. |  | Optional: \{\} <br /> |
| `autoScaling` _[AutoScalingSpec](#autoscalingspec)_ | AutoScaling configures the HorizontalPodAutoscaler. |  | Optional: \{\} <br /> |
| `nodeSelector` _object (keys:string, values:string)_ | NodeSelector specifies node selection constraints. |  | Optional: \{\} <br /> |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#toleration-v1-core) array_ | Tolerations specifies pod tolerations. |  | Optional: \{\} <br /> |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#affinity-v1-core)_ | Affinity specifies pod affinity rules. |  | Optional: \{\} <br /> |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#topologyspreadconstraint-v1-core) array_ | TopologySpreadConstraints specifies topology spread constraints. |  | Optional: \{\} <br /> |


#### BackupS3Spec



BackupS3Spec configures S3 backup destination.



_Appears in:_
- [BackupSpec](#backupspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bucket` _string_ | Bucket is the S3 bucket name. |  |  |
| `path` _string_ | Path is the S3 key prefix for backups. |  | Optional: \{\} <br /> |
| `region` _string_ | Region is the S3 region. |  | Optional: \{\} <br /> |
| `endpoint` _string_ | Endpoint is the S3-compatible endpoint URL. |  | Optional: \{\} <br /> |
| `credentialsSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core)_ | CredentialsSecretRef references a Secret containing AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY. |  | Optional: \{\} <br /> |


#### BackupSpec



BackupSpec configures database backups. Two complementary mechanisms:
the operator's pg_dump -> S3 CronJob (Schedule/S3, for managed/external
PostgreSQL) and Paperclip's built-in local-dir backups (AppNative).



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `schedule` _string_ | Schedule is a cron expression for the operator pg_dump -> S3 backup CronJob.<br />Omit to use only app-native backups. |  | Optional: \{\} <br /> |
| `s3` _[BackupS3Spec](#backups3spec)_ | S3 configures the S3 backup destination. Uses ObjectStorage config if not specified. |  | Optional: \{\} <br /> |
| `retentionDays` _integer_ | RetentionDays specifies how many days to retain operator (S3) backups. | 30 | Optional: \{\} <br /> |
| `appNative` _[AppNativeBackupSpec](#appnativebackupspec)_ | AppNative configures Paperclip's built-in database backups, written to a<br />local directory under the data PVC. Complementary to the operator's S3<br />CronJob; durable only when spec.storage.persistence is enabled. |  | Optional: \{\} <br /> |


#### CloudSandboxPersistenceSpec



CloudSandboxPersistenceSpec configures PVC-backed persistent workspaces.



_Appears in:_
- [CloudSandboxSpec](#cloudsandboxspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled enables PVC-backed workspaces instead of emptyDir. |  |  |
| `storageClass` _string_ | StorageClass is the storage class for workspace PVCs. |  | Optional: \{\} <br /> |
| `size` _string_ | Size is the storage size for workspace PVCs (e.g. "10Gi"). | 10Gi | Optional: \{\} <br /> |


#### CloudSandboxSpec



CloudSandboxSpec configures cloud sandbox execution for agent runtimes.



_Appears in:_
- [AdaptersSpec](#adaptersspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether cloud sandbox execution is available. | false | Optional: \{\} <br /> |
| `defaultImage` _string_ | DefaultImage is the default agent runtime container image. | ghcr.io/paperclipinc/agent-multi:latest | Optional: \{\} <br /> |
| `namespace` _string_ | Namespace is the namespace for sandbox pods. Defaults to the instance namespace. |  | Optional: \{\} <br /> |
| `idleTimeoutMin` _integer_ | IdleTimeoutMin is how long (in minutes) a sandbox pod can be idle before being reaped. | 30 | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#resourcerequirements-v1-core)_ | Resources specifies default compute resources for sandbox pods. |  | Optional: \{\} <br /> |
| `persistence` _[CloudSandboxPersistenceSpec](#cloudsandboxpersistencespec)_ | Persistence configures PVC-backed persistent workspaces for sandbox pods. |  | Optional: \{\} <br /> |
| `multiNamespace` _boolean_ | MultiNamespace enables per-company namespace isolation for sandbox pods.<br />When enabled, each company's sandbox pods run in a dedicated namespace. |  | Optional: \{\} <br /> |
| `inferenceProxy` _[InferenceProxySpec](#inferenceproxyspec)_ | InferenceProxy configures the transparent inference metering proxy. |  | Optional: \{\} <br /> |
| `resourceTiers` _object (keys:string, values:[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#resourcerequirements-v1-core))_ | ResourceTiers defines named resource presets for sandbox pods. |  | Optional: \{\} <br /> |


#### ConnectionsSpec



ConnectionsSpec configures third-party OAuth provider credentials.
The operator injects credentials as PAPERCLIP_OAUTH_CREDENTIALS from
the referenced Secret, enabling the Paperclip connections system to
manage OAuth flows and token lifecycle for external services.



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `credentialsSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core)_ | CredentialsSecretRef references a Secret containing OAuth client credentials.<br />The Secret must contain a key (default "PAPERCLIP_OAUTH_CREDENTIALS") whose<br />value is a JSON object mapping provider IDs to \{clientId, clientSecret\} pairs.<br />Example: \{"github":\{"clientId":"...","clientSecret":"..."\},"slack":\{"clientId":"...","clientSecret":"..."\}\} |  |  |
| `credentialsKey` _string_ | CredentialsKey is the key within the Secret that holds the JSON credentials.<br />Defaults to "PAPERCLIP_OAUTH_CREDENTIALS". | PAPERCLIP_OAUTH_CREDENTIALS | Optional: \{\} <br /> |
| `providersConfigRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core)_ | ProvidersConfigRef optionally references a ConfigMap containing a<br />PAPERCLIP_OAUTH_PROVIDERS key with a JSON provider catalog to extend<br />or override the built-in provider definitions at runtime. |  | Optional: \{\} <br /> |


#### DatabaseSpec



DatabaseSpec configures PostgreSQL.
For high-availability production deployments, use mode "external" with a managed
PostgreSQL service (e.g., Amazon RDS, Cloud SQL). The "managed" mode provides a
single-instance PostgreSQL suitable for development and small deployments.



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `mode` _string_ | Mode selects the database mode: "embedded" (PGlite), "external" (connection string), or "managed" (operator-managed StatefulSet). | managed | Enum: [embedded external managed] <br />Optional: \{\} <br /> |
| `externalURL` _string_ | ExternalURL is the PostgreSQL connection string for external mode.<br />WARNING: This value is stored in plaintext in the CRD spec (etcd). If the URL contains<br />credentials, use ExternalURLSecretRef instead to reference a Secret. |  | Optional: \{\} <br /> |
| `externalURLSecretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretkeyselector-v1-core)_ | ExternalURLSecretRef references a Secret containing the DATABASE_URL key. |  | Optional: \{\} <br /> |
| `managed` _[ManagedDatabaseSpec](#manageddatabasespec)_ | Managed configures the operator-managed PostgreSQL StatefulSet. |  | Optional: \{\} <br /> |


#### DeploymentSpec



DeploymentSpec controls deployment mode and exposure.



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `mode` _string_ | Mode sets the deployment mode: "local_trusted" (loopback / no auth) or<br />"authenticated" (login required). Matches Paperclip's DEPLOYMENT_MODES.<br />To run authenticated without public self-service sign-up, set<br />spec.auth.disableSignUp instead of a separate mode. | authenticated | Enum: [local_trusted authenticated] <br />Optional: \{\} <br /> |
| `exposure` _string_ | Exposure controls network exposure: "private" (ClusterIP only) or "public" (Ingress/LoadBalancer). | private | Enum: [private public] <br />Optional: \{\} <br /> |
| `publicURL` _string_ | PublicURL is the externally-reachable URL for the Paperclip instance.<br />Required when exposure is "public". |  | Optional: \{\} <br /> |
| `allowedHostnames` _string array_ | AllowedHostnames is a list of allowed hostnames for CORS. |  | Optional: \{\} <br /> |
| `platformAdmin` _[PlatformAdminSpec](#platformadminspec)_ | PlatformAdmin, when set, bootstraps the instance with a platform-managed<br />instance-admin so it is never left in the single-tenant "claim this<br />instance" state. The operator runs an idempotent seed init container<br />(`pnpm paperclipai auth seed-instance-admin`) after onboarding/migrations. |  | Optional: \{\} <br /> |


#### E2BSpec



E2BSpec configures the E2B sandbox provider API key.



_Appears in:_
- [AdaptersSpec](#adaptersspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiKeySecretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretkeyselector-v1-core)_ | APIKeySecretRef references a Secret key holding the E2B API key. |  |  |


#### ExecutionSpec



ExecutionSpec configures the agent execution policy enforced by the Paperclip
server at boot. It maps to PAPERCLIP_EXECUTION_MODE and the PAPERCLIP_K8S_*
env vars consumed by the fork's execution-policy bootstrap.



_Appears in:_
- [AdaptersSpec](#adaptersspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `mode` _string_ | Mode selects the forced execution policy. "kubernetes" forces all agent<br />runs onto the in-cluster Kubernetes sandbox provider and refuses local<br />execution; "any" leaves execution unrestricted (the operator emits no<br />PAPERCLIP_EXECUTION_MODE and provisions no execution RBAC).<br />Maps to PAPERCLIP_EXECUTION_MODE. | any | Enum: [kubernetes any] <br />Optional: \{\} <br /> |
| `kubernetes` _[K8sExecutionSpec](#k8sexecutionspec)_ | Kubernetes configures the in-cluster Kubernetes sandbox backend. Honored<br />only when Mode is "kubernetes". |  | Optional: \{\} <br /> |


#### GrafanaDashboardSpec



GrafanaDashboardSpec configures auto-provisioned Grafana dashboard ConfigMaps.



_Appears in:_
- [MetricsSpec](#metricsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled enables Grafana dashboard ConfigMap creation. | false | Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels to add to the dashboard ConfigMaps (in addition to grafana_dashboard: "1"). |  | Optional: \{\} <br /> |
| `folder` _string_ | Folder is the Grafana folder to place the dashboards in. | Paperclip | Optional: \{\} <br /> |


#### HTTPRouteParentRef



HTTPRouteParentRef identifies a Gateway (or other parent) that the HTTPRoute attaches to.



_Appears in:_
- [HTTPRouteSpec](#httproutespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the Gateway. |  |  |
| `namespace` _string_ | Namespace is the namespace of the Gateway.<br />Defaults to the same namespace as the HTTPRoute. |  | Optional: \{\} <br /> |
| `sectionName` _string_ | SectionName is the name of a specific listener on the Gateway to attach to. |  | Optional: \{\} <br /> |


#### HTTPRouteSpec



HTTPRouteSpec configures a Gateway API HTTPRoute.



_Appears in:_
- [NetworkingSpec](#networkingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether an HTTPRoute is created. | false | Optional: \{\} <br /> |
| `parentRefs` _[HTTPRouteParentRef](#httprouteparentref) array_ | ParentRefs specifies the Gateways this HTTPRoute attaches to.<br />Each ref identifies a Gateway by name (and optionally namespace and sectionName). |  | Optional: \{\} <br /> |
| `hostnames` _string array_ | Hostnames specifies the hostnames matched by this HTTPRoute. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations specifies additional annotations for the HTTPRoute. |  | Optional: \{\} <br /> |


#### HeartbeatSpec



HeartbeatSpec configures the agent heartbeat scheduler.



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether the heartbeat scheduler runs. Defaults to true. | true | Optional: \{\} <br /> |
| `intervalMS` _integer_ | IntervalMS sets the heartbeat interval in milliseconds. | 60000 | Optional: \{\} <br /> |


#### ImageSpec



ImageSpec configures the container image.



_Appears in:_
- [InstanceSpec](#instancespec)
- [PaperclipClusterDefaultsSpec](#paperclipclusterdefaultsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repository` _string_ | Repository is the container image repository. | ghcr.io/paperclipai/paperclip | Optional: \{\} <br /> |
| `tag` _string_ | Tag is the container image tag. Either tag or digest must be set; there is<br />no default, because pinning to a mutable tag like :latest can silently pull<br />a broken upstream build. |  | Optional: \{\} <br /> |
| `digest` _string_ | Digest overrides the tag with an image digest (e.g. sha256:abc...). |  | Optional: \{\} <br /> |
| `pullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#pullpolicy-v1-core)_ | PullPolicy specifies the image pull policy. | IfNotPresent | Enum: [Always Never IfNotPresent] <br />Optional: \{\} <br /> |
| `pullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core) array_ | PullSecrets specifies image pull secrets. |  | Optional: \{\} <br /> |
| `autoUpdate` _[AutoUpdateSpec](#autoupdatespec)_ | AutoUpdate enables automatic image updates by polling the registry for new digests. |  | Optional: \{\} <br /> |


#### InferenceProxySpec



InferenceProxySpec configures the transparent inference metering proxy.



_Appears in:_
- [CloudSandboxSpec](#cloudsandboxspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled enables the inference proxy sidecar for metered API access. |  |  |
| `image` _string_ | Image is the inference proxy container image. |  | Optional: \{\} <br /> |
| `port` _integer_ | Port is the port the proxy listens on. | 8090 | Optional: \{\} <br /> |


#### IngressSpec



IngressSpec configures the Kubernetes Ingress.



_Appears in:_
- [NetworkingSpec](#networkingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether an Ingress is created. | false | Optional: \{\} <br /> |
| `ingressClassName` _string_ | IngressClassName specifies the Ingress class name. |  | Optional: \{\} <br /> |
| `hosts` _string array_ | Hosts specifies the Ingress hostnames. |  | Optional: \{\} <br /> |
| `tls` _[IngressTLSSpec](#ingresstlsspec) array_ | TLS configures TLS for the Ingress. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations specifies additional annotations for the Ingress.<br />Tip: Add WebSocket support annotations for your ingress controller here<br />(e.g., nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"). |  | Optional: \{\} <br /> |


#### IngressTLSSpec



IngressTLSSpec configures TLS for an Ingress host.



_Appears in:_
- [IngressSpec](#ingressspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `hosts` _string array_ | Hosts specifies the TLS hostnames. |  |  |
| `secretName` _string_ | SecretName is the name of the TLS secret. |  |  |


#### Instance



Instance is the Schema for the instances API.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `paperclip.inc/v1alpha1` | | |
| `kind` _string_ | `Instance` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[InstanceSpec](#instancespec)_ |  |  |  |




#### InstanceSpec



InstanceSpec defines the desired state of a Paperclip instance.



_Appears in:_
- [Instance](#instance)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _[ImageSpec](#imagespec)_ | Image specifies the Paperclip container image to deploy. |  | Optional: \{\} <br /> |
| `deployment` _[DeploymentSpec](#deploymentspec)_ | Deployment controls the deployment mode and exposure settings. |  | Optional: \{\} <br /> |
| `database` _[DatabaseSpec](#databasespec)_ | Database configures the PostgreSQL connection. |  | Optional: \{\} <br /> |
| `auth` _[AuthSpec](#authspec)_ | Auth configures authentication settings. |  | Optional: \{\} <br /> |
| `secrets` _[SecretsSpec](#secretsspec)_ | Secrets configures the Paperclip secrets management system. |  | Optional: \{\} <br /> |
| `storage` _[StorageSpec](#storagespec)_ | Storage configures persistent storage for the Paperclip data directory. |  | Optional: \{\} <br /> |
| `objectStorage` _[ObjectStorageSpec](#objectstoragespec)_ | ObjectStorage configures S3-compatible object storage for multi-replica deployments. |  | Optional: \{\} <br /> |
| `heartbeat` _[HeartbeatSpec](#heartbeatspec)_ | Heartbeat configures the agent heartbeat scheduler. |  | Optional: \{\} <br /> |
| `adapters` _[AdaptersSpec](#adaptersspec)_ | Adapters configures agent runtime adapters. |  | Optional: \{\} <br /> |
| `connections` _[ConnectionsSpec](#connectionsspec)_ | Connections configures third-party OAuth provider credentials for<br />the Paperclip connections system (GitHub, GitLab, Slack, etc.). |  | Optional: \{\} <br /> |
| `plugins` _[PluginRef](#pluginref) array_ | Plugins lists plugins to install. |  | Optional: \{\} <br /> |
| `selfConfigure` _[SelfConfigureSpec](#selfconfigurespec)_ | SelfConfigure enables agents to modify their own Instance via<br />PaperclipSelfConfig resources, gated by an action allowlist. |  | Optional: \{\} <br /> |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#envvar-v1-core) array_ | Env specifies additional environment variables for the Paperclip container. |  | Optional: \{\} <br /> |
| `envFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#envfromsource-v1-core) array_ | EnvFrom specifies additional environment variable sources for the Paperclip container. |  | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#resourcerequirements-v1-core)_ | Resources specifies the compute resources for the Paperclip container. |  | Optional: \{\} <br /> |
| `security` _[SecuritySpec](#securityspec)_ | Security configures pod and container security settings. |  | Optional: \{\} <br /> |
| `shareProcessNamespace` _boolean_ | ShareProcessNamespace enables PID namespace sharing between all containers<br />in the pod. When true, the infrastructure (pause) container becomes PID 1<br />and reaps zombie processes, which prevents accumulation of defunct helper<br />processes (git, plugins, shells) under a Node.js server that does not call<br />waitpid(). Defaults to true.<br />Security note: enabling this lets every container in the pod see and signal<br />every other container's processes. A compromised sidecar could send signals<br />to the server and vice versa. Set to false to keep per-container PID<br />isolation; you are then responsible for reaping zombies (e.g. by baking<br />tini or dumb-init into the image). | true | Optional: \{\} <br /> |
| `suspended` _boolean_ | Suspended scales the workload to zero replicas when true. Non-runtime<br />resources (Service, ConfigMap, RBAC, NetworkPolicy, PVC) remain fully<br />managed. Set to false to resume normal operation. | false | Optional: \{\} <br /> |
| `networking` _[NetworkingSpec](#networkingspec)_ | Networking configures service, ingress, and WebSocket settings. |  | Optional: \{\} <br /> |
| `observability` _[ObservabilitySpec](#observabilityspec)_ | Observability configures metrics, logging, and monitoring. |  | Optional: \{\} <br /> |
| `availability` _[AvailabilitySpec](#availabilityspec)_ | Availability configures scaling, PDB, and pod scheduling. |  | Optional: \{\} <br /> |
| `probes` _[ProbesSpec](#probesspec)_ | Probes configures liveness, readiness, and startup probes. |  | Optional: \{\} <br /> |
| `backup` _[BackupSpec](#backupspec)_ | Backup configures periodic backup to S3-compatible storage. |  | Optional: \{\} <br /> |
| `restoreFrom` _string_ | RestoreFrom specifies a remote backup path to restore from on first boot. |  | Optional: \{\} <br /> |
| `tailscale` _[TailscaleSpec](#tailscalespec)_ | Tailscale configures an ephemeral Tailscale sidecar that Serves the<br />Paperclip app over the tailnet. |  | Optional: \{\} <br /> |
| `sidecars` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#container-v1-core) array_ | Sidecars specifies additional sidecar containers. |  | Optional: \{\} <br /> |
| `initContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#container-v1-core) array_ | InitContainers specifies additional init containers. |  | Optional: \{\} <br /> |
| `extraVolumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#volume-v1-core) array_ | ExtraVolumes specifies additional volumes to add to the pod. |  | Optional: \{\} <br /> |
| `extraVolumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#volumemount-v1-core) array_ | ExtraVolumeMounts specifies additional volume mounts for the Paperclip container. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations specifies additional annotations for the pod template. |  | Optional: \{\} <br /> |


#### K8sExecutionSpec



K8sExecutionSpec configures the @paperclipai/plugin-kubernetes backend. Each
field maps to a PAPERCLIP_K8S_* env var read by the server at boot.



_Appears in:_
- [ExecutionSpec](#executionspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `backend` _string_ | Backend selects the per-run workload primitive. "job" runs each agent as a<br />fire-and-forget batch/v1 Job (log-scraped output); "sandbox-cr" creates a<br />long-lived agent-sandbox CR (agents.x-k8s.io) that the server execs into.<br />Maps to PAPERCLIP_K8S_BACKEND. | job | Enum: [job sandbox-cr] <br />Optional: \{\} <br /> |
| `runtimeClassName` _string_ | RuntimeClassName is the RuntimeClass applied to agent pods (e.g. "gvisor")<br />for an extra kernel-isolation boundary. Maps to<br />PAPERCLIP_K8S_RUNTIME_CLASS_NAME. When set, the execution ClusterRole also<br />grants get/use on the named RuntimeClass. |  | Optional: \{\} <br /> |
| `egressMode` _string_ | EgressMode selects how per-tenant egress is enforced. "standard" uses plain<br />NetworkPolicy (CIDR-only, cannot match FQDNs); "cilium" uses a<br />CiliumNetworkPolicy for exact FQDN allow-listing (requires the Cilium CNI).<br />Maps to PAPERCLIP_K8S_EGRESS_MODE. |  | Enum: [standard cilium] <br />Optional: \{\} <br /> |
| `egressAllowFQDNs` _string array_ | EgressAllowFQDNs is the list of fully-qualified domain names tenant agent<br />pods may reach (e.g. the LLM gateway and required APIs). Enforced exactly<br />only under EgressMode "cilium". Maps to PAPERCLIP_K8S_EGRESS_ALLOW_FQDNS<br />(comma-separated). |  | Optional: \{\} <br /> |
| `egressAllowCIDRs` _string array_ | EgressAllowCIDRs is the list of CIDR blocks tenant agent pods may reach, in<br />addition to (or as the standard-mode substitute for) the FQDN allow-list.<br />Maps to PAPERCLIP_K8S_EGRESS_ALLOW_CIDRS (comma-separated). |  | Optional: \{\} <br /> |
| `namespacePrefix` _string_ | NamespacePrefix is prepended to each derived per-tenant namespace name,<br />letting multiple instances share a cluster without namespace collisions.<br />Maps to PAPERCLIP_K8S_NAMESPACE_PREFIX. |  | Optional: \{\} <br /> |


#### LoggingSpec



LoggingSpec configures logging.



_Appears in:_
- [ObservabilitySpec](#observabilityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `level` _string_ | Level sets the log level: "debug", "info", "warn", "error". | info | Enum: [debug info warn error] <br />Optional: \{\} <br /> |


#### ManagedDatabaseSpec



ManagedDatabaseSpec configures the operator-managed PostgreSQL instance.



_Appears in:_
- [DatabaseSpec](#databasespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image is the PostgreSQL container image. | postgres:17-alpine | Optional: \{\} <br /> |
| `storageSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api)_ | StorageSize is the PVC size for PostgreSQL data. | 10Gi | Optional: \{\} <br /> |
| `storageClass` _string_ | StorageClass is the storage class for the PostgreSQL PVC. |  | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#resourcerequirements-v1-core)_ | Resources specifies compute resources for the PostgreSQL container. |  | Optional: \{\} <br /> |




#### MetricsSpec



MetricsSpec configures Prometheus metrics.



_Appears in:_
- [ObservabilitySpec](#observabilityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether metrics are exposed. | false | Optional: \{\} <br /> |
| `serviceMonitor` _[ServiceMonitorSpec](#servicemonitorspec)_ | ServiceMonitor enables creating a Prometheus ServiceMonitor. |  | Optional: \{\} <br /> |
| `prometheusRule` _[PrometheusRuleSpec](#prometheusrulespec)_ | PrometheusRule configures an auto-provisioned PrometheusRule with default<br />operator alerts (reconcile errors, instance not-ready, restart rate). |  | Optional: \{\} <br /> |
| `grafanaDashboard` _[GrafanaDashboardSpec](#grafanadashboardspec)_ | GrafanaDashboard configures auto-provisioned Grafana dashboard ConfigMaps. |  | Optional: \{\} <br /> |


#### NetworkPolicySpec



NetworkPolicySpec configures network isolation.



_Appears in:_
- [SecuritySpec](#securityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether a NetworkPolicy is created. Defaults to true. | true | Optional: \{\} <br /> |
| `allowIngressCIDRs` _string array_ | AllowIngressCIDRs specifies additional CIDR blocks allowed to reach the Paperclip service. |  | items:Pattern: `^([0-9]\{1,3\}\.)\{3\}[0-9]\{1,3\}/[0-9]\{1,2\}$` <br />Optional: \{\} <br /> |
| `allowEgressCIDRs` _string array_ | AllowEgressCIDRs specifies additional CIDR blocks the pod can reach. |  | items:Pattern: `^([0-9]\{1,3\}\.)\{3\}[0-9]\{1,3\}/[0-9]\{1,2\}$` <br />Optional: \{\} <br /> |


#### NetworkingSpec



NetworkingSpec configures service and ingress.



_Appears in:_
- [InstanceSpec](#instancespec)
- [PaperclipClusterDefaultsSpec](#paperclipclusterdefaultsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _[ServiceSpec](#servicespec)_ | Service configures the Kubernetes Service. |  | Optional: \{\} <br /> |
| `ingress` _[IngressSpec](#ingressspec)_ | Ingress configures the Kubernetes Ingress. |  | Optional: \{\} <br /> |
| `httpRoute` _[HTTPRouteSpec](#httproutespec)_ | HTTPRoute configures a Gateway API HTTPRoute.<br />This is an alternative to Ingress for clusters using the Gateway API. |  | Optional: \{\} <br /> |


#### OAuthProviderSpec



OAuthProviderSpec references credentials for a social OAuth provider.



_Appears in:_
- [AuthSpec](#authspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `credentialsSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core)_ | CredentialsSecretRef references a Secret containing provider-specific OAuth<br />client credentials (e.g. GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET). |  |  |


#### ObjectStorageSpec



ObjectStorageSpec configures S3-compatible object storage.



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `provider` _string_ | Provider is the S3-compatible provider: "s3", "minio", "r2". |  | Enum: [s3 minio r2] <br /> |
| `bucket` _string_ | Bucket is the S3 bucket name. |  |  |
| `region` _string_ | Region is the S3 region. |  | Optional: \{\} <br /> |
| `endpoint` _string_ | Endpoint is the S3-compatible endpoint URL (for MinIO/R2). |  | Optional: \{\} <br /> |
| `credentialsSecretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core)_ | CredentialsSecretRef references a Secret containing AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY. |  | Optional: \{\} <br /> |


#### ObservabilitySpec



ObservabilitySpec configures monitoring and logging.



_Appears in:_
- [InstanceSpec](#instancespec)
- [PaperclipClusterDefaultsSpec](#paperclipclusterdefaultsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `metrics` _[MetricsSpec](#metricsspec)_ | Metrics configures Prometheus metrics. |  | Optional: \{\} <br /> |
| `logging` _[LoggingSpec](#loggingspec)_ | Logging configures log level and format. |  | Optional: \{\} <br /> |


#### PDBSpec



PDBSpec configures a PodDisruptionBudget.



_Appears in:_
- [AvailabilitySpec](#availabilityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether a PDB is created. |  |  |
| `minAvailable` _integer_ | MinAvailable specifies the minimum number of pods that must be available. |  | Optional: \{\} <br /> |
| `maxUnavailable` _integer_ | MaxUnavailable specifies the maximum number of pods that can be unavailable. |  | Optional: \{\} <br /> |


#### PaperclipClusterDefaults



PaperclipClusterDefaults is a cluster-scoped singleton (name must be "cluster")
that provides default values merged into every Instance at reconcile time. It
gives platform operators a single source of truth for org-wide image, storage
class, database mode, observability, networking, and shared environment-variable
defaults without duplicating the same boilerplate in every Instance manifest.

Precedence: per-instance fields always win over cluster defaults. A default is
only applied when the corresponding instance field is unset. The merged values
are used only for rendering owned resources; the user's stored spec in etcd is
never overwritten.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `paperclip.inc/v1alpha1` | | |
| `kind` _string_ | `PaperclipClusterDefaults` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PaperclipClusterDefaultsSpec](#paperclipclusterdefaultsspec)_ |  |  |  |


#### PaperclipClusterDefaultsSpec



PaperclipClusterDefaultsSpec defines cluster-wide defaults that the operator
merges into every Instance at reconcile time. Per-instance fields always win:
a default is only applied when the corresponding instance field is unset.



_Appears in:_
- [PaperclipClusterDefaults](#paperclipclusterdefaults)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _[ImageSpec](#imagespec)_ | Image is the default container image configuration applied to instances<br />where the corresponding instance fields are unset. Each sub-field is<br />merged independently (e.g. a cluster-default tag still applies even when<br />the instance sets its own repository). |  | Optional: \{\} <br /> |
| `storageClass` _string_ | StorageClass is the default storage class applied to the Paperclip data<br />PVC, the managed PostgreSQL PVC when those<br />fields are unset on the instance. |  | Optional: \{\} <br /> |
| `databaseMode` _string_ | DatabaseMode is the default database mode ("embedded", "external", or<br />"managed") applied to instances where spec.database.mode is unset. |  | Enum: [embedded external managed] <br />Optional: \{\} <br /> |
| `observability` _[ObservabilitySpec](#observabilityspec)_ | Observability configures cluster-wide observability defaults that are<br />merged into instances where the corresponding fields are unset. |  | Optional: \{\} <br /> |
| `networking` _[NetworkingSpec](#networkingspec)_ | Networking configures cluster-wide networking defaults. Currently only<br />the default Service type is merged when the instance leaves it unset. |  | Optional: \{\} <br /> |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#envvar-v1-core) array_ | Env is a list of default environment variables merged into every<br />instance's container env. Instance-level env entries with the same Name<br />override the cluster default for that name. Defaults appear first in the<br />resulting env list, followed by instance-only names. |  | Optional: \{\} <br /> |


#### PaperclipSelfConfig



PaperclipSelfConfig is the Schema for the paperclipselfconfigs API. It
represents a request from an agent to modify its own parent Instance spec,
gated by the parent's .spec.selfConfigure allowlist and applied via
Server-Side Apply with a dedicated field manager.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `paperclip.inc/v1alpha1` | | |
| `kind` _string_ | `PaperclipSelfConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[PaperclipSelfConfigSpec](#paperclipselfconfigspec)_ |  |  |  |


#### PaperclipSelfConfigSpec



PaperclipSelfConfigSpec is an agent-driven, audited request to mutate the
parent Instance. The operator validates against the parent's
.spec.selfConfigure policy, then applies via Server-Side Apply with the
dedicated field manager "paperclip-selfconfig" so GitOps controllers do not
flap over the agent-owned fields.



_Appears in:_
- [PaperclipSelfConfig](#paperclipselfconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `instanceRef` _string_ | InstanceRef is the name of the parent Instance in the same namespace. |  | MaxLength: 253 <br />MinLength: 1 <br /> |
| `addPlugins` _[PluginRef](#pluginref) array_ | AddPlugins appends plugins to the parent's .spec.plugins. |  | MaxItems: 20 <br />Optional: \{\} <br /> |
| `removePlugins` _string array_ | RemovePlugins is a list of plugin names to remove from the parent. |  | MaxItems: 20 <br />Optional: \{\} <br /> |
| `patchConfig` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#json-v1-apiextensions-k8s-io)_ | PatchConfig is a JSON object deep-merged into the agent runtime config<br />(exposed to the container as PAPERCLIP_CONFIG_PATCH). Protected keys are<br />rejected by the operator. |  | Optional: \{\} <br /> |
| `addEnvVars` _[SelfConfigEnvVar](#selfconfigenvvar) array_ | AddEnvVars is a list of environment variables to add (plain values only). |  | MaxItems: 20 <br />Optional: \{\} <br /> |
| `removeEnvVars` _string array_ | RemoveEnvVars is a list of environment variable names to remove. |  | MaxItems: 20 <br />Optional: \{\} <br /> |


#### PersistenceSpec



PersistenceSpec configures PVC settings.



_Appears in:_
- [StorageSpec](#storagespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether a PVC is created. Defaults to true. | true | Optional: \{\} <br /> |
| `size` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#quantity-resource-api)_ | Size is the PVC storage size. | 5Gi | Optional: \{\} <br /> |
| `storageClass` _string_ | StorageClass is the storage class for the PVC. |  | Optional: \{\} <br /> |
| `accessModes` _[PersistentVolumeAccessMode](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#persistentvolumeaccessmode-v1-core) array_ | AccessModes specifies the PVC access modes. |  | Optional: \{\} <br /> |


#### PlatformAdminSpec



PlatformAdminSpec describes the platform-managed instance-admin seeded into a
freshly provisioned Paperclip instance for the shared cloud. It maps to the
PAPERCLIP_SEED_ADMIN_* env vars consumed by the seed-instance-admin CLI.



_Appears in:_
- [DeploymentSpec](#deploymentspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `email` _string_ | Email is the platform instance-admin's email address. Required.<br />Wired to PAPERCLIP_SEED_ADMIN_EMAIL. |  |  |
| `name` _string_ | Name is the optional display name for the platform instance-admin.<br />Wired to PAPERCLIP_SEED_ADMIN_NAME. |  | Optional: \{\} <br /> |
| `userID` _string_ | UserID is the optional stable user ID to assign to the platform<br />instance-admin (e.g. the Paperclip ID). Wired to PAPERCLIP_SEED_ADMIN_USER_ID. |  | Optional: \{\} <br /> |


#### PluginRef



PluginRef references a Paperclip plugin.



_Appears in:_
- [InstanceSpec](#instancespec)
- [PaperclipSelfConfigSpec](#paperclipselfconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the plugin package name. |  |  |
| `version` _string_ | Version is the plugin version. |  | Optional: \{\} <br /> |


#### ProbeSpec



ProbeSpec configures an individual probe.



_Appears in:_
- [ProbesSpec](#probesspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `initialDelaySeconds` _integer_ | InitialDelaySeconds is the number of seconds after the container starts before the probe is initiated. |  | Optional: \{\} <br /> |
| `periodSeconds` _integer_ | PeriodSeconds is how often (in seconds) to perform the probe. |  | Optional: \{\} <br /> |
| `timeoutSeconds` _integer_ | TimeoutSeconds is the number of seconds after which the probe times out. |  | Optional: \{\} <br /> |
| `failureThreshold` _integer_ | FailureThreshold is the number of consecutive failures before the probe is considered failed. |  | Optional: \{\} <br /> |
| `successThreshold` _integer_ | SuccessThreshold is the number of consecutive successes before the probe is considered successful. |  | Optional: \{\} <br /> |


#### ProbesSpec



ProbesSpec configures health probes.



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | Type specifies the probe mechanism: "auto" (default), "http", or "tcp".<br />"auto" uses HTTP probes in local_trusted mode and TCP probes in authenticated mode<br />(where /api/health returns 403 without credentials). | auto | Enum: [auto http tcp] <br />Optional: \{\} <br /> |
| `liveness` _[ProbeSpec](#probespec)_ | Liveness configures the liveness probe against /api/health. |  | Optional: \{\} <br /> |
| `readiness` _[ProbeSpec](#probespec)_ | Readiness configures the readiness probe against /api/health. |  | Optional: \{\} <br /> |
| `startup` _[ProbeSpec](#probespec)_ | Startup configures the startup probe against /api/health. |  | Optional: \{\} <br /> |


#### PrometheusRuleSpec



PrometheusRuleSpec configures an auto-provisioned PrometheusRule.



_Appears in:_
- [MetricsSpec](#metricsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled enables PrometheusRule creation with operator alerts. | false | Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels to add to the PrometheusRule (e.g. for Prometheus rule selector matching). |  | Optional: \{\} <br /> |
| `runbookBaseURL` _string_ | RunbookBaseURL is the base URL for alert runbook links. | https://paperclip.inc/docs/operators/paperclip/runbooks | Optional: \{\} <br /> |


#### RBACSpec



RBACSpec configures ServiceAccount and RBAC.



_Appears in:_
- [SecuritySpec](#securityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `create` _boolean_ | Create controls whether a ServiceAccount is created. Defaults to true. | true | Optional: \{\} <br /> |
| `serviceAccountAnnotations` _object (keys:string, values:string)_ | ServiceAccountAnnotations specifies additional annotations for the ServiceAccount. |  | Optional: \{\} <br /> |


#### SecretsSpec



SecretsSpec configures Paperclip's built-in secrets management.



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `provider` _string_ | Provider selects the secrets backend. "local_encrypted" (default) encrypts<br />secrets with the master key; "aws_secrets_manager" stores them in AWS<br />Secrets Manager. Maps to PAPERCLIP_SECRETS_PROVIDER. | local_encrypted | Enum: [local_encrypted aws_secrets_manager] <br />Optional: \{\} <br /> |
| `aws` _[AWSSecretsManagerSpec](#awssecretsmanagerspec)_ | AWS configures the AWS Secrets Manager provider. AWS credentials are sourced<br />from the standard AWS SDK credential chain - use IRSA via<br />spec.security.rbac.serviceAccountAnnotations rather than injecting keys. |  | Optional: \{\} <br /> |
| `masterKeySecretRef` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#secretkeyselector-v1-core)_ | MasterKeySecretRef references a Secret containing the master encryption key. |  | Optional: \{\} <br /> |
| `strictMode` _boolean_ | StrictMode requires all sensitive values to use encrypted references. |  | Optional: \{\} <br /> |


#### SecuritySpec



SecuritySpec configures security settings.



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#podsecuritycontext-v1-core)_ | PodSecurityContext specifies security settings for the pod. |  | Optional: \{\} <br /> |
| `containerSecurityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#securitycontext-v1-core)_ | ContainerSecurityContext specifies security settings for the Paperclip container. |  | Optional: \{\} <br /> |
| `networkPolicy` _[NetworkPolicySpec](#networkpolicyspec)_ | NetworkPolicy configures network isolation. |  | Optional: \{\} <br /> |
| `rbac` _[RBACSpec](#rbacspec)_ | RBAC configures ServiceAccount and RBAC settings. |  | Optional: \{\} <br /> |


#### SelfConfigAction

_Underlying type:_ _string_

SelfConfigAction names a category of mutation. Used by
Instance.spec.selfConfigure.allowedActions to gate what the agent may
request via PaperclipSelfConfig.

_Validation:_
- Enum: [plugins config envVars]

_Appears in:_
- [SelfConfigureSpec](#selfconfigurespec)

| Field | Description |
| --- | --- |
| `plugins` | SelfConfigActionPlugins permits adding/removing plugins.<br /> |
| `config` | SelfConfigActionConfig permits patching the agent runtime config.<br /> |
| `envVars` | SelfConfigActionEnvVars permits adding/removing plain-value env vars.<br /> |


#### SelfConfigEnvVar



SelfConfigEnvVar defines a plain-value environment variable (no secret refs).



_Appears in:_
- [PaperclipSelfConfigSpec](#paperclipselfconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the environment variable. Must be a C_IDENTIFIER. |  | MinLength: 1 <br />Pattern: `^[A-Za-z_][A-Za-z0-9_]*$` <br /> |
| `value` _string_ | Value of the environment variable. |  |  |




#### SelfConfigureSpec



SelfConfigureSpec configures whether an agent can modify its own Instance.



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled enables self-configuration for this instance. When true, the<br />agent can create PaperclipSelfConfig resources to modify its own spec. | false | Optional: \{\} <br /> |
| `allowedActions` _[SelfConfigAction](#selfconfigaction) array_ | AllowedActions restricts which action categories the agent can perform.<br />If empty and enabled is true, no actions are allowed (fail-safe). |  | Enum: [plugins config envVars] <br />MaxItems: 3 <br />Optional: \{\} <br /> |


#### ServiceMonitorSpec



ServiceMonitorSpec configures a Prometheus ServiceMonitor.



_Appears in:_
- [MetricsSpec](#metricsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled controls whether a ServiceMonitor is created. |  |  |
| `interval` _string_ | Interval specifies the scrape interval. | 30s | Optional: \{\} <br /> |


#### ServiceSpec



ServiceSpec configures the Kubernetes Service.



_Appears in:_
- [NetworkingSpec](#networkingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#servicetype-v1-core)_ | Type is the Kubernetes Service type. | ClusterIP | Enum: [ClusterIP LoadBalancer NodePort] <br />Optional: \{\} <br /> |
| `port` _integer_ | Port is the service port. Defaults to 3100 (Paperclip's default). | 3100 | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations specifies additional annotations for the Service. |  | Optional: \{\} <br /> |


#### StorageSpec



StorageSpec configures persistent storage.



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `persistence` _[PersistenceSpec](#persistencespec)_ | Persistence configures the PVC for the Paperclip data directory (/paperclip). |  | Optional: \{\} <br /> |


#### TailscaleAuthKeySpec



TailscaleAuthKeySpec references a Secret key holding the Tailscale auth key.



_Appears in:_
- [TailscaleSpec](#tailscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretRef` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#localobjectreference-v1-core)_ | SecretRef references the Secret containing the auth key. |  |  |
| `key` _string_ | Key is the key within the referenced Secret. Defaults to "authkey". | authkey | Optional: \{\} <br /> |


#### TailscaleImageSpec



TailscaleImageSpec defines the Tailscale sidecar container image.



_Appears in:_
- [TailscaleSpec](#tailscalespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repository` _string_ | Repository is the container image repository. | ghcr.io/tailscale/tailscale | Optional: \{\} <br /> |
| `tag` _string_ | Tag is the container image tag. | stable | Optional: \{\} <br /> |
| `digest` _string_ | Digest is the container image digest for supply-chain security. |  | Optional: \{\} <br /> |


#### TailscaleSpec



TailscaleSpec configures an ephemeral Tailscale sidecar for secure tailnet
access. When enabled, a userspace tailscaled sidecar runs alongside the
Paperclip container and Serves the app (port 3100) over the tailnet via
TS_SERVE_CONFIG. Use an ephemeral, reusable auth key from the Tailscale admin
console so the node is automatically removed when the pod is deleted.



_Appears in:_
- [InstanceSpec](#instancespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled enables the Tailscale sidecar. | false | Optional: \{\} <br /> |
| `mode` _string_ | Mode selects the Tailscale exposure mode.<br />"serve" exposes the instance to tailnet members only (default).<br />"funnel" exposes the instance to the public internet via Tailscale Funnel. | serve | Enum: [serve funnel] <br />Optional: \{\} <br /> |
| `image` _[TailscaleImageSpec](#tailscaleimagespec)_ | Image configures the Tailscale sidecar container image. |  | Optional: \{\} <br /> |
| `authKey` _[TailscaleAuthKeySpec](#tailscaleauthkeyspec)_ | AuthKey references a Secret containing the Tailscale auth key. The Secret<br />must have a key matching AuthKey.Key (default: "authkey"). Use an<br />ephemeral+reusable key from the Tailscale admin console. |  | Optional: \{\} <br /> |
| `hostname` _string_ | Hostname sets the Tailscale device name (defaults to the instance name). |  | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.33/#resourcerequirements-v1-core)_ | Resources specifies compute resources for the Tailscale sidecar container. |  | Optional: \{\} <br /> |


