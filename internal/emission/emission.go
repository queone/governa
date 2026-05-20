// Package emission contains shared helpers for governa CLI commands that emit
// AC artifacts into consumer repositories.
package emission

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/queone/governa/internal/governance"
)

const (
	markerInfix  = "; emission-sha="
	markerSuffix = " -->"
)

// IsGovernaCheckout reports whether target points at a governa source checkout.
// Callers that need to permit governa-source operation (e.g., `governa deps`)
// branch on this directly rather than catching RefuseGovernaSource's error.
func IsGovernaCheckout(target string) bool {
	return governance.DetectGovernaCheckoutAt(target) == nil
}

// RefuseGovernaSource prevents consumer-run commands from targeting governa's source repo.
func RefuseGovernaSource(target, tool string) error {
	if IsGovernaCheckout(target) {
		return fmt.Errorf("%s: target %s looks like a governa checkout — %s is for adopted repos, not the governa source", tool, target, tool)
	}
	return nil
}

// RequireGovernaAdopted verifies target carries Governa adoption signals.
func RequireGovernaAdopted(target, tool string) error {
	if !fileExists(filepath.Join(target, "AGENTS.md")) {
		return fmt.Errorf("%s: %s is not a governa-adopted repo (AGENTS.md not found); run from the consumer repo root after `governa apply`", tool, target)
	}
	for _, sig := range []string{"docs/ac-template.md", "docs/release.md", "docs/build-release.md"} {
		if fileExists(filepath.Join(target, sig)) {
			return nil
		}
	}
	if changelog, err := os.ReadFile(filepath.Join(target, "CHANGELOG.md")); err == nil {
		if regexp.MustCompile(`(?i)governa\s+apply`).Match(changelog) {
			return nil
		}
	}
	return fmt.Errorf("%s: %s has AGENTS.md but no governa adoption signal (expected one of: docs/ac-template.md, docs/release.md, docs/build-release.md, or CHANGELOG row referencing 'governa apply'); ensure you are running from a governa-adopted repo root", tool, target)
}

// EnsureDocsDir creates the artifact directory used by emitted AC files.
func EnsureDocsDir(target, tool string) error {
	if err := os.MkdirAll(filepath.Join(target, "docs"), 0o755); err != nil {
		return fmt.Errorf("%s: ensure docs/ exists: %w", tool, err)
	}
	return nil
}

var preserveMarkerPatterns = []string{
	`preserve %s`,
	`do not sync %s`,
	`intentional divergence: %s`,
	`%s: keep local`,
}

// PreserveMarkers returns verbatim changelog/AC marker phrases (phrase-only;
// not the surrounding row) that preserve relpath. Each match captures from the
// phrase start to the first of `;`, `|`, `\r`, or end-of-line — whichever comes
// first — so a single row carrying multiple markers yields one clean citation
// per marker rather than the whole row.
func PreserveMarkers(targetRoot, relpath string) []string {
	var hits []string
	anchor := `(?:^|[|;])\s*(?:[-*]\s+|\*\*[^*]+\*\*\s+)?`
	var patterns []*regexp.Regexp
	for _, pattern := range preserveMarkerPatterns {
		phrase := fmt.Sprintf(pattern, relpath)
		// Capture group 1 isolates the phrase position so phrase-extent
		// extraction can start at the phrase, not at the anchor prefix.
		patterns = append(patterns, regexp.MustCompile(anchor+`(`+regexp.QuoteMeta(phrase)+`)`))
	}
	scan := func(content string) {
		for line := range strings.SplitSeq(content, "\n") {
			for _, pattern := range patterns {
				for _, idx := range pattern.FindAllStringSubmatchIndex(line, -1) {
					phraseStart := idx[2]
					end := len(line)
					for i := phraseStart; i < len(line); i++ {
						c := line[i]
						if c == ';' || c == '|' || c == '\r' {
							end = i
							break
						}
					}
					citation := strings.TrimSpace(line[phraseStart:end])
					if citation != "" {
						hits = append(hits, citation)
					}
				}
			}
		}
	}
	if changelog, err := os.ReadFile(filepath.Join(targetRoot, "CHANGELOG.md")); err == nil {
		scan(string(changelog))
	}
	if entries, err := os.ReadDir(filepath.Join(targetRoot, "docs")); err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasPrefix(name, "ac") || !strings.HasSuffix(name, ".md") {
				continue
			}
			content, err := os.ReadFile(filepath.Join(targetRoot, "docs", name))
			if err == nil {
				scan(string(content))
			}
		}
	}
	return uniq(hits)
}

// AllocateACNumber reuses a same-version stub number or allocates the next AC number.
func AllocateACNumber(target, slugStem, canonVersion string) (int, bool, error) {
	docsDir := filepath.Join(target, "docs")
	pattern := filepath.Join(docsDir, "ac*-"+slugStem+"-"+canonVersion+".md")
	matches, _ := filepath.Glob(pattern)
	var stubs []string
	for _, match := range matches {
		if !strings.HasSuffix(match, "-diffs.md") {
			stubs = append(stubs, match)
		}
	}

	stubRe := regexp.MustCompile(`^ac(\d+)-`)
	switch len(stubs) {
	case 1:
		base := filepath.Base(stubs[0])
		match := stubRe.FindStringSubmatch(base)
		if match == nil {
			return 0, false, fmt.Errorf("unexpected emitted AC filename: %s", base)
		}
		n, err := strconv.Atoi(match[1])
		if err != nil {
			return 0, false, fmt.Errorf("parse AC number from %s: %w", base, err)
		}
		return n, true, nil
	case 0:
	default:
		return 0, false, fmt.Errorf("multiple emitted AC stubs for %s %s: %v", slugStem, canonVersion, stubs)
	}

	maxN := 0
	if entries, err := os.ReadDir(docsDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			match := stubRe.FindStringSubmatch(entry.Name())
			if match == nil {
				continue
			}
			n, err := strconv.Atoi(match[1])
			if err == nil && n > maxN {
				maxN = n
			}
		}
	}

	cmd := exec.Command("git", "-C", target, "log", "--all", "--pretty=%B")
	out, runErr := cmd.Output()
	if runErr != nil {
		stderr := ""
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
		}
		switch {
		case strings.Contains(stderr, "does not have any commits"),
			strings.Contains(stderr, "bad default revision"),
			strings.Contains(stderr, "Not a valid object name"):
			out = nil
		default:
			return 0, false, fmt.Errorf("read git log for AC-number allocation in %s: %w (stderr: %s)", target, runErr, strings.TrimSpace(stderr))
		}
	}

	acRefRe := regexp.MustCompile(`\bAC(\d+)\b`)
	for _, match := range acRefRe.FindAllStringSubmatch(string(out), -1) {
		n, err := strconv.Atoi(match[1])
		if err == nil && n > maxN {
			maxN = n
		}
	}
	return maxN + 1, false, nil
}

// VerifyUnedited checks whether an emitted file body still matches its marker hash.
func VerifyUnedited(path, markerPrefix string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	idx := strings.IndexByte(string(data), '\n')
	if idx < 0 {
		return false, nil
	}
	stored := parseMarker(string(data[:idx]), markerPrefix)
	if stored == "" {
		return false, nil
	}
	return stored == bodySHA(string(data[idx+1:])), nil
}

// WriteWithMarker writes marker + body, preserving edit-detection metadata.
func WriteWithMarker(path, markerPrefix, canonVersion, body string) error {
	marker := markerPrefix + canonVersion + markerInfix + bodySHA(body) + markerSuffix
	return os.WriteFile(path, []byte(marker+"\n"+body), 0o644)
}

func parseMarker(line, markerPrefix string) string {
	if !strings.HasPrefix(line, markerPrefix) || !strings.HasSuffix(line, markerSuffix) {
		return ""
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(line, markerPrefix), markerSuffix)
	_, sha, ok := strings.Cut(inner, markerInfix)
	if !ok {
		return ""
	}
	return sha
}

func bodySHA(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func uniq(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	var out []string
	for _, value := range in {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
