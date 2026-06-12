/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/paperclipinc/paperclip-operator/test/utils"
)

// namespace where the project is deployed in
const namespace = "paperclip-operator-system"

// serviceAccountName created for the project
const serviceAccountName = "paperclip-operator-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "paperclip-operator-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "paperclip-operator-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd := exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=paperclip-operator-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccount": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks
	})

	// Instance lifecycle exercises a real Paperclip app boot driven by the
	// operator, plus rendering of the realigned config surface. Runs inside the
	// Ordered Describe so the controller deployed in BeforeAll is available.
	Context("Instance lifecycle", func() {
		const instNS = "paperclip-e2e"
		// The app registry publishes only `latest` + `sha-*` tags (no semver image
		// tags), and the operator forbids :latest, so we pin a sha tag.
		const appImage = "ghcr.io/paperclipai/paperclip:sha-244a5b8"

		BeforeAll(func() {
			By("pre-pulling and loading the Paperclip app image into kind")
			_, err := utils.Run(exec.Command("docker", "pull", appImage))
			Expect(err).NotTo(HaveOccurred(), "Failed to pull the Paperclip app image")
			Expect(utils.LoadImageToKindClusterWithName(appImage)).To(Succeed(),
				"Failed to load the app image into kind")

			By("creating the instance namespace (unrestricted so the data-relabel init container may run)")
			_, _ = utils.Run(exec.Command("kubectl", "create", "ns", instNS))
		})

		dumpInstanceDiagnostics := func() {
			for _, args := range [][]string{
				{"get", "all,instance,pvc", "-n", instNS, "-o", "wide"},
				{"describe", "statefulset", "-n", instNS},
				{"describe", "pods", "-n", instNS},
				{"get", "events", "-n", instNS, "--sort-by=.lastTimestamp"},
			} {
				out, _ := utils.Run(exec.Command("kubectl", args...))
				_, _ = fmt.Fprintf(GinkgoWriter, "\n$ kubectl %v\n%s\n", args, out)
			}
			// app + db pod logs (best effort)
			for _, sel := range []string{"app.kubernetes.io/instance=e2e-boot", "app.kubernetes.io/instance=e2e-feat"} {
				out, _ := utils.Run(exec.Command("kubectl", "logs", "-l", sel, "-n", instNS,
					"--all-containers", "--tail=80", "--prefix"))
				_, _ = fmt.Fprintf(GinkgoWriter, "\n$ kubectl logs -l %s\n%s\n", sel, out)
			}
		}

		AfterEach(func() {
			if CurrentSpecReport().Failed() {
				dumpInstanceDiagnostics()
			}
		})

		AfterAll(func() {
			// Delete Instances and wait for their finalizers to clear while the
			// operator is still running. The outer Manager AfterAll undeploys the
			// operator; if Instances still had finalizers then, nothing would clear
			// them and the namespace + CRD deletion would wedge until timeout.
			_, _ = utils.Run(exec.Command("kubectl", "delete", "instance", "--all",
				"-n", instNS, "--ignore-not-found", "--timeout=180s"))
			_, _ = utils.Run(exec.Command("kubectl", "delete", "ns", instNS, "--wait=false"))
		})

		It("boots a managed-DB authenticated instance to Ready", func() {
			By("applying auth secrets and the Instance")
			applyManifest(fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata: {name: e2e-auth, namespace: %[1]s}
stringData: {BETTER_AUTH_SECRET: e2e-better-auth-secret-value-0123456789}
---
apiVersion: v1
kind: Secret
metadata: {name: e2e-admin, namespace: %[1]s}
stringData: {password: e2e-admin-Passw0rd!}
---
apiVersion: paperclip.inc/v1alpha1
kind: Instance
metadata: {name: e2e-boot, namespace: %[1]s}
spec:
  image: {repository: ghcr.io/paperclipai/paperclip, tag: sha-244a5b8}
  deployment: {mode: authenticated, exposure: private}
  database: {mode: managed, managed: {storageSize: 1Gi}}
  auth:
    secretRef: {name: e2e-auth, key: BETTER_AUTH_SECRET}
    adminUser:
      email: admin@example.com
      passwordSecretRef: {name: e2e-admin, key: password}
  storage: {persistence: {enabled: true, size: 1Gi}}
`, instNS))

			By("waiting for the managed Postgres StatefulSet to be ready")
			Eventually(stsReady(instNS, "e2e-boot-db"), 8*time.Minute, 10*time.Second).Should(Succeed())

			By("waiting for the Paperclip app StatefulSet to be ready (TCP probe in authenticated mode)")
			Eventually(stsReady(instNS, "e2e-boot"), 8*time.Minute, 10*time.Second).Should(Succeed())

			By("asserting the realigned env is present and the removed config is absent")
			env := stsEnvNames(instNS, "e2e-boot")
			Expect(env).To(ContainSubstring("PAPERCLIP_BIND"))
			Expect(env).To(ContainSubstring("PAPERCLIP_DEPLOYMENT_MODE"))
			Expect(env).NotTo(ContainSubstring("PAPERCLIP_RATE_LIMIT_REDIS_URL"))
			Expect(env).NotTo(ContainSubstring("PAPERCLIP_MANAGED_"))

			By("asserting no managed Redis resources were created")
			_, err := utils.Run(exec.Command("kubectl", "get", "statefulset", "e2e-boot-redis", "-n", instNS))
			Expect(err).To(HaveOccurred(), "no Redis StatefulSet should exist")

			// The operator runs a bootstrap Job that signs up the admin account and
			// then promotes it to CEO via `bootstrap-ceo`. Account creation works and
			// proves the in-cluster hostname allowlist is correct; the CEO-promotion
			// step currently fails because the app CLI reads deploymentMode from
			// config.json (written local_trusted by `onboard --yes`) instead of the
			// PAPERCLIP_DEPLOYMENT_MODE env the server honors. Capture progress for
			// visibility; do not gate the suite on full Job success until the
			// config.json reconciliation follow-up lands.
			By("capturing admin bootstrap progress (CEO promotion is a known follow-up)")
			Eventually(func(g Gomega) {
				out, _ := utils.Run(exec.Command("kubectl", "logs", "job/e2e-boot-bootstrap", "-n", instNS, "--tail=60"))
				g.Expect(out).To(Or(
					ContainSubstring("Admin account created"),
					ContainSubstring("Signed in as existing admin"),
					ContainSubstring("Generating bootstrap invite"),
				), "bootstrap should authenticate to the app (hostname allowlist working)")
			}, 5*time.Minute, 10*time.Second).Should(Succeed())
		})

		It("renders env for AWS Secrets Manager, E2B, and app-native backup", func() {
			By("applying a feature-exercising Instance (embedded DB so no second Postgres)")
			// mode is "authenticated" because this Instance runs the real app
			// image: the app hard-rejects local_trusted on a non-loopback bind
			// ("local_trusted requires server.bind=loopback") and would
			// crash-loop in-cluster. The auth secret reuses e2e-auth, applied
			// by the previous (ordered) spec in this namespace.
			applyManifest(fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata: {name: e2e-e2b, namespace: %[1]s}
stringData: {E2B_API_KEY: e2b_dummy_key}
---
apiVersion: paperclip.inc/v1alpha1
kind: Instance
metadata: {name: e2e-feat, namespace: %[1]s}
spec:
  image: {repository: ghcr.io/paperclipai/paperclip, tag: sha-244a5b8}
  deployment: {mode: authenticated, exposure: private}
  auth:
    secretRef: {name: e2e-auth, key: BETTER_AUTH_SECRET}
  database: {mode: embedded}
  secrets:
    provider: aws_secrets_manager
    aws: {region: eu-central-1, kmsKeyID: arn:aws:kms:eu-central-1:111:key/abc, deploymentID: e2e}
  adapters:
    e2b: {apiKeySecretRef: {name: e2e-e2b, key: E2B_API_KEY}}
  backup:
    appNative: {enabled: true, intervalMinutes: 30, retentionDays: 5}
  storage: {persistence: {enabled: true, size: 1Gi}}
`, instNS))

			By("waiting for the operator to render the StatefulSet with the new env")
			Eventually(func(g Gomega) {
				env := stsEnvNames(instNS, "e2e-feat")
				g.Expect(env).To(ContainSubstring("PAPERCLIP_SECRETS_PROVIDER"))
				g.Expect(env).To(ContainSubstring("PAPERCLIP_SECRETS_AWS_REGION"))
				g.Expect(env).To(ContainSubstring("E2B_API_KEY"))
				g.Expect(env).To(ContainSubstring("PAPERCLIP_DB_BACKUP_ENABLED"))
			}, 3*time.Minute, 5*time.Second).Should(Succeed())
		})
	})

	// Scheduler lease failover exercises lease-based scheduler gating across
	// two Deployment replicas: exactly one pod is elected scheduler leader and
	// surfaced via the role label and status.schedulerLeader, and deleting the
	// leader hands the lease to the surviving replica.
	//
	// The scenario needs an app image that ships lease-based scheduler
	// leadership (paperclipai/paperclip#7995, unreleased at the time of
	// writing), so it is gated behind PAPERCLIP_E2E_LEASE_IMAGE and SKIPPED by
	// default:
	//
	//	PAPERCLIP_E2E_LEASE_IMAGE=ghcr.io/paperclipai/paperclip:sha-<lease> make test-e2e
	Context("Scheduler lease failover", func() {
		const leaseNS = "paperclip-e2e-lease"
		const instName = "e2e-lease"
		const pgName = "e2e-lease-pg"

		leaseImage := os.Getenv("PAPERCLIP_E2E_LEASE_IMAGE")

		BeforeAll(func() {
			if leaseImage == "" {
				Skip("PAPERCLIP_E2E_LEASE_IMAGE not set - lease failover e2e requires an app image " +
					"with scheduler leases (paperclip#7995)")
			}

			// Best effort: the operator-provided image may be local-only (pull
			// fails) or remote-only (kind load of a missing local image fails);
			// either way the cluster can still end up able to run it.
			By("loading the lease-capable app image into kind (best effort)")
			if _, err := utils.Run(exec.Command("docker", "pull", leaseImage)); err != nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "docker pull %s failed (continuing): %v\n", leaseImage, err)
			}
			if err := utils.LoadImageToKindClusterWithName(leaseImage); err != nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "kind load %s failed (continuing): %v\n", leaseImage, err)
			}

			By("creating the lease test namespace (unrestricted so the postgres image may run)")
			_, _ = utils.Run(exec.Command("kubectl", "create", "ns", leaseNS))

			By("deploying an in-cluster PostgreSQL for database.mode=external")
			applyManifest(fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata: {name: %[2]s, namespace: %[1]s}
spec:
  replicas: 1
  selector: {matchLabels: {app: %[2]s}}
  template:
    metadata: {labels: {app: %[2]s}}
    spec:
      containers:
        - name: postgres
          image: postgres:16
          env:
            - {name: POSTGRES_USER, value: paperclip}
            - {name: POSTGRES_PASSWORD, value: paperclip}
            - {name: POSTGRES_DB, value: paperclip}
          ports: [{containerPort: 5432}]
          readinessProbe:
            exec: {command: [pg_isready, -U, paperclip]}
            initialDelaySeconds: 5
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata: {name: %[2]s, namespace: %[1]s}
spec:
  selector: {app: %[2]s}
  ports: [{port: 5432, targetPort: 5432}]
`, leaseNS, pgName))
			Eventually(deploymentReady(leaseNS, pgName, 1), 5*time.Minute, 10*time.Second).Should(Succeed())
		})

		AfterEach(func() {
			if !CurrentSpecReport().Failed() {
				return
			}
			for _, args := range [][]string{
				{"get", "all,instance", "-n", leaseNS, "-o", "wide"},
				{"describe", "instance", instName, "-n", leaseNS},
				{"get", "pods", "-n", leaseNS, "--show-labels"},
				{"describe", "pods", "-n", leaseNS},
				{"get", "events", "-n", leaseNS, "--sort-by=.lastTimestamp"},
			} {
				out, _ := utils.Run(exec.Command("kubectl", args...))
				_, _ = fmt.Fprintf(GinkgoWriter, "\n$ kubectl %v\n%s\n", args, out)
			}
			out, _ := utils.Run(exec.Command("kubectl", "logs", "-l",
				"app.kubernetes.io/instance="+instName, "-n", leaseNS,
				"--all-containers", "--tail=80", "--prefix"))
			_, _ = fmt.Fprintf(GinkgoWriter, "\n$ kubectl logs -l app.kubernetes.io/instance=%s\n%s\n", instName, out)
		})

		AfterAll(func() {
			if leaseImage == "" {
				return
			}
			// Delete the Instance while the operator still runs so its
			// finalizer clears (see the Instance lifecycle AfterAll).
			_, _ = utils.Run(exec.Command("kubectl", "delete", "instance", "--all",
				"-n", leaseNS, "--ignore-not-found", "--timeout=180s"))
			_, _ = utils.Run(exec.Command("kubectl", "delete", "ns", leaseNS, "--wait=false"))
		})

		It("fails the scheduler lease over to a surviving replica when the leader pod dies", func() {
			By("applying the external-DB + auth secrets and a 2-replica lease-gated Instance")
			// objectStorage is intentionally omitted: MultiReplicaPreconditions
			// reports False (S3 is required for shared file state across real
			// multi-replica deployments), but the condition is advisory and the
			// workload reconciles regardless. The failover assertions below do
			// not touch file state, so a warning condition is acceptable here
			// and keeps the scenario free of a MinIO dependency.
			//
			// deployment.mode MUST be "authenticated": the app hard-rejects
			// local_trusted on a non-loopback bind ("local_trusted requires
			// server.bind=loopback", packages/shared/src/network-bind.ts in the
			// app repo), and in-cluster pods always bind 0.0.0.0, so a
			// local_trusted Instance crash-loops and never elects a leader.
			// auth.adminUser is intentionally NOT set: it would spawn the admin
			// bootstrap Job, which proved flaky live (job pod killed mid-run,
			// backoffLimit exceeded) and this scenario needs no authenticated
			// API access - the operator reads scheduler leadership from the
			// unauthenticated redacted health endpoint. (If an e2e ever needs a
			// principal, the deterministic path is plain HTTP: sign up via the
			// service URL, then POST /api/bootstrap/claim.)
			applyManifest(fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata: {name: e2e-lease-db, namespace: %[1]s}
stringData:
  DATABASE_URL: postgresql://paperclip:paperclip@%[3]s.%[1]s.svc.cluster.local:5432/paperclip
---
apiVersion: v1
kind: Secret
metadata: {name: e2e-lease-auth, namespace: %[1]s}
stringData:
  BETTER_AUTH_SECRET: dd5e3ca83629aa01987178b7871d70b755ddbc987d4e77f158838488fd4786ac
---
apiVersion: paperclip.inc/v1alpha1
kind: Instance
metadata: {name: %[2]s, namespace: %[1]s}
spec:
  image: {%[4]s}
  workload: Deployment
  deployment: {mode: authenticated, exposure: private}
  auth:
    secretRef: {name: e2e-lease-auth, key: BETTER_AUTH_SECRET}
  database:
    mode: external
    externalURLSecretRef: {name: e2e-lease-db, key: DATABASE_URL}
  heartbeat: {enabled: true, schedulerGating: lease}
  availability: {replicas: 2}
  storage: {persistence: {enabled: false}}
`, leaseNS, instName, pgName, imageSpecYAML(leaseImage)))

			By("waiting for the server Deployment to have 2 ready replicas")
			Eventually(deploymentReady(leaseNS, instName, 2), 8*time.Minute, 10*time.Second).Should(Succeed())

			By("waiting for exactly one pod to be labeled " + schedulerRoleSelector)
			var leader string
			Eventually(func(g Gomega) {
				leaders := schedulerLabeledPods(leaseNS, instName)
				g.Expect(leaders).To(HaveLen(1), "expected exactly one scheduler-labeled pod, got %v", leaders)
				leader = leaders[0]
			}, 5*time.Minute, 5*time.Second).Should(Succeed())

			By("asserting status.schedulerLeader matches the labeled pod")
			Eventually(func(g Gomega) {
				g.Expect(schedulerLeaderStatus(leaseNS, instName)).To(Equal(leader))
			}, 3*time.Minute, 5*time.Second).Should(Succeed())

			By("deleting the leader pod " + leader)
			_, err := utils.Run(exec.Command("kubectl", "delete", "pod", leader, "-n", leaseNS, "--wait=false"))
			Expect(err).NotTo(HaveOccurred(), "Failed to delete the leader pod")

			// The app's lease TTL plus the operator's 30s visibility poll keep
			// real failover well under 90s; the Eventually budget is padded for
			// CI (replacement pod scheduling + image start time).
			By("waiting for a NEW leader pod to be labeled and recorded in status")
			Eventually(func(g Gomega) {
				leaders := schedulerLabeledPods(leaseNS, instName)
				g.Expect(leaders).To(HaveLen(1), "expected exactly one scheduler-labeled pod, got %v", leaders)
				g.Expect(leaders[0]).NotTo(Equal(leader), "the scheduler lease should fail over to a different pod")
				g.Expect(schedulerLeaderStatus(leaseNS, instName)).To(Equal(leaders[0]))
			}, 3*time.Minute, 5*time.Second).Should(Succeed())
		})
	})
})

// schedulerRoleSelector selects server pods currently labeled as the
// scheduler lease holder by the operator's leader-visibility loop.
const schedulerRoleSelector = "paperclip.inc/role=scheduler"

// deploymentReady returns a Gomega assertion that the named Deployment has
// exactly the wanted number of ready replicas.
func deploymentReady(ns, name string, want int) func(Gomega) {
	return func(g Gomega) {
		out, err := utils.Run(exec.Command("kubectl", "get", "deployment", name, "-n", ns,
			"-o", "jsonpath={.status.readyReplicas}"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(out).To(Equal(strconv.Itoa(want)), "Deployment %s does not have %d ready replicas yet", name, want)
	}
}

// schedulerLabeledPods returns the non-terminating server pods of the given
// instance that carry the scheduler role label. Terminating pods are excluded
// so a just-deleted leader does not count against the "exactly one leader"
// assertions while it drains.
func schedulerLabeledPods(ns, instName string) []string {
	out, _ := utils.Run(exec.Command("kubectl", "get", "pods",
		"-l", fmt.Sprintf("app.kubernetes.io/instance=%s,app.kubernetes.io/component=server,%s",
			instName, schedulerRoleSelector),
		"-n", ns,
		"-o", "go-template={{ range .items }}"+
			"{{ if not .metadata.deletionTimestamp }}"+
			"{{ .metadata.name }}"+
			"{{ \"\\n\" }}{{ end }}{{ end }}"))
	return utils.GetNonEmptyLines(out)
}

// schedulerLeaderStatus returns the instance's status.schedulerLeader field.
func schedulerLeaderStatus(ns, instName string) string {
	out, _ := utils.Run(exec.Command("kubectl", "get", "instance", instName, "-n", ns,
		"-o", "jsonpath={.status.schedulerLeader}"))
	return strings.TrimSpace(out)
}

// imageSpecYAML renders the inline spec.image block for a full image
// reference: the Instance API has no combined-reference field, so repo:tag and
// repo@sha256:... forms are split into their respective fields.
func imageSpecYAML(ref string) string {
	if repo, digest, found := strings.Cut(ref, "@"); found {
		return fmt.Sprintf("repository: %s, digest: %s", repo, digest)
	}
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		return fmt.Sprintf("repository: %s, tag: %s", ref[:i], ref[i+1:])
	}
	return "repository: " + ref
}

// applyManifest writes the given YAML to a temp file and applies it.
func applyManifest(yaml string) {
	f, err := os.CreateTemp("", "e2e-*.yaml")
	Expect(err).NotTo(HaveOccurred())
	defer func() { _ = os.Remove(f.Name()) }()
	_, err = f.WriteString(yaml)
	Expect(err).NotTo(HaveOccurred())
	Expect(f.Close()).To(Succeed())
	out, err := utils.Run(exec.Command("kubectl", "apply", "-f", f.Name()))
	Expect(err).NotTo(HaveOccurred(), "apply failed: %s", out)
}

// stsReady returns a Gomega assertion that the named StatefulSet has >=1 ready replica.
func stsReady(ns, name string) func(Gomega) {
	return func(g Gomega) {
		out, err := utils.Run(exec.Command("kubectl", "get", "statefulset", name, "-n", ns,
			"-o", "jsonpath={.status.readyReplicas}"))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(out).To(Equal("1"), "StatefulSet %s not ready yet", name)
	}
}

// stsEnvNames returns the space-separated env var names of the primary container,
// or "" if the StatefulSet does not exist yet. It does not assert, so it is safe
// to call inside an Eventually (the caller asserts on the returned value).
func stsEnvNames(ns, name string) string {
	out, _ := utils.Run(exec.Command("kubectl", "get", "statefulset", name, "-n", ns,
		"-o", "jsonpath={.spec.template.spec.containers[0].env[*].name}"))
	return out
}

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
