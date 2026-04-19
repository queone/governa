# Development Cycle

This repo uses an acceptance-criteria-first workflow.

## Required Artifacts

- `AGENTS.md`
- `README.md`
- `arch.md`
- `plan.md`
- `docs/`

## Cycle

1. **Choose the next approved item.** Pull from the `Priorities` section of `plan.md` (never from `Ideas To Explore`, which is pre-rubric). Governance, sync-adoption, and director-originated ACs may originate outside this section — for example, template-upgrade ACs, hotfix ACs, or director-requested refinements. Draft those directly when authorized.
2. **Draft an acceptance-criteria doc.** Start from `docs/ac-template.md` (see preamble for the monotonic-numbering rule); save as `docs/ac<N>-<slug>.md`.
3. **Review and tighten scope before implementation.** When QA files findings on the AC, DEV responds in the conversation with proposed changes or explicit disagreement, but does not edit the AC file until QA replies and the director confirms the iteration is closed. Repeat until the AC is implementation-ready. See `docs/critique-protocol.md` for the full critique-round protocol (round-append structure, terminator shape, and DEV/QA cross-references).
4. **Implement code, tests, and direct doc updates together.**
5. **Capture decisions in durable docs or code when the AC is complete.** The AC file is removed during release prep (see `docs/build-release.md` Pre-Release Checklist).
6. **Run the build and validation flow.** See `docs/build-release.md`.
7. **Perform release work only when explicitly requested.**

## Notes

- keep roadmap decisions in `plan.md`
- keep architecture changes in `arch.md`
- keep repo-level governance in `AGENTS.md`
- record loose, pre-rubric follow-on ideas in `plan.md` under `Ideas To Explore` with an `IE<N>:` prefix
- remove IE entries when promoted to an AC or completed; the list is staging, not history
- record rubric-cleared follow-on work in `plan.md` under `Priorities`
- write AC docs to file (`docs/ac<N>-<slug>.md`); summarize in the response but do not dump full AC content into conversation
- promotion path: IE entry → discussion → objective-fit rubric (see `AGENTS.md` Approval Boundaries) → `Priorities` → AC
