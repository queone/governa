# governa Architecture

## Purpose

Provide a self-contained template repo for governed `CODE` and `DOC` repositories, plus a deterministic bootstrap tool (`governa apply`) that renders the template into target repos.

## System Summary

This repo has three main responsibilities:

- define the base governance contract in `internal/templates/base/`
- define repo-type overlays in `internal/templates/overlays/code/` and `internal/templates/overlays/doc/`
- provide Go-based maintenance tools in `cmd/` and `internal/`

The repo also serves as its own `CODE`-repo example by carrying its own `AGENTS.md`, `plan.md`, `arch.md`, `build.sh`, and supporting docs at the root.

## Current Platform

- Go CLI tooling
- Markdown templates and rendered governance docs

## Major Components

- `internal/templates/base/`: cross-repo governance artifacts such as `AGENTS.md`
- `internal/templates/overlays/`: concrete repo-type overlays for `CODE` and `DOC`
- `cmd/governa`: installable CLI binary. One command: `apply`.
- `cmd/build`, `cmd/prep`, and `cmd/rel`: Go entrypoints for local validation, release staging, and release orchestration
- `internal/`: shared logic for governance, build, release, colorized CLI output, and template access
- `governa examples`: on-demand command that renders sample repos to `/tmp/governa-examples/` for inspection and build validation

## Data And Control Flow

A user runs `governa apply` from inside a target repo or empty directory. Governa detects whether this is a new or existing repo, prompts for any missing parameters, and renders base plus overlay files into concrete output. All files are written directly — after apply, the consumer repo owns everything and evolves independently. The `.governa/manifest` records the template version and the rendering parameters; that's the only persistent bookkeeping governa writes.

Template improvements flow in the opposite direction through an out-of-band workflow documented in `docs/roles/dev.md`: DEV/QA agents reviewing the governa repo read consumer repos' governance files and AC history directly, then propose template changes as regular PRs through the normal AC workflow. There is no CLI subcommand for this.

## Architecture Notes

- generated repos must remain self-contained and must not depend on this repo at runtime
- this repo treats itself as a governed `CODE` repo, but does not re-bootstrap itself through `apply`
- shell wrappers are conveniences only; the canonical implementation lives in Go
- `docs/roles/` provides role-specific behavior docs (director reference, DEV, QA, maintainer) that supplement the shared governance contract; role selection is instruction-driven and defined in `Interaction Mode`
- governa-managed metadata in consumer repos lives at `.governa/manifest` (committed). Legacy paths (`.governa-manifest`, `.governa/proposed/`, `.governa/feedback/`, `.governa/config`) from pre-AC78 governa are auto-removed at apply start.
- pure stdlib; no external Go dependencies (verified via `go.mod`)
- templates use `{{PLACEHOLDER}}` substitution, not a templating engine (text/template intentionally not used)
- overlays are additive; they must not conflict with the base governance contract

## Conventions

- update this document when architecture or major workflow changes materially
- keep repo-shaping decisions here and transient implementation detail in code
