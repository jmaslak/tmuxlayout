# Contributing

You want to help? That's great!

## Ground rules

- **Don't break the public interface.** You can add to it, but code that
  already depends on `tmuxlayout.Apply`, `tmuxlayout.Layout.Render`, or
  `tmuxlayout.Checksum` should keep working.
- **Don't change what a picture renders to without saying so.** People build
  layouts into shell functions and keybindings and expect the same window back
  every time. A change to the division arithmetic is a user-visible change, not
  a refactor. `tmux_live_test.go` pins the results against a real tmux for this
  reason.
- **Bug reports** are welcome however you want to send them — GitHub issue
  preferred, email to <jmaslak@antelope.net> is fine. Security issues should be
  reported privately by email first, so there is a chance to fix them before
  they are public.
- **Code changes** are welcome however you want to send them, GitHub pull
  request preferred.
- **You will be credited** unless you ask otherwise. If the credit is wrong,
  send a note or a PR.

## LLM Policy

Because this was converted from Perl to Go using an LLM, it would be
hypocritical to deny LLM-assisted submissions. However, LLM-assisted
PRs and bug reports should be noted as such, and you are expected to
actually review the code produced (for PRs) and validate any issues LLMs
find in the code. Do not submit a PR or issue just to get credit without
understanding the code or issue you are submitting--I should not be the
first human to read the code!

## Things that would help

- Layouts that come out wrong. If tmux draws something other than the picture
  you wrote, that is a bug and the picture is the bug report.
- Pictures that are rejected as unbuildable but that tmux can in fact build.
  The split search is the part most likely to be too strict.
- Documentation and examples.
- If something is missing, confusing, or does not work the way you expect,
  say so. The tool makes sense to its author and their way of working; if it
  does not make sense to you, you are probably not the only one.

## Before sending a change

```sh
gofmt -l .          # should print nothing
go vet ./...
go test -race ./...
go test -tags tmux ./tmuxlayout
```

The `tmux` tagged tests need a real tmux on the path, and are the ones that
catch a wrong layout. tmux does not report a layout it disagrees with: handed
one whose panes do not quite tile the canvas, it repairs it and carries on, so
the panes come out at the wrong sizes and every other test still passes. Run
them.

If your change touches the picture parsing or the split search, also let the
fuzzer run for a while:

```sh
go test -fuzz FuzzRender -fuzztime 2m ./tmuxlayout
```

It checks that nothing panics and that any layout that renders has panes that
stay on the canvas and do not overlap.

## Relationship to the Perl version

This is a port of [Term::Tmux::Layout][perl], which has been deprecated now
that the code has moved to Go. The port is not bug for bug compatible; see
"Differences from the Perl version" in the README.

[perl]: https://github.com/jmaslak/Term-Tmux-Layout

By participating in this project you agree to abide by its
[Code of Conduct](CODE_OF_CONDUCT.md).
