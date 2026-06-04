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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// negativeCase describes one schema/CEL deny scenario exercised via kubectl apply.
// Paperclip enforces validation through CEL rules and enum/pattern constraints
// baked into the CRD (there is no admission webhook), so these run on a plain
// kind cluster with only the CRD installed.
type negativeCase struct {
	name             string
	yaml             string
	wantErrSubstring string
}

var negativeCases = []negativeCase{
	{
		name: "deny: image with neither tag nor digest",
		yaml: `apiVersion: paperclip.inc/v1alpha1
kind: Instance
metadata:
  name: neg-no-tag-digest
spec:
  image:
    repository: ghcr.io/paperclipai/paperclip
`,
		wantErrSubstring: "one of tag or digest must be set",
	},
	{
		name: "deny: invalid deployment.mode enum",
		yaml: `apiVersion: paperclip.inc/v1alpha1
kind: Instance
metadata:
  name: neg-bad-mode
spec:
  image:
    tag: v1.0.0
  deployment:
    mode: bogus
`,
		wantErrSubstring: "deployment.mode",
	},
	{
		name: "deny: invalid database.mode enum",
		yaml: `apiVersion: paperclip.inc/v1alpha1
kind: Instance
metadata:
  name: neg-bad-db-mode
spec:
  image:
    tag: v1.0.0
  database:
    mode: nosql
`,
		wantErrSubstring: "database.mode",
	},
	{
		name: "deny: replicas below minimum",
		yaml: `apiVersion: paperclip.inc/v1alpha1
kind: Instance
metadata:
  name: neg-zero-replicas
spec:
  image:
    tag: v1.0.0
  availability:
    replicas: 0
`,
		wantErrSubstring: "availability.replicas",
	},
}

var _ = Describe("negative (schema and CEL deny paths)", Ordered, func() {
	var ns string

	BeforeAll(func() {
		ns = freshNamespace("negative")
		DeferCleanup(func() { deleteNamespace(ns) })
	})

	for _, tc := range negativeCases {
		tc := tc
		It(tc.name, func() {
			out, err := kubectlApply(addNamespace(tc.yaml, ns))
			Expect(err).To(HaveOccurred(), "expected apply to be denied, got success: %s", out)
			Expect(strings.ToLower(out)).To(ContainSubstring(strings.ToLower(tc.wantErrSubstring)),
				"error output did not mention %q: %s", tc.wantErrSubstring, out)
		})
	}
})
