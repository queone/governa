# example-doc-repo

Example governed documentation repository rendered from the template.

## Overview

This is a governed `DOC` repo bootstrapped from `repo-governance-template`.

Publishing platform:

- Static site generator

Editorial style reference:

- Clear, factual, concise

## Working Agreement

- use `AGENTS.md` as the repo governance contract
- keep `style.md` current when editorial standards change
- keep `content-plan.md` current when publishing priorities or cadence change
- keep `publishing-workflow.md` current when review or publication steps change

## Core Repo Files

- `AGENTS.md`: base governance contract
- `style.md`: editorial style and voice rules
- `content-plan.md`: prioritized content roadmap
- `publishing-workflow.md`: review and publishing steps
- `cmd/rel/main.go`: Go release helper for documented release tagging
- `rel.sh`: shell convenience wrapper for Unix, Linux, and Git-Bash environments

## Workflow Summary

1. choose the next content priority from `content-plan.md`
2. draft or revise content to match `style.md`
3. review for clarity, accuracy, and consistency
4. publish only through the documented workflow
5. update planning docs when priorities or cadence change

## Replace Me

Replace this starter content with project-specific contribution, review, and publishing instructions.
