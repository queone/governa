# Build and Release

## Build and Test Rules

- use one canonical local build command and keep this document current
- run formatting, static checks, tests, and packaging through that command or documented sequence
- do not trigger release work during routine implementation

This repo is Go-based and keeps the real implementation in:

- `cmd/build/main.go`
- `cmd/rel/main.go`

The root `build.sh` script is a convenience wrapper for Unix, Linux, and Git-Bash environments.

## Minimum Validation

- formatting passes
- static checks pass
- automated tests pass
- changed docs match actual behavior

## Canonical Build Commands

```bash
go run ./cmd/build
```

Convenience wrapper:

```bash
./build.sh
```

To scope the run to selected commands:

```bash
go run ./cmd/build build rel
```

or:

```bash
./build.sh build rel
```

If you pass `build` or `rel` as targets, the command will validate those entrypoints but will not install binaries for them.

## Sandboxed Execution

Under sandboxed execution that blocks Go's build cache (look for `writing stat cache ... operation not permitted`), `staticcheck` may print a `matched no packages` warning even though it ran cleanly. Treat as advisory unless real findings appear; an unrestricted rerun confirms.

## Pre-Release Checklist

Do not start this checklist unless the user explicitly asks to prep for release or equivalent.

The operator flow is two steps:

1. **Run `go run ./cmd/prep/ vX.Y.Z "message"`.** Stages version bumps, inserts the CHANGELOG row, deletes completed AC files (plus `-critique.md` companions), runs validation builds before and after, and prints the canonical release command. The agent determines the version (semver classification from the AC's scope) and drafts the release message (≤ 80 characters) before invoking prep.
2. **Run the printed release command (`./build.sh vX.Y.Z "message"`).** `cmd/rel` shows `git status --short`, lists every git step it will execute, and prompts for interactive confirmation. On approval it orchestrates `git add → commit → tag → push tag → push branch`. Optional: run `git diff` between the two steps if you want to inspect the CHANGELOG row wording and version-string values before committing — `cmd/rel`'s own status preview is sufficient to catch wrong-file inclusions or deletions.

Present only the release command after prep; do not add trailing commentary about wrapper routing or prompts. The director already knows.

### Appendix: what prep does

`go run ./cmd/prep/` runs nine phases internally so the operator flow above stays short. Each phase has a clear failure mode:

1. **Validate inputs.** Semver pattern (`vX.Y.Z`), message non-empty and ≤ 80 characters.
2. **Validate git state.** Inside a git work tree, target tag does not exist yet, HEAD is not at the latest tag with a clean working tree.
3. **Pre-check build.** `./build.sh` run before any writes; skipped with `--no-build` or `--dry-run`.
4. **Detect version targets.** Scans `cmd/*/main.go` for `programVersion`, `TEMPLATE_VERSION` and `internal/templates/version.go` (each presence-gated). Multi-binary repos are picked up automatically.
5. **Detect CHANGELOG targets + fail-fast idempotency guard.** Root `CHANGELOG.md` and `internal/templates/CHANGELOG.md` (template-repo case). If any target already contains a row for the target version, prep exits with a fatal error before any writes.
6. **Parse AC refs.** `AC[0-9]+` scan on the release message; composites like `AC60+AC61` yield multiple refs.
7. **Apply writes.** Version bumps (per-file idempotent no-op when the file already has the target value); CHANGELOG row insertion under `| Unreleased | |`; AC file deletions plus `-critique.md` companion deletions. Skipped when `--dry-run`.
8. **Post-check build.** `./build.sh` run after writes; skipped with `--no-build` or `--dry-run`.
9. **Print release command.** Exactly `./build.sh vX.Y.Z "message"` — nothing else.

CHANGELOG row shape (enforced by prep's insertion code and by convention):

- File shape: `# Changelog` heading, then a 2-column markdown table (`| Version | Summary |` with a `|---|---|` separator); first data row is `| Unreleased | |`, followed by one row per release (e.g., `| <version> | <AC-ref>: <one-line summary> |`).
- Summaries are single-line, ≤ 500 characters; lead with the AC reference if any.
- Versions are unprefixed (`0.29.0`, not `v0.29.0`).
- Do not backfill historical tags or invent alternative shapes (Keep-a-Changelog, sectioned `## vX.Y.Z`, etc.).

Flags: `--dry-run` (or `-n`) prints intended writes without touching the working tree; `--no-build` skips phases 3 and 8. Both are for power users or tests — the common path is plain `go run ./cmd/prep/ vX.Y.Z "message"`.
