// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

// Command tmuxlayout rearranges the panes of the current tmux window to match
// a picture of the layout you want.
//
//	tmuxlayout 11123 11124
//
// Each argument is a row, each character a pane, and cells holding the same
// character are the same pane. See the package documentation of
// github.com/jmaslak/tmuxlayout/tmuxlayout for the format in full.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/jmaslak/tmuxlayout/tmuxlayout"
)

// version is the release this was built from. Release builds overwrite it with
//
//	-ldflags "-X main.version=$(git describe --tags)"
var version = "dev"

func main() {
	log.SetFlags(0)
	log.SetPrefix("tmuxlayout: ")

	showVersion := flag.Bool("version", false, "print the version and exit")
	update := flag.Bool("selfupdate", false, "update to the latest release and exit")
	printOnly := flag.Bool("print", false, "print the layout string instead of applying it")
	size := flag.String("size", "",
		"render for a canvas of this size, as WIDTHxHEIGHT, instead of asking tmux")
	flag.Usage = usage
	flag.Parse()

	if *showVersion {
		fmt.Printf("tmuxlayout %s\n", version)
		return
	}
	if *update {
		if err := selfUpdate(); err != nil {
			log.Fatal(err)
		}
		return
	}

	notifyUpdateAvailable(os.Stderr)

	def, err := description(flag.Args(), os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	if err := run(os.Stdout, def, *size, *printOnly); err != nil {
		log.Fatal(err)
	}
}

// run renders def and either prints the layout string or makes it the layout
// of the current tmux window.
func run(out io.Writer, def []string, size string, printOnly bool) error {
	l := tmuxlayout.New()

	// An explicit size is the whole point when tmux is not around to ask,
	// so giving one also means not asking.
	var err error
	if size != "" {
		l.Width, l.Height, err = parseSize(size)
	} else {
		l.Width, l.Height, err = windowSize()
	}
	if err != nil {
		return err
	}

	layout, err := l.Render(def...)
	if err != nil {
		return err
	}

	if printOnly {
		_, err := fmt.Fprintln(out, layout)
		return err
	}
	return selectLayout(layout)
}

// windowSize and selectLayout are the two things this command does to tmux.
// They are variables so tests can check what a run would have done without
// needing a tmux to do it to.
var (
	windowSize   = tmuxlayout.WindowSize
	selectLayout = tmuxlayout.SelectLayout
)

// description collects the layout to render: the arguments if there are any,
// and otherwise the lines read from in, so the layout can be piped in.
func description(args []string, in io.Reader) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}

	var def []string
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		def = append(def, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading the layout from standard input: %w", err)
	}
	if len(def) == 0 {
		return nil, fmt.Errorf("no layout given: %w", tmuxlayout.ErrEmptyLayout)
	}
	return def, nil
}

// parseSize reads a canvas size written as WIDTHxHEIGHT.
func parseSize(s string) (width, height int, err error) {
	w, h, ok := strings.Cut(s, "x")
	if !ok {
		return 0, 0, fmt.Errorf("cannot read %q as a size: it should be written WIDTHxHEIGHT, as in 80x24", s)
	}
	width, err = strconv.Atoi(w)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot read %q as the width of %q", w, s)
	}
	height, err = strconv.Atoi(h)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot read %q as the height of %q", h, s)
	}
	return width, height, nil
}

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprint(out, `Usage: tmuxlayout [options] [row...]

Rearrange the panes of the current tmux window to match a picture of the
layout. Each argument is a row, each character is a pane, and cells holding
the same character are the same pane:

    tmuxlayout 11123 11124

gives four panes, the first taking three fifths of the width and the full
height, and the last two stacked in the rightmost fifth:

    +------+--+--+
    |      |  |  |
    |      |  |  |
    |      |  +--+
    |      |  |  |
    |      |  |  |
    +------+--+--+

Rows can also be separated by pipes or newlines within one argument, so
'tmuxlayout 11123|11124' does the same thing. With no arguments, the layout
is read from standard input, one row per line.

Not every picture is a layout tmux can build: it makes a window by splitting
it in two and splitting each half again, so every region has to come apart
along a line that runs its full width or height. '1122|1134|5554' does not.

The window needs as many panes as the picture does before the layout will
apply; tmuxlayout rearranges panes, it does not create them.

Options:
`)
	flag.PrintDefaults()
}
