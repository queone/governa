# AC59 Governance Doc Canonicalization

Tighten governance docs by removing duplicated rule restatements and brittle cross-references surfaced during the template-audit pass that followed AC58's `Current Platform` removal. Four doc-only changes, each consolidating a rule into a single authoritative location: role-assignment logic (lives in `AGENTS.md`, was also restated in `docs/roles/README.md`); the adversarial-check governance principle (lives in `docs/roles/README.md` Critical Principle, was also restated in every role file's `## Counterparts` section); a brittle step-number citation in `AGENTS.md`; and a missing step-3 detail in the development-cycle template. Ships bundled with AC58 in release 0.37.0. MINOR (user-visible governance doc changes, no code).

## Summary

Four consolidation moves. (1) `docs/roles/README.md` Role Assignment section is trimmed from a 7-point restatement to a pointer that references `AGENTS.md` Interaction Mode; `AGENTS.md` stays unchanged for this item. (2) Every role file's `## Counterparts` section loses the restated adversarial-check paragraph; it becomes a pointer to `docs/roles/README.md` Critical Principle (which keeps the authoritative statement). (3) `AGENTS.md` Release Or Publish Triggers line citing `docs/build-release.md` Pre-Release Checklist "step 5" is reworded to cite the section without a numbered step, so renumbering does not silently stale the reference. (4) The overlay template + example `docs/development-cycle.md` step 3 is updated to include the QA-iteration detail present in governa's root version, so multi-agent consumer repos get the recommended workflow in their generated docs. Propagations applied across root + base template + code overlay + doc overlay + code example + doc example where each file family exists.

## Objective Fit

1. **Which part of the primary objective?** "Narrow, usable governance template" — four rule restatements and a brittle reference are tech debt that creates drift risk. Consolidating them tightens the template and reduces the chance that a future rule update lands in one copy but not the others.
2. **Why not advance a higher-priority task instead?** Priorities section of `plan.md` is empty (post-AC57); IE4 (LLM-assisted sync review) is larger unshaped work. This is bounded cleanup surfaced by reviewing AC58's removal pattern; it bundles into the already-in-flight 0.37.0 release so it ships at no extra release cost.
3. **What existing decision does it depend on or risk contradicting?** Builds on AC34 (roles directory + delivery model), AC35 (governance-first latch rule), AC50 (role files name counterparts explicitly), AC55 (governance refinement bundle). The role-assignment logic and the adversarial-check principle were each written in their current dual-location shape during earlier rounds; this AC finishes the canonicalization that those ACs partially started. No contradiction.
4. **Intentional pivot?** No. Direct cleanup.

## In Scope

### Files to modify

Item 1 — role-assignment pointer in roles README:

- `docs/roles/README.md` — trim `## Role Assignment` section to a one-paragraph pointer. Exact replacement text: `See `AGENTS.md` Interaction Mode for the full role-assignment rule — default to maintainer when `maintainer.md` is present, explicit assignment otherwise, case-insensitive lookup, `director.md` is reference-only.`
- `internal/templates/overlays/code/files/docs/roles/README.md.tmpl` — same.
- `internal/templates/overlays/doc/files/docs/roles/README.md.tmpl` — same.
- `examples/code/docs/roles/README.md` — same.
- `examples/doc/docs/roles/README.md` — same. (File verified present in this scope.)

Item 2 — adversarial-check pointer in role files (scope: `dev.md` and `qa.md` only):

- `docs/roles/dev.md` — in `## Counterparts`, keep the per-counterpart bullets (identifying QA and Director with their specific interactions) and replace the trailing adversarial-check paragraph with: `See `docs/roles/README.md` Critical Principle for the governance rationale on routing disagreements through the director.`
- `docs/roles/qa.md` — same pattern.
- `internal/templates/overlays/code/files/docs/roles/dev.md.tmpl`, `qa.md.tmpl` — same.
- `internal/templates/overlays/doc/files/docs/roles/dev.md.tmpl`, `qa.md.tmpl` — same.
- `examples/code/docs/roles/dev.md`, `qa.md` — same.
- `examples/doc/docs/roles/dev.md`, `qa.md` — same. (Files verified present.)

`maintainer.md` is intentionally **excluded** from this item. Its `## Counterparts` section does not restate the multi-role adversarial-check principle — it carries a role-appropriate single-agent self-review paragraph that points to the role's own self-review discipline. Leaving maintainer unchanged preserves the right rationale for the single-agent role; pointing it at the multi-role Critical Principle would introduce a false rationale. This exclusion is captured in AT2 (see below).

Item 3 — remove brittle cross-file numbered-step citations:

- `AGENTS.md` (root) — Release Or Publish Triggers bullet 2: replace `"docs/build-release.md" Pre-Release Checklist step 5` with `"docs/build-release.md" Pre-Release Checklist CHANGELOG step` (no number).
- `internal/templates/base/AGENTS.md` — same.
- `examples/code/AGENTS.md` and `examples/doc/AGENTS.md` — same. (Text confirmed present in all four variants.)
- `internal/templates/overlays/code/files/docs/build-release.md.tmpl` line 66 — replace `"codifies step 7 of the sync's Evaluation Methodology"` with `"codifies the Feedback step of the sync's Evaluation Methodology"` (no number). The sync methodology's numbering is code-controlled in `renderSyncReview` and is not stable across code changes; citing the step by name survives future renumbering.
- `examples/code/docs/build-release.md` — same.

Scan basis: a repository-wide grep for `"step [0-9]+"` across `AGENTS.md`, `docs/`, `internal/templates/`, and `examples/` found exactly these four text locations as cross-file brittle references (two distinct citations, each appearing in template + example copies). Same-file numbered references (a step citing another step in the same document, e.g., `"Use the tag from step 1"` in `docs/build-release.md.tmpl:52`) are stable by construction and are explicitly out of scope.

Item 4 — propagate step 3 QA-iteration detail:

- `internal/templates/overlays/code/files/docs/development-cycle.md.tmpl` — step 3 reworded from `"Review and tighten scope before implementation."` to match root's version: `"Review and tighten scope before implementation. When QA files findings on the AC, DEV responds in the conversation with proposed changes or explicit disagreement, but does not edit the AC file until QA replies and the director confirms the iteration is closed. Repeat until the AC is implementation-ready."`
- `examples/code/docs/development-cycle.md` — same.

### Implementation sequence

Apply items 1–4 in order. Each item touches a distinct file family with minimal cross-dependency; the order above moves from outermost (docs/roles/README) inward, ending with development-cycle which is structurally independent.

## Out of Scope

- Restructuring `AGENTS.md` Interaction Mode itself. The rule already lives there; we are not expanding, reformatting, or renumbering it.
- Changing the role-specific Rules sections in `dev.md` / `qa.md` / `maintainer.md`. Only the adversarial-check paragraph within `## Counterparts` changes.
- Any edits to `AGENTS.md` governed sections other than the single line in Release Or Publish Triggers. Other governed sections are untouched.
- Removing `## Counterparts` sections entirely. The role-specific counterpart identifications (DEV lists QA + Director, QA lists DEV + Director, etc.) are genuinely role-specific content and remain.
- DOC-overlay-specific content beyond role files. DOC overlay has its own `dev.md.tmpl` with content-workflow rules; only the adversarial-check paragraph is touched, not the DOC-specific rules.
- Governance changes to workflow itself. Step 3 in development-cycle gains the QA-iteration wording that the root already uses — we are propagating existing behavior, not inventing a new gate.
- `docs/roles/director.md` — the director role file is reference material, not a rule-bearing role file, and does not carry the adversarial-check restatement.
- Base AGENTS.md template's Interaction Mode paragraph. Role-assignment logic already lives there correctly; `docs/roles/README.md` is the side that needs trimming, not `AGENTS.md`.

## Implementation Notes

- **AC numbering.** Git log max is AC57 (AC58 is in-flight, uncommitted at draft time). Under the monotonic rule shipped in AC57, the next number would auto-compute as AC58. AC58 is conceptually reserved for the in-flight work, so this AC is drafted as AC59 explicitly. Directors intentional numbering overrides the auto-rule; the rule will self-correct once AC58 is committed and git log max becomes AC58.
- **Role-assignment canonicalization direction.** `AGENTS.md` Interaction Mode is the authoritative home for role-assignment logic — it is loaded every session and is the first place a fresh agent reads. `docs/roles/README.md` becomes a pointer, not vice versa, because (a) AGENTS.md is always loaded and roles/README.md is only read on demand, and (b) AGENTS.md already has the complete rule inline; the README version is the restatement.
- **Adversarial-check canonicalization direction.** `docs/roles/README.md` Critical Principle is the authoritative home for the adversarial-check governance principle because it is the only file that exists to describe the full multi-role model. Role files are session-time behavioral contracts for a specific role; they can cite the principle without restating it.
- **Counterparts section shape after trim.** In `dev.md` and `qa.md` the per-counterpart bullets stay (DEV identifies QA and Director as its counterparts with role-specific interaction rules). Only the trailing paragraph — "The value of this split is the adversarial check. If DEV and QA collude..." — is replaced by a pointer. This preserves the role file's self-contained read for identifying counterparts while removing the duplicated principle. `maintainer.md` is not touched: its Counterparts paragraph is about the single-agent self-review obligation (conflict of interest, stricter-than-QA self-review), which is genuinely role-specific and not a restatement of any governance principle.
- **Brittle-reference scan and replacement.** A repository-wide grep for `"step [0-9]+"` across `AGENTS.md`, `docs/`, `internal/templates/`, and `examples/` (excluding the AC59 draft itself) found exactly two cross-file brittle references, each propagated to multiple copies: (a) `AGENTS.md` → `docs/build-release.md` "step 5" (4 copies: root, base template, code example, doc example), and (b) `docs/build-release.md.tmpl` → sync Evaluation Methodology "step 7" (2 copies: overlay template, code example). Both are in Item 3 scope. Same-file numbered references (a doc citing its own numbered steps) are stable by construction and are explicitly out of scope. After this AC lands, the only numbered references remaining in governance docs are internal (same-file) and historical (CHANGELOG row summaries of prior releases, which are immutable descriptions).
- **Template + example propagation.** Items 1, 2, and 4 apply to root + overlay template + example; Item 3 applies to root + base template + example. Every propagation preserves the same wording across copies. All example files in this AC's scope are verified present in the repo, so every listed propagation is a required edit, not an optional one.
- **Semver classification.** MINOR. Governance doc shape changes are user-visible for consumer repos that sync. Consumers on the older shape will see `adopt` recommendations for the affected files on their next sync, which is the correct signal — they can adopt verbatim or run `governa ack` if they've intentionally diverged.
- **Bundling with AC58.** Shipping in the same 0.37.0 release as AC58. CHANGELOG 0.37.0 row is extended during release prep to reference both ACs.

## Acceptance Tests

Every AT labeled `[Automated]` or `[Manual]`.

**AT1** [Automated] — `TestRolesReadmeRoleAssignmentIsPointer`: in `docs/roles/README.md`, the `## Role Assignment` section contains no numbered list and contains a reference to `AGENTS.md` Interaction Mode. Verified across all five copies: root, code-overlay template, doc-overlay template, code example, doc example. Grep: section must not match `/^[0-9]+\./m` and must match `AGENTS\.md.*Interaction Mode`.

**AT2a** [Automated] — `TestDevAndQaCounterpartsCiteCriticalPrinciple`: in `dev.md` and `qa.md` (across root + code-overlay template + doc-overlay template + code example + doc example), the `## Counterparts` section contains the pointer string `"Critical Principle"` referencing `docs/roles/README.md`, and does not contain the phrase `"adversarial check"`.

**AT2b** [Automated] — `TestMaintainerCounterpartsUnchanged`: `maintainer.md` (across root + code-overlay template + doc-overlay template + code example + doc example) still carries the role-appropriate self-review paragraph in its `## Counterparts` section. Specifically: the phrase `"self-review"` is present and the phrase `"adversarial check"` is absent (already true pre-AC59). This locks maintainer as out-of-scope for Item 2 so future refactors cannot silently sweep it into the multi-role pointer pattern.

**AT3** [Automated] — `TestAgentsMdHasNoBrittleStepNumber`: each `AGENTS.md` variant (root, `internal/templates/base/AGENTS.md`, `examples/code/AGENTS.md`, `examples/doc/AGENTS.md`) produces zero matches for the pattern `Pre-Release Checklist step [0-9]+` and still contains the phrase `Pre-Release Checklist`. Grep form: `rg -n "Pre-Release Checklist step [0-9]+" <file>` must return no hits, and `rg -n "Pre-Release Checklist" <file>` must return at least one hit.

**AT3b** [Automated] — `TestBuildReleaseHasNoBrittleSyncStepNumber`: `internal/templates/overlays/code/files/docs/build-release.md.tmpl` and `examples/code/docs/build-release.md` produce zero matches for the pattern `step [0-9]+ of the sync` and contain the phrase `Feedback step of the sync`. Grep form: `rg -n "step [0-9]+ of the sync" <file>` must return no hits, and `rg -n "Feedback step of the sync" <file>` must return at least one hit.

**AT4** [Automated] — `TestDevelopmentCycleStep3HasQaIterationDetail`: `docs/development-cycle.md` (root), `internal/templates/overlays/code/files/docs/development-cycle.md.tmpl`, and `examples/code/docs/development-cycle.md` each contain the phrase `"When QA files findings on the AC"` in step 3. Verifies template + example caught up to the root's existing wording.

**AT5** [Automated] — `./build.sh` passes. Domain coverage stays ≥ 82.4% (current AC58 baseline); no regression from doc-only changes.

**AT6** [Automated] — `TestCanonicalizationPassesSelfReview`: after the changes, `governa enhance` (self-review, no `-r`) reports no new structural findings against the embedded baseline for role files, `docs/roles/README.md`, or `docs/development-cycle.md`. This asserts the embedded template and on-disk template remain in lockstep.

## Documentation Updates

- `CHANGELOG.md` and `internal/templates/CHANGELOG.md` — at release prep, the 0.37.0 row is reworded to reference both AC58 and AC59 (`AC58+AC59: ...`) with the combined scope.
- `arch.md` — no change. Governance doc canonicalization does not affect architecture notes.
- `README.md` — no change. No new commands, flags, or user-facing surfaces.
- `plan.md` — no change. No completed roadmap items to prune; IE4 unaffected.
- `docs/build-release.md` — no change; the referent stays the same, only the reference shape in `AGENTS.md` tightens.

## Status

`IN PROGRESS` — implementation complete, awaiting release prep (bundled with AC58 in v0.37.0).

- Item 1 (Role Assignment pointer): 5 files updated.
- Item 2 (adversarial-check pointer in dev/qa): 10 files updated; maintainer.md intentionally unchanged.
- Item 3 (brittle numbered-step references): 6 files updated (4 AGENTS.md + 2 build-release.md).
- Item 4 (development-cycle step 3 propagation): 2 files updated (overlay template + example).
- Tests: 6 new AC59 tests passing (AT1, AT2a, AT2b, AT3, AT3b, AT4); 4 pre-existing tests updated to match new canonical state.
- `./build.sh` clean; governance 88.0% coverage.
