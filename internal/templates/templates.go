package templates

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed all:base all:overlays
var EmbeddedFS embed.FS

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
