// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

package tmuxlayout

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

// stubTmux replaces the tmux command for the duration of a test, recording the
// arguments it was called with and replying with canned output.
func stubTmux(t *testing.T, reply func(args []string) (string, error)) *[][]string {
	t.Helper()

	var calls [][]string
	saved := runTmux
	runTmux = func(args ...string) ([]byte, error) {
		calls = append(calls, args)
		out, err := reply(args)
		return []byte(out), err
	}
	t.Cleanup(func() { runTmux = saved })
	return &calls
}

func TestWindowSize(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,123,0")
	calls := stubTmux(t, func([]string) (string, error) { return "204 49\n", nil })

	width, height, err := WindowSize()
	if err != nil {
		t.Fatalf("WindowSize: %v", err)
	}
	if width != 204 || height != 49 {
		t.Errorf("WindowSize = %dx%d, want 204x49", width, height)
	}
	if len(*calls) != 1 || !slices.Contains((*calls)[0], "display-message") {
		t.Errorf("WindowSize ran %v, want a display-message call", *calls)
	}
}

func TestWindowSizeBadOutput(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,123,0")

	for _, reply := range []string{"", "204", "204 49 7", "wide 49", "204 tall"} {
		t.Run(fmt.Sprintf("%q", reply), func(t *testing.T) {
			stubTmux(t, func([]string) (string, error) { return reply, nil })
			if w, h, err := WindowSize(); err == nil {
				t.Errorf("WindowSize on %q = %dx%d, nil; want an error", reply, w, h)
			}
		})
	}
}

// TestWindowSizeOutsideTmux covers the common mistake of running this from a
// plain shell. A tmux server may well be running, and would answer for
// whichever session it saw last, so the check has to happen before tmux is
// asked anything at all.
func TestWindowSizeOutsideTmux(t *testing.T) {
	t.Setenv("TMUX", "")
	calls := stubTmux(t, func([]string) (string, error) { return "204 49\n", nil })

	_, _, err := WindowSize()
	if !errors.Is(err, ErrNotInTmux) {
		t.Fatalf("WindowSize outside tmux = %v, want ErrNotInTmux", err)
	}
	if len(*calls) != 0 {
		t.Errorf("WindowSize outside tmux ran %v, want it to ask tmux nothing", *calls)
	}
}

// TestSelectLayoutOutsideTmux is the one that matters: rearranging panes from
// outside tmux would move a window nobody is looking at.
func TestSelectLayoutOutsideTmux(t *testing.T) {
	t.Setenv("TMUX", "")
	calls := stubTmux(t, func([]string) (string, error) { return "", nil })

	if err := SelectLayout("ac4b,80x24,0,0{40x24,0,0,100,39x24,41,0,100}"); !errors.Is(err, ErrNotInTmux) {
		t.Fatalf("SelectLayout outside tmux = %v, want ErrNotInTmux", err)
	}
	if len(*calls) != 0 {
		t.Errorf("SelectLayout outside tmux ran %v, want it to ask tmux nothing", *calls)
	}
}

// TestWindowSizeInsideTmuxReportsTmuxError covers tmux failing for some reason
// other than not being there, which must not be reported as ErrNotInTmux.
func TestWindowSizeInsideTmuxReportsTmuxError(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,123,0")
	stubTmux(t, func([]string) (string, error) { return "", errors.New("can't find window") })

	_, _, err := WindowSize()
	if err == nil {
		t.Fatal("WindowSize = nil, want an error")
	}
	if errors.Is(err, ErrNotInTmux) {
		t.Errorf("WindowSize = %v, want the tmux error rather than ErrNotInTmux", err)
	}
}

func TestApply(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,123,0")
	calls := stubTmux(t, func(args []string) (string, error) {
		if args[0] == "display-message" {
			return "80 24\n", nil
		}
		return "", nil
	})

	layout, err := Apply("xY")
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	want := "ac4b,80x24,0,0{40x24,0,0,100,39x24,41,0,100}"
	if layout != want {
		t.Errorf("Apply returned %q, want %q", layout, want)
	}
	if len(*calls) != 2 {
		t.Fatalf("Apply ran %v, want a display-message and a select-layout", *calls)
	}
	if got := (*calls)[1]; len(got) != 2 || got[0] != "select-layout" || got[1] != want {
		t.Errorf("Apply ran %v, want [select-layout %s]", got, want)
	}
}

// TestApplyBadLayoutRunsNoCommand checks that a description that cannot be
// rendered never reaches tmux, so a typo leaves the window as it was.
func TestApplyBadLayoutRunsNoCommand(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-501/default,123,0")
	calls := stubTmux(t, func(args []string) (string, error) {
		if args[0] == "display-message" {
			return "80 24\n", nil
		}
		return "", nil
	})

	if _, err := Apply("1122", "1134", "5554"); !errors.Is(err, ErrUnsplittable) {
		t.Fatalf("Apply on an unsplittable layout = %v, want ErrUnsplittable", err)
	}
	for _, call := range *calls {
		if call[0] == "select-layout" {
			t.Errorf("Apply ran %v after failing to render", call)
		}
	}
}
