# Template Propagation

Expands on: **Generated Artifact Propagation** in `development-guidelines.md`

## The Two-Layer Problem

governa maintains two copies of the same logic:

1. **Source of truth.** `internal/` (Go packages) and root files (`build.sh`).
2. **Overlay templates.** `internal/templates/overlays/code/files/` and `internal/templates/overlays/doc/files/` (`.tmpl` copies with placeholders).

These are not imports or symlinks. They are standalone copies. A bug in layer 1 exists independently in layer 2.

Rendered examples are no longer committed — `governa examples` renders them on demand to `/tmp/governa-examples/` for inspection and build validation.

## Why This Matters

A fix applied only to `internal/buildtool/buildtool.go` leaves the same bug live in:
- `internal/templates/overlays/code/files/internal/buildtool/buildtool.go.tmpl`

Any repo bootstrapped from the unfixed template inherits the bug. QA caught two propagation misses in the first fix pass (v0.1.0).

## The Rule

For every fix:

1. **Fix the source of truth first.** (`internal/`, root files)
2. **Grep the full repo for the pattern being changed.**
3. **Apply the same fix to overlay templates.** (`internal/templates/overlays/`)

If the grep turns up no other matches, the fix is contained. If it does, propagate before marking complete. Run `./build.sh` to validate — it renders examples to a temp dir and runs `go vet`/`go test` against them.

## Common Propagation Paths

| Source | Template Copy |
|--------|--------------|
| `internal/buildtool/buildtool.go`, `internal/buildtool/buildtool_test.go` | `internal/templates/overlays/code/files/internal/buildtool/buildtool.go.tmpl`, `internal/templates/overlays/code/files/internal/buildtool/buildtool_test.go.tmpl` |
| `internal/reltool/reltool.go`, `internal/reltool/reltool_test.go` | `internal/templates/overlays/code/files/internal/reltool/reltool.go.tmpl`, `internal/templates/overlays/code/files/internal/reltool/reltool_test.go.tmpl`, `internal/templates/overlays/doc/files/internal/reltool/reltool.go.tmpl`, `internal/templates/overlays/doc/files/internal/reltool/reltool_test.go.tmpl` |
| `cmd/build/main.go` (delegator entrypoint) | `internal/templates/overlays/code/files/cmd/build/main.go.tmpl` |
| `cmd/rel/main.go` (delegator entrypoint) | `internal/templates/overlays/code/files/cmd/rel/main.go.tmpl`, `internal/templates/overlays/doc/files/cmd/rel/main.go.tmpl` |
| `internal/color/` | `internal/templates/overlays/code/files/internal/color/color.go.tmpl`, `internal/templates/overlays/doc/files/internal/color/color.go.tmpl` |
| `build.sh` | `internal/templates/overlays/code/files/build.sh.tmpl` |

When propagating a `buildtool`/`reltool`/`color` source change, the only edit between source and template copy is the import-path rewrite `github.com/queone/governa/internal/<pkg>` → `{{MODULE_PATH}}/internal/<pkg>`.
