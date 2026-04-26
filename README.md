# governa
Template repo that bootstraps governance into new repositories and helps existing ones adopt it with minimal disruption. Built from:

- a common base contract in `internal/templates/base/`
- a repo-type overlay in `internal/templates/overlays/code/` or `internal/templates/overlays/doc/`
- a deterministic Go CLI that renders templates into target repos

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
The one mode. Run from a target repo or empty directory. Governa is read-only source — templates are embedded in the binary.

`sync` detects whether the target is a new or existing repo and prompts interactively for any missing parameters. All flags still work for fully non-interactive use.

**New repo** (empty directory):

```bash
governa sync
```

Or with flags to skip prompts:

```bash
governa sync -k CODE -n my-service -s "Go"
```

**Existing repo** (governance artifacts or manifest found): non-colliding files (new to the target, or identical to the template) are written automatically along with bookkeeping (`TEMPLATE_VERSION`, `.governa/manifest`). Any file whose existing content differs from the template is **not touched** — the collision is recorded in `.governa/sync-review.md` with a diff preview for DEV, QA, and the Director to review. DEV then drafts an AC against the review and either edits adopts manually or re-runs `governa sync --yes` (the escape hatch) to batch-overwrite every collision.

```bash
governa sync
```

Repo name, type, and stack are inferred from the target directory (directory basename, manifest files). Explicit flags override inference: `-n`, `-k`, `-s`. On re-sync, stored parameters from the `.governa/manifest` are reused automatically.

Run `governa help` for available commands, or `governa sync --help` for sync-specific flags.

### Template improvements
Template improvements happen out-of-band. DEV/QA agents working on the governa repo read consumer repos (their `AGENTS.md`, recent AC docs, and `.governa/manifest`) to identify portable improvements, then propose changes as regular template PRs through the normal AC workflow. There is no CLI subcommand for this — filesystem access plus the ordinary editor workflow is enough. See `docs/roles/dev.md` for the DEV-side shape.

## Design
The target repo stays self-contained. The template repo is read-only at bootstrap time and is not imported as a submodule, package, or runtime dependency. The bootstrap tool is Go-based so the template works across macOS, Linux, and Windows without requiring a specific shell.

## Current Stage

governa is early. Releases, commits, and pushes are driven by the human director; there's no branch or PR workflow yet. These are phase choices while the governance contract stabilizes — branch workflows and release automation layer on later, without changing the primitives.

Scope is also deliberately narrow. governa aims to be a small, stable collaboration contract — not a full-stack generator, not an opinionated starter kit, not an attempt to be another [gstack](https://github.com/garrytan/gstack). The fewer primitives governa ships, the less there is to drift against.

The primary validation surface so far has been CLI-type coding agents — [Claude Code](https://github.com/anthropics/claude-code) and [Codex CLI](https://github.com/openai/codex). The contract is file-based and agent-agnostic in principle — desktop clients and IDE-integrated agents can read the same files — but their session and context-loading models differ, so expect rougher edges there until the patterns are exercised.

## Self-Hosting Status
This repo is itself governed as a `CODE` repo and carries the core artifacts at the root:

- [`AGENTS.md`](AGENTS.md)
- [`arch.md`](arch.md)
- [`plan.md`](plan.md)
- [`CHANGELOG.md`](CHANGELOG.md)
- [`docs/README.md`](docs/README.md)
- [`docs/roles/`](docs/roles/)

## Rendered Examples

Run `governa examples` to render both CODE and DOC overlays to `/tmp/governa-examples/` for inspection or testing.

See [`docs/governance-model.md`](docs/governance-model.md).
