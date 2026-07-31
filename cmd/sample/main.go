// Command sample exercises the whole golibtcod port in one scene:
// rng seeds a BSP dungeon, fov computes a torch radius with recursive
// shadowcasting, path runs A* to the far room, noise adds floor texture,
// console composes the frame with blend modes, and pngout renders it.
package main

import (
	"fmt"
	"os"

	"github.com/shindakun/golibtcod/bsp"
	"github.com/shindakun/golibtcod/color"
	"github.com/shindakun/golibtcod/console"
	"github.com/shindakun/golibtcod/fov"
	"github.com/shindakun/golibtcod/noise"
	"github.com/shindakun/golibtcod/path"
	"github.com/shindakun/golibtcod/present/pngout"
	"github.com/shindakun/golibtcod/present/term"
	"github.com/shindakun/golibtcod/rng"
)

const mapW, mapH = 60, 36

type room struct{ x, y, w, h int }

func main() {
	r := rng.New(rng.CMWC, 0x5EED)

	// --- BSP dungeon ---
	tiles := make([]bool, mapW*mapH) // true = floor
	var rooms []room
	tree := bsp.New(1, 1, mapW-2, mapH-2)
	tree.SplitRecursive(r, 4, 7, 7, 1.5, 1.5)
	tree.TraversePostOrder(func(n *bsp.BSP) bool {
		if !n.IsLeaf() {
			return true
		}
		w := r.GetI(4, max(4, n.W-2))
		h := r.GetI(4, max(4, n.H-2))
		x := n.X + r.GetI(0, max(0, n.W-w-1))
		y := n.Y + r.GetI(0, max(0, n.H-h-1))
		rooms = append(rooms, room{x, y, w, h})
		for cy := y; cy < y+h && cy < mapH; cy++ {
			for cx := x; cx < x+w && cx < mapW; cx++ {
				tiles[cx+cy*mapW] = true
			}
		}
		return true
	})
	// corridors: connect each room to the next with an L
	for i := 1; i < len(rooms); i++ {
		ax, ay := rooms[i-1].x+rooms[i-1].w/2, rooms[i-1].y+rooms[i-1].h/2
		bx, by := rooms[i].x+rooms[i].w/2, rooms[i].y+rooms[i].h/2
		for x := min(ax, bx); x <= max(ax, bx); x++ {
			tiles[x+ay*mapW] = true
		}
		for y := min(ay, by); y <= max(ay, by); y++ {
			tiles[bx+y*mapW] = true
		}
	}

	// --- FOV map ---
	fm := fov.NewMap(mapW, mapH)
	for y := 0; y < mapH; y++ {
		for x := 0; x < mapW; x++ {
			fm.SetProperties(x, y, tiles[x+y*mapW], tiles[x+y*mapW])
		}
	}
	px, py := rooms[0].x+rooms[0].w/2, rooms[0].y+rooms[0].h/2
	last := rooms[len(rooms)-1]
	tx, ty := last.x+last.w/2, last.y+last.h/2
	const torch = 9
	if err := fm.ComputeFov(px, py, torch, true, fov.Shadow); err != nil {
		panic(err)
	}

	// --- A* to the far room ---
	astar := path.NewUsingMap(fm, 1.41)
	havePath := astar.Compute(px, py, tx, ty)

	// --- floor texture noise ---
	nz := noise.New(2, noise.DefaultHurst, noise.DefaultLacunarity, rng.New(rng.CMWC, 0x5EED))

	// --- compose the console ---
	con := console.New(mapW, mapH+2)
	con.SetDefaultBackground(color.Black)
	con.Clear()
	darkWall := color.RGB{R: 30, G: 30, B: 60}
	lightWall := color.RGB{R: 110, G: 95, B: 60}
	darkGround := color.RGB{R: 15, G: 15, B: 30}
	lightGround := color.RGB{R: 160, G: 130, B: 70}

	for y := 0; y < mapH; y++ {
		for x := 0; x < mapW; x++ {
			floor := tiles[x+y*mapW]
			vis := fm.InFov(x, y)
			switch {
			case floor && vis:
				// noise-textured lamplit floor
				v := nz.Get(float32(x)*0.35, float32(y)*0.35) // [-1,1]
				bg := color.Lerp(lightGround, color.Sepia, (v+1)/4)
				con.PutCharEx(x, y, '.', color.MultiplyScalar(bg, 1.3), color.MultiplyScalar(bg, 0.35))
			case floor:
				con.PutCharEx(x, y, ' ', darkGround, darkGround)
			case vis:
				con.PutCharEx(x, y, '#', color.MultiplyScalar(lightWall, 1.2), color.MultiplyScalar(lightWall, 0.5))
			default:
				con.PutCharEx(x, y, ' ', darkWall, color.MultiplyScalar(darkWall, 0.6))
			}
		}
	}
	// path overlay via ADD blend (console blend modes in action)
	if havePath {
		for i := 0; i < astar.Size(); i++ {
			x, y := astar.Get(i)
			con.SetCharBackground(x, y, color.RGB{R: 60, G: 20, B: 20}, console.BkgndAdd)
			if !fm.InFov(x, y) {
				con.SetChar(x, y, '*')
				con.SetCharForeground(x, y, color.RGB{R: 120, G: 60, B: 60})
			}
		}
	}
	con.PutCharEx(px, py, '@', color.White, color.MultiplyScalar(lightGround, 0.35))
	con.PutCharEx(tx, ty, '>', color.Amber, con.CharBackground(tx, ty))

	// status line
	fg := color.LightGrey
	label := fmt.Sprintf("GOTCOD SAMPLE  SEED 5EED  SHADOW FOV R%d  ASTAR %d STEPS", torch, astar.Size())
	con.PrintEx(1, mapH+1, label, &fg, nil, console.AlignLeft, console.BkgndNone)

	if err := pngout.Render(con, "sample_dungeon.png", pngout.Options{Scale: 2, Grain: 0.10, Vignette: 0.25, Seed: 1}); err != nil {
		panic(err)
	}
	ansi, err := os.Create("sample_dungeon.ans")
	if err != nil {
		panic(err)
	}
	defer ansi.Close()
	if err := term.Render(con, ansi, term.Options{}); err != nil {
		panic(err)
	}
	fmt.Println("wrote sample_dungeon.png and sample_dungeon.ans (cat it in a truecolor terminal)")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
