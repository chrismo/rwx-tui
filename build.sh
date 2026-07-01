#!/bin/sh
# build.sh — the one build entrypoint, used locally and in RWX CI. Plain POSIX
# sh; needs the Go toolchain + git, plus goreleaser + gh for releasing.
#
# Usage:
#   ./build.sh vet             # go vet ./...
#   ./build.sh test            # go test ./...
#   ./build.sh build           # version-stamped host binary -> bin/crux
#   ./build.sh ci              # vet + test + build (what RWX runs, end to end)
#   ./build.sh version         # print the computed version string
#   ./build.sh snapshot        # goreleaser dry run: all platforms + refresh bin/crux
#   ./build.sh release vX.Y.Z  # tag, push, and publish the release (local goreleaser)
#
# snapshot and release both refresh bin/crux, so a local run after either one is
# never stale. For plain local iteration just use: ./build.sh build && bin/crux

set -eu

PKG=./cmd/crux
BIN=bin/crux

require() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "build.sh: '$1' not found (try: brew install $1)" >&2
		exit 1
	}
}

# Version metadata, injected into package main via -ldflags -X. Falls back to
# "dev" when git or the .git dir is unavailable (e.g. an RWX clone that does not
# preserve git history).
compute_ldflags() {
	_version=$(git describe --tags --always --dirty 2>/dev/null || echo dev)
	_commit=$(git rev-parse --short HEAD 2>/dev/null || echo none)
	_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
	printf -- '-s -w -X main.version=%s -X main.commit=%s -X main.date=%s' \
		"$_version" "$_commit" "$_date"
}

cmd_vet() {
	go vet ./...
}

cmd_test() {
	go test ./...
}

cmd_build() {
	mkdir -p "$(dirname "$BIN")"
	go build -ldflags "$(compute_ldflags)" -o "$BIN" "$PKG"
	echo "built $BIN"
}

cmd_version() {
	git describe --tags --always --dirty 2>/dev/null || echo dev
}

cmd_ci() {
	cmd_vet
	cmd_test
	cmd_build
}

# Full goreleaser build across all platforms without publishing — a pre-flight.
# Also refreshes the host binary (bin/crux) so it never lags behind a snapshot.
cmd_snapshot() {
	require goreleaser
	goreleaser release --snapshot --clean
	cmd_build
}

# Cut a release locally: validate, test, tag, push the tag, then goreleaser
# publishes the GitHub Release and updates the Homebrew tap. Auth is your gh
# token (repo scope), which can write both chrismo/crux and chrismo/homebrew-crux.
cmd_release() {
	_v="${1:-}"
	[ -n "$_v" ] || { echo "usage: ./build.sh release vX.Y.Z" >&2; exit 2; }
	case "$_v" in v*) : ;; *) _v="v$_v" ;; esac

	require goreleaser
	require gh

	if [ -n "$(git status --porcelain)" ]; then
		echo "release: working tree is not clean — commit or stash first" >&2
		exit 1
	fi
	if git rev-parse -q --verify "refs/tags/$_v" >/dev/null 2>&1; then
		echo "release: tag $_v already exists" >&2
		exit 1
	fi

	# Fail before tagging anything. cmd_build also refreshes bin/crux so the
	# local host binary matches the commit being released.
	goreleaser check
	cmd_vet
	cmd_test
	cmd_build

	git tag -a "$_v" -m "$_v"
	git push origin "$_v"

	# If goreleaser fails after this point the tag is already pushed; just re-run
	#   GITHUB_TOKEN=$(gh auth token) goreleaser release --clean
	GITHUB_TOKEN="$(gh auth token)" goreleaser release --clean
	echo "released $_v — brew install chrismo/crux/crux"
}

usage() {
	sed -n '2,12p' "$0"
	exit "${1:-0}"
}

case "${1:-}" in
	vet)      cmd_vet ;;
	test)     cmd_test ;;
	build)    cmd_build ;;
	version)  cmd_version ;;
	ci)       cmd_ci ;;
	snapshot) cmd_snapshot ;;
	release)  cmd_release "${2:-}" ;;
	-h|--help|help|"") usage 0 ;;
	*) echo "unknown command: $1" >&2; usage 1 ;;
esac
