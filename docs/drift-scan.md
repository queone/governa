# Drift Scan

When the user invokes `drift-scan <repo-path>`, run `governa drift-scan <repo-path>` and fill the staged AC's `<!-- TBD by Operator -->` placeholders per the rules below.

## Protocol

- The tool walks canon, byte-compares each governed file against the target, classifies divergences, collects evidence, computes next-AC and next-IE numbers, and emits a markdown report. When `<target>/plan.md` and `<target>/docs/` both exist, it also stages a partially-filled AC stub plus a sister diffs file (see `## Staged artifacts`) and inserts an IE entry into `<target>/plan.md`.
- One repo per invocation. The tool makes no commits in the target.
- Assume the user has asserted the path is an adopted-governa repo. The tool refuses to run against the governa source itself.

## Staged artifacts

When prerequisites exist, the tool stages two files in `<target>/docs/`:

- **`ac<N>-drift-scan-from-<short-sha>.md`** — the AC stub (decision document). Carries routing summary table, per-file blocks (classification, canon ref, preserve markers, coupled local-only files, commit list), Director Review with grouped routing questions, ATs scoped to In Scope only.
- **`ac<N>-drift-scan-from-<short-sha>-diffs.md`** — the sister diffs file (verification material). Carries one `## <relpath>` section per divergent file with the verbatim `diff -u` hunk. The AC's `## Implementation Notes` opens with a cross-ref line pointing at the sister.

Both files share the `docs/ac<N>-*.md` prefix, so the existing release-prep wildcard deletes them together at AC-ship time — no special handling required.

## What the tool emits

The staged AC arrives with these sections already filled — no Operator action required:

- **Title** — `# AC<N> Drift-Scan from governa @ <short-sha>`.
- **`## In Scope`** — clear-sync items + missing-in-target files whose canon is non-empty (routed as `create from canon`); else `None`.
- **`## Out Of Scope`** — preserve-marker citations verbatim from `<target>/CHANGELOG.md` or `<target>/docs/ac*.md`.
- **`## Implementation Notes`** — opens with a `Canon: governa @ <sha>, flavor <f>` line, then `Counts: ...` tally, a one-line scan asymmetry note ("scan walks canon→target only..."), and a sister-file cross-ref line ("Per-file diffs: `docs/ac<N>-...-diffs.md`"). Sub-subsections (each emitted only when it has content):
  - `### Routing summary` — first sub-subsection. Markdown table with columns `File | Local edit source | What diverged | Recommendation`, one row per divergent file. Tool fills `File` and `Local edit source` (most-recent local commit subject; `—` for clear-sync files that have no commit history); the last two cells are Operator-fill placeholders.
  - `### Match evidence` — one bullet per `match` file naming the comparison command (byte-equal only).
  - `### Expected per-repo divergence` — files whose canon is a stub by design (e.g., `plan.md`); kept separate from byte-equal matches so the Operator does not misread "match" as "verified canonical".
  - `### Divergent files` — `preserve` / `ambiguity` / `clear-sync` files with classification, canon ref, preserve markers, `Coupled local-only files: <list>` line, and verbatim commit list. **No diff hunks** — diffs live in the sister file.
  - `### Missing in target (create candidates)` — missing-in-target files with non-empty canon; carries canon ref + content preview so the Operator does not need to leave the AC.
  - `### Files in target without canon` — `target-has-no-canon` files (the file exists in target and in the OTHER flavor's canon — possible flavor mismatch).
  - `### Warnings` — only missing-in-target with empty canon (rare; informational).
- **`## Acceptance Tests`** — one AT scaffold per file in `## In Scope` (clear-sync sync ATs and missing-in-target create ATs). When `## In Scope` is `None`, body is `None — this AC ships only the staged plan.md IE entry; nothing to verify in target.` Tool no longer emits AT-for-preserve-marker or AT-for-IE-pointer (both verified scaffolding placed by earlier ACs / by this scan's staging step, not this AC's deliverable).
- **`## Documentation Updates`** — standard `CHANGELOG.md` placeholder line.
- **`## Director Review`** — auto-populated with one numbered routing question per coupled set containing at least one ambiguity. Coupled-set grouping (see `## Coupling analysis`) collapses files that share a local-only sibling or are linked by shell→binary `go run` so the Director routes them together, not file-by-file. Operator-lean placeholder is `<!-- TBD by Operator -->`. When no ambiguity files exist, body is `None.`.
- **`## Status`** — body is exactly `` `PENDING` — awaiting Director critique. ``.

`plan.md` arrives with a single AC-pointer IE pointing to the staged AC. Insertion happens after the highest existing `IE<M>` entry, or replaces the `(none active)` placeholder if that's the convention in use. The AC carries the burden of detailing all per-file findings — separate IEs are not emitted per ambiguity.

## What the Operator fills

Five Operator-fill placeholders in the staged AC. Punting any of them to handoff is not a valid state — the Director critique starts from the AC, and a half-filled AC forces critique-as-completion-review.

- **`## Summary`** — one paragraph; if `## In Scope` is `None` (every divergent file is either preserved or pending Director classification), state explicitly that the AC ships only itself plus the staged `plan.md` IE entry (no file edits).
- **`## Objective Fit`** — answer the four questions per `docs/ac-template.md`.
- **`### Post-merge coherence audit`** (sub-subsection of `## Implementation Notes`) — mentally apply each canonical replacement, surface contradictions / redundancies / self-references, attribute each as either pre-existing in canon (point at a follow-up governa-side AC) or introduced by this change (resolve before staging).
- **`## Director Review` Operator-lean entries** — for each routing question, read the file's local commit + canon and write a recommendation (sync / preserve / defer) plus a one-line why. Punting all leans to `<!-- TBD by Operator -->` is not a valid handoff state; the Director's job is to confirm or override leans, not to derive them.
- **`### Routing summary` table cells** — fill `What diverged` (one-line characterization, not a mechanical count) and `Recommendation` (sync / preserve / defer) for each divergent file. The cells anchor the Director's review on the routing decision surface; leaving them blank pushes the analysis back onto the Director.

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
3. A `Coupled local-only files: <list>` line surfacing same-directory files that exist in target but not in canon (see `## Coupling analysis`).

The full `diff -u` hunk lives in the sister file `docs/ac<N>-drift-scan-from-<short-sha>-diffs.md`, not in the AC body — see `## Staged artifacts`.

## Coupling analysis

For every divergent file (classifications `preserve`, `ambiguity`, `clear-sync`), the tool enumerates **coupled local-only files**: same-directory files that exist in target but not in canon for this flavor. Per-AC files (`ac<N>-*.md`) are filtered. The list is rendered as a `Coupled local-only files: <list>` (or `Coupled local-only files: None`) line on each per-file block.

Rule of thumb: for every ambiguity file, the staged AC must surface the local-only files that exist because of the local divergence — they ride along with the routing decision. The directory-sibling rule covers the AC4-tips case (`cmd/rel/color.go`, `cmd/rel/main_test.go` ride with `cmd/rel/main.go`).

In addition, the tool detects a coarse **shell→binary** coupling: when a `*.sh` script in the divergent set contains `go run <pkg>` and `<pkg>` resolves to another file in the divergent set, the script and the package files are unioned into one routing group. Single regex pass over `*.sh` only — broader scanning of `bash -c`, Makefile recipes, and `*.go` build directives is deferred until a concrete failure case shows up. Failure to match (e.g., a script invokes via `bash -c` or computes the path) falls back to directory-sibling enumeration only — that is acceptable; the goal is to surface the obvious coupling, not be exhaustive.

`## Director Review` emits one numbered routing question per coupled set (not per file). Two divergent files are in the same set iff they share at least one coupled local-only sibling, or one is the named coupled target of the other (shell→binary). Groups containing only preserve/clear-sync files are skipped — those are already routed to Out Of Scope / In Scope respectively.

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

## Format pre-emption avoidance

When an ambiguity file is itself a format or template spec — for example, `docs/ac-template.md` whose content defines the shape of Operator-authored AC sections — the staged AC must not apply the canon form of that template until the routing decision is made. Either follow the local form, or flag the format adoption as a deliberate signal of the Operator lean (and call it out in `## Director Review`).

The drift-scan tool's own auto-emitted `## Director Review` format (numbered questions ending in `?`, Operator-lean placeholder) is **canon for governa** and unchanged by this rule. The rule binds **Operator-authored** sections (Summary, Objective Fit, Post-merge coherence audit, lean entries) on consumer-repo drift-scan ACs only — when the consumer's local `docs/ac-template.md` diverges from canon and is in the routing queue, the staged AC's Operator-authored sections must follow whichever form the local target carries until the routing decision lands.

## Handoff

After staging completes and the Operator has filled the placeholders, the report to the Director is a terse handoff — not a findings recap. The AC carries the findings; the chat message redirects.

> Staged `docs/ac<N>-drift-scan-from-<short-sha>.md` in repo `<repo-name>`. Governa's part in this drift-scan run is complete. Please work with the `<repo-name>` agent there and follow AC cadence.

Substitute the actual filename and repo name. Do not summarize the divergences, paste classification counts, or pad with next-step suggestions.
