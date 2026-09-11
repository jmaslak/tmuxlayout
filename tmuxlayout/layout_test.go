// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

package tmuxlayout

import (
	"errors"
	"strings"
	"testing"
)

// TestChecksum uses layout strings captured from a running tmux, so a change
// to the checksum shows up as a layout tmux would reject.
func TestChecksum(t *testing.T) {
	tests := []struct {
		layout string
		want   string
	}{
		{"364x94,0,0,9", "c846"},
		{"364x94,0,0{91x94,0,0,45,90x94,92,0,48,90x94,183,0,46,90x94,274,0,47}", "4f55"},
		{
			"364x94,0,0[364x31,0,0{91x31,0,0,0,90x31,92,0,1,90x31,183,0,34,90x31,274,0,30}," +
				"364x30,0,32{182x30,0,32,39,90x30,183,32,7,90x30,274,32,40}," +
				"364x31,0,63{182x31,0,63,8,181x31,183,63,44}]",
			"f245",
		},
		{"", "0000"},
	}

	for _, tt := range tests {
		if got := Checksum(tt.layout); got != tt.want {
			t.Errorf("Checksum(%q) = %q, want %q", tt.layout, got, tt.want)
		}
	}
}

// TestChecksumIgnoresTrailingNewline covers passing in a layout read from a
// command's output without trimming it first.
func TestChecksumIgnoresTrailingNewline(t *testing.T) {
	if got, want := Checksum("364x94,0,0,9\n"), Checksum("364x94,0,0,9"); got != want {
		t.Errorf("Checksum with a trailing newline = %q, want %q", got, want)
	}
}

func TestRender(t *testing.T) {
	tests := []struct {
		name string
		def  []string
		want string
	}{
		{"one pane", []string{"x"}, "acdf,80x24,0,0,100"},
		{"one pane, two cells wide", []string{"xx"}, "acdf,80x24,0,0,100"},
		{"one pane, two cells tall", []string{"x\nx"}, "acdf,80x24,0,0,100"},
		{"one pane, four cells", []string{"xx\nxx"}, "acdf,80x24,0,0,100"},
		{"two columns", []string{"xY"}, "ac4b,80x24,0,0{40x24,0,0,100,39x24,41,0,100}"},
		{"two columns, doubled", []string{"xxYY"}, "ac4b,80x24,0,0{40x24,0,0,100,39x24,41,0,100}"},
		{"two columns, two rows", []string{"xY\nxY"}, "ac4b,80x24,0,0{40x24,0,0,100,39x24,41,0,100}"},
		{"two columns, doubled both ways", []string{"xxYY\nxxYY"}, "ac4b,80x24,0,0{40x24,0,0,100,39x24,41,0,100}"},
		{"uneven columns", []string{"xYY\nxYY"}, "b6c3,80x24,0,0{26x24,0,0,100,53x24,27,0,100}"},
		{
			"three columns", []string{"xYYz"},
			"2fae,80x24,0,0{20x24,0,0,100,39x24,21,0,100,19x24,61,0,100}",
		},
		{
			"rows separated by pipes", []string{"xxYY|xxYY"},
			"ac4b,80x24,0,0{40x24,0,0,100,39x24,41,0,100}",
		},
		{
			"one row per argument", []string{"xxYY", "xxYY"},
			"ac4b,80x24,0,0{40x24,0,0,100,39x24,41,0,100}",
		},
		{
			"pipes and arguments mixed", []string{"xxYY|xxYY", "xxYY"},
			"ac4b,80x24,0,0{40x24,0,0,100,39x24,41,0,100}",
		},
		{
			"trailing newline ignored", []string{"xxYY\nxxYY\n"},
			"ac4b,80x24,0,0{40x24,0,0,100,39x24,41,0,100}",
		},
		{
			"split across rows then columns", []string{"ab", "cc"},
			"1519,80x24,0,0[80x12,0,0{40x12,0,0,100,39x12,41,0,100},80x11,0,13,100]",
		},
	}

	l := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := l.Render(tt.def...)
			if err != nil {
				t.Fatalf("Render(%q): %v", tt.def, err)
			}
			if got != tt.want {
				t.Errorf("Render(%q)\n got %q\nwant %q", tt.def, got, tt.want)
			}
		})
	}
}

// TestRenderChecksumMatchesBody guards against the checksum and the layout it
// covers drifting apart, which tmux would reject without saying why.
func TestRenderChecksumMatchesBody(t *testing.T) {
	got, err := New().Render("abc|abd|eed")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	sum, body, ok := strings.Cut(got, ",")
	if !ok {
		t.Fatalf("Render returned %q, which has no checksum prefix", got)
	}
	if want := Checksum(body); sum != want {
		t.Errorf("Render returned checksum %q for %q, want %q", sum, body, want)
	}
}

// TestRenderCountsCharactersNotBytes covers a picture drawn with characters
// that are more than one byte: each is one pane, not one per byte.
func TestRenderCountsCharactersNotBytes(t *testing.T) {
	l := New()

	got, err := l.Render("░░▓▓")
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if want, _ := l.Render("aabb"); got != want {
		t.Errorf("Render(%q) = %q, want the same layout as %q: %q", "░░▓▓", got, "aabb", want)
	}
	if n := len(leafPanes(t, got)); n != 2 {
		t.Errorf("Render(%q) produced %d panes, want 2", "░░▓▓", n)
	}
}

// TestRaggedRowsCountsCharacters checks that the error a ragged picture gets
// measures rows in panes rather than in bytes.
func TestRaggedRowsCountsCharacters(t *testing.T) {
	_, err := New().Render("░░", "░")
	if !errors.Is(err, ErrRaggedRows) {
		t.Fatalf("Render = %v, want ErrRaggedRows", err)
	}
	if !strings.Contains(err.Error(), "row 1 is 2 characters but row 2 is 1") {
		t.Errorf("Render = %q, want it to count characters, not bytes", err)
	}
}

func TestRenderErrors(t *testing.T) {
	tests := []struct {
		name string
		def  []string
		want error
	}{
		{"no arguments", nil, ErrEmptyLayout},
		{"empty string", []string{""}, ErrEmptyLayout},
		{"only separators", []string{"|\n|"}, ErrEmptyLayout},
		{"ragged rows", []string{"abc", "de"}, ErrRaggedRows},
		{"ragged rows, long second", []string{"ab", "cde"}, ErrRaggedRows},
		{"unsplittable", []string{"1122", "1134", "5554"}, ErrUnsplittable},
		{"unsplittable pinwheel", []string{"aab", "cab", "ccb"}, ErrUnsplittable},
	}

	l := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := l.Render(tt.def...)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Render(%q) = %q, %v; want error %v", tt.def, got, err, tt.want)
			}
		})
	}
}

// TestRenderTooSmall covers a canvas with no room for the panes asked of it.
// n divisions fit in exactly 2n-1 characters and no fewer, so each case here
// is one character short of a layout that renders.
func TestRenderTooSmall(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		def           string
	}{
		{"too narrow by one", 4, 24, "abc"},
		{"too short by one", 80, 4, "a|b|c"},
		{"far too narrow", 4, 24, "abcd"},
		{"no canvas at all", 0, 0, "x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Layout{Width: tt.width, Height: tt.height}
			got, err := l.Render(tt.def)
			if !errors.Is(err, ErrTooSmall) {
				t.Fatalf("Render(%q) on %dx%d = %q, %v; want ErrTooSmall",
					tt.def, tt.width, tt.height, got, err)
			}
		})
	}
}

// TestRenderSmallestCanvas checks the other side of the 2n-1 boundary: a
// canvas exactly big enough renders, and every pane on it is one character.
func TestRenderSmallestCanvas(t *testing.T) {
	l := &Layout{Width: 5, Height: 5}

	got, err := l.Render("abc", "def", "ghi")
	if err != nil {
		t.Fatalf("Render on the smallest canvas that fits: %v", err)
	}
	for _, p := range leafPanes(t, got) {
		if p.w != 1 || p.h != 1 {
			t.Errorf("Render on a 5x5 canvas produced pane %v, want every pane 1x1", p)
		}
	}
	checkPanes(t, got, "abc|def|ghi", l)
}

// TestRenderPanesFitCanvas checks the arithmetic against the canvas rather
// than against a captured string: every pane has to land inside the canvas,
// and no two panes may overlap once the borders between them are counted.
func TestRenderPanesFitCanvas(t *testing.T) {
	defs := []string{
		"x", "xY", "xYYz", "ab|cc", "abc|abc|ddd", "11123|11124",
		"abcd|abcd|efgh|efgh", "aab|aab|ccb",
	}
	sizes := []struct{ w, h int }{{80, 24}, {204, 49}, {364, 94}, {37, 11}}

	for _, size := range sizes {
		for _, def := range defs {
			l := &Layout{Width: size.w, Height: size.h}
			got, err := l.Render(def)
			if err != nil {
				t.Errorf("Render(%q) on %dx%d: %v", def, size.w, size.h, err)
				continue
			}
			checkPanes(t, got, def, l)
		}
	}
}

// checkPanes parses the leaf panes out of a layout string and verifies that
// each is on the canvas and that none overlaps another, counting the border
// column and row that tmux draws after every pane that is not at the edge.
func checkPanes(t *testing.T, layout, def string, l *Layout) {
	t.Helper()

	occupied := map[[2]int]string{}
	for _, p := range leafPanes(t, layout) {
		if p.w <= 0 || p.h <= 0 {
			t.Errorf("Render(%q) on %dx%d: pane %v has no area", def, l.Width, l.Height, p)
		}
		if p.x < 0 || p.y < 0 || p.x+p.w > l.Width || p.y+p.h > l.Height {
			t.Errorf("Render(%q) on %dx%d: pane %v falls outside the canvas",
				def, l.Width, l.Height, p)
		}
		for y := p.y; y < p.y+p.h; y++ {
			for x := p.x; x < p.x+p.w; x++ {
				if prev, ok := occupied[[2]int{x, y}]; ok {
					t.Fatalf("Render(%q) on %dx%d: panes %v and %s overlap at %d,%d",
						def, l.Width, l.Height, p, prev, x, y)
				}
				occupied[[2]int{x, y}] = p.String()
			}
		}
	}
}
