// Hand-written Go port of benchmarks/splay.rb, for the README comparison column.
// The Ruby's unused @tag ivars are dropped.
package main

import "fmt"

type node struct {
	key, value  int
	left, right *node
}

type splayTree struct {
	root *node
	size int
}

func (t *splayTree) insert(key, value int) {
	if t.root == nil {
		t.root = &node{key: key, value: value}
	} else {
		t.splay(key)
		if t.root.key != key {
			n := &node{key: key, value: value}
			if key > t.root.key {
				n.left = t.root
				n.right = t.root.right
				t.root.right = nil
			} else {
				n.right = t.root
				n.left = t.root.left
				t.root.left = nil
			}
			t.root = n
		}
	}
	t.size++
}

func (t *splayTree) find(key int) int {
	if t.root == nil {
		return -1
	}
	t.splay(key)
	return t.root.key
}

func (t *splayTree) splay(key int) {
	dummy := &node{}
	sl, sr := dummy, dummy
	cur := t.root
	for {
		if key < cur.key {
			if cur.left == nil {
				break
			}
			if key < cur.left.key {
				y := cur.left
				cur.left = y.right
				y.right = cur
				cur = y
				if cur.left == nil {
					break
				}
			}
			sr.left = cur
			sr = cur
			cur = cur.left
		} else if key > cur.key {
			if cur.right == nil {
				break
			}
			if key > cur.right.key {
				y := cur.right
				cur.right = y.left
				y.left = cur
				cur = y
				if cur.right == nil {
					break
				}
			}
			sl.right = cur
			sl = cur
			cur = cur.right
		} else {
			break
		}
	}
	sl.right = cur.left
	sr.left = cur.right
	cur.left = dummy.right
	cur.right = dummy.left
	t.root = cur
}

func main() {
	tree := &splayTree{}
	for idx := range 25000 {
		tree.insert((idx*7919)%100000, idx)
	}
	found := 0
	for idx := range 25000 {
		key := (idx * 7919) % 100000
		if tree.find(key) == key {
			found++
		}
	}
	fmt.Println(found)
	fmt.Println("done")
}
