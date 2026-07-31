package fov

// Faithful port of fov_permissive2.c: Jonathon Duerig's enhanced
// precise permissive FOV, with libtcod's 0..8 permissiveness grades.
// Ported from libtcod, BSD 3-Clause License, © 2008-2026 Jice and the
// libtcod contributors. See LICENSE.txt.

const permStepSize = 16

type permLine struct{ xi, yi, xf, yf int }

func (l *permLine) relativeSlope(x, y int) int {
	return (l.yf-l.yi)*(l.xf-x) - (l.xf-l.xi)*(l.yf-y)
}
func (l *permLine) below(x, y int) bool           { return l.relativeSlope(x, y) > 0 }
func (l *permLine) belowOrColinear(x, y int) bool { return l.relativeSlope(x, y) >= 0 }
func (l *permLine) above(x, y int) bool           { return l.relativeSlope(x, y) < 0 }
func (l *permLine) aboveOrColinear(x, y int) bool { return l.relativeSlope(x, y) <= 0 }
func (l *permLine) colinear(x, y int) bool        { return l.relativeSlope(x, y) == 0 }
func (l *permLine) lineColinear(o *permLine) bool {
	return l.colinear(o.xi, o.yi) && l.colinear(o.xf, o.yf)
}

type viewBump struct {
	x, y   int
	parent *viewBump
}

type permView struct {
	shallowLine, steepLine permLine
	shallowBump, steepBump *viewBump
}

type permState struct {
	m           *Map
	povX, povY  int
	views       []permView
	bumps       []viewBump
	bumpCount   int
	activeViews []*permView // active view "array" of pointers
}

func (s *permState) push(v *permView) { s.activeViews = append(s.activeViews, v) }

func (s *permState) remove(index int) {
	s.activeViews = append(s.activeViews[:index], s.activeViews[index+1:]...)
}

func (s *permState) insert(index int, v *permView) {
	s.activeViews = append(s.activeViews, nil)
	copy(s.activeViews[index+1:], s.activeViews[index:])
	s.activeViews[index] = v
}

func (s *permState) isBlocked(x, y, dx, dy int, lightWalls bool) bool {
	posX := x*dx/permStepSize + s.povX
	posY := y*dy/permStepSize + s.povY
	offset := posX + posY*s.m.w
	blocked := !s.m.cells[offset].transparent
	if !blocked || lightWalls {
		s.m.cells[offset].fov = true
	}
	return blocked
}

func (s *permState) addShallowBump(x, y int, view *permView) {
	view.shallowLine.xf = x
	view.shallowLine.yf = y
	shallow := &s.bumps[s.bumpCount]
	s.bumpCount++
	shallow.x, shallow.y = x, y
	shallow.parent = view.shallowBump
	view.shallowBump = shallow
	for cb := view.steepBump; cb != nil; cb = cb.parent {
		if view.shallowLine.above(cb.x, cb.y) {
			view.shallowLine.xi = cb.x
			view.shallowLine.yi = cb.y
		}
	}
}

func (s *permState) addSteepBump(x, y int, view *permView) {
	view.steepLine.xf = x
	view.steepLine.yf = y
	steep := &s.bumps[s.bumpCount]
	s.bumpCount++
	steep.x, steep.y = x, y
	steep.parent = view.steepBump
	view.steepBump = steep
	for cb := view.shallowBump; cb != nil; cb = cb.parent {
		if view.steepLine.below(cb.x, cb.y) {
			view.steepLine.xi = cb.x
			view.steepLine.yi = cb.y
		}
	}
}

// checkView removes degenerate views; index identifies the view in
// activeViews. Returns false if the view was removed.
func (s *permState) checkView(index int, offset, limit int) bool {
	view := s.activeViews[index]
	shallow, steep := &view.shallowLine, &view.steepLine
	if shallow.lineColinear(steep) &&
		(shallow.colinear(offset, limit) || shallow.colinear(limit, offset)) {
		s.remove(index)
		return false
	}
	return true
}

// visitCoords ports visit_coords; currentView is an index into activeViews,
// passed by pointer to mirror the C double-pointer iterator.
func (s *permState) visitCoords(x, y, dx, dy int, currentView *int, lightWalls bool, offset, limit int) {
	tlx, tly := x, y+permStepSize // top left
	brx, bry := x+permStepSize, y // bottom right
	var view *permView
	for *currentView != len(s.activeViews) {
		view = s.activeViews[*currentView]
		if !view.steepLine.belowOrColinear(brx, bry) {
			break
		}
		*currentView++
	}
	if *currentView == len(s.activeViews) || view.shallowLine.aboveOrColinear(tlx, tly) {
		return // no more active view
	}
	if !s.isBlocked(x, y, dx, dy, lightWalls) {
		return
	}
	if view.shallowLine.above(brx, bry) && view.steepLine.below(tlx, tly) {
		// view blocked
		s.remove(*currentView)
	} else if view.shallowLine.above(brx, bry) {
		// shallow bump
		s.addShallowBump(tlx, tly, view)
		s.checkView(*currentView, offset, limit)
	} else if view.steepLine.below(tlx, tly) {
		// steep bump
		s.addSteepBump(brx, bry, view)
		s.checkView(*currentView, offset, limit)
	} else {
		// view split
		viewsOffset := s.povX + x*dx/permStepSize + (s.povY+y*dy/permStepSize)*s.m.w
		shallowerView := &s.views[viewsOffset]
		viewIndex := *currentView
		*shallowerView = *s.activeViews[*currentView]
		s.insert(viewIndex, shallowerView)
		shallowerViewIt := viewIndex
		steeperViewIt := shallowerViewIt + 1
		*currentView = shallowerViewIt
		s.addSteepBump(brx, bry, shallowerView)
		if !s.checkView(shallowerViewIt, offset, limit) {
			steeperViewIt--
		}
		s.addShallowBump(tlx, tly, s.activeViews[steeperViewIt])
		s.checkView(steeperViewIt, offset, limit)
		if viewIndex > len(s.activeViews) {
			*currentView = len(s.activeViews)
		}
	}
}

func (s *permState) checkQuadrant(dx, dy, extentX, extentY int, lightWalls bool, offset, limit int) {
	s.bumpCount = 0
	s.activeViews = s.activeViews[:0]

	shallowLine := permLine{offset, limit, extentX * permStepSize, 0}
	steepLine := permLine{limit, offset, 0, extentY * permStepSize}
	view := &s.views[s.povX+s.povY*s.m.w]
	view.shallowLine = shallowLine
	view.steepLine = steepLine
	view.shallowBump = nil
	view.steepBump = nil
	s.push(view)
	maxI := extentX + extentY
	for i := 1; i <= maxI; i++ {
		if len(s.activeViews) == 0 {
			break
		}
		currentView := 0
		startJ := max(i-extentX, 0)
		maxJ := min(i, extentY)
		for j := startJ; j <= maxJ; j++ {
			if len(s.activeViews) == 0 || currentView == len(s.activeViews) {
				break
			}
			x := (i - j) * permStepSize
			y := j * permStepSize
			s.visitCoords(x, y, dx, dy, &currentView, lightWalls, offset, limit)
		}
	}
}

func (m *Map) permissive(povX, povY, maxRadius int, lightWalls bool, permissiveness int) error {
	offset := 8 - permissiveness
	limit := 8 + permissiveness

	m.cells[povX+povY*m.w].fov = true

	bumpCap := max(len(m.cells), 16)
	s := &permState{
		m: m, povX: povX, povY: povY,
		views:       make([]permView, len(m.cells)),
		bumps:       make([]viewBump, bumpCap),
		activeViews: make([]*permView, 0, len(m.cells)),
	}

	minX, maxX := povX, m.w-povX-1
	minY, maxY := povY, m.h-povY-1
	if maxRadius > 0 {
		minX = min(minX, maxRadius)
		maxX = min(maxX, maxRadius)
		minY = min(minY, maxRadius)
		maxY = min(maxY, maxRadius)
	}
	s.checkQuadrant(1, 1, maxX, maxY, lightWalls, offset, limit)
	s.checkQuadrant(1, -1, maxX, minY, lightWalls, offset, limit)
	s.checkQuadrant(-1, -1, minX, minY, lightWalls, offset, limit)
	s.checkQuadrant(-1, 1, minX, maxY, lightWalls, offset, limit)
	return nil
}
