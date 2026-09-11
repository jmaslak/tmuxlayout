# tmuxlayout

Arrange tmux panes by drawing a picture of the layout you want.

tmux can split a window any way you like, but getting back to a particular
arrangement means either clicking it back together pane by pane, or handing
`select-layout` a string like
`b77e,80x24,0,0{48x24,0,0,1,15x24,49,0,2,15x24,65,0[15x12,65,0,3,15x11,65,13,4]}`.
The built-in layouts — `even-horizontal`, `main-vertical`, `tiled` and the rest
— are the only ones you get for free, and none of them is the one you actually
want.

This draws it instead. One character per pane, cells with the same character
are the same pane:

```sh
tmuxlayout 11123 11124
```

```
+------+--+--+
|      |  |  |
|      |  |  |
|      |  +--+
|      |  |  |
|      |  |  |
+------+--+--+
```

Pane 1 gets three fifths of the width and the full height, pane 2 gets a fifth,
and panes 3 and 4 share the last fifth. The panes in the current window are
rearranged to match.

This is a Go port of [Term::Tmux::Layout][perl], which it replaces. The port
was made utilizing a LLM for machine translation between languages.

[perl]: https://github.com/jmaslak/Term-Tmux-Layout

## Install

For Linux (amd64, arm64, or riscv64), macOS (amd64 or arm64), or FreeBSD
(amd64 or arm64), install a prebuilt release binary with:

```sh
curl -fsSL https://raw.githubusercontent.com/jmaslak/tmuxlayout/main/install.sh | sh
```

This installs to `/usr/local/bin`, falling back to `~/.local/bin` if that is
not writable. Once installed, `tmuxlayout -selfupdate` fetches and installs any
later release in place.

Windows (amd64 or arm64) builds are also published on the
[releases page][releases], but `install.sh` is a POSIX shell script and does
not run there; download the `.exe` directly, or use `-selfupdate` once it is in
place.

With a Go toolchain instead:

```sh
go install github.com/jmaslak/tmuxlayout/cmd/tmuxlayout@latest
```

Or build from a checkout:

```sh
go build ./cmd/tmuxlayout
```

The program has no dependencies outside the standard library.

## Use

Run it from inside the tmux window you want to rearrange. Each argument is a
row:

```sh
tmuxlayout abc def
```

Rows can be separated by pipes or newlines inside a single argument instead, so
these all do the same thing:

```sh
tmuxlayout 'abc|def|ghi'
tmuxlayout abc def ghi
tmuxlayout 'abc|def' ghi
printf 'abc\ndef\nghi\n' | tmuxlayout
```

With no arguments the layout is read from standard input, one row per line.

Which characters you use does not matter, only which cells share one. `11123`
and `aaabc` are the same layout. Every row must be the same length, since the
picture is a rectangle.

The window needs as many panes as the picture does before the layout will
apply: this rearranges panes, it does not create them. If it has the wrong
number, tmux says so and nothing moves.

A shell function for the layout you keep going back to is worth having. In
`~/.bashrc` or `~/.zshrc`:

```sh
wide() { tmuxlayout 11123 11124; }
```

### Options

| Option | Effect |
| --- | --- |
| `-print` | Print the layout string instead of applying it. |
| `-size WxH` | Render for a canvas of this size rather than asking tmux. Implies not needing a tmux at all, unless the layout is also being applied. |
| `-version` | Print the version. |
| `-selfupdate` | Fetch and install the latest release in place. |

`-print` and `-size` together are how to see what a layout would be without
being in tmux:

```sh
$ tmuxlayout -print -size 80x24 11123 11124
3e6d,80x24,0,0{48x24,0,0,100,15x24,49,0,100,15x24,65,0[15x12,65,0,100,15x11,65,13,100]}
```

The height is the tmux canvas, which is one row shorter than the terminal
window because of the status line.

### Layouts tmux cannot build

tmux makes a window by splitting it in two, then splitting each half again, so
every region has to come apart along a line that runs its full width or height.
Most pictures do. This one does not:

```
1122
1134
5554
```

There is no single horizontal or vertical line that divides it, and no way to
get there by splitting. `tmuxlayout` says so rather than producing something
that does not fit:

```
tmuxlayout: layout cannot be split into tmux panes:
1122
1134
5554
```

The other limit is size. Ten columns of panes need at least nineteen columns of
terminal — one per pane, and one for each border between them — and below that
there is no layout to render.

## As a library

```go
package main

import (
    "fmt"
    "log"

    "github.com/jmaslak/tmuxlayout/tmuxlayout"
)

func main() {
    // Rearrange the current tmux window.
    if _, err := tmuxlayout.Apply("11123", "11124"); err != nil {
        log.Fatal(err)
    }

    // Or just render the string, for a canvas of a stated size.
    l := &tmuxlayout.Layout{Width: 80, Height: 24}
    layout, err := l.Render("11123|11124")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(layout)
}
```

`Apply` renders for the size of the current tmux window and makes the result
that window's layout. `Layout.Render` does the rendering alone and needs no
tmux. `Checksum` exposes the checksum tmux prefixes a layout string with, and
`WindowSize` and `SelectLayout` are the two things this asks of tmux.

Render errors are worth telling apart, and wrap `ErrEmptyLayout`,
`ErrRaggedRows`, `ErrUnsplittable`, or `ErrTooSmall`; anything that talks to
tmux from outside tmux wraps `ErrNotInTmux`. Test them with `errors.Is`. Full
documentation is on [pkg.go.dev][godoc].

[godoc]: https://pkg.go.dev/github.com/jmaslak/tmuxlayout/tmuxlayout

## How it works

The picture is laid over a grid with one cell per character. A canvas of `S`
characters divided into `n` gives `n-1` of them to the borders between panes
and shares the rest out evenly, with any remainder going to the leftmost or
topmost divisions — which is exactly how tmux divides a window for its own
`even-horizontal` and `even-vertical` layouts.

Rendering is then a recursion. A region of the picture holding one pane is a
leaf. Otherwise, look for a column that no pane straddles; if there is one,
there is a vertical split there. Cut at *every* such column at once, not one at
a time, because tmux never nests a division inside another of the same
direction — splitting a pane in an already side-by-side window adds a pane to
that division rather than nesting a new one. If no column works, try rows the
same way. If neither does, the picture is not a layout tmux can build.

| File | Contents |
| --- | --- |
| `layout.go` | Reading the picture, dividing the canvas, and `Render`. |
| `divide.go` | Finding the splits tmux would have made, and rendering them. |
| `checksum.go` | The checksum tmux puts on the front of a layout string. |
| `tmux.go` | Asking tmux the window size, and handing it a layout. |

## Differences from the Perl version

The layout strings are not always the ones `Term::Tmux::Layout` produced, in
two ways, both deliberate.

**Pane sizes are off by one in the Perl version.** It adds a character to the
start of every pane that is not against the edge of the canvas, for the border
before it, but takes the same character off the width of the *first* pane
rather than each one. On an 80 column canvas split in two it produces
`{39x24,0,0,...,40x24,41,0,...}`, whose second pane runs to column 80 on a
canvas whose last column is 79. tmux accepts it, silently repairs the offset,
and leaves the panes at 39 and 40 columns where its own even split is 40 and
39. This produces the latter, and the difference is visible: the divider sits
one column over from where tmux would have put it.

**Divisions of three or more panes are flat.** Splitting three columns, the
Perl version emits one pane beside a nested division of the other two;
this emits all three side by side, which is the shape tmux itself writes and
means a later resize spreads across all three rather than treating two of them
as a unit.

Both are checked against a running tmux rather than against recorded
expectations — see `tmux_live_test.go`, which applies each layout to a real
window and requires tmux to report back exactly what it was handed. Rendering
an even split now produces the same string tmux does, byte for byte.

Smaller changes: empty rows are ignored, so a trailing newline or pipe is no
longer an error; a canvas too small for the layout is reported instead of
producing panes of zero or negative width; and running outside tmux is refused
up front, rather than asking a tmux server that would have answered for
whichever session it saw last and rearranged a window nobody was looking at.

## Development

```sh
go test ./...                                   # unit tests
go test -race ./...
go test -tags tmux ./tmuxlayout                 # against a real tmux
go test -fuzz FuzzRender ./tmuxlayout           # property fuzzing
go vet ./...
```

The `tmux` build tag is the interesting one. A layout string is only correct if
tmux agrees, and tmux does not say when it disagrees — handed a layout whose
panes do not quite tile the canvas it repairs it and carries on, so the panes
come out at the wrong sizes and nothing reports an error. Those tests apply
each layout to a real tmux window and compare what comes back.

## Bugs

Bug reports are welcome as [GitHub issues][issues], or by email to
<jmaslak@antelope.net>.

If you believe a bug has security implications, please report it privately by
email to <jmaslak@antelope.net> before disclosing it publicly, so there is a
chance to fix it first.

[issues]: https://github.com/jmaslak/tmuxlayout/issues
[releases]: https://github.com/jmaslak/tmuxlayout/releases

## Author

Joelle Maslak <jmaslak@antelope.net>

## License

Copyright (C) 2015-2026 Joelle Maslak.

Licensed under the Artistic License 2.0. See [LICENSE](LICENSE).
