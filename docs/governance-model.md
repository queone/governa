# Governance Model

governa ships one command:

- `apply` — bootstrap governance into a new or existing repo (one-time)

After apply, all files are consumer-owned. Future governa improvements are adopted by having a coding agent in the consumer repo read governa's source, then cherry-pick what's useful. There is no re-sync mechanism.

Template improvements to the governa repo itself happen out-of-band through the normal AC workflow; there is no CLI subcommand for reviewing consumer repos. See § Template Improvement below.

## Core Principle

The agent runs in the target repo, not in this template repo. This template repo is a read-only source of concrete files and rules. Generated repos copy rendered files and do not inherit from this repo at runtime.

## Target-Repo Flow

1. open the target directory (new or existing)
2. run `governa apply`
3. governa detects whether this is a new repo or an existing one
4. prompts for any missing parameters (or uses flags/inference)
5. renders all files directly into the target repo — no collision negotiation
6. writes `docs/ac1-governa-apply.md` as an adoption record

## Command: `apply`

Single entry point for both new and existing repos. Detection order:

1. Governance artifacts found (AGENTS.md, CLAUDE.md) → **existing**
2. Otherwise → **new repo** bootstrap

### New-repo behavior

- prompt for missing parameters interactively (repo type, name, stack)
- all flags (`-n`, `-k`, `-s`) bypass individual prompts
- copy base files, apply the selected overlay, fill placeholders
- create `CLAUDE.md → AGENTS.md` symlink
- write `docs/ac1-governa-apply.md` (adoption record)
- optionally initialize git if the target is not already a repo

### Existing-repo behavior

- warn that existing governance files will be overwritten
- resolve metadata via priority: (1) explicit flag, (2) inference from target directory, (3) interactive prompt
- all template files are written directly
- symlinks: if a regular file blocks a planned symlink, warn on stderr and skip; otherwise create if missing
- write `docs/ac1-governa-apply.md` (adoption record)

## Implementation Constraints

The implementation is written in Go, not shell.

- cross-platform behavior matters
- Windows support should not depend on a specific shell environment
- requiring Go is acceptable for the target audience
- argument parsing, file operations, and dry-run reporting are easier to keep deterministic in Go

The canonical entrypoint is `cmd/governa/main.go`, installed via `go install github.com/queone/governa/cmd/governa@latest`.

## CLI Convention

Flag-shape convention lives in `AGENTS.md` `Project Rules` — see the rule on short/long flag forms. Applies repo-wide to every tool governa ships.

Flag mapping (mode is determined by the `apply` subcommand):

```text
-t, --target
-k, --type
-n, --repo-name
-s, --stack
-g, --init-git
```

## Agent Agnosticism

Governa is agent-agnostic. `AGENTS.md` is the canonical governance contract. Agent-specific entrypoints (`CLAUDE.md` for Claude Code, future names like `CURSOR.md` or `COPILOT.md` as they emerge) **must** be symlinks to `AGENTS.md` so every agent loads exactly the same rules.

During apply, if a regular file exists where a symlink is planned, governa warns on stderr and skips the symlink. The operator-facing migration:

1. diff the existing `CLAUDE.md` against the newly written `AGENTS.md`
2. migrate any unique repo-specific rules into `AGENTS.md` (use the governance section structure)
3. delete the existing `CLAUDE.md` and re-run `governa apply` to create the symlink

## Ownership Model

After apply, all files are consumer-owned. The consumer repo can freely modify any file governa produced. Apply is stateless — no bookkeeping directory, no persistent metadata. Provenance is recorded in `docs/ac1-governa-apply.md`.

## Template Improvement

Template improvements originate in the governa repo. The Operator proposes them by reviewing consumer repos directly — reading the consumer's AGENTS.md and recent AC docs — to identify patterns worth upstreaming. Each proposed change goes through the normal AC workflow (draft AC, critique, implement, release). There is no CLI subcommand for this; filesystem access plus the ordinary workflow is enough.
