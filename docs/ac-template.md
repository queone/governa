Copy this file to `docs/ac<N>-<slug>.md`, where `slug` is a kebab-case identifier and `N` follows the monotonic-numbering rule below. Set the file's heading to `# AC<N> Title`.

AC numbering is monotonic across release-prep deletions. Determine `N` by taking the maximum of (a) AC numbers currently in `docs/` and (b) AC numbers anywhere in `git log --all --pretty=%B` output (which covers commit subject + body — count every AC reference on every line, even when a single commit names multiple, e.g., `AC53+AC54`). Prior ACs removed during release prep still count. `N` is that maximum plus one.

The AC is the implementation contract for one approved roadmap item. The full development cycle that wraps around this template lives in `docs/development-cycle.md`. The enforceable rules around when to draft, review, and integrate an AC live in `AGENTS.md`.

## Companion Artifacts

ACs produce three optional companion files that live alongside the AC doc in `docs/`. All are instructional scaffolding — delete this `## Companion Artifacts` section when copying the template into a real AC.

- **`docs/ac<N>-<slug>-critique.md`** — external-review findings against the AC before implementation begins. The critique gate requires either this file or findings integrated directly into the AC itself (see `AGENTS.md` Approval Boundaries). **QA-owned.** DEV does not write to this file. DEV's response to findings is the AC revision itself (tracked in `git log`/`git diff`), optionally summarized in a `Disposition Log` subsection under Implementation Notes for ACs with extensive critique rounds. Critique file structure is **round-append** with a five-field terminator-with-residuals shape. See `docs/critique-protocol.md` for the full protocol (round heading levels, `F-new-N` monotonic numbering, terminator field order, DEV cross-reference sections). Deleted at release prep alongside the AC.
- **`docs/ac<N>-<slug>-feedback.md`** — per-sync feedback artifact produced by `governa sync` ACs. Captures genuine observations about the sync output (template defaults that fight the repo, scoring gaps, methodology issues, things that landed well). At release prep, moved to `.governa/feedback/ac<N>-<slug>.md` instead of deleted, so the consumer's feedback persists for governa's future `enhance -r` runs.
- **`docs/ac<N>-<slug>-dispositions.md`** — companion artifact for partial-adopt files. When an AC performs partial adoption — adopts some template content while preserving some existing content — list every preserved difference with: (1) content kept, (2) template content rejected, (3) repo-specific reason. Deleted at release prep alongside the AC. Before deletion, consolidate any long-term WHY-reasons into inline comments or durable docs. This records what was preserved during one AC; use `governa ack <path> --reason "..."` separately when a stable file-level carve-out should stop reappearing in future sync reviews.

# AC<N> Title

One-paragraph summary of what this AC delivers and why. State the change in plain terms — feature, refactor, infrastructure, or doc. Note whether it is code or doc-only.

## Summary

Describe the change in one short paragraph.

## Objective Fit

1. **Which part of the primary objective?** Tie the work to a concrete part of the product or platform objective. If you cannot answer this in one sentence, the AC is probably not ready.
2. **Why not advance a higher-priority task instead?** Either name the higher-priority blocker that this unblocks, or label the work as an intentional pivot and explain the trade-off.
3. **What existing decision does it depend on or risk contradicting?** Reference the prior AC, architectural decision, or shipped feature that this builds on. If it contradicts a prior decision, say so explicitly and explain why the contradiction is intentional.
4. **Intentional pivot?** If yes, state that here and reaffirm point 2.

## In Scope

List the concrete changes this AC will make. Use sub-headings for grouping (e.g. "Files to create", "Files to modify", "Schema changes"). Be specific — file paths, function names, table columns. The In Scope list is the authoritative scope contract.

### Files to create

- `path/to/new_file` — what it contains
- `docs/new-doc.md` — what it documents

### Files to modify

- `existing_file` — what changes
- `arch.md` — what gets updated

### Schema changes

(If any. Include the new schema version and the migration step.)

## Out Of Scope

List things the AC explicitly does **not** do. This is as important as the In Scope list — it bounds the change and prevents scope creep during implementation.

- Things deferred to a later AC (link the deferral)
- Adjacent improvements that would be tempting but are not required
- Things that look in scope but aren't (called out to prevent confusion)

## Implementation Notes

(Optional but encouraged.) Notes on approach, gotchas, edge cases, or design decisions that the implementer needs to know but that don't belong in the spec proper. If a particular approach was already considered and rejected, capture it here.

## Acceptance Tests

Every AT must be labeled `[Automated]` or `[Manual]` (see `docs/build-release.md` for the labeling convention).

**AT1** [Automated] — One-line description of what is verified, with the exact check (file existence, grep pattern, SQL query, or CLI output).

**AT2** [Automated] — ...

**AT3** [Manual] — One-line description plus the live action the user must take to confirm the result.

## Documentation Updates

List the docs that must be updated as part of this AC. If a change touches code that has corresponding documentation, update those docs in the same pass.

- `arch.md` — what section
- `README.md` — what section
- `CHANGELOG.md` — the release row is added at release prep time, not during implementation (the file itself is created by `governa sync` as a stub)

## Director Review

List every scope or wording trade-off chosen between two or more viable options during the DEV/QA cycle. Each entry names the option taken and a one-line why. Empty is allowed — write `None` when the AC has no judgment calls worth surfacing.

QA's final-round terminator cross-checks that this list is exhaustive (not just the calls DEV feels uncertain about). Omissions surfaced by QA land in the critique file's `Director attention` field.

- Decision X: option taken (alternatives considered: A, B). Why: <one-line>.

## Status

`PENDING` — awaiting user authorization to begin implementation.

(Other valid states: `IN PROGRESS`, `DEFERRED` (with reason and tracking ref). For partial completion, list status by phase. Completed ACs are deleted at release time per the development cycle — do not change status to DONE before deletion.)
