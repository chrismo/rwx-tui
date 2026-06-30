# RWX TUI — local build monitor with a better Flow graph

## Context

The RWX web UI (`cloud.rwx.com/mint/dscout/runs`) is serviceable but its Flow
dependency-graph viewer is weak — hard to see the critical path, no focused
subgraph filtering, cache/skip reasons are opaque, and tracing a failure to its
logs/blast-radius is clunky. We want a **local TUI** that:

1. Renders a **better Flow dependency-graph viewer** for RWX runs, with four
   specific wins over the web UI (critical-path, focus/filter, cache clarity,
   failure tracing).
2. Optionally fires **macOS notifications** on status changes for a watchlist of
   builds plus the current branch.
3. **Dogfoods RWX**: the tool's own CI runs on RWX (see Dogfooding below).

This is greenfield tooling that lives in its **own standalone personal repo
(`chrismo/rwxtui`)** — not in the dscout monorepo. Rationale: it's a tool *about*
a repo's CI, not part of any product; the monorepo has no Go toolchain and its
path-filter / tree-hash CI design would have to be taught to ignore it; and we
want to iterate freely without product-CI friction or org governance. It stays
**org-agnostic** — `org` and the target repo's `.rwx/` location are configured,
and it reads any local checkout's configs via a `--dir` flag, but its primary
data source is the live `rwx` CLI (see below), so a local checkout is optional.

### This machine (verified 2026-06-30)

Working repo: **`/Users/chrismo/modev/rwx-tui`** (git-initialized; `PLAN.md`
committed as `085003a`). Note: an earlier draft of this plan referenced
`/Users/chrismo/dev/rwx-tui` and a dscout checkout at
`/Users/chrismo/dev/no-linear-rwx-tui` — **neither exists on this machine**; that
material was captured on a different box and has been removed from this revision.

Toolchain installed and verified here:

- **Go 1.26.4** (`/opt/homebrew/bin/go`) — explicitly `brew install`ed so it is a
  leaf and won't get autoremoved.
- **rwx v3.19.0** — `brew install rwx-cloud/tap/rwx`. (The prior plan assumed a
  pre-installed v3.16; it was not present. v3.19 has a materially richer CLI —
  see Data layer.)
- **rwx agent skill** installed globally for Claude Code at
  `~/.claude/skills/rwx` (via `npx skills add rwx-cloud/skills -g -s rwx`).

> **Prerequisite — not yet done: `rwx login`.** `rwx whoami` currently fails
> ("no access token configured"). Every *live* call below needs auth. Run
> `rwx login` (device auth) or set `RWX_ACCESS_TOKEN` before the data layer or
> the dogfood CI can talk to the org. Until then, the JSON shapes here are taken
> from the authoritative field reference (`rwx docs pull /results`) but have
> **not** been sampled against a live run on this machine.

### Data layer reality (v3.19 — re-verified against the CLI on this machine)

The previous plan's central premise — *"RWX has no public list-runs or
full-DAG-status API, so you must parse `.rwx/*.yml` and fire ~78 concurrent
per-task status calls"* — is **obsolete in v3.19.** The CLI now exposes both a
list API and a single-call full-DAG-status payload. Confirmed surface:

- **List / resolve runs**: `rwx runs list --json` (alias `ls`). Filters:
  `--branch`, `--commit`, `--definition`, `--repository`, `--execution-status`
  (`waiting|in_progress|finished|aborted`), `--result-status`
  (`succeeded|debugged|sandboxed|failed|no_result`), `--mine`. Paginated:
  `--limit` (≤100) + `--cursor`, with `NextCursor` in the JSON. This replaces the
  old "no way to enumerate runs" gap and the branch/definition resolution dance.

- **Full run + per-task status in ONE call**: `rwx results <id> --json` (alias
  `rwx runs show <id>`). Returns run-level fields **plus the entire recursive
  `Tasks` tree** (tasks nest under `Subtasks`; walk with
  `recurse(.Subtasks[]?)`). Per the field reference (`rwx docs pull /results`),
  each task carries everything the four graph wins need — no per-task polling
  pool, no YAML re-derivation:
  - `Key`, `TaskType` (`command|parallel|package|embedded-run|app-config`).
  - `Status.Result` (`succeeded|failed|no_result`) and `Status.Execution`
    (`not_generated|waiting|ready|running|finished|aborted|skipped|user_error`).
  - **Cache clarity, built-in**: `CacheKey`, `CacheHitFromTaskID`, and
    `Status.FinishedSubStatus` (`cache_hit|executed|sandbox_closed|…`). A
    `skipped` if-condition surfaces as `Status.Execution == "skipped"` with a
    `skip_reason` Message. No need to parse `if:`/`filter:` and re-evaluate them.
  - **Real timing for critical path**: `CompletedRuntimeSeconds`,
    `ExecutionRuntimeSeconds`, `PreparingRuntimeSeconds`,
    `PostProcessingRuntimeSeconds`, `StartedAt`/`CompletedAt`.
  - **Failure tracing**: run-level `ResultPrompt` (LLM failure summary),
    per-task `Messages[]` (`user_error|produced_error|…` with
    `FileName`/`Line`/`Column`/`StackTrace`), `FailedTestCount`/`TestCount`,
    `ArtifactCount`, `ApproximateLogBytes`.
  - **Graph edges**: each task includes `RawDefinition` (its own YAML as it ran,
    pre-expression-eval) which contains the `use:` array — so dependency edges
    can be read straight from the run payload. (Static `.rwx/*.yml` parsing is
    now only a *fallback* for rendering a DAG when no run exists yet.)
  - Run-level: `Branch`, `CommitSha`, `Author`, `CommitMessage`,
    `DefinitionPath`, `Trigger`, `MergeRequestUrl`/`Number`/`Title`, `Title`,
    `RepositoryUrl`, `Status.Execution`, `Completed`.

- **Server-side completion polling**: `rwx results <id> --wait` blocks until the
  run completes (`--fail-fast` returns as soon as failures are available). Good
  for a one-shot "wait for green," but a live-updating TUI still polls the
  `results --json` snapshot on an interval (see Polling).

- **Logs**: `rwx logs <id> --task <key>` (downloads/extracts). Artifacts:
  `rwx artifacts`.
- **Run a definition without pushing**: `rwx run .rwx/<file>.yml --wait` patches
  the local git clone with uncommitted contents and runs it — no commit/push
  needed. (Basis for the dogfood loop.)
- **Lint a definition**: `rwx lint .rwx/<file>.yml`.
- **Docs**: `rwx docs search "<q>"` / `rwx docs pull <path>` (authoritative;
  `/results` and `/migrating/rwx-reference` are the two we lean on).
- macOS toast: assume no `terminal-notifier`; use
  `osascript -e 'display notification …'`.

## Decisions (from user)

- **Stack**: Go + Bubble Tea / Lipgloss (single static binary; matches `rwx`).
- **Data source**: **live `rwx` CLI** (`runs list` + `results --json`), not a
  local dscout checkout. A `--dir` static-YAML parse is a fallback only.
- **Polling**: smart — one `results --json` snapshot per cycle; back off as the
  run nears completion; stop when `Completed`.
- **Graph wins**: all four (critical-path, focus/filter, cache clarity, failure
  tracing) — now largely *driven directly by the results JSON*.
- **Notifications**: watchlist (pinned runs) + current repo branch, auto.
- **Dogfooding**: set up RWX to build/test this tool (see Dogfooding).

## Architecture

Standalone Go module in its own repo (`chrismo/rwxtui`). Primary data source is
the `rwx` CLI; `--dir` (default cwd) only matters for the static-YAML fallback.

```
rwxtui/                       # repo root (/Users/chrismo/modev/rwx-tui)
  go.mod
  .rwx/                       # dogfood CI (see Dogfooding)
    ci.yml
  cmd/rwxtui/main.go          # entrypoint, flag parsing (--branch, --definition, --dir, --run)
  internal/rwx/               # data layer — wraps the rwx CLI
    cli.go                    # exec rwx, parse --json, strip release-notice noise, surface errors
    runs.go                   # `runs list` (resolve/enumerate) + `results --json` (full tree)
    model.go                  # Run, Task (recursive Subtasks), Status enums from the field ref
    poll.go                   # single-call snapshot poller w/ backoff until Completed
    dag.go                    # FALLBACK: parse .rwx/*.yml → Graph when no run exists
  internal/graph/             # layout + analysis (pure, unit-tested)
    build.go                  # results-JSON Tasks tree + RawDefinition use: → Graph{Nodes,Edges}
    layout.go                 # topological layered (Sugiyama-lite) coords
    critpath.go               # longest-duration chain (ExecutionRuntimeSeconds; topo-depth fallback)
    focus.go                  # ancestors+descendants subgraph of a node
  internal/ui/                # Bubble Tea models
    app.go                    # root model, keymap, view routing
    graphview.go              # the Flow viewer (pan/zoom/scroll, render nodes+edges)
    detail.go                 # task detail pane (status, cache, timing, messages, logs)
    runlist.go                # pick run / branch / definition (from `runs list`)
    watchlist.go              # manage pinned runs
  internal/notify/
    macos.go                  # osascript toast; diff prev→curr status to fire
  internal/config/
    config.go                 # ~/.config/rwxtui/config.yml: watchlist, org, poll interval
```

### Data layer (`internal/rwx`)

- `cli.go`: thin `exec.Command("rwx", "--json", …)` wrapper. Strip the
  "new release available" stderr notice. Detect the not-authed error and surface
  a "run `rwx login`" message to the UI instead of crashing. Detect the
  multi-definition ambiguity (when `--definition` is required) and surface the
  choices.
- `runs.go`:
  - `List(filter)` → `rwx runs list --json` → `[]RunSummary` (+ `NextCursor`).
    Used by the run picker, branch resolution, and the watchlist.
  - `Results(id)` → `rwx results <id> --json` → one `Run` with the full recursive
    `Tasks` tree. This is the single source of truth for live status.
- `model.go`: structs mirroring the `/results` field reference — `Run`,
  `Task{Key, TaskType, Status{Result, Execution, FinishedSubStatus, …}, CacheKey,
  CacheHitFromTaskID, *RuntimeSeconds, StartedAt, CompletedAt, RawDefinition,
  Messages, ArtifactCount, Subtasks}`. Enums for Result/Execution/sub-statuses.
- `poll.go`: the smart poller. Cycle = **one** `Results(id)` call; diff against
  the previous snapshot; emit a `StatusMsg` into Bubble Tea via a channel.
  Backoff widens the interval as `Status.Execution` approaches `finished`; stop
  when `Completed`. (No worker pool — the prior plan's bounded concurrent
  per-task design is unnecessary now that one call returns the whole tree.)
- `dag.go` (fallback only): parse `<dir>/.rwx/*.yml` with `gopkg.in/yaml.v3` into
  a `Graph` for the *no-run-yet* case (render the static DAG before launching a
  run). Live runs build the graph from the results payload instead.

### Graph viewer (`internal/ui/graphview.go` + `internal/graph`)

`graph/build.go` turns a results `Run` into a `Graph`: nodes = tasks (walk
`recurse(.Subtasks[]?)`), edges = each task's `use:` parsed from its
`RawDefinition`. Layered top-down layout in `layout.go` (topological layering +
barycenter ordering to reduce crossings). Render Lipgloss boxes for nodes,
ASCII/Unicode connectors for edges, in a scrollable/pannable viewport. The four
wins — most now read straight from the JSON:

- **Critical-path** (`critpath.go`): longest chain by **real**
  `ExecutionRuntimeSeconds` (fall back to `CompletedRuntimeSeconds`, then to
  topological depth when timings are null/pre-run). Bold/color the chain; show
  total wall-time in the status bar.
- **Focus/filter**: `/` type-filters task keys; `f` on a selected node isolates
  its ancestor+descendant subgraph (`focus.go`) and dims the rest.
- **Cache clarity**: per-node glyph/color derived directly from
  `Status.Execution` + `Status.FinishedSubStatus` + `CacheHitFromTaskID`:
  `cache-hit | ran | skipped | running | waiting | failed | not-generated`.
  No `if:`/`filter:` re-evaluation needed.
- **Failure tracing**: `x` jumps to the first task with `Status.Result == failed`;
  `enter` opens the detail pane showing `Messages` (file/line/stack),
  `ResultPrompt`, test counts, and a "download logs" action (`rwx logs`); the
  downstream blast radius (descendants of the failed node) is highlighted.

### Notifications (`internal/notify`)

The poller keeps a prev-status map per watched run (keyed by task `Key`). On any
task or run-level transition (e.g. `running → failed`, run `succeeded`), fire an
`osascript` toast. Watched set = config watchlist + auto-resolved current-branch
run (resolved via `rwx runs list --branch <b> --json`). Toggle via flag/config.

### Config (`internal/config`)

`~/.config/rwxtui/config.yml`: `org`, `defaultDefinition`, `pollIntervalSec`,
`notify: bool`, `watchlist: [run-ids]`. Org-agnostic — nothing hardcoded.

## Dogfooding: RWX builds rwxtui

We eat our own dog food — the tool's CI runs on RWX, defined in `.rwx/ci.yml` in
this repo. Tasks: `go vet`, `go test ./...`, `go build ./...` (and `rwx lint`
self-check). Authored against `rwx docs pull /migrating/rwx-reference`; validated
locally with `rwx lint .rwx/ci.yml` and exercised with
`rwx run .rwx/ci.yml --wait` (no push required — RWX patches local contents).

**OSS-support caveat (verified):** RWX has **no automatic free OSS tier**. Per
rwx.com, open-source projects **email RWX to request free OSS credits**. So
before the dogfood CI can run against the cloud we must either (a) request OSS
credits for `chrismo/rwxtui`, (b) run it under an existing org's credits, or
(c) keep it local-only via `rwx run --wait` for now. This is a gating decision,
not a blocker for writing the config.

## Build order (incremental, each step runnable)

Work happens in `/Users/chrismo/modev/rwx-tui`.

0. **Done**: plan committed; toolchain (Go, rwx, skill) installed. **Next:
   `rwx login`** so live calls work.
1. **Module + skeleton**: `go mod init`, Bubble Tea hello-world, flag parsing
   (`--branch`, `--definition`, `--run`, `--dir`).
2. **Data layer (live)**: `cli.go` + `runs.go` — `runs list --json` and
   `results <id> --json` into the `model.go` structs. Print a resolved run's
   task count + a few `Key/Status` rows to validate against a real run picked
   from `rwx runs list`.
3. **Dogfood CI**: write `.rwx/ci.yml` (vet/test/build), `rwx lint`, then
   `rwx run .rwx/ci.yml --wait`. Gets us a *real* run of *our own* repo to point
   the TUI at, and validates the live data layer end-to-end.
4. **Graph build + static render**: `graph/build.go` (Tasks tree + RawDefinition
   `use:` → Graph), layered layout + viewport. No live updates yet.
5. **Live status**: smart snapshot poller → color nodes by
   `Status.Execution`/`Result`; status bar shows run-level result. Handle the
   not-authed and multi-definition cases.
6. **Four graph wins**: critical-path (real timings), focus/filter, cache
   glyphs, failure jump + blast radius.
7. **Detail pane + logs**: messages/timing/cache panel; `rwx logs` integration.
8. **Notifications**: macOS toasts on transitions; watchlist UI + config.

## Verification

- **Unit tests** (`go test ./...`) for the pure logic — most valuable where bugs
  hide: `graph/build.go` (results JSON → nodes/edges; recursive `Subtasks` walk;
  `use:` extraction from `RawDefinition`), `graph/layout.go` (no cycles, stable
  layering), `critpath.go` (longest-duration chain on a fixture, plus the
  topo-depth fallback when timings are null), `focus.go` (ancestor/descendant
  sets). This is new code, so tests ship with it.
- **CLI-layer tests** use a fake `rwx` (inject the exec function) returning
  captured real JSON shapes — a `runs list` page, an in-flight `results`
  (`Execution: in_progress`, some tasks `running`/`waiting`), and a completed
  `results` with a `failed` task + `ResultPrompt`. **Capture these fixtures from
  a live run once `rwx login` is done** (the field reference gives the shape; a
  real sample pins the exact JSON). No network in tests.
- **End-to-end manual run** against the live org once authed: resolve a run via
  `rwx runs list --branch <b> --json`, launch the TUI on it, confirm the graph
  renders, statuses populate, a failed task highlights with its blast radius, and
  the critical path (real `ExecutionRuntimeSeconds`) is plausible. The dogfood
  `rwx run .rwx/ci.yml --wait` gives a controlled run to test against.
- **Notification smoke test**: point at a finished run, force a synthetic
  prev→curr transition, confirm the macOS toast appears via `osascript`.

## Open risks / notes

- **Auth is the immediate gate.** Nothing live works until `rwx login`. The JSON
  shapes here are from the authoritative field reference, not yet sampled on this
  machine — pin exact fixtures right after auth.
- **OSS credits.** Dogfood cloud runs need RWX OSS credits (email request) or an
  existing org's credits; local `rwx run --wait` works in the meantime.
- Single-call `results --json` for ~78 tasks is one large payload rather than 78
  small calls — simpler and almost certainly faster, but confirm payload size /
  latency on a real big run; if needed, drill into a subtree with `--task`.
- Timing for critical path is now first-class (`ExecutionRuntimeSeconds`), so the
  prior "timings may be absent" risk is mostly resolved; still fall back to
  topological depth for pre-run / `not_generated` tasks.
- Standalone personal repo (`chrismo/rwxtui`), kept org-agnostic so it could
  later be pointed at any RWX org or open-sourced.
```
