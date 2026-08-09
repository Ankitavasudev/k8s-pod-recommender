package main

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAnalyzeMissingLimits(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "nginx"},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()
	recorder := record.NewFakeRecorder(10)

	r := &PodRecommenderReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: recorder,
	}

	recs := r.analyzeResourceUsage(context.Background(), pod)

	missingLimits := false
	for _, rec := range recs {
		if rec.Type == "MissingLimits" {
			missingLimits = true
			break
		}
	}
	if !missingLimits {
		t.Error("Expected MissingLimits recommendation for container without limits")
	}
}

func TestAnalyzeHighCpuRatio(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "nginx",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("100m"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("2000m"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()
	recorder := record.NewFakeRecorder(10)

	r := &PodRecommenderReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: recorder,
	}

	recs := r.analyzeResourceUsage(context.Background(), pod)

	highRatio := false
	for _, rec := range recs {
		if rec.Type == "HighCpuRatio" {
			highRatio = true
			break
		}
	}
	if !highRatio {
		t.Error("Expected HighCpuRatio recommendation for container with high CPU ratio")
	}
}

func TestAnalyzeHighMemoryRatio(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "nginx",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()
	recorder := record.NewFakeRecorder(10)

	r := &PodRecommenderReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: recorder,
	}

	recs := r.analyzeResourceUsage(context.Background(), pod)

	highMem := false
	for _, rec := range recs {
		if rec.Type == "HighMemoryRatio" {
			highMem = true
			break
		}
	}
	if !highMem {
		t.Error("Expected HighMemoryRatio recommendation")
	}
}

func TestAnalyzeWellConfigured(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "nginx",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pod).Build()
	recorder := record.NewFakeRecorder(10)

	r := &PodRecommenderReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: recorder,
	}

	recs := r.analyzeResourceUsage(context.Background(), pod)
	if len(recs) > 0 {
		t.Errorf("Expected no recommendations for well-configured pod, got %d", len(recs))
	}
}

func TestCalculateRecommendedCPU(t *testing.T) {
	scheme := runtime.NewScheme()
	r := &PodRecommenderReconciler{Scheme: scheme}

	container := corev1.Container{
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
		},
	}

	rec := r.calculateRecommendedCPU(container)
	if rec != 120 {
		t.Errorf("Expected 120m, got %d", rec)
	}
}

func TestReconcilePodNotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	recorder := record.NewFakeRecorder(10)

	r := &PodRecommenderReconciler{
		Client:   fakeClient,
		Scheme:   scheme,
		Recorder: recorder,
	}

	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: "nonexistent", Namespace: "default"},
	})

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result.RequeueAfter != 0 {
		t.Error("Expected no requeue for not found pod")
	}
}