# example-doc-repo

Example governed documentation repository rendered from the template.

## Why

State why this repo exists — not just what it does, but why it matters and what problem it solves.

## Overview

This is a governed `DOC` repo bootstrapped from `governa`.

Publishing platform:

- Static site generator

Editorial style reference:

- Clear, factual, concise

## Working Agreement

- use `AGENTS.md` as the repo governance contract
- keep `style.md` or `voice.md` current when editorial standards change
- keep `content-plan.md` or `calendar.md` current when publishing priorities or cadence change
- keep `publishing-workflow.md` current when review or publication steps change

## Core Repo Files

- `AGENTS.md`: base governance contract
- `style.md`: editorial style and formatting rules (alternate: `voice.md` for persona and audience focus)
- `content-plan.md`: prioritized content roadmap (alternate: `calendar.md` for date-driven scheduling)
- `publishing-workflow.md`: review and publishing steps
- `docs/agent-roles/`: role-specific behavior docs (DEV, QA, Maintainer)
- `cmd/rel/main.go`: Go release helper for documented release tagging
- `rel.sh`: shell convenience wrapper for Unix, Linux, and Git-Bash environments

## Workflow Summary

1. choose the next content priority from `content-plan.md` or `calendar.md`
2. draft or revise content to match `style.md` or `voice.md`
3. review for clarity, accuracy, and consistency
4. publish only through the documented workflow
5. update planning docs when priorities or cadence change

## Replace Me

Replace this starter content with project-specific contribution, review, and publishing instructions.
