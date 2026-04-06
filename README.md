# repo-governance-template

Template repo for generating governed repositories from:

- a common base contract in `base/`
- a repo-type overlay in `overlays/code/` or `overlays/doc/`
- a deterministic Go bootstrap command that renders concrete files into a target repo

Current milestone:

- base `AGENTS.md` contract
- `CODE` versus `DOC` overlay boundaries
- first working `bootstrap` command with `new`, `adopt`, and `enhance` modes
- self-hosted root governance artifacts so this repo can operate as a governed `CODE` repo

## Intended Use

This repo is meant to be used as a reference frame from inside a target working directory.

Two target-repo entry modes are supported:

- `new`: bootstrap an empty folder or near-empty repo into a governed `CODE` or `DOC` repo
- `adopt`: apply the methodology to an existing repo without making it depend on this template at runtime

In both cases, the user starts their coding agent in the target directory and points the agent at the absolute path of this template repo.

For template maintainers, a third maintenance mode is also needed:

- `enhance`: inspect another governed repo that may contain methodology improvements and write a deterministic review artifact before any template changes are considered

## Operating Model

The target repo stays self-contained.
The template repo is read only at bootstrap time and is not imported as a submodule, package, or runtime dependency.

The bootstrap implementation should be Go-based so the template remains usable across macOS, Linux, and Windows without requiring a specific shell.

The intended flow is:

1. user opens a coding agent in the target directory
2. user gives the agent the absolute path to this template repo
3. agent runs the bootstrap command from this template repo, targeting the current repo
4. agent inspects the target repo state
5. agent chooses bootstrap mode: `new` or `adopt`
6. agent gathers required inputs
7. agent writes concrete files into the target repo
8. generated repo records its template marker and becomes independently managed

## Operator Guide

Use `new` when the target directory is empty or nearly empty and you want a full rendered baseline.

Use `adopt` when the target repo already exists and you want conservative application behavior, fit assessment, and proposal files instead of broad overwrites.

Use `enhance` only from inside this template repo when you want to inspect another governed repo for portable improvements and generate a review artifact before making template changes.

## Self-Hosting Status

This repo now carries the core `CODE`-repo artifacts at the root:

- [`AGENTS.md`](AGENTS.md)
- [`arch.md`](arch.md)
- [`plan.md`](plan.md)
- [`CHANGELOG.md`](CHANGELOG.md)
- [`docs/README.md`](docs/README.md)

That keeps the template repo itself governed as a `CODE` repo while still using `enhance` rather than `new` or `adopt` for self-maintenance.

## Current Command

Run the bootstrap tool with Go:

```bash
go run <template-root>/cmd/bootstrap --help
```

Examples:

```bash
# new CODE repo into the current directory
go run <template-root>/cmd/bootstrap \
  -m new \
  -y CODE \
  -n my-repo \
  -p "Short project purpose" \
  -s "Go CLI"

# adopt into an existing repo, defaulting target to the current directory
go run <template-root>/cmd/bootstrap \
  -m adopt \
  -n existing-repo \
  -p "Short project purpose" \
  -s "Go service" \
  -d

# review another repo for template improvements
go run <template-root>/cmd/bootstrap \
  -m enhance \
  -r <reference-root> \
  -d
```

`enhance` is report-first. It prints the review summary to stdout and writes `docs/enhance-report.md` when not running in dry-run mode.
The current review logic is section-aware for `AGENTS.md` and uses deterministic file mappings for overlay and workflow artifacts.

## Rendered Examples

This repo includes generated examples:

- [`examples/code/`](examples/code/)
- [`examples/doc/`](examples/doc/)

See [`docs/bootstrap-model.md`](docs/bootstrap-model.md).
