package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type PodInfo struct {
	Name       string            +""+json:"name"+""+
	Namespace  string            +""+json:"namespace"+""+
	Status     string            +""+json:"status"+""+
	Restarts   int               +""+json:"restarts"+""+
	CPUReq     string            +""+json:"cpu_request"+""+
	CPULim     string            +""+json:"cpu_limit"+""+
	MemReq     string            +""+json:"memory_request"+""+
	MemLim     string            +""+json:"memory_limit"+""+
 Recommendation string         +""+json:"recommendation"+""+
}

type ResourceRecommendation struct {
	Pod         string +""+json:"pod"+""+
	Namespace   string +""+json:"namespace"+""+
	CurrentCPU  string +""+json:"current_cpu"+""+
	CurrentMem  string +""+json:"current_memory"+""+
	RecCPU      string +""+json:"recommended_cpu"+""+
	RecMem      string +""+json:"recommended_memory"+""+
	Reason      string +""+json:"reason"+""+
}

func runKubectl(args []string) ([]byte, error) {
	cmd := exec.Command("kubectl", args...)
	return cmd.CombinedOutput()
}

func getPods(namespace string) ([]PodInfo, error) {
	args := []string{"get", "pods", "-o", "json"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	} else {
		args = append(args, "--all-namespaces")
	}

	out, err := runKubectl(args)
	if err != nil {
		return nil, fmt.Errorf("kubectl failed: %w", err)
	}

	var result struct {
		Items []struct {
			Metadata struct {
				Name      string +""+json:"name"+""+
				Namespace string +""+json:"namespace"+""+
			} +""+json:"metadata"+""+
			Status struct {
				Phase             string +""+json:"phase"+""+
				ContainerStatuses []struct {
					RestartCount int +""+json:"restartCount"+""+
				} +""+json:"containerStatuses"+""+
			} +""+json:"status"+""+
			Spec struct {
				Containers []struct {
					Resources struct {
						Requests struct {
							CPU    string +""+json:"cpu"+""+
							Memory string +""+json:"memory"+""+
						} +""+json:"requests"+""+
						Limits struct {
							CPU    string +""+json:"cpu"+""+
							Memory string +""+json:"memory"+""+
						} +""+json:"limits"+""+
					} +""+json:"resources"+""+
				} +""+json:"containers"+""+
			} +""+json:"spec"+""+
		} +""+json:"items"+""+
	}

	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}

	var pods []PodInfo
	for _, item := range result.Items {
		restarts := 0
		for _, cs := range item.Status.ContainerStatuses {
			restarts += cs.RestartCount
		}

		cpuReq, cpuLim, memReq, memLim := "", "", "", ""
		if len(item.Spec.Containers) > 0 {
			c := item.Spec.Containers[0]
			cpuReq = c.Resources.Requests.CPU
			cpuLim = c.Resources.Limits.CPU
			memReq = c.Resources.Requests.Memory
			memLim = c.Resources.Limits.Memory
		}

		pods = append(pods, PodInfo{
			Name:      item.Metadata.Name,
			Namespace: item.Metadata.Namespace,
			Status:    item.Status.Phase,
			Restarts:  restarts,
			CPUReq:    cpuReq,
			CPULim:    cpuLim,
			MemReq:    memReq,
			MemLim:    memLim,
		})
	}
	return pods, nil
}

func recommend(pod PodInfo) ResourceRecommendation {
	rec := ResourceRecommendation{
		Pod:        pod.Name,
		Namespace:  pod.Namespace,
		CurrentCPU: pod.CPUReq,
		CurrentMem: pod.MemReq,
		RecCPU:     pod.CPUReq,
		RecMem:     pod.MemReq,
		Reason:     "No change needed",
	}

	if pod.Restarts > 5 {
		rec.Reason = fmt.Sprintf("High restart count (%d) - consider increasing memory limits", pod.Restarts)
		rec.RecMem = "512Mi"
	} else if pod.Restarts > 0 {
		rec.Reason = fmt.Sprintf("Restart detected (%d times) - monitor for issues", pod.Restarts)
	} else if pod.Status == "Running" && pod.CPUReq == "" {
		rec.Reason = "No resource requests set - adding defaults"
		rec.RecCPU = "100m"
		rec.RecMem = "128Mi"
	}

	return rec
}

func main() {
	namespace := flag.String("n", "", "Namespace (empty = all)")
	output := flag.String("o", "", "Output format: json, text")
	flag.Parse()

	pods, err := getPods(*namespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	var recommendations []ResourceRecommendation
	for _, pod := range pods {
		rec := recommend(pod)
		recommendations = append(recommendations, rec)
	}

	if *output == "json" {
		data, _ := json.MarshalIndent(recommendations, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Println("K8s Pod Resource Recommendations")
		fmt.Println(strings.Repeat("=", 60))
		for _, rec := range recommendations {
			fmt.Printf("\nPod: %s/%s\n", rec.Namespace, rec.Pod)
			fmt.Printf("  Current:  CPU=%s  Memory=%s\n", rec.CurrentCPU, rec.CurrentMem)
			fmt.Printf("  Recommended: CPU=%s  Memory=%s\n", rec.RecCPU, rec.RecMem)
			fmt.Printf("  Reason: %s\n", rec.Reason)
		}
	}
}