# K8s Pod Recommender

> Kubernetes operator that analyzes pod resource usage and recommends optimal CPU/memory configurations.

[![CI](https://github.com/Ankitavasudev/k8s-pod-recommender/actions/workflows/ci.yml/badge.svg)](https://github.com/Ankitavasudev/k8s-pod-recommender/actions)
[![Go 1.21+](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

## Features

- **Resource Analysis** - Analyzes actual CPU/memory usage patterns
- **Smart Recommendations** - Suggests optimal resource limits based on usage
- **RBAC Support** - Proper role-based access control
- **CRD-based** - Custom Resource Definitions for Kubernetes-native workflow
- **Controller Loop** - Continuous monitoring and recommendation updates

## Quick Start

```bash
# Install
git clone https://github.com/Ankitavasudev/k8s-pod-recommender.git
cd k8s-pod-recommender

# Build
go build -o pod-recommender .

# Run locally
./pod-recommender

# Deploy to Kubernetes
kubectl apply -f deploy/
```

## How It Works

1. **Observe** - Monitors pod resource requests vs actual usage
2. **Analyze** - Calculates utilization ratios and trends
3. **Recommend** - Suggests optimal CPU/memory configurations
4. **Apply** - Optionally auto-adjusts resource limits

## RBAC Permissions

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: pod-recommender
rules:
  - apiGroups: [""]
    resources: ["pods", "nodes"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods", "nodes"]
    verbs: ["get", "list"]
```

## Architecture

```
k8s-pod-recommender/
├── main.go           # Controller entry point
├── recommender.go    # Recommendation engine
├── analyzer.go       # Resource usage analysis
├── deploy/           # Kubernetes manifests
├── rbac/             # RBAC configuration
├── tests/            # Unit and integration tests
├── Dockerfile        # Container build
└── .github/workflows/ # CI/CD pipeline
```

## Tech Stack

- **Go 1.21+** - Core language
- **controller-runtime** - Kubernetes controller framework
- **client-go** - Kubernetes API client
- **metrics-server** - Resource usage metrics
- **Docker** - Container packaging

## License

MIT License - see [LICENSE](LICENSE) for details.