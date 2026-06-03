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

	"k8s.io/apimachinery/pkg/types"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
)

// upgrade-path matrix: an instance created with an older image tag is upgraded
// in place to a newer tag. The operator must roll the StatefulSet to the new
// image and the instance must return to Ready without losing its identity
// (same Service, same PVC).
var _ = Describe("upgrade-path matrix", Ordered, func() {
	var (
		ns       string
		instName = "conf-minimal"
	)

	BeforeAll(func() {
		ns = freshNamespace("upgrade")
		DeferCleanup(func() { deleteNamespace(ns) })
	})

	It("creates the instance on the baseline tag", func() {
		yaml := readFile(filepath.Join("testdata", "minimal.yaml"))
		out, err := kubectlApply(addNamespace(yaml, ns))
		Expect(err).ToNot(HaveOccurred(), "apply baseline: %s", out)
		waitForInstanceReady(suiteCtx, newClient(), ns, instName, 3*time.Minute)
	})

	It("upgrades the image tag in place and returns to Ready", func() {
		cl := newClient()

		pvcBefore := captureFingerprint(suiteCtx, cl, ns, instName).PVC

		inst := &paperclipv1alpha1.Instance{}
		Expect(cl.Get(suiteCtx, types.NamespacedName{Namespace: ns, Name: instName}, inst)).To(Succeed())
		inst.Spec.Image.Tag = "v1.0.1"
		Expect(cl.Update(suiteCtx, inst)).To(Succeed())

		// Allow the rollout to start, then wait for Ready again.
		time.Sleep(10 * time.Second)
		waitForInstanceReady(suiteCtx, cl, ns, instName, 3*time.Minute)

		// PVC identity must be preserved across the upgrade.
		pvcAfter := captureFingerprint(suiteCtx, cl, ns, instName).PVC
		Expect(pvcAfter.Generation).To(Equal(pvcBefore.Generation),
			"PVC generation changed across upgrade (data identity not preserved)")
	})
})
