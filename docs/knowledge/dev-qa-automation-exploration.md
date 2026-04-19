# DEV/QA Automation Exploration

Status: **Phase 1 shipped (AC65); Phase 2+ forward-looking**. Roadmap pointer: `plan.md` IE5.

Captures the design conversation between director, DEV, and QA about reducing the manual copy-paste cost of the DEV/QA critique loop without losing the property that makes critique valuable. Phase 1 (protocol tightening) shipped and is now live in `docs/critique-protocol.md` + `docs/ac-template.md` § Director Review — the Phase 1 section below preserves only the design-alternative rejection rationale as historical context. Phase 2+ sections remain forward-looking. This document is disposable — if IE5 is abandoned or Phase 2 ships into governance proper, the remaining forward-looking content can be deleted alongside the IE5 plan.md entry.

## Problem statement (director)

Today, DEV and QA run in separate terminal sessions. The director orchestrates the loop by copy-pasting QA's findings into DEV's session and DEV's revisions back to QA. Valuable because the director can intercept direction mid-cycle. Cumbersome at scale — AC64 ran six critique rounds with substantial round-to-round context to carry across.

The director asked: can a single Claude session switch DEV/QA hats, pipe responses between them, and stop at the end for final director review? What's the consistency cost with the existing dev cycle? What does this mean for the maintainer role?

## Why straight role-switching is risky (DEV + QA converge)

QA's value is not just fresh output — it is **fresh context**. A separate session means QA never sees DEV's reasoning, only DEV's artifacts. When the same model switches hats inside one session, it has already committed to its design choices and unconsciously defends them. Self-critique is measurably weaker than isolated-context critique.

QA evidence from AC64's six rounds: F1, F-new-1, F-new-5, F-new-6 were all caught by independently reading the repo code without seeing DEV's reasoning. A same-session hat-switch would likely have missed some of these.

## Options

Ordered from least to most automation ambition.

### Phase 1 — shipped (AC65)

Phase 1 shipped in AC65. The live protocol spec is `docs/critique-protocol.md` (round-append structure, `### F<N>` finding heading level, `F-new-N` monotonic numbering, five-field terminator shape, DEV cross-reference sections, critique-file lifecycle). The `## Director Review` section landed in `docs/ac-template.md` between `## Documentation Updates` and `## Status`.

Design-alternative rejections preserved here as historical rationale (useful if a future AC revisits any of these choices):

- **Critique file structure — round-append chosen.** Alternatives rejected: living by-finding (mutation-in-place makes diffs noisy and complicates future subagent handoff); hybrid with a status summary table (duplicates what the final-round terminator already provides at lower cost).
- **`Director Review` placement — new top-level AC section chosen.** Alternatives rejected: subsection under `Implementation Notes` (buried; defeats the cheap-final-walkthrough mitigation); short list in the `Summary` paragraph (pollutes the one-paragraph brief; doesn't scale).
- **Section naming.** "Director Review" chosen over "Director Decisions Pending" — the latter implies nothing's been decided, which isn't accurate; DEV/QA will have picked defaults that director is reviewing.
- **Terminator shape — five fields chosen.** QA's original proposal was three (Unresolved / Residual / Verdict). Director-approved expansion added `Coverage` (prevents false confidence) and `Director attention` (QA's cross-view distinct from DEV's self-report). Pilot is where we learn if all five earn their keep; any field with chronically low signal gets trimmed in a later AC.
- **`F-new-N` numbering — monotonic across all rounds after round 1.** Alternative rejected: reset per round (cross-references become ambiguous).
- **`Disposition Log` — optional, not required.** Alternative rejected: require on every AC (bookkeeping overhead for small ACs where `git log` already carries the info).
- **Critique file lifecycle — delete at release prep, not on convergence.** Alternative rejected: delete when QA converges (loses audit trail if a post-ship regression needs the design history).

### Phase-0.5 — mechanical bridge script (QA-proposed)

Keep separate sessions (full context isolation preserved). Add a small script that reads the latest QA response from one session's transcript, writes it to `-critique.md`, reads DEV's AC revision, pastes it back. Less automation ambition than Option 3; strictly preserves the property that makes critique valuable today. Cheap. Worth considering before committing to subagent architecture.

### Phase 2 Option 2 — full same-session role-switching

Director prompts DEV; DEV revises; director reprompts as QA; model switches hats. Automates the loop inside one session. **Loses the fresh-context property.** Loses mid-round director intervention. Highest risk.

### Phase 2 Option 3 — subagent-based QA

DEV (main session) spawns a QA subagent via the `Task` tool each round. Subagent sees only the AC file and the QA role prompt (`docs/roles/qa.md`) — not DEV's reasoning. Subagent writes findings to `-critique.md` and returns. DEV revises, re-spawns. Loop terminates on QA's `Round N Summary` verdict. Director reviews the final AC.

QA refinement: pass the **prior round's critique file** as input to the fresh subagent. Inherits context deliberately, losing a sliver of freshness but gaining continuity. Matches how human QA works — you remember what you flagged last round.

Preserves the isolation property; eliminates the manual shuffle; keeps director as final gate.

### Phase 3 — specialist QA fan-out (DEV-proposed; QA flagged as premature)

Instead of one generalist QA subagent per round, fan out to narrow specialists — security review, test coverage, doc alignment, protocol conformance. Each has a tighter role prompt. Better coverage, parallelizable, heavier upfront investment. **Do not design for this until generalist-subagent-QA is proven.**

## Consistency with the development cycle

`docs/development-cycle.md` assumes DEV-QA separation but does not specify the *mechanism*. Today the mechanism is "separate terminal sessions"; tomorrow it could be Phase-0.5 bridge, Option 2, or Option 3. The governance contract says "external critique" — which a subagent with isolated context still satisfies.

**Implication:** whichever Phase 2 option lands, `docs/development-cycle.md` must be updated in the same pass to make the separation mechanism explicit rather than incidental. The shipped option becomes the contractual mechanism.

## Maintainer role — orthogonal

`docs/roles/maintainer.md` is for small solo changes with a relaxed critique gate. It is **not** "DEV+QA fused." Extending maintainer latitude to all ACs would deprecate DEV/QA separation, which is not the intent. Keep maintainer for small changes; keep DEV/QA (with whatever mechanism Phase 2 lands on) for substantive ACs.

## Director-gating concern

Any automation that trims mid-cycle director intervention needs a compensating mechanism. The `## Director Decisions Pending` section in Phase 1 is the primary mitigation — every sub-loop judgment call is surfaced for the director's final walkthrough. QA's final-round cross-check that the list is exhaustive is the secondary mitigation.

DEV's original framing left "Director Decisions" as a self-report. QA tightened it: DEV must list every viable-options trade-off, not only uncertain ones, and QA verifies completeness. Agents have a blind spot for their own judgment calls; the cross-check closes it.

## Sequencing recommendation

1. **Ship Phase 1** first. Tighten the protocol (terminator-with-residuals, exhaustive director-decisions, critique-file lifecycle, disposition log). Run it under the current cross-session shuffle.
2. **Pilot 2–3 ACs** under the tight protocol. Collect evidence: does critique quality change? Does director-decisions cross-check catch real misses?
3. **Decide Phase 2 with data.** Choose among Phase-0.5 bridge, Option 2, or Option 3 based on observed critique quality and the cost/benefit of each. Do not pre-commit.
4. **Defer Phase 3** indefinitely.

"Let the data decide" — QA's phrasing, endorsed by DEV.

## Open questions

1. For Option 3, does the QA subagent have enough context to independently verify repo-code claims (as human-QA did with F-new-1's scorer vocabulary check, F-new-6's `Run()` inspection)? Subagents have isolated context but can still read files. Probably yes; Phase 1 pilot will tell us whether DEV's ACs surface enough repo pointers for QA to verify without hand-holding.
2. For Phase-0.5, what's the transcript format Claude Code exposes? Script feasibility depends on this — worth a short spike before committing.
3. How do we measure "critique quality" during the pilot? Proposed proxy metrics: findings count per round, blocker-vs-minor ratio, round count to convergence, post-implementation QA verification pass rate. Not perfect, but trackable.
4. Does the Option 3 subagent approach compose with long-running sessions (context compaction)? If DEV's main session compacts mid-cycle, does the AC state survive? Needs a smoke test.

## Attribution

Perspectives preserved where they diverged:

- **DEV** originally proposed `## No Further Findings` as a binary terminator; QA pushed back with terminator-with-residuals. QA's shape adopted.
- **DEV** originally proposed deleting the critique file on convergence; QA flagged the audit-trail regression. QA's position adopted.
- **DEV** originally framed `Director Decisions Pending` as a self-report of uncertain calls; QA tightened to exhaustive list + QA cross-check. QA's tightening adopted.
- **DEV** did not name Phase-0.5 (mechanical bridge script); QA added it as a middle path. Captured.
- **DEV** proposed specialist fan-out (Phase 3); QA flagged as premature. Deferred.
- **QA** caveat that same-session role-switching quality is speculative until piloted. DEV agrees.

## If this becomes an AC

IE5 (Phase 1 only) is the natural first AC. Phase 2 is a second AC after the pilot. Phase 3 stays in `plan.md` unless/until data motivates it.
