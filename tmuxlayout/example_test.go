// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

package tmuxlayout_test

import (
	"fmt"
	"log"

	"github.com/jmaslak/tmuxlayout/tmuxlayout"
)

// A picture of the panes becomes the layout string tmux's select-layout wants.
func ExampleLayout_Render() {
	l := tmuxlayout.New()

	layout, err := l.Render("11123", "11124")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(layout)
	// Output: 3e6d,80x24,0,0{48x24,0,0,100,15x24,49,0,100,15x24,65,0[15x12,65,0,100,15x11,65,13,100]}
}

// A layout for a canvas that is not tmux's 80x24 default.
func ExampleLayout_Render_size() {
	l := &tmuxlayout.Layout{Width: 204, Height: 49}

	layout, err := l.Render("ab|cc")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(layout)
	// Output: 37bd,204x49,0,0[204x24,0,0{102x24,0,0,100,101x24,103,0,100},204x24,0,25,100]
}

func ExampleChecksum() {
	fmt.Println(tmuxlayout.Checksum("364x94,0,0,9"))
	// Output: c846
}
