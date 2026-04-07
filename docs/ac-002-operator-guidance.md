# AC-002 Operator guidance for new, adopt, and enhance

## Objective Fit

1. Make `README.md` and the core docs sufficient for a maintainer to run `new`, `adopt`, and `enhance` without having to restate the operating model in chat.
2. This is the right next step because v0.1.1 shipped the AC-driven enhance workflow, and the root docs still reflect pre-release aspirational language and the old report-based enhance model.
3. The current docs are structurally sound but contain stale milestone text, outdated enhance descriptions, and no quick-start path. Fixing these is a tightening pass, not a redesign.
4. Direct R2 roadmap work.

## Summary

Refresh the root-level and core operator docs so they reflect shipped reality, provide a quick-start path for all three modes, and remove stale guidance that creates confusion or re-explanation overhead.

## In Scope

1. **Refresh `README.md` milestone section** -- replace pre-v0.1.0 aspirational text with shipped state summary
2. **Add Quick Start section to `README.md`** -- one CODE repo example, one DOC repo example, one enhance example, placed near the top for fast operator access
3. **Correct enhance wording throughout** -- replace all references to "review artifact" or "report" with AC-driven output language in:
   - `README.md`
   - `scripts/README.md`
   - `docs/bootstrap-model.md`
4. **Fix `docs/bootstrap-model.md` enhance flow step ordering** -- classification should come before AC creation
5. **Remove or rewrite `scripts/README.md`** -- the directory adds no value now that the entrypoint is `cmd/bootstrap/`. Either remove the file or reduce it to a one-line pointer to `cmd/bootstrap/`
6. **Trim stale "Still to add" wishlist text** in `overlays/code/README.md` and `overlays/doc/README.md` -- move any items worth keeping to `plan.md`, remove the rest

## Out Of Scope

- Redesigning overlay documentation structure
- Rewriting `docs/bootstrap-model.md` beyond the step ordering fix and enhance wording
- Adding new template files or changing bootstrap behavior
- Changing the AC template format
- Any code changes

## Implementation Notes

- This is a docs-only change -- no Go code modifications
- Quick Start should use `<template-root>` placeholder consistently, not absolute paths
- The DOC repo example should demonstrate `-u` and `-v` flags which are currently undocumented by example
- When trimming overlay wishlists, migrate items to `plan.md` under Improvement Intake if they represent real future work

## Acceptance Tests

- [Manual] A new maintainer can run `new` for a CODE repo using only the Quick Start section
- [Manual] A new maintainer can run `new` for a DOC repo using only the Quick Start section
- [Manual] A new maintainer can run `enhance` using only the Quick Start section
- [Manual] No operator-facing doc (README, arch, bootstrap-model, overlay READMEs) still references `docs/enhance-report.md` or describes enhance as producing a "report file" (historical references in CHANGELOG and completed AC docs are acceptable)
- [Manual] `scripts/README.md` is either removed or reduced to a pointer
- [Manual] No "Still to add" wishlist text remains in overlay READMEs without a corresponding `plan.md` entry

## Documentation Updates

- `README.md`
- `docs/bootstrap-model.md`
- `scripts/README.md` (remove or rewrite)
- `overlays/code/README.md`
- `overlays/doc/README.md`
- `plan.md` (receive any migrated wishlist items)

## Status

COMPLETE
