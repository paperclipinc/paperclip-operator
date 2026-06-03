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
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// operatorNamespace is where the operator Deployment runs. Override with
// OPERATOR_NAMESPACE; defaults to the kubebuilder default.
func operatorNamespace() string {
	if v := os.Getenv("OPERATOR_NAMESPACE"); v != "" {
		return v
	}
	return "paperclip-operator-system"
}

// failure-injection: the operator pod is deleted (SIGKILL via pod delete) in
// the middle of managing an instance. The operator Deployment must restart and
// continue reconciling: the instance returns to Ready and a subsequent spec
// edit is still honored. This proves the controller holds no durable in-memory
// state across restarts.
var _ = Describe("failure injection (controller restart)", Ordered, func() {
	var (
		ns       string
		instName = "conf-minimal"
	)

	BeforeAll(func() {
		waitForOperatorAvailable()
		ns = freshNamespace("failure")
		DeferCleanup(func() { deleteNamespace(ns) })
	})

	It("recovers after the operator pod is killed", func() {
		cl := newClient()

		yaml := readFile(filepath.Join("testdata", "minimal.yaml"))
		out, err := kubectlApply(addNamespace(yaml, ns))
		Expect(err).ToNot(HaveOccurred(), "apply: %s", out)
		waitForInstanceReady(suiteCtx, cl, ns, instName, 3*time.Minute)

		By("killing the operator pod(s)")
		opNS := operatorNamespace()
		_, err = kubectl("delete", "pod", "-n", opNS,
			"-l", "control-plane=controller-manager", "--grace-period=0", "--force")
		Expect(err).ToNot(HaveOccurred())

		By("waiting for the operator Deployment to become available again")
		Eventually(func() error {
			_, e := kubectl("rollout", "status", "-n", opNS,
				"deployment", "-l", "control-plane=controller-manager", "--timeout=120s")
			return e
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		By("editing the instance after restart and expecting it to be reconciled")
		inst := &paperclipv1alpha1.Instance{}
		Expect(cl.Get(suiteCtx, types.NamespacedName{Namespace: ns, Name: instName}, inst)).To(Succeed())
		if inst.Labels == nil {
			inst.Labels = map[string]string{}
		}
		inst.Labels["conformance/post-restart"] = "true"
		Expect(cl.Update(suiteCtx, inst)).To(Succeed())

		waitForInstanceReady(suiteCtx, cl, ns, instName, 3*time.Minute)
	})
})
