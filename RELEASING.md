# Releasing

Tagging and pushing a `v*` tag is what ships a release. It triggers
`.github/workflows/release.yml`, which runs GoReleaser and publishes a GitHub
Release with binaries for linux/amd64, linux/arm64, linux/riscv64,
darwin/amd64, darwin/arm64, windows/amd64, windows/arm64, freebsd/amd64, and
freebsd/arm64, plus `checksums.txt`.

## Before tagging

```sh
gofmt -l .          # should print nothing
go vet ./...
go test -race ./...
go test -tags tmux ./tmuxlayout   # needs a real tmux on the path
```

Update `CHANGELOG.md`: move the `Unreleased` entries under a new version
heading.

## Tag and push

```sh
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

`-a` makes an annotated tag, with a message, author, and date — use it, not a
lightweight tag, for anything that ships. Pushing the tag is what fires the
release workflow; watch it under the repository's Actions tab.

Version numbers follow [semver](https://semver.org/): bump the major version
for a breaking change to `tmuxlayout.Apply`, `tmuxlayout.Layout.Render`, or the
command-line interface, minor for new functionality, patch for fixes. A change
to what a picture renders to is breaking even when it is a fix: people have
layouts built into keybindings.

## If you tagged the wrong commit

Before anyone has pulled it:

```sh
git tag -d v0.1.0
git push origin --delete v0.1.0   # only if already pushed — deletes the remote tag
```

Then fix and re-tag. Once a release is published, prefer a new patch tag over
deleting a tag someone may have already fetched.
