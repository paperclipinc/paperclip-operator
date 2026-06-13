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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	Context("When using the scale subresource", func() {
		ctx := context.Background()

		It("round-trips replicas through /scale and reports status.replicas and status.selector", func() {
			nn := types.NamespacedName{Name: "scale-subresource", Namespace: "default"}
			defer func() {
				resource := &paperclipv1alpha1.Instance{}
				if err := k8sClient.Get(ctx, nn, resource); err == nil {
					Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
					r := &InstanceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
					_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				}
			}()

			By("creating a Deployment-workload Instance")
			resource := &paperclipv1alpha1.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
				Spec: paperclipv1alpha1.InstanceSpec{
					Image:    paperclipv1alpha1.ImageSpec{Tag: "v1.0.0"},
					Workload: "Deployment",
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

			r := &InstanceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			reconcileN(ctx, r, nn, 2)

			By("reading the scale subresource")
			updated := &paperclipv1alpha1.Instance{}
			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			scale := &autoscalingv1.Scale{}
			Expect(k8sClient.SubResource("scale").Get(ctx, updated, scale)).To(Succeed())
			Expect(scale.Spec.Replicas).To(Equal(int32(1))) // defaulted spec.availability.replicas
			wantSelector := metav1.FormatLabelSelector(&metav1.LabelSelector{
				MatchLabels: resources.SelectorLabels(updated),
			})
			Expect(scale.Status.Selector).To(Equal(wantSelector))
			Expect(updated.Status.Selector).To(Equal(wantSelector))

			By("scaling to 3 replicas through the scale subresource")
			scale.Spec.Replicas = 3
			Expect(k8sClient.SubResource("scale").Update(ctx, updated, client.WithSubResourceBody(scale))).To(Succeed())

			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			Expect(updated.Spec.Availability.Replicas).NotTo(BeNil())
			Expect(*updated.Spec.Availability.Replicas).To(Equal(int32(3)))

			By("verifying the reconciled Deployment picks up the scaled replica count")
			reconcileN(ctx, r, nn, 1)
			deploy := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, nn, deploy)).To(Succeed())
			Expect(deploy.Spec.Replicas).NotTo(BeNil())
			Expect(*deploy.Spec.Replicas).To(Equal(int32(3)))

			By("verifying status.replicas mirrors the observed workload replicas")
			// envtest runs no Deployment controller, so simulate its status.
			deploy.Status.Replicas = 3
			Expect(k8sClient.Status().Update(ctx, deploy)).To(Succeed())
			reconcileN(ctx, r, nn, 1)

			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			Expect(updated.Status.Replicas).To(Equal(int32(3)))
			Expect(updated.Status.ManagedResources.Deployment).To(Equal(nn.Name))
			Expect(updated.Status.ManagedResources.StatefulSet).To(BeEmpty())
			Expect(k8sClient.SubResource("scale").Get(ctx, updated, scale)).To(Succeed())
			Expect(scale.Status.Replicas).To(Equal(int32(3)))
		})
	})

	Context("When checking multi-replica preconditions", func() {
		ctx := context.Background()

		It("tracks the MultiReplicaPreconditions condition across spec changes", func() {
			nn := types.NamespacedName{Name: "multireplica-preconditions", Namespace: "default"}
			defer func() {
				resource := &paperclipv1alpha1.Instance{}
				if err := k8sClient.Get(ctx, nn, resource); err == nil {
					Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
					r := &InstanceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
					_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				}
			}()

			By("creating an Instance with replicas=3 and an embedded database")
			resource := &paperclipv1alpha1.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
				Spec: paperclipv1alpha1.InstanceSpec{
					Image:        paperclipv1alpha1.ImageSpec{Tag: "v1.0.0"},
					Database:     paperclipv1alpha1.DatabaseSpec{Mode: "embedded"},
					Availability: paperclipv1alpha1.AvailabilitySpec{Replicas: resources.Ptr(int32(3))},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			r := &InstanceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			reconcileN(ctx, r, nn, 2)

			By("verifying MultiReplicaPreconditions=False naming both gaps")
			updated := &paperclipv1alpha1.Instance{}
			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			cond := meta.FindStatusCondition(updated.Status.Conditions, ConditionMultiReplicaPreconditions)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Message).To(ContainSubstring("embedded"))
			Expect(cond.Message).To(ContainSubstring("objectStorage"))

			By("switching to an external database with object storage")
			updated.Spec.Database = paperclipv1alpha1.DatabaseSpec{
				Mode:        "external",
				ExternalURL: "postgres://user:pass@db.example.com:5432/paperclip",
			}
			updated.Spec.ObjectStorage = &paperclipv1alpha1.ObjectStorageSpec{
				Provider: "s3",
				Bucket:   "paperclip-shared",
			}
			Expect(k8sClient.Update(ctx, updated)).To(Succeed())
			reconcileN(ctx, r, nn, 1)

			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			cond = meta.FindStatusCondition(updated.Status.Conditions, ConditionMultiReplicaPreconditions)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))

			By("scaling back to one replica removes the condition")
			updated.Spec.Availability.Replicas = resources.Ptr(int32(1))
			Expect(k8sClient.Update(ctx, updated)).To(Succeed())
			reconcileN(ctx, r, nn, 1)

			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			Expect(meta.FindStatusCondition(updated.Status.Conditions, ConditionMultiReplicaPreconditions)).To(BeNil())
		})
	})

	Context("When discovering the scheduler leader (lease gating)", func() {
		ctx := context.Background()
		nn := types.NamespacedName{Name: "leader-visibility", Namespace: "default"}

		// makeServerPod creates a server pod carrying the instance's selector
		// labels and forces it Running with the given pod IP (envtest runs no
		// kubelet, so status is set through the status subresource).
		makeServerPod := func(instance *paperclipv1alpha1.Instance, name, ip string) *corev1.Pod {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: nn.Namespace,
					Labels:    resources.SelectorLabels(instance),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "paperclip", Image: "ghcr.io/paperclipai/paperclip:v1.0.0"},
					},
				},
			}
			ExpectWithOffset(1, k8sClient.Create(ctx, pod)).To(Succeed())
			pod.Status.Phase = corev1.PodRunning
			pod.Status.PodIP = ip
			ExpectWithOffset(1, k8sClient.Status().Update(ctx, pod)).To(Succeed())
			return pod
		}

		AfterEach(func() {
			for _, name := range []string{"leader-visibility-pod-0", "leader-visibility-pod-1"} {
				pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nn.Namespace}}
				_ = k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0))
			}
			resource := &paperclipv1alpha1.Instance{}
			if err := k8sClient.Get(ctx, nn, resource); err == nil {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
				r := &InstanceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
		})

		It("labels the leader, sets status.schedulerLeader and deletion-cost, and clears on gating flip", func() {
			By("creating a lease-gated Deployment-workload Instance with replicas=2")
			resource := &paperclipv1alpha1.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
				Spec: paperclipv1alpha1.InstanceSpec{
					Image:    paperclipv1alpha1.ImageSpec{Tag: "v1.0.0"},
					Workload: "Deployment",
					Database: paperclipv1alpha1.DatabaseSpec{
						Mode:        "external",
						ExternalURL: "postgres://user:pass@db.example.com:5432/paperclip",
					},
					ObjectStorage: &paperclipv1alpha1.ObjectStorageSpec{
						Provider: "s3",
						Bucket:   "paperclip-shared",
					},
					Storage: paperclipv1alpha1.StorageSpec{
						Persistence: paperclipv1alpha1.PersistenceSpec{Enabled: resources.Ptr(false)},
					},
					Availability: paperclipv1alpha1.AvailabilitySpec{Replicas: resources.Ptr(int32(2))},
					Heartbeat:    paperclipv1alpha1.HeartbeatSpec{SchedulerGating: "lease"},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			By("creating two running server pods with pod IPs")
			pod0 := makeServerPod(resource, "leader-visibility-pod-0", "10.244.0.10")
			pod1 := makeServerPod(resource, "leader-visibility-pod-1", "10.244.0.11")

			By("reconciling with a fake health probe reporting pod-1 as leader")
			r := &InstanceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				healthProbe: func(podIP string, port int32) (schedulerHealth, error) {
					Expect(port).To(Equal(resources.DefaultPort))
					var h schedulerHealth
					h.Scheduler.Candidate = true
					h.Scheduler.IsLeader = podIP == "10.244.0.11"
					return h, nil
				},
			}
			reconcileN(ctx, r, nn, 2)
			result, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(leaderVisibilityRequeueInterval))

			By("verifying the role labels and the leader's deletion-cost annotation")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod1), pod1)).To(Succeed())
			Expect(pod1.Labels[LabelPodRole]).To(Equal(PodRoleScheduler))
			Expect(pod1.Annotations[AnnotationPodDeletionCost]).To(Equal(PodDeletionCostLeader))

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod0), pod0)).To(Succeed())
			Expect(pod0.Labels[LabelPodRole]).To(Equal(PodRoleWeb))
			Expect(pod0.Annotations).NotTo(HaveKey(AnnotationPodDeletionCost))

			By("verifying status.schedulerLeader names the leader pod")
			updated := &paperclipv1alpha1.Instance{}
			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			Expect(updated.Status.SchedulerLeader).To(Equal("leader-visibility-pod-1"))

			By("keeping the previous leader when every poll fails (no thrash)")
			r.healthProbe = func(string, int32) (schedulerHealth, error) {
				return schedulerHealth{}, fmt.Errorf("connection refused")
			}
			reconcileN(ctx, r, nn, 1)
			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			Expect(updated.Status.SchedulerLeader).To(Equal("leader-visibility-pod-1"))

			By("flipping gating back to ordinal clears status and strips pod markers")
			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			updated.Spec.Heartbeat.SchedulerGating = "ordinal"
			Expect(k8sClient.Update(ctx, updated)).To(Succeed())
			reconcileN(ctx, r, nn, 1)

			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			Expect(updated.Status.SchedulerLeader).To(BeEmpty())

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod1), pod1)).To(Succeed())
			Expect(pod1.Labels).NotTo(HaveKey(LabelPodRole))
			Expect(pod1.Annotations).NotTo(HaveKey(AnnotationPodDeletionCost))
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod0), pod0)).To(Succeed())
			Expect(pod0.Labels).NotTo(HaveKey(LabelPodRole))
		})
	})

	Context("When checking heartbeat scheduler gating validity", func() {
		ctx := context.Background()

		It("tracks the SchedulerGatingValid condition across spec changes", func() {
			nn := types.NamespacedName{Name: "scheduler-gating-validity", Namespace: "default"}
			defer func() {
				resource := &paperclipv1alpha1.Instance{}
				if err := k8sClient.Get(ctx, nn, resource); err == nil {
					Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
					r := &InstanceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
					_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				}
			}()

			By("creating a Deployment-workload Instance with replicas=3 and default (ordinal) gating")
			resource := &paperclipv1alpha1.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
				Spec: paperclipv1alpha1.InstanceSpec{
					Image:    paperclipv1alpha1.ImageSpec{Tag: "v1.0.0"},
					Workload: "Deployment",
					Database: paperclipv1alpha1.DatabaseSpec{
						Mode:        "external",
						ExternalURL: "postgres://user:pass@db.example.com:5432/paperclip",
					},
					ObjectStorage: &paperclipv1alpha1.ObjectStorageSpec{
						Provider: "s3",
						Bucket:   "paperclip-shared",
					},
					Storage: paperclipv1alpha1.StorageSpec{
						Persistence: paperclipv1alpha1.PersistenceSpec{Enabled: resources.Ptr(false)},
					},
					Availability: paperclipv1alpha1.AvailabilitySpec{Replicas: resources.Ptr(int32(3))},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())

			r := &InstanceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			reconcileN(ctx, r, nn, 2)

			By("verifying SchedulerGatingValid=False with reason OrdinalGatingRequiresStatefulSet")
			updated := &paperclipv1alpha1.Instance{}
			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			cond := meta.FindStatusCondition(updated.Status.Conditions, ConditionSchedulerGatingValid)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			Expect(cond.Reason).To(Equal("OrdinalGatingRequiresStatefulSet"))
			Expect(cond.Message).To(ContainSubstring("schedulerGating=lease"))

			By("switching to lease gating")
			updated.Spec.Heartbeat.SchedulerGating = "lease"
			Expect(k8sClient.Update(ctx, updated)).To(Succeed())
			reconcileN(ctx, r, nn, 1)

			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			cond = meta.FindStatusCondition(updated.Status.Conditions, ConditionSchedulerGatingValid)
			Expect(cond).NotTo(BeNil())
			Expect(cond.Status).To(Equal(metav1.ConditionTrue))

			By("scaling back to one replica removes the condition")
			updated.Spec.Availability.Replicas = resources.Ptr(int32(1))
			Expect(k8sClient.Update(ctx, updated)).To(Succeed())
			reconcileN(ctx, r, nn, 1)

			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			Expect(meta.FindStatusCondition(updated.Status.Conditions, ConditionSchedulerGatingValid)).To(BeNil())
		})
	})

	// Regression for issue #83: the bootstrap Job (spec.auth.adminUser) must be
	// reconciled idempotently. A Job's pod template is immutable, so re-rendering
	// or patching it on every reconcile makes the Job controller churn and kill
	// the running bootstrap pod (SuccessfulDelete ~1s after start ->
	// BackoffLimitExceeded). Steady-state reconciles must leave the Job entirely
	// untouched; only a real config change may replace it (delete + recreate).
	Context("When reconciling the admin bootstrap Job", func() {
		ctx := context.Background()

		newBootstrapInstance := func(name, email string) *paperclipv1alpha1.Instance {
			return &paperclipv1alpha1.Instance{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: paperclipv1alpha1.InstanceSpec{
					Image: paperclipv1alpha1.ImageSpec{Tag: "v1.0.0"},
					Auth: paperclipv1alpha1.AuthSpec{
						AdminUser: &paperclipv1alpha1.AdminUserSpec{
							Email: email,
							PasswordSecretRef: corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "admin-secret"},
								Key:                  "password",
							},
						},
					},
				},
			}
		}

		It("does not update or recreate the Job across repeated reconciles", func() {
			const bootName = "bootstrap-idem"
			nn := types.NamespacedName{Name: bootName, Namespace: "default"}
			jobNN := types.NamespacedName{Name: bootName + "-bootstrap", Namespace: "default"}

			Expect(k8sClient.Create(ctx, newBootstrapInstance(bootName, "admin@test.com"))).To(Succeed())
			DeferCleanup(func() {
				resource := &paperclipv1alpha1.Instance{}
				if err := k8sClient.Get(ctx, nn, resource); err == nil {
					_ = k8sClient.Delete(ctx, resource)
				}
			})
			r := &InstanceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}

			By("reconciling twice so the finalizer requeue passes and the Job is created")
			reconcileN(ctx, r, nn, 2)

			job := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, jobNN, job)).To(Succeed())
			originalUID := job.UID
			originalRV := job.ResourceVersion
			Expect(job.Annotations).To(HaveKey(resources.BootstrapHashAnnotation))

			By("asserting the Job has an explicit, unique selector so its pods cannot be adopted")
			Expect(job.Spec.ManualSelector).NotTo(BeNil())
			Expect(*job.Spec.ManualSelector).To(BeTrue())
			Expect(job.Spec.Selector).NotTo(BeNil())
			Expect(job.Spec.Selector.MatchLabels).To(HaveKeyWithValue(resources.BootstrapJobLabel, jobNN.Name))
			Expect(job.Spec.Template.Labels).To(HaveKeyWithValue(resources.BootstrapJobLabel, jobNN.Name))

			By("reconciling several more times")
			reconcileN(ctx, r, nn, 5)

			By("verifying the Job was neither updated (same resourceVersion) nor recreated (same UID)")
			after := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, jobNN, after)).To(Succeed())
			Expect(after.UID).To(Equal(originalUID), "bootstrap Job was recreated on a steady-state reconcile")
			Expect(after.ResourceVersion).To(Equal(originalRV), "bootstrap Job spec was mutated on a steady-state reconcile")
		})

		It("replaces the Job (delete + recreate) only when the bootstrap config changes", func() {
			const bootName = "bootstrap-replace"
			nn := types.NamespacedName{Name: bootName, Namespace: "default"}
			jobNN := types.NamespacedName{Name: bootName + "-bootstrap", Namespace: "default"}

			Expect(k8sClient.Create(ctx, newBootstrapInstance(bootName, "admin@test.com"))).To(Succeed())
			DeferCleanup(func() {
				resource := &paperclipv1alpha1.Instance{}
				if err := k8sClient.Get(ctx, nn, resource); err == nil {
					_ = k8sClient.Delete(ctx, resource)
				}
			})
			r := &InstanceReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			reconcileN(ctx, r, nn, 2)

			job := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, jobNN, job)).To(Succeed())
			originalUID := job.UID

			By("changing the admin email, which changes the bootstrap content hash")
			updated := &paperclipv1alpha1.Instance{}
			Expect(k8sClient.Get(ctx, nn, updated)).To(Succeed())
			updated.Spec.Auth.AdminUser.Email = "different@test.com"
			Expect(k8sClient.Update(ctx, updated)).To(Succeed())

			By("the next reconcile deletes the stale Job (template is immutable, cannot patch)")
			reconcileN(ctx, r, nn, 1)
			// Foreground deletion may leave the object briefly with a deletion
			// timestamp; remove any finalizers the envtest GC won't process.
			stale := &batchv1.Job{}
			if err := k8sClient.Get(ctx, jobNN, stale); err == nil && stale.DeletionTimestamp != nil {
				stale.Finalizers = nil
				_ = k8sClient.Update(ctx, stale)
			}
			Eventually(func() bool {
				return errors.IsNotFound(k8sClient.Get(ctx, jobNN, &batchv1.Job{}))
			}).Should(BeTrue(), "stale bootstrap Job should be deleted")

			By("a subsequent reconcile recreates the Job with a new UID")
			reconcileN(ctx, r, nn, 1)
			recreated := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, jobNN, recreated)).To(Succeed())
			Expect(recreated.UID).NotTo(Equal(originalUID), "Job should have been recreated, not patched in place")
		})
	})
})
