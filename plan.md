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


## Deferred

| ID | Description | Reason |
|----|-------------|--------|

## Constraints

- Pure stdlib; no external Go dependencies
- Generated repos must be self-contained with no runtime dependence on governa
- Templates use `{{PLACEHOLDER}}` substitution, not a templating engine
- Overlays are additive; they must not conflict with the base governance contract
