# governa
Template repo that bootstraps and adopts governed repositories, and maintains itself through enhance mode. Built from:

- a common base contract in `internal/templates/base/`
- a repo-type overlay in `internal/templates/overlays/code/` or `internal/templates/overlays/doc/`
- a deterministic Go CLI that renders and reviews governed repo structure

## Why
AI-assisted repository work often breaks down not because of model limitations, but because the collaboration contract is implicit, inconsistent, and hard to reproduce. **governa** makes this contract explicit by defining a governance and workflow framework for deterministic human–AI collaboration. It provides a stable, versioned structure for proposing, reviewing, documenting, and maintaining work, ensuring both human and agent follow the same transparent rules instead of transient session context. The goal is not more process, but less coordination drift, reduced prompt-bound state, and more repeatable project outcomes.

## Install

```bash
go install github.com/kquo/governa/cmd/governa@latest
```

## Modes

### `sync`
Consumer mode, run from a target repo or empty directory. Governa is read-only source — templates are embedded in the binary.

`sync` detects whether the target is a new or existing repo and prompts interactively for any missing parameters. All flags still work for fully non-interactive use.

**New repo** (empty directory):

```bash
governa sync
```

Or with flags to skip prompts:

```bash
governa sync -y CODE -n my-service -p "API gateway for internal services" -s "Go CLI"
```

**Existing repo** (governance artifacts or manifest found): applies governance with conservative behavior — fit assessment, content-aware collision scoring, and a single `governa-adopt-review.md` at the repo root. New files are written directly; collisions are scored as `keep`, `review: cherry-pick`, or `review: no action likely`.

```bash
governa sync
```

Repo name, purpose, type, and stack are inferred from the target directory (directory basename, `README.md` first paragraph, manifest files). Explicit flags override inference: `-n`, `-p`, `-y`, `-s`. On re-sync, stored parameters from the `.governa-manifest` are reused automatically.

### `enhance`
Template-maintenance mode, run from inside this repo. The only mode that runs from governa itself and the only mode that can propose changes back into the template. Its purpose is to improve the entire templating set — base governance contract, overlays, and workflow patterns — that ships into all generated repos, as well as governa's own self-hosted governance.

With `-r`, enhance inspects another governed repo for portable improvements: patterns that every governed repo should benefit from, not project-specific local choices. It compares at the constraint level for governance sections and per-section for structured markdown files. When a `.governa-manifest` exists in the reference repo, enhance uses three-way comparison to distinguish user customizations from stale template content. With `--apply`, it writes `.template-proposed` files for assisted merge. No template files are overwritten automatically.

```bash
governa enhance \
  -r <reference-root> \
  -d
```

Without `-r`, enhance performs a self-review — comparing on-disk templates against the embedded versions to show what has changed since the last release. This is a pre-release audit tool.

```bash
governa enhance
```

Run `governa help` for all commands, or `governa <command> --help` for command-specific flags.

## Design
The target repo stays self-contained. The template repo is read-only at bootstrap time and is not imported as a submodule, package, or runtime dependency. The bootstrap tool is Go-based so the template works across macOS, Linux, and Windows without requiring a specific shell.

## Self-Hosting Status
This repo is itself governed as a `CODE` repo and carries the core artifacts at the root:

- [`AGENTS.md`](AGENTS.md)
- [`arch.md`](arch.md)
- [`plan.md`](plan.md)
- [`CHANGELOG.md`](CHANGELOG.md)
- [`docs/README.md`](docs/README.md)
- [`docs/agent-roles/`](docs/agent-roles/)

## Rendered Examples
Generated examples:

- [`examples/code/`](examples/code/)
- [`examples/doc/`](examples/doc/)

See [`docs/bootstrap-model.md`](docs/bootstrap-model.md).
