# Library Policy

governa's policy for the libraries extracted from `internal/<x>` packages. Establishes naming, repo skeleton, semver and deprecation cadence, per-library CHANGELOG format, README boilerplate, the convention-coupling test that gates each extraction, the first-consumer self-test that validates each extraction, and the pivot-period guidance for consumer repos surfacing issues during the structural transition.

This policy is intentionally minimum-viable. It refines via amendment as each extraction surfaces gaps, not via rewrite.

## Naming Convention

Library repos are named `governa-<x>` (e.g. `governa-color`, `governa-reltool`). Module paths are `github.com/queone/governa-<x>`. Lineage is visible in the repo and module name; future readers and consumers see "this came from the governa family" without grepping commit history. The naming is consistent with governa's role as the convention archive for the family.

## Repo Skeleton

Each library repo contains:

- `README.md` — see "README Boilerplate" below.
- `CHANGELOG.md` — see "CHANGELOG Format" below.
- `LICENSE`.
- `go.mod` declaring the module path `github.com/queone/governa-<x>`.
- Source files (`<x>.go`, supporting files).
- Tests (`<x>_test.go`).

Explicitly **not** in library repos:

- `AGENTS.md` / `CLAUDE.md`.
- Governed-section conventions, AC file machinery, critique-protocol artifacts.
- Any other governance-encoding doc.

Rationale: libraries inherit doctrine from governa, never duplicate it. A library repo with its own AGENTS.md re-creates the multi-repo coordination cost the apply-once model was built to retire.

## README Boilerplate

Each library README contains the following sections in order:

1. `# governa-<x>` (title).
2. One-paragraph purpose statement — what the library does, in convention-free terms.
3. `## Why` — short explanation of why this library exists in the governa family: the problem it solves, who uses it, why it lives in a separate repo rather than embedded in each consumer. Mirrors governa's own README pattern. Two short paragraphs is plenty.
4. `## Install` — one-line `go get github.com/queone/governa-<x>` snippet.
5. `## Usage` — minimum-viable example showing the most common entry point.
6. `## Versioning` — the back-reference clause: "This library follows the policy in [governa/docs/library-policy.md](https://github.com/queone/governa/blob/main/docs/library-policy.md). See `CHANGELOG.md` for version history and deprecations."

Boilerplate template (copy-paste starting point):

```markdown
# governa-<x>

<one-paragraph purpose statement>

## Why

<two-paragraph explanation of why the library exists, the problem it solves, and how it fits into the governa family>

## Install

go get github.com/queone/governa-<x>

## Usage

<minimum-viable example>

## Versioning

This library follows the policy in [governa/docs/library-policy.md](https://github.com/queone/governa/blob/main/docs/library-policy.md). See `CHANGELOG.md` for version history and deprecations.
```

## CHANGELOG Format

Per-library CHANGELOG follows the same shape as governa's CHANGELOG. Canonical spec lives in [`docs/build-release.md`](build-release.md#pre-release-checklist) under "CHANGELOG row shape." Summary:

- `# Changelog` heading.
- 2-column markdown table: `| Version | Summary |` with a `|---|---|` separator.
- First data row: `| Unreleased | |`.
- One row per release: `| <version> | <one-line summary> |`. Versions are unprefixed (`0.1.0`, not `v0.1.0`).
- Summaries are single-line, ≤ 500 characters; lead with a short tag (`fix:`, `doc:`, AC ref) when applicable.
- **No dates.** **No Keep-a-Changelog sub-sections** (`### Added` etc.) — the per-row summary is the entire entry.

The first row of any library's CHANGELOG is `| 0.1.0 | initial extraction from governa internal/<x> |`. Pre-1.0 versions follow standard SemVer pre-1.0 conventions (anything may change; minor bumps for breaking changes are allowed but discouraged once consumers exist).

## Semver and Deprecation

Libraries follow strict Semantic Versioning (MAJOR.MINOR.PATCH).

- **MAJOR** bump — incompatible API change.
- **MINOR** bump — backward-compatible feature addition.
- **PATCH** bump — backward-compatible bug fix or internal cleanup.

**Deprecation cadence:** deprecations precede removal by **at least two minor versions**. Example: a feature deprecated in v1.4.0 may be removed no earlier than v1.6.0. Removal in a major bump is also allowed and remains the primary mechanism for breaking changes.

Every deprecation MUST be announced in the relevant version's `### Deprecated` CHANGELOG entry with the planned-removal version named explicitly. Example: `*Deprecated*: \`OldFunc\` — replaced by \`NewFunc\`. Scheduled removal: v1.6.0.`

A deprecation that has not been removed by the announced removal version stays valid (the library may extend the timeline with an updated CHANGELOG note); a deprecation MUST NOT be removed earlier than announced.

## Convention-Coupling Test

The gating check applied during each extraction AC's drafting. It determines whether an `internal/<x>` package is extractable to a library or stays template.

**Question:** would the proposed library API need to know any of the following governance touch-points by name or shape?

- `docs/`
- `CHANGELOG.md`
- `AGENTS.md` / `CLAUDE.md`
- `ac<N>-<slug>.md` file shape
- Governed section names (in `AGENTS.md` or governance docs)
- The critique protocol artifacts (`### Round N`, `#### F<N>`, the four-field terminator, `### Disposition Log`)
- Any other governance-file convention specific to governa or a consumer's adopted variant

**If yes:** those concerns belong in a template-side adapter, not in the library core. The library core takes generic terms only — for example, "rewrite regex X in these files," "insert this row at this position in this table," "validate that this file declares this token." The adapter (in `internal/templates/` or in a consumer-side wrapper) supplies governa's specific convention names to the generic library API.

**If no:** the library core can be expressed in convention-free terms; extraction is real and the package is a library candidate.

**If the core API cannot be expressed in convention-free terms after honest effort:** the package stays in `internal/templates/`. The split is not real, only relocated, and shipping it as a library would re-create coupling under a different label.

This test is applied as a checklist during each extraction AC's drafting, not as a one-time judgment about all packages at once. It is also re-applied if the API surface grows during implementation.

### Convention-coupled boilerplate stays in-tree

The convention-coupling test identifies code that extracts cleanly. Its corollary: code that doesn't pass the test stays where it is. This includes thin Go cmd wrappers around extracted libraries when the wrapper is invoked via `go run ./cmd/<x>` from a project-local script (`build.sh`). Extracting such wrappers to the library's own `cmd/` works mechanically but trades inert ~20-line boilerplate for live version-pin propagation in non-Go scripts (`build.sh`, `build.sh.tmpl`), lacking compiler verification. When you find boilerplate that looks extractable, ask: is this a generic CLI anyone outside the family would invoke? If no, it stays in-tree; encode the reasoning at the call site (load-bearing comment) so the next pass doesn't re-litigate. The `cmd/rel` and `cmd/build` wrappers in this repo are the precedent.

## First-Consumer Self-Test

When a library is extracted, governa's own CLI (`cmd/governa/main.go`) becomes the first consumer of it.

**Rule:** an extraction is incomplete until governa's CLI imports the new library and the build (`./build.sh`) passes.

**Diagnostic value:** if the library API cannot be imported cleanly into governa's own CLI — if the import requires shims, type adapters, or convention-specific glue — the split is wrong before any external consumer sees it. The First-Consumer Self-Test is the tight feedback loop that catches a bad split at extraction time, not at first-consumer-migration time.

### Automated convention-coupling recheck

Each extraction AC MUST include an **automated** acceptance test that re-applies the convention-coupling test against the published library source (not just against the planned API). Pattern:

```bash
src=$(go list -m -json github.com/queone/governa-<x> | jq -r .Dir)
! rg -i 'AGENTS|CLAUDE|CHANGELOG|critique|governed|governance|ac[0-9]+|docs/' "$src"/<src files>
```

Returns clean if no governance terms slipped through extraction. A match (e.g., a stray AC reference in a code comment that came along with a verbatim copy) is a finding — clean it up and bump the library to a patch version. The recheck must remain automated; it is not a Director-eyeball task. The `rg` term list is the policy's enumerated governance touch-points (kept in sync with the Convention-Coupling Test section above).

## Pivot-Period Consumer Guidance

This section is **transitional**. It is expected to be removed (or rewritten as historical context) once all extractable internals have shipped as libraries and the consumer fleet has migrated. Future readers should not treat it as evergreen guidance.

### When this guidance applies

A consumer repo discovers an issue ("IE") in its copy of formerly-governa code that is *slated for extraction but not yet libraryized*. Examples: a regex bug, a doctrine ambiguity, a test gap, a behavior that diverges from intent.

### The four-step pattern

1. **Draft a portable advisory.** Describe the symptom, the trap (if any naive fix would worsen the situation), the root cause (across layers — regex, semantics, etc.), the canonical fix venue (which library will resolve it under the relevant extraction AC), and per-consumer-shape guidance. Use the existing advisories in [`docs/advisories/`](advisories/) as format reference. The advisory is intended for archive in governa's `docs/advisories/`, where it serves as design input for the eventual extraction AC.

2. **Ship a doctrine fix locally.** Codify the intended behavior in the consumer's own release docs (e.g. `docs/build-release.md`). This prevents future agents from re-introducing the bug or "fixing" it in a regressive way.

3. **Ship a guard test locally.** Add a test that asserts the intended behavior and fails if anyone modifies the underlying code in a way that worsens the situation. Assertion direction is "values must NOT change" — typically a bytes-equal end-to-end check with a sentinel input. The test's failure message should cite the relevant advisory in `docs/advisories/`.

4. **Do NOT patch the formerly-governa code itself.** Any local code patch to the slated-for-extraction code is throwaway when the consumer migrates to the eventual library; it adds drift in the meantime with zero lasting value. The structural fix lands in the library, not in the consumer copy.

### Worked example

The first instance of this pattern was set by the `utils` consumer repo's AC26 / IE9 handling — the `programVersion` bump regex. The canonical advisory is archived at [`docs/advisories/program-version-bump.md`](advisories/program-version-bump.md). Read it as the template shape for future advisories.

### Advisory archive

`docs/advisories/` is the canonical archive for portable advisories surfaced from consumer repos. Each advisory is a single Markdown file describing one issue and its canonical fix venue. The formal advisory-log mechanism (intake process, severity tiers, per-consumer ledger, lifecycle rules) is future work and is tracked as an Idea To Explore in [`plan.md`](../plan.md). Until it lands, the directory functions as a flat archive: each advisory follows the format established by `program-version-bump.md`, and the steward intakes new advisories ad hoc.
