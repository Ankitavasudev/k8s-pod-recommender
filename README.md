# K8s Pod Recommender

![CI](https://github.com/Ankitavasudev/k8s-pod-recommender/actions/workflows/ci.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/Ankitavasudev/k8s-pod-recommender)](https://goreportcard.com/report/github.com/Ankitavasudev/k8s-pod-recommender)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A Kubernetes operator that analyzes pod resource usage and recommends optimal CPU/memory limits to prevent OOMKills and improve cluster efficiency.

## Features

- **Resource Analysis** - Detects missing limits/requests, high CPU/memory ratios
- **Smart Recommendations** - Suggests optimal resource configurations
- **Event Recording** - Logs recommendations as Kubernetes events
- **Real-time Monitoring** - Reconciles every 10 minutes
- **RBAC** - Least-privilege access to cluster resources

## Installation

`ash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/deployment.yaml
`

## How It Works

1. Operator watches all running pods
2. Analyzes resource requests vs limits
3. Detects:
   - Missing CPU/memory limits
   - Missing CPU/memory requests
   - High CPU limit ratio (>10x request)
   - High memory limit ratio (>4x request)
4. Emits recommendations as Kubernetes events
5. Re-queues for next analysis in 10 minutes

## Detection Rules

| Rule | Condition | Recommendation |
|------|-----------|----------------|
| MissingLimits | No CPU/memory limits | Set resource limits |
| MissingRequests | No CPU/memory requests | Set resource requests |
| HighCpuRatio | CPU limit > 10x request | Reduce CPU limit |
| HighMemoryRatio | Memory limit > 4x request | Reduce memory limit |

## Example Output

`
Pod: default/nginx-abc123
  Status: Running | Restarts: 0 | Node: node-1 | Age: 5d
  Resource Recommendations:
    Container app: Set resource limits (current: None)
    Container app: Set resource requests (current: None)
`

## Development

`ash
# Run tests
go test -v ./...

# Build
go build -v ./...

# Run locally
go run . --metrics-bind-address=:8080
`

## Architecture

`
Operator Manager
    |
    v
Reconciler Loop
    |
    v
Pod Analysis
    |
    v
Resource Recommendations
    |
    v
Kubernetes Events
`

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT