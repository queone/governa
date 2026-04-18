# Build and Release

Reference for this repo's build pipeline, pre-release checklist, and acceptance test conventions. The enforceable one-liners live in `AGENTS.md`; this document explains the pipeline, the steps, and the rationale.

## Build

This repo has a single canonical build/test workflow: `./build.sh`.

`./build.sh` is a thin Bash dispatcher. The real implementation lives in `go run ./cmd/build` (build/test) and `go run ./cmd/rel` (release). Both `cmd/build` and `cmd/rel` are `go run` entrypoints — they are intentionally not installed as binaries.

The build pipeline runs these steps in order, fail-hard on each:

1. `go mod tidy` — ensure `go.mod` and `go.sum` are consistent
2. `go fmt ./...` — **fail-hard.** If `go fmt` rewrote any file (non-empty stdout), the build fails. Re-run after committing the formatting fix.
3. `go fix ./...` — advisory; output is logged but does not break the build
4. `go vet ./...` — **fail-hard**
5. Test suite with coverage — fail-hard on any test failure
6. `staticcheck ./...` — **fail-hard.** Installed via `go install staticcheck@latest` before each run.
7. Binary build — installs utilities to `$GOPATH/bin`

Invoking individual Go tools directly skips the tidy/fmt/lint pipeline above. A "passing" direct invocation can still produce a build that `./build.sh` would reject. The wrapper guarantees that what passes locally is what would pass in CI.

## Sandboxed Execution

Under sandboxed execution that blocks Go's build cache (look for `writing stat cache ... operation not permitted`), `staticcheck` may print a `matched no packages` warning even though it ran cleanly. Treat as advisory unless real findings appear; an unrestricted rerun confirms.

## Acceptance Tests

This repo uses a labeled-AT convention adopted with the AC-first workflow. Every AT in an AC document must be labeled `[Automated]` or `[Manual]`.

- **Automated** — The result can be verified from CLI output, test assertions, or file inspection. Automated ATs are run during implementation and re-run as part of the pre-release checklist.
- **Manual** — Requires a live end-to-end action and must be confirmed by the user. The agent cannot self-verify these.

Default to Automated whenever the result is verifiable without a live external service. Manual ATs add friction to the release flow, so reserve them for behaviors that genuinely cannot be checked any other way.

## Pre-Release Checklist

Do not begin this checklist until the user explicitly asks to prep for release or equivalent. This is gated by the release-prep trigger rule in `AGENTS.md` (Release Or Publish Triggers).

1. **Check the latest git tag and working tree.** Run `git tag --sort=-v:refname | head -1` and `git status`. If the tree is clean and the latest tag matches the `programVersion` constant in the main binary source, there is nothing to release — do not proceed. Never assume the current version from build output or prior conversation; always verify from git.
2. **Run `./build.sh`.** Fix all failures until the build is clean.
3. **Ask the user whether the live ATs were run.** Manual ATs cannot be verified from CLI output and require explicit confirmation.
4. **Audit `arch.md` against the code.** Verify affected reference docs are current.
5. **Update `CHANGELOG.md`.** Move the current `Unreleased` summary into a new row for the release version directly below `Unreleased`, then restore an empty `Unreleased` row.

    - File shape: `# Changelog` heading, then a 2-column markdown table (`| Version | Summary |` with a `|---|---|` separator); first data row is `| Unreleased | |`, followed by one row per release (e.g., `| <version> | <AC-ref>: <one-line summary> |`).
    - Summaries are single-line, ≤ 500 characters; lead with the AC reference if any.
    - Versions are unprefixed (`0.29.0`, not `v0.29.0`).
    - Do not backfill historical tags or invent alternative shapes (Keep-a-Changelog, sectioned `## vX.Y.Z`, etc.).
    - When motivated by consumer sync feedback, credit the consumer: `(addresses <consumer> feedback from vX.Y.Z sync)`.
    - When an AC closes a consumer-tracked IE, include `closes <consumer>:IE<N>` so sync can advise the consumer to retire the entry.
6. **Bump version constants.** Use the tag from step 1 as the baseline.
7. **Remove completed features from `plan.md`.**
8. **Consolidate finished AC decisions into durable docs, then delete the AC file.** Do not delete AC files that are PENDING, IN PROGRESS, or DEFERRED — they are still active contracts. Only completed (released) ACs are deleted. Move any `docs/ac<N>-<slug>-feedback.md` companion to `.governa/feedback/ac<N>-<slug>.md` instead of deleting — emit a one-line confirmation per file moved so the director sees the handoff.
9. **Present the release command for the user to run.** The agent never runs the release command directly. The release message must be **≤ 80 characters** — `cmd/rel` enforces this and will reject longer messages. Count before presenting. Present only the command; do not add trailing commentary explaining what it does, how the wrapper routes, or what prompts will appear. The director already knows.

The release command (`./build.sh vX.Y.Z "message"`) executes `cmd/rel`, which orchestrates `git add → commit → tag → push tag → push branch` and produces recovery guidance if any step fails.

## Template Upgrade

This repo was generated from a governa governance template. To check for template updates:

1. Run `governa sync` to generate a review document with per-file recommendations. This also updates `TEMPLATE_VERSION` to the current template version.
2. Compare `TEMPLATE_VERSION` in this repo against the template's current version. `TEMPLATE_VERSION` reflects the last template version this repo was evaluated against, not the original bootstrap version.
3. `.governa/manifest`, if present, records SHA-256 checksums of each file at bootstrap time. This enables comparison to distinguish your customizations from stale template content.
4. **Produce a per-sync feedback artifact.** Every governa sync that produces an adoption AC must also produce a separate file at `docs/ac<N>-<slug>-feedback.md` capturing genuine observations about the sync output (template defaults that fight the repo, scoring gaps, methodology issues, things that landed well). The artifact is out-of-band — not folded into the sync AC's body — so the feedback exists independently of the adoption work. The director routes its content upstream to governa. At release prep, the artifact is moved to `.governa/feedback/ac<N>-<slug>.md` (not deleted) so the feedback persists for governa's future `enhance -r` runs. This codifies the Feedback step of the sync's Evaluation Methodology so it cannot be silently skipped. (See `docs/ac-template.md` Companion Artifacts for the full convention, including `-critique.md` and `-dispositions.md`.)
5. When a file should remain a stable repo-specific carve-out after review, record that decision with `governa ack <path> --reason "..."` so future syncs move it into `## Acknowledged Drift` instead of re-flagging it in `## Adoption Items`.

Template refresh is operator-driven. The governa tool proposes; the repo maintainer decides what to adopt.
