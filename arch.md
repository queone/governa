# repokit Architecture

## Purpose

Provide a self-contained template repo for governed `CODE` and `DOC` repositories, plus deterministic tooling to bootstrap, adopt, and enhance that governance model.

## System Summary

This repo has three main responsibilities:

- define the base governance contract in `base/`
- define repo-type overlays in `overlays/code/` and `overlays/doc/`
- provide Go-based maintenance tools in `cmd/` and `internal/`

The repo also serves as its own `CODE`-repo example by carrying its own `AGENTS.md`, `plan.md`, `arch.md`, `build.sh`, and supporting docs at the root.

## Current Platform

- Go CLI tooling
- Markdown templates and rendered governance docs

## Major Components

- `base/`: cross-repo governance artifacts such as `AGENTS.md`
- `overlays/`: concrete repo-type overlays for `CODE` and `DOC`
- `cmd/bootstrap`: deterministic renderer for `new`, `adopt`, and `enhance`
- `cmd/build` and `cmd/rel`: Go entrypoints for local validation and release
- `internal/`: shared logic for bootstrap, build, release, and colorized CLI output
- `examples/`: rendered sample repos used to verify that output is concrete and usable

## Data And Control Flow

For `new` and `adopt`, an agent runs from inside a target repo, points at this repo as read-only source material, and invokes `cmd/bootstrap` to assess the target, render base plus overlay files, and write concrete output.

For `enhance`, a maintainer runs from inside this repo, points at another governed repo, and reviews governed sections and mapped overlay artifacts. If actionable improvements are found, enhance creates an AC doc under `docs/` for the highest-priority candidate. No template changes are applied automatically.

## Architecture Notes

- generated repos must remain self-contained and must not depend on this repo at runtime
- this repo treats itself as a governed `CODE` repo, but does not re-bootstrap itself through `new` or `adopt`
- `enhance` is report-first and intentionally conservative
- shell wrappers are conveniences only; the canonical implementation lives in Go

## Conventions

- update this document when architecture or major workflow changes materially
- keep repo-shaping decisions here and transient implementation detail in code
