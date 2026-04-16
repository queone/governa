package templates

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed all:base all:overlays all:stack-ignores CHANGELOG.md
var EmbeddedFS embed.FS

// Changelog returns the embedded governa CHANGELOG.md content. Kept in sync
// with the repo-root CHANGELOG.md during release prep. Used by `governa sync`
// to emit a "Template Changes" summary when syncing across template versions.
func Changelog() string {
	b, err := fs.ReadFile(EmbeddedFS, "CHANGELOG.md")
	if err != nil {
		return ""
	}
	return string(b)
}

// DiskFS returns a filesystem rooted at the templates directory
// within a local governa checkout. Used by enhance mode.
func DiskFS(repoRoot string) fs.FS {
	return os.DirFS(filepath.Join(repoRoot, "internal", "templates"))
}

// DirPath returns the absolute path to the templates directory
// within a local governa checkout.
func DirPath(repoRoot string) string {
	return filepath.Join(repoRoot, "internal", "templates")
}
