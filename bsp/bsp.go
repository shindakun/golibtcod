// Package bsp is a faithful Go port of libtcod's bsp_c.c.
//
// Ported from libtcod (github.com/libtcod/libtcod), BSD 3-Clause License,
// Copyright © 2008-2026, Jice and the libtcod contributors.
// See LICENSE.txt at the repository root.
package bsp

import "golibtcod/rng"

// BSP mirrors TCOD_bsp_t. Children are Left (sons) and Right (sons.next).
type BSP struct {
	X, Y, W, H int
	Position   int  // position of the split
	Level      int  // level in the tree
	Horizontal bool // horizontal split?

	father, left, right *BSP
}

func New(x, y, w, h int) *BSP { return &BSP{X: x, Y: y, W: w, H: h} }

func (n *BSP) Left() *BSP   { return n.left }
func (n *BSP) Right() *BSP  { return n.right }
func (n *BSP) Father() *BSP { return n.father }
func (n *BSP) IsLeaf() bool { return n.left == nil }

// RemoveSons is TCOD_bsp_remove_sons.
func (n *BSP) RemoveSons() { n.left, n.right = nil, nil }

func newIntern(father *BSP, left bool) *BSP {
	b := &BSP{}
	if father.Horizontal {
		b.X = father.X
		b.W = father.W
		if left {
			b.Y = father.Y
			b.H = father.Position - b.Y
		} else {
			b.Y = father.Position
			b.H = father.Y + father.H - father.Position
		}
	} else {
		b.Y = father.Y
		b.H = father.H
		if left {
			b.X = father.X
			b.W = father.Position - b.X
		} else {
			b.X = father.Position
			b.W = father.X + father.W - father.Position
		}
	}
	b.Level = father.Level + 1
	b.father = father
	return b
}

// SplitOnce is TCOD_bsp_split_once.
func (n *BSP) SplitOnce(horizontal bool, position int) {
	n.Horizontal = horizontal
	n.Position = position
	n.left = newIntern(n, true)
	n.right = newIntern(n, false)
}

// SplitRecursive is TCOD_bsp_split_recursive. r must not be nil (golibtcod has
// no global default generator by design).
func (n *BSP) SplitRecursive(r *rng.Random, nb, minHSize, minVSize int, maxHRatio, maxVRatio float32) {
	if nb == 0 || (n.W < 2*minHSize && n.H < 2*minVSize) {
		return
	}
	var horiz bool
	// promote square rooms
	if n.H < 2*minVSize || float32(n.W) > float32(n.H)*maxHRatio {
		horiz = false
	} else if n.W < 2*minHSize || float32(n.H) > float32(n.W)*maxVRatio {
		horiz = true
	} else {
		horiz = r.GetInt(0, 1) == 0
	}
	var position int
	if horiz {
		position = r.GetInt(n.Y+minVSize, n.Y+n.H-minVSize)
	} else {
		position = r.GetInt(n.X+minHSize, n.X+n.W-minHSize)
	}
	n.SplitOnce(horiz, position)
	n.left.SplitRecursive(r, nb-1, minHSize, minVSize, maxHRatio, maxVRatio)
	n.right.SplitRecursive(r, nb-1, minHSize, minVSize, maxHRatio, maxVRatio)
}

// Resize is TCOD_bsp_resize.
func (n *BSP) Resize(x, y, w, h int) {
	n.X, n.Y, n.W, n.H = x, y, w, h
	if n.left != nil {
		if n.Horizontal {
			n.left.Resize(x, y, w, n.Position-y)
			n.right.Resize(x, n.Position, w, y+h-n.Position)
		} else {
			n.left.Resize(x, y, n.Position-x, h)
			n.right.Resize(n.Position, y, x+w-n.Position, h)
		}
	}
}

// Contains is TCOD_bsp_contains.
func (n *BSP) Contains(x, y int) bool {
	return x >= n.X && y >= n.Y && x < n.X+n.W && y < n.Y+n.H
}

// FindNode is TCOD_bsp_find_node.
func (n *BSP) FindNode(x, y int) *BSP {
	if !n.Contains(x, y) {
		return nil
	}
	if !n.IsLeaf() {
		if n.left.Contains(x, y) {
			return n.left.FindNode(x, y)
		}
		if n.right.Contains(x, y) {
			return n.right.FindNode(x, y)
		}
	}
	return n
}

/* --- traversal (all five classic orders); listener returns false to stop --- */

func (n *BSP) TraversePreOrder(listener func(*BSP) bool) bool {
	if !listener(n) {
		return false
	}
	if n.left != nil && !n.left.TraversePreOrder(listener) {
		return false
	}
	if n.right != nil && !n.right.TraversePreOrder(listener) {
		return false
	}
	return true
}

func (n *BSP) TraverseInOrder(listener func(*BSP) bool) bool {
	if n.left != nil && !n.left.TraverseInOrder(listener) {
		return false
	}
	if !listener(n) {
		return false
	}
	if n.right != nil && !n.right.TraverseInOrder(listener) {
		return false
	}
	return true
}

func (n *BSP) TraversePostOrder(listener func(*BSP) bool) bool {
	if n.left != nil && !n.left.TraversePostOrder(listener) {
		return false
	}
	if n.right != nil && !n.right.TraversePostOrder(listener) {
		return false
	}
	return listener(n)
}

func (n *BSP) TraverseLevelOrder(listener func(*BSP) bool) bool {
	queue := []*BSP{n}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if node.left != nil {
			queue = append(queue, node.left)
		}
		if node.right != nil {
			queue = append(queue, node.right)
		}
		if !listener(node) {
			return false
		}
	}
	return true
}

func (n *BSP) TraverseInvertedLevelOrder(listener func(*BSP) bool) bool {
	stack1 := []*BSP{n}
	var stack2 []*BSP
	for len(stack1) > 0 {
		node := stack1[0]
		stack1 = stack1[1:]
		stack2 = append(stack2, node)
		if node.left != nil {
			stack1 = append(stack1, node.left)
		}
		if node.right != nil {
			stack1 = append(stack1, node.right)
		}
	}
	for i := len(stack2) - 1; i >= 0; i-- {
		if !listener(stack2[i]) {
			return false
		}
	}
	return true
}
