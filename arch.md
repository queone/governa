# governa Architecture

## Purpose

Provide a self-contained template repo for governed `CODE` and `DOC` repositories, plus a deterministic bootstrap tool (`governa apply`) that renders the template into target repos.

## System Summary

This repo has three main responsibilities:

- define the base governance contract in `internal/templates/base/`
- define repo-type overlays in `internal/templates/overlays/code/` and `internal/templates/overlays/doc/`
- provide the Go CLI and shared logic in `cmd/` and `internal/`, plus shell-based build and release tooling

The repo also serves as its own `CODE`-repo example by carrying its own `AGENTS.md`, `plan.md`, `arch.md`, `build.sh`, and supporting docs at the root.

## Current Platform

- Go CLI tooling
- Markdown templates and rendered governance docs

## Major Components

- `internal/templates/base/`: cross-repo governance artifacts such as `AGENTS.md`
- `internal/templates/overlays/`: concrete repo-type overlays for `CODE` and `DOC`
- `cmd/governa`: installable CLI binary. One command: `apply`.
- `build.sh`: self-contained Bash script for local validation (`./build.sh`), release staging (`./build.sh prep …`), and release orchestration (`./build.sh vX.Y.Z "…"`)
- `internal/`: shared logic for governance, colorized CLI output, and template access
- `governa render-canon`: on-demand command that renders flavor-specific canon files into a target directory; canon-only (no adoption record). Drives drift-scan adoption and CODE/DOC build-validation harnesses.

## Data And Control Flow

A user runs `governa apply` from inside a target repo or empty directory. Governa detects whether this is a new or existing repo, prompts for any missing parameters, and renders base plus overlay files into concrete output. All files are written directly — after apply, the consumer repo owns everything and evolves independently. Apply is fully stateless: no network call, no bookkeeping directory, no persistent metadata beyond the rendered files themselves.

Template improvements flow in the opposite direction through an out-of-band workflow documented in `governa/governance-model.md`: the Operator reviewing the governa repo reads consumer repos' governance files and AC history directly, then proposes template changes through the normal AC workflow. There is no CLI subcommand for this.

## Architecture Notes

- generated repos must remain self-contained and must not depend on this repo at runtime
- this repo treats itself as a governed `CODE` repo, but does not re-bootstrap itself through `apply`
- `build.sh` is the canonical build/release tool; implementation lives in shell, not Go
- `governa/roles.md` defines the two-role model (Operator, Director) that supplements the shared governance contract
- apply is stateless: no `.governa/` directory and no manifest in consumer repos. Provenance is recorded in `governa/ac1-governa-apply.md`.
- retain only `governa-color` as an external Go dependency (verified via `go.mod`)
- templates use `{{PLACEHOLDER}}` substitution, not a templating engine (text/template intentionally not used)
- overlays are additive; they must not conflict with the base governance contract

## Conventions

- update this document when architecture or major workflow changes materially
- keep repo-shaping decisions here and transient implementation detail in code
