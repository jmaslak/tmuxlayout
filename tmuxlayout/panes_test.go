// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

package tmuxlayout

import (
	"fmt"
	"regexp"
	"strconv"
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
