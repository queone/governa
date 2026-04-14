# Development Cycle

This repo uses an acceptance-criteria-first workflow.

## Required Artifacts

- `AGENTS.md`
- `README.md`
- `arch.md`
- `plan.md`
- `docs/`

## Cycle

1. choose the next approved item from the `Priorities` section of `plan.md` (never from `Ideas To Explore`, which is pre-rubric)
2. draft an acceptance-criteria doc from `docs/ac-template.md`; save as `docs/ac<N>-<slug>.md`
3. review and tighten scope before implementation
4. implement code, tests, and direct doc updates together
5. when the AC is complete, capture its decisions in durable docs or code; the AC file is removed during release prep (see `docs/build-release.md` Pre-Release Checklist)
6. run the build and validation flow from `docs/build-release.md`
7. perform release work only when explicitly requested

## Notes

- keep roadmap decisions in `plan.md`
- keep architecture changes in `arch.md`
- keep repo-level governance in `AGENTS.md`
- record loose, pre-rubric follow-on ideas in `plan.md` under `Ideas To Explore` with an `IE<N>:` prefix
- remove IE entries when promoted to an AC or completed; the list is staging, not history
- record rubric-cleared follow-on work in `plan.md` under `Priorities`
- write AC docs to file (`docs/ac<N>-<slug>.md`); summarize in the response but do not dump full AC content into conversation
- promotion path: IE entry → discussion → objective-fit rubric (see `AGENTS.md` Approval Boundaries) → `Priorities` → AC
