package smoothwrr

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func mustAdd(t *testing.T, b *Balancer, id string, w int64) {
	t.Helper()
	if err := b.Add(id, w); err != nil {
		t.Fatalf("Add(%q, %d) failed: %v", id, w, err)
	}
}

func nextID(t *testing.T, b *Balancer) string {
	t.Helper()
	id, ok := b.Next()
	if !ok {
		t.Fatal("Next returned ok=false on non-empty balancer")
	}
	return id
}

func TestAddAndRemove(t *testing.T) {
	b := New()
	mustAdd(t, b, "a", 1)
	mustAdd(t, b, "b", 2)
	if got := b.Len(); got != 2 {
		t.Fatalf("Len = %d, want 2", got)
	}
	if err := b.Remove("a"); err != nil {
		t.Fatalf("Remove(a) failed: %v", err)
	}
	if got := b.Len(); got != 1 {
		t.Fatalf("Len after Remove = %d, want 1", got)
	}
	if err := b.Remove("a"); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("Remove missing = %v, want ErrNodeNotFound", err)
	}
	if err := b.Remove(""); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("Remove empty id = %v, want ErrNodeNotFound", err)
	}
}

func TestAddRejectsInvalidInput(t *testing.T) {
	b := New()
	if err := b.Add("", 1); !errors.Is(err, ErrEmptyID) {
		t.Fatalf("Add empty id = %v, want ErrEmptyID", err)
	}
	if err := b.Add("a", 0); !errors.Is(err, ErrInvalidWeight) {
		t.Fatalf("Add zero weight = %v, want ErrInvalidWeight", err)
	}
	if err := b.Add("a", -3); !errors.Is(err, ErrInvalidWeight) {
		t.Fatalf("Add negative weight = %v, want ErrInvalidWeight", err)
	}
	if got := b.Len(); got != 0 {
		t.Fatalf("Len = %d, want 0 after rejected adds", got)
	}
}

func TestAddDuplicate(t *testing.T) {
	b := New()
	mustAdd(t, b, "a", 1)
	if err := b.Add("a", 5); !errors.Is(err, ErrNodeExists) {
		t.Fatalf("duplicate Add = %v, want ErrNodeExists", err)
	}
	// The original weight must be untouched.
	if got := b.TotalWeight(); got != 1 {
		t.Fatalf("TotalWeight = %d, want 1", got)
	}
}

func TestNextEmptyBalancer(t *testing.T) {
	b := New()
	if id, ok := b.Next(); ok || id != "" {
		t.Fatalf("Next on empty = (%q, %v), want (\"\", false)", id, ok)
	}
	if got := b.Len(); got != 0 {
		t.Fatalf("Len = %d, want 0", got)
	}
	if got := b.TotalWeight(); got != 0 {
		t.Fatalf("TotalWeight = %d, want 0", got)
	}
	if got := b.Members(); len(got) != 0 {
		t.Fatalf("Members = %v, want empty", got)
	}
}

func TestSmoothSequence(t *testing.T) {
	b := New()
	mustAdd(t, b, "a", 5)
	mustAdd(t, b, "b", 1)
	mustAdd(t, b, "c", 1)
	want := []string{"a", "a", "b", "a", "c", "a", "a"}
	for i, w := range want {
		if got := nextID(t, b); got != w {
			t.Fatalf("pick %d = %q, want %q", i+1, got, w)
		}
	}
}

func TestTieBreakByID(t *testing.T) {
	b := New()
	// Insert in reverse lexicographic order to prove the tie-break does
	// not depend on insertion or map iteration order.
	mustAdd(t, b, "c", 1)
	mustAdd(t, b, "b", 1)
	mustAdd(t, b, "a", 1)
	want := []string{"a", "b", "c", "a", "b", "c"}
	for i, w := range want {
		if got := nextID(t, b); got != w {
			t.Fatalf("pick %d = %q, want %q", i+1, got, w)
		}
	}
}

func TestUpdateWeight(t *testing.T) {
	b := New()
	if err := b.UpdateWeight("ghost", 2); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("UpdateWeight missing = %v, want ErrNodeNotFound", err)
	}
	mustAdd(t, b, "p", 1)
	mustAdd(t, b, "q", 1)
	mustAdd(t, b, "r", 1)
	if err := b.UpdateWeight("p", 0); !errors.Is(err, ErrInvalidWeight) {
		t.Fatalf("UpdateWeight zero = %v, want ErrInvalidWeight", err)
	}

	if got := nextID(t, b); got != "p" {
		t.Fatalf("pick 1 = %q, want p", got)
	}
	if got := nextID(t, b); got != "q" {
		t.Fatalf("pick 2 = %q, want q", got)
	}
	// p's current weight is now -1. Updating p to weight 2 must keep that
	// current weight: the next pick is r. If p's current weight had been
	// reset to 0, p would tie r and win the lexicographic tie-break.
	if err := b.UpdateWeight("p", 2); err != nil {
		t.Fatalf("UpdateWeight(p, 2) failed: %v", err)
	}
	if got := b.TotalWeight(); got != 4 {
		t.Fatalf("TotalWeight = %d, want 4", got)
	}
	if got := nextID(t, b); got != "r" {
		t.Fatalf("pick 3 = %q, want r (current weight must be preserved)", got)
	}
}

func TestMembersSortedAndIsolated(t *testing.T) {
	b := New()
	mustAdd(t, b, "c", 3)
	mustAdd(t, b, "a", 1)
	mustAdd(t, b, "b", 2)
	got := b.Members()
	want := []Node{{ID: "a", Weight: 1}, {ID: "b", Weight: 2}, {ID: "c", Weight: 3}}
	if len(got) != len(want) {
		t.Fatalf("Members = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Members[%d] = %v, want %v", i, got[i], want[i])
		}
	}
	// Mutating the returned slice must not affect the balancer.
	got[0].Weight = 999
	if again := b.Members(); again[0].Weight != 1 {
		t.Fatalf("Members not isolated: weight became %d", again[0].Weight)
	}
}

func TestTotalWeight(t *testing.T) {
	b := New()
	mustAdd(t, b, "a", 2)
	mustAdd(t, b, "b", 3)
	if got := b.TotalWeight(); got != 5 {
		t.Fatalf("TotalWeight = %d, want 5", got)
	}
	if err := b.UpdateWeight("a", 10); err != nil {
		t.Fatal(err)
	}
	if got := b.TotalWeight(); got != 13 {
		t.Fatalf("TotalWeight after update = %d, want 13", got)
	}
	if err := b.Remove("b"); err != nil {
		t.Fatal(err)
	}
	if got := b.TotalWeight(); got != 10 {
		t.Fatalf("TotalWeight after remove = %d, want 10", got)
	}
}

func TestConcurrent(t *testing.T) {
	b := New()
	const adders = 4
	const perAdder = 25

	var wg sync.WaitGroup
	for g := 0; g < adders; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < perAdder; i++ {
				id := fmt.Sprintf("n-%d-%d", g, i)
				if err := b.Add(id, 1); err != nil {
					t.Errorf("Add(%q) failed: %v", id, err)
				}
			}
		}(g)
	}
	wg.Wait()
	if got := b.Len(); got != adders*perAdder {
		t.Fatalf("Len = %d, want %d", got, adders*perAdder)
	}

	valid := make(map[string]bool, adders*perAdder)
	for _, n := range b.Members() {
		valid[n.ID] = true
	}

	const pickers = 4
	const picksEach = 250
	for g := 0; g < pickers; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < picksEach; i++ {
				id, ok := b.Next()
				if !ok {
					t.Error("Next returned ok=false on non-empty balancer")
					return
				}
				if !valid[id] {
					t.Errorf("Next returned unknown id %q", id)
					return
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_ = b.Members()
			_ = b.Len()
			_ = b.TotalWeight()
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			if err := b.UpdateWeight(fmt.Sprintf("n-0-%d", i%perAdder), int64(i%3+1)); err != nil {
				t.Errorf("UpdateWeight failed: %v", err)
			}
		}
	}()
	wg.Wait()

	for _, n := range b.Members() {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if err := b.Remove(id); err != nil {
				t.Errorf("Remove(%q) failed: %v", id, err)
			}
		}(n.ID)
	}
	wg.Wait()
	if got := b.Len(); got != 0 {
		t.Fatalf("Len = %d, want 0 after removing all nodes", got)
	}
	if _, ok := b.Next(); ok {
		t.Fatal("Next returned ok=true after all nodes removed")
	}
}
