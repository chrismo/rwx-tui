# Graph-view navigation — forward proposals

Candidate features that build on the focus **history stack** shipped in `cf1d734`.
These are planner proposals, not locked decisions — each is grounded against
current code so the coder can pick one up when the user greenlights it.

## Foundation already in place (`cf1d734`)

- `viewState{filter, pins, selected}` (`internal/ui/app.go:308`) — the unit of
  focus history.
- `history []viewState` back-stack (app.go:215), `pushHistory` before any focus
  mutation (app.go:318), `popHistory` on `esc` (app.go:330), capped at
  `maxHistory = 50`.
- Pins are a pure set; `space` toggles (`togglePin`, app.go:422). `esc` = cancel
  live filter, else pop one snapshot.
- Visible set derives from filter substring or pin cone (`computeVisible`,
  `internal/ui/collapse.go:18`); `ov.Focus` is the intersection of pinned
  anchors' cones.

The through-line: **pins are a set you edit forward; history is a stack you walk
backward.** Every proposal below should preserve that split, not re-entangle it.

## 1. Redo (forward stack) — smallest, highest-leverage

`popHistory` currently **discards** the popped state, so `esc` is one-way. Make
the stack a proper `pushd`/`popd`:

- Add `future []viewState`. On `esc`/`popHistory`, push the *current* state onto
  `future` before restoring the popped one.
- Add a redo key (`shift+esc` is awkward in terminals — consider `ctrl+r` or a
  redo binding) that pops `future` back onto `history` and restores it.
- **Any new forward mutation (pin/unpin/filter-commit) clears `future`** — the
  standard undo/redo invariant. Fold this into `pushHistory`.

Cost: ~15 lines + tests. Symmetric back/forward nav for free. Do this before
breadcrumbs — the HUD wants both stacks.

## 2. History HUD / breadcrumbs

The history stack is invisible; make it legible. `viewState` already carries
`filter` and `pins`, so a trail is pure render:

- Render a compact trail in the header: `web › lint-web › node-deps` (each hop =
  the distinguishing filter/pin of that snapshot).
- With redo (#1) in place, show position in the trail (e.g. dim the entries
  ahead of the cursor).
- Pure additive render off existing state; no model change beyond #1.

## 3. Named views / bookmarks

Pins already persist across runs (commit `3fb6612`, "persistent pins"). Extend
that: save a `(filter, pins)` under a name and recall it.

- New verb to save the current focus as a named view; a picker to recall.
- Reuse the pin-persistence store; a named view is `{name, filter, []pins}`.
- Recall = set filter + pins + `pushHistory` (so it's undoable like any focus
  change).
- Open question: scope — per-repo? global? Follow wherever pin persistence
  already lives.

## 4. Selection-driven filtering (descend into a cone)

Tie `moveSelection` to the visible-set machinery: descend into the selected
node's cone as a focus, keyboard-only.

- `l`/enter on a selected node sets focus to that node's ancestor+descendant cone
  (the `focus.go` cone logic that already backs `ov.Focus`), `pushHistory` first
  so `esc` backs out.
- Effectively "zoom into this node's world" without typing a filter — the
  fast-navigation power move.
- Interacts with pins: decide whether descend *adds* a pin (refine) or sets a
  transient focus. Leaning transient focus (undoable via history), leaving pins
  for deliberate multi-anchor work.

## Suggested order

1 (redo) → 2 (breadcrumbs, wants redo) → 4 (selection descend) → 3 (named
views, largest surface). Each is independently shippable; none requires
`internal/graph` changes.
