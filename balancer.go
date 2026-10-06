// Package smoothwrr implements a concurrency-safe smooth weighted
// round-robin load balancer.
package smoothwrr

import (
	"errors"
	"sort"
	"sync"
)

var (
	ErrEmptyID       = errors.New("smoothwrr: node ID must not be empty")
	ErrInvalidWeight = errors.New("smoothwrr: weight must be a positive integer")
	ErrNodeExists    = errors.New("smoothwrr: node already exists")
	ErrNodeNotFound  = errors.New("smoothwrr: node not found")
)

// Node is a snapshot of a balancer member: its ID and configured weight.
type Node struct {
	ID     string
	Weight int64
}

type nodeState struct {
	weight  int64
	current int64
}

// Balancer is a smooth weighted round-robin balancer safe for concurrent use.
// The zero value is ready to use.
type Balancer struct {
	mu    sync.Mutex
	nodes map[string]*nodeState
	total int64
}

// New returns an empty Balancer.
func New() *Balancer {
	return &Balancer{nodes: make(map[string]*nodeState)}
}

// Add registers a new node with the given ID and positive weight.
// The node's current weight starts at 0.
func (b *Balancer) Add(id string, weight int64) error {
	if id == "" {
		return ErrEmptyID
	}
	if weight <= 0 {
		return ErrInvalidWeight
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.nodes == nil {
		b.nodes = make(map[string]*nodeState)
	}
	if _, ok := b.nodes[id]; ok {
		return ErrNodeExists
	}
	b.nodes[id] = &nodeState{weight: weight}
	b.total += weight
	return nil
}

// Remove deletes a node and all of its state.
func (b *Balancer) Remove(id string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	n, ok := b.nodes[id]
	if !ok {
		return ErrNodeNotFound
	}
	b.total -= n.weight
	delete(b.nodes, id)
	return nil
}

// UpdateWeight changes the weight of an existing node while preserving its
// current weight, so selection continues to evolve under the new weight.
func (b *Balancer) UpdateWeight(id string, weight int64) error {
	if weight <= 0 {
		return ErrInvalidWeight
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	n, ok := b.nodes[id]
	if !ok {
		return ErrNodeNotFound
	}
	b.total += weight - n.weight
	n.weight = weight
	return nil
}

// Next atomically advances all current weights, selects the node with the
// highest current weight (ties broken by ascending ID), and subtracts the
// total weight from the selected node's current weight. It returns false
// when the balancer has no nodes.
func (b *Balancer) Next() (string, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.nodes) == 0 {
		return "", false
	}
	var bestID string
	var best *nodeState
	for id, n := range b.nodes {
		n.current += n.weight
		if best == nil || n.current > best.current ||
			(n.current == best.current && id < bestID) {
			bestID, best = id, n
		}
	}
	best.current -= b.total
	return bestID, true
}

// Members returns a snapshot of all nodes sorted by ascending ID.
// The result is independent of the balancer's internal state.
func (b *Balancer) Members() []Node {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Node, 0, len(b.nodes))
	for id, n := range b.nodes {
		out = append(out, Node{ID: id, Weight: n.weight})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Len returns the number of nodes.
func (b *Balancer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.nodes)
}

// TotalWeight returns the sum of all node weights.
func (b *Balancer) TotalWeight() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.total
}
