// Hand-written Go port of benchmarks/binary_trees.rb, for the README comparison column.
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

type Node struct{ left, right *Node }

func makeTree(depth int) *Node {
	if depth == 0 {
		return &Node{}
	}
	d := depth - 1
	return &Node{makeTree(d), makeTree(d)}
}

func checkTree(n *Node) int {
	if n.left == nil {
		return 1
	}
	return 1 + checkTree(n.left) + checkTree(n.right)
}

func main() {
	maxDepth := 10
	if len(os.Args) > 1 {
		n, err := strconv.Atoi(os.Args[1])
		if err != nil {
			log.Fatal(err)
		}
		maxDepth = n
	}
	minDepth := 4
	maxDepth = max(maxDepth, minDepth+2)

	fmt.Println(checkTree(makeTree(maxDepth + 1)))

	longLived := makeTree(maxDepth)

	for depth := minDepth; depth <= maxDepth; depth += 2 {
		iterations := 1 << (maxDepth - depth + minDepth)
		check := 0
		for range iterations {
			check += checkTree(makeTree(depth))
		}
		fmt.Println(check)
	}

	fmt.Println(checkTree(longLived))
}
