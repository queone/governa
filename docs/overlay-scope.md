# Overlay Scope

This document defines what belongs only in the `CODE` overlay versus only in the `DOC` overlay.
Anything that applies to both belongs in [`base/AGENTS.md`](../internal/templates/base/AGENTS.md), not in an overlay.

## Base Only

Keep only cross-repo governance here:

- interaction mode
- approval boundaries
- review style
- file-change discipline
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

Governance + planning + release tooling for documentation repos. Editorial structure (voice guides, style guides, publishing workflows) is the repo owner's domain.

- `plan.md` for content direction, editorial goals, and ideas to explore
- release tooling (`rel.sh`, `cmd/rel/main.go`, `internal/reltool/`)
- review rules for editorial quality, accuracy, consistency, and source handling

## Boundary Rules

- If a rule mentions build tools, tests, packages, migrations, binaries, deployments, or shipped software behavior, it belongs in `CODE`.
- If a rule mentions content planning, editorial review standards, or doc-specific release tooling, it belongs in `DOC`.
- If a rule is about how the agent should operate in any repo, it belongs in the base contract.
- If a rule is only true for one project, keep it in that generated repo's concrete files, not in the template base.
