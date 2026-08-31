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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	autoscalingv1alpha1 "scaler-controller/api/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

type fakeMetricsProvider struct {
	queueDepth int32
	gpuUtil    float64
}

func (f *fakeMetricsProvider) GetQueueDepth(ctx context.Context, targetDeployment string) (int32, error) {
	return f.queueDepth, nil
}

func (f *fakeMetricsProvider) GetGPUUtilization(ctx context.Context) (float64, error) {
	return f.gpuUtil, nil
}

var _ = Describe("ScalingPolicy Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-resource"
			resourceNamespace = "default"
			deploymentName    = "test-target-deployment"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		scalingpolicy := &autoscalingv1alpha1.ScalingPolicy{}

		BeforeEach(func() {
			By("creating the target Deployment")
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: resourceNamespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(10)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test-target"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "test-target"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "test", Image: "busybox"},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

			By("creating the custom resource for the Kind ScalingPolicy")
			err := k8sClient.Get(ctx, typeNamespacedName, scalingpolicy)
			if err != nil && errors.IsNotFound(err) {
				resource := &autoscalingv1alpha1.ScalingPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: autoscalingv1alpha1.ScalingPolicySpec{
						TargetDeployment: deploymentName,
						MinReplicas:      ptr.To(int32(1)),
						MaxReplicas:      5,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &autoscalingv1alpha1.ScalingPolicy{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance ScalingPolicy")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			By("cleanup the target Depployment")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: deploymentName, Namespace: resourceNamespace}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			Expect(k8sClient.Delete(ctx, deployment)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ScalingPolicyReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("checking the target Deployment's replica count was clamped to MaxReplicas")
			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: deploymentName, Namespace: resourceNamespace}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			Expect(*deployment.Spec.Replicas).To(Equal(int32(5)))

			By("checking the ScalingPolicy status reports Available=True")
			updatedPolicy := &autoscalingv1alpha1.ScalingPolicy{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updatedPolicy)).To(Succeed())
			Expect(apimeta.IsStatusConditionTrue(updatedPolicy.Status.Conditions, "Available")).To(BeTrue())
		})
	})

	Context("When reconciling with a queue-depth-driven policy", func() {
		const (
			metricsResourceName   = "test-resource-metrics"
			metricsDeploymentName = "test-target-deployment-metrics"
			resourceNamespace     = "default"
		)

		ctx := context.Background()
		metricsNamespacedName := types.NamespacedName{
			Name:      metricsResourceName,
			Namespace: resourceNamespace,
		}

		BeforeEach(func() {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      metricsDeploymentName,
					Namespace: resourceNamespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(2)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test-target-metrics"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "test-target-metrics"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "test", Image: "busybox"},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

			resource := &autoscalingv1alpha1.ScalingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      metricsResourceName,
					Namespace: resourceNamespace,
				},
				Spec: autoscalingv1alpha1.ScalingPolicySpec{
					TargetDeployment:    metricsDeploymentName,
					MinReplicas:         ptr.To(int32(1)),
					MaxReplicas:         10,
					QueueDepthThreshold: ptr.To(int32(10)),
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &autoscalingv1alpha1.ScalingPolicy{}
			Expect(k8sClient.Get(ctx, metricsNamespacedName, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: metricsDeploymentName, Namespace: resourceNamespace}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			Expect(k8sClient.Delete(ctx, deployment)).To(Succeed())
		})

		It("should scale propertionally to the observed queue deplth", func() {
			controllerReconciler := &ScalingPolicyReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				MetricsProvider: &fakeMetricsProvider{queueDepth: 30},
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: metricsNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: metricsDeploymentName, Namespace: resourceNamespace}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			// current = 2, threshold = 10, queueDepth = 30 -> ratio = 3 -> ceil = 6, whithin[1, 10]
			Expect(*deployment.Spec.Replicas).To(Equal(int32(6)))

			By("checking the ScalingPolicy status reports Available=True")
			updatedPolicy := &autoscalingv1alpha1.ScalingPolicy{}
			Expect(k8sClient.Get(ctx, metricsNamespacedName, updatedPolicy)).To(Succeed())
			Expect(apimeta.IsStatusConditionTrue(updatedPolicy.Status.Conditions, "Available")).To(BeTrue())
		})
	})

	Context("When reconciling with a GPU-utilization-driven policy", func() {
		const (
			gpuResourceName   = "test-resource-gpu"
			gpuDeploymentName = "test-target-deployment-gpu"
			resourceNamespace = "default"
		)

		ctx := context.Background()
		gpuNamespacedName := types.NamespacedName{
			Name:      gpuResourceName,
			Namespace: resourceNamespace,
		}

		BeforeEach(func() {
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gpuDeploymentName,
					Namespace: resourceNamespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To(int32(2)),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test-target-gpu"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "test-target-gpu"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "test", Image: "busybox"},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

			resource := &autoscalingv1alpha1.ScalingPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      gpuResourceName,
					Namespace: resourceNamespace,
				},
				Spec: autoscalingv1alpha1.ScalingPolicySpec{
					TargetDeployment:        gpuDeploymentName,
					MinReplicas:             ptr.To(int32(1)),
					MaxReplicas:             10,
					GPUUtilizationThreshold: ptr.To(int32(50)),
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		})

		AfterEach(func() {
			resource := &autoscalingv1alpha1.ScalingPolicy{}
			Expect(k8sClient.Get(ctx, gpuNamespacedName, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())

			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: gpuDeploymentName, Namespace: resourceNamespace}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			Expect(k8sClient.Delete(ctx, deployment)).To(Succeed())
		})

		It("should scale proportionally to fleet-average GPU utilization", func() {
			controllerReconciler := &ScalingPolicyReconciler{
				Client:          k8sClient,
				Scheme:          k8sClient.Scheme(),
				MetricsProvider: &fakeMetricsProvider{gpuUtil: 90},
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: gpuNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: gpuDeploymentName, Namespace: resourceNamespace}
			Expect(k8sClient.Get(ctx, deploymentKey, deployment)).To(Succeed())
			// current = 2, threshold = 50, gpuUtil = 90 -> ratio = 1.8 -> ceil(2*1.8) = 4, within [1, 10]
			Expect(*deployment.Spec.Replicas).To(Equal(int32(4)))
		})
	})
})
