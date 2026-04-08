# Development Cycle

This repo uses an acceptance-criteria-first workflow.

## Required Artifacts

- `AGENTS.md`
- `README.md`
- `arch.md`
- `plan.md`
- `docs/`

## Cycle

1. choose the next approved item from `plan.md`
2. draft an acceptance-criteria doc from `docs/ac-template.md`
3. review and tighten scope before implementation
4. implement code, tests, and direct doc updates together
5. when the AC is complete and its decisions are captured in durable docs or code, remove the AC file in the same change
6. run the build and validation flow from `docs/build-release.md`
7. perform release work only when explicitly requested

## Notes

- keep roadmap decisions in `plan.md`
- keep architecture changes in `arch.md`
- keep repo-level governance in `AGENTS.md`
