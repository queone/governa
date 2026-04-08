# AC-007 Adopt Mode Section-Level Patching

## Objective Fit

1. Adopt mode becomes more useful for repos that already have an AGENTS.md — instead of proposing a full replacement, it patches in only the missing governed sections
2. This directly addresses deferred item D1 from plan.md
3. Must preserve the review-first model — patched content is still proposed, not written directly
4. Direct roadmap work

## Summary

When adopt mode encounters an existing AGENTS.md, it currently proposes the entire template AGENTS.md as a `.template-proposed` replacement. The user must manually diff and merge. This AC changes adopt to parse both files at the section level, identify which governed sections are missing from the existing file, and propose a patched version that adds only the missing sections while preserving all existing content.

## In Scope

- New `patchGovernedSections` function that merges template governed sections into an existing AGENTS.md
- Parse existing file into preamble + `##` sections using existing `parseLevel2Sections`
- For each governed section in the template (in the template's governed-section order): if missing from existing, append it
- Missing sections are appended in the same order they appear in the template's governed section list, ensuring deterministic output
- Preserve existing preamble, existing sections (governed or not), and section order
- Never modify or remove existing content — only add missing governed sections
- If all governed sections already exist, skip the proposal entirely (no `.template-proposed` file)
- If some sections are missing, write the patched result as the `.template-proposed` file
- Modify `applyAdoptTransforms` to use section-level patching for the "base governance contract" operation
- Tests for all patching scenarios
- Update `docs/bootstrap-model.md` adopt section to describe section-level patching

## Out Of Scope

- Modifying existing governed sections (even if they differ from the template)
- Merging constraint-level differences within a section
- Patching overlay files (only AGENTS.md gets section-level treatment)
- Changing enhance mode behavior

## Implementation Notes

- The governed section list is the same one used in `reviewGovernedSections`: Purpose, Governed Sections, Interaction Mode, Approval Boundaries, Review Style, File-Change Discipline, Release Or Publish Triggers, Documentation Update Expectations
- `parseLevel2Sections` already handles the parsing; `sectionMap` provides name-based lookup
- The preamble (content before first `## `) needs to be extracted separately — `parseLevel2Sections` currently skips it
- Missing sections are appended after all existing sections in the template's governed-section order (Purpose, Governed Sections, Interaction Mode, ..., Documentation Update Expectations) — not map iteration or parse order
- The `operation` struct already has `content` — the patched content replaces the canonical template content before propose/skip transforms
- Manifest should still record the canonical (unpatched) template checksum, since the manifest represents what the template intended, not the patched result

## Acceptance Tests

- [Automated] Adopt with existing AGENTS.md missing some governed sections produces a proposal containing original content plus missing sections
- [Automated] Adopt with existing AGENTS.md that has all governed sections produces no proposal (skipped)
- [Automated] Adopt with existing AGENTS.md preserves non-governed sections
- [Automated] Adopt with existing AGENTS.md preserves preamble
- [Automated] Adopt with no existing AGENTS.md writes the full template directly (current behavior unchanged)
- [Automated] Patched proposal never modifies existing section content

## Documentation Updates

- `docs/bootstrap-model.md` — update adopt mode description to describe section-level patching for AGENTS.md

## Status

COMPLETE
