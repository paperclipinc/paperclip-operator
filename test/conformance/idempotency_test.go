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
	"fmt"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// idempotencyCorpus maps a human-readable label to a testdata fixture. Each
// fixture is applied once, allowed to become Ready, then force-requeued
// repeatedly. After each requeue the resource fingerprint (generation +
// resourceVersion of owned objects) must be unchanged. A reconciler that
// rewrites owned objects on every pass (a bare r.Update instead of
// CreateOrUpdate) fails here.
var idempotencyCorpus = []struct {
	label   string
	fixture string
}{
	{"minimal", "minimal.yaml"},
	{"maximal", "maximal.yaml"},
	{"networking-ingress", "networking-ingress.yaml"},
	{"observability-full", "observability-full.yaml"},
}

const (
	idempotencyReconciles = 10
	idempotencyReadyWait  = 3 * time.Minute
	idempotencyPokeWait   = 15 * time.Second
)

var _ = Describe("idempotency canary", Ordered, func() {
	var ns string

	BeforeAll(func() {
		ns = freshNamespace("idempotency")
		DeferCleanup(func() { deleteNamespace(ns) })
	})

	for _, entry := range idempotencyCorpus {
		entry := entry

		Describe(fmt.Sprintf("corpus entry: %s", entry.label), Ordered, func() {
			var instName string
			var namespaced string

			BeforeAll(func() {
				yaml := readFile(filepath.Join("testdata", entry.fixture))
				namespaced = addNamespace(yaml, ns)
				out, err := kubectlApply(namespaced)
				Expect(err).ToNot(HaveOccurred(), "applying fixture %s: %s", entry.fixture, out)
				instName = extractName(yaml)
				Expect(instName).ToNot(BeEmpty(), "could not extract name from fixture %s", entry.fixture)
				DeferCleanup(func() { _, _ = kubectlDelete(namespaced) })
			})

			It("becomes Ready", func() {
				waitForInstanceReady(suiteCtx, newClient(), ns, instName, idempotencyReadyWait)
			})

			It(fmt.Sprintf("resource fingerprint is stable across %d reconciles", idempotencyReconciles), func() {
				cl := newClient()
				before := captureFingerprint(suiteCtx, cl, ns, instName)
				for i := 1; i < idempotencyReconciles; i++ {
					forceRequeue(suiteCtx, cl, ns, instName)
					time.Sleep(idempotencyPokeWait)
					after := captureFingerprint(suiteCtx, cl, ns, instName)
					expectFingerprintUnchanged(before, after)
					before = after
				}
			})
		})
	}
})
