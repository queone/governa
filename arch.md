# governa Architecture

## Frozen

This repo is feature-frozen at `v0.160.3`; `govna` (`https://github.com/queone/govna`) is the canonical implementation going forward. The architecture below describes governa's frozen state, not an active target for further development.

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
- `internal/templates/overlays/`: concrete repo-type overlays for `CODE` and `DOC`, including first-class Go, Rust, Terraform, and Swift CODE build templates
- `internal/templates/stack-ignores/`: stack-specific `.gitignore` fragments
- `internal/templates/stack-guidelines/`: stack-specific development-guideline fragments composed above the consumer-owned boundary
- `cmd/governa`: single installable CLI binary containing the `apply`, `drift-scan`, `rm`, `deps`, and `render-canon` subcommands; there is no standalone `driftscan` binary
- `build.sh`: self-contained Bash script for local validation (`./build.sh`), release staging (`./build.sh prep …`), and release orchestration (`./build.sh vX.Y.Z "…"`)
- `internal/`: shared logic for governance, colorized CLI output, and template access
- `governa render-canon`: on-demand command that renders flavor-specific canon files into a target directory; canon-only (no adoption record). Drives drift-scan adoption and CODE/DOC build-validation harnesses.

## Data And Control Flow

A user runs `governa apply` from inside a target repo or empty directory. Governa detects whether this is a new or existing repo, prompts for any missing parameters, and renders base plus overlay files into concrete output. All files are written directly — after apply, the consumer repo owns everything and evolves independently. Apply is fully stateless: no network call, no bookkeeping directory, no persistent metadata beyond the rendered files themselves.

Post-adoption commands provide bounded maintenance paths. `governa drift-scan` compares a consumer with embedded canon and writes an adoption AC. `governa rm` writes a cleanup AC plus targeted diffs. `governa deps` performs read-only Go dependency inspection. `governa render-canon` writes canon only to its explicit target for inspection, adoption, and validation.

## AC Lifecycle Control Flow

The governed change path is `Draft → Audit → Refine → Implement → Ratify → Package`. Draft creates the AC; Audit, Refine, Implement, and Ratify are the four AC phases; Package is post-Ratify release preparation and is not a fifth phase.

Acceptance Criteria are non-runtime control artifacts for non-trivial changes. An AC carries Director intent through bounded Operator implementation and verification, then is deleted during release prep after durable decisions land elsewhere. Trivial changes may proceed directly when authorized, but they do not bypass approval, validation, or self-review rules. `AGENTS.md` is authoritative for the AC threshold and gates.

Template improvements flow in the opposite direction through an out-of-band workflow documented in `governa/governance-model.md`: the Operator reviewing the governa repo reads consumer repos' governance files and AC history directly, then proposes template changes through the normal AC workflow. There is no CLI subcommand for this.

## Architecture Notes

- generated repos must remain self-contained and must not depend on this repo at runtime
- this repo treats itself as a governed `CODE` repo, but does not re-bootstrap itself through `apply`
- `build.sh` is the canonical build/release tool; implementation lives in shell, not Go
- `build.sh` owns local validation, release prep, and interactive release orchestration
- ACs control non-trivial change flow but are not runtime architecture
- `governa/roles.md` defines the two-role model (Operator, Director) that supplements the shared governance contract
- apply is stateless: no `.governa/` directory and no manifest in consumer repos. Provenance is recorded in `governa/ac1-governa-apply.md`.
- retain only `governa-color` as an external Go dependency (verified via `go.mod`)
- templates use `{{PLACEHOLDER}}` substitution, not a templating engine (text/template intentionally not used)
- overlays are additive; they must not conflict with the base governance contract
- every template asset root must be registered in `internal/templates/templates.go` and exercised through rendered-output tests
- first-class CODE stacks emit one canonical stack-specific `build.sh`; unsupported stacks emit the generic CODE scaffold without a build script
- Rust CODE builds isolate compilation in one invocation-owned temporary Cargo target, clean it on handled exits, and install all package binaries through Cargo into its selected external home
- Rust release prep suppresses installation during pre-change validation, isolates Cargo.lock refresh, and installs binaries only after successful post-change validation
- Swift CODE canon uses a SwiftPM backend for one root package, isolates build artifacts in invocation-owned external scratch state, discovers executable products through SwiftPM JSON parsed by the Swift toolchain, and installs full or selected release products into the configured external bin directory
- Swift full and scoped builds share package-wide formatting and tests, while scoped debug builds, release builds, and installation operate on selected executable products
- Swift build, prep, and release presentation follows the canonical CODE color and plain-text policy
- native Xcode projects and Apple application bundles remain a separate future Swift backend
- stack guidance is composed immediately above `## Project Practices`, leaving the boundary and consumer-owned tail outside stack canon

## Conventions

- update this document when architecture or major workflow changes materially
- keep repo-shaping decisions here and transient implementation detail in code
