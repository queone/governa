// Package updatecheck implements Governa's best-effort install freshness notice.
package updatecheck

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const cacheTTL = 24 * time.Hour

type cacheEntry struct {
	CheckedAt     time.Time `json:"checked_at"`
	LatestVersion string    `json:"latest_version"`
}

var (
	now                   = time.Now
	cacheHome             = os.UserHomeDir
	fetchLatest           = defaultFetchLatest
	errOut      io.Writer = os.Stderr
)

// Check reports a newer governa release on stderr when cached or fetched data says one exists.
func Check(currentVersion string) {
	if os.Getenv("GOVERNA_NO_UPDATE_CHECK") != "" {
		return
	}

	cachePath, err := lastCheckPath()
	if err != nil {
		return
	}

	entry, fresh := readFreshCache(cachePath)
	if !fresh {
		latest, err := fetchLatest()
		if err != nil {
			return
		}
		entry = cacheEntry{CheckedAt: now(), LatestVersion: latest}
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			return
		}
		data, err := json.Marshal(entry)
		if err != nil {
			return
		}
		if err := os.WriteFile(cachePath, data, 0o644); err != nil {
			return
		}
	}

	if versionGreater(entry.LatestVersion, currentVersion) {
		fmt.Fprintf(errOut, "governa %s available (you are on %s); install: go install github.com/queone/governa/cmd/governa@latest\n", normalizeVersion(entry.LatestVersion), normalizeVersion(currentVersion))
	}
}

func defaultFetchLatest() (string, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/queone/governa/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("github releases status %s", resp.Status)
	}
	var body struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if strings.TrimSpace(body.TagName) == "" {
		return "", fmt.Errorf("latest release tag missing")
	}
	return body.TagName, nil
}

func lastCheckPath() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CACHE_HOME")); xdg != "" {
		return filepath.Join(xdg, "governa", "last-check"), nil
	}
	home, err := cacheHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "governa", "last-check"), nil
}

func readFreshCache(path string) (cacheEntry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cacheEntry{}, false
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return cacheEntry{}, false
	}
	if entry.CheckedAt.IsZero() || strings.TrimSpace(entry.LatestVersion) == "" {
		return cacheEntry{}, false
	}
	return entry, now().Sub(entry.CheckedAt) <= cacheTTL
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "v0.0.0"
	}
	if strings.HasPrefix(v, "v") {
		return v
	}
	return "v" + v
}

func versionGreater(latest, current string) bool {
	l := versionParts(latest)
	c := versionParts(current)
	for i := range len(l) {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

func versionParts(v string) [3]int {
	v = strings.TrimPrefix(normalizeVersion(v), "v")
	base := strings.SplitN(v, "-", 2)[0]
	parts := strings.Split(base, ".")
	var out [3]int
	for i := 0; i < len(out) && i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err == nil {
			out[i] = n
		}
	}
	return out
}
