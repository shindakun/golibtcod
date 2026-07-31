package bsp

import (
	"testing"

	"golibtcod/rng"
)

func TestSplitOnceGeometry(t *testing.T) {
	n := New(0, 0, 100, 60)
	n.SplitOnce(true, 30) // horizontal split at y=30
	l, r := n.Left(), n.Right()
	if l.X != 0 || l.Y != 0 || l.W != 100 || l.H != 30 {
		t.Fatalf("left = %+v", *l)
	}
	if r.X != 0 || r.Y != 30 || r.W != 100 || r.H != 30 {
		t.Fatalf("right = %+v", *r)
	}
	if l.Level != 1 || r.Level != 1 {
		t.Fatal("child level wrong")
	}
}

func TestSplitRecursivePartitions(t *testing.T) {
	r := rng.New(rng.CMWC, 0xdeadbeef)
	root := New(0, 0, 80, 50)
	root.SplitRecursive(r, 4, 5, 5, 1.5, 1.5)

	// Leaves must exactly tile the root: every cell inside exactly one leaf.
	counts := make([]int, 80*50)
	root.TraversePostOrder(func(n *BSP) bool {
		if n.IsLeaf() {
			if n.W < 5 || n.H < 5 {
				t.Fatalf("leaf smaller than min size: %+v", *n)
			}
			for y := n.Y; y < n.Y+n.H; y++ {
				for x := n.X; x < n.X+n.W; x++ {
					counts[x+y*80]++
				}
			}
		}
		return true
	})
	for i, c := range counts {
		if c != 1 {
			t.Fatalf("cell %d covered %d times", i, c)
		}
	}
}

func TestFindNode(t *testing.T) {
	r := rng.New(rng.MT, 7)
	root := New(0, 0, 64, 64)
	root.SplitRecursive(r, 3, 8, 8, 1.5, 1.5)
	n := root.FindNode(10, 10)
	if n == nil || !n.IsLeaf() || !n.Contains(10, 10) {
		t.Fatalf("FindNode returned %+v", n)
	}
	if root.FindNode(-1, 5) != nil {
		t.Fatal("FindNode out of bounds should be nil")
	}
}

func TestTraversalOrders(t *testing.T) {
	root := New(0, 0, 40, 40)
	root.SplitOnce(false, 20)
	var pre, in, post []int
	root.TraversePreOrder(func(n *BSP) bool { pre = append(pre, n.Level); return true })
	root.TraverseInOrder(func(n *BSP) bool { in = append(in, n.Level); return true })
	root.TraversePostOrder(func(n *BSP) bool { post = append(post, n.Level); return true })
	if len(pre) != 3 || pre[0] != 0 || post[2] != 0 || in[1] != 0 {
		t.Fatalf("orders wrong: pre=%v in=%v post=%v", pre, in, post)
	}
}
