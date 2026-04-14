# governa Plan

## Product Direction

Provide a narrow, usable governance template that can bootstrap new repos, adopt existing repos safely, and improve itself through a controlled enhancement path.

## Current Platform

- Go CLI tooling
- Markdown governance templates

## Priorities

(no active roadmap items)

## Ideas To Explore

Pre-rubric ideas captured for future discussion. Prefix each with `IE<N>:` (sequential N) for stable references. These are not commitments and have not passed the objective-fit rubric in `AGENTS.md`. Remove entries when promoted to an AC, completed, or no longer interesting; this section is pre-rubric staging, not a historical record.

- IE1: Extract buildtool/reltool into a shared Go module (`devtools`) so governa, skout, utils, and iq import from one source instead of each carrying template-copied versions; relaxes "self-contained" constraint for build infrastructure only, not governance docs
- IE2: Move repo from `github.com/kquo/governa` to `github.com/queone/governa` — requires go.mod module path change, all internal import paths, `sourceRepo` const, version-check URL, template references, and README/doc links
- IE3: Subsection-level scoring for sync and enhance — currently AGENTS.md gets `## Section` scoring but overlay files (role docs, development-cycle, ac-template, build-release) get whole-file scoring; extend section-level comparison to all markdown files with `##`/`###` structure so the review doc can say "Rules: keep, Governa Templating Maintenance: content changed (structural)" instead of showing monolithic full-file diffs; for enhance, enable `### Subsection` granularity within deferred sections so portable subsections inside project-specific parents (e.g., a generic `### Shell Tool Efficiency` inside a project-specific `## Project Rules`) can be identified as `accept` candidates; includes rendering line-level diffs for small deltas instead of full "Your version" / "Template version" blocks

## Deferred

| ID | Description | Reason |
|----|-------------|--------|

## Constraints

- Pure stdlib; no external Go dependencies
- Generated repos must be self-contained with no runtime dependence on governa
- Templates use `{{PLACEHOLDER}}` substitution, not a templating engine
- Overlays are additive; they must not conflict with the base governance contract
