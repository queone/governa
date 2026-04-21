# governa Plan

## Product Direction

Provide a narrow, usable governance template that can bootstrap new repos, adopt existing repos safely, and improve itself through a controlled enhancement path.

## Ideas To Explore

Ideas captured for future reference. Prefix each with `IE<N>:` (sequential N) for stable references. Entries come in two shapes: (a) **pre-rubric idea** — `IE<N>: <one-liner>` awaiting director discussion and the objective-fit rubric (see `AGENTS.md` Approval Boundaries); (b) **pointer to a drafted AC stub** not yet scoped through the critique cycle — `IE<N>: <one-liner> → docs/ac<N>-<slug>.md`. A shape (a) entry that clears the rubric converts to shape (b) at AC-draft time (keeping the same `IE<N>` number) rather than being removed, so the entry persists as a pointer until the pointed-to AC ships. Remove entries when the underlying idea is closed: rejected, retired, or (for AC pointers) the pointed-to AC has shipped and its file has been deleted. This section is not a historical record.

- IE4: LLM-assisted sync review — add an optional LLM call to governa sync that evaluates diffs and generates concrete summaries of what changed and why it matters, draft dispositions for each item, and a recommended action list. Addresses the observed pattern where agents summarize standing drift as "nothing to do" despite detailed advisory notes. Requires: API key management, provider abstraction, cost/latency tradeoffs, opt-in flag
- IE5: DEV/QA automation — reduce the manual cross-terminal copy-paste of the DEV/QA critique loop without losing the fresh-context isolation that makes critique valuable. Phase 1 (tighten the AC/critique protocol) shipped in AC65; the live spec is `docs/critique-protocol.md` plus the `## Director Review` section in `docs/ac-template.md`. Remaining scope: (a) pilot 2–3 ACs under the shipped Phase 1 protocol to collect evidence on critique quality, then (b) choose the Phase 2 mechanism among a mechanical bridge script, same-session role-switching, or subagent-based QA based on pilot data. Phase 3 specialist fan-out deferred. Forward-looking design context (Phase-0.5 bridge, Option 2, Option 3, open questions) preserved in `docs/knowledge/dev-qa-automation-exploration.md`.
