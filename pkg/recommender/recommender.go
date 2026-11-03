// Package recommender turns pod specs into resource suggestions.
//
// The rules here are deliberately conservative: without a metrics server the
// tool cannot know real usage, so it only fixes what is provably wrong
// (missing requests, requests above limits, limits far above requests) and
// flags the rest for a human.
package recommender

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Pod is the subset of a pod spec the recommender needs. The resource fields
// hold the sum across the pod's containers, because that is what the scheduler
// reserves for the pod.
type Pod struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Status     string `json:"status"`
	Restarts   int    `json:"restarts"`
	Containers int    `json:"containers"`
	CPUReq     string `json:"cpu_request"`
	CPULim     string `json:"cpu_limit"`
	MemReq     string `json:"memory_request"`
	MemLim     string `json:"memory_limit"`
}

// Recommendation is one pod's suggested change.
type Recommendation struct {
	Pod        string `json:"pod"`
	Namespace  string `json:"namespace"`
	CurrentCPU string `json:"current_cpu"`
	CurrentMem string `json:"current_memory"`
	RecCPU     string `json:"recommended_cpu"`
	RecMem     string `json:"recommended_memory"`
	Action     string `json:"action"`
	Reason     string `json:"reason"`
}

// Headroom applied when sizing from observed usage.
const headroom = 1.5

// Minimum values the tool will ever suggest.
const (
	minCPU    = 0.1  // 100m
	minMemMiB = 64.0 // 64Mi
)

// CalculateRecommendedCPU returns a CPU request for the given usage in cores.
// It applies 50% headroom, rounds up to 10m, and never goes below 100m.
func CalculateRecommendedCPU(usage float64) float64 {
	if usage <= 0 {
		return minCPU
	}
	target := roundUp(usage*headroom, 0.01)
	if target < minCPU {
		return minCPU
	}
	return target
}

// CalculateRecommendedMemory is the same rule in MiB.
func CalculateRecommendedMemory(usageMiB float64) float64 {
	if usageMiB <= 0 {
		return minMemMiB
	}
	target := roundUp(usageMiB*headroom, 16)
	if target < minMemMiB {
		return minMemMiB
	}
	return target
}

func roundUp(value, step float64) float64 {
	// The epsilon keeps 0.8*1.5 = 1.2000000000000002 from rounding up to 1.21.
	return math.Ceil(value/step-1e-9) * step
}

// ParseCPU converts a Kubernetes CPU quantity ("100m", "0.5", "2") to cores.
// Unparseable values return 0.
func ParseCPU(quantity string) float64 {
	q := strings.TrimSpace(quantity)
	if q == "" {
		return 0
	}
	if strings.HasSuffix(q, "m") {
		milli, err := strconv.ParseFloat(strings.TrimSuffix(q, "m"), 64)
		if err != nil {
			return 0
		}
		return milli / 1000
	}
	if strings.HasSuffix(q, "n") {
		nano, err := strconv.ParseFloat(strings.TrimSuffix(q, "n"), 64)
		if err != nil {
			return 0
		}
		return nano / 1e9
	}
	cores, err := strconv.ParseFloat(q, 64)
	if err != nil {
		return 0
	}
	return cores
}

var memoryUnits = []struct {
	suffix string
	factor float64 // to MiB
}{
	{"Ki", 1.0 / 1024},
	{"Mi", 1},
	{"Gi", 1024},
	{"Ti", 1024 * 1024},
	{"K", 1000.0 / (1024 * 1024)},
	{"M", 1e6 / (1024 * 1024)},
	{"G", 1e9 / (1024 * 1024)},
}

// ParseMemory converts a Kubernetes memory quantity to MiB. Unparseable
// values return 0.
func ParseMemory(quantity string) float64 {
	q := strings.TrimSpace(quantity)
	if q == "" {
		return 0
	}
	for _, unit := range memoryUnits {
		if strings.HasSuffix(q, unit.suffix) {
			value, err := strconv.ParseFloat(strings.TrimSuffix(q, unit.suffix), 64)
			if err != nil {
				return 0
			}
			return value * unit.factor
		}
	}
	bytes, err := strconv.ParseFloat(q, 64)
	if err != nil {
		return 0
	}
	return bytes / (1024 * 1024)
}

// FormatCPU renders cores the way Kubernetes does: "250m", "1".
func FormatCPU(cores float64) string {
	if cores == 0 {
		return "0"
	}
	round := math.Round(cores*1000) / 1000
	if round < 1 {
		return fmt.Sprintf("%dm", int(math.Round(round*1000)))
	}
	if round == math.Trunc(round) {
		return strconv.Itoa(int(round))
	}
	return strconv.FormatFloat(round, 'f', -1, 64)
}

// FormatMemory renders MiB as "128Mi", or GiB when it divides evenly.
func FormatMemory(mib float64) string {
	if mib >= 1024 && math.Mod(mib, 1024) == 0 {
		return fmt.Sprintf("%dGi", int(mib/1024))
	}
	return fmt.Sprintf("%dMi", int(math.Round(mib)))
}

// Recommend inspects one pod and returns a suggestion.
//
// Order matters: correctness problems (request above limit) are reported first,
// then missing values, then crash loops, and only then a no-op verdict.
func Recommend(pod Pod) Recommendation {
	containers := pod.Containers
	if containers < 1 {
		containers = 1
	}

	rec := Recommendation{
		Pod:        pod.Name,
		Namespace:  pod.Namespace,
		CurrentCPU: pod.CPUReq,
		CurrentMem: pod.MemReq,
		RecCPU:     pod.CPUReq,
		RecMem:     pod.MemReq,
		Action:     "keep",
		Reason:     "No change needed",
	}

	cpuReq, cpuLim := ParseCPU(pod.CPUReq), ParseCPU(pod.CPULim)
	memReq, memLim := ParseMemory(pod.MemReq), ParseMemory(pod.MemLim)

	if cpuReq > 0 && cpuLim > 0 && cpuReq > cpuLim {
		rec.Action = "fix"
		rec.Reason = fmt.Sprintf("CPU request (%s) exceeds limit (%s) — the pod cannot schedule", pod.CPUReq, pod.CPULim)
		return rec
	}
	if memReq > 0 && memLim > 0 && memReq > memLim {
		rec.Action = "fix"
		rec.Reason = fmt.Sprintf("Memory request (%s) exceeds limit (%s)", pod.MemReq, pod.MemLim)
		return rec
	}

	if cpuReq == 0 && memReq == 0 {
		rec.Action = "set"
		rec.RecCPU = FormatCPU(minCPU * float64(containers))
		rec.RecMem = FormatMemory(minMemMiB * float64(containers))
		rec.Reason = fmt.Sprintf("No resource requests on %s — the scheduler cannot place this pod fairly", containerCount(containers))
		return rec
	}

	if pod.Restarts > 5 {
		rec.Action = "investigate"
		rec.Reason = fmt.Sprintf("%d restarts — check OOMKilled/evicted events before raising limits", pod.Restarts)
		return rec
	}

	if cpuLim > 0 && cpuReq > 0 && cpuLim > cpuReq*4 {
		// No usage data is available here, so this deliberately suggests no
		// number. Picking one from the limit would be a guess, and for a pod
		// with a large burst limit that guess could be far above what the pod
		// ever uses.
		rec.Action = "tighten"
		rec.Reason = fmt.Sprintf("CPU limit (%s) is %.0fx the request (%s) — the scheduler reserves only the request, so this pod is over-committed and evicted first; set the request from measured usage or lower the limit", pod.CPULim, cpuLim/cpuReq, pod.CPUReq)
		return rec
	}

	if pod.Restarts > 0 {
		rec.Action = "watch"
		rec.Reason = fmt.Sprintf("%d restart(s) — not enough evidence to change resources", pod.Restarts)
	}

	return rec
}

func containerCount(n int) string {
	if n == 1 {
		return "1 container"
	}
	return fmt.Sprintf("%d containers", n)
}
