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
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// GitOps coexistence: a GitOps controller (Flux/Argo) applies the Instance via
// server-side apply under its own field manager. The operator must reconcile
// owned objects without fighting the GitOps writer: re-applying the identical
// manifest via SSA must not flap the spec generation, and the instance must
// stay Ready.
var _ = Describe("GitOps coexistence (server-side apply)", Ordered, func() {
	var (
		ns         string
		instName   string
		namespaced string
	)

	BeforeAll(func() {
		waitForOperatorAvailable()
		ns = freshNamespace("gitops")
		yaml := readFile(filepath.Join("testdata", "minimal.yaml"))
		namespaced = addNamespace(yaml, ns)
		instName = extractName(yaml)
		out, err := kubectlApplyServerSide("flux", namespaced)
		Expect(err).ToNot(HaveOccurred(), "initial SSA apply: %s", out)
		DeferCleanup(func() {
			_, _ = kubectlDelete(namespaced)
			deleteNamespace(ns)
		})
	})

	It("is reconciled under a GitOps field manager", func() {
		waitForInstanceReconciled(suiteCtx, newClient(), ns, instName, 3*time.Minute)
	})

	It("does not flap when the same manifest is re-applied via SSA", func() {
		cl := newClient()
		before := captureFingerprint(suiteCtx, cl, ns, instName)
		for i := 0; i < 5; i++ {
			out, err := kubectlApplyServerSide("flux", namespaced)
			Expect(err).ToNot(HaveOccurred(), "re-apply via SSA: %s", out)
			time.Sleep(10 * time.Second)
		}
		after := captureFingerprint(suiteCtx, cl, ns, instName)
		expectFingerprintUnchanged(before, after)
	})
})
