// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

package tmuxlayout

import (
	"strings"
	"testing"
)

// FuzzRender checks that no description can make Render panic or loop, and
// that anything it does accept is a well formed layout string whose panes fit
// the canvas without overlapping.
func FuzzRender(f *testing.F) {
	for _, seed := range []string{
		"x", "xY", "xYYz", "ab|cc", "11123|11124", "1122|1134|5554",
		"", "|", "a|bb", "\n\n", "aa|aa", strings.Repeat("a", 200),
	} {
		f.Add(seed, 80, 24)
	}

	f.Fuzz(func(t *testing.T, def string, width, height int) {
		// Only sizes a terminal could plausibly have: the arithmetic is
		// not defined for a canvas of zero or negative size, and a huge
		// one just makes the test slow.
		if width < 1 || width > 1000 || height < 1 || height > 1000 {
			t.Skip()
		}
		// Likewise, a description with a million rows says nothing a
		// small one does not.
		if len(def) > 256 {
			t.Skip()
		}

		l := &Layout{Width: width, Height: height}
		got, err := l.Render(def)
		if err != nil {
			return
		}

		sum, body, ok := strings.Cut(got, ",")
		if !ok {
			t.Fatalf("Render(%q) on %dx%d = %q, which has no checksum prefix", def, width, height, got)
		}
		if len(sum) != 4 {
			t.Errorf("Render(%q) on %dx%d = %q, whose checksum is not four digits", def, width, height, got)
		}
		if want := Checksum(body); sum != want {
			t.Errorf("Render(%q) on %dx%d = %q, whose checksum should be %q", def, width, height, got, want)
		}
		checkPanes(t, got, def, l)
	})
}
