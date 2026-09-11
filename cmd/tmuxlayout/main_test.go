// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jmaslak/tmuxlayout/tmuxlayout"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		in            string
		width, height int
		wantErr       bool
	}{
		{in: "80x24", width: 80, height: 24},
		{in: "204x49", width: 204, height: 49},
		{in: "1x1", width: 1, height: 1},
		{in: "80", wantErr: true},
		{in: "80x", wantErr: true},
		{in: "x24", wantErr: true},
		{in: "80X24", wantErr: true},
		{in: "eighty x 24", wantErr: true},
		{in: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			width, height, err := parseSize(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseSize(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if err == nil && (width != tt.width || height != tt.height) {
				t.Errorf("parseSize(%q) = %dx%d, want %dx%d", tt.in, width, height, tt.width, tt.height)
			}
		})
	}
}

func TestDescription(t *testing.T) {
	t.Run("arguments win", func(t *testing.T) {
		got, err := description([]string{"ab", "cd"}, strings.NewReader("ignored\n"))
		if err != nil {
			t.Fatalf("description: %v", err)
		}
		if len(got) != 2 || got[0] != "ab" || got[1] != "cd" {
			t.Errorf("description = %q, want [ab cd]", got)
		}
	})

	t.Run("standard input when there are none", func(t *testing.T) {
		got, err := description(nil, strings.NewReader("ab\ncd\n"))
		if err != nil {
			t.Fatalf("description: %v", err)
		}
		if len(got) != 2 || got[0] != "ab" || got[1] != "cd" {
			t.Errorf("description = %q, want [ab cd]", got)
		}
	})

	t.Run("empty standard input", func(t *testing.T) {
		if got, err := description(nil, strings.NewReader("")); !errors.Is(err, tmuxlayout.ErrEmptyLayout) {
			t.Errorf("description = %q, %v; want ErrEmptyLayout", got, err)
		}
	})
}

// stubTmux replaces the two things run does to tmux, and returns a pointer to
// the layout string that reached select-layout.
func stubTmux(t *testing.T, width, height int, selectErr error) *string {
	t.Helper()

	savedSize, savedSelect := windowSize, selectLayout
	windowSize = func() (int, int, error) { return width, height, nil }

	var applied string
	selectLayout = func(layout string) error {
		applied = layout
		return selectErr
	}

	t.Cleanup(func() { windowSize, selectLayout = savedSize, savedSelect })
	return &applied
}

func TestRunAppliesLayout(t *testing.T) {
	applied := stubTmux(t, 80, 24, nil)
	var out bytes.Buffer

	if err := run(&out, []string{"xY"}, "", false); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := "ac4b,80x24,0,0{40x24,0,0,100,39x24,41,0,100}"
	if *applied != want {
		t.Errorf("run applied %q, want %q", *applied, want)
	}
	// Applying a layout is its own feedback: the panes move.
	if out.Len() != 0 {
		t.Errorf("run wrote %q to standard output, want nothing", out.String())
	}
}

func TestRunPrintsWithoutApplying(t *testing.T) {
	applied := stubTmux(t, 80, 24, nil)
	var out bytes.Buffer

	if err := run(&out, []string{"xY"}, "", true); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := "ac4b,80x24,0,0{40x24,0,0,100,39x24,41,0,100}\n"
	if out.String() != want {
		t.Errorf("run printed %q, want %q", out.String(), want)
	}
	if *applied != "" {
		t.Errorf("run applied %q with -print, want nothing", *applied)
	}
}

// TestRunSizeDoesNotAskTmux covers rendering for a stated size, which has to
// work with no tmux to ask.
func TestRunSizeDoesNotAskTmux(t *testing.T) {
	savedSize, savedSelect := windowSize, selectLayout
	windowSize = func() (int, int, error) {
		t.Error("run asked tmux for the window size despite -size")
		return 0, 0, errors.New("should not be called")
	}
	selectLayout = func(string) error { return nil }
	t.Cleanup(func() { windowSize, selectLayout = savedSize, savedSelect })

	var out bytes.Buffer
	if err := run(&out, []string{"xY"}, "204x49", true); err != nil {
		t.Fatalf("run: %v", err)
	}

	if !strings.Contains(out.String(), "204x49,0,0") {
		t.Errorf("run printed %q, want a layout for a 204x49 canvas", out.String())
	}
}

func TestRunErrors(t *testing.T) {
	tests := []struct {
		name string
		def  []string
		size string
		want error
	}{
		{name: "unsplittable", def: []string{"1122", "1134", "5554"}, want: tmuxlayout.ErrUnsplittable},
		{name: "ragged", def: []string{"abc", "de"}, want: tmuxlayout.ErrRaggedRows},
		{name: "too small for the canvas", def: []string{"abcd"}, size: "4x24", want: tmuxlayout.ErrTooSmall},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applied := stubTmux(t, 80, 24, nil)
			var out bytes.Buffer

			if err := run(&out, tt.def, tt.size, false); !errors.Is(err, tt.want) {
				t.Fatalf("run = %v, want %v", err, tt.want)
			}
			// A layout that does not render must leave the window alone.
			if *applied != "" {
				t.Errorf("run applied %q after failing to render", *applied)
			}
		})
	}
}

// TestRunBadSize checks that a malformed -size is reported rather than falling
// back to tmux or to the default canvas.
func TestRunBadSize(t *testing.T) {
	stubTmux(t, 80, 24, nil)
	var out bytes.Buffer

	err := run(&out, []string{"xY"}, "wide", false)
	if err == nil {
		t.Fatal("run with -size wide = nil, want an error")
	}
	if !strings.Contains(err.Error(), "WIDTHxHEIGHT") {
		t.Errorf("run with -size wide = %q, want it to say how a size is written", err)
	}
}

// TestRunReportsSelectLayoutFailure covers tmux refusing the layout, which it
// does when the window has the wrong number of panes.
func TestRunReportsSelectLayoutFailure(t *testing.T) {
	wantErr := errors.New("have 2 panes but need 4")
	stubTmux(t, 80, 24, wantErr)
	var out bytes.Buffer

	if err := run(&out, []string{"ab", "cd"}, "", false); !errors.Is(err, wantErr) {
		t.Errorf("run = %v, want %v", err, wantErr)
	}
}
