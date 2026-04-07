# repokit Plan

## Project Direction

Provide a narrow, usable governance template that can bootstrap new repos, adopt existing repos safely, and improve itself through a controlled enhancement path.

## Current Platform

- Go CLI tooling
- Markdown governance templates

## Objective-Fit Rubric

Every new roadmap item should answer:

1. what user or system outcome does this serve
2. why is this a better next step than competing work
3. what existing decisions or constraints does it depend on
4. is this direct roadmap work or an intentional pivot

## Priorities

- ~~`R1` Release readiness: keep this repo passing as a governed `CODE` repo and prepare for `v0.1.0`~~ (done -- v0.1.0 shipped 2026-04-06)
- ~~`R2` Operator guidance: tighten the root docs so maintainers can run `new`, `adopt`, and `enhance` without re-explaining the model~~ (done -- AC-002 complete, v0.1.2)
  - AC-002 complete: Quick Start, refreshed milestone, corrected enhance wording, cleaned up stale docs
- `R3` Safe refresh path: keep improving owned-section refresh behavior without letting `AGENTS.md` or overlay files drift
  - AC-001 complete (v0.1.1): enhance mode now produces AC docs instead of standing report; deterministic candidate ranking

## Improvement Intake

- capture follow-on improvements here before implementation
- keep the list prioritized; do not let this become an unstructured backlog
- move an item into an AC doc under `docs/` before implementation when the scope needs explicit acceptance criteria

### Overlay improvements

- CODE overlay: deeper release and upgrade guidance
- CODE overlay: richer example acceptance-criteria docs
- DOC overlay: richer platform-specific publishing examples
- DOC overlay: optional alternate `voice.md` or `calendar.md` variants

## Notes

- use `docs/` for acceptance criteria, critiques, and supporting implementation contracts
