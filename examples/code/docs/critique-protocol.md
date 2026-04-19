# AC Critique Protocol

Formalizes the AC/critique loop referenced in `AGENTS.md` Approval Boundaries → AC critique gate. The file `docs/ac<N>-<slug>-critique.md` is QA-owned — DEV does not write to it (see `docs/ac-template.md` Companion Artifacts). This doc codifies what the file contains across rounds.

## Round structure — append-only

Each QA round adds a new `## Round N` heading (round 1 is the initial critique; rounds 2+ are verification passes). Round sections are never edited retroactively; new information goes in a new round.

## Finding heading level

Each finding is an `### F<N>` H3 heading with a severity label. Example:

    ### F1 [Blocker] — governance-model.md already carries this convention

Round 1 findings use labels `F1`, `F2`, …. All rounds after round 1 use `F-new-N` labels numbered **monotonically across all subsequent rounds** (e.g., Round 2 uses `F-new-1`, `F-new-2`; if Round 3 introduces a new finding it is `F-new-3`, not a fresh `F-new-1`). This keeps labels globally unique across the critique file and makes cross-references unambiguous.

## Authoring mechanism

QA authors the critique file directly — there is no transport or intermediary. In the current cross-session shuffle this is a human (or agent) writing to the file; in future Phase 2 automation a QA subagent writes the file. Either way, the mechanism is direct write.

## Terminator shape — five fields in order

QA's final round writes `## Round N Summary (terminator)` with exactly these fields in order:

1. **Unresolved findings** — list by severity: blocker / major / minor / nit. Empty allowed.
2. **Residual risks accepted** — items QA flagged but chose not to block on. Empty allowed.
3. **Coverage** — what QA verified this round and what was explicitly out of scope. Prevents false confidence.
4. **Director attention** (optional) — items QA wants the director to see even if not blockers. Distinct from DEV's `Director Review` section in the AC — that is DEV's self-report; this is QA's cross-view.
5. **Verdict** — `no blockers` or `blockers present`.

## DEV's cross-references in the AC

DEV maintains two sections that pair with the critique file:

- **`### Disposition Log`** (H3 subsection under `## Implementation Notes`) — cross-references each QA finding by label and names the resulting AC change. Required for ACs with extensive critique rounds; optional otherwise (`git log` on the AC file carries the same info for short cycles).
- **`## Director Review`** (top-level, between `## Documentation Updates` and `## Status`) — lists every viable-options trade-off chosen during the cycle (not just ones DEV feels uncertain about). QA's final-round `Director attention` field cross-checks that this list is exhaustive and surfaces omissions.

## Critique file lifecycle

The critique file is **not** deleted when QA converges (verdict `no blockers`). It is deleted at release prep alongside the AC, matching the companion-artifact pattern introduced in AC55 and tightened in AC64.

## Termination

The director reviews the AC after QA's verdict `no blockers` lands. Director may redirect (new round opens as `Round N+1`), accept with changes, or authorize implementation.
