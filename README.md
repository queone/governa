# governa
Template repo that bootstraps governance into new repositories and helps existing ones adopt it with minimal disruption. Built from:

- a common base contract in `internal/templates/base/`
- a repo-type overlay in `internal/templates/overlays/code/` or `internal/templates/overlays/doc/`
- a deterministic Go CLI that renders templates into target repos

## Why

AI-assisted coding is here to stay. Teams that code alone, teams that work entirely with human contributors, and teams that work with a mix of humans and agents all continue to exist — often in the same repo across different phases. **governa** is not a prerequisite for any of them. If you prefer to code without agents, governa stays out of the way. What governa does is add a little order to the new paradigm: when you choose to bring a coding agent into a repo, the collaboration contract is already explicit, versioned, and reproducible — not reinvented prompt by prompt.

The contract covers what humans and agents agree on before work starts: who is authorized to make which changes, how proposals are reviewed, what governance files mean, and how the template itself evolves. File-based and deterministic; nothing depends on transient session context.

## Roles

governa ships a closed two-role model so agent sessions have a predictable starting point:

- **Operator** — LLM agent role. Owns implementation, tests, doc alignment, and mandatory self-review. Automatic and unannounced; it is the only agent role.
- **Director** — human role. Owns intent, priorities, irreversible decisions (releases, architectural bets, scope), and the meta-loop. Not assignable to an agent.

Full role definitions and the self-review contract live in [`docs/roles.md`](docs/roles.md). The shared `AGENTS.md` contract applies in every case.

## Usage

Install the binary:

```bash
go install github.com/queone/governa/cmd/governa@latest
```

### `apply`

One-time governance bootstrap. Run from a target repo or empty directory. Governa is read-only source — templates are embedded in the binary. After apply, all files are consumer-owned — modify freely to fit the repo's needs.

**New repo** (empty directory):

```bash
governa apply
```

Or with flags to skip prompts:

```bash
governa apply -k CODE -n my-service -s "Go"
```

Go is the only stack with full overlay support today. Other values are accepted but produce a generic scaffold.

**Existing repo** (governance artifacts found): all template files are written directly. Repo name, type, and stack are inferred from the target directory (directory basename, manifest files). Explicit flags override inference: `-n`, `-k`, `-s`.

```bash
governa apply
```

Run `governa help` for available commands, or `governa apply --help` for apply-specific flags.

### Self-service updates

To adopt future governa improvements, have a coding agent in the consumer repo read governa's `AGENTS.md`, role files, and `CHANGELOG.md`, then cherry-pick what's useful. There is no re-sync mechanism — improvements are pulled by the consumer, not pushed by the template.

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
- [`docs/roles.md`](docs/roles.md)

## Library Family

governa is shifting from a single one-off-applied template to a layered model: convention applied once + code distributed as separate-repo libraries (`governa-<x>`). Extractions are gated by [`docs/library-policy.md`](docs/library-policy.md). See also [`docs/advisories/`](docs/advisories/) for portable advisories surfaced from consumer repos.

- [`governa-color`](https://github.com/queone/governa-color) — ANSI terminal color helpers for CLI output.
- [`governa-reltool`](https://github.com/queone/governa-reltool) — Git tag, commit, and push orchestration for release flows.
- [`governa-buildtool`](https://github.com/queone/governa-buildtool) — Build pipeline orchestration: programVersion validation, vet/test, build/install, next-tag suggestion.

## Rendered Examples

Run `governa examples` to render both CODE and DOC overlays to `/tmp/governa-examples/` for inspection or testing.

See [`docs/governance-model.md`](docs/governance-model.md).
