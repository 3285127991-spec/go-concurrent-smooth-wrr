# smoothwrr

A small, concurrency-safe **smooth weighted round-robin** (SWRR) load
balancer library for Go, with a minimal CLI demo. No third-party
dependencies.

## Algorithm

Each `Next` call performs one atomic selection step:

1. Every node's `currentWeight` is increased by its configured `Weight`.
2. The node with the largest `currentWeight` is selected; ties are broken
   deterministically by ascending node ID (never by map iteration order).
3. The selected node's `currentWeight` is decreased by the current total
   weight of all nodes.

This produces the classic smooth distribution, e.g. weights 5/1/1 yield
`a a b a c a a` over one full cycle.

## API

```go
b := smoothwrr.New()

err := b.Add("a", 5)          // rejects empty ID, non-positive weight, duplicates
err  = b.Remove("a")          // ErrNodeNotFound if missing
err  = b.UpdateWeight("a", 3) // keeps currentWeight, selection evolves with new weight
id, err := b.Next()           // ErrNoNodes when empty
members := b.Members()        // []Node sorted by ID, isolated snapshot
n := b.Len()                  // number of nodes
w := b.TotalWeight()          // sum of all weights
```

All methods are safe for concurrent use; each `Next` step is fully atomic.

## Layout

- `balancer.go` — the library (package `smoothwrr`)
- `balancer_test.go` — unit tests plus a basic concurrency test
- `cmd/demo/main.go` — minimal CLI demo

## Run

```sh
go test ./...        # unit tests
go test -race ./...  # tests with the race detector
go run ./cmd/demo    # minimal demo
```
