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
	// NOTE: do NOT wait for the operator Deployment here. The negative category
	// (negative_test.go) is API-server-only: it exercises CEL/structural-schema
	// denial with only the CRD installed and no controller-manager running, so a
	// global wait would hang that job. The operator-readiness gate lives in the
	// BeforeAll of each operator-dependent spec instead, via
	// waitForOperatorAvailable (see helpers.go).
})

var _ = AfterSuite(func() {
	if suiteCancel != nil {
		suiteCancel()
	}
})

// waitForOperatorAvailable blocks until the controller-manager Deployment is
// Available. `make deploy` only applies manifests and returns immediately; the
// controller-manager Pod still has to pull its image, start, and win leader
// election before it reconciles anything. Specs that create Instances must gate
// on this so they do not race the operator and time out waiting for owned
// resources. Mirrors openclaw-operator's `helm install --wait`.
//
// This is intentionally scoped to operator-dependent specs (called from their
// BeforeAll) rather than the global BeforeSuite: the negative category is
// API-server-only (CEL/schema denial with just the CRD installed, no operator)
// and must never block on the controller-manager. (operatorNamespace lives in
// failure_modes_test.go, same package.)
func waitForOperatorAvailable() {
	opNS := operatorNamespace()
	out, err := kubectl("wait", "--for=condition=Available",
		"deployment", "-n", opNS, "-l", "control-plane=controller-manager", "--timeout=5m")
	Expect(err).ToNot(HaveOccurred(),
		"operator Deployment in %s never became Available: %s", opNS, out)
}
