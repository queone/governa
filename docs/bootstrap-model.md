# Bootstrap Model

This template needs to support two distinct workflows from a target repo:

- bootstrap a new repo
- adopt the methodology into an existing repo

It also needs one template-maintenance workflow:

- enhance this template from lessons captured in another governed repo

All workflows should use the same deterministic command surface and the same template source tree.

## Core Principle

The agent runs in the target repo, not in this template repo.
This template repo is a read-only source of concrete files and rules.
Generated repos copy rendered files and do not inherit from this repo at runtime.

Exception:

- `enhance` runs from inside this template repo because its purpose is to improve the template itself

## Target-Repo Flow

The intended user flow is:

1. create or open the target directory
2. start a coding agent in that directory
3. tell the agent where this template repo lives, for example `<template-root>`
4. ask the agent to bootstrap or adopt the governance template
5. let the agent inspect the current repo state and gather missing inputs
6. render concrete files into the target repo

This means the template must be organized so an agent can reason about it easily from another working directory.

## Supported Modes

### Mode: `new`

Use when the target directory is empty or effectively empty.

Behavior:

- require `repo type`: `CODE` or `DOC`
- require `repo name`
- require `project purpose`
- require `stack/platform` if `CODE`
- require `publishing platform/style` if `DOC`
- copy base files
- apply the selected overlay
- fill placeholders
- create `CLAUDE.md -> AGENTS.md`
- write `TEMPLATE_VERSION`
- optionally initialize git if the target is not already a repo

### Mode: `adopt`

Use when the target already contains an existing project.

Behavior:

- inspect current files before writing anything
- require the same metadata inputs as `new`
- create missing governed files from the template
- patch only clearly owned sections in existing governed files
- avoid broad rewrites of user-authored docs unless explicitly approved
- write `TEMPLATE_VERSION`
- create `CLAUDE.md -> AGENTS.md` if missing

`adopt` must be conservative by default.
It should prefer adding missing files or appending clearly labeled sections over replacing existing docs wholesale.

Before writing files, `adopt` should also assess how well the template fits the target repo and report that result to the user.

### Mode: `enhance`

Use when maintaining this template repo itself.

Behavior:

- inspect another governed repo by absolute path
- infer whether that repo appears to derive from this methodology
- extract candidate improvements in governance, overlays, workflow, or bootstrap behavior
- compare those candidates against this template's current files
- produce a deterministic review report before changing anything
- patch this template repo only after explicit approval

`enhance` is not a blind sync operation.
It is a review-driven proposal flow for template maintainers.

## Implementation Constraints

The bootstrap implementation should be written in Go, not shell.

Reasons:

- cross-platform behavior matters
- Windows support should not depend on a specific shell environment
- requiring Go is acceptable for the target audience
- argument parsing, file operations, and dry-run reporting are easier to keep deterministic in Go

The canonical entrypoint is `cmd/bootstrap/main.go`, invoked via `go run <template-root>/cmd/bootstrap`.

## CLI Convention

Every bootstrap argument should have:

- a single-letter short flag
- a long-form alias

The short flag is required.
The long form exists as the readable alias.

Recommended initial mapping:

```text
-t, --target
-m, --mode
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

This convention should be documented and kept consistent across bootstrap commands and future companion tools.

## Ownership Model

The template should define which files are:

- fully template-owned
- partially template-owned
- user-owned

Recommended initial ownership model:

- fully template-owned when created by bootstrap:
  - `AGENTS.md`
  - `CLAUDE.md` symlink
  - `TEMPLATE_VERSION`
- overlay-owned by default when newly created:
  - `README.md`
  - `arch.md`
  - `plan.md`
  - `style.md` or `voice.md`
  - `content-plan.md` or `calendar.md`
- user-owned unless explicitly mapped:
  - source code
  - app content
  - business docs
  - CI config

For adoption mode, template-owned sections should be narrow and explicit.
`AGENTS.md` is the clearest case: it should be treated like a governed config file with named sections.

## Bootstrap Entry Point

The template should provide one deterministic entrypoint:

- a Go-based bootstrap command exposed through a stable repo-local entrypoint

Recommended arguments:

```text
bootstrap \
  [-t, --target <target-root>] \
  -m, --mode new|adopt|enhance \
  -y, --type CODE|DOC \
  -n, --repo-name "<name>" \
  -p, --purpose "<purpose>" \
  [-s, --stack "<stack/platform>"] \
  [-u, --publishing-platform "<platform>"] \
  [-v, --style "<voice/style>"] \
  [-r, --reference <reference-root>] \
  [-g, --init-git] \
  [-d, --dry-run]
```

The agent may gather these interactively, but the script itself should accept explicit inputs so runs are reproducible.
If `target` is omitted for `new` or `adopt`, the command should default to the current working directory.

Mode-specific expectations:

- `new`: requires `type`, `repo-name`, `purpose`, plus overlay-specific metadata
- `adopt`: requires `type` unless inferred confidently, `repo-name`, and `purpose`
- `enhance`: requires `reference` and should normally run with this template repo as the current working tree

The implementation lives in `cmd/bootstrap/`, invoked via `go run <template-root>/cmd/bootstrap`.

## Agent Responsibilities

The coding agent should:

- inspect the target repo before choosing `new` versus `adopt`
- inspect the reference repo before proposing `enhance` changes
- explain what it plans to create or modify
- gather missing required inputs
- run the deterministic bootstrap step
- summarize exactly what was written

The agent should not improvise template structure from memory.
It should use this repo's concrete files and documented rules.

## Adoption Safety Rules

Adoption mode should avoid clobbering an existing repo.

Rules:

- never overwrite an existing file silently
- if a target file already exists, either:
  - patch a named owned section, or
  - write a sibling `*.template-proposed.md` file, or
  - stop and ask for approval
- never rewrite an existing `AGENTS.md` wholesale unless the user explicitly requests replacement
- preserve unrelated local changes

## Adoption Fit Assessment

`adopt` should estimate template fit before applying changes.

The first version does not need ML or scoring complexity.
A deterministic report is enough.

Recommended outputs:

- inferred repo shape: likely `CODE`, likely `DOC`, mixed, or unclear
- existing artifact coverage: which expected files already exist
- collision risk: low, medium, or high
- adoption recommendation: safe to apply, safe with proposals only, or needs manual mapping first

Recommended signals:

- presence of source directories, build files, test files, package manifests, CI configs
- presence of editorial docs, content calendars, style guides, publishing configs
- presence or absence of `README.md`, `arch.md`, `plan.md`, `docs/`, `AGENTS.md`, `CLAUDE.md`
- degree of conflict between existing files and overlay-owned target files

Example decision rules:

- likely `CODE` if the repo contains source files, test files, package manifests, or build configs
- likely `DOC` if the repo is mostly markdown/content plus editorial structure and lacks software build signals
- high collision risk if `README.md`, `arch.md`, `plan.md`, or style-guide files already exist with substantial user content
- proposals-only mode if the repo type is ambiguous or collision risk is high

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

One useful first version is report-first behavior:

1. inspect reference repo
2. extract candidate deltas
3. compare `AGENTS.md` by named governed sections
4. compare overlay and workflow artifacts by deterministic file mapping
5. write a review artifact such as `docs/enhance-report.md`
6. classify each delta as portable, needs-review, or project-specific
7. recommend accept, adapt, defer, or reject
8. only then patch this template repo

The current implementation stops at the report stage, writes `docs/enhance-report.md`, and does not auto-apply enhancements.

## Why This Fits The Template Repo

To support the workflow above, this repo should contain:

- `base/` for cross-repo governance
- `overlays/code/` for code-repo-specific files
- `overlays/doc/` for doc-repo-specific files
- `cmd/bootstrap/` as the single deterministic renderer
- `examples/` showing one bootstrapped `CODE` repo and one bootstrapped `DOC` repo

This structure lets an agent use the template repo as a stable frame of reference while keeping every generated repo self-contained.
