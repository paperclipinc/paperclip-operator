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
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Conformance suite for the Paperclip Instance CR. Categories live in sibling
// files:
//   - negative_test.go             CEL/schema deny paths on apply
//   - idempotency_test.go          10-reconcile no-op canary
//   - upgrade_test.go              prior-release -> HEAD smoke
//   - gitops_coexistence_test.go   server-side apply coexistence
//   - failure_modes_test.go        controller restart mid-reconcile
//
// The whole suite is gated on KUBECONFIG: it requires a live kind cluster with
// the operator installed and is skipped otherwise so it never breaks the unit
// or envtest jobs.
func TestConformance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "paperclip-operator conformance suite")
}

var (
	suiteCtx    context.Context
	suiteCancel context.CancelFunc
)

var _ = BeforeSuite(func() {
	suiteCtx, suiteCancel = context.WithCancel(context.Background())
	SetDefaultEventuallyTimeout(5 * time.Minute)
	SetDefaultEventuallyPollingInterval(2 * time.Second)
	if os.Getenv("KUBECONFIG") == "" {
		Skip("KUBECONFIG not set: conformance suite requires a live kind cluster with the operator installed")
	}
	// `make deploy` only applies manifests and returns immediately. Wait for
	// the controller-manager Deployment to be Available before any spec creates
	// an Instance, otherwise the first specs race the operator's image pull,
	// startup, and leader election and time out waiting for owned resources.
	// CI also waits via `kubectl rollout status`; this is defense-in-depth for
	// local runs and mirrors openclaw-operator's helm `--wait` install.
	// (operatorNamespace() lives in failure_modes_test.go, same package.)
	opNS := operatorNamespace()
	out, err := kubectl("wait", "--for=condition=Available",
		"deployment", "-n", opNS, "-l", "control-plane=controller-manager", "--timeout=5m")
	Expect(err).ToNot(HaveOccurred(),
		"operator Deployment in %s never became Available: %s", opNS, out)
})

var _ = AfterSuite(func() {
	if suiteCancel != nil {
		suiteCancel()
	}
})
