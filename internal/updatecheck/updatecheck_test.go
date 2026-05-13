package updatecheck

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func resetUpdatecheckTestState(t *testing.T) string {
	t.Helper()
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	t.Setenv("GOVERNA_NO_UPDATE_CHECK", "")
	oldNow := now
	oldFetch := fetchLatest
	oldErrOut := errOut
	now = func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }
	errOut = &bytes.Buffer{}
	t.Cleanup(func() {
		now = oldNow
		fetchLatest = oldFetch
		errOut = oldErrOut
	})
	return cacheDir
}

func TestCheckOptOutSkipsFetchAndOutput(t *testing.T) {
	resetUpdatecheckTestState(t)
	t.Setenv("GOVERNA_NO_UPDATE_CHECK", "1")
	called := false
	fetchLatest = func() (string, error) {
		called = true
		return "v9.9.9", nil
	}
	Check("0.127.0")
	if called {
		t.Fatal("fetchLatest called despite opt-out")
	}
	if got := errOut.(*bytes.Buffer).String(); got != "" {
		t.Fatalf("expected no stderr output, got %q", got)
	}
}

func TestCheckFreshCacheNotifiesOnlyWhenNewer(t *testing.T) {
	cacheDir := resetUpdatecheckTestState(t)
	writeCache := func(latest string) {
		t.Helper()
		path := filepath.Join(cacheDir, "governa", "last-check")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(cacheEntry{CheckedAt: now(), LatestVersion: latest})
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeCache("v0.127.0")
	Check("0.127.0")
	if got := errOut.(*bytes.Buffer).String(); got != "" {
		t.Fatalf("unexpected current-version notice: %q", got)
	}

	errOut = &bytes.Buffer{}
	writeCache("v0.128.0")
	Check("0.127.0")
	if got := errOut.(*bytes.Buffer).String(); !strings.Contains(got, "governa v0.128.0 available") {
		t.Fatalf("expected newer-version notice, got %q", got)
	}
}

func TestCheckStaleCacheFetchesAndWrites(t *testing.T) {
	cacheDir := resetUpdatecheckTestState(t)
	fetchLatest = func() (string, error) { return "v0.128.0", nil }
	Check("0.127.0")
	if got := errOut.(*bytes.Buffer).String(); !strings.Contains(got, "v0.128.0") {
		t.Fatalf("expected fetched-version notice, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "governa", "last-check")); err != nil {
		t.Fatalf("expected cache write: %v", err)
	}
}

func TestVersionGreater(t *testing.T) {
	if !versionGreater("v1.0.0", "0.127.0") {
		t.Fatal("expected major newer version to compare greater")
	}
	if versionGreater("v0.127.0", "0.127.0") {
		t.Fatal("equal versions must not compare greater")
	}
}
