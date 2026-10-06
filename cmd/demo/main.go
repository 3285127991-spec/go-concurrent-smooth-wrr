// Command demo is a minimal demonstration of the smoothwrr balancer.
package main

import (
	"fmt"
	"log"

	"smoothwrr"
)

func main() {
	b := smoothwrr.New()
	for _, n := range []smoothwrr.Node{
		{ID: "a", Weight: 5},
		{ID: "b", Weight: 1},
		{ID: "c", Weight: 1},
	} {
		if err := b.Add(n.ID, n.Weight); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Println("nodes:", b.Members())
	fmt.Println("smooth round-robin sequence (7 picks):")
	for i := 0; i < 7; i++ {
		id, ok := b.Next()
		if !ok {
			log.Fatal("unexpected empty balancer")
		}
		fmt.Printf("  pick %d -> %s\n", i+1, id)
	}

	if err := b.UpdateWeight("b", 3); err != nil {
		log.Fatal(err)
	}
	fmt.Println("after UpdateWeight(b, 3), total weight:", b.TotalWeight())
	for i := 0; i < 3; i++ {
		id, ok := b.Next()
		if !ok {
			log.Fatal("unexpected empty balancer")
		}
		fmt.Printf("  pick %d -> %s\n", i+1, id)
	}

	if err := b.Remove("c"); err != nil {
		log.Fatal(err)
	}
	fmt.Println("after Remove(c), members:", b.Members())
}
