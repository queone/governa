# Drift Scan

When the user invokes `drift-scan <repo-path>`, run `governa drift-scan <repo-path>` and fill the staged AC's `<!-- TBD by Operator -->` placeholders per the rules below.

## Protocol

- The tool walks canon, byte-compares each governed file against the target, classifies divergences, collects evidence, computes next-AC and next-IE numbers, and emits a markdown report. When `<target>/plan.md` and `<target>/docs/` both exist, it also stages a partially-filled AC stub (`<target>/docs/ac<N>-drift-scan-from-<short-sha>.md`) and inserts IE entries into `<target>/plan.md`.
- One repo per invocation. The tool makes no commits in the target.
- Assume the user has asserted the path is an adopted-governa repo. The tool refuses to run against the governa source itself.

## What the tool emits

The staged AC arrives with these sections already filled — no Operator action required:

- **Title** — `# AC<N> Drift-Scan from governa @ <short-sha>`.
- **`## In Scope`** — clear-sync items + missing-in-target files whose canon is non-empty (routed as `create from canon`); else `None`.
- **`## Out Of Scope`** — preserve-marker citations verbatim from `<target>/CHANGELOG.md` or `<target>/docs/ac*.md`.
- **`## Implementation Notes`** — opens with a `Counts: ...` tally line; for divergent files, full `diff -u` hunks, every commit returned by `git log -n 5 --follow` (adoption-style commits annotated with `(adoption)`), and SHA-pinned canon refs. Sub-subsections (each emitted only when it has content):
  - `### Match evidence` — one bullet per `match` file naming the comparison command (byte-equal only).
  - `### Expected per-repo divergence` — files whose canon is a stub by design (e.g., `plan.md`); kept separate from byte-equal matches so the Operator does not misread "match" as "verified canonical".
  - `### Divergent files` — `preserve` / `ambiguity` / `clear-sync` files with full diff + commit history.
  - `### Missing in target (create candidates)` — missing-in-target files with non-empty canon; carries canon ref + content preview so the Operator does not need to leave the AC.
  - `### Files in target without canon` — `target-has-no-canon` files (the file exists in target and in the OTHER flavor's canon — possible flavor mismatch).
  - `### Warnings` — only missing-in-target with empty canon (rare; informational).
- **`## Acceptance Tests`** — one tool-generated AT per preserve marker (literal-string check against the FULL marker line in `CHANGELOG.md`, not just the short phrase) and one for the inserted IE (literal-string check against the full shape-(b) line in `plan.md`).
- **`## Documentation Updates`** — standard `CHANGELOG.md` placeholder line.
- **`## Director Review`** — when the scan found ambiguities, auto-populated with one numbered routing question per ambiguity (`Should <file> be synced to canon, preserved with a marker, or deferred?` plus Operator-lean placeholder). When there are no ambiguities, body is `None.`.
- **`## Status`** — body is exactly `` `PENDING` — awaiting Director critique. ``.

`plan.md` arrives with a single shape-(b) IE pointing to the staged AC. Insertion happens after the highest existing `IE<M>` entry, or replaces the `(none active)` placeholder if that's the convention in use. The AC carries the burden of detailing all per-file findings — separate IEs are not emitted per ambiguity.

## What the Operator fills

Three sections in the staged AC carry `<!-- TBD by Operator -->` placeholders:

- **`## Summary`** — one paragraph; if `## In Scope` is `None` (every divergent file is either preserved or pending Director classification), state explicitly that the AC ships only itself plus the staged `plan.md` IE entry (no file edits).
- **`## Objective Fit`** — answer the four questions per `docs/ac-template.md`.
- **`### Post-merge coherence audit`** (sub-subsection of `## Implementation Notes`) — mentally apply each canonical replacement, surface contradictions / redundancies / self-references, attribute each as either pre-existing in canon (point at a follow-up governa-side AC) or introduced by this change (resolve before staging).

## Divergence classification

The tool emits one of the classifications below for every file. The Operator can override by editing the staged AC before commit, and should re-route the file in `## In Scope` / `## Out Of Scope` accordingly.

- **`match`** — canon and target byte-equal. Listed under `### Match evidence`.
- **`expected-divergence`** — canon is a per-repo stub by design (currently `plan.md`); the tool skips the byte-compare and lists the file under `### Expected per-repo divergence`. Treated as no-action.
- **`preserve`** — a verbatim preserve-marker phrase was found citing this file in `<target>/CHANGELOG.md` or `<target>/docs/ac*.md`. Routed to `## Out Of Scope` with the marker quoted verbatim.
- **`ambiguity`** — local commits exist for this file (`git log -n 5 --follow` returned ≥ 1 commit) but no preserve marker was found. The file's diff and commits appear under `### Divergent files`; the Director routes it via the auto-populated `## Director Review` entry. Not softened with "could be intentional" in `## Out Of Scope`.
- **`clear-sync`** — divergent with neither local commits nor preserve marker. Routed to this AC's `## In Scope` as `sync to canon`.
- **`missing-in-target`** — canon ships the file; target does not. If canon is non-empty, routed to `## In Scope` as `create from canon` and detailed under `### Missing in target (create candidates)` with a content preview. If canon is empty, listed under `### Warnings` only.
- **`target-has-no-canon`** — file exists in target, NOT in canon for this flavor, but DOES exist in the other flavor's canon. Listed under `### Files in target without canon` so the Director can confirm flavor selection or accept the file as a per-repo addition.

For every divergent file, the staged AC's `## Implementation Notes` carries:

1. The verbatim preserve-marker citations (if any) — every line that matched a recognized phrase.
2. Every commit returned by `git log -n 5 --follow -- <file>`. Verbatim, not abridged.
3. The full `diff -u` hunk (truncated to the configured `-l|--diff-lines`). The diff hunk is the SHA-pinned canon-anchor source; no separate canon-snippet emission is needed.

## Preserve-marker phrase set

The tool recognizes exactly the four phrases below in `<target>/CHANGELOG.md` table rows or `<target>/docs/ac*.md` content. Implicit AC references locking the local form (e.g., `migrate <x> to <path>`, `<path> from governa overlay`) **do not** count — the tool will misclassify those files as `ambiguity` until the row is backfilled with an explicit marker.

| Phrase | Example |
|---|---|
| `preserve <path> <qualifier>` | `preserve docs/release.md customization` |
| `do not sync <path>` | `do not sync docs/local-overrides.md` |
| `intentional divergence: <path>` | `intentional divergence: rel.sh` |
| `<path>: keep local` | `docs/team-rituals.md: keep local` |

When shipping an AC that locks a local form against canon, include one of these phrases in the CHANGELOG row alongside whatever else the row says. See `docs/release.md` (consumer overlay) for the rule.

## Match evidence

For every `match`-classified file, the staged AC's `### Match evidence` sub-subsection names the comparison method — `byte-equal (canon @ <sha> vs <relpath>)`. Files whose canon is a per-repo stub appear under `### Expected per-repo divergence` instead, with a note explaining the divergence is by design.

## Refinement tracing

When canonical text overwrites a section the target touched recently (check `git log -n 5 --follow -- <file>`), call out in `## Implementation Notes` which local wording is preserved verbatim in canon vs which is superseded. The Operator does this during their review of the staged AC's diff hunks.

## Small-drift simplification

When drift is one or two lines across one or two files: state this in the Summary, keep sections proportional. The Operator can leave Out Of Scope / Director Review as `None` and minimize ATs. The AC content requirements above still apply. Do not pad to look complete.

When `In Scope` is `None` (every divergent file is either preserved or pending Director classification), state in the Summary that the AC ships only itself plus the staged `plan.md` IE entry (no file edits).

## Handoff

After staging completes and the Operator has filled the placeholders, the report to the Director is a terse handoff — not a findings recap. The AC carries the findings; the chat message redirects.

> Staged `docs/ac<N>-drift-scan-from-<short-sha>.md` in repo `<repo-name>`. Governa's part in this drift-scan run is complete. Please work with the `<repo-name>` agent there and follow AC cadence.

Substitute the actual filename and repo name. Do not summarize the divergences, paste classification counts, or pad with next-step suggestions.
