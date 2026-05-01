# Drift Scan

When the user invokes `drift-scan <repo-path>`, run `governa drift-scan <repo-path>` and fill the staged AC's `<!-- TBD by Operator -->` placeholders per the rules below.

## Protocol

- The tool walks canon, byte-compares each governed file against the target, classifies divergences, collects evidence, computes next-AC and next-IE numbers, and emits a markdown report. When `<target>/plan.md` and `<target>/docs/` both exist, it also stages a partially-filled AC stub (`<target>/docs/ac<N>-drift-scan-from-<short-sha>.md`) and inserts IE entries into `<target>/plan.md`.
- One repo per invocation. The tool makes no commits in the target.
- Assume the user has asserted the path is an adopted-governa repo. The tool refuses to run against the governa source itself.

## What the tool emits

The staged AC arrives with these sections already filled — no Operator action required:

- **Title** — `# AC<N> Drift-Scan from governa @ <short-sha>`.
- **`## In Scope`** — clear-sync items if any, else `None`.
- **`## Out Of Scope`** — preserve-marker citations verbatim from `<target>/CHANGELOG.md` or `<target>/docs/ac*.md`.
- **`## Implementation Notes`** — per-file outcomes; for divergent files, full `diff -u` hunks, every commit returned by `git log -n 5 --follow`, and SHA-pinned canon refs. Two sub-subsections: `### Match evidence` (one bullet per match-classified file naming the comparison command) and `### Warnings` (`missing-in-target` / `target-has-no-canon` files surfaced by the scan).
- **`## Acceptance Tests`** — one tool-generated AT per preserve marker (regex check against `CHANGELOG.md`) and one per inserted IE (regex check against `plan.md`).
- **`## Documentation Updates`** — standard `CHANGELOG.md` placeholder line.
- **`## Director Review`** — body is exactly `None`. Ambiguity decisions live as separate shape-(a) IEs in `plan.md`, not in this AC's Director Review (see *Divergence classification* below).
- **`## Status`** — body is exactly `` `PENDING` — awaiting Director critique. ``.

`plan.md` arrives with one shape-(b) IE pointing to the staged AC, plus one shape-(a) IE per ambiguity-classified file. Insertion happens after the highest existing `IE<M>` entry, or replaces the `(none active)` placeholder if that's the convention in use.

## What the Operator fills

Three sections in the staged AC carry `<!-- TBD by Operator -->` placeholders:

- **`## Summary`** — one paragraph; if every divergent file routes to a preserve marker or an ambiguity IE (`## In Scope` is `None`), state explicitly that the AC ships only itself plus the staged `plan.md` IE entries (no file edits).
- **`## Objective Fit`** — answer the four questions per `docs/ac-template.md`.
- **`### Post-merge coherence audit`** (sub-subsection of `## Implementation Notes`) — mentally apply each canonical replacement, surface contradictions / redundancies / self-references, attribute each as either pre-existing in canon (point at a follow-up governa-side AC) or introduced by this change (resolve before staging).

The Operator may also refine the shape-(a) IE one-liners in `plan.md` if a richer rationale than "see <ac-path> for diff and classification" is warranted.

## Divergence classification

The tool emits one of the four classifications below for every file. The Operator can override by editing the staged AC before commit, and should re-route the file in `## In Scope` / `## Out Of Scope` accordingly.

- **`match`** — canon and target byte-equal. Listed under `### Match evidence`.
- **`preserve`** — a verbatim preserve-marker phrase was found citing this file in `<target>/CHANGELOG.md` or `<target>/docs/ac*.md`. Routed to `## Out Of Scope` with the marker quoted verbatim.
- **`ambiguity`** — local commits exist for this file (`git log -n 5 --follow` returned ≥ 1 commit) but no preserve marker was found. Routed to a separate shape-(a) IE in `plan.md`. Not added to this AC's `## Director Review`. Not softened with "could be intentional" in `## Out Of Scope`.
- **`clear-sync`** — divergent with neither local commits nor preserve marker. Routed to this AC's `## In Scope`.

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

For every `match`-classified file, the staged AC's `### Match evidence` sub-subsection names the comparison method — typically `byte-equal (canon @ <sha> vs <relpath>)`. Do not assert `match` without naming the check.

## Refinement tracing

When canonical text overwrites a section the target touched recently (check `git log -n 5 --follow -- <file>`), call out in `## Implementation Notes` which local wording is preserved verbatim in canon vs which is superseded. The Operator does this during their review of the staged AC's diff hunks.

## Small-drift simplification

When drift is one or two lines across one or two files: state this in the Summary, keep sections proportional. The Operator can leave Out Of Scope / Director Review as `None` and minimize ATs. The AC content requirements above still apply. Do not pad to look complete.

When every divergent file routes to a preserve marker or an ambiguity IE — `In Scope` is `None` — state in the Summary that the AC ships only itself plus the staged `plan.md` IE entries (no file edits).
