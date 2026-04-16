# Template Propagation

Expands on: **Generated Artifact Propagation** in `development-guidelines.md`

## The Three-Layer Problem

governa maintains three copies of the same logic:

1. **Source of truth** — `internal/` (Go packages) and root files (`build.sh`)
2. **Overlay templates** — `internal/templates/overlays/code/files/` and `internal/templates/overlays/doc/files/` (`.tmpl` copies with placeholders)
3. **Rendered examples** — `examples/code/` and `examples/doc/` (concrete output from templates)

These are not imports or symlinks. They are standalone copies. A bug in layer 1 exists independently in layers 2 and 3.

## Why This Matters

A fix applied only to `internal/buildtool/buildtool.go` leaves the same bug live in:
- `internal/templates/overlays/code/files/cmd/build/main.go.tmpl`
- `examples/code/cmd/build/main.go`

Any repo bootstrapped from the unfixed template inherits the bug. QA caught two propagation misses in the first fix pass (v0.1.0).

## The Rule

For every fix:

1. Fix the source of truth first (`internal/`, root files)
2. Grep the full repo for the pattern being changed
3. Apply the same fix to overlay templates (`internal/templates/overlays/`)
4. Regenerate rendered examples (`examples/`)

If the grep turns up no other matches, the fix is contained. If it does, propagate before marking complete.

## Common Propagation Paths

| Source | Template Copy | Example Copy |
|--------|--------------|--------------|
| `internal/buildtool/buildtool.go`, `internal/buildtool/buildtool_test.go` | `internal/templates/overlays/code/files/internal/buildtool/buildtool.go.tmpl`, `internal/templates/overlays/code/files/internal/buildtool/buildtool_test.go.tmpl` | `examples/code/internal/buildtool/buildtool.go`, `examples/code/internal/buildtool/buildtool_test.go` |
| `internal/reltool/reltool.go`, `internal/reltool/reltool_test.go` | `internal/templates/overlays/code/files/internal/reltool/reltool.go.tmpl`, `internal/templates/overlays/code/files/internal/reltool/reltool_test.go.tmpl`, `internal/templates/overlays/doc/files/internal/reltool/reltool.go.tmpl`, `internal/templates/overlays/doc/files/internal/reltool/reltool_test.go.tmpl` | `examples/code/internal/reltool/reltool.go`, `examples/code/internal/reltool/reltool_test.go`, `examples/doc/internal/reltool/reltool.go`, `examples/doc/internal/reltool/reltool_test.go` |
| `cmd/build/main.go` (delegator entrypoint) | `internal/templates/overlays/code/files/cmd/build/main.go.tmpl` | `examples/code/cmd/build/main.go` |
| `cmd/rel/main.go` (delegator entrypoint) | `internal/templates/overlays/code/files/cmd/rel/main.go.tmpl`, `internal/templates/overlays/doc/files/cmd/rel/main.go.tmpl` | `examples/code/cmd/rel/main.go`, `examples/doc/cmd/rel/main.go` |
| `internal/color/` | `internal/templates/overlays/code/files/internal/color/color.go.tmpl`, `internal/templates/overlays/doc/files/internal/color/color.go.tmpl` | `examples/code/internal/color/color.go`, `examples/doc/internal/color/color.go` |
| `build.sh` | `internal/templates/overlays/code/files/build.sh.tmpl` | `examples/code/build.sh` |

When propagating a `buildtool`/`reltool`/`color` source change, the only edit between source and template copy is the import-path rewrite `github.com/queone/governa/internal/<pkg>` → `{{MODULE_PATH}}/internal/<pkg>`. The example copy resolves `{{MODULE_PATH}}` to `github.com/queone/governa/examples/code` (or `…/examples/doc`).
