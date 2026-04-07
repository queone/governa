# AC-001 Replace enhance report with AC workflow

## Objective Fit

1. Enhance mode currently produces a standing `docs/enhance-report.md` artifact that accumulates stale review data. Replacing it with the repo's AC workflow means actionable improvements get formalized as discrete, trackable work items instead of drifting in a monolithic report.
2. This is the right next step because v0.1.0 shipped with the report-first model, and the next improvement pass (R2/R3) benefits from a tighter feedback loop between enhance findings and actual implementation.
3. The current enhance implementation is already review-first and conservative. The AC workflow is already documented in `docs/ac-template.md`. This change connects the two.
4. Direct roadmap work -- aligns with R3 (safe refresh path) and reduces doc drift.

## Summary

Change enhance mode so it no longer writes `docs/enhance-report.md`. Instead, when enhance finds actionable improvements (candidates with disposition `accept` or `adapt`), it creates an AC doc under `docs/` using the repo's existing AC template conventions. When no actionable improvements are found, it prints a summary and creates no file.

## In Scope

- Modify `runEnhance` in `internal/bootstrap/bootstrap.go` to produce AC output instead of `enhance-report.md`
- Define selection logic: if one actionable candidate, create its AC doc; if multiple, create an AC doc for the highest-priority candidate and list remaining candidates in a "deferred" section within that AC doc
- Preserve the stdout summary (mode, reference, candidate count, dispositions)
- Remove `renderEnhancementReport` and the `enhance-report.md` writing path
- Remove `docs/enhance-report.md` from the repo
- Update `docs/README.md` to remove the `enhance-report.md` inventory entry
- Update `docs/bootstrap-model.md` to describe the new AC-based output
- Update `README.md` enhance description to reference AC output instead of report file
- Update tests in `internal/bootstrap/bootstrap_test.go` to cover:
  - no-actionable-change case (no file created, summary printed)
  - single-actionable-change case (AC doc created with correct structure)
  - multiple-actionable-change case (single AC doc for top candidate, others listed as deferred)
- Respect dry-run: in dry-run mode, print what AC doc would be created but do not write it

## Out Of Scope

- Changing enhance's review logic, classification, or portability analysis
- Auto-applying template changes (enhance must remain review-first)
- Changing the AC template format itself
- Adding interactive prompts to enhance (selection among multiple candidates is deterministic, not interactive)
- Modifying `new` or `adopt` modes

## Implementation Notes

- AC doc naming convention: `ac-NNN-<slug>.md` where NNN is the next available number (scan `docs/ac-*.md` to determine)
- The AC doc content should be generated from `EnhancementCandidate` fields, mapped to AC template sections:
  - **Summary**: from candidate `Summary` and `Reason`
  - **In Scope**: the specific template target and what the enhancement proposes
  - **Implementation Notes**: source path, portability classification, collision impact
  - **Status**: `PENDING`
- A candidate is **actionable** if its disposition is `accept` or `adapt`. Candidates with disposition `defer` or `reject` are non-actionable and are never selected for AC creation.
- Selection among multiple actionable candidates uses this deterministic ranking:
  1. `accept` + `portable` (highest priority)
  2. `accept` + `needs-review`
  3. `adapt` + `portable`
  4. `adapt` + `needs-review`
- Ties within the same rank are broken by the candidate's position in the sorted candidate list, which is already stable (sorted by area, then section, then path in `ReviewEnhancement`).
- The highest-ranked candidate becomes the AC doc subject. All other actionable candidates are listed in a `## Deferred Candidates` section at the bottom of the AC doc so they are not lost. Non-actionable candidates appear only in the stdout summary.
- The `renderEnhancementReport` function and its helpers (`countEnhancementCandidates`, `displayReferenceRoot`, `displayReferencePath`) may still be useful for stdout summary output. Evaluate which to keep and which to remove.

## Acceptance Tests

- [Automated] Enhance with a reference repo that has no actionable changes: no file created, function returns nil
- [Automated] Enhance with a reference repo that has one portable `accept` candidate: AC doc created under `docs/` with correct AC structure, correct naming, and `PENDING` status
- [Automated] Enhance with a reference repo that has multiple actionable candidates: single AC doc created for the top candidate, deferred candidates listed in the doc
- [Automated] Enhance with dry-run flag: no file created, stdout shows what would have been created
- [Automated] AC doc numbering: correctly determines next available number by scanning existing `ac-*.md` files
- [Manual] Run enhance against a real governed repo and verify the AC doc is readable and actionable

## Documentation Updates

- `README.md`: update enhance description (line 103)
- `docs/bootstrap-model.md`: update enhance output description (lines 292, 297)
- `docs/README.md`: remove `enhance-report.md` entry, add note about AC docs from enhance
- Remove `docs/enhance-report.md`

## Status

COMPLETE
