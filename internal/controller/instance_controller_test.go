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

package controller

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	paperclipv1alpha1 "github.com/paperclipinc/paperclip-operator/api/v1alpha1"
	"github.com/paperclipinc/paperclip-operator/internal/resources"
)

// reconcileN reconciles the instance n times. The first reconcile of a fresh
// Instance only adds the finalizer and requeues, so tests reconcile at least
// twice before asserting on built resources.
func reconcileN(ctx context.Context, r *InstanceReconciler, nn types.NamespacedName, n int) {
	for i := 0; i < n; i++ {
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
	}
}

// recordedEvent drains the fake recorder and reports whether any event
// contains the given substring.
func recordedEvent(rec *record.FakeRecorder, substr string) bool {
	for {
		select {
		case e := <-rec.Events:
			if strings.Contains(e, substr) {
				return true
			}
		default:
			return false
		}
	}
}

var _ = Describe("Instance Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		instance := &paperclipv1alpha1.Instance{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Instance")
			err := k8sClient.Get(ctx, typeNamespacedName, instance)
			if err != nil && errors.IsNotFound(err) {
				resource := &paperclipv1alpha1.Instance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: paperclipv1alpha1.InstanceSpec{
						Image: paperclipv1alpha1.ImageSpec{
							Tag: "v1.0.0",
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &paperclipv1alpha1.Instance{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Instance")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &InstanceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	Context("When the instance is suspended", func() {
		const suspendedName = "suspended-resource"

		ctx := context.Background()
		nn := types.NamespacedName{Name: suspendedName, Namespace: "default"}

		AfterEach(func() {
			resource := &paperclipv1alpha1.Instance{}
			if err := k8sClient.Get(ctx, nn, resource); err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
		})

		It("scales the StatefulSet to zero and reports the Suspended phase", func() {
			By("creating a suspended Instance with replicas=3")
			resource := &paperclipv1alpha1.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: suspendedName, Namespace: "default"},
				Spec: paperclipv1alpha1.InstanceSpec{
					Image:        paperclipv1alpha1.ImageSpec{Tag: "v1.0.0"},
					Suspended:    true,
					Availability: paperclipv1alpha1.AvailabilitySpec{Replicas: resources.Ptr(int32(3))},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			controllerReconciler := &InstanceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("reconciling twice to pass the finalizer requeue")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			By("verifying the StatefulSet has 0 replicas")
			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
			Expect(sts.Spec.Replicas).NotTo(BeNil())
			Expect(*sts.Spec.Replicas).To(Equal(int32(0)))

			By("verifying the Suspended phase and condition")
			updated := &paperclipv1alpha1.Instance{}
			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			Expect(updated.Status.Phase).To(Equal(paperclipv1alpha1.PhaseSuspended))
			cond := meta.FindStatusCondition(updated.Status.Conditions, ConditionSuspended)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
		})
	})

	Context("When selecting the server workload kind", func() {
		ctx := context.Background()

		cleanup := func(nn types.NamespacedName) {
			resource := &paperclipv1alpha1.Instance{}
			if err := k8sClient.Get(ctx, nn, resource); err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
				// Run the finalizer so the object is actually removed.
				r := &InstanceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
		}

		It("builds a Deployment (and no StatefulSet) for auto workload without persistence, and points the HPA at it", func() {
			nn := types.NamespacedName{Name: "auto-workload", Namespace: "default"}
			defer cleanup(nn)

			By("creating an auto-workload Instance with an external database and HPA enabled")
			resource := &paperclipv1alpha1.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
				Spec: paperclipv1alpha1.InstanceSpec{
					Image:    paperclipv1alpha1.ImageSpec{Tag: "v1.0.0"},
					Workload: "auto",
					Database: paperclipv1alpha1.DatabaseSpec{
						Mode:        "external",
						ExternalURL: "postgres://user:pass@db.example.com:5432/paperclip",
					},
					Availability: paperclipv1alpha1.AvailabilitySpec{
						AutoScaling: &paperclipv1alpha1.AutoScalingSpec{Enabled: true, MaxReplicas: 3},
					},
					Storage: paperclipv1alpha1.StorageSpec{
						Persistence: paperclipv1alpha1.PersistenceSpec{Enabled: resources.Ptr(false)},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			r := &InstanceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			reconcileN(ctx, r, nn, 2)

			By("verifying a Deployment exists and no StatefulSet")
			deploy := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, nn, deploy)).To(Succeed())
			sts := &appsv1.StatefulSet{}
			Expect(errors.IsNotFound(k8sClient.Get(ctx, nn, sts))).To(BeTrue())

			By("verifying the HPA targets the Deployment")
			updated := &paperclipv1alpha1.Instance{}
			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			hpa := &autoscalingv2.HorizontalPodAutoscaler{}
			hpaNN := types.NamespacedName{Name: resources.HPAName(updated), Namespace: nn.Namespace}
			Expect(k8sClient.Get(ctx, hpaNN, hpa)).To(Succeed())
			Expect(hpa.Spec.ScaleTargetRef.Kind).To(Equal("Deployment"))
			Expect(hpa.Spec.ScaleTargetRef.Name).To(Equal(nn.Name))

			By("verifying the workload profile is reported valid")
			cond := meta.FindStatusCondition(updated.Status.Conditions, ConditionWorkloadProfileValid)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))
		})

		It("keeps building a StatefulSet for a default instance (regression)", func() {
			nn := types.NamespacedName{Name: "default-workload", Namespace: "default"}
			defer cleanup(nn)

			resource := &paperclipv1alpha1.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
				Spec: paperclipv1alpha1.InstanceSpec{
					Image: paperclipv1alpha1.ImageSpec{Tag: "v1.0.0"},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			r := &InstanceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			reconcileN(ctx, r, nn, 2)

			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
			deploy := &appsv1.Deployment{}
			Expect(errors.IsNotFound(k8sClient.Get(ctx, nn, deploy))).To(BeTrue())
		})

		It("migrates the workload kind when spec.workload flips StatefulSet -> Deployment", func() {
			nn := types.NamespacedName{Name: "flip-workload", Namespace: "default"}
			defer cleanup(nn)

			By("creating a StatefulSet-workload Instance without persistence")
			resource := &paperclipv1alpha1.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
				Spec: paperclipv1alpha1.InstanceSpec{
					Image:    paperclipv1alpha1.ImageSpec{Tag: "v1.0.0"},
					Workload: "StatefulSet",
					Database: paperclipv1alpha1.DatabaseSpec{
						Mode:        "external",
						ExternalURL: "postgres://user:pass@db.example.com:5432/paperclip",
					},
					Storage: paperclipv1alpha1.StorageSpec{
						Persistence: paperclipv1alpha1.PersistenceSpec{Enabled: resources.Ptr(false)},
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			rec := record.NewFakeRecorder(128)
			r := &InstanceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Recorder: rec}
			reconcileN(ctx, r, nn, 2)

			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())

			By("flipping spec.workload to Deployment")
			updated := &paperclipv1alpha1.Instance{}
			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			updated.Spec.Workload = "Deployment"
			Expect(k8sClient.Update(ctx, updated)).To(Succeed())
			reconcileN(ctx, r, nn, 1)

			By("verifying the Deployment replaced the StatefulSet")
			deploy := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, nn, deploy)).To(Succeed())
			Expect(errors.IsNotFound(k8sClient.Get(ctx, nn, sts))).To(BeTrue())
			Expect(recordedEvent(rec, "WorkloadKindMigrated")).To(BeTrue())
		})

		It("keeps the StatefulSet and reports WorkloadProfileValid=False for workload=Deployment with persistence", func() {
			nn := types.NamespacedName{Name: "pvc-safety", Namespace: "default"}
			defer cleanup(nn)

			resource := &paperclipv1alpha1.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
				Spec: paperclipv1alpha1.InstanceSpec{
					Image:    paperclipv1alpha1.ImageSpec{Tag: "v1.0.0"},
					Workload: "Deployment",
					// persistence stays at its default (enabled)
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			r := &InstanceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			reconcileN(ctx, r, nn, 2)

			By("verifying the StatefulSet is still built and no Deployment exists")
			sts := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, nn, sts)).To(Succeed())
			deploy := &appsv1.Deployment{}
			Expect(errors.IsNotFound(k8sClient.Get(ctx, nn, deploy))).To(BeTrue())

			By("verifying the WorkloadProfileValid condition")
			updated := &paperclipv1alpha1.Instance{}
			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			cond := meta.FindStatusCondition(updated.Status.Conditions, ConditionWorkloadProfileValid)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal("PersistenceRequiresStatefulSet"))
			Expect(cond.Message).To(ContainSubstring("persistence"))
		})
	})
})
