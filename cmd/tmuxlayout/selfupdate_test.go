// Copyright (C) 2015-2026 Joelle Maslak
// SPDX-License-Identifier: Artistic-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestChecksumFor(t *testing.T) {
	checksums := []byte("aaa  tmuxlayout_linux_amd64\nbbb  tmuxlayout_darwin_arm64\n")

	got, err := checksumFor(checksums, "tmuxlayout_darwin_arm64")
	if err != nil {
		t.Fatalf("checksumFor: %v", err)
	}
	if got != "bbb" {
		t.Errorf("checksumFor = %q, want %q", got, "bbb")
	}

	if _, err := checksumFor(checksums, "tmuxlayout_windows_amd64"); err == nil {
		t.Error("checksumFor: expected an error for a missing entry, got nil")
	}
}

func TestInstallOver(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "tmuxlayout")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatalf("seeding target: %v", err)
	}

	if err := installOver(target, []byte("new")); err != nil {
		t.Fatalf("installOver: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("reading target: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("installOver: target contains %q, want %q", got, "new")
	}

	if runtime.GOOS == "windows" {
		// Windows has no POSIX executable bit: os.Chmod only toggles the
		// read-only attribute, so the mode is not worth checking there.
		return
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat target: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("installOver: target mode is %v, want it executable", info.Mode().Perm())
	}
}

// releaseServer serves a GitHub-shaped "latest release" for a single binary,
// with checksums, and returns the URL of the release endpoint.
func releaseServer(t *testing.T, tag string, bin []byte, corruptChecksum bool) string {
	t.Helper()

	assetName := fmt.Sprintf("tmuxlayout_%s_%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}

	sum := sha256.Sum256(bin)
	checksum := hex.EncodeToString(sum[:])
	if corruptChecksum {
		checksum = strings.Repeat("0", len(checksum))
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/bin", func(w http.ResponseWriter, r *http.Request) { w.Write(bin) })
	mux.HandleFunc("/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s  %s\n", checksum, assetName)
	})
	mux.HandleFunc("/release", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(release{
			TagName: tag,
			Assets: []asset{
				{Name: assetName, BrowserDownloadURL: srv.URL + "/bin"},
				{Name: "checksums.txt", BrowserDownloadURL: srv.URL + "/checksums.txt"},
			},
		})
	})

	return srv.URL + "/release"
}

// useTempCache points the update cache at a throwaway file so tests never
// read or write the real one.
func useTempCache(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "update-check.json")
	saved := updateCachePath
	updateCachePath = func() (string, error) { return path, nil }
	t.Cleanup(func() { updateCachePath = saved })
	return path
}

func TestSelfUpdate(t *testing.T) {
	useTempCache(t)

	savedVersion, savedURL, savedInstall := version, releasesURL, installExecutable
	t.Cleanup(func() { version, releasesURL, installExecutable = savedVersion, savedURL, savedInstall })

	version = "v1.0.0"
	releasesURL = releaseServer(t, "v1.1.0", []byte("the new binary"), false)

	var installed []byte
	installExecutable = func(bin []byte) error {
		installed = bin
		return nil
	}

	if err := selfUpdate(); err != nil {
		t.Fatalf("selfUpdate: %v", err)
	}
	if string(installed) != "the new binary" {
		t.Errorf("selfUpdate installed %q, want %q", installed, "the new binary")
	}

	c, ok := readUpdateCache()
	if !ok || c.Latest != "v1.1.0" {
		t.Errorf("selfUpdate left cache %+v (found=%v), want Latest v1.1.0", c, ok)
	}
}

// TestSelfUpdateRejectsBadChecksum is the one that matters: a binary that does
// not match the published checksum must never be installed.
func TestSelfUpdateRejectsBadChecksum(t *testing.T) {
	useTempCache(t)

	savedVersion, savedURL, savedInstall := version, releasesURL, installExecutable
	t.Cleanup(func() { version, releasesURL, installExecutable = savedVersion, savedURL, savedInstall })

	version = "v1.0.0"
	releasesURL = releaseServer(t, "v1.1.0", []byte("the new binary"), true)
	installExecutable = func([]byte) error {
		t.Error("selfUpdate installed a binary whose checksum did not match")
		return nil
	}

	err := selfUpdate()
	if err == nil {
		t.Fatal("selfUpdate with a bad checksum = nil, want an error")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("selfUpdate with a bad checksum = %q, want it to name the mismatch", err)
	}
}

func TestSelfUpdateAlreadyCurrent(t *testing.T) {
	useTempCache(t)

	savedVersion, savedURL, savedInstall := version, releasesURL, installExecutable
	t.Cleanup(func() { version, releasesURL, installExecutable = savedVersion, savedURL, savedInstall })

	version = "v1.1.0"
	releasesURL = releaseServer(t, "v1.1.0", []byte("the new binary"), false)
	installExecutable = func([]byte) error {
		t.Error("selfUpdate reinstalled the version already running")
		return nil
	}

	if err := selfUpdate(); err != nil {
		t.Fatalf("selfUpdate: %v", err)
	}
}

func TestNotifyUpdateAvailable(t *testing.T) {
	savedVersion, savedRefresh := version, scheduleRefresh
	t.Cleanup(func() { version, scheduleRefresh = savedVersion, savedRefresh })

	tests := []struct {
		name        string
		version     string
		cache       *updateCache
		wantNotice  bool
		wantRefresh bool
	}{
		{
			name: "newer release cached", version: "v1.0.0",
			cache:      &updateCache{CheckedAt: time.Now(), Latest: "v1.1.0"},
			wantNotice: true,
		},
		{
			name: "already current", version: "v1.1.0",
			cache: &updateCache{CheckedAt: time.Now(), Latest: "v1.1.0"},
		},
		{
			name: "no cache yet", version: "v1.0.0",
			wantRefresh: true,
		},
		{
			name: "stale cache is refreshed", version: "v1.0.0",
			cache:       &updateCache{CheckedAt: time.Now().Add(-48 * time.Hour), Latest: "v1.0.0"},
			wantRefresh: true,
		},
		{
			// A development build has no release to compare against.
			name: "development build says nothing", version: "dev",
			cache: &updateCache{CheckedAt: time.Now(), Latest: "v9.9.9"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := useTempCache(t)
			if tt.cache != nil {
				data, err := json.Marshal(tt.cache)
				if err != nil {
					t.Fatalf("marshalling cache: %v", err)
				}
				if err := os.WriteFile(path, data, 0o644); err != nil {
					t.Fatalf("seeding cache: %v", err)
				}
			}

			version = tt.version
			refreshed := false
			scheduleRefresh = func() { refreshed = true }

			var out bytes.Buffer
			notifyUpdateAvailable(&out)

			if gotNotice := strings.Contains(out.String(), "update available"); gotNotice != tt.wantNotice {
				t.Errorf("notifyUpdateAvailable wrote %q, wantNotice %v", out.String(), tt.wantNotice)
			}
			if refreshed != tt.wantRefresh {
				t.Errorf("notifyUpdateAvailable refreshed = %v, want %v", refreshed, tt.wantRefresh)
			}
		})
	}
}

// TestNotifyUpdateAvailableSurvivesUnreadableCache covers a cache file that is
// missing, empty, or corrupt: a startup notice is never worth failing over.
func TestNotifyUpdateAvailableSurvivesUnreadableCache(t *testing.T) {
	savedVersion, savedRefresh := version, scheduleRefresh
	t.Cleanup(func() { version, scheduleRefresh = savedVersion, savedRefresh })
	version = "v1.0.0"
	scheduleRefresh = func() {}

	for _, content := range []string{"", "not json", `{"checked_at":"nonsense"}`} {
		path := useTempCache(t)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("seeding cache: %v", err)
		}

		var out bytes.Buffer
		notifyUpdateAvailable(&out)
		if out.Len() != 0 {
			t.Errorf("notifyUpdateAvailable on cache %q wrote %q, want nothing", content, out.String())
		}
	}
}
