# governa
Template repo that syncs governance into new and existing repositories, and maintains itself through enhance mode. Built from:

- a common base contract in `internal/templates/base/`
- a repo-type overlay in `internal/templates/overlays/code/` or `internal/templates/overlays/doc/`
- a deterministic Go CLI that renders and reviews governed repo structure

## Why

AI-assisted coding is here to stay. Teams that code alone, teams that work entirely with human contributors, and teams that work with a mix of humans and agents all continue to exist — often in the same repo across different phases. **governa** is not a prerequisite for any of them. If you prefer to code without agents, governa stays out of the way. What governa does is add a little order to the new paradigm: when you choose to bring a coding agent into a repo, the collaboration contract is already explicit, versioned, and reproducible — not reinvented prompt by prompt.

The contract covers what humans and agents agree on before work starts: who is authorized to make which changes, how proposals are reviewed, what governance files mean, and how the template itself evolves. File-based and deterministic; nothing depends on transient session context.

## Default Roles

governa ships with a small role split so agent sessions have a predictable starting point:

- **DEV** — implements approved work, writes tests alongside, keeps docs aligned.
- **QA** — reviews and red-teams DEV's work, files findings for the director.
- **Maintainer** — default if none is assigned; handles broad repo upkeep.
- **Director** — human role; owns intent, priorities, and irreversible decisions. Not assignable to an agent.

Role definitions live in [`docs/roles/`](docs/roles/). By default, sessions run as Maintainer when `docs/roles/maintainer.md` exists; explicit assignment (e.g., "act as DEV") overrides. The shared `AGENTS.md` contract applies in every case.

## Modes

First, install the binary:

```bash
go install github.com/queone/governa/cmd/governa@latest
```

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

**Existing repo** (governance artifacts or manifest found): applies governance with conservative behavior — fit assessment, content-aware collision scoring, and a single `.governa/sync-review.md` at the repo root. New files are written directly; collisions are scored as `keep` (no adoption work needed) or `adopt` (template has improvements — compare `.governa/proposed/<path>` and adopt). Scoring includes file preamble content, section rename detection, structural observation promotion, and advisory notes for template sections missing from developed files.

```bash
governa sync
```

Repo name, purpose, type, and stack are inferred from the target directory (directory basename, `README.md` first paragraph, manifest files). Explicit flags override inference: `-n`, `-p`, `-y`, `-s`. On re-sync, stored parameters from the `.governa/manifest` are reused automatically.

### `enhance`
Template-maintenance mode, run from inside this repo. The only mode that runs from governa itself and the only mode that can propose changes back into the template. Its purpose is to improve the entire templating set — base governance contract, overlays, and workflow patterns — that ships into all generated repos, as well as governa's own self-hosted governance.

With `-r`, enhance inspects another governed repo for portable improvements: patterns that every governed repo should benefit from, not project-specific local choices. It compares at the constraint level for governance sections and per-section for structured markdown files. For governance sections classified as project-specific, enhance drills into `### Subsections` to identify portable content within otherwise deferred parents. When a `.governa/manifest` exists in the reference repo, enhance uses three-way comparison to distinguish user customizations from stale template content. The only output is an AC doc — no template files are overwritten automatically.

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

## Current Stage

governa is early. Releases, commits, and pushes are driven by the human director; there's no branch or PR workflow yet. These are phase choices while the governance contract stabilizes — branch workflows and release automation layer on later, without changing the primitives.

Scope is also deliberately narrow. governa aims to be a small, stable collaboration contract — not a full-stack generator, not an opinionated starter kit, not an attempt to be another [gstack](https://github.com/garrytan/gstack). The fewer primitives governa ships, the less there is to drift against.

## Self-Hosting Status
This repo is itself governed as a `CODE` repo and carries the core artifacts at the root:

- [`AGENTS.md`](AGENTS.md)
- [`arch.md`](arch.md)
- [`plan.md`](plan.md)
- [`CHANGELOG.md`](CHANGELOG.md)
- [`docs/README.md`](docs/README.md)
- [`docs/roles/`](docs/roles/)

## Rendered Examples
Generated examples:

- [`examples/code/`](examples/code/)
- [`examples/doc/`](examples/doc/)

See [`docs/governance-model.md`](docs/governance-model.md).
