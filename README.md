# k8s-pod-recommender

Inspect the pods in your cluster and get a resource-request suggestion for each
one. Single binary, no CRDs, no controller loop â€” it reads
`kubectl get pods -o json` and prints what it would change.

```bash
go build -o pod-recommender .
./pod-recommender                 # every namespace
./pod-recommender -n payments     # one namespace
./pod-recommender -o json         # for scripting
```

## Install

```bash
go install github.com/Ankitavasudev/k8s-pod-recommender@v0.1.0
```

Go 1.21+. Nothing else — `go.mod` has no dependencies. From a clone:
`go build -o pod-recommender .`

## What it actually does

For each pod, `Recommend` looks at the first container's CPU/memory requests and
limits, then returns one verdict:

| Action | When | Suggested change |
|---|---|---|
| `fix` | request > limit on CPU or memory | the pod can never schedule; no numbers guessed |
| `set` | no requests at all | 100m CPU / 64Mi memory |
| `investigate` | more than 5 restarts | nothing â€” restarts usually mean OOMKilled or an evicted node, so look at events first |
| `tighten` | CPU limit > 4Ã— the request | flags the limit; the request is left alone |
| `watch` | 1â€“5 restarts | nothing yet |
| `keep` | none of the above | no change |

Sizing helpers (`CalculateRecommendedCPU`, `CalculateRecommendedMemory`) apply
50 % headroom, round up to 10m / 16Mi, and never suggest less than 100m / 64Mi.

**It does not read live usage.** There is no metrics-server client here â€” the
tool can only spot configurations that are internally inconsistent or missing,
which is why `investigate` refuses to invent a memory value. Adding
`kubectl top` numbers is the obvious next step, not something this claims today.

## Output

```text
K8s Pod Resource Recommendations
============================================================

Pod: prod/web-0
  Current:     CPU=250m  Memory=128Mi
  Suggested:   CPU=250m  Memory=128Mi
  Action:      keep
  Reason:      No change needed
```

`-o json` emits an array of the same records
(`pod`, `namespace`, `current_cpu`, `current_memory`, `recommended_cpu`,
`recommended_memory`, `action`, `reason`).

Exit code is `1` if `kubectl` fails, so it is safe to run in CI. The stderr
message now includes kubectl's own output instead of just "exit status 1".

## Layout

```
main.go                     CLI: flag parsing, kubectl call, text/JSON rendering
pkg/recommender/            Pod -> Recommendation, quantity parsing, formatting
  recommender.go
  recommender_test.go
main_test.go                JSON parsing, namespace flags, error propagation
```

Only the standard library is used (`go.mod` has no `require` block).

## Development

```bash
go test ./...
go vet ./...
```

`ParsePods` and `fetchPods` take interfaces (`[]byte` and a `runner` func), so the
tests never shell out to a real cluster.

## Licence

MIT â€” see [LICENSE](LICENSE).
