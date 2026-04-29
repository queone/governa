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
3. `## Install` — one-line `go get github.com/queone/governa-<x>` snippet.
4. `## Usage` — minimum-viable example showing the most common entry point.
5. `## Versioning` — the back-reference clause: "This library follows the policy in [governa/docs/library-policy.md](https://github.com/queone/governa/blob/main/docs/library-policy.md). See `CHANGELOG.md` for version history and deprecations."

Boilerplate template (copy-paste starting point):

```markdown
# governa-<x>

<one-paragraph purpose statement>

## Install

go get github.com/queone/governa-<x>

## Usage

<minimum-viable example>

## Versioning

This library follows the policy in [governa/docs/library-policy.md](https://github.com/queone/governa/blob/main/docs/library-policy.md). See `CHANGELOG.md` for version history and deprecations.
```

## CHANGELOG Format

Per-library CHANGELOG follows Keep-a-Changelog conventions.

File header:

```markdown
# Changelog

All notable changes to this project are documented in this file.

The format is based on Keep a Changelog and this project adheres to Semantic Versioning.
```

Per-version section: `## [<version>] - <YYYY-MM-DD>` with sub-sections in this order: `### Added`, `### Changed`, `### Deprecated`, `### Removed`, `### Fixed`. Omit empty sub-sections.

The first entry of any library's CHANGELOG is `## [0.1.0] - <YYYY-MM-DD>` for the initial extraction. Pre-1.0 versions follow standard SemVer pre-1.0 conventions (anything may change; minor bumps for breaking changes are allowed but discouraged once consumers exist).

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

## First-Consumer Self-Test

When a library is extracted, governa's own CLI (`cmd/governa/main.go`) becomes the first consumer of it.

**Rule:** an extraction is incomplete until governa's CLI imports the new library and the build (`./build.sh`) passes.

**Diagnostic value:** if the library API cannot be imported cleanly into governa's own CLI — if the import requires shims, type adapters, or convention-specific glue — the split is wrong before any external consumer sees it. The First-Consumer Self-Test is the tight feedback loop that catches a bad split at extraction time, not at first-consumer-migration time.

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
