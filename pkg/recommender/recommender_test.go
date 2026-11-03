package recommender

import (
	"math"
	"strings"
	"testing"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestCalculateRecommendedCPU(t *testing.T) {
	tests := []struct {
		name     string
		usage    float64
		expected float64
	}{
		{"zero usage", 0, 0.1},
		{"tiny usage", 0.05, 0.1},
		{"half a core", 0.5, 0.75},
		{"eighty percent", 0.8, 1.2},
		{"two cores", 2, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateRecommendedCPU(tt.usage); !almostEqual(got, tt.expected) {
				t.Errorf("CalculateRecommendedCPU(%v) = %v, want %v", tt.usage, got, tt.expected)
			}
		})
	}
}

func TestCalculateRecommendedCPUAppliesHeadroom(t *testing.T) {
	if CalculateRecommendedCPU(1) <= 1 {
		t.Error("expected headroom above observed usage")
	}
}

func TestCalculateRecommendedCPUAddsTenMilliResolution(t *testing.T) {
	got := CalculateRecommendedCPU(0.34)
	want := 0.51 // 0.34 * 1.5 = 0.51
	if !almostEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCalculateRecommendedMemory(t *testing.T) {
	tests := []struct {
		name     string
		usageMiB float64
		expected float64
	}{
		{"zero usage", 0, 64},
		{"small usage", 10, 64},
		{"hundred mebibytes", 100, 160},
		{"half a gibibyte", 512, 768},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateRecommendedMemory(tt.usageMiB); !almostEqual(got, tt.expected) {
				t.Errorf("CalculateRecommendedMemory(%v) = %v, want %v", tt.usageMiB, got, tt.expected)
			}
		})
	}
}

func TestParseCPU(t *testing.T) {
	cases := map[string]float64{
		"":      0,
		"100m":  0.1,
		"250m":  0.25,
		"1":     1,
		"0.5":   0.5,
		"2":     2,
		"500u":  0,
		"abc":   0,
		"1000m": 1,
	}
	for input, want := range cases {
		if got := ParseCPU(input); !almostEqual(got, want) {
			t.Errorf("ParseCPU(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestParseMemory(t *testing.T) {
	cases := map[string]float64{
		"":       0,
		"128Mi":  128,
		"1Gi":    1024,
		"512Ki":  0.5,
		"100M":   95.367431640625,
		"1024Mi": 1024,
		"bogus":  0,
	}
	for input, want := range cases {
		if got := ParseMemory(input); math.Abs(got-want) > 1e-6 {
			t.Errorf("ParseMemory(%q) = %v, want %v", input, got, want)
		}
	}
}

func TestFormatCPU(t *testing.T) {
	cases := map[float64]string{
		0:     "0",
		0.1:   "100m",
		0.25:  "250m",
		1:     "1",
		1.5:   "1.5",
		0.999: "999m",
	}
	for input, want := range cases {
		if got := FormatCPU(input); got != want {
			t.Errorf("FormatCPU(%v) = %q, want %q", input, got, want)
		}
	}
}

func TestFormatMemory(t *testing.T) {
	cases := map[float64]string{
		128:  "128Mi",
		1024: "1Gi",
		1536: "1536Mi",
		64:   "64Mi",
	}
	for input, want := range cases {
		if got := FormatMemory(input); got != want {
			t.Errorf("FormatMemory(%v) = %q, want %q", input, got, want)
		}
	}
}

func TestRecommendHealthyPod(t *testing.T) {
	rec := Recommend(Pod{
		Name: "ok", Namespace: "default", Status: "Running",
		CPUReq: "100m", CPULim: "200m", MemReq: "128Mi", MemLim: "256Mi",
	})
	if rec.Action != "keep" {
		t.Errorf("expected keep, got %s (%s)", rec.Action, rec.Reason)
	}
	if rec.RecCPU != "100m" || rec.RecMem != "128Mi" {
		t.Errorf("unexpected suggestion %s/%s", rec.RecCPU, rec.RecMem)
	}
}

func TestRecommendMissingRequests(t *testing.T) {
	rec := Recommend(Pod{Name: "bare", Namespace: "default", Status: "Running"})
	if rec.Action != "set" {
		t.Errorf("expected set, got %s", rec.Action)
	}
	if rec.RecCPU != "100m" || rec.RecMem != "64Mi" {
		t.Errorf("unexpected defaults %s/%s", rec.RecCPU, rec.RecMem)
	}
}

func TestRecommendRequestAboveLimit(t *testing.T) {
	rec := Recommend(Pod{Name: "broken", CPUReq: "2", CPULim: "1", MemReq: "64Mi", MemLim: "128Mi"})
	if rec.Action != "fix" {
		t.Errorf("expected fix, got %s", rec.Action)
	}
	if !strings.Contains(rec.Reason, "exceeds limit") {
		t.Errorf("unexpected reason: %s", rec.Reason)
	}
}

func TestRecommendCrashLoopDoesNotGuessMemory(t *testing.T) {
	rec := Recommend(Pod{
		Name: "crash", Status: "Running", Restarts: 12,
		CPUReq: "100m", CPULim: "200m", MemReq: "128Mi", MemLim: "256Mi",
	})
	if rec.Action != "investigate" {
		t.Errorf("expected investigate, got %s", rec.Action)
	}
	if rec.RecMem != "128Mi" {
		t.Errorf("crash loops must not silently change memory, got %s", rec.RecMem)
	}
}

func TestRecommendWideLimit(t *testing.T) {
	rec := Recommend(Pod{
		Name: "wide", Status: "Running",
		CPUReq: "100m", CPULim: "4", MemReq: "128Mi", MemLim: "256Mi",
	})
	if rec.Action != "tighten" {
		t.Errorf("expected tighten, got %s", rec.Action)
	}
	// Without usage data there is no defensible number to suggest, so the
	// request must be left alone rather than raised to the limit.
	if rec.RecCPU != "100m" {
		t.Errorf("tighten should not invent a request, got %s", rec.RecCPU)
	}
	if !strings.Contains(rec.Reason, "over-committed") {
		t.Errorf("unexpected reason: %s", rec.Reason)
	}
}

func TestRecommendMissingRequestsScalesWithContainers(t *testing.T) {
	tests := []struct {
		name       string
		containers int
		wantCPU    string
		wantMem    string
	}{
		{"single container", 1, "100m", "64Mi"},
		{"two containers", 2, "200m", "128Mi"},
		{"unset falls back to one", 0, "100m", "64Mi"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := Recommend(Pod{Name: "bare", Containers: tt.containers})
			if rec.Action != "set" {
				t.Fatalf("expected set, got %s", rec.Action)
			}
			if rec.RecCPU != tt.wantCPU || rec.RecMem != tt.wantMem {
				t.Errorf("got %s/%s, want %s/%s", rec.RecCPU, rec.RecMem, tt.wantCPU, tt.wantMem)
			}
		})
	}
}

func TestRecommendSingleRestart(t *testing.T) {
	rec := Recommend(Pod{
		Name: "blip", Status: "Running", Restarts: 2,
		CPUReq: "100m", MemReq: "128Mi",
	})
	if rec.Action != "watch" {
		t.Errorf("expected watch, got %s", rec.Action)
	}
}

func TestRecommendPreservesIdentity(t *testing.T) {
	rec := Recommend(Pod{Name: "p", Namespace: "ns", CPUReq: "100m", MemReq: "64Mi"})
	if rec.Pod != "p" || rec.Namespace != "ns" || rec.CurrentCPU != "100m" || rec.CurrentMem != "64Mi" {
		t.Errorf("recommendation lost pod identity: %+v", rec)
	}
}
