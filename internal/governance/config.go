package governance

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// configFileName is the repo-relative path of the optional consumer config.
const configFileName = ".governa/config"

// Critique modes recognized by ConfigCritiqueMode.
const (
	critiqueModeIntegrated = "integrated"
	critiqueModeExternal   = "external"
)

// ConfigCritiqueMode reads .governa/config in targetDir and returns the value of
// the `critique-mode` key. Returns "integrated" (the default) when:
//   - the file does not exist,
//   - the file is unreadable,
//   - the key is missing,
//   - the value is not one of "integrated" or "external" (with a stderr warning).
//
// Unknown keys are ignored with a stderr warning. No error/exit branches —
// config mistakes never block a sync.
func ConfigCritiqueMode(targetDir string) string {
	values, _ := readGovernaConfig(targetDir)
	raw, ok := values["critique-mode"]
	if !ok {
		return critiqueModeIntegrated
	}
	switch raw {
	case critiqueModeIntegrated, critiqueModeExternal:
		return raw
	default:
		fmt.Fprintf(os.Stderr, "governa: warning: .governa/config critique-mode=%q is not recognized (expected %q or %q); using default %q\n",
			raw, critiqueModeIntegrated, critiqueModeExternal, critiqueModeIntegrated)
		return critiqueModeIntegrated
	}
}

// configFilePresent reports whether .governa/config exists in targetDir. Used
// by the stdout advisory to stay silent when the consumer has not opted in.
func configFilePresent(targetDir string) bool {
	_, err := os.Stat(filepath.Join(targetDir, configFileName))
	return err == nil
}

// readGovernaConfig parses .governa/config into a key-value map. The format is
// one `key: value` per line; `#` starts a comment; blank lines are ignored.
// Whitespace around keys and values is trimmed. Unknown keys warn to stderr
// (so future-key consumers don't silently swallow typos).
func readGovernaConfig(targetDir string) (map[string]string, bool) {
	path := filepath.Join(targetDir, configFileName)
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()
	values := make(map[string]string)
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			fmt.Fprintf(os.Stderr, "governa: warning: .governa/config line %d missing ':' separator; skipping\n", lineNum)
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if !isKnownConfigKey(key) {
			fmt.Fprintf(os.Stderr, "governa: warning: .governa/config line %d: unknown key %q; ignoring\n", lineNum, key)
			continue
		}
		values[key] = val
	}
	return values, true
}

func isKnownConfigKey(key string) bool {
	switch key {
	case "critique-mode":
		return true
	}
	return false
}
