# Changelog

## [0.19.1](https://github.com/paperclipinc/paperclip-operator/compare/v0.19.0...v0.19.1) (2026-09-06)


### Bug Fixes

* **bootstrap:** honor spec.security.podSecurityContext on the bootstrap Job ([#114](https://github.com/paperclipinc/paperclip-operator/issues/114)) ([f0c582f](https://github.com/paperclipinc/paperclip-operator/commit/f0c582fd8ad09ff2a906ad1f6d0372dc96dac8a1))
* **bootstrap:** repair the already-bootstrapped short-circuit and stop the Job loop ([#116](https://github.com/paperclipinc/paperclip-operator/issues/116)) ([ff64ac1](https://github.com/paperclipinc/paperclip-operator/commit/ff64ac17badd21a849819c3523f6952a38a5b56c))

## [0.19.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.18.1...v0.19.0) (2026-07-28)


### Features

* **availability:** configurable terminationGracePeriodSeconds + preStop for server drain ([#106](https://github.com/paperclipinc/paperclip-operator/issues/106)) ([c414636](https://github.com/paperclipinc/paperclip-operator/commit/c414636a8c0b6eb449aef9aab67c4f5afa5d3ee4))
* configurable server termination grace + preStop drain hook ([c414636](https://github.com/paperclipinc/paperclip-operator/commit/c414636a8c0b6eb449aef9aab67c4f5afa5d3ee4))
* egressPolicy passthrough ([#104](https://github.com/paperclipinc/paperclip-operator/issues/104)) ([d459426](https://github.com/paperclipinc/paperclip-operator/commit/d4594263150368ff9df150586ef84f0b3d20feac))
* egressPolicy passthrough for tenant sandbox egress posture ([d459426](https://github.com/paperclipinc/paperclip-operator/commit/d4594263150368ff9df150586ef84f0b3d20feac))

## [0.18.1](https://github.com/paperclipinc/paperclip-operator/compare/v0.18.0...v0.18.1) (2026-07-18)


### Bug Fixes

* **chart:** allow kubeVersion pre-release suffixes from managed providers ([#102](https://github.com/paperclipinc/paperclip-operator/issues/102)) ([cb60cc4](https://github.com/paperclipinc/paperclip-operator/commit/cb60cc46e55923995ca71e8f1f287ffb8e81b3fe)), closes [#101](https://github.com/paperclipinc/paperclip-operator/issues/101)

## [0.18.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.17.2...v0.18.0) (2026-07-12)


### Features

* **crd:** perTenantQuota/perTenantLimitRange on K8sExecutionSpec ([#94](https://github.com/paperclipinc/paperclip-operator/issues/94)) ([f21a965](https://github.com/paperclipinc/paperclip-operator/commit/f21a9653b69a267ba8ee508227fc96a991d6b3c5))
* **instance:** add priorityClassName for the product pod (outage guard) ([#100](https://github.com/paperclipinc/paperclip-operator/issues/100)) ([0528728](https://github.com/paperclipinc/paperclip-operator/commit/05287287f9b446175cb38982a1ec03d2c667f1a5))


### Bug Fixes

* add spec.security.seLinuxRelabel opt-out for relabel init container ([#98](https://github.com/paperclipinc/paperclip-operator/issues/98)) ([d91ccb0](https://github.com/paperclipinc/paperclip-operator/commit/d91ccb0f673f6e66f8091b253e53cd792d486dc1))

## [0.17.2](https://github.com/paperclipinc/paperclip-operator/compare/v0.17.1...v0.17.2) (2026-06-16)


### Bug Fixes

* **operatorhub:** add updateGraph semver-mode to submitted ci.yaml ([#92](https://github.com/paperclipinc/paperclip-operator/issues/92)) ([a609947](https://github.com/paperclipinc/paperclip-operator/commit/a6099474eb4e08668f918e3f05ae5fed8f875456))

## [0.17.1](https://github.com/paperclipinc/paperclip-operator/compare/v0.17.0...v0.17.1) (2026-06-16)


### Bug Fixes

* **bundle:** declare com.redhat.openshift.versions for OpenShift cert ([#90](https://github.com/paperclipinc/paperclip-operator/issues/90)) ([48f1ccc](https://github.com/paperclipinc/paperclip-operator/commit/48f1ccca2e53d2c9fa3db063e7842d9e4b290c89))
* **bundle:** declare com.redhat.openshift.versions=v4.15 for OpenShift cert ([48f1ccc](https://github.com/paperclipinc/paperclip-operator/commit/48f1ccca2e53d2c9fa3db063e7842d9e4b290c89))

## [0.17.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.16.0...v0.17.0) (2026-06-16)


### Features

* **chart:** world-class Artifact Hub metadata + cosign chart signing ([#88](https://github.com/paperclipinc/paperclip-operator/issues/88)) ([3a5d624](https://github.com/paperclipinc/paperclip-operator/commit/3a5d624ab96a2e31199be142d4f48c051e0ad030))

## [0.16.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.15.0...v0.16.0) (2026-06-14)


### Features

* Deployment workload profile, multi-replica preconditions, scale subresource ([#81](https://github.com/paperclipinc/paperclip-operator/issues/81)) ([a3871bf](https://github.com/paperclipinc/paperclip-operator/commit/a3871bf9b6f88664e8861f6633335e3081b09075))
* **instance:** optional brand theming via spec.branding.cssConfigMapRef ([e5da3ab](https://github.com/paperclipinc/paperclip-operator/commit/e5da3ab17280d00264341c06b8165f9ef6b8f179))
* lease-aware scheduler gating, leader visibility, failover e2e ([#82](https://github.com/paperclipinc/paperclip-operator/issues/82)) ([de2005d](https://github.com/paperclipinc/paperclip-operator/commit/de2005dfce731bf0027fc8ac4fc087e6518c7d42))


### Bug Fixes

* **bootstrap:** make bootstrap Job reconcile idempotent (no immutable-template churn) ([#85](https://github.com/paperclipinc/paperclip-operator/issues/85)) ([3f1beac](https://github.com/paperclipinc/paperclip-operator/commit/3f1beac818e756dbba0c5f12d9a52f1de7f3b5d6)), closes [#83](https://github.com/paperclipinc/paperclip-operator/issues/83)

## [0.15.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.14.0...v0.15.0) (2026-06-08)


### Features

* spec.adapters.registry -&gt; PAPERCLIP_ADAPTERS ([#79](https://github.com/paperclipinc/paperclip-operator/issues/79)) ([b8b96de](https://github.com/paperclipinc/paperclip-operator/commit/b8b96de0108c1ac6d1aedeed54ac6589a82c32ca))

## [0.14.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.13.0...v0.14.0) (2026-06-05)


### Features

* **instance:** in-cluster Kubernetes execution config + scoped RBAC ([#77](https://github.com/paperclipinc/paperclip-operator/issues/77)) ([7f65f03](https://github.com/paperclipinc/paperclip-operator/commit/7f65f03744cf5ce6f1774925e1d072809bbfa976))
* **instance:** seed platform instance-admin via init container ([#75](https://github.com/paperclipinc/paperclip-operator/issues/75)) ([2b07483](https://github.com/paperclipinc/paperclip-operator/commit/2b07483edb1b51eec1e250e179aa1851e6bd2bf9))

## [0.13.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.12.1...v0.13.0) (2026-06-05)


### ⚠ BREAKING CHANGES

* remove fabricated managed-inference config
* remove managed Redis (app does not consume it)
* emit PAPERCLIP_BIND/PAPERCLIP_BIND_HOST instead of legacy HOST
* align deployment mode enum with app (local_trusted|authenticated)

### Features

* add app-native DB backup config (PAPERCLIP_DB_BACKUP_*) ([4e067c4](https://github.com/paperclipinc/paperclip-operator/commit/4e067c42236bef08210142c026a2406eb89b31b4))
* add AWS Secrets Manager secrets provider ([c2104a7](https://github.com/paperclipinc/paperclip-operator/commit/c2104a7929a210b8c5645d78d8de3a60160c3192))
* add E2B sandbox API key (spec.adapters.e2b -&gt; E2B_API_KEY) ([5fdc615](https://github.com/paperclipinc/paperclip-operator/commit/5fdc6157f913544ca4a434284e5a40f0246cac2e))
* align deployment mode enum with app (local_trusted|authenticated) ([a12efd9](https://github.com/paperclipinc/paperclip-operator/commit/a12efd9b8c44c1ba44a167d8ae4c410da8797120))
* emit PAPERCLIP_BIND/PAPERCLIP_BIND_HOST instead of legacy HOST ([ec38bb9](https://github.com/paperclipinc/paperclip-operator/commit/ec38bb9b98413cf31f5ab8aaebc5984285722196))
* remove fabricated managed-inference config ([45a4518](https://github.com/paperclipinc/paperclip-operator/commit/45a4518eacd2755c3825304e8895a5b369a7754c))
* remove managed Redis (app does not consume it) ([afde2f5](https://github.com/paperclipinc/paperclip-operator/commit/afde2f5434c18df0f26232b937990ed99de84f00))


### Bug Fixes

* **controller:** only watch Gateway API HTTPRoute when its CRD is installed ([b9e3874](https://github.com/paperclipinc/paperclip-operator/commit/b9e3874ce07b708549512462c449a8ff772a653f))
* **image:** default app image to ghcr.io/paperclipai/paperclip ([80092b5](https://github.com/paperclipinc/paperclip-operator/commit/80092b5dd1abf6b98e41a8470c1a972350fd2ba0))
* **statefulset:** make the app actually boot under restricted security ([d753fa9](https://github.com/paperclipinc/paperclip-operator/commit/d753fa90f700d5f35ab93342c950a12a9a6a8ba2))

## [0.12.1](https://github.com/paperclipinc/paperclip-operator/compare/v0.12.0...v0.12.1) (2026-06-04)


### Bug Fixes

* **olm:** include PaperclipClusterDefaults and PaperclipSelfConfig CRDs in the bundle ([#71](https://github.com/paperclipinc/paperclip-operator/issues/71)) ([f1be2dd](https://github.com/paperclipinc/paperclip-operator/commit/f1be2ddf516e7b487a2d6d0e91af6b04f78f8eac))
* **olm:** use the official Paperclip logo for the bundle icon ([#69](https://github.com/paperclipinc/paperclip-operator/issues/69)) ([b53cc5e](https://github.com/paperclipinc/paperclip-operator/commit/b53cc5edba653e3896a8b5765a046ac07fdc8e2f))

## [0.12.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.11.2...v0.12.0) (2026-06-03)


### Features

* bring paperclip-operator to parity (tier 1/2 feature port) ([#65](https://github.com/paperclipinc/paperclip-operator/issues/65)) ([97b9d0f](https://github.com/paperclipinc/paperclip-operator/commit/97b9d0ffd8ce570b6408d39ee2daa0e12b13a488))
* Tier 3 cross-pollination - PaperclipClusterDefaults + PaperclipSelfConfig CRDs + Tailscale sidecar ([#68](https://github.com/paperclipinc/paperclip-operator/issues/68)) ([5bb11f0](https://github.com/paperclipinc/paperclip-operator/commit/5bb11f0e49b32f91a627e83da41acb1a327d4dc2))

## [0.11.2](https://github.com/paperclipinc/paperclip-operator/compare/v0.11.1...v0.11.2) (2026-06-03)


### Bug Fixes

* **ci:** ship operators/paperclip-operator/ci.yaml with every submission ([#60](https://github.com/paperclipinc/paperclip-operator/issues/60)) ([42b323e](https://github.com/paperclipinc/paperclip-operator/commit/42b323e1897bcebea3d616a00ebfc1e3b71c3dc0))
* point NOTICE attribution at github.com/paperclipinc ([#62](https://github.com/paperclipinc/paperclip-operator/issues/62)) ([b50e5ed](https://github.com/paperclipinc/paperclip-operator/commit/b50e5ed08c5af179151dbb1b2a6f2d3f0af9a0d4))

## [0.11.1](https://github.com/paperclipinc/paperclip-operator/compare/v0.11.0...v0.11.1) (2026-04-18)


### Bug Fixes

* **ci:** drop --remote from gh repo fork in OperatorHub submission ([#58](https://github.com/paperclipinc/paperclip-operator/issues/58)) ([5893df1](https://github.com/paperclipinc/paperclip-operator/commit/5893df15622fe11883c093cc44107178db3cc734))

## [0.11.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.10.0...v0.11.0) (2026-04-17)


### Features

* add Gateway API HTTPRoute support ([#51](https://github.com/paperclipinc/paperclip-operator/issues/51)) ([2fc8ff6](https://github.com/paperclipinc/paperclip-operator/commit/2fc8ff6dfbf058eab2ff688938a47d48179690c3)), closes [#48](https://github.com/paperclipinc/paperclip-operator/issues/48)


### Bug Fixes

* add NODE_OPTIONS to preload OTEL instrumentation ([#39](https://github.com/paperclipinc/paperclip-operator/issues/39)) ([9c16a85](https://github.com/paperclipinc/paperclip-operator/commit/9c16a8537cf422f794979072778e899fb11d8fb2))
* add NODE_OPTIONS to preload OTEL instrumentation before app start ([9c16a85](https://github.com/paperclipinc/paperclip-operator/commit/9c16a8537cf422f794979072778e899fb11d8fb2))
* add SELinux relabel init container for persistent volumes ([#41](https://github.com/paperclipinc/paperclip-operator/issues/41)) ([93df250](https://github.com/paperclipinc/paperclip-operator/commit/93df250d6a4089febfdd037313f7117f2de5d6a6))
* allow OTEL collector egress in NetworkPolicy ([#40](https://github.com/paperclipinc/paperclip-operator/issues/40)) ([dc26f4f](https://github.com/paperclipinc/paperclip-operator/commit/dc26f4fb941bedd4db0c4067d3ec0f8c64aa117a))
* allow OTEL collector egress in NetworkPolicy (ports 4317/4318) ([dc26f4f](https://github.com/paperclipinc/paperclip-operator/commit/dc26f4fb941bedd4db0c4067d3ec0f8c64aa117a))
* allow Redis egress in NetworkPolicy for external mode ([#44](https://github.com/paperclipinc/paperclip-operator/issues/44)) ([94fc4a0](https://github.com/paperclipinc/paperclip-operator/commit/94fc4a0ad72b4b6ec333b78ec0bcedf0d1f85f82))
* apply CRD security context override to all Paperclip containers ([#46](https://github.com/paperclipinc/paperclip-operator/issues/46)) ([7e5b87a](https://github.com/paperclipinc/paperclip-operator/commit/7e5b87a20c0697cc3dc84585e1721f00f98aff50))
* apply CRD security context override to onboard and bootstrap containers ([7e5b87a](https://github.com/paperclipinc/paperclip-operator/commit/7e5b87a20c0697cc3dc84585e1721f00f98aff50)), closes [#45](https://github.com/paperclipinc/paperclip-operator/issues/45)
* require explicit image tag or digest instead of defaulting to :latest ([#54](https://github.com/paperclipinc/paperclip-operator/issues/54)) ([90a945e](https://github.com/paperclipinc/paperclip-operator/commit/90a945e6cc49ea1fb90cfd88f3a35b934aa19c41)), closes [#52](https://github.com/paperclipinc/paperclip-operator/issues/52)
* set runAsNonRoot=false on SELinux relabel init container ([#42](https://github.com/paperclipinc/paperclip-operator/issues/42)) ([d6aac33](https://github.com/paperclipinc/paperclip-operator/commit/d6aac338a51981885912eb74369ec7e10bbf987c))

## [0.10.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.9.1...v0.10.0) (2026-04-06)


### Features

* inject OTEL env vars and Prometheus scrape annotations ([#37](https://github.com/paperclipinc/paperclip-operator/issues/37)) ([c10a83a](https://github.com/paperclipinc/paperclip-operator/commit/c10a83a514a8c1a46ff31082d0ac6078e0753494))


### Bug Fixes

* add NODE_OPTIONS to preload OTEL instrumentation ([#39](https://github.com/paperclipinc/paperclip-operator/issues/39)) ([9c16a85](https://github.com/paperclipinc/paperclip-operator/commit/9c16a8537cf422f794979072778e899fb11d8fb2))
* add NODE_OPTIONS to preload OTEL instrumentation before app start ([9c16a85](https://github.com/paperclipinc/paperclip-operator/commit/9c16a8537cf422f794979072778e899fb11d8fb2))
* add SELinux relabel init container for persistent volumes ([#41](https://github.com/paperclipinc/paperclip-operator/issues/41)) ([93df250](https://github.com/paperclipinc/paperclip-operator/commit/93df250d6a4089febfdd037313f7117f2de5d6a6))
* allow OTEL collector egress in NetworkPolicy ([#40](https://github.com/paperclipinc/paperclip-operator/issues/40)) ([dc26f4f](https://github.com/paperclipinc/paperclip-operator/commit/dc26f4fb941bedd4db0c4067d3ec0f8c64aa117a))
* allow OTEL collector egress in NetworkPolicy (ports 4317/4318) ([dc26f4f](https://github.com/paperclipinc/paperclip-operator/commit/dc26f4fb941bedd4db0c4067d3ec0f8c64aa117a))
* allow PostgreSQL egress in NetworkPolicy for external databases ([#36](https://github.com/paperclipinc/paperclip-operator/issues/36)) ([56939c8](https://github.com/paperclipinc/paperclip-operator/commit/56939c83b4645e724d9e59205fea014151587b9d))
* allow Redis egress in NetworkPolicy for external mode ([#44](https://github.com/paperclipinc/paperclip-operator/issues/44)) ([94fc4a0](https://github.com/paperclipinc/paperclip-operator/commit/94fc4a0ad72b4b6ec333b78ec0bcedf0d1f85f82))
* apply CRD security context override to all Paperclip containers ([#46](https://github.com/paperclipinc/paperclip-operator/issues/46)) ([7e5b87a](https://github.com/paperclipinc/paperclip-operator/commit/7e5b87a20c0697cc3dc84585e1721f00f98aff50))
* apply CRD security context override to onboard and bootstrap containers ([7e5b87a](https://github.com/paperclipinc/paperclip-operator/commit/7e5b87a20c0697cc3dc84585e1721f00f98aff50)), closes [#45](https://github.com/paperclipinc/paperclip-operator/issues/45)
* set runAsNonRoot=false on SELinux relabel init container ([#42](https://github.com/paperclipinc/paperclip-operator/issues/42)) ([d6aac33](https://github.com/paperclipinc/paperclip-operator/commit/d6aac338a51981885912eb74369ec7e10bbf987c))

## [0.9.1](https://github.com/paperclipinc/paperclip-operator/compare/v0.9.0...v0.9.1) (2026-03-30)


### Bug Fixes

* add namespaces, pods/exec, pods/log to operator RBAC ([#33](https://github.com/paperclipinc/paperclip-operator/issues/33)) ([5bc541c](https://github.com/paperclipinc/paperclip-operator/commit/5bc541cb0791fc144504561e0601eb1183fbc1e9))

## [0.9.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.8.0...v0.9.0) (2026-03-30)


### Features

* auto-generate secrets master key and multi-namespace sandbox RBAC ([#31](https://github.com/paperclipinc/paperclip-operator/issues/31)) ([66009de](https://github.com/paperclipinc/paperclip-operator/commit/66009deea812d811cb83f67e965aac6ca95deba0)), closes [#29](https://github.com/paperclipinc/paperclip-operator/issues/29)


### Bug Fixes

* exclude Ready condition from its own aggregation loop ([#30](https://github.com/paperclipinc/paperclip-operator/issues/30)) ([257ec9d](https://github.com/paperclipinc/paperclip-operator/commit/257ec9d629985f442296b59e6199869844765f66)), closes [#28](https://github.com/paperclipinc/paperclip-operator/issues/28)

## [0.8.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.7.0...v0.8.0) (2026-03-25)


### Features

* add DB backup CronJob builder ([4a4f4d2](https://github.com/paperclipinc/paperclip-operator/commit/4a4f4d275e3ca65aef4eb8508cd2249e08881550))
* DB backup CronJob builder ([#26](https://github.com/paperclipinc/paperclip-operator/issues/26)) ([4a4f4d2](https://github.com/paperclipinc/paperclip-operator/commit/4a4f4d275e3ca65aef4eb8508cd2249e08881550))

## [0.7.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.6.0...v0.7.0) (2026-03-25)


### Features

* add OAuth provider and email config to AuthSpec ([e2314a9](https://github.com/paperclipinc/paperclip-operator/commit/e2314a962eba340bd25435a6de041a2888a3d0fe))


### Bug Fixes

* align S3 env var names with server config ([#24](https://github.com/paperclipinc/paperclip-operator/issues/24)) ([af31956](https://github.com/paperclipinc/paperclip-operator/commit/af3195620c5a4699b77ec12fc0a42cbd5e06439f))
* bootstrap job uses wrong health endpoint ([#21](https://github.com/paperclipinc/paperclip-operator/issues/21)) ([2011328](https://github.com/paperclipinc/paperclip-operator/commit/201132808a26b120706db31f526ffa4ced7ddcdd))
* use /api/health/details for bootstrap status check ([2011328](https://github.com/paperclipinc/paperclip-operator/commit/201132808a26b120706db31f526ffa4ced7ddcdd))

## [0.6.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.5.2...v0.6.0) (2026-03-25)


### Features

* add Redis support for rate limiting ([#19](https://github.com/paperclipinc/paperclip-operator/issues/19)) ([2385c38](https://github.com/paperclipinc/paperclip-operator/commit/2385c38be293ccba4aba18b6d1895fe2323297b7))

## [0.5.2](https://github.com/paperclipinc/paperclip-operator/compare/v0.5.1...v0.5.2) (2026-03-24)


### Bug Fixes

* add get verb to pods/exec RBAC for WebSocket exec ([#17](https://github.com/paperclipinc/paperclip-operator/issues/17)) ([ebb12cf](https://github.com/paperclipinc/paperclip-operator/commit/ebb12cfed78051e661511675c7a2103f2274b960))
* add K8s API egress and sandbox scheduling env vars ([ebb12cf](https://github.com/paperclipinc/paperclip-operator/commit/ebb12cfed78051e661511675c7a2103f2274b960))
* add K8s API egress and sandbox scheduling env vars ([edb5c33](https://github.com/paperclipinc/paperclip-operator/commit/edb5c33de19ff10e516d1cfe5913e50d36c2472b))
* add K8s API egress to NetworkPolicy for cloud sandbox ([#15](https://github.com/paperclipinc/paperclip-operator/issues/15)) ([edb5c33](https://github.com/paperclipinc/paperclip-operator/commit/edb5c33de19ff10e516d1cfe5913e50d36c2472b))

## [0.5.1](https://github.com/paperclipinc/paperclip-operator/compare/v0.5.0...v0.5.1) (2026-03-24)


### Bug Fixes

* add pods/exec and pods/log to operator ClusterRole ([#13](https://github.com/paperclipinc/paperclip-operator/issues/13)) ([97b948e](https://github.com/paperclipinc/paperclip-operator/commit/97b948e346464e6da72dfadfe24727154cdaea1b))

## [0.5.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.4.0...v0.5.0) (2026-03-24)


### Features

* managed inference, persistence, multi-namespace CRD support ([#11](https://github.com/paperclipinc/paperclip-operator/issues/11)) ([f6b1f87](https://github.com/paperclipinc/paperclip-operator/commit/f6b1f87fbf51eab097e0d482c746748fab3d387d))

## [0.4.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.3.0...v0.4.0) (2026-03-23)


### Features

* cloud sandbox support — RBAC, CRD, and env var injection ([#9](https://github.com/paperclipinc/paperclip-operator/issues/9)) ([5c7cfca](https://github.com/paperclipinc/paperclip-operator/commit/5c7cfca8965c60479bdec5042c925d2058fde7c5))

## [0.3.0](https://github.com/paperclipinc/paperclip-operator/compare/v0.2.0...v0.3.0) (2026-03-23)


### Features

* add connections spec for third-party OAuth credentials ([#6](https://github.com/paperclipinc/paperclip-operator/issues/6)) ([34add3f](https://github.com/paperclipinc/paperclip-operator/commit/34add3f06f2319dca8f495baf859bda6ec8e5b4e))
* automatic image updates via OCI registry digest polling ([#8](https://github.com/paperclipinc/paperclip-operator/issues/8)) ([90858c1](https://github.com/paperclipinc/paperclip-operator/commit/90858c14f4a305db462c28e647f8dcb3e70a1b0e))

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
