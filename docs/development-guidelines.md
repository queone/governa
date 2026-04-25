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
- Exported functions in template-owned packages (`internal/buildtool`, `internal/reltool`, `internal/color`) carry godoc single-line comments. Consumers that wholesale-adopt these packages inherit a correctly-commented surface.

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

- `{{PROJECT_PURPOSE}}` — renders into `## Purpose` in `AGENTS.md` and into the overlay `README.md` opener. Expect a one- or two-sentence repo-identity description: what the repo is and what it delivers. Not a one-line slug ("internal tool"), not meta-guidance about the governance file itself (template-usage principles already live in `## Governed Sections`). A good shape names the domain, the surface, and the primary deliverable — e.g., skout's "skout is a read-only decision-support CLI for Yahoo Fantasy Baseball. It syncs league/roster/opponent data, enriches players with MLB StatsAPI, Baseball Savant, FanGraphs, and FantasyPros stats, and surfaces matchup-aware guidance…". Consumers don't need to match that structure exactly, but short-slug Purposes tend to drift back into meta-guidance or get replaced ad hoc. `governa` infers Purpose from the first content paragraph of the consumer's `README.md` (capped at 200 characters) when no explicit `--purpose` flag is supplied.
- `{{REPO_NAME}}` — the repo identifier; use the same form as the module path's final component (e.g., `skout`, not `Skout Baseball`).
- `{{STACK_OR_PLATFORM}}` — the primary language or runtime stack for CODE repos (e.g., `Go`, `Python`, `Node`). Inferred from manifest files when not supplied.
- `{{PUBLISHING_PLATFORM}}` — for DOC repos only; the platform the docs publish to (e.g., `Hugo`, `MkDocs`, `docs.example.com`).
- `{{DOC_STYLE}}` — for DOC repos only; the editorial voice or style guide reference (e.g., `Microsoft Writing Style Guide`, `house voice`).
