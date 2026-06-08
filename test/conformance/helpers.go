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

package conformance

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

func run(cmd string, args ...string) (string, error) {
	c := exec.Command(cmd, args...)
	b, err := c.CombinedOutput()
	return string(b), err
}

func runStdin(cmd string, args []string, stdin string) (string, error) {
	c := exec.Command(cmd, args...)
	c.Stdin = strings.NewReader(stdin)
	b, err := c.CombinedOutput()
	return string(b), err
}

func kubectl(args ...string) (string, error) { return run("kubectl", args...) }

func kubectlApply(yaml string) (string, error) {
	return runStdin("kubectl", []string{"apply", "-f", "-"}, yaml)
}

func kubectlApplyServerSide(fieldManager, yaml string) (string, error) {
	return runStdin("kubectl", []string{"apply", "--server-side", "--field-manager", fieldManager, "-f", "-"}, yaml)
}

func kubectlDelete(yaml string) (string, error) {
	return runStdin("kubectl", []string{"delete", "--ignore-not-found", "-f", "-"}, yaml)
}

func clientcmdPath() string {
	if p := os.Getenv("KUBECONFIG"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return home + "/.kube/config"
}

func newClient() client.Client {
	cfg, err := clientcmd.BuildConfigFromFlags("", clientcmdPath())
	Expect(err).ToNot(HaveOccurred())
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(paperclipv1alpha1.AddToScheme(scheme))
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	Expect(err).ToNot(HaveOccurred())
	return c
}

func newKubeClient() *kubernetes.Clientset {
	cfg, err := clientcmd.BuildConfigFromFlags("", clientcmdPath())
	Expect(err).ToNot(HaveOccurred())
	cs, err := kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())
	return cs
}

func waitForInstanceReady(ctx context.Context, c client.Client, ns, name string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		inst := &paperclipv1alpha1.Instance{}
		err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, inst)
		if err == nil && hasReadyTrue(inst) {
			return
		}
		time.Sleep(2 * time.Second)
	}
	Fail(fmt.Sprintf("Instance %s/%s did not become Ready within %s", ns, name, timeout))
}

// waitForOwnedResources blocks until the operator has created the instance's
// owned workload resources (the application StatefulSet and Service), or fails
// after timeout. Unlike waitForInstanceReady it does NOT require the workload
// Pod to pass its readiness probe: the app image may take minutes to pull and
// boot on a tiny kind cluster, and the database may not be reachable at
// all in a lightweight conformance fixture. Idempotency conformance only needs
// the operator to have settled the desired child objects so their fingerprint
// can be compared across reconciles; it does not need the app to serve traffic.
func waitForOwnedResources(ctx context.Context, c client.Client, ns, name string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		sts := &appsv1.StatefulSet{}
		stsErr := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, sts)
		svc := &corev1.Service{}
		svcErr := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, svc)
		if stsErr == nil && svcErr == nil {
			return
		}
		time.Sleep(2 * time.Second)
	}
	Fail(fmt.Sprintf("Instance %s/%s did not have its owned StatefulSet and Service created within %s", ns, name, timeout))
}

func hasReadyTrue(inst *paperclipv1alpha1.Instance) bool {
	for _, cond := range inst.Status.Conditions {
		if cond.Type == "Ready" && cond.Status == "True" {
			return true
		}
	}
	return false
}

// waitForInstanceReconciled blocks until the operator has observed and settled
// the instance's CURRENT spec generation, or fails after timeout. "Settled"
// means status.observedGeneration has caught up to metadata.generation AND the
// owned StatefulSet and Service exist.
//
// This is the correct readiness gate for conformance on a kind cluster. The
// Instance's aggregate "Ready" condition (status.conditions[Ready]=True) is
// gated on StatefulSetReady, which in turn requires the workload Pod to report
// ReadyReplicas > 0 (see reconcileStatefulSet in instance_controller.go). That
// only happens once the real ghcr.io/paperclipai/paperclip application image is
// pulled, the Paperclip-specific onboard init container completes, and the HTTP
// readiness probe passes. None of that is reachable in a lightweight kind
// fixture: the app image is large/private, the operator hard-codes
// Paperclip-specific init/main commands (so no generic stub image can satisfy
// the probes), and there is no real backing database. Gating conformance on the
// app serving traffic therefore times out deterministically and is what made
// the Failure modes / GitOps coexistence / Upgrade path jobs fail on every PR.
//
// The behaviours these specs actually assert (operator restarts and re-reconciles
// a spec edit; SSA re-apply does not flap; an image-tag change rolls the
// StatefulSet) are all about the operator converging desired child objects, not
// about the app Pod passing its probe. So we gate on the operator having
// reconciled, mirroring waitForOwnedResources used by the idempotency suite.
func waitForInstanceReconciled(ctx context.Context, c client.Client, ns, name string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		inst := &paperclipv1alpha1.Instance{}
		if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, inst); err != nil {
			last = fmt.Sprintf("get instance: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		observed := inst.Status.ObservedGeneration >= inst.Generation
		sts := &appsv1.StatefulSet{}
		stsErr := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, sts)
		svc := &corev1.Service{}
		svcErr := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, svc)
		if observed && stsErr == nil && svcErr == nil {
			return
		}
		last = fmt.Sprintf("observedGeneration=%d generation=%d stsErr=%v svcErr=%v",
			inst.Status.ObservedGeneration, inst.Generation, stsErr, svcErr)
		time.Sleep(2 * time.Second)
	}
	Fail(fmt.Sprintf("Instance %s/%s was not reconciled by the operator within %s (last: %s)", ns, name, timeout, last))
}

// statefulSetImageTag returns the image tag (the substring after the last ":")
// of the main app container in the instance's owned StatefulSet. Used by the
// upgrade-path spec to assert the operator rolled the pod template to the new
// tag, which is observable without the app Pod ever becoming Ready.
func statefulSetImageTag(ctx context.Context, c client.Client, ns, name string) (string, error) {
	sts := &appsv1.StatefulSet{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, sts); err != nil {
		return "", err
	}
	containers := sts.Spec.Template.Spec.Containers
	if len(containers) == 0 {
		return "", fmt.Errorf("StatefulSet %s/%s has no containers", ns, name)
	}
	img := containers[0].Image
	if i := strings.LastIndex(img, ":"); i >= 0 {
		return img[i+1:], nil
	}
	return img, nil
}

func forceRequeue(ctx context.Context, c client.Client, ns, name string) {
	inst := &paperclipv1alpha1.Instance{}
	Expect(c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, inst)).To(Succeed())
	if inst.Annotations == nil {
		inst.Annotations = map[string]string{}
	}
	inst.Annotations["paperclip.inc/conformance-poke"] = fmt.Sprintf("%d", time.Now().UnixNano())
	Expect(c.Update(ctx, inst)).To(Succeed())
}

type metaTuple struct {
	Generation      int64
	ResourceVersion string
}

// pvcFingerprint captures only the stable, operator-owned identity of the app
// PVC. We deliberately do NOT include resourceVersion here. The operator
// reconciles the PVC create-only (reconcilePVC in instance_controller.go does a
// Get and, when found, returns without ever writing it again), so it can never
// be the source of PVC churn. On a kind cluster the PVC's storageclass uses
// volumeBindingMode: WaitForFirstConsumer, so the moment the workload Pod is
// scheduled the local-path provisioner and the Kubernetes PV binding controller
// patch the PVC out-of-band: they add the volume.kubernetes.io/selected-node and
// pv.kubernetes.io/bind-completed annotations, set spec.volumeName, and flip
// status.phase Pending -> Bound. Each of those writes bumps metadata.resourceVersion
// even though nothing the operator manages changed. Including resourceVersion in
// the fingerprint therefore reported a phantom "idempotency broken" failure for a
// purely external, no-op-for-the-operator mutation.
//
// metadata.generation only advances on a spec change, so it is a faithful
// idempotency signal: if the operator re-applied the PVC spec on every reconcile
// it would bump generation and this check would (correctly) fail. We additionally
// pin the requested storage size and access modes to catch any spec rewrite that
// somehow preserved generation.
type pvcFingerprint struct {
	Generation  int64
	StorageReq  string
	AccessModes string
}

type resourceFingerprint struct {
	StatefulSet metaTuple
	Service     metaTuple
	PVC         pvcFingerprint
}

func captureFingerprint(ctx context.Context, c client.Client, ns, name string) resourceFingerprint {
	fp := resourceFingerprint{}
	sts := &appsv1.StatefulSet{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, sts); err == nil {
		fp.StatefulSet = metaTuple{sts.Generation, sts.ResourceVersion}
	}
	svc := &corev1.Service{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, svc); err == nil {
		fp.Service = metaTuple{svc.Generation, svc.ResourceVersion}
	}
	pvc := &corev1.PersistentVolumeClaim{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name + "-data"}, pvc); err == nil {
		req := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		modes := make([]string, 0, len(pvc.Spec.AccessModes))
		for _, m := range pvc.Spec.AccessModes {
			modes = append(modes, string(m))
		}
		fp.PVC = pvcFingerprint{
			Generation:  pvc.Generation,
			StorageReq:  req.String(),
			AccessModes: strings.Join(modes, ","),
		}
	}
	return fp
}

func expectFingerprintUnchanged(before, after resourceFingerprint) {
	check := func(fieldName string, b, a metaTuple) {
		Expect(a.Generation).To(Equal(b.Generation),
			fmt.Sprintf("%s.metadata.generation changed: %d -> %d (idempotency broken)", fieldName, b.Generation, a.Generation))
		Expect(a.ResourceVersion).To(Equal(b.ResourceVersion),
			fmt.Sprintf("%s.metadata.resourceVersion changed: %s -> %s (idempotency broken)", fieldName, b.ResourceVersion, a.ResourceVersion))
	}
	check("StatefulSet", before.StatefulSet, after.StatefulSet)
	check("Service", before.Service, after.Service)

	// PVC: compare stable spec only. resourceVersion is intentionally excluded
	// (see pvcFingerprint doc) because external storage controllers churn it on
	// kind while the operator only ever creates the PVC once.
	Expect(after.PVC.Generation).To(Equal(before.PVC.Generation),
		fmt.Sprintf("PVC.metadata.generation changed: %d -> %d (operator re-applied the PVC spec; idempotency broken)",
			before.PVC.Generation, after.PVC.Generation))
	Expect(after.PVC.StorageReq).To(Equal(before.PVC.StorageReq),
		fmt.Sprintf("PVC requested storage changed: %s -> %s (idempotency broken)", before.PVC.StorageReq, after.PVC.StorageReq))
	Expect(after.PVC.AccessModes).To(Equal(before.PVC.AccessModes),
		fmt.Sprintf("PVC access modes changed: %s -> %s (idempotency broken)", before.PVC.AccessModes, after.PVC.AccessModes))
}

func readFile(path string) string {
	b, err := os.ReadFile(path)
	Expect(err).ToNot(HaveOccurred(), "reading %s", path)
	return string(b)
}

// IsNotFoundError returns true if err is a Kubernetes not-found error.
func IsNotFoundError(err error) bool { return apierrors.IsNotFound(err) }

func freshNamespace(prefix string) string {
	ns := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	out, err := kubectl("create", "namespace", ns)
	Expect(err).ToNot(HaveOccurred(), "create ns: %s", out)
	return ns
}

func deleteNamespace(ns string) {
	_, _ = kubectl("delete", "namespace", ns, "--ignore-not-found", "--wait=false")
}

// addNamespace injects "  namespace: <ns>" under the metadata block of a
// single-document manifest so a fixture can be applied into a fresh namespace.
func addNamespace(yaml, ns string) string {
	lines := splitLines(yaml)
	out := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		out = append(out, line)
		if line == "metadata:" {
			out = append(out, "  namespace: "+ns)
		}
	}
	return strings.Join(out, "\n")
}

// extractName parses the `name:` field from the first metadata block.
func extractName(yaml string) string {
	inMeta := false
	for _, line := range splitLines(yaml) {
		if line == "metadata:" {
			inMeta = true
			continue
		}
		if inMeta {
			if strings.HasPrefix(line, "  name: ") {
				return strings.TrimPrefix(line, "  name: ")
			}
			if len(line) > 0 && line[0] != ' ' {
				inMeta = false
			}
		}
	}
	return ""
}

func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

// Quiet "imported and not used" warnings for helpers referenced from sibling
// test files that may be compiled independently.
var (
	_ = newKubeClient
	_ = kubectlApply
	_ = kubectlApplyServerSide
	_ = kubectlDelete
	_ = IsNotFoundError
	_ = readFile
	_ = waitForInstanceReady
	_ = waitForInstanceReconciled
	_ = statefulSetImageTag
	_ = waitForOwnedResources
	_ = forceRequeue
	_ = freshNamespace
	_ = deleteNamespace
	_ = captureFingerprint
	_ = expectFingerprintUnchanged
	_ = addNamespace
	_ = extractName
)
