# Drift Scan

`governa drift-scan <repo-path>` walks the canon overlay, classifies each file against the consumer-repo target, and emits two report files at the consumer repo root. The consumer Operator authors the AC manually from the report using normal AC discipline (governed by the consumer's own `AGENTS.md` and `docs/ac-template.md`).

## Protocol

- The tool walks canon, byte-compares each governed file against the target, classifies divergences, collects evidence (preserve markers, recent commits, diffs), and emits two report files at the consumer repo root: `drift-report-<short-sha>.md` and `drift-report-<short-sha>-diffs.md`.
- Before any canon→target walk, the tool runs the `## Canon-coherence precondition` check. If canon is internally incoherent on a registered cross-file rule, the tool refuses to emit and reports the incoherence on stdout. No target writes occur.
- Idempotent: re-scanning against the same canon SHA overwrites the existing report files at the same path. No append, no error, no suffix.
- One repo per invocation. The tool makes no commits in the target. It does NOT stage an AC, modify `plan.md`, or write under `<target>/docs/`.
- Assume the user has asserted the path is an adopted-governa repo. The tool refuses to run against the governa source itself.

## What the tool emits

Two files at the consumer repo root, plus a single-line stdout summary.

**`<target>/drift-report-<sha>.md`** — the file-level drift report. Header carries `Invocation`, `Canon: governa @ <sha>`, `Target`, `Flavor`, `Repo name`, `Counts: ...` (per-classification tally), and the scan-asymmetry note. Then a `## Files` section with one `### \`<relpath>\` — <classification>` block per file, each carrying:

- `Canon ref: \`<canon-path>\`` — the path under canon that produced the comparison (or a "no canon path for flavor" annotation for `target-has-no-canon` files).
- `Format-defining: yes` — when the file is in `formatDefiningCanonPaths` (consumer Operator decides routing; the tool only flags).
- `Preserve markers:` — when one or more preserve-marker phrases were grepped from `<target>/CHANGELOG.md` or `<target>/docs/ac*.md`.
- `Local commits:` — recent `git log -n 5 --follow` lines for the relpath, with `(adoption)` annotation on commits whose subject matches the governance-adoption pattern.

Diff hunks live in the sister file, not file 1.

**`<target>/drift-report-<sha>-diffs.md`** — the per-file diffs. Title `# Drift-Scan Diffs (governa @ <sha>)`. Convention stamp on line 2: `_Diff convention: \`+\` lines exist in TARGET; \`-\` lines exist in CANON. \`+\` is "target has this; canon does not"; \`-\` is "canon has this; target does not"._`. One `## \`<relpath>\`` H2 section per divergent file, each with the verbatim `diff -u` hunk in a fenced code block. Empty body when no divergent files.

**Stdout summary** — single line: `wrote drift-report-<sha>.md and drift-report-<sha>-diffs.md (<counts>)`. Suppressed when `--json` is set; in JSON mode the full Report struct goes to stdout instead.

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

  Both branches list the file under `### Files in target without canon` and emit a Director Review Q with options `keep / delete` (see `## Decision-surface coverage`). Migrating a file into canon is a separate governa-side workflow, not a drift-scan resolution; the consumer agent surfaces the file via `keep` and Director coordinates with governa maintainer if upstream migration is desired.

For every divergent file, the report file 1 carries the verbatim preserve-marker citations (if any) and every commit returned by `git log -n 5 --follow -- <file>` (verbatim, not abridged). The full `diff -u` hunk lives in the sister file (`drift-report-<sha>-diffs.md`).

## Format-defining files

The `formatDefiningCanonPaths` registry lists canon files whose content defines the form the consumer Operator's AC instantiates. The report flags these with a `Format-defining: yes` line in the per-file block — the consumer Operator sees the flag and decides routing accordingly. The tool itself does not auto-route; routing is the Operator's call.

**Initial registry:**

- `docs/ac-template.md` (defines every AC's section shape)
- `docs/critique-protocol.md` (round-append structure + four-field terminator)
- `AGENTS.md` (Objective Fit form, AT-label convention)

**Inclusion criterion:** a canon file belongs in this registry iff syncing it is the only way to keep the consumer Operator's AC consistent with canon's section shape. Divergence on these files surfaces in the report with the flag so the Operator does not miss it. Importance, frequency-of-edit, or being-a-template are not sufficient on their own.

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

