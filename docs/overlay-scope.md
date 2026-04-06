# Overlay Scope

This document defines what belongs only in the `CODE` overlay versus only in the `DOC` overlay.
Anything that applies to both belongs in [`base/AGENTS.md`](../base/AGENTS.md), not in an overlay.

## Base Only

Keep only cross-repo governance here:

- interaction mode
- approval boundaries
- review style
- file-change discipline
- release or publish trigger rules
- doc-update expectations
- governed-edit rules for `AGENTS.md`

## CODE Only

These rules and files are code-repo specific and should not appear in the base contract:

- `README.md` with setup, run, and developer workflow
- `arch.md` for system design, components, and architecture decisions
- `plan.md` for roadmap, prioritization, and decision gates
- acceptance-criteria workflow and any AC document conventions
- build, test, lint, typecheck, format, migration, and release rules
- dependency-management rules
- CI expectations tied to software validation
- implementation-test expectations for logic changes
- runtime, packaging, or deployment instructions
- changelog/versioning semantics for shipped software

## DOC Only

These rules and files are documentation-repo specific and should not appear in the base contract:

- simpler `README.md` focused on purpose, audience, and contribution path
- `style.md` or `voice.md` for editorial standards
- `content-plan.md`, `calendar.md`, or equivalent publishing plan
- publishing cadence and update workflow
- review rules for editorial quality, accuracy, consistency, and source handling
- content inventory, taxonomy, and information-architecture notes
- platform-specific publishing instructions for the target CMS, site, or channel

## Boundary Rules

- If a rule mentions build tools, tests, packages, migrations, binaries, deployments, or shipped software behavior, it belongs in `CODE`.
- If a rule mentions voice, tone, style guide, editorial calendar, publishing cadence, or channel-specific content workflow, it belongs in `DOC`.
- If a rule is about how the agent should operate in any repo, it belongs in the base contract.
- If a rule is only true for one project, keep it in that generated repo's concrete files, not in the template base.
