// Hand-written Go port of benchmarks/pairing_heap.rb, for the README comparison column.
// The gem (MinPriorityQueue, SimplePairingHeap) is ported inline; panics mark its internal invariant raises.
package main

import "fmt"

const side = 150

// minQueue is the gem's PairingHeap with order <= (MinPriorityQueue).
type node struct {
	elem, priority               int
	subheaps, parent, prev, next *node
}

type minQueue struct {
	root  *node
	nodes map[int]*node
}

func newMinQueue() *minQueue { return &minQueue{nodes: map[int]*node{}} }

func (q *minQueue) meld(left, right *node) *node {
	parent, child := right, left
	if left.priority <= right.priority {
		parent, child = left, right
	}
	child.next = parent.subheaps
	parent.subheaps = child
	if child.next != nil {
		child.next.prev = child
	}
	child.prev = nil
	child.parent = parent
	return parent
}

func (q *minQueue) mergePairs(heaps *node) *node {
	if heaps == nil || heaps.next == nil {
		return heaps
	}
	var pairs *node
	left := heaps
	for left != nil {
		right := left.next
		if right == nil {
			left.next = pairs
			pairs = left
			break
		}
		nextVal := right.next
		right = q.meld(left, right)
		right.next = pairs
		pairs = right
		left = nextVal
	}
	left = pairs
	right := pairs.next
	for right != nil {
		nextVal := right.next
		left = q.meld(left, right)
		right = nextVal
	}
	return left
}

func (q *minQueue) push(elem, priority int) {
	if _, ok := q.nodes[elem]; ok {
		panic("element already in the heap")
	}
	n := &node{elem: elem, priority: priority}
	q.nodes[elem] = n
	if q.root != nil {
		q.root = q.meld(q.root, n)
	} else {
		q.root = n
	}
}

func (q *minQueue) empty() bool { return q.root == nil }

func (q *minQueue) include(elem int) bool {
	_, ok := q.nodes[elem]
	return ok
}

func (q *minQueue) popWithPriority() (elem, priority int) {
	r := q.root
	delete(q.nodes, r.elem)
	q.root = q.mergePairs(r.subheaps)
	if q.root != nil {
		q.root.parent, q.root.next, q.root.prev = nil, nil, nil
	}
	return r.elem, r.priority
}

func (n *node) removeFromParentsList() {
	if n.prev != nil {
		n.prev.next = n.next
		if n.next != nil {
			n.next.prev = n.prev
		}
	} else {
		n.parent.subheaps = n.next
		if n.next != nil {
			n.next.prev = nil
		}
	}
	n.prev, n.next = nil, nil
}

func (q *minQueue) decreaseKey(elem, priority int) {
	n := q.nodes[elem]
	if n == nil {
		panic("provided element is not in heap")
	}
	if priority > n.priority {
		panic("priority cannot be changed to a less prioritary value")
	}
	n.priority = priority
	if n.parent == nil || n.parent.priority <= n.priority {
		return
	}
	n.removeFromParentsList()
	q.root = q.meld(n, q.root)
	q.root.parent = nil
}

// simpleHeap is the gem's SimplePairingHeap with elem == priority.
type snode struct {
	elem           int
	subheaps, next *snode
}

type simpleHeap struct {
	root *snode
	size int
}

func smeld(left, right *snode) *snode {
	parent, child := right, left
	if left.elem <= right.elem {
		parent, child = left, right
	}
	child.next = parent.subheaps
	parent.subheaps = child
	return parent
}

func smergePairs(heaps *snode) *snode {
	if heaps == nil || heaps.next == nil {
		return heaps
	}
	var pairs *snode
	left := heaps
	for left != nil {
		right := left.next
		if right == nil {
			left.next = pairs
			pairs = left
			break
		}
		nextVal := right.next
		right = smeld(left, right)
		right.next = pairs
		pairs = right
		left = nextVal
	}
	left = pairs
	right := pairs.next
	for right != nil {
		nextVal := right.next
		left = smeld(left, right)
		right = nextVal
	}
	return left
}

func (h *simpleHeap) push(elem int) {
	n := &snode{elem: elem}
	if h.root != nil {
		h.root = smeld(h.root, n)
	} else {
		h.root = n
	}
	h.size++
}

func (h *simpleHeap) any() bool { return h.root != nil }

func (h *simpleHeap) pop() int {
	h.size--
	elem := h.root.elem
	h.root = smergePairs(h.root.subheaps)
	if h.root != nil {
		h.root.next = nil
	}
	return elem
}

func neighbors(cell int) []int {
	row, col := cell/side, cell%side
	out := make([]int, 0, 4)
	if row > 0 {
		out = append(out, cell-side)
	}
	if row < side-1 {
		out = append(out, cell+side)
	}
	if col > 0 {
		out = append(out, cell-1)
	}
	if col < side-1 {
		out = append(out, cell+1)
	}
	return out
}

func main() {
	dist := map[int]int{0: 0}
	queue := newMinQueue()
	queue.push(0, 0)
	for !queue.empty() {
		cell, d := queue.popWithPriority()
		for _, n := range neighbors(cell) {
			alt := d + (cell*7919+n*104_729)%97 + 1
			if known, ok := dist[n]; ok && known <= alt {
				continue
			}
			dist[n] = alt
			if queue.include(n) {
				queue.decreaseKey(n, alt)
			} else {
				queue.push(n, alt)
			}
		}
	}
	fmt.Println(dist[side*side-1])
	values := make([]int, 0, len(dist))
	for _, v := range dist {
		values = append(values, v)
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	fmt.Println(sum)

	heap := &simpleHeap{}
	for i := range 100_000 {
		heap.push((i * 7919) % 100_003)
	}
	sorted, prev := 0, -1
	for heap.any() {
		x := heap.pop()
		if x < prev {
			panic("out of order")
		}
		prev = x
		sorted++
	}
	fmt.Println(sorted)
}
