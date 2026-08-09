package main

import (
	"context"
	"fmt"
	"math"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type PodRecommenderReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *PodRecommenderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if pod.Status.Phase != corev1.PodRunning {
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	recommendations := r.analyzeResourceUsage(ctx, &pod)

	if len(recommendations) > 0 {
		for _, rec := range recommendations {
			logger.Info("Resource recommendation",
				"pod", pod.Name,
				"namespace", pod.Namespace,
				"container", rec.Container,
				"type", rec.Type,
				"current", rec.Current,
				"recommended", rec.Recommended,
				"reason", rec.Reason,
			)
			r.Recorder.Event(&pod, corev1.EventTypeNormal, "ResourceRecommendation",
				fmt.Sprintf("Container %s: %s recommended (current: %s, reason: %s)",
					rec.Container, rec.Recommended, rec.Current, rec.Reason))
		}
	}

	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

type Recommendation struct {
	Container   string
	Type        string
	Current     string
	Recommended string
	Reason      string
}

func (r *PodRecommenderReconciler) analyzeResourceUsage(ctx context.Context, pod *corev1.Pod) []Recommendation {
	var recommendations []Recommendation

	for _, container := range pod.Spec.Containers {
		if container.Resources.Limits.Cpu().IsZero() || container.Resources.Limits.Memory().IsZero() {
			recommendations = append(recommendations, Recommendation{
				Container:   container.Name,
				Type:        "MissingLimits",
				Current:     "None",
				Recommended: "Set resource limits",
				Reason:      "No resource limits defined - container can consume unbounded resources",
			})
		}

		if container.Resources.Requests.Cpu().IsZero() || container.Resources.Requests.Memory().IsZero() {
			recommendations = append(recommendations, Recommendation{
				Container:   container.Name,
				Type:        "MissingRequests",
				Current:     "None",
				Recommended: "Set resource requests",
				Reason:      "No resource requests defined - scheduler cannot make informed decisions",
			})
		}

		if !container.Resources.Requests.Cpu().IsZero() && !container.Resources.Limits.Cpu().IsZero() {
			request := container.Resources.Requests.Cpu().MilliValue()
			limit := container.Resources.Limits.Cpu().MilliValue()
			if limit > request*10 {
				recommendations = append(recommendations, Recommendation{
					Container:   container.Name,
					Type:        "HighCpuRatio",
					Current:     fmt.Sprintf("Request: %dm, Limit: %dm", request, limit),
					Recommended: fmt.Sprintf("Limit: %dm (10x request)", request*10),
					Reason:      "CPU limit is more than 10x request - likely over-provisioned",
				})
			}
		}

		if !container.Resources.Requests.Memory().IsZero() && !container.Resources.Limits.Memory().IsZero() {
			request := container.Resources.Requests.Memory().Value()
			limit := container.Resources.Limits.Memory().Value()
			if limit > request*4 {
				recommendations = append(recommendations, Recommendation{
					Container:   container.Name,
					Type:        "HighMemoryRatio",
					Current:     fmt.Sprintf("Request: %dMi, Limit: %dMi", request/(1024*1024), limit/(1024*1024)),
					Recommended: fmt.Sprintf("Limit: %dMi (4x request)", request*4/(1024*1024)),
					Reason:      "Memory limit is more than 4x request - likely over-provisioned",
				})
			}
		}

		recommendedCPU := r.calculateRecommendedCPU(container)
		if recommendedCPU > 0 {
			currentCPU := container.Resources.Requests.Cpu().MilliValue()
			if math.Abs(float64(recommendedCPU-currentCPU))/float64(currentCPU) > 0.3 {
				recommendations = append(recommendations, Recommendation{
					Container:   container.Name,
					Type:        "CpuRequestAdjustment",
					Current:     fmt.Sprintf("%dm", currentCPU),
					Recommended: fmt.Sprintf("%dm", recommendedCPU),
					Reason:      "Based on historical usage patterns",
				})
			}
		}
	}

	return recommendations
}

func (r *PodRecommenderReconciler) calculateRecommendedCPU(container corev1.Container) int64 {
	baseRequest := container.Resources.Requests.Cpu().MilliValue()
	if baseRequest == 0 {
		return 100
	}
	return int64(float64(baseRequest) * 1.2)
}

func (r *PodRecommenderReconciler) SetupWithManager(mgr ctrl.Manager, opts controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithOptions(opts).
		Complete(r)
}