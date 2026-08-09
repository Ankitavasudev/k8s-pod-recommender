package main

import (
	"testing"
)

func TestRecommend(t *testing.T) {
	pod := PodInfo{
		Name:      "test-pod",
		Namespace: "default",
		Status:    "Running",
		Restarts:  0,
		CPUReq:    "100m",
		MemReq:    "128Mi",
	}

	rec := recommend(pod)
	if rec.Pod != "test-pod" {
		t.Errorf("Expected test-pod, got %s", rec.Pod)
	}
	if rec.Reason != "No change needed" {
		t.Errorf("Expected No change needed, got %s", rec.Reason)
	}
}

func TestRecommendHighRestarts(t *testing.T) {
	pod := PodInfo{
		Name:      "crash-pod",
		Namespace: "default",
		Status:    "Running",
		Restarts:  10,
	}

	rec := recommend(pod)
	if rec.RecMem != "512Mi" {
		t.Errorf("Expected 512Mi, got %s", rec.RecMem)
	}
}

func TestRecommendNoResources(t *testing.T) {
	pod := PodInfo{
		Name:      "no-resources",
		Namespace: "default",
		Status:    "Running",
		Restarts:  0,
	}

	rec := recommend(pod)
	if rec.RecCPU != "100m" {
		t.Errorf("Expected 100m, got %s", rec.RecCPU)
	}
}

func TestRecommendMultipleRestarts(t *testing.T) {
	pod := PodInfo{
		Name:      "restart-pod",
		Namespace: "default",
		Status:    "Running",
		Restarts:  3,
	}

	rec := recommend(pod)
	if rec.Reason == "No change needed" {
		t.Error("Expected restart message")
	}
}
