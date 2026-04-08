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

- active work items only; remove when shipped

## Deferred

| ID | Description | Reason |
|----|-------------|--------|

## Constraints

- Pure stdlib; no external Go dependencies
- Generated repos must be self-contained with no runtime dependence on repokit
- Templates use `{{PLACEHOLDER}}` substitution, not a templating engine
- Overlays are additive; they must not conflict with the base governance contract
