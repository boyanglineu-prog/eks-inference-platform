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
	"math"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	autoscalingv1alpha1 "scaler-controller/api/v1alpha1"
)

type MetricsProvider interface {
	GetQueueDepth(ctx context.Context, targetDeployment string) (int32, error)
	// GetGPUUtilization returns fleet-average GPU utilization (0-100), not
	// scoped to a single Deployment -- GPU utilization is a physical-device
	// metric with no inherent per-Deployment attribution, unlike queue depth
	// (which vLLM reports per-instance itself).
	GetGPUUtilization(ctx context.Context) (float64, error)
}

// ScalingPolicyReconciler reconciles a ScalingPolicy object
type ScalingPolicyReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	MetricsProvider MetricsProvider
}

// +kubebuilder:rbac:groups=autoscaling.example.com,resources=scalingpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling.example.com,resources=scalingpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=autoscaling.example.com,resources=scalingpolicies/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ScalingPolicy object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *ScalingPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var policy autoscalingv1alpha1.ScalingPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	var deploy appsv1.Deployment
	deployKey := client.ObjectKey{Namespace: policy.Namespace, Name: policy.Spec.TargetDeployment}
	if err := r.Get(ctx, deployKey, &deploy); err != nil {
		log.Error(err, "target deployment not found", "deployment", policy.Spec.TargetDeployment)

		apimeta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
			Type:    "Available",
			Status:  metav1.ConditionFalse,
			Reason:  "DeploymentNotFound",
			Message: err.Error(),
		})
		if statusErr := r.Status().Update(ctx, &policy); statusErr != nil {
			log.Error(statusErr, "failed to update ScalingPolicy status")
		}

		return ctrl.Result{}, err
	}

	var current int32
	if deploy.Spec.Replicas != nil {
		current = *deploy.Spec.Replicas
	}

	min := int32(1)
	if policy.Spec.MinReplicas != nil {
		min = *policy.Spec.MinReplicas
	}
	max := policy.Spec.MaxReplicas

	desired := current
	if policy.Spec.QueueDepthThreshold != nil && *policy.Spec.QueueDepthThreshold > 0 && r.MetricsProvider != nil {
		queueDepth, err := r.MetricsProvider.GetQueueDepth(ctx, policy.Spec.TargetDeployment)
		if err != nil {
			log.Error(err, "failed to read queue depth metric")
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		}

		base := current
		if base < 1 {
			base = 1
		}
		threshold := float64(*policy.Spec.QueueDepthThreshold)
		ratio := float64(queueDepth) / threshold
		desired = int32(math.Ceil(float64(base) * ratio))
	}

	if policy.Spec.GPUUtilizationThreshold != nil && *policy.Spec.GPUUtilizationThreshold > 0 && r.MetricsProvider != nil {
		gpuUtil, err := r.MetricsProvider.GetGPUUtilization(ctx)
		if err != nil {
			log.Error(err, "failed to read GPU utilization metric")
			return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
		}

		base := current
		if base < 1 {
			base = 1
		}
		threshold := float64(*policy.Spec.GPUUtilizationThreshold)
		ratio := gpuUtil / threshold
		desiredFromGPU := int32(math.Ceil(float64(base) * ratio))

		// Take the max across metrics -- same "most-constrained-metric wins"
		// semantics real HPA uses when multiple metrics are configured.
		if desiredFromGPU > desired {
			desired = desiredFromGPU
		}
	}

	if desired < min {
		desired = min
	}

	if desired > max {
		desired = max
	}

	if desired != current {
		deploy.Spec.Replicas = &desired
		if err := r.Update(ctx, &deploy); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("adjusetd replicas count", "from", current, "to", desired)
	}

	apimeta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
		Type:    "Available",
		Status:  metav1.ConditionTrue,
		Reason:  "ReconcileSuccess",
		Message: fmt.Sprintf("target deployment %q at %d replicas (desired %d)", policy.Spec.TargetDeployment, current, desired),
	})
	if err := r.Status().Update(ctx, &policy); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScalingPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&autoscalingv1alpha1.ScalingPolicy{}).
		Named("scalingpolicy").
		Complete(r)
}
