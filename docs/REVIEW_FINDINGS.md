# Code review findings: golibtcod

Review of the pure-Go libtcod port against the C sources at
`github.com/libtcod/libtcod@main` (cloned and read locally, not fetched as
web pages).

Date: 2026-07-30
Baseline at time of review: `go vet ./...` clean, `gofmt -l .` clean,
`go test ./...` all packages passing.

> **Status: all findings below are FIXED** as of session 5 (see
> `BUILDLOG.md`), except the three recorded under "Left alone,
> deliberately" there: the `generateMip` non-power-of-two averaging, the
> negative-diagonal-cost hang (faithful to C), and `BSP.Level`'s type width.
> Every fix has a regression test in the relevant `*/regression_test.go`,
> and the fixture suite is byte-identical to before the changes. The text
> below is kept in the past tense of the review, as the record of what was
> found and how it was verified.

**Every finding below was verified by running code**, not by reading alone.
Where a claim is about C behavior, it was checked against the C source and,
where noted, against a compiled C binary. Findings that turned out to be
wrong on investigation are recorded in "Checked and found correct" so the
same ground is not re-covered later.

---

## Summary

The port is high quality and the fidelity claims in `README.md` and
`docs/BUILDLOG.md` hold up. The fixture suite genuinely replays ground truth
produced by compiled C: the MT19937 seed-5489 stream begins `1351727965`,
the canonical known-answer value, and every count the README quotes matches
what the tests actually print.

Where the port was compared line-by-line against C (console blit, all 13
blend modes, `alpha_blend`, image `Blit`/`Scale`/`MipmapPixel`/
`generateQuadrantGraphic`/`Blit2x`, color HSV, rexpaint layout), it is
faithful, including the deliberately preserved quirks. It is faithful even
in undefined-behaviour corners: `color.Lerp` with an overshooting
coefficient returns 246, matching C's `uint8_t` wraparound exactly.

Consequently **almost every real defect is in argument validation and error
paths**, precisely the surface the golden fixtures were never designed to
reach. The algorithms are sound; the edges are not.

### Priority

| # | Finding | Severity |
| --- | --- | --- |
| 1 | Dijkstra queue overflow; C's 4x allocation cushion dropped | High: crashes ~14% of ordinary inputs |
| 2 | rexpaint unbounded layer dimensions | High: 24 GB alloc from a ~40 byte file |
| 3 | `fov.Permissive(p)` unvalidated, silently runs another algorithm | Medium: silent wrong results |
| 4 | `pngout` glyph coverage; sample renders route identically to walls | Medium: visibly broken quick start |
| 5 | Dice parser divergences, incl. an unbounded roll count | Medium |
| 6 | Nil-map accessors panic | Low to medium |
| 7 | `AStar.Get` / `Dijkstra.Get` panic on empty path | Low to medium |
| 8 | Degenerate-input panics in heightmap | Low |
| 9 | Assorted minor divergences | Low |

---

## 1. Dijkstra queue overflow (High)

**Location:** [`path/path.go:444`](path/path.go#L444) and
[`path/path.go:452`](path/path.go#L452), in `(*Dijkstra).Compute`.
Allocation at [`path/path.go:366-367`](path/path.go#L366) and
[`path/path.go:378-380`](path/path.go#L378).

The pending-queue insertion walk writes `d.nodes[j+1]`, where `j` derives
from `lastIndex - 1`. `lastIndex` is incremented on every insertion and is
never bounded against `len(d.nodes)`, so `j+1` can reach exactly `w*h`,
one past the end of the slice.

### Measured impact

On 8x8 maps with varied costs and **every cell walkable** (no adversarial
input, no blocked cells):

```
varied-cost 8x8 maps, ALL CELLS WALKABLE: 71/500 panicked (14.2%)
example: runtime error: index out of range [64] with length 64
```

The index is exactly one past the end, matching the analysis above.

### Why the fixtures missed it

The fixture generator (`internal/fixtures/gen.c.txt:186`) only exercises
`TCOD_dijkstra_new(m, 1.41f)`, the **map-based** constructor, where every
cost is uniformly 0 or 1. The cost-function constructor is not covered at
all. Verified directly:

| constructor | result over 500 random maps |
| --- | --- |
| `NewDijkstra` (map-based, fixture-covered) | 0/500 panicked |
| `NewDijkstraUsingFunc` (not fixture-covered) | 71/500 panicked |

Uniform costs are safe; the crash needs *varied* costs, which is the entire
reason to use a cost function.

### This is not purely a faithful upstream bug

C's insertion walk has the same unbounded `nodes[j + 1]` write
(`path_c.c:560`, `:564`), so the logic error is upstream. **But C's
cost-function constructor deliberately over-allocates 4x:**

```c
/* path_c.c:473-474, TCOD_dijkstra_new_using_function */
data->distances = malloc(map_width * map_height * sizeof(*data->distances) * 4);
data->nodes     = malloc(map_width * map_height * sizeof(*data->nodes) * 4);
...
data->nodes_max = map_width * map_height;   /* NOT multiplied */
```

That `* 4` is the slack that absorbs the overflow in C. The map-based
constructor (`path_c.c:456-457`) gets no such padding, which is consistent
with the map-based path also being the safe one in Go.

`golibtcod` allocates exactly `n` for **both** constructors, so the port
dropped a mitigation upstream put there on purpose. That is why the
cost-function variant crashes here and does not in C.

### Suggested fix

Either match C's allocation for the func-variant, or (better) bound the
insertion walk so `lastIndex` cannot exceed `len(d.nodes)`. The latter fixes
the actual defect rather than hiding it behind slack memory. If the
preserved-quirk policy argues for keeping C's exact arithmetic, note the
divergence explicitly: reproducing a heap-corruption bug is not a fidelity
win.

### Related, already handled

At [`path/path.go:447-449`](path/path.go#L447) the port adds
`if j < 0 { break }`. In C, `j` is `unsigned`, so `j--` at zero wraps to
`0xFFFFFFFF` and reads wildly. The Go guard is a correct, deliberate
hardening. It is a control-flow divergence from C and worth keeping
documented.

---

## 2. rexpaint: unbounded layer dimensions (High)

**Location:** [`rexpaint/rexpaint.go:85-88`](rexpaint/rexpaint.go#L85), in
`ReadLayers`.

`LayerCount` is sanity-checked (`<= 64`), but `Width` and `Height` are only
checked for `> 0`. There is no upper bound and no cross-check against the
number of bytes actually remaining in the stream. `console.New(w, h)` then
allocates `w*h*16` bytes before a single tile of data is read.

### Measured impact

Crafted `.xp` inputs of roughly 40 bytes:

| claimed layer size | tiles | allocation | observed |
| --- | --- | --- | --- |
| 40000 x 40000 | 1.6e9 | ~23.8 GB | took **26 seconds**, then `EOF` |
| 65536 x 65536 | 4.29e9 | ~64 GB | had to be **killed after 100 s** |

No panic and no memory-safety issue: the failure mode is resource
exhaustion. `.xp` files are exactly the kind of asset loaded from untrusted
or user-supplied sources, so this is worth closing.

### Suggested fix

Bound `Width`/`Height` to something plausible for a console, and/or validate
`width*height*sizeof(tile)` against the remaining input before allocating.
Reading tiles into a pre-sized buffer incrementally would also work.

---

## 3. `fov.Permissive(p)` is unvalidated (Medium)

**Location:** [`fov/fov.go:39`](fov/fov.go#L39).

```go
func Permissive(p int) Algorithm { return Permissive0 + Algorithm(p) }
```

C validates the range and returns `TCOD_E_INVALID_ARGUMENT` for anything
outside `0..8` (`fov_permissive2.c:284-287`). Go performs unchecked
arithmetic, so out-of-range values collide with adjacent enum members.

Enum values are `Basic=0, Diamond=1, Shadow=2, Permissive0=3 ... Permissive8=11,
Restrictive=12, SymmetricShadowcast=13`. Verified:

| call | resulting algorithm | error returned |
| --- | --- | --- |
| `Permissive(-3)` | `Basic` (0) | `nil` |
| `Permissive(-1)` | `Shadow` (2) | `nil` |
| `Permissive(9)` | `Restrictive` (12) | `nil` |
| `Permissive(10)` | `SymmetricShadowcast` (13) | `nil` |
| `Permissive(100)` | 103 | `fov: unknown algorithm 103` |

`Permissive(-1)` produced FOV output **byte-identical to `Shadow`** with a
`nil` error. Only `Permissive(100)` errors, and only because it overflows
past the last defined member.

A caller doing `fov.Permissive(userConfigValue)` gets silently wrong field
of view rather than an error. One-line fix: range-check `p` and return an
invalid sentinel (or have `ComputeFov` reject it).

---

## 4. `present/pngout` glyph coverage (Medium)

**Location:** [`present/pngout/pngout.go:21-92`](present/pngout/pngout.go#L21)
(glyph table), [`present/pngout/pngout.go:114-117`](present/pngout/pngout.go#L114)
(fallback).

The embedded 8x8 font defines 68 glyphs. Of the 26 lowercase letters, only
`d` and `z` are present; the other 24 are missing. Any unmapped rune falls
back to `'#'`:

```go
g, ok := glyphs[rune(cell.Ch)]
if !ok {
    g = glyphs['#'] // visible "missing glyph"
}
```

The problem is that `'#'` is also a **real, commonly used glyph**: it is
what `cmd/sample` draws walls with (`cmd/sample/main.go:106`). So the
"missing glyph" marker is indistinguishable from legitimate content.

### Measured impact

`cmd/sample` draws its A* route with `'*'` (`cmd/sample/main.go:118`), which
is **not** in the glyph table. Verified by rendering both to PNG and
comparing bytes:

```
'*' render identical to '#' render: true
```

In the generated `sample_dungeon.png`, the A* route and the dungeon walls
render as the same glyph. Since `README.md` instructs users to run
`go run ./cmd/sample` as the quick start, this is the most user-visible
defect in the repository.

The status line avoids the issue only because it is written in all caps
(`"GOTCOD SAMPLE  SEED 5EED  SHADOW FOV R%d  ASTAR %d STEPS"`).

### Suggested fix

`pngout` is explicitly a demo-grade software presenter, so a full font is
not required. But adding `'*'` and the
remaining lowercase letters is cheap, and the fallback glyph should be
something that cannot be confused with real content (an empty box, or a
checkerboard) rather than `'#'`.

### Related

`present/pngout` has **no test file** (0.0% coverage) despite being on the
documented quick-start path. `cmd/sample` is likewise untested.

---

## 5. Dice parser divergences (Medium)

**Location:** [`rng/rng.go:369-411`](rng/rng.go#L369), `ParseDice` and
`atoiPrefix`.

`atoiPrefix` is documented as having "atoi semantics" but implements only
"parse leading digits". C's `atoi` additionally accepts leading whitespace
and a `+`/`-` sign, and truncates to 32 bits. Verified Go output:

| input | `Rolls` | `Faces` | note |
| --- | --- | --- | --- |
| `"3d6"` | 3 | 6 | correct |
| `" 3d6"` | **0** | 6 | one leading space, rolls nothing |
| `"-2d6"` | **0** | 6 | sign not accepted |
| `"3d6 "` | 3 | 6 | trailing space fine |
| `"99999999999d6"` | **99999999999** | 6 | no 32-bit truncation |

Two distinct consequences:

**Silent zero rolls.** `Roll` loops `for i := 0; i < d.Rolls`, so
`Rolls == 0` returns 0 with no error. Verified: `Roll(ParseDice(" 3d6"))`
returns **0**. A single stray space in a config file silently disables a
dice expression.

**Unbounded roll count.** C's `atoi` returns a 32-bit `int`, so
`"99999999999d6"` truncates to `1215752191`. Go's `int` is 64-bit and
accumulates without wrapping. Verified: `Roll` on the parsed value was
**still running after 3 seconds**.

### Severity is bounded by the parser layer, but not eliminated

`parser.Value.Dice()` ([`parser/schema.go:372-378`](parser/schema.go#L372))
applies `strings.TrimSpace` and rejects `Rolls <= 0`, so schema-validated
configs are protected from the first two rows. Verified:

| raw value | via `Value.Dice()` |
| --- | --- |
| `" 3d6"` | `{Rolls:3 Faces:6}`, no error (TrimSpace saves it) |
| `"-2d6"` | error: `not dice notation` |
| `"abc"` | error: `not dice notation` |
| `"99999999999d6"` | **`{Rolls:99999999999 Faces:6}`, no error** |

So the hang **does** reach callers through the validated path. Direct
`rng.ParseDice` / `rng.RollS` users are exposed to all of the above.

### Suggested fix

Make `atoiPrefix` actually match `atoi` (whitespace, sign, 32-bit
truncation), or clamp `Rolls`/`Faces` to a sane maximum and have
`Value.Dice()` reject implausible counts. Note that C's own parser has a
stale-buffer quirk (it reuses a 128-byte `tmp` across parse phases and can
advance past the NUL terminator), so exact fidelity here is neither
achievable nor desirable; a documented, bounded divergence is the right
outcome.

---

## 6. Nil-map accessors panic (Low to medium)

**Location:** [`fov/fov.go:59-90`](fov/fov.go#L59).

`NewMap` returns `nil` for non-positive dimensions
([`fov/fov.go:52-56`](fov/fov.go#L52)), but the accessors dereference `m`
unconditionally. C nil-checks in `TCOD_map_get_width`, `get_height`,
`is_transparent`, `is_walkable`, `is_in_fov` and `TCOD_map_clear`, returning
`0`/`false`.

Verified: `fov.NewMap(0, 10)` returns nil, and the subsequent `.Width()`
panics with a nil-pointer dereference. `Height`, `InBounds`,
`IsTransparent`, `IsWalkable`, `InFov`, `SetProperties` and `Clear` behave
the same way.

This is a realistic trap precisely because `NewMap` signals failure by
returning nil: a caller who does not check gets a panic on the next line
rather than a benign zero value.

`ComputeFov` ([`fov/fov.go:94-96`](fov/fov.go#L94)) is the one method that
nil-checks correctly. `Postprocess` does not, but neither does
`TCOD_map_postprocess`, so that one is faithful.

---

## 7. `Get` panics on an empty or exhausted path (Low to medium)

**Location:** [`path/path.go:324-337`](path/path.go#L324) (`AStar.Get`) and
`Dijkstra.Get`, which has the same shape.

`pos := len(p.path) - 1` is `-1` for an empty path, and `p.path[pos]` is
evaluated before any bounds test. Verified:

```
Compute(2,2,2,2) = true, Size = 0
Get(0) on trivial/empty path PANIC: index out of range [-1]

path size = 3
Get(Size()) PANIC: index out of range [-1]
```

The first case is natural: pathfinding to the cell you already occupy
succeeds, reports `Size() == 0`, and then `Get(0)` crashes.

C uses `do { ... } while (index >= 0)`, so it always executes the body once
and calls `TCOD_list_get(list, -1)`, reading before the array. Neither is
correct; the failure modes simply differ. A length guard returning the
origin (or an `ok` bool) would close it.

---

## 8. Degenerate-input panics in heightmap (Low)

Both verified:

- **`MidPointDisplacement` on a 1x1 map.**
  `heightmap.New(1,1).MidPointDisplacement(r, 0.5)` panics with
  `index out of range [-1]`. `initSz = min(W,H)-1 = 0`, so the code indexes
  `Values[sz-1]` = `Values[-1]`. C computes the same index and writes out of
  bounds.
- **`AddVoronoi` with a short coefficient slice.**
  `hm.AddVoronoi(3, 3, []float32{1.0}, r)` panics with
  `index out of range [1] with length 1`. Both languages clamp `nbCoef`
  against `nbPoints` but neither clamps against `len(coef)`; in Go that is a
  guaranteed panic, in C a silent out-of-bounds read.

A length check on `coef` and a minimum-size guard on `MidPointDisplacement`
would close both.

---

## 9. Minor divergences (Low)

- **`SetCharBackground` default-flag test.**
  [`console/console.go:214`](console/console.go#L214) tests
  `flag&0xff == BkgndDefault`; C tests the whole flag
  (`if (flag == TCOD_BKGND_DEFAULT)`). Verified: a flag with low byte 13 and
  a non-zero high byte (e.g. `BkgndDefault | 255<<8`) mutates the tile in Go,
  where C falls through to `default:` and leaves it untouched. **Latent, not
  live**: no public constructor produces such a value (`AddAlpha` and
  `Alpha` yield low bytes 9 and 12 respectively).

- **`generateMip` averaging on non-power-of-two images.**
  [`image/image.go:112-114`](image/image.go#L112) adds a bounds guard C
  lacks, and skipped samples are excluded from `count`. The guard is a
  genuine improvement (C reads out of bounds here), but it changes the
  computed average versus C on non-power-of-two images. Worth a comment
  noting the intentional divergence.

- **`rexpaint.Write` substitutes `' '` for `Ch == 0`**
  ([`rexpaint/rexpaint.go:168-170`](rexpaint/rexpaint.go#L168)); C writes the
  zero codepoint through unchanged.

- **`BSP.Level` is `int`; C's is `uint8_t`** (`bsp.h:51`), which wraps at
  256. Only reachable past tree depth 255, so a fidelity note rather than a
  practical concern.

- **Pre-existing infinite loop, faithful to C.**
  `NewUsingMap(m, -1.0)` followed by `Compute` loops forever in both
  languages (`covered` decreases without bound). Flagged only because a Go
  caller may reasonably expect a bounded call.

---

## Checked and found correct

Recorded so this ground is not re-covered.

**Verified faithful line-by-line against C:** console blit (`blitCell`,
`blitLerp`, all four glyph-resolution cases), all 13 background blend modes
and every `channel_*` function, `alpha_blend` integer arithmetic, image
`Blit` (both fast and rotated paths), `Scale`, `MipmapPixel` including the
`mip--` sharpness quirk, `generateQuadrantGraphic` including every merge
branch, `Blit2x`, `color.SetHSV`, `color.Lerp`, rexpaint column-major
layout and fuchsia key colour, FOV `castRay` / `postprocessQuadrant` /
`castLight` / symmetric shadowcaster, the diamond and MRPAS ports, the
permissive index-rewrite including the view-split path, A* heap operations
/ `Reverse` / `Walk`, and all of `bsp.go`.

**Empirically probed, no defects found:**

- `color.Lerp` overshoot: Go returns 246 for a negative intermediate,
  byte-matching a compiled C binary's `uint8_t` wraparound.
- Markup: round-trips correctly through `R=0` and `R=255`; `MarkupWidth`
  agrees with what `PrintMarkup` actually paints, including truncated RGB
  sequences and channel bytes that collide with the control-code range.
- FOV: all 8 algorithms across 1x1 / 1xN / Nx1 / 2x2 maps at radii 0, 1 and
  100, both `lightWalls` settings. No panics.
- Image: `Blit2x` with negative destinations and oversized sources, `Scale`
  up and down, `Rotate90` with negative and >4 rotations, `MipmapPixel` on a
  non-power-of-two image. No panics.
- BSP: 1x1, 0x0 and 2x100 maps with recursive splitting. No panics.
- Dijkstra: map-based constructor, 500 random walkability patterns. No
  panics, confirming the defect is specific to the cost-function path.

**Deliberately preserved C quirks, all confirmed correctly implemented and
not to be "fixed":** the MRPAS horizontal-edge double-increment, the Perlin
4D trailing-zero lattice calls, the Dijkstra `(int)(cost*100+0.1)` rounding,
the RNG closed-interval float scaling, and the symmetric shadowcaster's
`>= radius^2` rim trim.

---

## Note on the fixture suite

The golden-fixture harness is the strongest asset in this project, and the
Dijkstra defect shows its one blind spot: it covers the *outputs of
algorithms on well-formed inputs* thoroughly, and *argument validation* not
at all.

Adding a cost-function Dijkstra case to `gen.c.txt` would have caught the
highest-severity finding in this review. That said, it needs care: on
inputs that crash the Go port, the C reference corrupts its own heap rather
than producing comparable output, so the fixture would have to assert
"terminates without panic" rather than "matches C".

More generally, the fixtures cannot help with nil handling, out-of-range
enums, or resource limits, because C's answer in those cases is undefined
behaviour rather than a value worth matching. Those need ordinary Go tests.
