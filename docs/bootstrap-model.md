# Bootstrap Model

This template supports two workflows:

- `sync` — bootstrap a new repo or update governance in an existing repo (single command, auto-detected)
- `enhance` — improve this template from lessons captured in another governed repo

Both workflows use the same deterministic command surface and the same template source tree.

## Core Principle

The agent runs in the target repo, not in this template repo.
This template repo is a read-only source of concrete files and rules.
Generated repos copy rendered files and do not inherit from this repo at runtime.

Exception:

- `enhance` runs from inside this template repo because its purpose is to improve the template itself

## Target-Repo Flow

The intended user flow is:

1. open the target directory (new or existing)
2. run `governa sync`
3. governa detects whether this is a new repo, a first-time sync, or a re-sync
4. prompts for any missing parameters (or uses flags/manifest/inference)
5. renders concrete files into the target repo

This means the template must be organized so an agent can reason about it easily from another working directory.

## Supported Modes

### Mode: `sync`

Single entry point for both new and existing repos. Detection order:

1. `.governa-manifest` or `.repokit-manifest` found → **re-sync** (existing repo with stored params)
2. Governance artifacts found (AGENTS.md, CLAUDE.md, docs/roles/) → **first sync** (existing repo, no stored params)
3. Otherwise → **new repo** bootstrap

#### New-repo behavior

- prompt for missing parameters interactively (repo type, name, purpose, stack/platform)
- all flags (`-n`, `-y`, `-p`, `-s`, `-u`, `-v`) bypass individual prompts
- copy base files, apply the selected overlay, fill placeholders
- create `CLAUDE.md -> AGENTS.md`
- write `TEMPLATE_VERSION`
- optionally initialize git if the target is not already a repo

#### Existing-repo behavior

- inspect current files before writing anything
- resolve metadata via priority: (1) explicit flag, (2) stored manifest params, (3) inference from target directory, (4) interactive prompt
- create missing governed files from the template
- score existing file collisions using content-aware comparison and report in a consolidated review document
- avoid broad rewrites of user-authored docs unless explicitly approved
- write `TEMPLATE_VERSION`
- create `CLAUDE.md -> AGENTS.md` if missing

Sync is conservative by default for existing repos.
It writes new files directly and reports collisions in a single review document with per-file recommendations.

For `AGENTS.md` specifically, sync checks which governed sections are present. If all governed sections exist, the file scores as `keep`. If sections are missing, the governance patch (with missing sections appended) is included in the review document for the operator to apply manually. Existing content is never modified automatically.

Before writing files, sync assesses how well the template fits the target repo and reports that result to the user.

### Mode: `enhance`

Use when maintaining this template repo itself.

Behavior:

- inspect another governed repo by absolute path
- infer whether that repo appears to derive from this methodology
- extract candidate improvements in governance, overlays, workflow, or bootstrap behavior
- compare those candidates against this template's current files
- create an AC doc for the highest-priority actionable candidate, if any
- patch this template repo only after explicit approval

`enhance` is not a blind sync operation.
It is a review-driven, AC-first proposal flow for template maintainers.
The AC doc is the only output — no `.template-proposed` files are written.

## Implementation Constraints

The bootstrap implementation should be written in Go, not shell.

Reasons:

- cross-platform behavior matters
- Windows support should not depend on a specific shell environment
- requiring Go is acceptable for the target audience
- argument parsing, file operations, and dry-run reporting are easier to keep deterministic in Go

The canonical entrypoint is `cmd/governa/main.go`, installed via `go install github.com/kquo/governa/cmd/governa@latest`.

## CLI Convention

Every bootstrap argument should have:

- a single-letter short flag
- a long-form alias

The short flag is required.
The long form exists as the readable alias.

Recommended flag mapping (mode is determined by subcommand, not a flag):

```text
-t, --target
-y, --type
-n, --repo-name
-p, --purpose
-s, --stack
-u, --publishing-platform
-v, --style
-r, --reference
-g, --init-git
-d, --dry-run
```

This convention should be documented and kept consistent across governa subcommands.

## Ownership Model

The template should define which files are:

- fully template-owned
- partially template-owned
- user-owned

Recommended initial ownership model:

- fully template-owned when created by bootstrap:
  - `AGENTS.md`
  - `CLAUDE.md` symlink
  - `TEMPLATE_VERSION` — records the last governa template version this repo was synced against; updated automatically by `governa sync` on every run
  - `.governa-manifest`
- overlay-owned by default when newly created:
  - `README.md`
  - `arch.md`
  - `plan.md`
  - `style.md` or `voice.md`
  - `content-plan.md` or `calendar.md`
  - `docs/roles/` (DEV, QA, director reference, and custom role docs)
- user-owned unless explicitly mapped:
  - source code
  - app content
  - business docs
  - CI config

For sync on existing repos, template-owned sections should be narrow and explicit.
`AGENTS.md` is the clearest case: it should be treated like a governed config file with named sections.

## Bootstrap Entry Point

The canonical entrypoint is an installable binary:

```
go install github.com/kquo/governa/cmd/governa@latest
```

Subcommand interface:

```text
governa sync [-t <target>] [-y CODE|DOC] [-n "<name>"] [-p "<purpose>"] [-s "<stack>"] [-g] [-d]
governa enhance [-r <reference>] [-d]
governa version
```

`sync` prompts interactively for missing parameters but accepts all flags for fully non-interactive use. If `target` is omitted, the command defaults to the current working directory.

Mode-specific expectations:

- `sync` (new repo): prompts for type, repo-name, purpose, and overlay-specific metadata; flags bypass individual prompts
- `sync` (existing repo): all parameters are resolved in priority order: (1) explicit flag, (2) stored manifest params, (3) inference from the target directory, (4) interactive prompt. Repo name is inferred from the directory basename, purpose from the first `README.md` paragraph, stack from manifest files (`go.mod`, `package.json`, etc.), and type from `AssessTarget` signals. On re-sync, the manifest provides all previously stored values so no flags or prompts are needed
- `enhance` with `-r`: inspects a reference repo for portable improvements
- `enhance` without `-r`: self-review comparing on-disk templates against the embedded baseline

The implementation lives in `cmd/governa/`.

## Agent Responsibilities

The coding agent should:

- run `sync` in the target repo (detection is automatic)
- inspect the reference repo before proposing `enhance` changes
- explain what it plans to create or modify
- gather missing required inputs
- run the deterministic bootstrap step
- summarize exactly what was written

The agent should not improvise template structure from memory.
It should use this repo's concrete files and documented rules.

## Sync Safety Rules

Sync for existing repos should avoid clobbering the repo.

Rules:

- never overwrite an existing file silently
- if a target file already exists, score the collision using content-aware comparison (line count ratio, section count, missing sections, template source changes) and classify as: `keep` (existing is more developed or identical to template), `review: cherry-pick` (proposed adds sections worth considering), `review: content changed` (template source changed since last sync and existing still differs), or `review: no action likely` (structurally different but not clearly better)
- report all collisions in `governa-adopt-review.md` at the repo root — no `.template-proposed` files are written
- for markdown files: identical content → `keep`; existing ≥2x lines → `keep` (unless template changed → `review: content changed`); existing has more sections → `keep` (unless template changed → `review: content changed`); proposed adds missing sections → `review: cherry-pick`; otherwise → `review: no action likely`
- for non-markdown files: if template source-sha256 changed since last sync → `review: content changed`; otherwise → `review: no action likely`
- content-change detection compares old manifest `source-sha256` against new template `source-sha256` and requires that existing content still differs from the new template (no false positives if the repo already absorbed the change manually)
- never rewrite an existing `AGENTS.md` wholesale unless the user explicitly requests replacement
- preserve unrelated local changes

## Sync Fit Assessment

`sync` should estimate template fit before applying changes to existing repos.

The first version does not need ML or scoring complexity.
A deterministic report is enough.

Recommended outputs:

- inferred repo shape: likely `CODE`, likely `DOC`, mixed, or unclear
- existing artifact coverage: which expected files already exist
- collision risk: low, medium, or high
- adoption recommendation: safe to apply, review collisions first, or needs manual mapping first

Recommended signals:

- presence of source directories, build files, test files, package manifests, CI configs
- presence of editorial docs, content calendars, style guides, publishing configs
- presence or absence of `README.md`, `arch.md`, `plan.md`, `docs/`, `AGENTS.md`, `CLAUDE.md`
- degree of conflict between existing files and overlay-owned target files

Example decision rules:

- likely `CODE` if the repo contains source files, test files, package manifests, or build configs
- likely `DOC` if the repo is mostly markdown/content plus editorial structure and lacks software build signals
- high collision risk if `README.md`, `arch.md`, `plan.md`, or style-guide files already exist with substantial user content
- review-only mode if the repo type is ambiguous or collision risk is high

The agent or bootstrap command should show this assessment before modifying an existing repo.

## Enhancement Review

`enhance` should evaluate whether another repo contains genuine template improvements rather than project-specific local choices.

Recommended outputs:

- candidate improvements grouped by area:
  - base governance
  - `CODE` overlay
  - `DOC` overlay
  - bootstrap behavior
  - examples or upgrade path
- section-level deltas for `AGENTS.md` rather than a whole-file rewrite recommendation
- portability assessment: reusable template improvement or project-specific customization
- adoption recommendation: accept, adapt, defer, or reject
- collision impact on this template repo

Recommended evidence sources in the reference repo:

- `AGENTS.md` or equivalent governance files
- bootstrap scripts or commands
- `README.md`, `arch.md`, `plan.md`, `style.md`, `voice.md`, `content-plan.md`
- docs describing workflow, release rules, or upgrade mechanics

Rules:

- never treat a difference as an improvement without explaining why it is better
- prefer portable methodological gains over domain-specific preferences
- do not update this template from a reference repo automatically
- require explicit maintainer approval before applying accepted enhancements
- keep rejected or deferred ideas out of the template baseline unless intentionally added as optional patterns

The current enhance workflow:

1. inspect reference repo
2. read `.governa-manifest` from reference if present (enables three-way comparison)
3. extract candidate deltas
4. compare `AGENTS.md` by named governed sections using constraint-level comparison (each bullet is a distinct constraint; signal matching acts as a fast pre-filter, then normalized constraint sets are compared so two sections with the same keywords but different rules still produce a candidate)
5. compare overlay and workflow artifacts by deterministic file mapping with section-level diffing (structured markdown files with `##` headings are compared per-section; the candidate reports which specific sections changed rather than flagging the whole file; unstructured files fall back to whole-file comparison)
6. when manifest is available, classify each difference by origin: `user` (reference changed, template did not — potential improvement to adopt), `template` (template evolved, reference is stale — defer), `both` (both changed — needs careful review), or empty (no manifest, two-way fallback)
7. classify each delta using a data-driven rule table (project-specific markers, governance, workflow helpers, default fallback); rules are declarative Go structs — adding a new classification is a table entry, not a code branch
8. recommend accept, adapt, defer, or reject
9. create an AC doc under `docs/` for the highest-priority actionable candidate, if any
10. only then patch this template repo after AC approval

The current implementation stops at the review stage and, when actionable improvements are found, creates an AC doc under `docs/` for the highest-priority candidate. It does not auto-apply enhancements.

Before writing a new AC, enhance scans `docs/` for existing enhance-generated ACs (identified by a `# ACN Enhance:` heading in the first line). If found, it prompts: replace the existing AC (same number, new content), update it in place, or create a new one with the next sequential number. EOF or invalid input defaults to "new". Multiple existing enhance ACs are listed by AC number ascending for selection. This prevents AC accumulation from repeated enhance runs against the same reference.

### Bootstrap Manifest

During `sync`, governa writes a `.governa-manifest` file into the generated repo. This file records:

- the template version used at bootstrap time
- SHA-256 checksums of each generated file (after placeholder substitution)
- the source template path and checksum for each file
- adopt parameters (repo name, purpose, type, stack, and DOC-specific fields) so subsequent adopt runs can reuse them without flags

The manifest enables three-way comparison during enhance: by comparing the current reference file against its bootstrap-time checksum, and the current template source against its bootstrap-time checksum, enhance can determine whether a difference came from user customization, template evolution, or both. Repos bootstrapped before this feature have no manifest and fall back to the original two-way comparison. Manifests written before parameter storage are backward compatible — missing params are inferred from the target directory on the next sync run and stored in the updated manifest.

## Why This Fits The Template Repo

To support the workflow above, this repo should contain:

- `internal/templates/base/` for cross-repo governance
- `internal/templates/overlays/code/` for code-repo-specific files
- `internal/templates/overlays/doc/` for doc-repo-specific files
- `cmd/governa/` as the single deterministic renderer
- `examples/` showing one bootstrapped `CODE` repo and one bootstrapped `DOC` repo

This structure lets an agent use the template repo as a stable frame of reference while keeping every generated repo self-contained.
