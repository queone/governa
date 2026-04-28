# AC Critique Protocol

Formalizes the AC/critique loop referenced in `AGENTS.md` Approval Boundaries → AC critique gate. Editor findings live inside the AC file in a `## Critique` section — there is no separate companion file. This doc codifies what that section contains across rounds.

## Where findings live

Every AC carries a `## Critique` section (typically the second-to-last top-level section, above `## Status`). Editor-authored content lands there; Operator transcribes Editor's findings verbatim from the critique channel into this section. Operator's response to each finding is visible as AC revisions (via `git log`/`git diff`) plus entries in the `### Disposition Log` subsection under `## Implementation Notes`.

## Round structure — append-only

Each Editor round adds a new `### Round N` H3 heading inside the `## Critique` section (round 1 is the initial critique; rounds 2+ are verification passes). Round sections are never edited retroactively; new information goes in a new round.

## Finding heading level

Each finding is an `#### F<N>` H4 heading with a severity label. Example:

    #### F1 [Blocker] — cross-reference points to a deleted section

Round 1 findings use labels `F1`, `F2`, …. All rounds after round 1 use `F-new-N` labels numbered **monotonically across all subsequent rounds** (e.g., Round 2 uses `F-new-1`, `F-new-2`; if Round 3 introduces a new finding it is `F-new-3`, not a fresh `F-new-1`). This keeps labels globally unique across the AC's critique history and makes cross-references unambiguous.

Heading-level note: the integrated-mode levels are `## Critique` (H2) → `### Round N` (H3) → `#### F<N>` (H4). The extra depth comes from embedding rounds inside the AC rather than using a separate file; round headings were H2 in the prior separate-file mode.

## Authoring mechanism

Editor authors findings in the critique channel (the mechanism the director chooses — today a separate Claude Code session relaying findings in conversation; future Phase 2 automation may spawn an Editor subagent). Operator transcribes those findings **verbatim** into the AC's `## Critique` section. Operator does not modify or reinterpret Editor's wording — the content is Editor-authored even though Operator operates the write.

## Terminator shape — five fields in order

Editor's final round writes `### Round N Summary` with exactly these fields in order:

1. **Unresolved findings** — list by severity: blocker / major / minor / nit. Empty allowed.
2. **Residual risks accepted** — items Editor flagged but chose not to block on. Empty allowed.
3. **Coverage** — what Editor verified this round and what was explicitly out of scope. Prevents false confidence.
4. **Director attention** (optional) — items Editor wants the director to see even if not blockers. Distinct from Operator's `Director Review` section in the AC — that is Operator's self-report; this is Editor's cross-view.
5. **Verdict** — `no blockers` or `blockers present`.

## Operator's cross-references in the AC

Operator maintains two sections that pair with the `## Critique` section:

- **`### Disposition Log`** (H3 subsection under `## Implementation Notes`) — cross-references each Editor finding by label and names the resulting AC change. Required for ACs with extensive critique rounds; optional otherwise (`git log` on the AC file carries the same info for short cycles).
- **`## Director Review`** (top-level, between `## Documentation Updates` and `## Status`) — lists every viable-options trade-off chosen during the cycle (not just ones Operator feels uncertain about). Editor's final-round `Director attention` field cross-checks that this list is exhaustive and surfaces omissions.

## Lifecycle

The `## Critique` section is part of the AC file itself. The entire AC (including its critique history) is deleted at release prep alongside other per-AC artifacts.

## Termination

The director reviews the AC after Editor's verdict `no blockers` lands. Director may redirect (new round opens as `Round N+1`), accept with changes, or authorize implementation.
