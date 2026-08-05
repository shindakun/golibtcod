#!/usr/bin/env bash
#
# Unit test for upstream-watch.sh's file classifier. No network, no GitHub.
# Run with: bash scripts/upstream-watch-test.sh
#
# The classifier decides whether an upstream C file maps to a Go package we
# port, is excluded by design, or is irrelevant. Getting this wrong either
# floods the issue tracker or silently misses a real upstream change, so the
# mapping is pinned here.
set -euo pipefail
# shellcheck source=scripts/upstream-watch.sh
UPSTREAM_WATCH_SOURCE=1 source scripts/upstream-watch.sh

fail=0
check() { # check <file> <expect-pkg-or-EXCLUDED-or-IGNORE>
  local f="$1" want="$2" p got
  p="$(ported_package "$f")"
  if [ -n "$p" ]; then got="$p"
  elif is_excluded "$f"; then got="EXCLUDED"
  else got="IGNORE"; fi
  if [ "$got" != "$want" ]; then
    printf 'FAIL %-42s got=%-18s want=%s\n' "$f" "$got" "$want"; fail=1
  else
    printf 'ok   %-42s -> %s\n' "$f" "$got"
  fi
}

# Ported modules
check src/libtcod/mersenne_c.c            rng
check src/libtcod/fov_c.c                 fov
check src/libtcod/fov_permissive2.c       fov
check src/libtcod/path_c.c                path
check src/libtcod/pathfinder.c            "path (Pathfinder)"
check src/libtcod/heapq.c                 "path (heap)"
check src/libtcod/console.c               console
check src/libtcod/console_printing.c      console
check src/libtcod/console_rexpaint.c      rexpaint
check src/libtcod/tileset_bdf.c           tileset
check src/libtcod/parser_c.c              parser
check src/libtcod/lex_c.c                 parser
check src/libtcod/image_c.c               image
check src/libtcod/noise_c.c               noise
check src/libtcod/heightmap_c.c           heightmap
check src/libtcod/color.c                 color
check src/libtcod/bsp_c.c                 bsp
check src/libtcod/namegen_c.c             namegen
check src/libtcod/bresenham_c.c           bresenham

# Excluded by design
check src/libtcod/renderer_sdl2.c         EXCLUDED
check src/libtcod/sys_sdl_c.c             EXCLUDED
check src/libtcod/context.c               EXCLUDED
check src/libtcod/tileset_render.c        EXCLUDED
check src/libtcod/tileset_truetype.c      EXCLUDED
check src/libtcod/txtfield_c.c            EXCLUDED
check src/libtcod/console_etc.c           EXCLUDED
check src/libtcod/list_c.c                EXCLUDED
check src/libtcod/console.cpp             EXCLUDED

# Neither: no issue
check README.md                           IGNORE
check .github/workflows/ci.yml            IGNORE
check tests/test_console.cpp              IGNORE
check docs/index.rst                      IGNORE

exit $fail
