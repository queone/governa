# AGENTS.md

This file is the governance contract for this repo and the only doc guaranteed to be loaded every agent session. `CLAUDE.md` is a symlink to this file. Other `docs/*.md` files are supplementary and loaded on demand.

## Purpose

{{PROJECT_PURPOSE}}

## Governed Sections

This file is loaded every agent session and must remain enforceable on its own. Detail, rationale, and examples live in extracted docs (`docs/development-guidelines.md`, `docs/build-release.md`, `docs/development-cycle.md`). Those docs are loaded on demand.

Each section must preserve its semantic intent across edits. Add new rules under the section that best fits — do not invent ad-hoc top-level sections.

Only these sections may be edited through a guided update:

- `Purpose`
- `Governed Sections`
- `Interaction Mode`
- `Approval Boundaries`
- `Review Style`
- `File-Change Discipline`
- `Release Or Publish Triggers`
- `Documentation Update Expectations`
- `Project Rules`

Do not add new sections, reorder sections, or rewrite the whole file unless the user explicitly asks for a contract change.
Treat this file as a governed config artifact, not freeform prose.
When asked to update it, propose the exact section names to change and keep edits local to those sections.

Prefer flat `##` sections with inline bullets over `###` sub-subsections in governance files. Sub-subsections add navigation overhead without adding enforceability. If a section grows large enough to need internal grouping, consider splitting it into a separate `##` section or extracting detail to a supplementary doc. `### Sub-subsections` are acceptable only when the repo has a documented technical reason (e.g., many domain-specific rules in Project Rules that benefit from grouping).

## Interaction Mode

- Treat requests as exploratory discussion unless the user explicitly asks for implementation or file changes.
- Do not create artifacts or make changes unless the user explicitly authorizes them.
- When the user authorizes changes, make the smallest concrete change that satisfies the request.
- Surface assumptions, ambiguities, and missing context plainly before taking action that could change project direction.
- If `docs/roles/` exists and the user has not explicitly assigned a role: if `docs/roles/maintainer.md` is present, default to maintainer immediately and announce the active role in the first response (e.g., "Operating as maintainer (default)."). If no maintainer role exists, ask which role to assume. Role assignment requires an explicit instruction such as "act as DEV", "use docs/roles/qa.md", or "you are QA". After assignment, read `docs/roles/<role>.md` (case-insensitive lookup) and follow it alongside this file. `AGENTS.md` defines the shared repo contract; the assigned role file defines role-specific behavior for that session. Assignment persists for the session unless the user explicitly switches. If the requested role file does not exist, say so and continue under shared governance only. `director.md` is a reference document describing the human's role — it is not an assignable agent role. If asked to operate as director, decline and ask for a valid agent role.

## Approval Boundaries

- Authorization is per-scope. A user approving a change once does not authorize future changes by analogy.
- Do not create, delete, rename, publish, release, or perform destructive changes without explicit user approval.
- Do not change governance files, CI/release configuration, secrets handling, or external integrations without explicit user approval.
- Use an AC-first workflow for non-trivial changes. Before implementation, draft an AC doc (`docs/ac<N>-<slug>.md`) that defines scope, out-of-scope, objective fit, and required tests. Use `docs/ac-template.md` as the starting point if available. Do not begin implementation until the AC is reviewed and the user authorizes it.
- **AC critique gate:** After drafting an AC, ask the user to initiate an external critique. Do not proceed to implementation until: (1) the critique has happened — its findings either saved as `docs/ac<N>-<slug>-critique.md` or integrated directly into the AC, and (2) the user explicitly confirms the AC is implementation-ready. (See `docs/ac-template.md` Companion Artifacts for the full convention, including `-feedback.md` and `-dispositions.md`.)
- Before committing to an AC, every roadmap item must answer: (1) What user or system outcome does this serve? (2) Why is this a better next step than competing work? (3) What existing decisions or constraints does it depend on? (4) Is this direct roadmap work or an intentional pivot? These questions must be answered in the AC's Objective Fit section.
- Normal in-scope edits to existing project files are allowed once the user has asked for implementation.
- Never run the release command yourself; present it for the user to run.
- If a request is ambiguous and the change would be hard to reverse, stop and ask.

## Review Style

- Keep completion and status messages terse by default.
- Default to a review mindset when the user asks for review: prioritize bugs, regressions, missing tests, and drift from documented behavior.
- Present findings before summaries.
- Prefer concrete evidence: file paths, behavior, and missing coverage.
- If no issues are found, say so directly and note any residual risk or verification gap.
- Prefer terse completions: lead with what changed, then flat bullets and a one-sentence next step. Do not add extra sections like "What's in it", "Main conclusion", or "Next steps" unless the user asks.
- Prefer plain text and simple bullets over heavy Markdown tables or ASCII art. Use richer structure only when content clearly benefits.
- Do not note skipped checks when the skip is already implied by repo rules or the review scope. Mention them only if the omission is unusual or affects confidence.
- When presenting an architectural decision to the director, surface two bounded options plus a recommendation. Avoid open menus of three or more — they read as dithering. If only one option exists, state it as a recommendation; if more than two are genuinely viable, name the two best and note the rest in one sentence.

## File-Change Discipline

- Prefer targeted edits over broad rewrites.
- Preserve user changes and unrelated local modifications.
- Update only the files required for the task, plus directly affected docs. During implementation, keep docs current — do not defer doc updates to a follow-up.
- When follow-on improvements are discovered but are not part of the current authorized change, record them in `plan.md` or the repo's planning artifact instead of expanding scope ad hoc. If neither exists, note them to the user.
- Do not commit personal absolute filesystem paths in docs, templates, config, or generated artifacts; use repo-relative paths or clear placeholders such as `<template-root>`.
- Keep generated repos self-contained; do not introduce runtime dependence on this template repo.
- When a change adds a file, command, flag, or major decision, update affected docs in the same pass.
- When a decision changes mid-implementation, complete the migration in one pass: update all affected files, docs, and tests together rather than leaving a half-migrated state.
- Follow existing repo conventions unless the user asks to change them.
- **Do not present a code change as complete without accompanying tests. No exceptions for "small" changes, CLI output, or formatting.**
- **When an agent receives a correction about repo behavior (build process, release workflow, file conventions, review expectations), codify it in the appropriate governance doc — not in agent-local memory or session state. If the correction refines an existing rule, update that rule in place. If it is new, add it to the section that best fits. Agent-local memory must not be used as a shadow governance system.**

## Release Or Publish Triggers

- Do not prepare or execute a release, publish, deploy, or distribution step unless the user explicitly asks for it.
- Bootstrap and maintain a root `CHANGELOG.md` for release-bearing repos, following the canonical table specified in `docs/build-release.md` Pre-Release Checklist CHANGELOG step. Do not invent alternative shapes.
- Do not start release-prep bookkeeping early. Only begin the Pre-Release Checklist in `docs/build-release.md` when the user explicitly asks to prep for release or equivalent.
- Version bumps, changelog/release-note updates, tag prep, and publish workflows are release-scoped work, not routine edits.
- When release prep is explicitly requested, run the documented pre-release checklist, prepare the exact version and a concrete release message derived from the actual changes, and then present only the canonical release command for the user to run or approve. Show the full git sequence only if the user explicitly asks for it.
- When the user does trigger a release or publish flow, update the required release artifacts in the same pass.

## Documentation Update Expectations

- Keep documentation aligned with behavior in the same change that introduces the behavior.
- Update user-facing docs when commands, setup, workflows, outputs, published content structure, or operating instructions change.
- Update architecture, planning, or style docs only when the change materially affects them.
- Do not let docs silently drift from the implemented or published reality.
- Every AC doc must end with a `## Status` section. Valid states: `PENDING`, `IN PROGRESS`, `DEFERRED` (with reason). For partial completion, list status by phase. Completed ACs are deleted per the development cycle — do not change status to DONE before deletion.

## Project Rules

- Always use the repo's canonical build command (`./build.sh` or equivalent) — never run individual tool commands directly. See `docs/build-release.md` for the full pipeline.
- Follow semver: PATCH for changes invisible to users (bug fixes, refactors, tooling). MINOR for user-visible changes (commands, flags, schema, behavioral). Batch PATCH-level changes when possible.
- Pin dependencies to explicit versions. Do not stay on an older version without a documented reason.
- Follow existing repo patterns unless a clear improvement is approved.
- Every new feature or logic change must include tests in the same pass.
- Wrap user-facing errors with operation context and recovery guidance.
- Every AC must label each acceptance test as `[Automated]` or `[Manual]`. See `docs/ac-template.md`.
