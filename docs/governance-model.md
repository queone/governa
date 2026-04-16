# Governance Model

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

The canonical entrypoint is `cmd/governa/main.go`, installed via `go install github.com/queone/governa/cmd/governa@latest`.

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

## Agent Agnosticism

Governa is agent-agnostic. `AGENTS.md` is the canonical governance contract. Agent-specific entrypoints (`CLAUDE.md` for Claude Code, future names like `CURSOR.md` or `COPILOT.md` as they emerge) **must** be symlinks to `AGENTS.md` so every agent loads exactly the same rules. One contract, many agent-specific names pointing at it.

This invariant is enforced during sync. When sync plans a `CLAUDE.md → AGENTS.md` symlink and finds an existing regular file at `CLAUDE.md`, it does not overwrite the file (preserving operator content) but records a conflict in the review doc and prints a final `disposition: needs manual resolution` line to stderr. Sync also returns a non-zero exit code so scripted callers can detect the unresolved state.

The same rule applies to any planned symlink-to-AGENTS.md op that collides with a regular file, so future agent-specific entrypoints inherit the same protection without code changes.

### Safe migration sequence

Removing the existing `CLAUDE.md` without inspection risks discarding the only copy of repo-specific governance. The operator-facing workflow is:

1. diff the existing `CLAUDE.md` against the newly written `AGENTS.md`
2. migrate any unique repo-specific rules into `AGENTS.md` (use the governance section structure)
3. delete the existing `CLAUDE.md` and re-run `governa sync` to create the symlink

If the existing `CLAUDE.md` content is already covered by `AGENTS.md`, deletion alone is safe.

### Operator-facing surfaces

Two distinct outputs report sync state to the operator:

- **pre-sync assessment** — printed before writes. Shows repo shape, signals, existing artifacts, and collision risk. Useful for dry-runs and understanding what sync is about to evaluate. As of AC46, this output does not include a `recommendation:` line — that field was derived from repo shape + collision risk (both still printed) and created perceived contradiction with the final disposition. The `Assessment.Recommendation` struct field remains for programmatic use but is no longer printed. As of AC47, the `collisions:` line is suppressed when it would duplicate `existing-artifacts:` (the common case); it still prints when the two differ (e.g., a zero-size expected file is present). As of AC48, `signals:` counts exclude `.governa-proposed/` (a working artifact) and governa-owned paths (bookkeeping files, agent entrypoints, overlay markdown written by governa) so first-sync and re-sync produce the same counts for the same underlying repo content. The exclusion is scoped to signal counting only — `ExistingArtifacts`, collision scoring, review rendering, and `.governa-proposed/` materialization are unaffected.
- **type provenance** — after the assessment, the resolved repo type is surfaced with a provenance label. On the manifest path (re-sync), `printParamSources` prints `type: CODE (manifest)`. When the type was inferred from repo shape (new-repo bootstrap, or first sync of an existing repo without a manifest), `runSync` prints `type: CODE (inferred)` after `printAssessment`. Exactly one provenance line is emitted per run.
- **final sync disposition** — printed after transforms when conflicts are detected. Prefixed with `disposition:` and reports the final state (e.g., `needs manual resolution — N conflict(s) detected`). When no conflicts are detected, the drift summary is the final recommendation and no `disposition:` line is emitted.
- **`.governa-proposed/` contents** — materializes template counterparts for (1) all `adopt` items, and (2) `keep` items that carry advisory notes (`missingSections` or `sectionRenames`). `keep` items without advisory notes are not materialized. A single predicate (`shouldMaterializeProposal`) governs both `writeProposedFiles` and the Advisory Notes diff-command suffix so the review doc can never point at a missing file. As of AC51, the directory is cleaned at the start of every sync run (guarded by `!dryRun`) so stale entries from prior runs never accumulate.
- **`## Template Changes`** (AC51) — when sync moves across template versions, a brief summary at the top of `governa-sync-review.md` lists CHANGELOG rows for intermediate versions, sourced from governa's embedded `CHANGELOG.md`. Omitted on first sync or same-version re-sync.
- **Adoption Items new-section wording** (AC51) — when a template addition introduces new sections, the Adoption Items entry uses `adds sections:` (not the prior `missing sections:`) and surfaces new sections even alongside other changes (not subsumed into `(cosmetic)` classifications).
- **Manifest `sha256:` accuracy** (AC51) — for `adopt` and `keep` items where governa did not overwrite the repo file, the manifest records the actual on-disk sha256, not the planned-write sha. `source-sha256:` continues to reflect the template source.
- **Stack-aware `.gitignore`** (AC51) — when `cfg.Stack` matches a known stack (Go today; extensible via `internal/templates/stack-ignores/`), the rendered `.gitignore` appends a language-specific block after the cross-language base. Eliminates the recurring Standing Divergence pattern for Go repos.

The `## Conflicts` section in `governa-sync-review.md` is the durable operator-facing surface for these invariant violations. It renders before `## Recommendations` because conflicts must be resolved before the rest of the review is actionable.

As of AC48, the `## Status` section in `governa-sync-review.md` reflects the actual review state: `PENDING — operator review required` when `adopt` items or conflicts exist, `CLEAN — no required adoption/conflict action` otherwise. `CLEAN` does not imply "nothing to review" — `keep`-with-advisory items are still reviewable but do not block sync completion.

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
go install github.com/queone/governa/cmd/governa@latest
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
- if a target file already exists, score the collision using content-aware comparison (line count ratio, section count, missing sections, template source changes) and classify as: `keep` (file is identical, more developed, or structurally different with no template evolution) or `adopt` (template has improvements — proposed adds sections, template changed since last sync, un-adopted differences from previous syncs, or structural alignment needed)
- report all collisions in `governa-sync-review.md` at the repo root. The review doc header shows the template version transition (e.g., `Template version: 0.17.0 → 0.18.0`). For `adopt` files, sync writes the template version to `.governa-proposed/<path>` for direct comparison
- for markdown files: identical content → `keep`; existing ≥2x lines → `keep` (unless template changed → `adopt`); existing has more sections → `keep` (unless template changed → `adopt`); proposed adds missing sections → `adopt`; files with structural observations (subsections deeper than template) → `adopt`; otherwise → `keep`
- for non-markdown files: if template source-sha256 changed since last sync → `adopt`; otherwise → `keep`
- content-change detection compares old manifest `source-sha256` against new template `source-sha256` and requires that existing content still differs from the new template (no false positives if the repo already absorbed the change manually)
- for `adopt` items with changed sections, each is classified as `structural` (heading/list-item count changed, numbered steps reordered, paragraphs added/removed) or `cosmetic` (wording changes within same-shaped content). The classification appears in the reason string
- section-level scoring applies to all markdown overlay files with `##` sections (not just AGENTS.md). The recommendations table stays file-level; section names appear in the Adoption Items section
- preamble content (text before the first `##` heading) is captured as a synthetic `(preamble)` section and participates in change detection and classification like any named section
- for `keep` files that have template sections not present in the existing file: an advisory note appears under `## Advisory Notes` listing the missing sections. The recommendation stays `keep` — the note is informational
- when a section exists in one version but not the other and their bodies share >=50% of lines, sync reports it as a section rename in Advisory Notes
- when the template source hasn't changed since last sync but the actual file still differs from the template, promote from `keep` to `adopt` — this surfaces un-adopted changes from previous sync rounds
- the review doc is a lean action list: each `adopt` item lists affected sections and a `diff` command pointing to `.governa-proposed/<file>`. Full file content lives on disk, not inline in the review doc. The directory explanation is `.governa-proposed/ABOUT.md` (not `README.md`, to avoid collision with a proposed repo README)
- scaffold demotion: for known scaffold files (`README.md`, `arch.md`, `plan.md`), if the proposed content contains placeholder markers (e.g., "State why this repo exists", "## Replace Me") and the existing content does not, demote `adopt` to `keep` — the repo has replaced the starter content with real project content. Does not apply when the adopt reason is content-changed or structural
- extracted-package demotion: for non-markdown files where the existing file is ≤ ¼ the proposed line count and imports a local package (module-path-prefixed), demote `adopt` to `keep` — the repo has intentionally extracted the template's monolithic logic into a reusable package
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

The `Assessment.Recommendation` struct field (`safe to apply`, `safe with proposals only`, `needs manual mapping first`) is still computed for programmatic callers, but it is no longer printed to the operator. Repo shape + collision risk convey the same signal, and the final `disposition:` line (printed only on conflicts) is the authoritative final recommendation.

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
