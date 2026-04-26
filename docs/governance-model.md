# Governance Model

governa ships one workflow:

- `sync` — bootstrap a new repo, or update governance in an existing repo (auto-detected)

Template improvements happen out-of-band through the normal AC workflow; there is no CLI subcommand for reviewing consumer repos. See `docs/roles/dev.md` § Template Improvement.

## Core Principle

The agent runs in the target repo, not in this template repo. This template repo is a read-only source of concrete files and rules. Generated repos copy rendered files and do not inherit from this repo at runtime.

## Target-Repo Flow

1. open the target directory (new or existing)
2. run `governa sync`
3. governa detects whether this is a new repo, a first-time sync, or a re-sync
4. prompts for any missing parameters (or uses flags/manifest/inference)
5. renders concrete files into the target repo; for any file that already exists and would differ from the template, records the collision in `.governa/sync-review.md` (with diff preview) instead of overwriting — `--yes` is the escape hatch for batch-overwrite

## Mode: `sync`

Single entry point for both new and existing repos. Detection order:

1. `.governa/manifest` (or pre-AC55 `.governa-manifest`, or pre-governa `.repokit-manifest`) found → **re-sync** (existing repo with stored params)
2. Governance artifacts found (AGENTS.md, CLAUDE.md, docs/roles/) → **first sync** (existing repo, no stored params)
3. Otherwise → **new repo** bootstrap

### New-repo behavior

- prompt for missing parameters interactively (repo type, name, stack)
- all flags (`-n`, `-y`, `-s`) bypass individual prompts
- copy base files, apply the selected overlay, fill placeholders
- create `CLAUDE.md → AGENTS.md` symlink
- write `TEMPLATE_VERSION`
- optionally initialize git if the target is not already a repo

### Existing-repo behavior

- inspect current files before writing anything
- resolve metadata via priority: (1) explicit flag, (2) stored manifest params, (3) inference from target directory, (4) interactive prompt
- create missing governed files from the template (no collision, written automatically)
- for each file that already exists and would differ from the template, **do not touch the file** — record the collision in `.governa/sync-review.md` with a diff preview
- `--yes` is the escape hatch: batch-overwrite every colliding file directly, skip the review-doc workflow
- always write `TEMPLATE_VERSION` and `.governa/manifest` (bookkeeping bypasses the collision path)
- create `CLAUDE.md → AGENTS.md` symlink if missing

The review-doc workflow: DEV runs sync, shares the summary + `.governa/sync-review.md` with the Director, Director routes to QA, the three iterate until decisions are clear, DEV drafts an AC implementing the adopts (manual edits against the review, or re-run `governa sync --yes` after AC ships). Before writing files, sync prints an assessment of how well the template fits the target repo.

## Implementation Constraints

The bootstrap implementation is written in Go, not shell.

- cross-platform behavior matters
- Windows support should not depend on a specific shell environment
- requiring Go is acceptable for the target audience
- argument parsing, file operations, and dry-run reporting are easier to keep deterministic in Go

The canonical entrypoint is `cmd/governa/main.go`, installed via `go install github.com/queone/governa/cmd/governa@latest`.

## CLI Convention

Flag-shape convention lives in `AGENTS.md` `Base Rules` — see the rule on short/long flag forms. Applies repo-wide to every tool governa ships.

Flag mapping (mode is determined by the `sync` subcommand):

```text
-t, --target
-y, --type
-n, --repo-name
-s, --stack
-g, --init-git
    --yes
```

## Agent Agnosticism

Governa is agent-agnostic. `AGENTS.md` is the canonical governance contract. Agent-specific entrypoints (`CLAUDE.md` for Claude Code, future names like `CURSOR.md` or `COPILOT.md` as they emerge) **must** be symlinks to `AGENTS.md` so every agent loads exactly the same rules.

This invariant is enforced during sync. When sync plans a `CLAUDE.md → AGENTS.md` symlink and finds an existing regular file at `CLAUDE.md`, it does not overwrite the file (preserving operator content) but reports the conflict on stderr and returns a non-zero exit code so scripted callers can detect the unresolved state. The same rule applies to any planned symlink-to-AGENTS.md op that collides with a regular file.

### Safe migration sequence

Removing the existing `CLAUDE.md` without inspection risks discarding the only copy of repo-specific governance. The operator-facing workflow is:

1. diff the existing `CLAUDE.md` against the newly written `AGENTS.md`
2. migrate any unique repo-specific rules into `AGENTS.md` (use the governance section structure)
3. delete the existing `CLAUDE.md` and re-run `governa sync` to create the symlink

## Ownership Model

- fully template-owned when created by bootstrap:
  - `AGENTS.md`
  - `CLAUDE.md` symlink
  - `TEMPLATE_VERSION` — records the last governa template version this repo was synced against; updated automatically by `governa sync` on every run
  - `.governa/manifest`
- overlay-owned by default when newly created:
  - `README.md` (CODE only)
  - `arch.md` (CODE only)
  - `plan.md`
  - `docs/roles/` (DEV, QA, director reference, and custom role docs)
- user-owned unless explicitly mapped:
  - source code
  - app content
  - business docs
  - CI config

For sync on existing repos, template-owned sections should be narrow and explicit. `AGENTS.md` is the clearest case: it is treated like a governed config file with named sections. Sync is section-aware for `AGENTS.md`: template-managed sections are compared individually (collisions recorded per-section), while `## Project Rules` is consumer-owned and preserved verbatim across re-syncs.

## Bootstrap Manifest

During `sync`, governa writes a `.governa/manifest` file into the generated repo recording:

- the template version used at bootstrap time (updated on every sync)
- adopt parameters (repo name, type, and stack) so subsequent sync runs can reuse them without flags

The manifest is intentionally minimal — no per-file checksums, no acknowledged-drift ledger, no source-sha tracking. Pre-AC78 manifests carrying those fields parse cleanly; the data is ignored and dropped when the manifest is rewritten on the next sync.
