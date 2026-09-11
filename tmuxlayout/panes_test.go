// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

package tmuxlayout

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// pane is one leaf of a layout string: a rectangle of the canvas.
type pane struct {
	w, h, x, y int
}

func (p pane) String() string { return fmt.Sprintf("%dx%d+%d+%d", p.w, p.h, p.x, p.y) }

// leafPattern matches a leaf of a layout string, "WxH,X,Y,id". A division
// rather than a leaf has a brace or a bracket where the pane id would be, so
// it does not match.
var leafPattern = regexp.MustCompile(`(\d+)x(\d+),(\d+),(\d+),\d+`)

// leafPanes pulls the panes out of a layout string, ignoring the divisions
// that hold them.
func leafPanes(t *testing.T, layout string) []pane {
	t.Helper()

	var panes []pane
	for _, m := range leafPattern.FindAllStringSubmatch(layout, -1) {
		n := make([]int, 4)
		for i := range n {
			v, err := strconv.Atoi(m[i+1])
			if err != nil {
				t.Fatalf("parsing %q out of layout %q: %v", m[0], layout, err)
			}
			n[i] = v
		}
		panes = append(panes, pane{w: n[0], h: n[1], x: n[2], y: n[3]})
	}
	if len(panes) == 0 {
		t.Fatalf("no panes found in layout %q", layout)
	}
	return panes
}

// paneID matches the pane id on the end of a leaf of a layout string.
var paneID = regexp.MustCompile(`(\d+x\d+,\d+,\d+),\d+`)

// normalize strips the parts of a layout string that are not geometry: the
// checksum, which covers the pane ids, and the pane ids themselves, which tmux
// assigns and this package cannot know.
func normalize(layout string) string {
	_, body, ok := strings.Cut(layout, ",")
	if !ok {
		body = layout
	}
	return paneID.ReplaceAllString(body, "$1,0")
}

// geometry matches the size and position on the front of a leaf or a division.
var geometry = regexp.MustCompile(`\d+x\d+,\d+,\d+`)

// shape reduces a layout string to its structure, with every leaf written as L
// and the sizes and positions dropped, so that two layouts built the same way
// compare equal whatever sizes they came out at. It turns
// "80x24,0,0{40x24,0,0,1,39x24,41,0,2}" into "{L,L}".
func shape(layout string) string {
	s := paneID.ReplaceAllString(normalize(layout), "L")
	return geometry.ReplaceAllString(s, "")
}

// columns returns n distinct single character pane names.
func columns(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = string(rune('a' + i))
	}
	return names
}

// Layout strings recorded from two tmux versions for the same even split of an
// 80x24 window into six columns. They divide the leftover characters
// differently — 3.6 spreads them across the leftmost panes, the older one puts
// the whole remainder on the last — so the sizes differ while the structure
// does not.
const (
	evenSixTmux36  = "0f4a,80x24,0,0{13x24,0,0,72,13x24,14,0,73,13x24,28,0,74,12x24,42,0,75,12x24,55,0,76,12x24,68,0,77}"
	evenSixTmuxOld = "0f4a,80x24,0,0{12x24,0,0,0,12x24,13,0,0,12x24,26,0,0,12x24,39,0,0,12x24,52,0,0,15x24,65,0,0}"
)

// TestShapeIgnoresSizes checks the comparison the tmux tagged tests rely on:
// two layouts built the same way compare equal even when their panes came out
// at different sizes. Without this, those tests pin whichever tmux is
// installed, and they did — CI's tmux failed them for producing a legitimate
// even split of its own.
func TestShapeIgnoresSizes(t *testing.T) {
	ours, err := (&Layout{Width: 80, Height: 24}).Render("abcdef")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	const want = "{L,L,L,L,L,L}"
	for name, layout := range map[string]string{
		"ours":       ours,
		"tmux 3.6":   evenSixTmux36,
		"older tmux": evenSixTmuxOld,
	} {
		if got := shape(layout); got != want {
			t.Errorf("shape of the %s layout = %q, want %q", name, got, want)
		}
	}
}

// TestShapeDistinguishesNesting checks that shape has not been flattened into
// something that ignores structure too, which would make the tmux tests pass
// on the nested output this deliberately does not produce.
func TestShapeDistinguishesNesting(t *testing.T) {
	flat := "2fae,80x24,0,0{20x24,0,0,100,39x24,21,0,100,19x24,61,0,100}"
	nested := "7858,80x24,0,0{20x24,0,0,100,59x24,21,0{39x24,21,0,100,19x24,61,0,100}}"

	if shape(flat) == shape(nested) {
		t.Errorf("shape does not tell a flat division from a nested one: both are %q", shape(flat))
	}
	if got, want := shape(nested), "{L,{L,L}}"; got != want {
		t.Errorf("shape of a nested division = %q, want %q", got, want)
	}
}
