package main

import (
	"encoding/json"
	"errors"
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
