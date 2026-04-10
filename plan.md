# repokit Plan

## Product Direction

Provide a narrow, usable governance template that can bootstrap new repos, adopt existing repos safely, and improve itself through a controlled enhancement path.

## Current Platform

- Go CLI tooling
- Markdown governance templates

## Objective-Fit Rubric

Every new roadmap item should answer:

1. what user or system outcome does this serve
2. why is this a better next step than competing work
3. what existing decisions or constraints does it depend on
4. is this direct roadmap work or an intentional pivot

## Priorities

(no active roadmap items)

## Ideas To Explore

Pre-rubric ideas captured for future discussion. Prefix each with `IE<N>:` (sequential N) for stable references. These are not commitments and have not passed the Objective-Fit Rubric. Remove entries when promoted to an AC, completed, or no longer interesting; this section is pre-rubric staging, not a historical record.

- IE1: non-git target support: bootstrap into directories that are not git repos, for security/privacy use cases where git may be added later or never
- IE2: optional LLM assistance in enhance: evaluate additive LLM roles (candidate summarization, rationale drafting, second-opinion review) on top of the deterministic core; deterministic logic stays as the enforcement layer, LLM output is informational only and opt-in
- IE4: Aside from ac-template.md, what else can repokit adopt from skout repo? or is this just a real-world test of `repokit enhance -r /Users/tek1/code/skouts`?

## Deferred

| ID | Description | Reason |
|----|-------------|--------|

## Constraints

- Pure stdlib; no external Go dependencies
- Generated repos must be self-contained with no runtime dependence on repokit
- Templates use `{{PLACEHOLDER}}` substitution, not a templating engine
- Overlays are additive; they must not conflict with the base governance contract
