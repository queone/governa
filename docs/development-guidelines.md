# Development Guidelines

Engineering guidance for any agent or contributor working in this repo.
These are durable coding practices, not workflow or process rules.
For workflow, see `development-cycle.md`. For validation, see `build-release.md`.

## Identifier Strategy

- Template placeholders use `{{UPPERCASE_NAME}}` — not a templating engine, just literal string substitution
- Placeholder names are the canonical identifiers; do not introduce aliases or alternate forms

## Schema And Data Migrations

- When template structure changes, propagate to both source and overlay in the same pass
- When renaming a module or import path, audit all Go source and overlay templates in a single pass

## External Integration Patterns

- Generated repos must be fully self-contained with no runtime dependence on governa
- When apply reads a target repo, it treats the target as read-only input; all output goes to the target's own tree

## Generated Artifact Propagation

- Source-of-truth code lives in `internal/`; overlay templates under `internal/templates/overlays/` carry standalone copies of the same logic
- Fixes to `internal/buildtool`, `internal/reltool`, or `build.sh` must propagate to the overlay template copies
- Overlay copies of `roles.md` and `critique-protocol.md` are consumer-facing. When root governance docs evolve, propagate to overlay copies with targeted edits that filter governa-specific content. The DOC overlay `roles.md` references `docs/release.md` where the CODE overlay references `docs/build-release.md`.
- Grep the full repo for the pattern being changed before considering a fix complete
- If a template and its rendered output diverge, the template is authoritative
- Exported functions in template-owned packages (`internal/buildtool`, `internal/reltool`, `internal/color`) carry godoc single-line comments. Consumers that wholesale-adopt these packages inherit a correctly-commented surface.
- When propagating a `buildtool`/`reltool`/`color` source change, the only edit between source and template copy is the import-path rewrite `github.com/queone/governa/internal/<pkg>` → `{{MODULE_PATH}}/internal/<pkg>`

### Common Propagation Paths

| Source | Template Copy |
|--------|--------------|
| `internal/buildtool/buildtool.go`, `*_test.go` | `internal/templates/overlays/code/files/internal/buildtool/buildtool.go.tmpl`, `*_test.go.tmpl` |
| `internal/reltool/reltool.go`, `*_test.go` | `internal/templates/overlays/code/files/internal/reltool/reltool.go.tmpl`, `*_test.go.tmpl`; same under `overlays/doc/` |
| `internal/color/color.go` | `internal/templates/overlays/code/files/internal/color/color.go.tmpl`; same under `overlays/doc/` |
| `cmd/build/main.go` | `internal/templates/overlays/code/files/cmd/build/main.go.tmpl` |
| `cmd/rel/main.go` | `internal/templates/overlays/code/files/cmd/rel/main.go.tmpl`; same under `overlays/doc/` |
| `build.sh` | `internal/templates/overlays/code/files/build.sh.tmpl` |

## Program Version Declaration

- Every installable `cmd/<name>/main.go` must declare a non-empty `const programVersion` string literal
- Script-only helper entrypoints (`build`, `rel`) are exempt
- The build tool validates this before compiling installable binaries; missing or empty declarations fail the build

## Error Handling And Validation

- `go vet` and `staticcheck` errors are build failures, not warnings — use fail-hard checks
- Validate apply config at entry (mode, repo type, required fields) before rendering any files
- Prefer explicit error returns over silent fallbacks; a clear failure is better than wrong output

## Testing Expectations

- Tests are part of implementation, not a follow-up step
- Subprocess-dependent functions (exec.Command wrappers) have a documented coverage ceiling — do not mock them, document the gap
- End-to-end tests should call the exported entry point (e.g. `governance.RunWithFS`), not internal helpers
- Label tests that require live systems as `[Manual]`

## Dependency And Import Hygiene

- Pure stdlib only — no external Go dependencies
- After any module rename, grep all `.go` files and `.go.tmpl` files for stale import paths
- Shell wrappers (`build.sh`) are convenience only; canonical implementation lives in Go under `cmd/`

## CLI Usage Formatting

- All commands must accept `-h`, `-?`, and `--help` as help flags
- Help output uses `color.FormatUsage` from `internal/color` for consistent formatting
- "Usage:" is rendered in bold white (`color.BoldW`)
- Each flag line is indented 2 spaces; descriptions align at column 38
- Short and long flag forms are combined on one line (e.g. `-m, --mode string`)
- Footer text (constraints, notes) appears after a blank line following the flag list
- When adding new flags to any command, add the entry to the `FormatUsage` call — do not use `flag.PrintDefaults`

## Documentation Alignment

- Docs ship with the code change that introduces the behavior
- `arch.md` reflects what is built, not what is planned; `plan.md` is forward-looking only
- When a doc references a function, file path, or CLI flag, verify it still exists before committing
- Overlay READMEs list every template file; update them when adding or removing templates
- Numbered checklist steps with substantive content use `N. **Imperative title.** Details.` format. Short one-liner enumerations and rule statements stay lowercase-prose.

## Template Placeholder Guidance

The base template emits rendered files with several prose-content placeholders. The shape of the content the consumer supplies (or that governa infers) matters for how the governance contract reads in the generated repo. Source-code interpolation placeholders like `{{MODULE_PATH}}` (substituted verbatim into Go file templates) have obvious substitution semantics and are not covered here — this section is about placeholders whose content is prose.

- `{{REPO_NAME}}` — the repo identifier; use the same form as the module path's final component (e.g., `skout`, not `Skout Baseball`).
- `{{STACK_OR_PLATFORM}}` — the primary language or runtime stack for CODE repos (e.g., `Go`, `Python`, `Node`). Inferred from manifest files when not supplied.
