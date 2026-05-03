# Drift Scan

When the user invokes `drift-scan <repo-path>`, run `governa drift-scan <repo-path>` and fill the staged AC's `<!-- TBD by Operator -->` placeholders per the rules below.

## Protocol

- The tool walks canon, byte-compares each governed file against the target, classifies divergences, collects evidence, computes next-AC and next-IE numbers, and emits a markdown report. When `<target>/plan.md` and `<target>/docs/` both exist, it also stages a partially-filled AC stub plus a sister diffs file (see `## Staged artifacts`) and inserts an IE entry into `<target>/plan.md`.
- Before any canon→target walk, the tool runs the `## Canon-coherence precondition` check. If canon is internally incoherent on a registered cross-file rule, the tool refuses to emit and reports the incoherence on stdout. No target writes occur.
- One repo per invocation. The tool makes no commits in the target.
- Assume the user has asserted the path is an adopted-governa repo. The tool refuses to run against the governa source itself.

## Staged artifacts

When prerequisites exist, the tool stages two files in `<target>/docs/`:

- **`ac<N>-drift-scan-from-<short-sha>.md`** — the AC stub (decision document). Carries per-file blocks (classification, canon ref, preserve markers, commit list), Director Review with one numbered question per ambiguity-or-target-has-no-canon file, ATs scoped to In Scope only.
- **`ac<N>-drift-scan-from-<short-sha>-diffs.md`** — the sister diffs file (verification material). Carries one `## <relpath>` section per divergent file with the verbatim `diff -u` hunk. The AC's `## Implementation Notes` opens with a cross-ref line pointing at the sister. The sister body opens with a convention stamp (AC108 Class U): `_Diff convention: `+` lines exist in TARGET; `-` lines exist in CANON. Routing leans depend on direction — read the per-file `Direction:` summary in the AC body before drawing conclusions._`

Both files share the `docs/ac<N>-*.md` prefix, so the existing release-prep wildcard deletes them together at AC-ship time — no special handling required.

## What the tool emits

The staged AC arrives with these sections already filled — no Operator action required:

- **Title** — `# AC<N> Drift-Scan from governa @ <short-sha>`.
- **`## In Scope`** — clear-sync items + missing-in-target files whose canon is non-empty (routed as `create from canon`) + format-defining files (auto-routed to sync per `## Format-defining files`); else `None`. When the body is otherwise empty AND Director Review has at least one open Q, body is the terse one-liner `None — body lands as Director resolves Q1–Q<N>.` (resolution mechanic lives in `## Resolution protocol`; the Director Review menu lists the per-Q choice options). When Director Review has no open Qs and In Scope is otherwise empty, body is `None — this AC ships only the staged plan.md IE entry; nothing to verify in target.`
- **`## Out Of Scope`** — preserve-marker citations verbatim from `<target>/CHANGELOG.md` or `<target>/docs/ac*.md`. Empty body is `None.` regardless of Director Review state.
- **`## Implementation Notes`** — opens with a `Canon: governa @ <sha>, flavor <f>` line, then `Counts: ...` tally (AC108 Class T: when at least one file is hard-routed via the format-defining registry, the `ambiguity` count carries `(M hard-routed via format-defining)`; when at least one missing-in-target with non-empty canon is auto-routed, the `missing-in-target` count carries `(M auto-routed as create-from-canon)` — both annotations reconcile the count line with the routing-table row count), a one-line scan asymmetry note ("scan walks canon→target only..."), and a sister-file cross-ref line ("Per-file diffs: `docs/ac<N>-...-diffs.md`"). Sub-subsections (each emitted only when it has content):
  - (AC112 Class Y: `### Routing summary` table dropped. `What diverged` moved to per-file `### Divergent files` blocks where it sits under the `Direction:` line; lean lives only in `## Director Review` per-Q. Single source of truth for routing decisions.)
  - `### Format-defining file routing` — emitted when any registry file is divergent. Names each format-defining file with rationale (staged AC instantiates canon's form for these files — both tool-emitted and Operator-fill sections — so any other routing would leave the AC self-contradictory). See `## Format-defining files`.
  - `### Missing-in-target file routing` — emitted when at least one missing-in-target file with non-empty canon is auto-routed to In Scope as create-from-canon. Names each one with rationale citing AGENTS.md Approval Boundaries (the AC critique gate is the approval surface). See `## Missing-in-target file routing`.
  - `### Match evidence` — one bullet per `match` file naming the comparison command (byte-equal only).
  - `### Expected per-repo divergence` — files whose canon is a stub by design and whose path is registered (see `## Expected-divergence registry`); kept separate from byte-equal matches so the Operator does not misread "match" as "verified canonical".
  - `### Divergent files` — `preserve` / `ambiguity` / `clear-sync` files with classification, canon ref, preserve markers, optional `Coupled-with: <signal-name> set (see § Coupled sets)` line (AC108 Class R: only when the file is in a coupled set; uncoupled files emit no Coupled-with line), `Direction:` summary line (AC108 Class U: target/canon line counts and a target-leads/canon-leads/mutual qualitative label so the Operator does not have to read +/- glyphs to determine direction), `What diverged: <!-- TBD by Operator -->` Operator-fill line (AC112 Class Y: positioned after `Direction:` so the Operator reads direction first; one-line characterization of the change), and verbatim commit list. **No diff hunks** — diffs live in the sister file.
  - `### Missing in target (create candidates)` — missing-in-target files with non-empty canon; carries canon ref + content preview so the Operator does not need to leave the AC.
  - `### Files in target without canon` — `target-has-no-canon` files (the file exists in target and in the OTHER flavor's canon — possible flavor mismatch). Carries content preview (first/last lines) and the canon path the file would map to under the other flavor. Each of these files also gets a Director Review Q with options `keep / delete / migrate-to-canon` (see `## Decision-surface coverage`).
  - `### Coupled sets (informational — routing decisions per Q above)` — emitted when at least one coupling has been detected. The heading qualifier `(informational — routing decisions per Q above)` signals the section's nature; body is one bullet per coupled set naming the signal that produced it (e.g., `Go same-package: cmd/rel/main.go, cmd/rel/color.go, cmd/rel/main_test.go`; `Shell→binary: rel.sh → cmd/rel/main.go`; `Name-reference: README.md mentions index.md`). The subsection is descriptive, not prescriptive — no language like `route together`, `should`, `must`, `consider as a unit` survives in emission.
  - `### Warnings` — only missing-in-target with empty canon (rare; informational).
- **`## Acceptance Tests`** — one AT scaffold per file in `## In Scope` (clear-sync sync ATs and missing-in-target create ATs). Missing-in-target ATs use a byte-equality check that embeds canon content (target file matches canon byte-for-byte). When `## In Scope` is `None`, body is `None — this AC ships only the staged plan.md IE entry; nothing to verify in target.` Tool no longer emits AT-for-preserve-marker or AT-for-IE-pointer (both verified scaffolding placed by earlier ACs / by this scan's staging step, not this AC's deliverable).
- **`## Documentation Updates`** — standard `CHANGELOG.md` placeholder line.
- **`## Director Review`** — auto-populated with a routing-matrix shape (AC109 Class V): when at least one Q exists, the section opens with a bulleted routing-menu block (AC111 Class X human-readable form):

  ```
  **Routing menu** (pick one per Q):

  - `sync` — file moves to In Scope
  - `preserve` — file stays in Out Of Scope; backfill `preserve <path> <qualifier>` in CHANGELOG.md at next release prep
  - `defer` — file becomes a follow-on AC pointer (new IE in `plan.md`)
  - For `target-has-no-canon` files: `keep` / `delete` / `migrate-to-canon` instead. See `governa @ <sha>: docs/drift-scan.md ## Resolution protocol`.
  ```

  Followed by one numbered entry per ambiguity file (`<N>. **`<file>`** — <!-- TBD by Operator -->. Why: <!-- TBD by Operator -->.`) plus one per `target-has-no-canon` file (same shape with `(target-has-no-canon)` annotation between file and placeholder). Per-Q text is shape-only — the menu lives once at the top. Coupling info is purely informational via `### Coupled sets` — never duplicated per Q. Format-defining files (registered) emit no Director Review Q — they are auto-routed to In Scope as sync. When no Q-emitting classifications fire, body is `None.` and no menu block is emitted.

  **Tool-emission exception to `docs/ac-template.md`'s question-form rule.** `docs/ac-template.md` requires Director Review entries to "lead with a literal question ending in `?` so the Director can reference entries inline (\"Regarding #1, …\")." The tool-emitted form deviates from "lead with a literal question": entries lead with the file in backticks instead. Numbering still serves the inline-reference purpose. The question-form rule was written for human-drafted ACs where each entry is a genuinely open question; tool-emission is a routing matrix where every entry is the same shape and the menu is documented once at the top. Class-G negative-regex tests must not fire on the routing-matrix shape.
- **`## Status`** — body is exactly `` `PENDING` — awaiting Director critique. ``.

`plan.md` arrives with a single AC-pointer IE pointing to the staged AC. Insertion happens after the highest existing `IE<M>` entry, or replaces the `(none active)` placeholder if that's the convention in use. The AC carries the burden of detailing all per-file findings — separate IEs are not emitted per ambiguity.

## What the Operator fills

Five Operator-fill spots in the staged AC. Fill them in the order below. Half-filled handoff to Director critique is not a valid state — the Director critique starts from the AC, and a half-filled AC forces critique-as-completion-review.

### 1. Per-file `What diverged` (each `### Divergent files` block)

For each divergent file (preserve, ambiguity, clear-sync), fill the `What diverged: <!-- TBD by Operator -->` line with a one-line characterization of the change.

Before writing, the Operator MUST read in this order:

1. The `Direction:` line immediately above (target-leads / canon-leads / mutual N+/M-). This is mechanical: it tells you which side carries the divergence.
2. The `Local commits:` block.
3. The full diff hunk in the sister file `docs/ac<N>-...-diffs.md` if needed.

**Failure mode named:** writing "canon dropped X" when the diff shows X as `-` (canon-side, present in canon, absent in target) inverts the routing decision and the lean. The convention stamp at the top of the sister file pins the convention; reread it if uncertain.

### 2. Director Review per-Q lean + why

For each open Q in `## Director Review`, fill:

- The lean placeholder — pick from the routing menu at the top of the section (`sync` / `preserve` / `defer` for ambiguity; `keep` / `delete` / `migrate-to-canon` for `target-has-no-canon`).
- The `Why: <!-- TBD by Operator -->` rationale — one line, anchored on the `Direction:` line and the per-file `What diverged` from step 1.

**Sequencing constraint:** the Operator MUST fill step 1 before step 2. The per-file `What diverged` is the input to the lean rationale; filling leans first and characterizing diffs after invites confirmation-bias.

**Failure mode:** filling the lean without the why (or vice versa) hands the Director half a decision. The lean alone doesn't tell the Director why the Operator chose it; the why alone doesn't tell the Director what the lean is. Both fields MUST be filled together.

### 3. Post-merge coherence audit (`### Post-merge coherence audit` under `## Implementation Notes`)

Mentally apply the proposed routing AND verify cross-file invariants. AC114 Parts B+C pre-fill the audit body based on sync/preserve state — Operator content depends on which branch fired:

- **Sync ∧ preserve fires (Part B):** tool emits a checklist scaffold with rules mechanically extracted from each synced file's diff via the `imperativeRuleRe` pattern (case-insensitive `\b(must|every|requires|shall|always|never|each)\b`). Each extracted rule appears as `- [TBD] R<N>: <synced-file> adds at line <N>: <excerpt> — reconciliation: ?`. The Operator MUST replace each `[TBD]` and `?` with the reconciliation outcome (acknowledged / intentional opt-out / contradiction). The Operator MAY augment manually if the heuristic missed any rules.
- **Sync-empty (Part C):** tool emits the verbatim vacuous body `Cross-file rule reconciliation is trivially vacuous — no synced files in this AC, so no canon-side rules are introduced; preserved files cannot contradict what wasn't added.` No Operator action required.
- **Preserve-empty (Part C):** tool emits the verbatim vacuous body `Cross-file rule reconciliation is trivially vacuous — no preserved files in this AC, so no opportunity for cross-file contradiction (everything either syncs to canon or defers).` No Operator action required.

Procedure when sync ∧ preserve fires (the Operator MUST execute this on the pre-filled checklist):

1. The tool has listed synced and preserved files at the top of the audit section. Confirm the lists are complete (no manually-added In Scope/Out Of Scope items missed).
2. For each `[TBD] R<N>:` line, read the rule excerpt. If the rule is real (vs. a heuristic false positive on prose `must`/`each`/etc.), keep it; otherwise delete the line.
3. For each remaining rule, verify each preserved file either:
   - **Acknowledges** the rule (consistent with it), OR
   - **Documents an intentional opt-out** — the preserve marker explanation makes the divergence intentional.
4. Replace each `[TBD]` with `acknowledged in <preserved-file>` / `intentional opt-out per <preserve-marker>` / `contradiction — <description>`.
5. Augment with any rule the heuristic missed. **Silent contradiction is the worst outcome** — a future AT against a preserved file may fail because canon's added rule isn't reflected.

**Concrete failure to avoid (from AC4 in tips):** synced AGENTS.md + docs/ac-template.md both reintroduced the AT-label timing-axis rule; preserved docs/release.md had no timing-axis paragraph. Three files ended up disagreeing post-sync. The audit didn't catch this because the Operator narrative didn't enumerate cross-file invariants per the procedure above.

### 4. Summary

One short paragraph. Frame all routing decisions as **Operator leans pending Director resolution**, not as decided routings — the Director Review section still has open Qs at handoff. Categorize counts by routing source separately:

- hard-routed sync (format-defining registry)
- leaned sync (Operator-leaned, ambiguity Q resolved as sync at staging)
- marker-preserve (existing CHANGELOG preserve marker)
- leaned-preserve (Operator-leaned, ambiguity Q resolved as preserve at staging)
- leaned-defer (Operator-leaned, ambiguity Q resolved as defer at staging)
- create-from-canon (missing-in-target auto-routed)
- target-has-no-canon (Operator-leaned per Q type)

State each total separately. **Do not aggregate across sources.**

If `## In Scope` is `None` (every divergent file is preserved, pending Director classification, or absent from canon), state explicitly that the AC ships only itself plus the staged `plan.md` IE entry (no file edits).

**Failure mode (F1 stance):** writing "six ambiguity files preserve" while Director Review presents them as open Qs implies decisions are made. Critique then reads the Summary as Director-set and the Director Review section as residual scaffolding. Both states co-exist in the AC; the Summary MUST acknowledge the lean-pending-resolution state.

**Failure mode (F2/P3 categorization):** miscounting buckets when categories overlap (e.g., "preserved" can mean marker-preserve OR leaned-preserve; the totals diverge). Per-source separation prevents aggregation errors.

### 5. Objective Fit

Fill each numbered placeholder with content matching the form pre-filled by the tool. AC114 Part A pre-fills the scaffold by parsing target's local `docs/ac-template.md` Objective Fit section; if the target file is missing/unparseable, falls back to canon's 3-part form (`Outcome` / `Priority` / `Dependencies`). The Operator-fill content goes in each `<N>. **<heading>** <!-- TBD by Operator -->` slot. Reference dependent ACs. Name intentional contradictions explicitly.

### Handoff verification

After all five fills, run `governa drift-scan verify <ac-path>` (AC114 Part D / R4.12). The subcommand performs five structural-compliance checks:

1. No `<!-- TBD by Operator -->` substring remains anywhere.
2. Each `### Divergent files` per-file block has a non-TBD `What diverged:` line.
3. Each `## Director Review` numbered Q has a non-TBD lean.
4. Each `## Director Review` numbered Q has a non-TBD `Why:`.
5. When sync ∧ preserve indicated (per the pinned heuristic in `## Verify subcommand`), the `### Post-merge coherence audit` body has no `[TBD]` substring (Part B's checklist must be filled).

Output: each failure as `<line>:<section>: <description>` (gcc/golangci-lint format for editor jump-to-error). Exit 0 = clean, exit 1 = failures present.

Punting any spot to handoff is not a valid state.

## Divergence classification

The tool emits one of the classifications below for every file. The Operator can override by editing the staged AC before commit, and should re-route the file in `## In Scope` / `## Out Of Scope` accordingly.

- **`match`** — canon and target byte-equal. Listed under `### Match evidence`.
- **`expected-divergence`** — canon is a per-repo stub by design and the file's path is in the `ExpectedDivergencePaths` registry (see `## Expected-divergence registry`); the tool skips the byte-compare and lists the file under `### Expected per-repo divergence`. Treated as no-action.
- **`preserve`** — a verbatim preserve-marker phrase was found citing this file in `<target>/CHANGELOG.md` or `<target>/docs/ac*.md`. Routed to `## Out Of Scope` with the marker quoted verbatim.
- **`ambiguity`** — local commits exist for this file (`git log -n 5 --follow` returned ≥ 1 commit) but no preserve marker was found. The file's commits appear under `### Divergent files`; the Director routes it via the auto-populated `## Director Review` entry. Format-defining files (see `## Format-defining files`) are an exception: they are hard-routed to sync regardless of classification, so they emit no Director Review Q. Not softened with "could be intentional" in `## Out Of Scope`.
- **`clear-sync`** — divergent with neither local commits nor preserve marker. Routed to this AC's `## In Scope` as `sync to canon`.
- **`missing-in-target`** — canon ships the file; target does not. If canon is non-empty, routed to `## In Scope` as `create from canon` and detailed under `### Missing in target (create candidates)` with a content preview. The auto-emitted AT is a byte-equality check against canon content. If canon is empty, listed under `### Warnings` only.
- **`target-has-no-canon`** — file exists in target, NOT in canon for this flavor. Two branches surface a file under this classification (per the asymmetry note's promise):
  - **Cross-flavor branch:** the file exists in the OTHER flavor's canon. Possible flavor mismatch.
  - **Name-reference branch (AC112 Class Z):** the file exists in target only (no canon counterpart in either flavor) but is name-referenced from a divergent target file (e.g., `rel.sh` references `./cmd/rel/color.go` and color.go has no canon presence).

  Both branches list the file under `### Files in target without canon` and emit a Director Review Q with options `keep / delete / migrate-to-canon` (see `## Decision-surface coverage`). The `migrate-to-canon` option in the name-reference branch means migrate to the current flavor's canon (since the other flavor doesn't have it either).

For every divergent file, the staged AC's `## Implementation Notes` carries:

1. The verbatim preserve-marker citations (if any) — every line that matched a recognized phrase.
2. Every commit returned by `git log -n 5 --follow -- <file>`. Verbatim, not abridged.
3. A `Coupled-with: <list>` line surfacing files coupled by the unified coupling rule (see `## Coupling analysis`).

The full `diff -u` hunk lives in the sister file `docs/ac<N>-drift-scan-from-<short-sha>-diffs.md`, not in the AC body — see `## Staged artifacts`.

## Format-defining files

The `FormatDefiningCanonPaths` registry lists canon files whose content defines the form of a section INSTANTIATED in the staged AC.

**Inclusion criterion:** A file belongs in this registry iff its content defines the form of a section INSTANTIATED in the staged AC — either shape:

1. **Tool-emitted form** — the canon file defines the form of a section the staged AC's tool-emitted text instantiates (e.g., `docs/critique-protocol.md` defines `## Director Review` round-append structure the tool emits on subsequent rounds).
2. **Operator-instantiated form** — the canon file defines the form of a section the staged AC's Operator-fill text instantiates (e.g., `AGENTS.md` defines the `## Objective Fit` 3-part form the Operator fills, and the AT-label convention every AT line carries).

Both shapes hard-route to sync via the same mechanic. Divergence in either shape would make the staged AC's text contradict canon's specification of that form. Importance, frequency-of-edit, or being-a-template are not sufficient on their own.

**Initial registry:**

- `docs/ac-template.md` (tool-emit + Operator-fill: defines every AC's section shape)
- `docs/critique-protocol.md` (tool-emit: round-append structure + four-field terminator)
- `AGENTS.md` (Operator-fill: Objective Fit 3-part form, AT-label convention)

**Hard-route-to-sync rule:** when any registry file is divergent (any classification other than `match` or `expected-divergence`), the file is auto-routed into `## In Scope` as a sync action regardless of its raw classification. The Director Review Q for these files is suppressed; the routing is forced. The staged AC carries a `### Format-defining file routing` sub-subsection under `## Implementation Notes` naming each one with the rationale: the staged AC instantiates canon's form for these files (tool-emitted sections + Operator-fill sections); routing as anything other than sync would leave the AC self-contradictory.

**Note on Operator-fill form via the registry.** The staged AC's `## Objective Fit` (Operator-fill) follows the form prescribed by canon's `AGENTS.md` and `docs/ac-template.md`. Because both files are in this registry and force-synced when divergent, consumer-local divergence is reconciled every drift-scan run; the Operator-fill form therefore stays consistent with canon's form by construction. The tool-emitted `## Director Review` form is governed by Class V's exception (documented under `## What the tool emits` `## Director Review` entry) — a routing-matrix shape distinct from canon's question-form rule.

**Addition criterion:** the contributing AC MUST add a future canon file to the registry when (and only when) it passes the inclusion test above (either shape). Importance, frequency-of-edit, or being-a-template MUST NOT be used as standalone justification.

**Failure mode:** registry bloat. Adding a file based on importance, being-a-template, or frequency-of-edit (without actual form-instantiation in the staged AC) inflates the hard-route surface. Each unnecessary entry forces an AC to sync that file even when divergence is intentional and Director would otherwise route preserve.

## Missing-in-target file routing

When a canon file with non-empty content is absent from the target, the tool classifies it as `missing-in-target` and auto-routes it into `## In Scope` as `create from canon`. Per AGENTS.md Approval Boundaries (create requires explicit approval), this AC's critique gate IS the approval surface — Director routes the file to `## Out Of Scope` to keep absent.

The staged AC carries a `### Missing-in-target file routing` sub-subsection under `## Implementation Notes` (parallel to `### Format-defining file routing`) naming each missing-in-target file with the rationale, so the Operator does not have to infer why the file landed in In Scope without a Director Review Q.

## Expected-divergence registry

The `ExpectedDivergencePaths` registry lists canon files that are per-repo stubs by design — files whose canon content is a placeholder and whose target content is expected to diverge. The tool skips the byte-compare for these paths and classifies them as `expected-divergence`.

**Initial registry:** `plan.md`.

**Extension:** when a future canon file is introduced as a per-repo stub, the contributing AC MUST add the file's path to `ExpectedDivergencePaths` in the same code change. The registry MAY be per-flavor if a stub is flavor-specific.

**Failure mode:** introducing a per-repo stub without registering it in `ExpectedDivergencePaths` causes drift-scan to byte-compare the stub against target's filled content on every run, producing a stream of `clear-sync` (or worse, `ambiguity`) classifications that route to In Scope as "sync to canon" — silently overwriting target's per-repo content with the canon stub.

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

## Reference qualification

Tool-emitted text inside the staged consumer AC must qualify any reference to a governa-only path as `governa @ <sha>: <path>`. Bare references resolve in governa but break in the consumer repo (where the file does not exist), so the consumer reader cannot follow the pointer.

**Registry:** `governaOnlyPathPrefixes` in `internal/driftscan/driftscan.go` enumerates path prefixes that exist ONLY in governa, not in any consumer overlay. Initial entries: `docs/drift-scan.md`, `docs/development-cycle.md`, `docs/development-guidelines.md`, `docs/build-release.md`, anything under `internal/`, anything under `cmd/governa/`. Paths shared with consumer overlays (`docs/ac-template.md`, `docs/critique-protocol.md`, `AGENTS.md`, `CHANGELOG.md`, etc.) stay out of the registry — those references are target-relative when emitted into the staged AC and must NOT be qualified.

**Helper:** `qualifyGovernaPath(sha, path)` in the same file returns the qualified form. Every emission site that references a governa-only path uses it.

**Enforcement:** the registry-driven test (AC107 AT2) walks the staged AC body, tokenizes backticked paths, and trips on any unqualified governa-only path. Adding a future governa-only prefix to the registry extends coverage without rewriting the test; forgetting the helper at a new emission site fails the test on the first run.

## Verify subcommand

`governa drift-scan verify <ac-path>` runs structural-compliance checks on a staged AC file before handoff. AC114 Part D / R4.12 promotes the Operator-side Handoff verification rules to code.

**Five checks:**

1. **No `<!-- TBD by Operator -->` substring remains anywhere** — every Operator-fill placeholder must be replaced.
2. **Each `### Divergent files` per-file block has a non-TBD `What diverged:` line** — per-file Operator-fill required.
3. **Each `## Director Review` numbered Q has a non-TBD lean** — each Q's lean placeholder filled.
4. **Each `## Director Review` numbered Q has a non-TBD `Why:`** — each Q's why placeholder filled.
5. **When sync ∧ preserve indicated, the audit body has no `[TBD]` substring** — Part B's checklist must be filled.

**Sync ∧ preserve heuristic (pinned parse rules):**

- **Section bounds:** `## In Scope` body = lines after the `## In Scope` heading until (but not including) the next `## ` heading. Same shape for `## Out Of Scope`.
- **Sync item (in In Scope body):** any line matching `` ^- `[^`]+` — (sync to canon|create from canon) ``. Both `sync to canon` (clear-sync + format-defining hard-route) and `create from canon` (missing-in-target auto-route) count — both write to target.
- **Preserve marker (in Out Of Scope body):** any line matching `` ^- `[^`]+` — preserve marker present:``.
- **Sync ∧ preserve fires:** at least one sync-item line in In Scope AND at least one preserve-marker line in Out Of Scope.

**Output format:** `<line>:<section>: <description>` per failure (gcc/golangci-lint convention for editor jump-to-error). Exit 0 = clean, exit 1 = failures present, exit 2 = usage error.

**Usage:** `governa drift-scan verify <ac-path>`. Run before handoff to the Director, after filling all five Operator-fill spots in the staged AC.

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

**Output:** each divergent file in a coupled set (≥2 files) carries a `Coupled-with: <signal-name> set (see § Coupled sets)` line; uncoupled files emit no `Coupled-with:` line. The `### Coupled sets` reading-aid sub-subsection under `## Implementation Notes` summarizes the coupling graph as informational bullets naming the signal that produced each set.

`## Director Review` emits one numbered routing question per ambiguity file (not per coupled set). Coupling is surfaced informationally ONLY via the `### Coupled sets` reading aid; routing is per-file.

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

When a synced file's target had recent local commits (check the `Local commits:` block in the per-file Divergent files entry), the Operator MUST trace which local wording survives in canon vs. is superseded. Procedure:

1. For each synced file with at least one local commit since canon adoption, open the per-file diff hunk in the sister diffs file.
2. Identify each `+` line (target-side, what target carries that canon doesn't). These are local additions the sync would erase.
3. Identify each `-` line (canon-side, what canon carries that target doesn't). These are canon additions that supersede prior local content.
4. Document the trade-off in `## Implementation Notes` — narrative paragraph for one or two files, or a dedicated `### Refinement tracing` sub-subsection when multiple files need tracing. Cite each preserved-verbatim local phrase and each superseded local phrase.

**Failure mode:** the Operator describes the post-merge state without acknowledging local wording about to be lost. The Director critiques the AC without realizing recent local intent is being overwritten — and may reverse the sync after critique discovers it. The trace surfaces the trade-off BEFORE the routing decision lands.

## Small-drift simplification

When the AC's total drift is small (one or two divergent files, a handful of changed lines), the Operator MUST keep the AC proportional:

1. State the small-drift framing in the Summary's first sentence (e.g., "Single-line drift in one file: ...").
2. Leave `## Out Of Scope` and `## Director Review` as `None.` if no items genuinely apply.
3. Keep ATs minimal — one AT per actually-changed file is sufficient. Do not invent boundary-case ATs to inflate the AC.

**Failure mode:** padding small-drift ACs with redundant ATs, speculative Director Review Qs, or extensive Out Of Scope lists makes them look like substantial work. Critique becomes a length-vs-substance review, not a content review. Disproportionate scaffolding signals scope creep where there is none.

When `## In Scope` is otherwise `None` (every divergent file is preserved, pending Director classification, or absent from canon):

- Tool emits the In Scope body as either `None — body lands as Director resolves Q1–Q<N>.` (when Director Review has open Qs) or `None — this AC ships only the staged plan.md IE entry; nothing to verify in target.` (when Director Review is `None.`).
- Operator MUST state in the Summary that the AC ships only itself plus the staged `plan.md` IE entry (no file edits).

## Handoff

After all five Operator-fill spots are filled and `governa drift-scan verify <ac-path>` reports zero failures, the Operator MUST send a terse handoff message to the Director using EXACTLY the template below (substitute the actual filename and repo name):

> Staged `docs/ac<N>-drift-scan-from-<short-sha>.md` in repo `<repo-name>`. Governa's part in this drift-scan run is complete. Please work with the `<repo-name>` agent there and follow AC cadence.

**Failure mode:** the Operator pads the handoff with a findings recap (classification counts, per-file divergences, lean summaries). The Director already has the AC; recap noise wastes attention and signals that the Operator considers the chat thread the canonical record (it isn't — the AC is). The handoff redirects; it doesn't summarize.
