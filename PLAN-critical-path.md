# Critical path — clarity review (low priority, pick up later)

User is unsure what the "critical path" display is actually showing. Not a bug
report — a legibility question. Capture now, revisit later.

## What it computes today (grounded)

- `graph.CriticalPath` (`internal/graph/critpath.go`): the **heaviest
  root→leaf dependency chain**. Weight per node = its real duration
  (`ExecutionRuntimeSeconds`, else `CompletedRuntimeSeconds`, stored as
  `Node.DurationSeconds`). **Fallback:** if *no* node has timing data, every node
  weighs 1, so it degrades to the **longest chain by edge count** (depth), not
  time.
- Surfaced two ways:
  - A header line (`CriticalPathLine`, `graphview.go:358`): `critical path:
    a → b → c · 42s` — the chain plus total seconds.
  - Thick border on on-path node boxes in the graph (`opts.Crit`,
    `graphview.go:41`).

## Why it may confuse (hypotheses, unverified with the user)

1. **Ambiguous meaning of "total."** `· 42s` is the summed weight of the chain,
   but a viewer may read it as wall-clock, or as the run's total time. It's
   neither exactly — it's the heaviest *dependency* chain's summed durations,
   which ignores parallelism/scheduling gaps.
2. **Silent fallback.** On a run with no timing (queued, or all-skipped), the
   "critical path" is really "longest chain by count" — but the label still says
   "critical path · Ns" (N could be small/0). Nothing tells the user the metric
   silently changed meaning.
3. **In-progress runs.** Durations are partial mid-run, so the highlighted chain
   can shift as data lands — may look unstable/arbitrary without explanation.
4. **No "why this chain."** The display shows *the* path but not what makes it
   critical (which node dominates the total). On a gnarly DAG the user can't tell
   if it's one slow task or a long tail.

## Directions to consider (when picked up)

- **Label the metric honestly.** Distinguish "critical path (by time)" vs
  "longest chain (no timing yet)" so the fallback is visible, not silent.
- **Show the dominant contributor** — e.g. bold the single heaviest node on the
  path, or annotate its share of the total.
- **Clarify the total** — call it "chain total" or add a hint that it's summed
  task time along dependencies, not wall-clock.
- **Confirm the actual confusion first.** Before building anything, get the user
  to point at a specific run and say what they *expected* the display to mean —
  the fix depends entirely on which of the hypotheses above is the real gap.

## Status

Low priority. No code change yet. Start by pinning down the user's actual
expectation vs. what's shown, then pick a direction above.
