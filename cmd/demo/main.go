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

	fmt.Println("nodes: a=5, b=1, c=1")
	pick(b, 7)

	if err := b.UpdateWeight("b", 3); err != nil {
		log.Fatal(err)
	}
	fmt.Println("after UpdateWeight(b, 3):")
	pick(b, 4)

	if err := b.Remove("c"); err != nil {
		log.Fatal(err)
	}
	fmt.Println("after Remove(c):")
	pick(b, 4)
}

func pick(b *smoothwrr.Balancer, n int) {
	for i := 0; i < n; i++ {
		id, err := b.Next()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  pick %d: %s\n", i+1, id)
	}
}
