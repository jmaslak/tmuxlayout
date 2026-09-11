# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project uses
[semantic versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Added

- Initial Go release, a port of [Term::Tmux::Layout][perl], which it replaces.
  The port was made utilizing a LLM for machine translation between languages.
- `-print` renders a layout string without applying it, and `-size WxH` renders
  for a stated canvas instead of asking tmux. Together they work outside tmux.
- `-selfupdate` fetches and installs the latest release in place, verifying it
  against the release's published checksums, and a daily cached check mentions
  when a newer release exists.

### Fixed

- Pane sizes are no longer off by one. The Perl version added a character to
  the start of every pane not against the edge of the canvas, for the border
  before it, but took that character off the width of the *first* pane rather
  than each one, so its last pane ran one column past the edge. tmux accepted
  the result, silently repaired the offset, and left the panes a column away
  from where it would have put them itself. An even split now renders to the
  same string tmux writes, byte for byte.

### Changed

- A division of three or more panes is rendered flat rather than as nested
  pairs, matching the shape tmux gives a window it built itself. A later resize
  spreads across all of them instead of treating some as a unit.
- A canvas too small for the layout is reported as an error, rather than
  rendering panes of zero or negative width for tmux to reject or repair.
- Empty rows are ignored, so a trailing newline or pipe is no longer an error.
- Running from outside tmux is refused up front. Previously the tmux server
  would answer for whichever session it had seen last, and the layout would be
  applied to a window nobody was looking at.

[perl]: https://github.com/jmaslak/Term-Tmux-Layout
