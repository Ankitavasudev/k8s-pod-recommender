package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/Ankitavasudev/k8s-pod-recommender/pkg/recommender"
)

// podList mirrors the parts of `kubectl get pods -o json` we care about.
type podList struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Status struct {
			Phase             string `json:"phase"`
			ContainerStatuses []struct {
				RestartCount int `json:"restartCount"`
			} `json:"containerStatuses"`
		} `json:"status"`
		Spec struct {
			Containers []struct {
				Resources struct {
					Requests struct {
						CPU    string `json:"cpu"`
						Memory string `json:"memory"`
					} `json:"requests"`
					Limits struct {
						CPU    string `json:"cpu"`
						Memory string `json:"memory"`
					} `json:"limits"`
				} `json:"resources"`
			} `json:"containers"`
		} `json:"spec"`
	} `json:"items"`
}

// ParsePods turns `kubectl get pods -o json` output into recommender pods.
// Exported for tests; the command-line path uses it too.
//
// Requests and limits are summed across the pod's containers, since a pod is
// scheduled on the total it reserves, not on any single container.
func ParsePods(raw []byte) ([]recommender.Pod, error) {
	var list podList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("parse kubectl output: %w", err)
	}

	pods := make([]recommender.Pod, 0, len(list.Items))
	for _, item := range list.Items {
		restarts := 0
		for _, cs := range item.Status.ContainerStatuses {
			restarts += cs.RestartCount
		}

		pod := recommender.Pod{
			Name:       item.Metadata.Name,
			Namespace:  item.Metadata.Namespace,
			Status:     item.Status.Phase,
			Restarts:   restarts,
			Containers: len(item.Spec.Containers),
		}

		var cpuReq, cpuLim, memReq, memLim float64
		var hasCPUReq, hasCPULim, hasMemReq, hasMemLim bool
		for _, container := range item.Spec.Containers {
			res := container.Resources
			if v := recommender.ParseCPU(res.Requests.CPU); v > 0 {
				cpuReq += v
				hasCPUReq = true
			}
			if v := recommender.ParseCPU(res.Limits.CPU); v > 0 {
				cpuLim += v
				hasCPULim = true
			}
			if v := recommender.ParseMemory(res.Requests.Memory); v > 0 {
				memReq += v
				hasMemReq = true
			}
			if v := recommender.ParseMemory(res.Limits.Memory); v > 0 {
				memLim += v
				hasMemLim = true
			}
		}
		if hasCPUReq {
			pod.CPUReq = recommender.FormatCPU(cpuReq)
		}
		if hasCPULim {
			pod.CPULim = recommender.FormatCPU(cpuLim)
		}
		if hasMemReq {
			pod.MemReq = recommender.FormatMemory(memReq)
		}
		if hasMemLim {
			pod.MemLim = recommender.FormatMemory(memLim)
		}
		pods = append(pods, pod)
	}
	return pods, nil
}

type runner func(args []string) ([]byte, error)

func kubectl(args []string) ([]byte, error) {
	return exec.Command("kubectl", args...).CombinedOutput()
}

func fetchPods(run runner, namespace string) ([]recommender.Pod, error) {
	args := []string{"get", "pods", "-o", "json"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	} else {
		args = append(args, "--all-namespaces")
	}
	out, err := run(args)
	if err != nil {
		// kubectl repeats its connection warnings; the last line is the useful one.
		detail := strings.TrimSpace(string(out))
		if i := strings.LastIndex(detail, "\n"); i >= 0 {
			detail = detail[i+1:]
		}
		return nil, fmt.Errorf("kubectl failed: %w: %s", err, detail)
	}
	return ParsePods(out)
}

// readPods reads saved `kubectl get pods -o json` output, so the tool can be
// used on a laptop with no cluster access, in CI, or against a captured
// incident.
func readPods(path string) ([]recommender.Pod, error) {
	var (
		data []byte
		err  error
	)
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("read pods: %w", err)
	}
	return ParsePods(data)
}

// sortRecommendations orders by namespace then pod name so that the output of
// two runs over the same input is identical.
func sortRecommendations(recs []recommender.Recommendation) {
	sort.SliceStable(recs, func(i, j int) bool {
		if recs[i].Namespace != recs[j].Namespace {
			return recs[i].Namespace < recs[j].Namespace
		}
		return recs[i].Pod < recs[j].Pod
	})
}

// Render formats recommendations as text, JSON or HTML.
func Render(recs []recommender.Recommendation, format string) (string, error) {
	switch format {
	case "json":
		data, err := json.MarshalIndent(recs, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
	case "html":
		return renderHTML(recs)
	}

	var b strings.Builder
	b.WriteString("K8s Pod Resource Recommendations\n")
	b.WriteString(strings.Repeat("=", 60))
	b.WriteString("\n")
	for _, rec := range recs {
		fmt.Fprintf(&b, "\nPod: %s/%s\n", rec.Namespace, rec.Pod)
		fmt.Fprintf(&b, "  Current:     CPU=%s  Memory=%s\n", orDash(rec.CurrentCPU), orDash(rec.CurrentMem))
		fmt.Fprintf(&b, "  Suggested:   CPU=%s  Memory=%s\n", orDash(rec.RecCPU), orDash(rec.RecMem))
		fmt.Fprintf(&b, "  Action:      %s\n", rec.Action)
		fmt.Fprintf(&b, "  Reason:      %s\n", rec.Reason)
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}

func orDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func main() {
	namespace := flag.String("n", "", "Namespace (empty = all)")
	output := flag.String("o", "text", "Output format: text, json, html")
	input := flag.String("f", "", "Read saved `kubectl get pods -o json` output from a file, or - for stdin")
	showVersion := flag.Bool("version", false, "Print the version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(versionString())
		return
	}

	var (
		pods []recommender.Pod
		err  error
	)
	if *input != "" {
		pods, err = readPods(*input)
	} else {
		pods, err = fetchPods(kubectl, *namespace)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	recs := make([]recommender.Recommendation, 0, len(pods))
	for _, pod := range pods {
		recs = append(recs, recommender.Recommend(pod))
	}
	sortRecommendations(recs)

	rendered, err := Render(recs, *output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(rendered)
}
