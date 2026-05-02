# Drift Scan

When the user invokes `drift-scan <repo-path>`, run `governa drift-scan <repo-path>` and fill the staged AC's `<!-- TBD by Operator -->` placeholders per the rules below.

## Protocol

- The tool walks canon, byte-compares each governed file against the target, classifies divergences, collects evidence, computes next-AC and next-IE numbers, and emits a markdown report. When `<target>/plan.md` and `<target>/docs/` both exist, it also stages a partially-filled AC stub plus a sister diffs file (see `## Staged artifacts`) and inserts an IE entry into `<target>/plan.md`.
- Before any canon→target walk, the tool runs the `## Canon-coherence precondition` check. If canon is internally incoherent on a registered cross-file rule, the tool refuses to emit and reports the incoherence on stdout. No target writes occur.
- One repo per invocation. The tool makes no commits in the target.
- Assume the user has asserted the path is an adopted-governa repo. The tool refuses to run against the governa source itself.

## Staged artifacts

When prerequisites exist, the tool stages two files in `<target>/docs/`:

- **`ac<N>-drift-scan-from-<short-sha>.md`** — the AC stub (decision document). Carries routing summary table, per-file blocks (classification, canon ref, preserve markers, coupled local-only files, commit list), Director Review with one numbered question per ambiguity-or-target-has-no-canon file, ATs scoped to In Scope only.
- **`ac<N>-drift-scan-from-<short-sha>-diffs.md`** — the sister diffs file (verification material). Carries one `## <relpath>` section per divergent file with the verbatim `diff -u` hunk. The AC's `## Implementation Notes` opens with a cross-ref line pointing at the sister.

Both files share the `docs/ac<N>-*.md` prefix, so the existing release-prep wildcard deletes them together at AC-ship time — no special handling required.

## What the tool emits

The staged AC arrives with these sections already filled — no Operator action required:

- **Title** — `# AC<N> Drift-Scan from governa @ <short-sha>`.
- **`## In Scope`** — clear-sync items + missing-in-target files whose canon is non-empty (routed as `create from canon`) + format-defining files (auto-routed to sync per `## Format-defining files`); else `None`. When `## Director Review` has at least one open question, the body is preceded by a header note: `_In Scope expands as Director resolves Q1–Q<N>. Sync resolutions add a sync line here; preserve resolutions add a CHANGELOG marker-backfill line here at the same time. See `docs/drift-scan.md ## Resolution protocol`._` When the In Scope body is otherwise `None` (every divergent file is either preserved or pending Director classification) and Director Review has open Qs, the header note replaces the `None` body — the note is the body. When Director Review has no open Qs and In Scope is otherwise empty, the body is `None — this AC ships only the staged plan.md IE entry; nothing to verify in target.`
- **`## Out Of Scope`** — preserve-marker citations verbatim from `<target>/CHANGELOG.md` or `<target>/docs/ac*.md`.
- **`## Implementation Notes`** — opens with a `Canon: governa @ <sha>, flavor <f>` line, then `Counts: ...` tally, a one-line scan asymmetry note ("scan walks canon→target only..."), and a sister-file cross-ref line ("Per-file diffs: `docs/ac<N>-...-diffs.md`"). Sub-subsections (each emitted only when it has content):
  - `### Routing summary` — first sub-subsection. Markdown table with columns `File | Local edit source | What diverged | Operator lean (as of staging)`, one row per divergent file. Tool fills `File` and `Local edit source` (most-recent local commit subject; `—` for clear-sync files that have no commit history); the last two cells are Operator-fill placeholders. Directly under the heading, before the table, the tool emits a one-line stamp: `_Operator lean below reflects staging-time analysis. Director-resolved routing lives in the Director Review section below; this table does not auto-update on resolution._`
  - `### Format-defining file routing` — emitted when any registry file is divergent. Names each format-defining file with rationale (the staged AC's auto-emitted form would contradict consumer-local divergent form). See `## Format-defining files`.
  - `### Match evidence` — one bullet per `match` file naming the comparison command (byte-equal only).
  - `### Expected per-repo divergence` — files whose canon is a stub by design and whose path is registered (see `## Expected-divergence registry`); kept separate from byte-equal matches so the Operator does not misread "match" as "verified canonical".
  - `### Divergent files` — `preserve` / `ambiguity` / `clear-sync` files with classification, canon ref, preserve markers, `Coupled-with: <list>` line, and verbatim commit list. **No diff hunks** — diffs live in the sister file.
  - `### Missing in target (create candidates)` — missing-in-target files with non-empty canon; carries canon ref + content preview so the Operator does not need to leave the AC.
  - `### Files in target without canon` — `target-has-no-canon` files (the file exists in target and in the OTHER flavor's canon — possible flavor mismatch). Carries content preview (first/last lines) and the canon path the file would map to under the other flavor. Each of these files also gets a Director Review Q with options `keep / delete / migrate-to-canon` (see `## Decision-surface coverage`).
  - `### Coupled sets (informational — routing decisions per Q above)` — emitted when at least one coupling has been detected. Lead-in stamp directly under the heading: `_The list below names files linked by build-relationship or name-reference signal. It is informational. Routing decisions are made per-file in the Director Review questions above._` Body: each coupled set as a bullet naming the signal that produced it (e.g., `Go same-package: cmd/rel/main.go, cmd/rel/color.go, cmd/rel/main_test.go`; `Shell→binary: rel.sh → cmd/rel/main.go`; `Name-reference: README.md mentions index.md`). The subsection is descriptive, not prescriptive — no language like `route together`, `should`, `must`, `consider as a unit` survives in emission.
  - `### Warnings` — only missing-in-target with empty canon (rare; informational).
- **`## Acceptance Tests`** — one AT scaffold per file in `## In Scope` (clear-sync sync ATs and missing-in-target create ATs). Missing-in-target ATs use a byte-equality check that embeds canon content (target file matches canon byte-for-byte). When `## In Scope` is `None`, body is `None — this AC ships only the staged plan.md IE entry; nothing to verify in target.` Tool no longer emits AT-for-preserve-marker or AT-for-IE-pointer (both verified scaffolding placed by earlier ACs / by this scan's staging step, not this AC's deliverable).
- **`## Documentation Updates`** — standard `CHANGELOG.md` placeholder line.
- **`## Director Review`** — auto-populated with one numbered routing question per ambiguity file plus one per `target-has-no-canon` file. Q text carries an informational `Coupled-with: <file-list>` annotation when applicable, but no "must route together" assertion or any other routing-constraint claim — coupling is informational; routing is per-file. Operator-lean placeholder is `<!-- TBD by Operator -->`. Format-defining files (registered) emit no Director Review Q — they are auto-routed to In Scope as sync. When no Q-emitting classifications fire, body is `None.`.
- **`## Status`** — body is exactly `` `PENDING` — awaiting Director critique. ``.

`plan.md` arrives with a single AC-pointer IE pointing to the staged AC. Insertion happens after the highest existing `IE<M>` entry, or replaces the `(none active)` placeholder if that's the convention in use. The AC carries the burden of detailing all per-file findings — separate IEs are not emitted per ambiguity.

## What the Operator fills

Five Operator-fill placeholders in the staged AC. Punting any of them to handoff is not a valid state — the Director critique starts from the AC, and a half-filled AC forces critique-as-completion-review.

- **`## Summary`** — one paragraph; if `## In Scope` is `None` (every divergent file is either preserved or pending Director classification), state explicitly that the AC ships only itself plus the staged `plan.md` IE entry (no file edits).
- **`## Objective Fit`** — answer the three concepts (Outcome / Priority / Dependencies) per `docs/ac-template.md`.
- **`### Post-merge coherence audit`** (sub-subsection of `## Implementation Notes`) — mentally apply each canonical replacement, surface contradictions / redundancies / self-references, attribute each as either pre-existing in canon (point at a follow-up governa-side AC) or introduced by this change (resolve before staging).
- **`## Director Review` Operator-lean entries** — for each routing question, read the file's local commit + canon and write a recommendation (sync / preserve / defer; for `target-has-no-canon` files, keep / delete / migrate-to-canon) plus a one-line why. Punting all leans to `<!-- TBD by Operator -->` is not a valid handoff state; the Director's job is to confirm or override leans, not to derive them.
- **`### Routing summary` table cells** — fill `What diverged` (one-line characterization, not a mechanical count) and `Operator lean (as of staging)` (sync / preserve / defer) for each divergent file. The cells anchor the Director's review on the routing decision surface; leaving them blank pushes the analysis back onto the Director.

## Divergence classification

The tool emits one of the classifications below for every file. The Operator can override by editing the staged AC before commit, and should re-route the file in `## In Scope` / `## Out Of Scope` accordingly.

- **`match`** — canon and target byte-equal. Listed under `### Match evidence`.
- **`expected-divergence`** — canon is a per-repo stub by design and the file's path is in the `ExpectedDivergencePaths` registry (see `## Expected-divergence registry`); the tool skips the byte-compare and lists the file under `### Expected per-repo divergence`. Treated as no-action.
- **`preserve`** — a verbatim preserve-marker phrase was found citing this file in `<target>/CHANGELOG.md` or `<target>/docs/ac*.md`. Routed to `## Out Of Scope` with the marker quoted verbatim.
- **`ambiguity`** — local commits exist for this file (`git log -n 5 --follow` returned ≥ 1 commit) but no preserve marker was found. The file's commits appear under `### Divergent files`; the Director routes it via the auto-populated `## Director Review` entry. Format-defining files (see `## Format-defining files`) are an exception: they are hard-routed to sync regardless of classification, so they emit no Director Review Q. Not softened with "could be intentional" in `## Out Of Scope`.
- **`clear-sync`** — divergent with neither local commits nor preserve marker. Routed to this AC's `## In Scope` as `sync to canon`.
- **`missing-in-target`** — canon ships the file; target does not. If canon is non-empty, routed to `## In Scope` as `create from canon` and detailed under `### Missing in target (create candidates)` with a content preview. The auto-emitted AT is a byte-equality check against canon content. If canon is empty, listed under `### Warnings` only.
- **`target-has-no-canon`** — file exists in target, NOT in canon for this flavor, but DOES exist in the other flavor's canon. Listed under `### Files in target without canon` with content preview and other-flavor canon path. Each such file gets a Director Review Q with options `keep / delete / migrate-to-canon` (see `## Decision-surface coverage`).

For every divergent file, the staged AC's `## Implementation Notes` carries:

1. The verbatim preserve-marker citations (if any) — every line that matched a recognized phrase.
2. Every commit returned by `git log -n 5 --follow -- <file>`. Verbatim, not abridged.
3. A `Coupled-with: <list>` line surfacing files coupled by the unified coupling rule (see `## Coupling analysis`).

The full `diff -u` hunk lives in the sister file `docs/ac<N>-drift-scan-from-<short-sha>-diffs.md`, not in the AC body — see `## Staged artifacts`.

## Format-defining files

The `FormatDefiningCanonPaths` registry lists canon files whose content defines the form of a section the staged AC itself emits.

**Inclusion criterion:** A file belongs in this registry iff its content defines the form of a section the staged AC itself emits — i.e., divergence in the file would make the staged AC's own text contradict canon's specification of that form. Importance, frequency-of-edit, or being-a-template are not sufficient on their own.

**Initial registry:** `docs/ac-template.md`, `docs/critique-protocol.md`.

**Hard-route-to-sync rule:** when any registry file is divergent (any classification other than `match` or `expected-divergence`), the file is auto-routed into `## In Scope` as a sync action regardless of its raw classification. The Director Review Q for these files is suppressed; the routing is forced. The staged AC carries a `### Format-defining file routing` sub-subsection under `## Implementation Notes` naming each one with the rationale: the staged AC's auto-emitted form already adopts canon's form for this file; routing as anything other than sync would leave the AC self-contradictory.

**Note on the auto-emitted Director Review form:** the staged AC's `## Director Review` block uses the form prescribed by canon's `docs/ac-template.md`. Because that file is in this registry and force-synced when divergent, consumer-local divergence is reconciled every drift-scan run; the staged AC's own form therefore stays consistent with canon's form by construction.

**Addition criterion:** a future canon file is added to the registry when (and only when) it passes the inclusion test above. Importance, frequency-of-edit, or being-a-template are not sufficient on their own.

## Expected-divergence registry

The `ExpectedDivergencePaths` registry lists canon files that are per-repo stubs by design — files whose canon content is a placeholder and whose target content is expected to diverge. The tool skips the byte-compare for these paths and classifies them as `expected-divergence`.

**Initial registry:** `plan.md`.

**Extension:** future per-repo stubs are added to the registry in the same code change that introduces them. The registry can be per-flavor if a stub is flavor-specific.

## Canon-coherence precondition

Before any canon→target walk, the tool checks canon for internal coherence on a set of registered cross-file rules. The check is canon-only — it does not read the target.

**Authoritative source:** `AGENTS.md` (governa root and base overlay) is authoritative for any rule it describes. Overlay templates and other canon files must conform.

**Behavior on incoherence:** the tool **hard-fails** — refuses to emit, exits non-zero, and writes nothing to the target.

- **Channel:** the structured report replaces what would have been the staged-AC summary on stdout. H1 reads `# Canon-Coherence Precondition Failed` so consumer agents reading drift-scan stdout route on H1.
- **Report content:** for each incoherent rule — the rule name, every conflicting canon path with line numbers and conflicting text, the authoritative source per AGENTS.md, the canonical wording the conflicting sites must conform to.
- **Framing:** the report opens with one line stating this is a **governa-side** defect requiring canon reconciliation, not a consumer-side routing decision. Consumer Director's action is "ping governa maintainer," not "route a divergence."
- **Enumerate, don't bail:** when multiple rules are simultaneously incoherent, the precondition surfaces all of them in one report. Failing at the first hit forces reconcile-rerun thrash.
- **Fire early:** the precondition runs canon-only and does not need the target. It runs before the canon→target walk so canon-side defects surface in seconds, not after a full target enumeration.
- **No target writes:** nothing under `<target>/docs/` is staged, no IE inserted into `<target>/plan.md`, no sister diffs file. The precondition runs before any target write, so nothing to roll back.

The check is registry-driven: rule definitions live next to `FormatDefiningCanonPaths`, so adding future cross-file rules extends the check.

## Resolution protocol

When the Director resolves a Director Review question, the Operator applies the protocol below. The tool stages once; the Operator updates the AC body on resolution.

- **`sync` resolution:** file moves into `## In Scope` as a sync action. AT auto-extends with a byte-equality check against canon content. Resolution attributed inline with `(Director-set)` per `docs/ac-template.md`'s convention.
- **`preserve` resolution:** file stays in `## Out Of Scope` with the marker named. CHANGELOG marker-backfill action lands in `## In Scope` as `add preserve marker for <path> in CHANGELOG.md row at next release prep`. Resolution attributed inline.
- **`defer` resolution:** file stays in routing queue if more critique rounds are expected, or moves to a follow-on AC pointer added to `plan.md` as a new IE. Resolution attributed inline.
- **`target-has-no-canon` resolutions:** `keep` leaves the file as a per-repo addition (no AC action); `delete` adds a removal line to `## In Scope`; `migrate-to-canon` adds a follow-on IE pointing at a governa-side AC to introduce the file into canon.

The Operator applies the protocol on resolution; the tool does not auto-update the AC after staging.

## Decision-surface coverage

Every classification that requires a Director call pairs with an auto-emitted decision surface (a `## Director Review` Q). Classifications that are terminal — no Director call needed — emit no Q.

| Classification | Director Review Q | Note |
|---|---|---|
| `match` | No | Terminal — byte-equal |
| `expected-divergence` | No | Terminal — registry-controlled |
| `preserve` | No | Terminal — marker already routed |
| `clear-sync` | No | Terminal — auto-routed to In Scope |
| `missing-in-target` | No | Terminal — auto-routed to In Scope (or to Warnings) |
| `ambiguity` | Yes | Routing decision: sync / preserve / defer |
| `target-has-no-canon` | Yes | Routing decision: keep / delete / migrate-to-canon |
| Format-defining (registered) | No | Hard-routed to sync regardless of raw classification |

## Coupling analysis

For every divergent file (classifications `preserve`, `ambiguity`, `clear-sync`), the tool enumerates files coupled to it. The unified coupling rule is applied at all depths — repo root, subdirectories, anywhere.

**Coupling rule:** file `F` is coupled to file `G` iff one of:

- **(a) Build-relationship signal** — `F` and `G` participate in the same build artifact:
  - **Go same-package:** both files declare the same `package X` (parsed from their package declaration).
  - **Shell→binary:** when a `*.sh` script in the divergent set contains `go run <pkg>` and `<pkg>` resolves to another file in the divergent set, the script and the package files are unioned into one routing group.
- **(b) Name-reference body scan** — `F`'s content references `G` by repo-relative path or basename (substring match, no extension stripping). False positives (e.g., `README.md` mentions `index.md` in passing) are accepted as a heuristic limitation.

**Directory-sibling enumeration is no longer used as a coupling signal.** It was too coarse at any depth — heterogeneous content lives next to each other in many repo layouts (Jekyll site files at root next to release tooling, governance docs next to subcommand source, etc.). Sharing a directory does not imply build coupling.

**Output:** each divergent file's per-file block carries a `Coupled-with: <list>` (or `Coupled-with: None`) line. The `### Coupled sets` reading-aid sub-subsection under `## Implementation Notes` summarizes the coupling graph as informational bullets naming the signal that produced each set.

`## Director Review` emits one numbered routing question per ambiguity file (not per coupled set). Coupling is surfaced informationally in the Q text (`Coupled-with:`) and in the `### Coupled sets` reading aid; routing is per-file and survives lean-split.

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

When `In Scope` is otherwise `None` (every divergent file is either preserved or pending Director classification): state in the Summary that the AC ships only itself plus the staged `plan.md` IE entry (no file edits). If `## Director Review` has at least one open question, the In Scope header note replaces the `None` body — the note is the body. If Director Review is also `None`, the In Scope body remains `None — this AC ships only the staged plan.md IE entry; nothing to verify in target.`

## Handoff

After staging completes and the Operator has filled the placeholders, the report to the Director is a terse handoff — not a findings recap. The AC carries the findings; the chat message redirects.

> Staged `docs/ac<N>-drift-scan-from-<short-sha>.md` in repo `<repo-name>`. Governa's part in this drift-scan run is complete. Please work with the `<repo-name>` agent there and follow AC cadence.

Substitute the actual filename and repo name. Do not summarize the divergences, paste classification counts, or pad with next-step suggestions.
