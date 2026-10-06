# smoothwrr

A concurrency-safe **smooth weighted round-robin** load balancer written in
pure Go. Library only, plus a minimal CLI demo. No third-party dependencies.

## Algorithm

Each node has a fixed `Weight` and an internal `currentWeight`. On every
`Next` call:

1. Every node's `currentWeight` increases by its `Weight`.
2. The node with the highest `currentWeight` is selected; ties are broken
   deterministically by ascending node ID (never by map iteration order).
3. The selected node's `currentWeight` decreases by the current total of
   all weights.

This yields the classic smooth distribution, e.g. weights `5, 1, 1` produce
the repeating sequence `a a b a c a a`.

Semantics of mutating operations:

- `Add` starts the node's `currentWeight` at 0 and rejects empty IDs,
  non-positive weights, and duplicate IDs.
- `Remove` deletes all of the node's state.
- `UpdateWeight` changes only the weight of an existing node and preserves
  its `currentWeight`, so selection keeps evolving under the new weight.

## Concurrency

All methods are safe for concurrent use. A single mutex makes each `Next`
call (all `currentWeight` updates plus the final selection) one atomic
step, and no goroutine can observe a half-updated state during adds,
removals, or weight changes.

## API

```go
b := smoothwrr.New()
err := b.Add("a", 5)              // ErrEmptyID / ErrInvalidWeight / ErrNodeExists
err  = b.Remove("a")              // ErrNodeNotFound
err  = b.UpdateWeight("a", 3)     // ErrNodeNotFound / ErrInvalidWeight
id, ok := b.Next()                // ok == false when the balancer is empty
nodes  := b.Members()             // []Node sorted by ID, isolated snapshot
n      := b.Len()                 // number of nodes
total  := b.TotalWeight()         // sum of all weights
```

## Run the tests

```sh
go test -race ./...
```

## Run the demo

```sh
go run ./cmd/demo
```

Adds nodes `a=5, b=1, c=1`, prints seven smooth `Next` picks, then
demonstrates one `UpdateWeight` and one `Remove`.

