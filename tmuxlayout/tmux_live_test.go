// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

//go:build tmux

// This test needs a real tmux to talk to, so it is behind a build tag:
//
//	go test -tags tmux -run TestAgainstTmux ./tmuxlayout
//
// It is the test that matters most. A layout string is only correct if tmux
// agrees it is, and tmux does not say when it disagrees: given a layout whose
// panes do not quite tile the canvas it silently repairs it and carries on, so
// the panes come out at the wrong sizes and nothing reports an error. Checking
// Render against expectations recorded in another test file cannot catch that.
// Applying the layout and asking tmux what it ended up with can.
package tmuxlayout

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestAgainstTmux renders a layout, applies it to a real tmux window of the
// same size and pane count, and requires tmux to report back exactly what it
// was given.
func TestAgainstTmux(t *testing.T) {
	requireTmux(t)

	tests := []struct {
		width, height int
		def           string
	}{
		{80, 24, "x"},
		{80, 24, "xY"},
		{80, 24, "xYYz"},
		{80, 24, "ab|cc"},
		{80, 24, "abc|def"},
		{80, 24, "11123|11124"},
		{80, 24, "abc|abc|ddd"},
		{80, 24, "aab|aab|ccb"},
		{80, 24, "abcd|abcd|efgh|efgh"},
		{80, 24, "abc|abd|eed"},
		{204, 49, "ab|cc"},
		{204, 49, "abcde"},
		{204, 49, "11123|11124"},
		{364, 94, "ab|cd"},
		{364, 94, "abcd|abcd|efgh|efgh"},
		{100, 30, "abc|def"},
		{50, 20, "aabb|aabb|ccdd"},
		{37, 11, "xY"},
		// Small canvases, where the rounding has the least room to be
		// wrong. tmux will not create panes much smaller than this, so
		// the exactly-as-small-as-it-fits cases are left to
		// TestRenderSmallestCanvas, which needs no tmux to check them.
		{20, 12, "ab|cd"},
		{15, 9, "abc|def"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%dx%d/%s", tt.width, tt.height, tt.def), func(t *testing.T) {
			l := &Layout{Width: tt.width, Height: tt.height}
			want, err := l.Render(tt.def)
			if err != nil {
				t.Fatalf("Render(%q) on %dx%d: %v", tt.def, tt.width, tt.height, err)
			}

			got := applyInTmux(t, tt.width, tt.height, panesIn(tt.def), want)
			if normalize(got) != normalize(want) {
				t.Errorf("tmux changed the layout, so it was not one tmux would write\n ours: %s\n tmux: %s",
					normalize(want), normalize(got))
			}
		})
	}
}

// TestEvenLayoutsMatchTmux checks the rendered layout against the one tmux
// builds itself for its even-horizontal and even-vertical layouts. Those are
// the only layouts tmux and this package both have an opinion about, so they
// are the only place the division arithmetic can be compared directly.
func TestEvenLayoutsMatchTmux(t *testing.T) {
	requireTmux(t)

	const width, height = 80, 24
	for panes := 1; panes <= 6; panes++ {
		for _, even := range []struct{ name, def string }{
			{"even-horizontal", strings.Join(columns(panes), "")},
			{"even-vertical", strings.Join(columns(panes), "|")},
		} {
			t.Run(fmt.Sprintf("%s/%d", even.name, panes), func(t *testing.T) {
				session := newSession(t, width, height, panes)
				tmuxOrFail(t, "select-layout", "-t", session, even.name)
				want := tmuxOrFail(t, "list-windows", "-t", session, "-F", "#{window_layout}")

				l := &Layout{Width: width, Height: height}
				got, err := l.Render(even.def)
				if err != nil {
					t.Fatalf("Render(%q): %v", even.def, err)
				}

				if normalize(got) != normalize(want) {
					t.Errorf("Render(%q) does not divide the window the way tmux's %s does\n ours: %s\n tmux: %s",
						even.def, even.name, normalize(got), normalize(want))
				}
			})
		}
	}
}

// columns returns n distinct single character pane names.
func columns(n int) []string {
	names := make([]string, n)
	for i := range names {
		names[i] = string(rune('a' + i))
	}
	return names
}

func requireTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux is not installed")
	}
}

// panesIn counts the distinct panes a description names.
func panesIn(def string) int {
	seen := map[byte]bool{}
	for i := range len(def) {
		if def[i] != '|' && def[i] != '\n' {
			seen[def[i]] = true
		}
	}
	return len(seen)
}

// newSession starts a detached tmux session of the given size and splits it
// until it holds the requested number of panes, returning the session name.
// Re-tiling between splits keeps every pane big enough to split again.
func newSession(t *testing.T, width, height, panes int) string {
	t.Helper()

	session := fmt.Sprintf("tmuxlayout-test-%d-%s", panes, strings.ReplaceAll(t.Name(), "/", "-"))
	tmuxOrFail(t, "new-session", "-d", "-x", strconv.Itoa(width), "-y", strconv.Itoa(height), "-s", session)
	t.Cleanup(func() { runTmux("kill-session", "-t", session) })

	for range panes - 1 {
		tmuxOrFail(t, "select-layout", "-t", session, "tiled")
		tmuxOrFail(t, "split-window", "-t", session)
	}

	if got := len(strings.Fields(tmuxOrFail(t, "list-panes", "-t", session, "-F", "#"))); got != panes {
		t.Fatalf("set up a window with %d panes, want %d", got, panes)
	}
	return session
}

// applyInTmux makes layout the layout of a fresh window of the given size and
// pane count, and returns what tmux reports the window's layout to be after.
func applyInTmux(t *testing.T, width, height, panes int, layout string) string {
	t.Helper()

	session := newSession(t, width, height, panes)
	tmuxOrFail(t, "select-layout", "-t", session, layout)
	return tmuxOrFail(t, "list-windows", "-t", session, "-F", "#{window_layout}")
}

func tmuxOrFail(t *testing.T, args ...string) string {
	t.Helper()

	out, err := runTmux(args...)
	if err != nil {
		t.Fatalf("tmux %s: %v", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out))
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
