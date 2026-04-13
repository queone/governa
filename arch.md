# governa Architecture

## Purpose

Provide a self-contained template repo for governed `CODE` and `DOC` repositories, plus deterministic tooling to sync and enhance that governance model.

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
- `cmd/governa`: installable CLI binary for `sync`, `enhance`, and self-review
- `cmd/build` and `cmd/rel`: Go entrypoints for local validation and release
- `internal/`: shared logic for bootstrap, build, release, colorized CLI output, and template access
- `examples/`: rendered sample repos used to verify that output is concrete and usable

## Data And Control Flow

For `sync`, a user runs `governa sync` from inside a target repo. Governa detects whether this is a new or existing repo, prompts for any missing parameters, and renders base plus overlay files into concrete output.

For `enhance`, a maintainer runs from inside this repo, points at another governed repo, and reviews governed sections and mapped overlay artifacts. Governance sections are compared at the constraint level (not just keyword signals), and structured markdown files are diffed per-section. When a `.governa-manifest` exists in the reference repo, enhance performs three-way comparison to distinguish user customizations from stale template content. Classification uses a data-driven rule table. If actionable improvements are found, enhance creates an AC doc under `docs/` for the highest-priority candidate. No template files are overwritten automatically.

## Architecture Notes

- generated repos must remain self-contained and must not depend on this repo at runtime
- this repo treats itself as a governed `CODE` repo, but does not re-bootstrap itself through `sync`
- `enhance` is report-first and intentionally conservative
- shell wrappers are conveniences only; the canonical implementation lives in Go
- `docs/roles/` provides role-specific behavior docs (director reference, DEV, QA, maintainer) that supplement the shared governance contract; role selection is instruction-driven and defined in `Interaction Mode`

## Conventions

- update this document when architecture or major workflow changes materially
- keep repo-shaping decisions here and transient implementation detail in code
