# AC-009 Maintainer Agent Role

## Objective Fit

1. Smaller repos need a single agent that implements, tests, and reviews its own changes
2. Combining DEV and QA behaviors without an explicit contract creates rule conflicts (prefix, mindset, workflow)
3. A dedicated `maintainer.md` resolves this by defining a merged role with clear rules
4. Direct follow-on from AC-008

## Summary

Add `docs/agent-roles/maintainer.md` as a third role for repos where one agent handles both implementation and review. The role combines DEV's test-and-build discipline with QA's verification-against-docs requirement, using a single consistent prefix and an explicit self-review step before presenting work as complete.

## In Scope

- Create `docs/agent-roles/maintainer.md` for this repo (self-hosted)
- Create CODE overlay template `docs/agent-roles/maintainer.md.tmpl`
- Regenerate rendered example `examples/code/docs/agent-roles/maintainer.md`
- Update `docs/agent-roles/README.md` (self-hosted, overlay template, and rendered example) to list the new role
- Update `overlays/code/README.md` to list the new template file
- Add automated test verifying bootstrap `new` mode produces `maintainer.md`

## Out Of Scope

- Changes to `base/AGENTS.md` (role bootstrap rule already handles arbitrary role names)
- Changes to DEV or QA role docs
- DOC overlay (deferred per AC-008)

## Implementation Notes

- `maintainer.md` seed rules:
  - Start every response with "MAINT says:"
  - Write test coverage for every code change
  - Use the repo's canonical build command for all validation
  - Follow the documented pre-release checklist exactly and in order
  - Never run the release command; present it for the user to run
  - Propagate fixes to overlay templates and rendered examples in the same change
  - Before presenting work as complete, perform explicit self-review: verify behavior against documented contracts (`AGENTS.md`, `docs/build-release.md`, AC docs) and report the result — either concrete findings ordered by severity with file references, or an explicit "no findings" statement noting any residual risk or verification gap
  - When an AC document exists for the current work, follow its scope and update its status when complete
- The role does not inherit from DEV or QA — it is a standalone doc that combines the relevant rules from both
- Naming: `maintainer.md`, not `generalist.md` — describes responsibility for the whole repo
- "MAINT says:" is the output prefix only; role assignment uses the full name ("act as MAINTAINER", "use docs/agent-roles/maintainer.md"). "MAINT" alone is not a valid role selector — the lookup maps user input to `maintainer.md`, not `maint.md`

## Acceptance Tests

- [Automated] Bootstrap `new` mode for CODE produces `docs/agent-roles/maintainer.md`
- [Automated] Bootstrap `adopt` mode proposes `docs/agent-roles/maintainer.md` and updated `README.md` via existing overlay machinery
- [Manual] Explicit assignment ("act as MAINTAINER") loads the maintainer role doc
- [Manual] After assignment, agent includes test coverage for code changes AND reports self-review result (findings or explicit "no findings" with residual risk) before closing work
- [Manual] Generated `maintainer.md` explicitly requires both implementation tests and self-review with defined output format
- [Manual] `docs/agent-roles/README.md` lists maintainer alongside DEV and QA

## Documentation Updates

- `docs/agent-roles/README.md` — add maintainer to available roles table (self-hosted, overlay, example)
- `overlays/code/README.md` — add `docs/agent-roles/maintainer.md` to template file list

## Status

COMPLETE
