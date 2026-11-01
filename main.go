package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Status:    item.Status.Phase,
			Restarts:  restarts,
		}
		if len(item.Spec.Containers) > 0 {
			res := item.Spec.Containers[0].Resources
			pod.CPUReq = res.Requests.CPU
			pod.CPULim = res.Limits.CPU
			pod.MemReq = res.Requests.Memory
			pod.MemLim = res.Limits.Memory
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

// Render formats recommendations as text or JSON.
func Render(recs []recommender.Recommendation, format string) (string, error) {
	if format == "json" {
		data, err := json.MarshalIndent(recs, "", "  ")
		if err != nil {
			return "", err
		}
		return string(data), nil
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
	return b.String(), nil
}

func orDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}

func main() {
	namespace := flag.String("n", "", "Namespace (empty = all)")
	output := flag.String("o", "text", "Output format: json, text")
	flag.Parse()

	pods, err := fetchPods(kubectl, *namespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	recs := make([]recommender.Recommendation, 0, len(pods))
	for _, pod := range pods {
		recs = append(recs, recommender.Recommend(pod))
	}

	rendered, err := Render(recs, *output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(rendered)
}
