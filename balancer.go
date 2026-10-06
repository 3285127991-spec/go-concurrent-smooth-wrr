// Package smoothwrr implements a concurrency-safe smooth weighted
// round-robin load balancer.
package smoothwrr

import (
	"errors"
	"sort"
	"sync"
)

var (
	// ErrEmptyID is returned when a node ID is empty.
	ErrEmptyID = errors.New("smoothwrr: node id must not be empty")
	// ErrInvalidWeight is returned when a weight is not a positive integer.
	ErrInvalidWeight = errors.New("smoothwrr: weight must be a positive integer")
	// ErrNodeExists is returned when adding a node whose ID already exists.
	ErrNodeExists = errors.New("smoothwrr: node already exists")
	// ErrNodeNotFound is returned when operating on a node that does not exist.
	ErrNodeNotFound = errors.New("smoothwrr: node not found")
	// ErrNoNodes is returned by Next when the balancer has no nodes.
	ErrNoNodes = errors.New("smoothwrr: balancer has no nodes")
)

// Node describes a balancer member: its unique ID and its configured weight.
type Node struct {
	ID     string
	Weight int64
}

// node is the internal state of a member, including its evolving
// currentWeight used by the smooth weighted round-robin algorithm.
type node struct {
	id            string
	weight        int64
	currentWeight int64
}

// Balancer is a concurrency-safe smooth weighted round-robin balancer.
// The zero value is not ready for use; create one with New.
type Balancer struct {
	mu          sync.Mutex
	nodes       map[string]*node
	totalWeight int64
}

// New returns an empty Balancer.
func New() *Balancer {
	return &Balancer{nodes: make(map[string]*node)}
}

// Add registers a new node. The ID must be non-empty and unique, and the
// weight must be positive. The node's currentWeight starts at 0.
func (b *Balancer) Add(id string, weight int64) error {
	if id == "" {
		return ErrEmptyID
	}
	if weight <= 0 {
		return ErrInvalidWeight
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.nodes[id]; ok {
		return ErrNodeExists
	}
	b.nodes[id] = &node{id: id, weight: weight}
	b.totalWeight += weight
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
	delete(b.nodes, id)
	b.totalWeight -= n.weight
	return nil
}

// UpdateWeight changes the weight of an existing node while preserving its
// currentWeight, so selection continues to evolve under the new weight.
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
	b.totalWeight += weight - n.weight
	n.weight = weight
	return nil
}

// Next selects a node using the smooth weighted round-robin algorithm:
// every node's currentWeight is increased by its weight, the node with the
// largest currentWeight is chosen (ties broken by ascending ID), and the
// chosen node's currentWeight is decreased by the total weight. The whole
// step is atomic with respect to other goroutines.
func (b *Balancer) Next() (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.nodes) == 0 {
		return "", ErrNoNodes
	}
	var best *node
	for _, n := range b.nodes {
		n.currentWeight += n.weight
		if best == nil ||
			n.currentWeight > best.currentWeight ||
			(n.currentWeight == best.currentWeight && n.id < best.id) {
			best = n
		}
	}
	best.currentWeight -= b.totalWeight
	return best.id, nil
}

// Members returns a snapshot of all nodes, sorted by ID in ascending
// lexicographic order. The result is independent of the balancer's
// internal state.
func (b *Balancer) Members() []Node {
	b.mu.Lock()
	defer b.mu.Unlock()
	members := make([]Node, 0, len(b.nodes))
	for _, n := range b.nodes {
		members = append(members, Node{ID: n.id, Weight: n.weight})
	}
	sort.Slice(members, func(i, j int) bool { return members[i].ID < members[j].ID })
	return members
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
	return b.totalWeight
}
