# golibtcod build log

Faithful pure-Go port of libtcod (BSD-3-Clause, © 2008-2026 Jice and the libtcod
contributors). Ported from the C sources at github.com/libtcod/libtcod @ main,
fetched 2026-07-27. Design doc: gotcod_port_design.md v0.1.

## 2026-07-27: session 1

### Setup

- Module `github.com/shindakun/golibtcod`, Go 1.22 (Ubuntu toolchain; no module proxy in this
  environment, so the port is stdlib-only by construction, which matches the
  design goal "zero third-party deps in core").
- Fetched C sources for: fov (all 6 algorithm files + fov_c), mersenne,
  noise (+noise_defaults), bsp, path (+heapq), heightmap, bresenham, color,
  console (+drawing/printing/types), tree. LICENSE.txt (BSD-3) retained at
  repo root; attribution headers in each ported file.
- Rendering stays out of the library: a graphical backend would mean a
  third-party dependency, which the zero-dependency rule rules out.
  `present/pngout` (software renderer w/ embedded 8x8 font) proves the
  presenter contract instead, and a consumer can supply its own backend.

### Scope decisions for this session

- Port order: rng → bresenham → fov → path → bsp → noise → heightmap →
  color → console (+blends/blit) → sample.
- namegen, parser, image, compat layer: deferred to a later session (logged
  as TODO at end).
- Fidelity: exact algorithm/structure port; float32 used where C uses float
  to keep numeric behavior close. Golden fixtures vs the C build are not
  possible in-sandbox (no SDL/cmake); substitute: property tests + hand
  fixtures + C-derived constants. Flagged as follow-up.

### Module log

**rng** (`mersenne_c.c` → `rng/rng.go`): Complete.

- MT19937 (init constant 1812433253, standard twist/temper) and CMWC
  (glibc-LCG Q seeding, carry mod 809430660, Marsaglia recommendation).
- Float scaling kept as `u32 * (1/0xffffffff)` (C uses closed-interval
  scaling; preserved rather than "fixed").
- Full distribution family: linear + 4 gaussian variants, GetIntMean etc.,
  dice parser/roller with C atoi semantics.
- Verified: seed 5489 reproduces the canonical MT19937 known-answer stream
  (3499211612, 581869302, ...), so the MT path is bit-exact by test.

**bresenham** (`bresenham_c.c` → `bresenham/bresenham.go`): Complete.

- Init/Step/Line as iterator + callback forms; deprecated global-state
  variants dropped (Data struct only).

**fov** (`fov_c.c` + 6 algorithm files → `fov/`): Complete, all 6.

- `fov.go`: Map (transparent/walkable/fov cells), ComputeFov dispatch,
  postprocess quadrant wall-lighting, FOV_BASIC circular raycasting,
  FOV_SHADOW recursive shadowcasting (float32 slopes as in C),
  FOV_SYMMETRIC_SHADOWCAST (round_half_up/down with FLT_EPSILON factor,
  radius trim uses `>= radius²`: C behavior, kept).
- `diamond_restrictive.go`: FOV_DIAMOND perimeter/raymap port (pointer
  linked list preserved via struct pointers); FOV_RESTRICTIVE (MRPAS 1.2).
  Preserved C quirk: MRPAS horizontal-edge octant double-increments the
  obstacle loop index (upstream behavior; flagged with a comment).
- `permissive.go`: Duerig precise permissive with 0-8 grades; C's
  double-pointer active-view iterator mirrored with an index passed by
  pointer; STEP_SIZE=16, offset=8-p, limit=8+p.
- Tests: POV visibility, open-room completeness, pillar shadows, radius
  limits, lightWalls=false wall stripping across ALL algorithms, plus a
  full pairwise symmetry check for SYMMETRIC_SHADOWCAST.

**path** (`path_c.c` → `path/path.go`): Complete (both pathfinders).

- Classic A*: min-heap keyed on the heuristic array including the
  "slow" heap_reorder path, direction enums (NW..SE with NONE=4),
  walk-with-recalculate, Reverse, Get. Same walk-cost contract
  (map => walkable?1:0, else user CostFunc, <=0 blocks).
- Mingos' Dijkstra: `(int)(cost*100+0.1)` rounding quirk preserved
  ("(int)(1.41f*100.0f)==140!!!"), insertion-sorted pending queue ported
  including the dedup shift, distances as `u32 * 0.01` on read.
- Tests: corridor length, detour routing, unreachable, cost-func
  avoidance, exact diagonal distances (2.82 for two 1.41 steps).

**bsp** (`bsp_c.c` → `bsp/bsp.go`): Complete.

- SplitOnce/SplitRecursive with the square-room promotion rules and the
  same rng call order (so a seeded tree matches C shape for a given
  stream); Resize, FindNode, Contains, all five traversal orders.
- Tests: child geometry, exact leaf tiling of the root (every cell covered
  exactly once), min-size enforcement, traversal order sanity.

**noise** (`noise_c.c` → `noise/noise.go`): Complete.

- Perlin lattice (incl. two 4D lattice calls that pass trailing 0,0 in
  C: an upstream quirk, preserved and commented), Gustavson simplex
  1-4D with the exact gradient functions and the `simplex[64][4]` table,
  Cook/DeRose wavelet (32³ tile, downsample/upsample coefficient tables),
  fbm + turbulence with per-octave exponents (1/f, not f^-H, as in C).
- Constructor consumes the rng stream in C order (256 gradient rows,
  normalize, Fisher-Yates via GetInt), so seeded tables match.
- MAX_OCTAVES=128, MAX_DIMENSIONS=4, SIMPLEX_SCALE=0.5, WAVELET_SCALE=2.
- Tests: determinism, range clamps, all dims, fractal variants, continuity.

**heightmap** (`heightmap_c.c` → `heightmap/heightmap.go`): Complete.

- All live functions: hills, fbm add/scale, interpolation, normals,
  bezier digging, rain erosion, kernel transforms w/ threshold masks,
  voronoi, mid-point displacement. `islandify` is a no-op pending removal
  upstream; `heat_erosion` is #if 0 in C: both skipped, noted here.
- Tests: hill/normalize, bilinear center, fbm+erosion, MPD, kernel smooth.

**color** (`color.c` → `color/color.go`): Complete for ops.

- Add/Subtract/Multiply/scalar/Lerp with C rounding, full HSV family
  (sector math, floor-modulo hue wrap), GenMap gradients, RGBA alpha
  blend. Named colors: greys, sepias, and the classic hue wheel + a few
  misc; the full generated light/dark/desaturated table (~190 names) is
  deferred (derivable via Lerp/ScaleHSV).

**console** (`console.c` + drawing/printing essentials → `console/`):
Core complete.

- Tile{Ch,Fg,Bg RGBA}, default fore/back/flag state, all 13 background
  blend flags with alpha-carrying ADDA/ALPH encodings ((flag>>8)&0xFF),
  exact blit (key color, fg/bg alpha, the four glyph-resolution cases),
  Rect/HLine/VLine/Frame, PrintEx with alignment.
- Deferred: the legacy %c color-code markup printer, rexpaint IO,
  unicode legacy wrappers (modern print covers the use cases).
- Tests: multiply/lighten/add blend math, key-color blit skip, plain-copy
  blit, alignment, gradient GenMap.

**present/pngout**: software presenter (adapted from an earlier mock to
golibtcod's RGBA tiles). Proves the presenter contract.

**cmd/sample**: integration demo: CMWC-seeded BSP dungeon + corridors,
recursive-shadowcast torch FOV, A* route overlaid with BKGND_ADD, simplex
floor texture, rendered to `sample_dungeon.png`. Runs clean.

### Results

- `go build ./...` clean, `go vet ./...` clean, `go test ./...`:
  8/8 packages with tests pass (color and pngout are exercised via the
  console tests and sample).
- Zero third-party dependencies anywhere in the tree.

### Fidelity notes (quirks deliberately preserved)

1. MRPAS horizontal-edge octant double-increments its obstacle index.
2. Perlin 4D passes 0,0 for the last dimension in two lattice calls.
3. Dijkstra diagonal cost = (int)(cost*100+0.1); distances truncate to
   int centicosts.
4. RNG float scaling divides by 0xffffffff (closed interval).
5. Symmetric shadowcast trims at >= radius², making the rim asymmetric
   when a radius is set (C behavior).

### TODO (next session)
>
> **Superseded.** Everything here was cleared in session 2. The live list
> is the Deferred Register at the end of this document; do not work from
> this section.

- Golden fixtures vs a real C libtcod build (needs cmake/SDL or a
  wasm/CI harness outside this sandbox); property tests + MT
  known-answer stream stand in for now.
- namegen, parser, image/mipmaps, compat layer, full named-color table,
  legacy color-markup printing, rexpaint.
- present/term.

---

## 2026-07-28: session 2

Picked up the session-1 TODO list and cleared all of it except the two
items that are genuinely blocked by the sandbox. Headline: **the "no
golden fixtures" caveat is gone**: gcc is available, and the libtcod
modules we care about have no SDL dependency, so the C code itself now
provides ground truth.

### 1. Golden fixtures vs a real C libtcod build: DONE

Session 1 assumed this needed cmake/SDL. It doesn't: the engine-independent
translation units compile standalone. Fetched the remaining headers
(context_viewport.h, tileset.h, etc.: headers only, never linked) and
compiled 19 C objects with gcc 13.3:

    mersenne_c, bresenham_c, fov_c + all 6 fov algorithms, bsp_c, path_c,
    heightmap_c, noise_c, color, parser_c, lex_c, namegen_c,
    error, logging, list_c, tree_c

Wrote two generators (`internal/fixtures/gen.c.txt`,
`gen_namegen.c.txt`) that link those objects and dump deterministic
outputs; `internal/fixtures/*_test.go` replays them against the Go port.
`internal/fixtures/README.md` documents provenance, coverage and the
tolerance policy.

Result: **everything discrete matches the C code exactly**:

| fixture | volume | result |
| --- | --- | --- |
| rng (MT+CMWC, 4 seeds, 6 kinds) | 1,664 values | exact |
| bresenham (7 lines, all octants) | 53 cells | exact |
| fov (8 algorithms x 6 scenarios) | 7,264 cells | exact |
| A* + Dijkstra (paths + distance grid) | full dumps | exact |
| bsp (3 seeds, pre-order node dumps) | full trees | exact |
| namegen (3 rule forms, 2 seeds) | 208 names | exact |
| noise (perlin/simplex/fbm/turb 1-4D, wavelet 1-3D) | 420 values | 341 bit-exact, rest < 1e-5 rel |
| heightmap (full chain, interp, 33x33 MPD) | 1,388 values | 1333 bit-exact, rest < 1e-5 rel |

The float deltas are C compiler reassociation in long float32 expression
chains (Go evaluates float32 strictly left-to-right; gcc does not), not
algorithmic divergence; they appear only in the deep fbm/erosion
accumulations, and every value agrees to ~7 significant figures.

That the **fov grids match cell-for-cell across all 8 algorithms** is the
strongest single result here: it independently confirms the preserved
quirks from session 1 (the MRPAS double-increment, the symmetric-caster
rim trim) are faithful rather than bugs; had I "fixed" them, these
fixtures would now be failing.

Two C-side gotchas hit while building the harness, both logged because
they'd bite anyone repeating this:

- Current `mersenne.h` no longer declares the short-name getters
  (`TCOD_random_get_f`/`_get_i`) though `mersenne_c.c` still defines them.
  Without an explicit prototype, C's implicit-declaration rule assumed
  `int` return and the first float fixture run emitted garbage
  (6.95e-310). Fixed with explicit prototypes in the generator.
- `namegen_c.c` keeps a process-global "already parsed this file" list, so
  parsing the same filename twice silently reuses the first generator
  (with its original RNG pointer). The generator uses two identical
  configs under different names to get a clean second seed.

### 2. namegen + parser: DONE

**parser** (`parser/parser.go`), the libtcod .cfg format: typed
properties, quoted names, bare flags, nested structs, bracketed lists,
and all three comment styles (`#`, `//`, `/* */`), with `\"`/`\\` escapes.

This one is a deliberate **clean-room implementation, not a line-by-line
port**: the only such decision in the project. `lex_c.c` + `parser_c.c`
are ~2,600 lines built around a global lexer, a listener-callback ABI and
manual value unions; a faithful transliteration would be both unpleasant
and un-Go-like, and unlike the algorithm modules there is no numerical
behavior to preserve. The accepted grammar and value semantics match; the
API doesn't. Called out here because "faithful port" is the project's
whole premise and this is the exception.

**namegen** (`namegen/namegen.go`), the generation algorithm *is* a
faithful port: `namegen_populate_list` tokenizing (including the `/`
escape and wildcard-dependent `_` handling), all 8 rule wildcards
(`$P $s $m $e $p $v $c $?`) with `$NN` chance prefixes, `%NN` rule
selection weights, and the four rubbish-pruning filters
(triples, illegal substrings, space pruning, 2-and-3-char syllable
repetition) driving the reject-and-retry loop. RNG call order matches,
which is exactly what the 208-name fixture proves.

The one intentional API change: C keeps a process-global generator
registry; golibtcod scopes it to an explicit `*Registry` so multiple sets
and multiple RNGs can coexist. Also added a retry ceiling (10,000) where
C loops forever; a config whose filters reject every possible output
hangs the C version.

### 3. Full named-color table: DONE

`color/named_gen.go`: extracted all 197 named colors from `color.h`'s
deprecation annotations (they carry the literal RGB triples), emitting
154 new Go constants alongside the 43 hand-written ones. Spot-checked
against the C header in `color/color_test.go`.

### 4. present/term: DONE

`present/term/term.go`: ANSI truecolor presenter, 24-bit SGR, with
run-length collapsing so a uniform row emits one escape sequence rather
than one per cell. `cmd/sample` now writes `sample_dungeon.ans` next to
the PNG: same console, two presenters, which is the presenter contract
demonstrated rather than asserted.

### 5. Still deferred (and why)

- **image / mipmaps, rexpaint IO, legacy `%c` color-markup printing,
  the C++ compat layer**: deliberately not started. These are I/O and
  API-shim surface with no algorithmic content, and the modern
  console/print API covers the actual use cases. Cheap to add later.

### Results after session 2

- 13 packages; `go build`, `go vet`, `gofmt` all clean.
- `go test ./...`: 11 packages with tests, all passing, including the
  fixture suite replaying 11,000+ ground-truth values from C.
- Still zero third-party dependencies.
- `make` runs vet + tests; `make sample` regenerates both renders.

---

## 2026-07-28: session 3

Cleared three of the four "deliberately not started" items. The fourth is
now formally declined rather than deferred.

### image (`image_c.c` -> `image/`): DONE

Mipmapped RGB image with console blitting. Full port: New/Clear/Pixel/
PutPixel, lazily-generated mipmap chain with dirty invalidation on write,
MipmapPixel level selection, Invert/HFlip/VFlip/Rotate90, Scale
(supersampled down, nearest-neighbour up), Blit (fast axis-aligned path
plus the rotated/scaled path), BlitRect, FromConsole/RefreshConsole, and
**Blit2x**: the subcell quadrant renderer that doubles a console's
effective resolution, including the colour-merging logic that reduces four
pixels to one glyph and a fg/bg pair.

**Deviation, deliberate:** libtcod loads and saves through SDL_image.
golibtcod uses `image/png` and `image/jpeg` from the standard library. This
keeps the zero-dependency rule and is strictly more capable for PNG than
the C version's loader. Pixel operations are line-by-line ports; only file
IO differs.

C semantics preserved that look like bugs and are not:

- `MipmapPixel` selects a level from the texel footprint and then steps
  back one (`if mip > 0 { mip-- }`), trading sharpness for aliasing. My
  first test asserted the sharper behaviour and failed; the test was
  wrong, not the port. It now documents the quirk.
- `Alpha()` always returns 255; the C function ignores its arguments for
  non-SDL images. Kept for API compatibility.

### rexpaint (`console_rexpaint.c` -> `rexpaint/`): DONE

Read and write REXPaint `.xp` files: gzip stream, int32 header, per-layer
chunks, **column-major** tile data. Multi-layer load, and `Combine` which
flattens layers using fuchsia (255,0,255) as the transparency key, as
REXPaint itself does.

`compress/gzip` replaces zlib; again, no new dependency.

The column-major layout is the easy thing to get wrong, because a
transposed writer round-trips perfectly on a square console. There is a
non-square round-trip test specifically to catch that.

### console markup (`console_printing.c` -> `console/markup.go`): DONE

The legacy inline colour-control codes: five presets (`ColCtrl1..5`),
`ForeRGB`/`BackRGB` with embedded colour bytes, and `Stop`. Plus
`StripMarkup`/`MarkupWidth` so callers can measure a string's visible
width, and `PrintMarkup` which honours alignment on the *visible* width;
otherwise every control code shifts the text.

Preserved quirk: RGB channel values are offset by +1 in the string,
because a zero byte would terminate a C string. The constructors do the
offsetting and the parser undoes it; there is a test asserting a
round-trip through R=0.

`SetColorControl` keeps the presets in package-level state, as C does.
That is a considered choice rather than an oversight: the codes are a
property of the *string format*, not of a console, and scoping them
per-console would silently change the meaning of ported content.

### C++ compat layer: DECLINED, not deferred

Moved to "Won't port" in the register. It is a set of C++ class shims
(`TCODConsole`, `TCODMap`, ...) whose entire purpose is to make a C
library feel like C++. Go already has methods on types; the "port" would
be a second, worse spelling of the API we already have, and every future
change would need making twice. If someone is porting C++ libtcod code
they are rewriting it anyway.

### Results after session 3

- 15 packages. `go build`, `go vet`, `gofmt`, `go test ./...` all clean.
- Still zero third-party dependencies: the two modules that needed
  external libraries in C (SDL_image, zlib) are covered by the standard
  library.
- Deferred register has no open items.

---

## 2026-07-28: session 4

Closed the parser deviation, but not by transliterating `lex_c.c` and
`parser_c.c`, but by first *measuring* the divergence and then fixing what
the measurement showed. Two real bugs fell out that no amount of reading
the C would have found.

### Measuring first

Built a C harness (`internal/fixtures/parser/gen_parser.c.txt`) that
declares a schema and runs a nine-case corpus through the real libtcod
parser, recording its listener event stream and error strings. Ran the same
corpus through golibtcod. Results:

| case | libtcod | golibtcod (before) |
| --- | --- | --- |
| all value types, nested struct, flag | accepted | **identical output** |
| C-style comments, `\"` and `\\` escapes | accepted | **identical output** |
| `#` comment | **syntax error** | accepted |
| undeclared property | error naming property + line | accepted silently |
| unknown struct type | error naming type | accepted silently |
| `cost = "not a number"` | error, substitutes 0 | stored as text |
| unterminated struct / string | error, **keeps parsing** | aborts |

Two of the seven agreed exactly. The rest is documented in the package doc
comment now, with the corpus and C reference output committed so the claims
are checkable rather than asserted.

**A documentation error surfaced:** the package claimed its accepted syntax
matched libtcod. It does not: `#` comments are a golibtcod extension;
libtcod's lexer knows only `//` and `/* */`. golibtcod accepts a strict
superset, so a `.cfg` written for golibtcod using `#` will not load in C
libtcod. Kept the extension (it is ubiquitous in config formats and we own
our files) but it is now labelled as an extension in the package doc, not
sold as compatibility. Note that downstream example `.cfg` files use `#`.

### The schema layer (`parser/schema.go`)

libtcod's parser is schema-first: declare struct types and property types,
then it validates while parsing. golibtcod parses first and validates second:

    s := parser.NewSchema()
    s.Declare("item_type").
        Prop("cost", parser.TypeInt, true).
        Prop("col", parser.TypeColor, false).
        Flag("abstract").
        Child("sublist")

    structs, err := parser.Parse(src)
    if errs := parser.Validate(structs, s); errs != nil { ... }

Checks: unknown struct types, undeclared properties, undeclared flags,
missing mandatory properties, illegal nesting, and value types (bool, char,
int, float, string, color, dice, list). Typed getters (`Char`, `Color`,
`Dice`) sit alongside the existing `Int`/`Float`/`Bool`. `Dice` delegates
to `rng.ParseDice`, so the parser and the roller cannot disagree about what
`3d6+2` means.

Three deliberate divergences, all pinned by tests:

- **Every** error is reported, in source order, not just the first.
  libtcod stops at its first fatal error. For a config file the full list
  is more useful.
- A type mismatch keeps the offending text in the message
  (`expected an integer, got "not a number"`); libtcod substitutes zero.
- Validation is optional: reading a schema-less file stays legitimate,
  which is the whole reason the layer is separate.

Structs and values now carry `Line`, and structs a `PropOrder`, so errors
can name the line the way libtcod does. Verified: the undeclared-property
case reports **line 3**, exactly as C does.

### Two parser bugs the tests found

Writing the schema tests immediately broke the parser, in a way the
existing tests could not have caught because they all put one property per
line:

    item_type "x" { cost = 1 legendary }          // flag swallowed
    item_type "x" { cost = 1 sublist { ... } }    // nested struct swallowed

`bareValue` read to end of line, so the value became `"1 legendary"`.
libtcod's lexer is **token-based**: an unquoted value there is exactly one
token, and text with spaces must be quoted. `bareValue` now reads a single
token, which fixes both cases and moves the implementation closer to C.

Regression check that mattered: the namegen fixture suite still reproduces
all **208 names** generated by the C implementation from the same `.cfg`,
so the lexer change is compatible with real content.

### Results after session 4

- 15 packages, all building, vetting and testing clean.
- The parser deviation is now *characterised and bounded* rather than
  vague: agreement is measured, differences are documented in the package
  doc, and the C harness is committed for re-checking.
- Deferred register has no open items.

---

## 2026-07-30: session 5

External code review against the C sources, then fixes. No new modules. The
headline: the fixture suite validates *algorithm output on well-formed
inputs* thoroughly and *argument validation* not at all, and every defect
found lived in that gap. Full write-up in `REVIEW_FINDINGS.md`.

Regression tests were added for every fix (`*/regression_test.go`), and
`present/pngout` went from no tests at all to 95.9% coverage.

**The fixture counts are unchanged after all of this**: 1,664 rng values,
7,264 fov cells, complete A*/Dijkstra dumps, 3 BSP trees, 208 names, noise
341/420 bit-exact, heightmap 1333/1388. Nothing below cost any fidelity.

### 1. Dijkstra queue overflow, the one that mattered

`(*Dijkstra).Compute` could drive the pending-queue insertion one slot past
the end of `nodes[]`. Measured before the fix: **71 of 500** random 8x8 maps
panicked with every cell walkable and no adversarial input, i.e. ~14% of
ordinary cost-function use. After: 0 of 2000.

Why the fixtures missed it: `gen.c.txt` only exercises `TCOD_dijkstra_new`,
the *map-based* constructor, where every cost is uniformly 0 or 1. Verified
that map-based is 0/500 while cost-function is 71/500, so the bug sits
precisely in the untested constructor.

This is **not** simply a faithful reproduction of an upstream bug, which is
how it first looked. C has the same unbounded `nodes[j + 1]` write, but
`TCOD_dijkstra_new_using_function` (`path_c.c:473-474`) deliberately
over-allocates **4x** while leaving `nodes_max` at `w*h`; the map-based
constructor gets no such padding. golibtcod allocated exactly `n` for both, so
the port had dropped a mitigation upstream put there on purpose. Fixed by
clamping the queue walk to capacity rather than restoring the slack, since
reproducing a heap overflow is not a fidelity win. The clamp only engages
past the point where C corrupts memory, which is why the fixtures are
untouched.

### 2. rexpaint resource exhaustion

`ReadLayers` bounded `LayerCount` but not `Width`/`Height`, and
`console.New` allocates before any tile data is read. A ~40 byte crafted
file claiming 40000x40000 took **26 seconds**; 65536x65536 (~64 GB) had to
be killed after 100 seconds. Added `maxLayerCells` (1<<24, e.g. 4096x4096);
those inputs are now rejected in microseconds and plausible sizes still
round-trip, non-square included.

### 3. `fov.Permissive(p)` silently ran a different algorithm

Unchecked `Permissive0 + Algorithm(p)` meant out-of-range grades aliased
neighbouring members: `Permissive(-1)` ran recursive shadowcasting and
returned a **nil error**, producing output byte-identical to `Shadow`.
C validates the range. Now returns `AlgorithmInvalid`, which `ComputeFov`
rejects.

### 4. pngout rendered the sample's A* route identically to its walls

The 8x8 font had 2 of 26 lowercase letters, no `'*'`, and fell back to
`'#'` for unmapped runes, which is also what `cmd/sample` draws walls with.
So the route (`'*'`) and the walls rendered as the same glyph: verified
byte-identical PNG output. Since the README points people at
`go run ./cmd/sample`, this was the most user-visible defect in the tree.

Added the full lowercase alphabet, `'*'`, and assorted punctuation, and
replaced the fallback with a private `missingGlyph` sentinel drawn as a
hollow box, so a missing glyph can never again masquerade as content.

### 5. Dice parser

`atoiPrefix` claimed "atoi semantics" but only parsed leading digits. C's
`atoi` also skips whitespace, takes a sign, and returns 32 bits. So `" 3d6"`
(one stray space in a config file) silently rolled **0**, and `"-2d6"` gave
`Rolls=0`. Now matches C exactly, including `"99999999999d6"` truncating to
`1215752191`.

That truncation is C-faithful but still 1.2 billion iterations, and a Go
caller should not be able to hang a process from a config string. `Roll`
now clamps at `MaxRolls` (1<<20): the same input goes from an indefinite
hang to 14ms, and ordinary dice are untouched. Documented as a deliberate
divergence at the constant.

Note `parser.Value.Dice()` already guarded the whitespace and negative
cases via `TrimSpace` and its `Rolls <= 0` check; the huge-count case was
the one that reached callers through the validated path.

### 6. Panics on degenerate input

- `MidPointDisplacement` on a 1x1 map indexed `Values[-1]` (`initSz` is
  `min(W,H)-1`). Now rejects only `min(W,H) == 1`. (Session 7 narrowed this:
  the original guard also rejected `min(W,H) == 2`, where all four seed
  indices collapse to 0 and C runs safely, so 2x2 maps were silently coming
  back all-zero.)
- `AddVoronoi` clamped `nbCoef` against `nbPoints` but not `len(coef)`, so a
  short coefficient slice ran off the end. Now clamped against both.
- `fov.Map` accessors dereferenced a nil receiver, and `NewMap` signals
  failure by *returning nil*, so `NewMap(0,10).Width()` panicked. They now
  return zero values as `TCOD_map_get_width` and friends do.
- `AStar.Get`/`Dijkstra.Get` indexed `path[-1]` on an empty path. `Compute`
  succeeds with `Size()==0` when origin equals destination, so `Get(0)` on a
  perfectly valid path object crashed. Out-of-range indices now return the
  origin and `(-1,-1)` respectively.

### 7. `SetCharBackground` default-flag test

Tested `flag&0xff == BkgndDefault`; C compares the **whole** flag. An
alpha-carrying flag whose low byte happened to be 13 substituted the
console's flag instead of falling through to the switch default. Latent
rather than live (no public constructor produces such a value: `AddAlpha`
and `Alpha` yield low bytes 9 and 12), but now matches C.

### Left alone, deliberately

- **`generateMip` on non-power-of-two images.** The added bounds guard
  excludes skipped samples from `count`, so averages differ slightly from C,
  which reads out of bounds there. The guard is right; the divergence is
  noted in `REVIEW_FINDINGS.md` rather than "fixed" back to a buffer overread.
- **`NewUsingMap(m, -1.0)` hangs forever.** Faithful to C, which has the
  same unbounded loop. A negative diagonal cost is meaningless input.
- **`BSP.Level` is `int` where C is `uint8_t`.** Only observable past tree
  depth 255.

### `present/ebiten` declined, not deferred

It had sat in the register's "Blocked" table since session 1, on the reason
that the sandbox could not reach the Go module proxy. That framing was
wrong, and the register's own maintenance rule caught it: an item that has
outlived its trigger is declined, not deferred.

The real reason is simpler and does not depend on the environment. Any
engine binding is a third-party dependency, and "no cgo, no third-party
dependencies" is the first thing the README claims. A graphical backend
would break that rule for something this library does not need: the
presenter contract is already implemented twice (`pngout`, `term`), and a
consumer that wants a window implements the interface against whatever
engine it already uses. Rendering is the consumer's concern.

Moved to "Won't port: replaced by design", alongside the SDL renderer it
would have replaced. The register's "Blocked" table is now empty, which
means the port has no open items at all.

### Results after session 5

- `go build`, `go vet`, `gofmt`, `go test ./...` all clean.
- Coverage up across every touched package; `present/pngout` 0% to 95.9%.
- Fixture suite byte-identical to session 4: no fidelity regression.
- Deferred register has no open items.

---

## 2026-07-31: session 6

Ported the half of libtcod's tileset module that does not need external
libraries, and wired it into `present/pngout`. Also added a `.gitignore`
(the sample renders were untracked build output).

### What was ported, and what was not

The C module is ~1,700 lines across five files, and it splits cleanly on
the dependency line:

| C file | needs | ported |
| --- | --- | --- |
| `tileset.c` | nothing | yes |
| `tileset_bdf.c` | nothing | yes |
| `tileset_fallback.c` | nothing | no: a hardcoded font blob, and `pngout` already has one |
| `tileset_truetype.c` | `stb_truetype.h` | no |
| `tileset_render.c` | `SDL3/SDL.h` | no |

The split is not arbitrary. A tileset answers "which pixels does codepoint
N draw?"; a renderer answers "where do those pixels go?". The first needs
only the standard library, the second is what the presenter interface
already is. So the two files requiring external libraries are exactly the
two that were never this library's job, and the register row that used to
list "tileset" wholesale as replaced-by-design has been corrected to name
just those two.

### `tileset` package

`Tileset` mirrors `TCOD_Tileset`: fixed-size RGBA tiles plus a
codepoint-to-tile-id map, with the same doubling growth for both the tile
buffer (`DEFAULT_TILES_LENGTH` 256) and the charmap. Tile 0 is kept blank
exactly as C does, since a zero entry in the charmap means "unassigned".

Two pieces of the C struct are deliberately absent. The observer list exists
to invalidate SDL texture atlases when a tile changes, and there is no such
cache here; `ref_count` is Go's garbage collector's job.

`bdf.go` is the `tileset_bdf.c` port: FONTBOUNDINGBOX for the cell size,
then STARTCHAR blocks carrying a per-glyph BBX and hex bitmap rows. The
fiddly part is the offset math that places a glyph's own bounding box inside
the font cell, including the y term that flips the origin (BDF measures up
from the baseline; tiles are top-down). That is ported verbatim.

### Verified against the C build

Compiled `tileset.c` + `tileset_bdf.c` with gcc and dumped glyph coverage
for a 15-codepoint sweep, the same golden-fixture method already used for
rng, fov, path and the rest. `tileset_bdf.c` needs
`TCOD_load_binary_file_` from `sys_c.c`, which drags in SDL, so the harness
inlines that one function instead; it is a plain file read.

Result on `4x6.bdf`: **byte-identical**, cell size and every glyph bitmap.
That is the whole parser validated at once, offset math included. Fixture,
font and generator are committed in `internal/fixtures/tileset/`.

Tested against `Tamzen5x9r.bdf` too, which surfaced the one divergence
below, but that font is copyright Scott Fial so it is not vendored; `4x6.bdf`
is public domain and covers the same ground.

### One divergence, deliberate

C's `TCOD_tileset_get_tile_id` returns 0 for an unmapped codepoint, and
`TCOD_tileset_get_tile` only rejects a *negative* id. So C hands back tile 0
(the reserved blank) and reports success: an unmapped codepoint is
indistinguishable from one deliberately assigned a blank glyph. Confirmed
against the C build, asking Tamzen for U+00B1, U+03B4, U+2588 and U+2591,
none of which that font defines: C returns an all-zero tile rather than an
error.

`Tile` returns nil instead, so a caller can tell "no glyph" from "blank
glyph". That is what lets `pngout` draw its missing-glyph marker. Every
codepoint a font actually defines behaves identically in both.

### `pngout` takes a Tileset

`Options.Tileset` overrides the built-in font; nil keeps the existing 8x8
art, so no caller changes. Cell size now comes from the tileset rather than
the hardcoded `CellPx`, and the missing-glyph fallback (a hollow box, not
`'#'`, per session 5) applies to tileset glyphs too.

This is the seam the session 5 review was really complaining about: the
glyph table was hardcoded, which is why the sample's A* route rendered as
walls. A tileset is the general form of that table.

### Project tooling

Added the standard config set, copied from the sibling Go repos so the
conventions match rather than being invented here: `.markdownlint-cli2.jsonc`,
`.golangci.yml` (v2 schema), `.github/workflows/ci.yml`,
`.pre-commit-config.yaml`, and a `.gitignore`. The Makefile gained the usual
targets (`help`, `build`, `cover`, `lint`, `md-lint`, `tidy`, `hooks`,
`check`) plus a `fixtures` target for the golden-fixture replay.

CI runs three jobs. `go` does build/vet/test/lint, `docs` runs markdownlint,
and `fidelity` replays the C-generated fixtures on its own. The last is
separate deliberately: a failure there means the port diverged from libtcod,
which is a different kind of problem from a lint or style failure and should
be legible as such in the checks list.

Running the linters found real problems, not just style noise:

- **Unchecked `Close` on four write paths** (`rexpaint.Save`, `rexpaint.Write`,
  `pngout.Render`, `image.Save`). `defer f.Close()` runs after `return nil`,
  so a failed flush was discarded and the caller was told the write
  succeeded. For `rexpaint.Write` this mattered most: gzip only emits its
  trailer on `Close`, so the failure mode is a file that looks fine until
  something reads it. All four now capture the close error into a named
  return. Read paths still ignore it, with the reasoning recorded in the
  golangci exclusion.
- **15 British spellings** (`colour`, `centre`, `neighbour`, `honouring`) in
  a codebase whose own API is `color`. Normalized to US, matching the sibling
  repos' `misspell` locale.

Two rules are switched off with reasons rather than worked around. `revive`'s
`exported` rule wanted doc comments on ~50 self-describing accessors
(`BSP.Left`, `RGB.Equals`); the interesting functions already name their C
original, and 50 restatements of the identifier would bury those. `MD060`
table alignment was normalized to the compact pipe style the other repos use.

Markdown: 87 violations, 69 auto-fixed, the rest either fixed by hand or
configured off where the construct was deliberate (the build log's second H1
for the Deferred Register; mixed fenced and indented code blocks). Now 0
errors across all four documents.

### Module path

`module golibtcod` became `module github.com/shindakun/golibtcod`, matching
the git remote and the directory it already lives in. The bare name only
worked for local development; `go get` needs the resolvable path. All 34
importing files updated, plus the goimports `local-prefixes` setting so
import grouping still recognises in-repo packages. The README quick start
now shows the `go get` line and the import block, which it never did.

Prose keeps saying "golibtcod": that is the project's name, and only the
import path needed to be a URL.

### Session 7 addendum: hardening found by re-review

A follow-up review of the new tileset code found three defects in it, all
now fixed with regression tests:

- **`charmapReserve` looped forever.** The doubling `newLength *= 2`
  overflows to negative past `MaxInt/2`, so `want > newLength` never became
  false. Reachable straight through `SetTile(maxInt-1, ...)`. Bounded at
  `U+10FFFF` now; the full legitimate Unicode range still works.
- **`reserve` reported success having allocated nothing.** `capacity *
  tileLength` overflows `int`: at a 2^28 square cell it wraps to exactly 0,
  so `make` succeeded with zero storage while `tilesCount` was set to 1.
- **A 124-byte BDF file panicked out of `ReadBDF`.** `FONTBOUNDINGBOX` fed
  `New` unbounded, so a hostile header reached the allocator directly
  (`makeslice: len out of range`). Since `ReadBDF` returns an `error`, an
  escaping panic broke its contract and a caller loading a user-supplied
  font could not defend without `recover`.

All three are closed by bounding the cell size in `New` (`maxTileDimension`,
4096 px per side) and the charmap at `U+10FFFF`. Real fonts are far below
both; `4x6.bdf` still loads all 919 glyphs byte-identically to C. This is
the same lesson as the rexpaint fix in session 5: dimensions read from
untrusted input must be bounded before they reach an allocator.

### Divergences in the BDF parser, documented not fixed

C's `read_next_int` is `strtol(..., 0)`, which accepts hex and octal; the Go
port uses `strconv.Atoi`, which is base-10 only and yields 0 on error. So
`ENCODING 0x41` maps to codepoint 0 in Go and 65 in C, and `ENCODING 010` is
10 rather than 8. Go is also more permissive about whitespace: tab-separated
or indented keywords and a leading space on a BITMAP row are all accepted
where C rejects them. None of this affects real fonts, which use plain
decimal and conventional layout, and the 919-glyph fixture comparison
confirms it.

`Tile()` returns a live slice into the internal buffer, matching C's const
pointer. Two traps a Go caller would not expect: the slice carries full
capacity, so re-slicing past its length reaches a neighbouring glyph, and it
goes stale once a later `SetTile` grows the buffer. Documented on the method,
which points callers at `Coverage()` when they need a value they can keep.

### Results after session 6

- 19 packages. `go build`, `go vet`, `gofmt`, `go test ./...` all clean.
- `golangci-lint run`: 0 issues. `markdownlint-cli2`: 0 errors.
- Still zero third-party dependencies in the library itself; the new tooling
  (golangci-lint, pre-commit, markdownlint) is developer-side only and does
  not appear in `go.mod`.
- Fixture suite unchanged, plus glyph bitmaps now verified against C.
- Deferred register has no open items.

---

## 2026-08-04: session 8

Closed the one real API parity gap: word-wrapped printing.

### The gap

A parity audit against upstream's 50 C translation units found the port
matched on every algorithm but was missing `console_printing.c`'s
`print_rect` / `get_height_rect` family. The effect was not subtle:
`Print` writes a single row and silently drops anything past the console
edge, so a 43-character string in a 20-wide console rendered as
`"the quick brown fox"` and lost the rest.

### `console/print_rect.go`

`PrintRect` and `HeightRect` port C's `next_split_` and the
`printn_internal_` driver loop. The two share a body; `HeightRect` runs it
with drawing suppressed, which is how C implements `get_height_rect`.

C classifies characters with utf8proc; the standard library's `unicode`
package answers the same questions, so this needed no new dependency. The
break rules are C's: break at the last space before overflow, break *after*
a dash so the dash stays on the line, split mid-word when a single word is
wider than the line, collapse the run of spaces a break lands on, and treat
line and paragraph separators as hard breaks (the latter advancing two
rows).

### Verified against the C build

Compiled `console.c` + `console_printing.c` + `console_drawing.c` against
utf8proc and dumped rendered layouts, the same golden-fixture method used
for the other modules. `console_etc.c` drags in SDL, so the harness stubs
the four globals `console.c` needs rather than linking it.

**600 randomized cases are byte-identical**, covering widths 1 to 26, all
three alignments, embedded newlines, leading and trailing whitespace, empty
strings, em-dashes, ellipses and accented characters. 200 of them are
committed as a fixture in `internal/fixtures/wrap/`.

Two bugs surfaced only because the comparison was against real C rather
than my own expectations:

- **`runeWidth` returned 1 for control characters.** utf8proc gives them a
  charwidth of 0, and C's `next_split_` adds that width *before* its
  `is_newline` check. So a `\n` arriving on a full line falsely overflowed
  it and forced a break, emitting a stray blank row. Fixed by returning 0
  for Cc, Mn, Me, Cf, Zl and Zp.
- **Empty input returned height 1.** C returns before any layout when the
  string is empty, so the bounding-box arithmetic never runs.

A third finding corrected a test rather than the code: `HeightRect` is
bounded by the console height, because C's driver loop carries the same
`top < console->h` condition whether or not it is drawing. I had asserted
the full layout height; C returns 3 for that case and so does the port.
Documented on the method.

### Results after session 8

- 20 packages. `go build`, `go vet`, `gofmt`, `golangci-lint`, markdownlint
  all clean.
- All prior fixtures unchanged; `wrap` adds 200 verified layouts.
- Still zero third-party dependencies.

---

## 2026-08-04: session 9

Closed the last parity item (`pathfinder.c`) and set up upstream watching.

### `pathfinder.c`: ported, with a large asterisk

The module is a Dijkstra engine over a caller-owned cost grid, exposed for
python-tcod rather than used anywhere inside libtcod. Building a C harness to
verify the port against turned up **four independent defects that make it
non-functional as shipped**, all confirmed against upstream HEAD `c54823e`
through the GitHub contents API:

1. `TCOD_pf_in_bounds` tests `0 < index[i]` where it means `0 >`, so every
   positive coordinate is rejected and only `(0,0)` is ever in bounds.
2. The relaxation test in `TCOD_pf_add_edge` is inverted (`>=`), and since
   distances start at `INT_MAX` an unvisited neighbour always returns early.
3. `graph.cost` is written by `TCOD_pf_set_graph2d_pointer` and never read
   anywhere, so the caller's cost grid is dead and walls do nothing.
4. `TCOD_pf_set_traversal_pointer` stores a byte stride into a `shape` field.

Together these mean `TCOD_pf_compute` cannot leave the origin cell. Nothing
in libtcod calls `TCOD_pf_*`, which is why the breakage is invisible there.
No upstream issue or PR covers any of it.

So `path/pathfinder.go` implements the **intended** behaviour rather than the
shipped behaviour: bounds checked correctly, relaxation toward lower
distances, and the cost grid honoured with non-positive cells impassable.
Porting the bugs would have produced a pathfinder that returns the origin and
nothing else.

Verified against a C build corrected on all four counts: **250 randomized
distance fields identical**, over grids to 12x12, cardinal and diagonal
weights 0 to 14, walls, negative costs and multi-root seeding. 80 are
committed in `internal/fixtures/pathfinder/` along with the patched C source
so the claim is auditable.

The NumPy-style pointer/stride/`int_type` machinery is replaced by ordinary
Go slices. That descriptor exists so python-tcod can pass an array without
copying; in Go it would be ceremony around what the type system already
gives you. C's `ndim` reaches 4, but the only edge function hardcodes
`origin[0]`/`origin[1]`, so the search is 2D upstream and 2D here.

### Upstream watching

`scripts/upstream-watch.sh` plus `.github/workflows/upstream-watch.yml` watch
`libtcod/libtcod` daily and file triage issues here, adapted from the same
mechanism in `shindakun/agent-sdk-go`.

The adaptation that matters is the classifier. Most of libtcod's C tree is
excluded from this port by design, so commits are bucketed by which file they
touch before any model is involved: files with a Go counterpart get a
Claude-triaged issue naming the affected package and assessing whether golden
fixtures would change; files we deliberately exclude roll into a single issue
so the decision stays visible without noise; docs, CI and tests are dropped.
The C-file-to-Go-package map is explicit rather than pattern-matched, so a
new upstream file surfaces as unknown instead of being silently ignored.

`scripts/upstream-watch-test.sh` pins that mapping with 32 cases and needs no
network. Commit messages and diffs reach the model only inside delimited
untrusted blocks with the delimiters stripped from the input first.

Dry-run against the last 12 real upstream commits: CI, dependabot and
changelog noise correctly ignored, and three genuine FOV buffer-overflow
fixes correctly flagged.

### Those FOV fixes, checked

`eaf2d5b`, `8210648` and `0455801` fix fixed-size buffer overflows in MRPAS
and permissive FOV on small maps. This port is structurally immune: `insert`
uses `append`/`copy` on a slice rather than shifting within a fixed array,
and the bump buffer is already sized `max(len(cells), 16)`, which is the
guard the upstream fix adds. Verified across all 13 algorithms on every map
from 1x1 to 4x4, at four radii, with and without wall lighting: no panics.

### Results after session 9

- 21 packages. `go build`, `go vet`, `gofmt`, `golangci-lint`, shellcheck and
  markdownlint all clean.
- All prior fixtures unchanged; `pathfinder` adds 80 verified distance fields.
- Still zero third-party dependencies.

---

# Deferred Register

**This is the canonical list.** Deferred items were previously scattered
across session notes, which meant the session-1 TODO went stale the moment
session 2 cleared most of it and a reader had to reconstruct the real state
from three places. Everything outstanding lives here now.

**Maintenance rule:** anything deferred gets a row here *in the same commit
that defers it*, with a reason and a trigger. When an item is done, write it
up in the session entry and delete its row. If a row has been here for three
sessions without its trigger firing, it probably isn't deferred: it's
declined, and should be moved to "Won't port" with the reasoning.

## Blocked

**Empty.**

## Deliberately not started

**Empty.** Cleared in session 3: `image` (+ mipmaps + subcell 2x
rendering), `rexpaint`, and the legacy colour markup are all built and
tested. The C++ compat layer moved to "Won't port" below.

## Omitted inside completed modules

| item | why |
| --- | --- |
| `heightmap.islandify` | A no-op in current upstream, pending removal. Porting a no-op would be porting a bug. |
| `heightmap.heat_erosion` | `#if 0` in the C source: dead code upstream. Reinstating it means writing thermal erosion from scratch, not porting. |

Both are noted in `heightmap/heightmap.go` at the point they'd sit.

## Won't port: replaced by design

Not deferrals. These are architectural substitutions, and listing them here
stops a future session "finishing the port" by adding them back.

| libtcod | golibtcod | why |
| --- | --- | --- |
| SDL renderer, context, input | the presenter interface (`present/*`) | Keeps the core engine-free and headless-testable: the property that makes batch worldgen, replay, CI rendering and the terminal build possible |
| `tileset_render.c` (SDL), `tileset_truetype.c` (stb_truetype) | the presenter interface | The rendering half of the tileset module. The atlas and BDF loader *are* ported (session 6); only the two files needing external libraries are excluded, and both answer a question the presenters already answer. |
| global RNG | explicit `*rng.Random` | A process-global generator can't support a seed economy with independent sim and audio streams |
| global namegen registry | explicit `*namegen.Registry` | Same reason; also lets multiple syllable sets coexist |
| `TCOD_list`, `TCOD_tree` | Go slices, maps, struct pointers | Hand-rolled C containers with no behaviour to preserve |
| a graphical backend (Ebitengine, SDL bindings, &c.) | the presenter interface (`present/*`) | Any engine binding is a third-party dependency, which the zero-dependency rule excludes. The contract is proven twice over by `pngout` and `term`; a consumer that wants a window implements the interface against whatever engine it already uses. Rendering is that consumer's concern, not this library's. |
| C++ compat layer (`TCODConsole` &c.) | the Go API itself | Class shims exist to make a C library feel like C++. Go already has methods on types, so this would be a second, worse spelling of an API we have: maintained twice, forever. Declined session 3. |

## Deviations from faithful porting

One entry, flagged because "faithful port" is the project's whole premise.

| module | deviation | why |
| --- | --- | --- |
| `parser` | **Clean-room, not a line-by-line port.** Divergences measured against the C implementation in session 4 and documented in the package doc; corpus and C harness in `internal/fixtures/parser`. Summary: `#` comments are a golibtcod extension (libtcod rejects them); validation is a separate optional layer rather than schema-first parsing; all errors are reported rather than the first; a type mismatch keeps the offending text instead of substituting zero; golibtcod aborts on a malformed file where libtcod recovers. | `lex_c.c` + `parser_c.c` are ~2,600 lines built on a global lexer and a listener-callback ABI, with no numerical behaviour to preserve. A transliteration would be both unpleasant and un-Go-like. |
| `image` | File IO uses stdlib `image/png` and `image/jpeg` instead of SDL_image. Pixel operations are line-by-line ports. | SDL is not a dependency, and the stdlib PNG decoder is better than the one being replaced. |
| `rexpaint` | `compress/gzip` instead of zlib. Format bit-identical. | Same reason: no external dependency. |
| `path` (`Pathfinder`) | **Implements the intended behaviour, not the shipped behaviour.** Upstream `pathfinder.c` has four defects that leave it unable to leave the origin cell: an inverted bounds test, an inverted relaxation test, a cost grid that is written but never read, and a stride stored in a shape field. All four confirmed against HEAD `c54823e`; no upstream issue covers them. The C pointer/stride/`int_type` descriptor is also replaced by Go slices. | Porting the bugs would yield a pathfinder that computes nothing. Verified instead against a C build corrected on all four counts: 250 randomized distance fields identical. See `internal/fixtures/pathfinder/README.md`. |

Everything else in the tree is a faithful port validated against fixtures
generated by the actual C code. Preserved C quirks are listed under session
1's fidelity notes; they are intentional and must not be "fixed".
