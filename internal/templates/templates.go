package templates

import "embed"

//go:embed all:base all:overlays all:stack-ignores
var EmbeddedFS embed.FS
