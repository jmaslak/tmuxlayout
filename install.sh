#!/bin/sh
# Copyright (C) 2015-2026 Joelle Maslak
# SPDX-License-Identifier: Artistic-2.0
#
# Installs the latest tmuxlayout release for this machine's OS and
# architecture. Usage:
#
#   curl -fsSL https://raw.githubusercontent.com/jmaslak/tmuxlayout/main/install.sh | sh
set -eu

repo="jmaslak/tmuxlayout"

os=$(uname -s)
case "$os" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	FreeBSD) os=freebsd ;;
	*)
		echo "install.sh: unsupported OS: $os (only Linux, macOS, and FreeBSD builds are published)" >&2
		exit 1
		;;
esac

arch=$(uname -m)
case "$arch" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	riscv64) arch=riscv64 ;;
	*)
		echo "install.sh: unsupported architecture: $arch" >&2
		exit 1
		;;
esac

if { [ "$os" = "darwin" ] || [ "$os" = "freebsd" ]; } && [ "$arch" = "riscv64" ]; then
	echo "install.sh: no $os/riscv64 build is published" >&2
	exit 1
fi

api_url="https://api.github.com/repos/$repo/releases/latest"
asset="tmuxlayout_${os}_${arch}"

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

echo "install.sh: looking up the latest release..." >&2
release_json="$tmpdir/release.json"
curl -fsSL "$api_url" -o "$release_json"

# Extracted with sed rather than a JSON tool, since neither jq nor python3 is
# guaranteed to be present on a bare install target.
extract_url() {
	sed -n 's/.*"browser_download_url": *"\([^"]*'"$1"'\)".*/\1/p' "$release_json" | head -n1
}

bin_url=$(extract_url "$asset")
sums_url=$(extract_url "checksums.txt")

if [ -z "$bin_url" ] || [ -z "$sums_url" ]; then
	echo "install.sh: could not find a $asset release asset" >&2
	exit 1
fi

echo "install.sh: downloading $asset..." >&2
curl -fsSL "$bin_url" -o "$tmpdir/$asset"
curl -fsSL "$sums_url" -o "$tmpdir/checksums.txt"

echo "install.sh: verifying checksum..." >&2
want_sum=$(grep " $asset\$" "$tmpdir/checksums.txt" | awk '{print $1}')
if [ -z "$want_sum" ]; then
	echo "install.sh: no checksum found for $asset" >&2
	exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
	got_sum=$(sha256sum "$tmpdir/$asset" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
	got_sum=$(shasum -a 256 "$tmpdir/$asset" | awk '{print $1}')
else
	echo "install.sh: neither sha256sum nor shasum is available, cannot verify the download" >&2
	exit 1
fi

if [ "$got_sum" != "$want_sum" ]; then
	echo "install.sh: checksum mismatch for $asset (got $got_sum, want $want_sum) - refusing to install" >&2
	exit 1
fi

chmod +x "$tmpdir/$asset"

install_dir="/usr/local/bin"
if [ ! -w "$install_dir" ]; then
	install_dir="$HOME/.local/bin"
	mkdir -p "$install_dir"
fi

mv "$tmpdir/$asset" "$install_dir/tmuxlayout"

echo "install.sh: installed tmuxlayout to $install_dir/tmuxlayout" >&2
case ":$PATH:" in
	*":$install_dir:"*) ;;
	*) echo "install.sh: $install_dir is not on your PATH - add it to use tmuxlayout directly" >&2 ;;
esac
