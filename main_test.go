package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Ankitavasudev/k8s-pod-recommender/pkg/recommender"
)

const samplePods = `{
  "items": [
    {
      "metadata": {"name": "web-0", "namespace": "prod"},
      "status": {
        "phase": "Running",
        "containerStatuses": [{"restartCount": 2}, {"restartCount": 3}]
      },
      "spec": {
        "containers": [{
          "resources": {
            "requests": {"cpu": "250m", "memory": "128Mi"},
            "limits": {"cpu": "500m", "memory": "256Mi"}
          }
        }]
      }
    },
    {
      "metadata": {"name": "bare", "namespace": "default"},
      "status": {"phase": "Pending"},
      "spec": {"containers": [{"resources": {}}]}
    }
  ]
}`

func TestParsePods(t *testing.T) {
	pods, err := ParsePods([]byte(samplePods))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}

	first := pods[0]
	if first.Name != "web-0" || first.Namespace != "prod" || first.Status != "Running" {
		t.Errorf("unexpected first pod: %+v", first)
	}
	if first.Restarts != 5 {
		t.Errorf("restart counts should sum across containers, got %d", first.Restarts)
	}
	if first.CPUReq != "250m" || first.CPULim != "500m" || first.MemReq != "128Mi" || first.MemLim != "256Mi" {
		t.Errorf("resources not parsed: %+v", first)
	}

	second := pods[1]
	if second.CPUReq != "" || second.Restarts != 0 {
		t.Errorf("expected empty resources, got %+v", second)
	}
}

func TestParsePodsRejectsBadJSON(t *testing.T) {
	if _, err := ParsePods([]byte("not json")); err == nil {
		t.Fatal("expected an error")
	}
}

func TestParsePodsHandlesEmptyList(t *testing.T) {
	pods, err := ParsePods([]byte(`{"items": []}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(pods) != 0 {
		t.Fatalf("expected no pods, got %d", len(pods))
	}
}

func TestFetchPodsBuildsNamespaceFlags(t *testing.T) {
	var got []string
	run := func(args []string) ([]byte, error) {
		got = args
		return []byte(samplePods), nil
	}

	if _, err := fetchPods(run, "prod"); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "-n prod") {
		t.Errorf("expected -n prod, got %v", got)
	}

	if _, err := fetchPods(run, ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(got, " "), "--all-namespaces") {
		t.Errorf("expected --all-namespaces, got %v", got)
	}
}

func TestFetchPodsPropagatesKubectlErrors(t *testing.T) {
	run := func(args []string) ([]byte, error) { return nil, errors.New("no cluster") }
	if _, err := fetchPods(run, ""); err == nil || !strings.Contains(err.Error(), "kubectl failed") {
		t.Fatalf("expected kubectl failure, got %v", err)
	}
}

func TestRenderText(t *testing.T) {
	recs := []recommender.Recommendation{{
		Pod: "web-0", Namespace: "prod", Action: "keep",
		CurrentCPU: "250m", CurrentMem: "128Mi", RecCPU: "250m", RecMem: "128Mi",
		Reason: "No change needed",
	}}
	out, err := Render(recs, "text")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"prod/web-0", "250m", "No change needed", "keep"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestRenderTextShowsDashForMissingValues(t *testing.T) {
	out, _ := Render([]recommender.Recommendation{{Pod: "p", Namespace: "n"}}, "text")
	if !strings.Contains(out, "—") {
		t.Errorf("expected a dash placeholder:\n%s", out)
	}
}

func TestRenderJSON(t *testing.T) {
	recs := []recommender.Recommendation{{Pod: "p", Namespace: "n", Action: "set"}}
	out, err := Render(recs, "json")
	if err != nil {
		t.Fatal(err)
	}
	var decoded []recommender.Recommendation
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(decoded) != 1 || decoded[0].Pod != "p" {
		t.Errorf("unexpected payload: %+v", decoded)
	}
}

func TestRenderDefaultsToText(t *testing.T) {
	out, err := Render(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "K8s Pod Resource Recommendations") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestParsePodsSumsContainers(t *testing.T) {
	raw := []byte(`{"items": [{
	  "metadata": {"name": "api", "namespace": "prod"},
	  "status": {"phase": "Running", "containerStatuses": [{"restartCount": 1}, {"restartCount": 2}]},
	  "spec": {"containers": [
	    {"resources": {"requests": {"cpu": "250m", "memory": "256Mi"}, "limits": {"cpu": "1", "memory": "512Mi"}}},
	    {"resources": {"requests": {"cpu": "50m", "memory": "64Mi"}, "limits": {"cpu": "100m", "memory": "128Mi"}}}
	  ]}
	}]}`)

	pods, err := ParsePods(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(pods) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(pods))
	}
	pod := pods[0]
	if pod.Containers != 2 {
		t.Errorf("expected 2 containers, got %d", pod.Containers)
	}
	if pod.CPUReq != "300m" || pod.CPULim != "1.1" {
		t.Errorf("cpu was not summed: %+v", pod)
	}
	if pod.MemReq != "320Mi" || pod.MemLim != "640Mi" {
		t.Errorf("memory was not summed: %+v", pod)
	}
	if pod.Restarts != 3 {
		t.Errorf("restarts were not summed: %d", pod.Restarts)
	}
}

func TestParsePodsLeavesAbsentResourcesEmpty(t *testing.T) {
	pods, err := ParsePods([]byte(`{"items": [{
	  "metadata": {"name": "bare", "namespace": "default"},
	  "spec": {"containers": [{"resources": {}}]}
	}]}`))
	if err != nil {
		t.Fatal(err)
	}
	pod := pods[0]
	if pod.CPUReq != "" || pod.CPULim != "" || pod.MemReq != "" || pod.MemLim != "" {
		t.Errorf("missing resources should stay empty, got %+v", pod)
	}
}

func TestReadPodsFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pods.json")
	if err := os.WriteFile(path, []byte(samplePods), 0o600); err != nil {
		t.Fatal(err)
	}
	pods, err := readPods(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(pods) != 2 {
		t.Fatalf("expected 2 pods, got %d", len(pods))
	}
	if pods[0].Name != "web-0" {
		t.Errorf("unexpected first pod: %+v", pods[0])
	}
}

func TestReadPodsReportsMissingFile(t *testing.T) {
	if _, err := readPods(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestSortRecommendations(t *testing.T) {
	recs := []recommender.Recommendation{
		{Namespace: "prod", Pod: "web-1"},
		{Namespace: "default", Pod: "api"},
		{Namespace: "prod", Pod: "api"},
	}
	sortRecommendations(recs)
	got := []string{}
	for _, rec := range recs {
		got = append(got, rec.Namespace+"/"+rec.Pod)
	}
	want := []string{"default/api", "prod/api", "prod/web-1"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRenderHTML(t *testing.T) {
	recs := []recommender.Recommendation{
		{
			Pod: `<img src=x onerror=alert(1)>`, Namespace: "prod", Action: "set",
			CurrentCPU: "", CurrentMem: "", RecCPU: "100m", RecMem: "64Mi",
			Reason: "No resource requests",
		},
		{Pod: "ok", Namespace: "prod", Action: "keep", CurrentCPU: "100m", CurrentMem: "64Mi", RecCPU: "100m", RecMem: "64Mi"},
	}

	out, err := Render(recs, "html")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<!DOCTYPE html>",
		"<style>",
		"2 pods inspected",
		"&lt;img src=x onerror=alert(1)&gt;",
		"does not read live usage",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report is missing %q", want)
		}
	}
	if strings.Contains(out, "<script") {
		t.Error("report contains a script tag")
	}
	if strings.Contains(out, `<img src=x`) {
		t.Error("pod name was not escaped")
	}
	if strings.Contains(out, "<link") || strings.Contains(out, `src="http`) {
		t.Error("report loads an external resource")
	}
}
