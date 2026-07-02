# crux

A little terminal UI for watching [RWX](https://www.rwx.com) runs, with a Flow
dependency-graph viewer that's trying to be friendlier inside complex DAGs.

It doesn't do all that much yet, and the graph
rendering is crude. But it lists your runs, draws the DAG, colors what
cached vs. ran vs. failed, and points at the critical path — which is most of
what you actually want to know mid-build.

## Install

```sh
brew install chrismo/crux/crux
```

To upgrade later, use the same fully-qualified name — `brew upgrade
chrismo/crux/crux`. The bare `brew upgrade crux` only resolves once Homebrew has
refreshed the tap (`brew update`); the full `user/tap/token` path works from a
cold cache.

It shells out to the [`rwx` CLI](https://www.rwx.com/docs) (**v3.19.0 or
newer**), so you'll need that installed and authed:

```sh
brew install rwx-cloud/tap/rwx
rwx login
```

crux checks the CLI version on startup and tells you if it's too old, so if it
exits with a version complaint, `brew upgrade rwx-cloud/tap/rwx`.

## Use

```sh
crux                  # home: a list of recent runs — pick one, hit enter
crux --run <run-id>   # jump straight into a run's flow graph
crux --branch <name>  # filter the list to a branch
crux --print          # render once to stdout and exit (no TUI; handy for pipes)
crux --version
```

What the graph tries to make obvious:

- **cache clarity** — a glyph per task: `✓` ran, `⚡` cache hit, `✗` failed,
  `⊘` skipped, `●` running, …
- **critical path** — the heaviest chain, highlighted, with the total time
- **failure tracing** — the failed task and its downstream blast radius
- **focus/filter** — isolate a node's ancestors + descendants, or filter by name

In-flight runs live-update on a poll.

## Status

Very early. Known rough edges:

- The graph layout is deliberately simple — rows of boxes with `│` cues, no real
  edge routing. It gets cramped on big, gnarly DAGs, which is exactly where it
  most needs the work.
- macOS + Linux, single static binary. It's unsigned, so the Homebrew cask
  strips the Gatekeeper quarantine on install.
- The interactive bits are tested by driving the model directly, not a real
  terminal — expect the occasional papercut. Reports welcome.

## Build from source

```sh
git clone https://github.com/chrismo/crux
cd crux
./build.sh build   # -> bin/crux
./build.sh ci      # vet + test + build
```

Releases are cut locally with GoReleaser — see [RELEASING.md](RELEASING.md).
