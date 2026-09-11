// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

package tmuxlayout

import (
	"fmt"
	"math/bits"
	"strings"
)

// Checksum returns the four hex digit checksum tmux prefixes a layout string
// with, for the layout string without that prefix. It is the algorithm of
// layout_checksum() in the tmux source: a 16 bit sum, rotated right by one bit
// before each byte is added.
//
// A trailing newline is ignored, so the output of a command that printed a
// layout can be passed straight in.
func Checksum(s string) string {
	s = strings.TrimSuffix(s, "\n")

	var sum uint16
	for i := range len(s) {
		sum = bits.RotateLeft16(sum, -1) + uint16(s[i])
	}
	return fmt.Sprintf("%04x", sum)
}
