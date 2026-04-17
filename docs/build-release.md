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

## Drift Check

After installing binaries, the build tool runs a passive governance drift check via `governa enhance -d` (self-review mode). If `governa` was installed by this build, it uses that exact binary; otherwise it falls back to any `governa` on `$PATH`. If neither is available the step is silently skipped.

When drift is detected the build prints a `summary:` line (e.g. `summary: 1 changed, 0 added, 0 removed`). The check is advisory and never blocks the build.

## Sandboxed Execution

Under sandboxed execution that blocks Go's build cache (look for `writing stat cache ... operation not permitted`), `staticcheck` may print a `matched no packages` warning even though it ran cleanly. Treat as advisory unless real findings appear; an unrestricted rerun confirms.

## Canonical Release Commands

```bash
go run ./cmd/rel vX.Y.Z "release message"
```

Convenience wrapper:

```bash
./build.sh vX.Y.Z "release message"
```

The release command always asks for interactive confirmation before it runs the git steps.

## Release Trigger

Only perform release work when the user explicitly asks for release, publish, or version preparation.

## Release Checklist

1. run the canonical build and validation flow
2. confirm root docs and architecture notes are current
3. update `CHANGELOG.md` for the release
4. confirm `TEMPLATE_VERSION` matches the template release version being prepared
5. prepare version or publish artifacts only within the explicit release request

## Pre-Release Checklist

Do not start this checklist unless the user explicitly asks to prep for release or equivalent.

Before offering a release commit or release command:

1. **Check the latest git tag and working tree.** Run `git tag --sort=-v:refname | head -1` and `git status` to confirm the working tree has uncommitted changes. If the tree is clean and the latest tag matches `programVersion` in `cmd/governa/main.go`, `TemplateVersion` in `internal/templates/version.go`, and `TEMPLATE_VERSION`, there is nothing to release — do not proceed. Never assume the current version from build output or prior conversation; always verify from git.
2. **Run the canonical build and validation flow.** Fix failures until clean.
3. **Ask the user whether any required manual or live acceptance checks were run.**
4. **Audit `arch.md` and any affected reference docs against the actual behavior.**
5. **Update `CHANGELOG.md`.** Move the current `Unreleased` summary into a new row for the release version directly below `Unreleased`, then restore an empty `Unreleased` row.

    - File shape: `# Changelog` heading, then a 2-column markdown table (`| Version | Summary |` with a `|---|---|` separator); first data row is `| Unreleased | |`, followed by one row per release (e.g., `| <version> | <AC-ref>: <one-line summary> |`).
    - Summaries are single-line, ≤ 500 characters; lead with the AC reference if any.
    - Versions are unprefixed (`0.29.0`, not `v0.29.0`).
    - Do not backfill historical tags or invent alternative shapes (Keep-a-Changelog, sectioned `## vX.Y.Z`, etc.).
    - When motivated by consumer sync feedback, credit the consumer: `(addresses <consumer> feedback from vX.Y.Z sync)`.
    - When an AC closes a consumer-tracked IE, include `closes <consumer>:IE<N>` so sync can advise the consumer to retire the entry.
6. **Confirm `TEMPLATE_VERSION` matches the intended template release version.**
7. **Remove or reprioritize completed roadmap items in `plan.md`.**
8. **Remove completed AC files.** Consolidate their decisions into durable docs and delete the AC files before release; release prep is not complete while completed AC files remain (keep `ac-template.md`). Move any `docs/ac<N>-<slug>-feedback.md` companion to `.governa/feedback/ac<N>-<slug>.md` instead of deleting — emit a one-line confirmation per file moved so the director sees the handoff. (See `docs/ac-template.md` Companion Artifacts for the full convention, including `-critique.md` and `-dispositions.md`.)
9. **Present the canonical release command for the user to run or approve.** The release message must be **≤ 80 characters** — `cmd/rel` enforces this and will reject longer messages. Count before presenting. Present only the command; do not add trailing commentary explaining what it does, how the wrapper routes, or what prompts will appear. The director already knows.

## Release Artifacts

- `CHANGELOG.md` is the human-readable release history
- `TEMPLATE_VERSION` is the machine-readable template contract version used by generated repos and future refresh tooling
- for this repo, keep `TEMPLATE_VERSION` aligned with the released template version unless there is a deliberate reason to version them separately
