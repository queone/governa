# Build and Release

Reference for this repo's build pipeline, pre-release checklist, and acceptance test conventions. The enforceable one-liners live in `AGENTS.md`; this document explains the pipeline, the steps, and the rationale.

## Build

This repo has a single canonical build/test workflow: `./build.sh`.

`./build.sh` is a thin Bash dispatcher. The real implementation lives in `go run ./cmd/build` (build/test) and `go run ./cmd/rel` (release). Both `cmd/build` and `cmd/rel` are `go run` entrypoints — they are intentionally not installed as binaries.

The build pipeline runs these steps in order, fail-hard on each:

1. `mdcheck` — **fail-hard.** Scans tracked markdown files for nested-fence bugs (3-backtick outer fence containing a tagged 3-backtick inner opener). The fix is to widen the outer fence to 4+ backticks or switch to `~~~`.
2. `go mod tidy` — ensure `go.mod` and `go.sum` are consistent
3. `go fmt ./...` — **fail-hard.** If `go fmt` rewrote any file (non-empty stdout), the build fails. Re-run after committing the formatting fix.
4. `go fix ./...` — advisory; output is logged but does not break the build
5. `go vet ./...` — **fail-hard**
6. Test suite with coverage — fail-hard on any test failure
7. `staticcheck ./...` — **fail-hard.** Installed via `go install staticcheck@latest` before each run.
8. Binary build — installs utilities to `$GOPATH/bin`

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

The operator flow is two steps:

1. **Run `./prep.sh vX.Y.Z "message"`.** Stages version bumps, inserts the CHANGELOG row, deletes completed AC files (plus `-critique.md` and `-dispositions.md` companions), moves any `-feedback.md` companion to `.governa/feedback/`, runs validation builds before and after, and prints the canonical release command. The agent determines the version (semver classification from the AC's scope) and drafts the release message (≤ 80 characters) before invoking prep.
2. **Run the printed release command (`./build.sh vX.Y.Z "message"`).** `cmd/rel` shows `git status --short`, lists every git step it will execute, and prompts for interactive confirmation. On approval it orchestrates `git add → commit → tag → push tag → push branch`. Optional: run `git diff` between the two steps if you want to inspect the CHANGELOG row wording and version-string values before committing — `cmd/rel`'s own status preview is sufficient to catch wrong-file inclusions or deletions.

Present only the release command after prep; do not add trailing commentary about wrapper routing or prompts. The director already knows.

### Appendix: what prep does

`./prep.sh` runs nine phases internally so the operator flow above stays short:

1. **Validate inputs.** Semver pattern (`vX.Y.Z`), message non-empty and ≤ 80 characters.
2. **Validate git state.** Inside a git work tree, target tag does not exist yet, HEAD is not at the latest tag with a clean working tree.
3. **Pre-check build.** `./build.sh` run before any writes; skipped with `--no-build` or `--dry-run`.
4. **Detect version targets.** Scans `cmd/*/main.go` for `programVersion`, plus `TEMPLATE_VERSION` and `internal/templates/version.go` when present (both presence-gated). Multi-binary repos are picked up automatically.
5. **Detect CHANGELOG targets + fail-fast idempotency guard.** Root `CHANGELOG.md` and `internal/templates/CHANGELOG.md` (template-repo case). If any target already contains a row for the target version, prep exits with a fatal error before any writes.
6. **Parse AC refs.** `AC[0-9]+` scan on the release message; composites like `AC60+AC61` yield multiple refs.
7. **Apply writes.** Version bumps (per-file idempotent no-op when the file already has the target value); CHANGELOG row insertion under `| Unreleased | |`; AC file deletions plus `-critique.md`/`-dispositions.md` companion deletions; `-feedback.md` moved to `.governa/feedback/ac<N>-<slug>.md`. Skipped when `--dry-run`.
8. **Post-check build.** `./build.sh` run after writes; skipped with `--no-build` or `--dry-run`.
9. **Print release command.** Exactly `./build.sh vX.Y.Z "message"` — nothing else.

CHANGELOG row shape (enforced by prep's insertion code and by convention):

- File shape: `# Changelog` heading, then a 2-column markdown table (`| Version | Summary |` with a `|---|---|` separator); first data row is `| Unreleased | |`, followed by one row per release (e.g., `| <version> | <AC-ref>: <one-line summary> |`).
- Summaries are single-line, ≤ 500 characters; lead with the AC reference if any.
- Versions are unprefixed (`0.29.0`, not `v0.29.0`).
- Do not backfill historical tags or invent alternative shapes (Keep-a-Changelog, sectioned `## vX.Y.Z`, etc.).
- When motivated by consumer sync feedback, credit the consumer: `(addresses <consumer> feedback from vX.Y.Z sync)`.
- When an AC closes a consumer-tracked IE, include `closes <consumer>:IE<N>` so sync can advise the consumer to retire the entry.

Flags: `--dry-run` (or `-n`) prints intended writes without touching the working tree; `--no-build` skips phases 3 and 8. Both are for power users or tests — the common path is plain `./prep.sh vX.Y.Z "message"`.

## Template Upgrade

This repo was generated from a governa governance template. To check for template updates:

1. Run `governa sync` to generate a review document with per-file recommendations. This also updates `TEMPLATE_VERSION` to the current template version.
2. Compare `TEMPLATE_VERSION` in this repo against the template's current version. `TEMPLATE_VERSION` reflects the last template version this repo was evaluated against, not the original bootstrap version.
3. `.governa/manifest`, if present, records SHA-256 checksums of each file at bootstrap time. This enables comparison to distinguish your customizations from stale template content.
4. **Produce a per-sync feedback artifact.** Every governa sync that produces an adoption AC must also produce a separate file at `docs/ac<N>-<slug>-feedback.md` capturing genuine observations about the sync output (template defaults that fight the repo, scoring gaps, methodology issues, things that landed well). The artifact is out-of-band — not folded into the sync AC's body — so the feedback exists independently of the adoption work. The director routes its content upstream to governa. At release prep, the artifact is moved to `.governa/feedback/ac<N>-<slug>.md` (not deleted) so the feedback persists for governa's future `enhance -r` runs. This codifies the Feedback step of the sync's Evaluation Methodology so it cannot be silently skipped. (See `docs/ac-template.md` Companion Artifacts for the full convention, including `-critique.md` and `-dispositions.md`.)
5. **Feedback-file filename convention.** When moving `-feedback.md` to `.governa/feedback/`, encode the governa version the feedback was produced against in the destination filename — for example, `ac<N>-governa-sync-<vX.Y.Z>.md`. This lets future `governa sync` runs detect when governa has addressed the feedback and offer automated cleanup via `-f` / `--prune-feedback`. Filenames without a parseable `X.Y.Z` substring (e.g., pre-convention `ac<N>-governa-sync-adoption.md`) are left for manual cleanup.
6. **Prune addressed feedback files.** On a sync where governa's CHANGELOG includes an `(addresses <consumer> feedback from v<range> syncs)` credit matching this repo, the sync review flags the closed feedback files under Advisory Notes. To delete them in the same sync run, pass `-f` (or `--prune-feedback`) — the flag is opt-in and respects `-d` / `--dry-run` (emits `prune: would remove <path>` without deleting). Pre-convention filenames are never pruned automatically.
7. When a file should remain a stable repo-specific carve-out after review, record that decision with `governa ack <path> --reason "..."` so future syncs move it into `## Acknowledged Drift` instead of re-flagging it in `## Adoption Items`.

Template refresh is operator-driven. The governa tool proposes; the repo maintainer decides what to adopt.
