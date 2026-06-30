#!/bin/sh
# build.sh — the one build entrypoint, used locally, in RWX CI, and (later) for
# release. Plain POSIX sh, no dependencies beyond the Go toolchain and git.
#
# Usage:
#   ./build.sh vet         # go vet ./...
#   ./build.sh test        # go test ./...
#   ./build.sh build       # version-stamped host binary -> bin/rwxtui
#   ./build.sh ci          # vet + test + build (what RWX runs, end to end)
#   ./build.sh version     # print the computed version string
#
# Cross-compile (`dist`) and the RWX release workflow are deferred until the
# first tagged release; this script is structured so they slot in later.

set -eu

PKG=./cmd/rwxtui
BIN=bin/rwxtui

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

usage() {
	sed -n '2,15p' "$0"
	exit "${1:-0}"
}

case "${1:-}" in
	vet)     cmd_vet ;;
	test)    cmd_test ;;
	build)   cmd_build ;;
	version) cmd_version ;;
	ci)      cmd_ci ;;
	-h|--help|help|"") usage 0 ;;
	*) echo "unknown command: $1" >&2; usage 1 ;;
esac
