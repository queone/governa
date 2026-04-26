# Governance Model

governa ships one command:

- `apply` — bootstrap governance into a new or existing repo (one-time)

After apply, all files are consumer-owned. Future governa improvements are adopted by having a coding agent in the consumer repo read governa's source, then cherry-pick what's useful. There is no re-sync mechanism.

Template improvements to the governa repo itself happen out-of-band through the normal AC workflow; there is no CLI subcommand for reviewing consumer repos. See `docs/roles/dev.md` § Template Improvement.

## Core Principle

The agent runs in the target repo, not in this template repo. This template repo is a read-only source of concrete files and rules. Generated repos copy rendered files and do not inherit from this repo at runtime.

## Target-Repo Flow

1. open the target directory (new or existing)
2. run `governa apply`
3. governa detects whether this is a new repo or an existing one (with or without a manifest)
4. prompts for any missing parameters (or uses flags/manifest/inference)
5. renders all files directly into the target repo — no collision negotiation
6. writes `docs/ac1-governa-apply.md` as an adoption record

## Command: `apply`

Single entry point for both new and existing repos. Detection order:

1. `.governa/manifest` (or pre-AC55 `.governa-manifest`, or pre-governa `.repokit-manifest`) found → **re-apply** (existing repo with stored params)
2. Governance artifacts found (AGENTS.md, CLAUDE.md, docs/roles/) → **existing** (no stored params)
3. Otherwise → **new repo** bootstrap

### New-repo behavior

- prompt for missing parameters interactively (repo type, name, stack)
- all flags (`-n`, `-k`, `-s`) bypass individual prompts
- copy base files, apply the selected overlay, fill placeholders
- create `CLAUDE.md → AGENTS.md` symlink
- write `TEMPLATE_VERSION`
- write `docs/ac1-governa-apply.md` (adoption record)
- optionally initialize git if the target is not already a repo

### Existing-repo behavior

- inspect current files before writing anything
- resolve metadata via priority: (1) explicit flag, (2) stored manifest params, (3) inference from target directory, (4) interactive prompt
- all template files are written directly
- always write `TEMPLATE_VERSION` and `.governa/manifest`
- symlinks: if a regular file blocks a planned symlink, warn on stderr and skip; otherwise create if missing
- write `docs/ac1-governa-apply.md` (adoption record)

## Implementation Constraints

The bootstrap implementation is written in Go, not shell.

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

After apply, all files are consumer-owned. The consumer repo can freely modify any file governa produced. Bookkeeping files record provenance but impose no constraints:

- `TEMPLATE_VERSION` — records the governa template version used at apply time
- `.governa/manifest` — records apply parameters (repo name, type, stack) for re-apply inference

## Bootstrap Manifest

During `apply`, governa writes a `.governa/manifest` file into the generated repo recording:

- the template version used at apply time
- apply parameters (repo name, type, and stack) so subsequent apply runs can reuse them without flags

The manifest is intentionally minimal — no per-file checksums, no acknowledged-drift ledger, no source-sha tracking. Pre-AC78 manifests carrying those fields parse cleanly; the data is ignored and dropped when the manifest is rewritten on the next apply.
