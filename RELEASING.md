# Releasing crux

Releases are cut **locally** with [GoReleaser](https://goreleaser.com)
(`.goreleaser.yaml`) — no CI, no PAT. It cross-compiles darwin/linux ×
amd64/arm64, publishes a GitHub Release on `chrismo/crux`, and commits the
Homebrew cask to `chrismo/homebrew-crux`. GoReleaser authenticates with your
local `gh` token (`gh auth token`), which has `repo` scope and can write both
repos. (This mirrors how grdy releases: build locally, push the tap from your
own machine.)

Install target once released: `brew install chrismo/crux/crux`.

## One-time setup

The tap repo `chrismo/homebrew-crux` already exists. Push the local scaffold once
so it has its README (GoReleaser adds `Casks/crux.rb` on the first release):

```sh
cd ../homebrew-crux
git remote add origin git@github.com:chrismo/homebrew-crux.git   # if not set
git push -u origin main
```

## Cut a release

```sh
# pre-flight: validate + full local build, no publish
goreleaser check
goreleaser release --snapshot --clean

# real release
git tag v0.1.0
git push origin v0.1.0
GITHUB_TOKEN=$(gh auth token) goreleaser release --clean
```

The last command publishes the GitHub Release and updates the tap. Then
`brew install chrismo/crux/crux` (or `brew upgrade crux`).

## Notes

- macOS binaries are unsigned; the cask's postflight strips the Gatekeeper
  quarantine attribute so `crux` runs without a prompt.
- To move releases to CI later: re-add a workflow and a cross-repo PAT
  (`HOMEBREW_TAP_GITHUB_TOKEN`) as the cask `repository.token` in
  `.goreleaser.yaml` — CI's default `GITHUB_TOKEN` can't push to another repo.
