# k8s-pod-recommender

Read a cluster's pod specs and say which ones have resource settings worth
changing, and why. It works on live output from `kubectl`, on a saved
`kubectl get pods -o json` file, or on stdin, so you can run it without cluster
access.

```bash
k8s-pod-recommender                          # every namespace, live
k8s-pod-recommender -n payments              # one namespace
k8s-pod-recommender -f examples/pods.json    # a saved capture, no cluster needed
k8s-pod-recommender -f examples/pods.json -o html > report.html
```

[See a generated report](https://ankitavasudev.github.io/k8s-pod-recommender/),
built from [`examples/pods.json`](examples/pods.json).

## Install

```bash
go install github.com/Ankitavasudev/k8s-pod-recommender@v0.2.0
```

Go 1.21 or newer, and no third-party modules. `go install` names the binary after
the module, so the command is `k8s-pod-recommender`. From a clone,
`go build -o k8s-pod-recommender .` produces the same binary.

## What it does

For each pod it sums the CPU and memory requests and limits across the pod's
containers — that total is what the scheduler reserves — and returns one verdict:

| Action | When | Suggested change |
|---|---|---|
| `fix` | a request is above its limit on CPU or memory | none; the pod cannot schedule, so the numbers need a human |
| `set` | the pod has no requests at all | 100m CPU and 64Mi memory per container |
| `investigate` | more than 5 restarts | none — restarts usually mean OOMKilled or an evicted node, so read the events first |
| `tighten` | the CPU limit is more than 4x the request | none — see below |
| `watch` | 1 to 5 restarts | none yet |
| `keep` | none of the above | none |

**It does not read live usage.** There is no metrics-server client here, so the
tool can only see configurations that are internally inconsistent or missing a
value. That is why `investigate` refuses to invent a memory number, and why
`tighten` leaves the request alone: raising it to the limit would be a guess, and
for a pod with a large burst limit that guess could be far above what the pod
ever uses. The reason text explains the trade-off instead.

For reference, the sizing helpers used when a value is suggested
(`CalculateRecommendedCPU`, `CalculateRecommendedMemory`) apply 50% headroom,
round up to 10m or 16Mi, and never go below 100m or 64Mi. They are exported and
tested, so the next step — feeding them numbers from `kubectl top` — only needs a
new input path, not a rewrite of the rules.

## Output

```text
K8s Pod Resource Recommendations
============================================================

Pod: staging/feature-flags-1
  Current:     CPU=—  Memory=—
  Suggested:   CPU=200m  Memory=128Mi
  Action:      set
  Reason:      No resource requests on 2 containers — the scheduler cannot place this pod fairly
```

`-o json` emits an array of the same records (`pod`, `namespace`, `current_cpu`,
`current_memory`, `recommended_cpu`, `recommended_memory`, `action`, `reason`).

`-o html` writes a single self-contained page with inline CSS and no JavaScript
or external requests. It opens from disk or from a static host, and pod names and
reasons are HTML-escaped, so it is safe to attach to a ticket.

Results are sorted by namespace and then pod name, so two runs over the same
input produce the same output.

Exit code is `1` when `kubectl` fails or the input cannot be read, so it is safe
to use in CI. The stderr message includes kubectl's own last line instead of just
"exit status 1".

## Reading a saved capture

`-f` takes a file path, or `-` for stdin:

```bash
kubectl get pods -o json > pods.json
k8s-pod-recommender -f pods.json

kubectl get pods -n prod -o json | k8s-pod-recommender -f - -o json
```

This is the mode used by the demo and by CI, and it is how you review a cluster
you cannot reach from your laptop.

## Layout

```
main.go                        CLI: flags, kubectl call, JSON parsing, text/JSON rendering
report.go                      HTML report and version reporting
main_test.go                   parsing, container sums, sorting, file input, HTML escaping
pkg/recommender/recommender.go rules, quantity parsing and formatting
pkg/recommender/recommender_test.go
examples/pods.json             12 pods that cover every action
scripts/build-demo.sh          regenerates the published report
docs/                          the report served by GitHub Pages
```

Only the standard library is used; `go.mod` has no `require` block.

## Development

```bash
go test ./...
go vet ./...
gofmt -l .                # should print nothing
./scripts/build-demo.sh   # regenerate docs/index.html
```

`ParsePods` and `fetchPods` take plain data and a `runner` func, so the tests
never shell out to a real cluster.

## Licence

MIT — see [LICENSE](LICENSE).
