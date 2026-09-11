// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// releasesURL points at the GitHub API's "latest release" endpoint for this
// project. Overridden by tests so they never hit the network.
var releasesURL = "https://api.github.com/repos/jmaslak/tmuxlayout/releases/latest"

// httpClient is shared so tests can point it at a local server via a custom
// Transport instead of touching the network.
var httpClient = &http.Client{Timeout: 30 * time.Second}

type release struct {
	TagName string  `json:"tag_name"`
	Assets  []asset `json:"assets"`
}

type asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func (r release) assetURL(name string) (string, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.BrowserDownloadURL, true
		}
	}
	return "", false
}

// selfUpdate replaces the running executable with the latest release build for
// the current OS and architecture, verifying it against the release's
// published checksums first.
func selfUpdate() error {
	rel, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("checking for updates: %w", err)
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	if version != "dev" && latest == strings.TrimPrefix(version, "v") {
		writeUpdateCache(updateCache{CheckedAt: time.Now(), Latest: rel.TagName})
		fmt.Printf("tmuxlayout %s is already the latest version\n", version)
		return nil
	}

	assetName := fmt.Sprintf("tmuxlayout_%s_%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}
	binURL, ok := rel.assetURL(assetName)
	if !ok {
		return fmt.Errorf("release %s has no build for %s/%s", rel.TagName, runtime.GOOS, runtime.GOARCH)
	}
	checksumsURL, ok := rel.assetURL("checksums.txt")
	if !ok {
		return fmt.Errorf("release %s is missing checksums.txt", rel.TagName)
	}

	checksums, err := download(checksumsURL)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	wantSum, err := checksumFor(checksums, assetName)
	if err != nil {
		return err
	}

	bin, err := download(binURL)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", assetName, err)
	}
	if gotSum := sha256.Sum256(bin); hex.EncodeToString(gotSum[:]) != wantSum {
		return fmt.Errorf("checksum mismatch for %s: the download is corrupt or the release was tampered with", assetName)
	}

	if err := installExecutable(bin); err != nil {
		return err
	}

	writeUpdateCache(updateCache{CheckedAt: time.Now(), Latest: rel.TagName})

	fmt.Printf("tmuxlayout updated to %s\n", rel.TagName)
	return nil
}

func fetchLatestRelease() (release, error) {
	resp, err := httpClient.Get(releasesURL)
	if err != nil {
		return release{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return release{}, fmt.Errorf("unexpected status from GitHub: %s", resp.Status)
	}

	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return release{}, fmt.Errorf("decoding release metadata: %w", err)
	}
	return rel, nil
}

func download(url string) ([]byte, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s from %s", resp.Status, url)
	}
	return io.ReadAll(resp.Body)
}

// checksumFor finds name's checksum in a checksums.txt produced by
// `goreleaser`, formatted as one "<sha256>  <filename>" pair per line.
func checksumFor(checksums []byte, name string) (string, error) {
	for line := range strings.SplitSeq(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no checksum found for %s", name)
}

// updateCheckInterval is how long a cached "latest release" result is trusted
// before notifyUpdateAvailable refreshes it from GitHub.
const updateCheckInterval = 24 * time.Hour

// updateCache is the on-disk record of the last time we asked GitHub for the
// latest release, so most runs can report staleness without a network call.
type updateCache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest"`
}

// updateCachePath is a package variable so tests can point it at a temp file
// instead of the real user cache directory.
var updateCachePath = defaultUpdateCachePath

func defaultUpdateCachePath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tmuxlayout", "update-check.json"), nil
}

func readUpdateCache() (updateCache, bool) {
	path, err := updateCachePath()
	if err != nil {
		return updateCache{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return updateCache{}, false
	}
	var c updateCache
	if err := json.Unmarshal(data, &c); err != nil {
		return updateCache{}, false
	}
	return c, true
}

func writeUpdateCache(c updateCache) {
	path, err := updateCachePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

// refreshUpdateCache asks GitHub for the latest release and records it, so the
// next invocation can report staleness without hitting the network.
func refreshUpdateCache() {
	rel, err := fetchLatestRelease()
	if err != nil {
		// Best effort: a transient network failure just means the next
		// run tries again. Nothing worth interrupting someone who is
		// rearranging their panes.
		return
	}
	writeUpdateCache(updateCache{CheckedAt: time.Now(), Latest: rel.TagName})
}

// notifyUpdateAvailable prints a one line notice to w if a cached "latest
// release" is newer than the running version, then refreshes the cache in the
// background if it is missing or older than updateCheckInterval. It never
// blocks on the network.
func notifyUpdateAvailable(w io.Writer) {
	if version == "dev" {
		return
	}

	c, ok := readUpdateCache()
	if ok && c.Latest != "" {
		latest := strings.TrimPrefix(c.Latest, "v")
		if latest != strings.TrimPrefix(version, "v") {
			fmt.Fprintf(w, "tmuxlayout: update available: %s -> run 'tmuxlayout -selfupdate' to install\n", c.Latest)
		}
	}

	if !ok || time.Since(c.CheckedAt) > updateCheckInterval {
		scheduleRefresh()
	}
}

// scheduleRefresh kicks off refreshUpdateCache without blocking the caller. It
// is a package variable so tests can replace it and avoid touching the real
// network.
var scheduleRefresh = func() { go refreshUpdateCache() }

// installExecutable overwrites the running binary with bin. It is a package
// variable so tests can redirect it at a throwaway file instead of the test
// binary itself.
var installExecutable = replaceExecutable

// replaceExecutable atomically overwrites the running binary with bin. It
// writes to a temporary file in the same directory first, so the rename that
// replaces the live executable is a same-filesystem, single-syscall swap.
func replaceExecutable(bin []byte) error {
	target, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating the running executable: %w", err)
	}
	target, err = filepath.EvalSymlinks(target)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", target, err)
	}
	return installOver(target, bin)
}

// installOver atomically overwrites target with bin. Split out from
// replaceExecutable so tests can exercise it against a throwaway file instead
// of the test binary itself.
func installOver(target string, bin []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".tmuxlayout-update-*")
	if err != nil {
		return fmt.Errorf("creating temporary file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return fmt.Errorf("writing new binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("writing new binary: %w", err)
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return fmt.Errorf("making new binary executable: %w", err)
	}

	if err := os.Rename(tmp.Name(), target); err != nil {
		return fmt.Errorf("installing new binary over %s: %w", target, err)
	}
	return nil
}
