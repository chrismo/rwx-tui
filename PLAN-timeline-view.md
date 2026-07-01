# Timeline / waterfall view — placeholder plan

A third view alongside list and graph: a **left-to-right waterfall** of task
timings, like the RWX website's timeline, but with crux's power-tools layered on
— **type-to-filter** and **pins**, reusing the graph view's machinery. Placeholder
(intent captured, not designed). User says the website's version is "pretty
good" — replicate that, then add the fast-nav tools it lacks.

## The idea

Each task is a horizontal bar positioned by **when it ran** (start offset from
run start) and sized by **how long** it ran — the classic waterfall/Gantt shape.
Reading L→R shows the run's real shape: what ran in parallel, where the gaps and
long poles are. This is the wall-clock story the critical-path line only
summarizes (see `PLAN-critical-path.md`) — the two are complementary.

## Data — what exists, what's missing

- Task model already carries **`StartedAt`** and **`CompletedAt`** (ISO-8601)
  plus `ExecutionRuntimeSeconds`/`CompletedRuntimeSeconds`
  (`internal/rwx/model.go:38-41`). Absolute positioning is therefore possible —
  a true waterfall, not just bar lengths.
- **Gap:** the graph `Node` only captures `DurationSeconds`/`HasTiming`
  (`internal/graph/build.go:18`); `StartedAt`/`CompletedAt` are **not** threaded
  onto nodes. Plumbing those through (or reading straight from the task model for
  this view) is the one real prerequisite.
- Run start = min `StartedAt` across tasks (or the run's own start if available).
  Bar x = `StartedAt - runStart`; width ∝ duration; scale = viewport width /
  total wall-clock.

## Reuse from the graph view (the point)

These already exist and should back the timeline, not be re-implemented:

- **Type-to-filter** — the same substring narrowing (`computeVisible` /
  `ov.Filter` in `internal/ui/collapse.go`). Filtered-out tasks drop from the
  waterfall (collapse, not dim), same law as the graph.
- **Pins** — the pure-set pin model + `esc` history stack (`viewState`,
  `app.go:308`). Pin a task to keep it anchored while filtering; `esc` walks
  focus back. Reuse wholesale — don't fork the state.
- **Selection + keyboard nav** — `moveSelection`, viewport pan. Here movement is
  primarily **vertical** (row per task) with **horizontal pan** across the time
  axis for long runs.
- **Glyphs/colors/theme** — status glyphs and `theme.RunStatus` carry over;
  a bar is colored by task state (ran/cache/failed/skipped/running).

## Rendering sketch (rune-canvas, like the graph)

- One row per (visible) task; label on the left, bar in the time track.
- Bar drawn on the existing rune-canvas (`internal/ui/canvas.go`,
  `go-runewidth`) — filled cells for the run span, e.g. `▐████▌`, with a
  distinct style for cache hits vs real execution.
- Time axis header with tick marks (`0s   30s   1m   …`).
- Horizontal pan (reuse `xOffset` machinery) when the run is longer than the
  viewport; critical-path tasks could carry the thick-border / accent treatment
  for continuity with the graph.
- Honor the `--print` body-parity invariant (Screen/RenderX back both TUI and
  headless) if this view is ever printed.

## Open questions (for design, later)

- **Sort order** — by start time (true waterfall) vs topological (dependency
  order) vs duration (longest first)? Website uses start time; probably match,
  maybe toggle.
- **Parallelism density** — many concurrent short tasks get thin/overlapping
  rows; need a min bar width and possibly grouping.
- **In-progress runs** — running tasks have `StartedAt` but no end; draw an
  open-ended bar to "now" and live-update on the poll.
- **How to switch views** — a key to cycle list ↔ graph ↔ timeline for the same
  run, sharing filter/pin/selection state across them where it makes sense.
- **Dependencies on the waterfall** — show `use:` edges as connectors between
  bars (like the graph) or keep it clean and timing-only? Website keeps it mostly
  timing-only; start there.

## Status

Placeholder — intent only. Before building: confirm the sort model and whether
edges are drawn, and decide the view-switch UX. Prerequisite work item: thread
`StartedAt`/`CompletedAt` onto graph nodes (or read from the task model directly
for this view).
