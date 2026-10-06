package smoothwrr

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func mustAdd(t *testing.T, b *Balancer, id string, weight int64) {
	t.Helper()
	if err := b.Add(id, weight); err != nil {
		t.Fatalf("Add(%q, %d) failed: %v", id, weight, err)
	}
}

func nextSeq(t *testing.T, b *Balancer, n int) []string {
	t.Helper()
	seq := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id, err := b.Next()
		if err != nil {
			t.Fatalf("Next() failed: %v", err)
		}
		seq = append(seq, id)
	}
	return seq
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAddAndRemove(t *testing.T) {
	b := New()
	mustAdd(t, b, "a", 5)
	mustAdd(t, b, "b", 1)
	if got := b.Len(); got != 2 {
		t.Fatalf("Len() = %d, want 2", got)
	}
	if err := b.Remove("a"); err != nil {
		t.Fatalf("Remove(a) failed: %v", err)
	}
	if got := b.Len(); got != 1 {
		t.Fatalf("Len() = %d, want 1", got)
	}
	if err := b.Remove("a"); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("Remove(missing) = %v, want ErrNodeNotFound", err)
	}
	if err := b.UpdateWeight("a", 2); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("UpdateWeight(missing) = %v, want ErrNodeNotFound", err)
	}
}

func TestAddRejectsInvalidInput(t *testing.T) {
	b := New()
	if err := b.Add("", 1); !errors.Is(err, ErrEmptyID) {
		t.Fatalf("Add(empty id) = %v, want ErrEmptyID", err)
	}
	if err := b.Add("a", 0); !errors.Is(err, ErrInvalidWeight) {
		t.Fatalf("Add(weight 0) = %v, want ErrInvalidWeight", err)
	}
	if err := b.Add("a", -3); !errors.Is(err, ErrInvalidWeight) {
		t.Fatalf("Add(weight -3) = %v, want ErrInvalidWeight", err)
	}
	if got := b.Len(); got != 0 {
		t.Fatalf("Len() = %d, want 0", got)
	}
}

func TestAddDuplicate(t *testing.T) {
	b := New()
	mustAdd(t, b, "a", 1)
	if err := b.Add("a", 2); !errors.Is(err, ErrNodeExists) {
		t.Fatalf("Add(duplicate) = %v, want ErrNodeExists", err)
	}
	// The original node must be untouched.
	if got := b.TotalWeight(); got != 1 {
		t.Fatalf("TotalWeight() = %d, want 1", got)
	}
}

func TestNextEmpty(t *testing.T) {
	b := New()
	id, err := b.Next()
	if !errors.Is(err, ErrNoNodes) {
		t.Fatalf("Next() error = %v, want ErrNoNodes", err)
	}
	if id != "" {
		t.Fatalf("Next() id = %q, want empty", id)
	}
}

func TestSmoothSequence(t *testing.T) {
	b := New()
	mustAdd(t, b, "a", 5)
	mustAdd(t, b, "b", 1)
	mustAdd(t, b, "c", 1)
	want := []string{"a", "a", "b", "a", "c", "a", "a"}
	if got := nextSeq(t, b, len(want)); !equalStrings(got, want) {
		t.Fatalf("sequence = %v, want %v", got, want)
	}
}

func TestTieBreak(t *testing.T) {
	b := New()
	// Insert in reverse order; selection must still be deterministic.
	mustAdd(t, b, "c", 1)
	mustAdd(t, b, "b", 1)
	mustAdd(t, b, "a", 1)
	want := []string{"a", "b", "c", "a", "b", "c"}
	if got := nextSeq(t, b, len(want)); !equalStrings(got, want) {
		t.Fatalf("sequence = %v, want %v", got, want)
	}
}

func TestUpdateWeight(t *testing.T) {
	b := New()
	mustAdd(t, b, "a", 1)
	mustAdd(t, b, "b", 1)

	if err := b.UpdateWeight("a", 0); !errors.Is(err, ErrInvalidWeight) {
		t.Fatalf("UpdateWeight(weight 0) = %v, want ErrInvalidWeight", err)
	}

	// Two picks to build up currentWeight state, ending at 0/0.
	if got := nextSeq(t, b, 2); !equalStrings(got, []string{"a", "b"}) {
		t.Fatalf("pre-update sequence = %v, want [a b]", got)
	}

	if err := b.UpdateWeight("a", 3); err != nil {
		t.Fatalf("UpdateWeight(a, 3) failed: %v", err)
	}
	if got := b.TotalWeight(); got != 4 {
		t.Fatalf("TotalWeight() = %d, want 4", got)
	}
	// Selection continues from the preserved currentWeight under the new
	// weight (verified by hand: a, a, b, a).
	want := []string{"a", "a", "b", "a"}
	if got := nextSeq(t, b, len(want)); !equalStrings(got, want) {
		t.Fatalf("post-update sequence = %v, want %v", got, want)
	}
}

func TestMembersSortedAndIsolated(t *testing.T) {
	b := New()
	mustAdd(t, b, "c", 3)
	mustAdd(t, b, "a", 1)
	mustAdd(t, b, "b", 2)

	members := b.Members()
	want := []Node{{ID: "a", Weight: 1}, {ID: "b", Weight: 2}, {ID: "c", Weight: 3}}
	if len(members) != len(want) {
		t.Fatalf("Members() = %v, want %v", members, want)
	}
	for i := range want {
		if members[i] != want[i] {
			t.Fatalf("Members()[%d] = %v, want %v", i, members[i], want[i])
		}
	}

	// Mutating the returned slice must not affect the balancer.
	members[0].Weight = 999
	members[1].ID = "zzz"
	again := b.Members()
	for i := range want {
		if again[i] != want[i] {
			t.Fatalf("after mutation Members()[%d] = %v, want %v", i, again[i], want[i])
		}
	}
	if got := b.TotalWeight(); got != 6 {
		t.Fatalf("TotalWeight() = %d, want 6", got)
	}
}

func TestLenAndTotalWeight(t *testing.T) {
	b := New()
	if got := b.TotalWeight(); got != 0 {
		t.Fatalf("TotalWeight() = %d, want 0", got)
	}
	mustAdd(t, b, "a", 5)
	mustAdd(t, b, "b", 2)
	if got := b.TotalWeight(); got != 7 {
		t.Fatalf("TotalWeight() = %d, want 7", got)
	}
	if err := b.UpdateWeight("b", 4); err != nil {
		t.Fatalf("UpdateWeight(b, 4) failed: %v", err)
	}
	if got := b.TotalWeight(); got != 9 {
		t.Fatalf("TotalWeight() = %d, want 9", got)
	}
	if err := b.Remove("a"); err != nil {
		t.Fatalf("Remove(a) failed: %v", err)
	}
	if got := b.Len(); got != 1 {
		t.Fatalf("Len() = %d, want 1", got)
	}
	if got := b.TotalWeight(); got != 4 {
		t.Fatalf("TotalWeight() = %d, want 4", got)
	}
}

func TestConcurrentAccess(t *testing.T) {
	b := New()
	for _, id := range []string{"a", "b", "c"} {
		mustAdd(t, b, id, 1)
	}

	var wg sync.WaitGroup
	// Readers calling Next and the query methods.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if _, err := b.Next(); err != nil {
					t.Errorf("Next() failed: %v", err)
				}
				_ = b.Members()
				_ = b.Len()
				_ = b.TotalWeight()
			}
		}()
	}
	// Writers adding, updating, and removing distinct nodes.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				id := fmt.Sprintf("n%d", base*50+j)
				if err := b.Add(id, int64(j%3+1)); err != nil {
					t.Errorf("Add(%q) failed: %v", id, err)
				}
				if err := b.UpdateWeight(id, int64(j%5+1)); err != nil {
					t.Errorf("UpdateWeight(%q) failed: %v", id, err)
				}
				if err := b.Remove(id); err != nil {
					t.Errorf("Remove(%q) failed: %v", id, err)
				}
			}
		}(i)
	}
	wg.Wait()

	if got := b.Len(); got != 3 {
		t.Fatalf("Len() = %d, want 3", got)
	}
	if got := b.TotalWeight(); got != 3 {
		t.Fatalf("TotalWeight() = %d, want 3", got)
	}
}
