// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

package tmuxlayout

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// ErrNotInTmux is returned when a tmux command cannot be run because this
// process is not inside a tmux window. Test for it with [errors.Is].
var ErrNotInTmux = errors.New("not running inside tmux")

// runTmux is how this package shells out to tmux. It is a variable so tests
// can substitute a stub and not need a running tmux server.
var runTmux = func(args ...string) ([]byte, error) {
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			return nil, fmt.Errorf("tmux %s: %s", strings.Join(args, " "),
				strings.TrimSpace(string(exit.Stderr)))
		}
		return nil, fmt.Errorf("tmux %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// tmux runs a tmux command, first refusing to run one at all from outside
// tmux.
//
// Everything this package asks of tmux is about "the current window", and from
// outside tmux there is no such thing: tmux would answer for whichever session
// it saw most recently and quietly rearrange a window nobody is looking at.
// The TMUX environment variable is set in every pane and nowhere else, so it
// is what says whether there is a current window to talk about.
func tmux(args ...string) ([]byte, error) {
	if os.Getenv("TMUX") == "" {
		return nil, fmt.Errorf("%w: it acts on the window it is run from, so it has to be run inside one", ErrNotInTmux)
	}
	return runTmux(args...)
}

// WindowSize returns the size of the tmux canvas of the current window: its
// width in columns and its height in rows, the height excluding the status
// line.
func WindowSize() (width, height int, err error) {
	out, err := tmux("display-message", "-p", "#{window_width} #{window_height}")
	if err != nil {
		return 0, 0, err
	}

	fields := strings.Fields(string(out))
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("cannot parse the window size from tmux output %q", out)
	}
	width, err = strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("cannot parse the window width from tmux output %q", out)
	}
	height, err = strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, fmt.Errorf("cannot parse the window height from tmux output %q", out)
	}
	return width, height, nil
}

// SelectLayout makes an already rendered layout string the layout of the
// current tmux window. tmux applies a layout to the panes the window already
// has, so it wants exactly as many panes as the layout describes.
func SelectLayout(layout string) error {
	_, err := tmux("select-layout", layout)
	return err
}

// Apply renders a pane picture for the size of the current tmux window and
// makes it that window's layout, returning the layout string it applied. It
// can only be called from inside tmux; see [Layout.Render] for the format of
// the description.
func Apply(def ...string) (string, error) {
	width, height, err := WindowSize()
	if err != nil {
		return "", err
	}

	l := &Layout{Width: width, Height: height}
	layout, err := l.Render(def...)
	if err != nil {
		return "", err
	}

	if err := SelectLayout(layout); err != nil {
		return "", err
	}
	return layout, nil
}
