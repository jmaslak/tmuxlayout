// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

package tmuxlayout

import (
	"fmt"
	"strings"
)

// grid holds where each division boundary falls on the canvas. hgrid[i] is the
// column the i'th division starts at and vgrid[j] the row the j'th starts at,
// each with a sentinel one past the end of the canvas.
type grid struct {
	hgrid []int
	vgrid []int
}

// section is one rectangle of the pane picture together with where it sits:
// hcell and vcell are its top left corner in grid cells, hcells and vcells its
// size in grid cells. There is one grid cell per character of the picture, so
// hcells is the length of a row and vcells the number of them.
type section struct {
	rows           [][]rune
	hcell, vcell   int
	hcells, vcells int
}

// axis is the direction a section is cut in: vertically puts the parts side by
// side, horizontally stacks them.
type axis int

const (
	vertically axis = iota
	horizontally
)

// brackets returns the pair tmux wraps a division of this direction in.
func (a axis) brackets() (byte, byte) {
	if a == vertically {
		return '{', '}'
	}
	return '[', ']'
}

// divide renders one section of the picture, recursing into the parts that the
// split tmux would have made here divides it into.
func (g *grid) divide(s section) (string, error) {
	// A section starts where its first grid division does and stops one
	// short of where the division after its last one starts, because that
	// character belongs to the border drawn between them.
	left := g.hgrid[s.hcell]
	top := g.vgrid[s.vcell]
	width := g.hgrid[s.hcell+s.hcells] - left - 1
	height := g.vgrid[s.vcell+s.vcells] - top - 1

	geometry := fmt.Sprintf("%dx%d,%d,%d", width, height, left, top)

	// A section of a single pane is a leaf. The trailing number is the pane
	// id, which tmux assigns itself when it applies a layout, so any value
	// will do.
	if singlePane(s.rows) {
		return geometry + ",100", nil
	}

	if cuts := verticalCuts(s.rows); len(cuts) > 0 {
		return g.split(s, geometry, vertically, cuts)
	}
	if cuts := horizontalCuts(s.rows); len(cuts) > 0 {
		return g.split(s, geometry, horizontally, cuts)
	}

	return "", fmt.Errorf("%w:\n%s", ErrUnsplittable, picture(s.rows))
}

// split renders the parts that cuts divides a section into and wraps them in
// the brackets tmux uses for a division of that direction.
//
// Every cut that can be made is made here, rather than one at a time with the
// rest left to a nested division, because that is the shape tmux gives a window
// it built itself: splitting a pane in a window that is already divided the
// same way adds a pane to that division instead of nesting a new one inside it,
// so a tmux layout never holds a division of the same direction as the one
// containing it.
func (g *grid) split(s section, geometry string, a axis, cuts []int) (string, error) {
	end := s.hcells
	if a == horizontally {
		end = s.vcells
	}

	from := 0
	parts := make([]string, 0, len(cuts)+1)
	for _, to := range append(cuts, end) {
		part, err := g.divide(s.part(a, from, to))
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
		from = to
	}

	opening, closing := a.brackets()
	return geometry + string(opening) + strings.Join(parts, ",") + string(closing), nil
}

// part carves the section between two cuts out of s, measuring from the start
// of s in grid cells.
func (s section) part(a axis, from, to int) section {
	part := s
	if a == vertically {
		part.hcell, part.hcells = s.hcell+from, to-from
		part.rows = make([][]rune, len(s.rows))
		for i, row := range s.rows {
			part.rows[i] = row[from:to]
		}
		return part
	}

	part.vcell, part.vcells = s.vcell+from, to-from
	part.rows = s.rows[from:to]
	return part
}

// singlePane reports whether every cell of rows names the same pane.
func singlePane(rows [][]rune) bool {
	first := rows[0][0]
	for _, row := range rows {
		for _, c := range row {
			if c != first {
				return false
			}
		}
	}
	return true
}

// verticalCuts returns every column the picture can be cut down.
func verticalCuts(rows [][]rune) []int {
	var cuts []int
	for col := 1; col < len(rows[0]); col++ {
		if !straddlesColumn(rows, col) {
			cuts = append(cuts, col)
		}
	}
	return cuts
}

// horizontalCuts returns every row the picture can be cut across.
func horizontalCuts(rows [][]rune) []int {
	var cuts []int
	for row := 1; row < len(rows); row++ {
		if !straddlesRow(rows[row-1], rows[row]) {
			cuts = append(cuts, row)
		}
	}
	return cuts
}

// straddlesColumn reports whether a pane sits on both sides of the line down
// the left of col, which is what stops the picture being cut there.
func straddlesColumn(rows [][]rune, col int) bool {
	for _, row := range rows {
		if row[col-1] == row[col] {
			return true
		}
	}
	return false
}

// straddlesRow reports whether a pane sits in both of two neighbouring rows,
// which is what stops the picture being cut between them.
func straddlesRow(above, below []rune) bool {
	for col := range above {
		if above[col] == below[col] {
			return true
		}
	}
	return false
}

// picture renders a pane map back into the form it was written in, for error
// messages that name the region that could not be split.
func picture(rows [][]rune) string {
	var b strings.Builder
	for _, row := range rows {
		b.WriteString(string(row))
		b.WriteByte('\n')
	}
	return b.String()
}
