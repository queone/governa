package templates

import (
	"embed"
	"io/fs"
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
