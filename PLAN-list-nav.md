# Run-list navigation plan

Making the home run-list a fast, filterable power-tool — the list-side companion
to the graph view's type-to-filter. Decided 2026-07-01.

## Grounding (current code)

- `RenderRunList` (`internal/ui/listview.go:37`) renders rows most-recent-first;
  no view-level filtering exists.
- List actions are letter keys (`internal/ui/keys.go`): `a` all, `m` mine, `b`
  branch, `r` refresh, `g`/`G` top/bottom, `q` quit. All of `a`/`m`/`b` are
  **server-side re-fetches** (`rwx runs list` flags via `rwx.ListFilter`), not
  view filters.
- `b` "does nothing" here because every run is on `main`: it re-fetches
  main→main. Not a bug — homogeneous data. Keep it.
- Startup flags (`cmd/crux/main.go`): `--branch`/`--definition` resolve a single
  run to open in graph mode; `--filter`/`--pin` seed the **graph**. **Nothing
  filters the home list from startup or interactively today.**
- `rwx.ListFilter` already supports server-side `Branch`, `Mine`, `ResultStatus`,
  `Limit`, `Cursor`.

## The architecture: two tiers of filtering

Name the distinction explicitly — it's the spine of the whole feature.

- **Fetch filters** — change what `rwx runs list` returns (server-side, network).
  Today: `a` (all), `m` (mine), `b` (branch of selection). Add `--failed` /
  result-status. These stay on their letter keys.
- **View filters** — narrow already-fetched rows instantly (client-side
  substring, no network). This is the new type-to-filter. Matches against
  **Title, DefinitionPath, Branch** (case-insensitive substring; a row matches
  if the term is in any of the three).

The two **stack**: a fetch filter decides what's loaded; the view filter narrows
the view of that. Honest limitation (same as the graph's filter): a view filter
only sees loaded pages — it's "filter what's fetched," not "search all history."
Surface the count so this is legible: `filter: web  (3 of 40 shown)`.

## Interaction model: type-to-filter + Tab-cycle scope (chosen 2026-07-01)

Consistent with the graph view: **just type** to build the view filter, no `/`.
The fetch-scope trio (`a`/`m`/`b`) — which has no clean single-key home once
letters type — collapses into one **Tab** that cycles scope. This keeps "one
law, just type" across screens while preserving scope access.

Key map:

- **type** — printable keys build the view-filter term live; list narrows per
  keystroke.
- **Tab** — cycle fetch scope all → mine → branch (Shift+Tab reverse); each is a
  server re-fetch. Tab→branch uses the *selected* run's branch (skip if none).
- **arrows** — move selection; **Home/End** top/bottom; **ctrl+r** refresh;
  **ctrl+c** quit; **enter** open the selected run.
- **esc** — clear the view filter (empty = show all).
- Selection clamps to the first visible row if the current one filters out.

Header shows both tiers together (fetch scope + view filter), e.g.
`mine · filter: web  (3 of 40 shown)`.

## Startup args

Mirror the graph's seeding pattern (`--filter`/`--pin` seed the graph):

- `--list-filter <substr>` — seeds the **view** filter so `crux --list-filter
  web` opens the list pre-narrowed. The existing graph `--filter` is renamed to
  `--graph-filter` (the two filter different domains — run titles vs node keys —
  so they get distinct names rather than one overloaded flag).
- `--failed` — a **fetch** filter (result-status = failed). The killer startup
  filter: "just show me what broke." Maps to existing `ListFilter.ResultStatus`.
- `--mine` — expose the existing `ListFilter.Mine` as a startup flag.
- Existing `--branch` already narrows fetch (server-side) — confirm it also
  applies to the list path, not only single-run resolution.

## Bugs to fix alongside

### Vertical alignment (listview.go:54)

Rows pad with byte-width `fmt.Sprintf` (`%-13s`, `%-26s`):

- **`DefinitionPath` (`%-13s`) is min-width but never truncated** — any path >13
  chars shoves every following column right *on that row only* → ragged list.
  Primary cause.
- Padding counts **bytes, not display cells**, so multibyte runes in titles
  drift.

Fix: truncate DefinitionPath to a fixed cell width and pad via `go-runewidth`
(already imported in `internal/ui/canvas.go`) instead of `%-Ns`. Snapshot-test
with a long definition path and a multibyte title.

### Log view scrolls by mouse only (app.go:852–865)

When the detail/log pane is open (`detailOpen`), the key handler only listens for
`esc`/`l`; every other key returns `a, nil`, so keyboard never reaches the
viewport. Mouse wheel works via the unconditional `tea.MouseMsg` route
(app.go:702).

Fix: while the log pane is open, forward Up/Down/PgUp/PgDn and `g`/`G` to
`a.viewport.Update`. Small, self-contained.

## Phases

### Phase 0 — quick wins (independent, ship first)
- Alignment fix + snapshot.
- Log keyboard-scroll fix.

### Phase 1 — view filter core
- `/` filter mode: state, live substring match over Title/DefinitionPath/Branch,
  selection clamp, header count `(n of N shown)`.
- Header shows fetch label + view filter together (Option 1).
- Tests: filtering narrows rows, selection clamps, esc clears, enter persists.

### Phase 2 — startup + fetch flags
- `--failed`, `--mine`, list view-filter seed flag.
- Confirm `--branch` narrows the list path.

## Testing

Model-driven (drive the App directly), same as existing UI tests. Snapshot the
filtered list at a fixed width. No network — reuse the run-list fixtures /
injected `now`.

## Decisions locked / open

- LOCKED: type-to-filter (not `/`-modal) + Tab-cycles fetch scope, matching the
  graph. Match Title, DefinitionPath, Branch. Two-tier fetch-vs-view model.
  Trigger field NOT matched (dropped).
- LOCKED: header shows both tiers (scope + view filter) side by side.
- LOCKED: `--list-filter` seeds the view; graph `--filter` → `--graph-filter`.
- Phase 0 (alignment + log keyboard-scroll bugs) SHIPPED.
