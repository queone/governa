# Development Guidelines

Engineering guidance for any agent or contributor working in this repo.
These are durable coding practices, not workflow or process rules.
For workflow, see `development-cycle.md`. For validation, see `build-release.md`.

## Identifier Strategy

- Template placeholders use `{{UPPERCASE_NAME}}` — not a templating engine, just literal string substitution
- Placeholder names are the canonical identifiers; do not introduce aliases or alternate forms

## Schema And Data Migrations

- Overlay templates are versioned through `TEMPLATE_VERSION`; when template structure changes, bump the version
- When renaming a module or import path, audit all Go source, overlay templates, and rendered examples in a single pass

## External Integration Patterns

- Generated repos must be fully self-contained with no runtime dependence on governa
- When bootstrap reads a target repo, it treats the target as read-only input; all output goes to the target's own tree

## Generated Artifact Propagation

- Source-of-truth code lives in `internal/`; overlay templates under `internal/templates/overlays/` carry standalone copies of the same logic
- Fixes to `internal/buildtool`, `internal/reltool`, or `build.sh` must propagate to both overlay template copies and rendered examples
- Grep the full repo for the pattern being changed before considering a fix complete
- If a template and its rendered output diverge, the template is authoritative

## Program Version Declaration

- Every installable `cmd/<name>/main.go` must declare a non-empty `const programVersion` string literal
- Script-only helper entrypoints (`build`, `rel`) are exempt
- The build tool validates this before compiling installable binaries; missing or empty declarations fail the build

## Error Handling And Validation

- `go vet` and `staticcheck` errors are build failures, not warnings — use fail-hard checks
- Validate bootstrap config at entry (mode, repo type, required fields) before rendering any files
- Prefer explicit error returns over silent fallbacks; a clear failure is better than wrong output

## Testing Expectations

- Tests are part of implementation, not a follow-up step
- Subprocess-dependent functions (exec.Command wrappers) have a documented coverage ceiling — do not mock them, document the gap
- End-to-end tests should call the exported entry point (e.g. `RunEnhance`), not internal helpers
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
