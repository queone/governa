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

- IE5: Acknowledged drift — allow repos to declare intentional divergence for specific files so sync stops flagging them as `adopt` on every run. Mechanism could be a `.governa-manifest` section (e.g., `[acknowledged]` entries with reason), a comment marker in the file itself, or a `.governa-ignore` file. Standing drift items that have been evaluated and kept as repo-specific should not require re-evaluation each sync cycle. Needs: format design, interaction with `promoteStandingDrift`, review doc rendering for acknowledged items
- IE4: LLM-assisted sync review — add an optional LLM call to governa sync that evaluates diffs and generates concrete summaries of what changed and why it matters, draft dispositions for each item, and a recommended action list. Addresses the observed pattern where agents summarize standing drift as "nothing to do" despite detailed advisory notes. Requires: API key management, provider abstraction, cost/latency tradeoffs, opt-in flag

## Deferred

| ID | Description | Reason |
|----|-------------|--------|

## Constraints

- Pure stdlib; no external Go dependencies
- Generated repos must be self-contained with no runtime dependence on governa
- Templates use `{{PLACEHOLDER}}` substitution, not a templating engine
- Overlays are additive; they must not conflict with the base governance contract
