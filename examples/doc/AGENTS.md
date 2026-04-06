# AGENTS.md

## Purpose

This file is the base governance contract for a generated repo.
Keep it short, stable, and cross-repo only.
Repo-specific workflow belongs in the selected overlay, not here.

## Governed Sections

Only these sections may be edited through a guided update:

- `Purpose`
- `Governed Sections`
- `Interaction Mode`
- `Approval Boundaries`
- `Review Style`
- `File-Change Discipline`
- `Release Or Publish Triggers`
- `Documentation Update Expectations`

Do not add new sections, reorder sections, or rewrite the whole file unless the user explicitly asks for a contract change to this template itself.
Treat this file as a governed config artifact, not freeform prose.
When asked to update it, propose the exact section names to change and keep edits local to those sections.

## Interaction Mode

- Treat requests as exploratory discussion unless the user explicitly asks for implementation or file changes.
- Do not create artifacts or make changes unless the user explicitly authorizes them.
- When the user authorizes changes, make the smallest concrete change that satisfies the request.
- Surface assumptions, ambiguities, and missing context plainly before taking action that could change project direction.

## Approval Boundaries

- Do not create, delete, rename, publish, release, or perform destructive changes without explicit user approval.
- Do not change governance files, CI/release configuration, secrets handling, or external integrations without explicit user approval.
- Normal in-scope edits to existing project files are allowed once the user has asked for implementation.
- If a request is ambiguous and the change would be hard to reverse, stop and ask.

## Review Style

- Default to a review mindset when the user asks for review: prioritize bugs, regressions, missing tests, and drift from documented behavior.
- Present findings before summaries.
- Prefer concrete evidence: file paths, behavior, and missing coverage.
- If no issues are found, say so directly and note any residual risk or verification gap.

## File-Change Discipline

- Prefer targeted edits over broad rewrites.
- Preserve user changes and unrelated local modifications.
- Update only the files required for the task, plus directly affected docs.
- When follow-on improvements are discovered but are not part of the current authorized change, record them in `plan.md` or the repo's planning artifact instead of expanding scope ad hoc.
- Do not commit personal absolute filesystem paths in docs, templates, config, or generated artifacts; use repo-relative paths or clear placeholders such as `<template-root>`.
- Keep generated repos self-contained; do not introduce runtime dependence on this template repo.
- Follow existing repo conventions unless the user asks to change them.

## Release Or Publish Triggers

- Do not prepare or execute a release, publish, deploy, or distribution step unless the user explicitly asks for it.
- Bootstrap and maintain a root `CHANGELOG.md` for release-bearing repos. Keep it current as the human-readable release history.
- Do not start release-prep bookkeeping early. Only begin the repo's documented pre-release checklist when the user explicitly asks to prep for release or equivalent.
- Version bumps, changelog/release-note updates, tag prep, and publish workflows are release-scoped work, not routine edits.
- When release prep is explicitly requested, run the documented pre-release checklist, prepare the exact version and release message, and then present the canonical release command for the user to run or approve.
- When the user does trigger a release or publish flow, update the required release artifacts in the same pass.

## Documentation Update Expectations

- Keep documentation aligned with behavior in the same change that introduces the behavior.
- Update user-facing docs when commands, setup, workflows, outputs, published content structure, or operating instructions change.
- Update architecture, planning, or style docs only when the change materially affects them.
- Do not let docs silently drift from the implemented or published reality.
