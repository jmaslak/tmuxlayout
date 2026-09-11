// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

// Package tmuxlayout builds tmux layout strings from a picture of the panes.
//
// A layout is written as a grid of characters, one character per pane, where
// every cell holding the same character belongs to the same pane:
//
//	11123
//	11124
//
// describes four panes — pane 1 covering three fifths of the width and the
// full height, pane 2 a fifth of the width and the full height, and panes 3
// and 4 stacked in the remaining fifth:
//
//	+------+--+--+
//	|      |  |  |
//	|      |  |  |
//	|      |  +--+
//	|      |  |  |
//	|      |  |  |
//	+------+--+--+
//
// [Layout.Render] turns that picture into the checksum-prefixed string tmux's
// select-layout command expects, and [Apply] hands it to the running tmux.
package tmuxlayout

import (
	"errors"
	"fmt"
	"strings"
)

// The canvas size a Layout starts with, matching tmux's own default window
// size. It is the whole canvas: the height excludes the status line, so it is
// one row less than the terminal emulator's height.
const (
	DefaultWidth  = 80
	DefaultHeight = 24
)

// Errors reported by [Layout.Render]. They are wrapped with the detail of what
// went wrong, so test them with [errors.Is].
var (
	// ErrEmptyLayout is returned when the description holds no rows at all.
	ErrEmptyLayout = errors.New("empty layout")

	// ErrRaggedRows is returned when the rows are not all the same length.
	// The description is a rectangle; a short row is a typo, not a shape.
	ErrRaggedRows = errors.New("all rows must be the same length")

	// ErrUnsplittable is returned for a picture tmux cannot express. tmux
	// builds a window by splitting it in two, then splitting each half, and
	// so on, so every region has to come apart along a full-width or
	// full-height line. "1122|1134|5554" does not.
	ErrUnsplittable = errors.New("layout cannot be split into tmux panes")

	// ErrTooSmall is returned when the canvas has too few rows or columns
	// for the layout: some pane would be left no room once the borders
	// between panes are accounted for.
	ErrTooSmall = errors.New("canvas too small for layout")
)

// A Layout renders pane pictures onto a canvas of a fixed size. The zero value
// is not useful; call [New], or set both fields.
type Layout struct {
	// Width is the width of the tmux canvas, in columns.
	Width int

	// Height is the height of the tmux canvas, in rows. This is the canvas
	// only: it excludes tmux's status line, so it is one less than the
	// height of the terminal emulator window.
	Height int
}

// New returns a Layout for tmux's default 80x24 canvas.
func New() *Layout {
	return &Layout{Width: DefaultWidth, Height: DefaultHeight}
}

// Render turns a pane picture into a tmux layout string: a four hex digit
// checksum, a comma, and the nested description of the panes.
//
// Rows are separated by newlines or pipes, and each argument also starts a new
// row, so these all describe the same three by three grid:
//
//	l.Render("abc|def|ghi")
//	l.Render("abc\ndef\nghi")
//	l.Render("abc", "def", "ghi")
//	l.Render("abc|def", "ghi")
//
// Empty rows are ignored, so a trailing newline or pipe is harmless. Every
// remaining row must be the same length.
func (l *Layout) Render(def ...string) (string, error) {
	rows, err := parseMap(def)
	if err != nil {
		return "", err
	}

	// n divisions need n characters of their own and n-1 between them for
	// the borders. Below that some pane would be left nothing, so there is
	// no layout to render rather than a cramped one.
	cols := len(rows[0])
	minWidth, minHeight := 2*cols-1, 2*len(rows)-1
	if l.Width < minWidth || l.Height < minHeight {
		return "", fmt.Errorf("%w: %s by %s of panes need a canvas of at least %dx%d, but this one is %dx%d",
			ErrTooSmall, plural(cols, "column"), plural(len(rows), "row"), minWidth, minHeight, l.Width, l.Height)
	}

	g := &grid{
		hgrid: divisions(l.Width, cols),
		vgrid: divisions(l.Height, len(rows)),
	}

	body, err := g.divide(section{rows: rows, hcells: cols, vcells: len(rows)})
	if err != nil {
		return "", err
	}

	return Checksum(body) + "," + body, nil
}

// divisions returns the offset at which each of n equal divisions of a span of
// the given size begins, followed by a sentinel one past the end of the span.
// The span of divisions [i, j) is therefore offsets[j]-offsets[i]-1 characters
// wide: one character goes to the border drawn after it, and the sentinel
// accounts for the border the last division does not need.
//
// The n-1 borders come out of the span first, and what is left over is shared
// out evenly, with any remainder going to the leftmost or topmost divisions,
// so that no division is more than one character bigger than another.
//
// Where the remainder goes is a choice rather than a rule, and tmux makes it
// differently in different versions: tmux 3.6 also gives it to the leftmost
// divisions, while older ones hand all of it to the last division, which for
// six columns of an 80 character canvas is 12,12,12,12,12,15 against
// 13,13,13,12,12,12. Both are even splits, and tmux accepts either.
//
// size must be at least 2n-1, or some division is left no room at all; Render
// rejects a layout that small before it gets here.
func divisions(size, n int) []int {
	available := size - (n - 1)
	each, remainder := available/n, available%n

	offsets := make([]int, n+1)
	for i := range n {
		width := each
		if i < remainder {
			width++
		}
		offsets[i+1] = offsets[i] + width + 1
	}
	return offsets
}

// parseMap splits a pane description into a rectangle of pane characters.
//
// A cell is one character, not one byte: which characters name the panes is up
// to whoever wrote the picture, and "░░|▓▓" is two panes side by side rather
// than six.
func parseMap(def []string) ([][]rune, error) {
	var rows [][]rune
	for line := range strings.SplitSeq(strings.Join(def, "|"), "|") {
		for row := range strings.SplitSeq(line, "\n") {
			// A trailing newline off a here-doc or a pipe at the end
			// of an argument is punctuation, not a row of zero panes.
			if row == "" {
				continue
			}
			rows = append(rows, []rune(row))
		}
	}

	if len(rows) == 0 {
		return nil, ErrEmptyLayout
	}
	for i, row := range rows {
		if len(row) != len(rows[0]) {
			return nil, fmt.Errorf("%w: row 1 is %d characters but row %d is %d",
				ErrRaggedRows, len(rows[0]), i+1, len(row))
		}
	}
	return rows, nil
}

// plural renders a count with its noun, for error messages that would
// otherwise say "1 rows".
func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
