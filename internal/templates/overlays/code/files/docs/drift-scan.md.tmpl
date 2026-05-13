# Drift Scan

`governa drift-scan` walks the canon overlay, classifies each governed file against the consumer-repo's content, and emits a partially-filled AC stub plus a sister diffs file under the consumer repo's `docs/`. The consumer Operator iterates on the emitted stub under normal AC discipline (governed by the consumer's own `AGENTS.md` and `docs/ac-template.md`).

The tool is consumer-run. Install the binary (`go install github.com/queone/governa/cmd/governa@latest`), then run `governa drift-scan` from the consumer repo root — no positional arguments, no governa source checkout needed.

## Protocol

- Invocation accepts no positional arguments. The tool runs against the current working directory only.
- The cwd must be a governa-adopted repo: `AGENTS.md` must be present, plus at least one of `docs/ac-template.md`, `docs/release.md`, `docs/build-release.md`, or a `CHANGELOG.md` row referencing `governa apply`. The tool hard-errors with recovery guidance if the check fails.
- The tool refuses to run against the governa source itself.
- The tool walks canon, byte-compares each governed file against the cwd, classifies divergences, collects evidence (preserve markers, recent commits, diffs), and emits two files under `docs/`: the AC stub and the sister diffs file. Both share the same AC number `N` and slug stem `drift-scan-v<X.Y.Z>`.
- Before any canon→cwd walk, the tool runs the `## Canon-coherence precondition` check. If canon is internally incoherent on a registered cross-file rule, the tool refuses to emit and reports the incoherence on stdout. No file writes occur.
- One repo per invocation. The tool makes no commits in the cwd and does not modify `plan.md`. Writes under `<cwd>/docs/` are limited to the AC stub and its sister diffs file.

## What the tool emits

Two files under the consumer repo's `docs/`, plus a single-line stdout summary.

**`docs/ac<N>-drift-scan-v<X.Y.Z>.md`** — the emitted AC stub. Conforms to `docs/ac-template.md` shape minus the copy-instruction preamble:

- H1 title: `# AC<N> Drift-Scan Adoption from governa v<X.Y.Z>`.
- Opening one-sentence summary of the canon delta.
- `## Summary` — concise paragraph describing the classifications surfaced and pointing at the sister diffs file.
- `## Objective Fit` with three numbered items: **Outcome** ("Sync this repo to governa @ v<X.Y.Z> canon."), **Priority** ("Canon hygiene; recurring drift-scan cycle."), **Dependencies** ("governa @ v<X.Y.Z>; prior drift-scan ACs in this repo, if any.").
- `## In Scope` — `clear-sync` and non-empty `missing-in-target` entries, each citing the sister diffs file by H2 anchor.
- `## Out Of Scope` — `preserve` (with marker citation) and `expected-divergence` entries.
- `## Implementation Notes` — pointer to the sister diffs file. Code-flavor consumers also see the reachability gate sentence.
- `## Acceptance Tests` — one byte-equality AT per `clear-sync` item plus an AT for the canon-coherence precondition pass.
- `## Documentation Updates` — list of affected docs derived from the canon walk.
- `## Director Review` — one numbered item per `ambiguity` and `target-has-no-canon` file, phrased as the existing decision-surface question.
- `## Status` — `PENDING` on initial emission.
- No `## Critique` section on initial emission; the consumer Operator adds it when critique rounds begin (per `docs/critique-protocol.md`).

**`docs/ac<N>-drift-scan-v<X.Y.Z>-diffs.md`** — the sister diffs file. Carries one `## \`<relpath>\`` H2 section per divergent file with the verbatim `diff -u` hunk, prefaced with a `Direction:` line (`target leads`, `canon leads`, or mutual line counts). Empty body when no divergent files.

**Emission marker.** Each emitted file carries an HTML-comment marker on line 1:

```
<!-- drift-scan: emitted-by governa v<X.Y.Z>; emission-sha=<sha256> -->
```

`emission-sha` is the SHA-256 of the file body (everything after the marker line). The marker is the tool's edit-detection signal on re-runs.

**Stdout summary** — single line: `wrote docs/ac<N>-drift-scan-v<X.Y.Z>.md and docs/ac<N>-drift-scan-v<X.Y.Z>-diffs.md (<counts>)`. Paths are repo-root-relative. Suppressed when `--json` is set; in JSON mode the structured `Report` struct goes to stdout alongside the markdown emission.

## AC number allocation

The tool determines `N` per `docs/ac-template.md` line 3:

- If a same-canon-version stub already exists at `docs/ac<M>-drift-scan-v<X.Y.Z>.md`, the tool reuses `M` (subject to the edit-detection guard, below). No bump on re-run against the same canon version.
- Otherwise, `N = max((a) AC numbers in docs/ac*.md filenames, (b) AC references anywhere in 'git log --all --pretty=%B' output covering subject and body, counting composites like "AC53+AC54") + 1`.

## Re-run behavior and edit-detection guard

Re-running drift-scan against the same canon version is idempotent on **unedited** stubs — both the AC stub and sister diffs file overwrite in place using the existing `N`. The tool refuses to overwrite if either file has been edited since emission:

- On re-run, the tool reads the existing file, parses the line-1 marker, recomputes the SHA-256 of the body, and compares.
- Match → file unedited → overwrite.
- Mismatch or no marker → file edited → refuse with recovery guidance: "AC stub at `<path>` has been edited since last drift-scan emission. To re-run: (a) commit edits and delete the stub to regenerate, or (b) rename the stub off the `drift-scan-v<X.Y.Z>` slug."

This protects in-progress consumer Operator critique edits from accidental clobber on re-run.

## Divergence classification

The tool emits one of the classifications below for every file. The Operator can override by editing the emitted AC stub before commit, routing the file in `## In Scope` / `## Out Of Scope` accordingly.

- **`match`** — canon and target byte-equal. Not listed in the AC stub by default.
- **`expected-divergence`** — canon is a per-repo stub by design and the file's path is in the `ExpectedDivergencePaths` registry (see `## Expected-divergence registry`); the tool skips the byte-compare and lists the file under `## Out Of Scope`. Treated as no-action.
- **`preserve`** — a verbatim preserve-marker phrase was found citing this file in `<cwd>/CHANGELOG.md` or `<cwd>/docs/ac*.md`. Routed to `## Out Of Scope` with the marker quoted verbatim.
- **`ambiguity`** — local commits exist for this file (`git log -n 5 --follow` returned ≥ 1 commit) but no preserve marker was found. Routed to `## Director Review` as a numbered routing question. Format-defining files (see `## Format-defining files`) are an exception: they are hard-routed to sync regardless of classification.
- **`clear-sync`** — divergent with neither local commits nor preserve marker. Routed to `## In Scope` as `sync to canon`, with the sister diffs file referenced by H2 anchor.
- **`missing-in-target`** — canon ships the file; target does not. If canon is non-empty, routed to `## In Scope` as `create from canon`. If canon is empty, surfaced as an informational note only.
- **`target-has-no-canon`** — file exists in cwd, NOT in canon for this flavor. Two branches surface a file under this classification:
  - **Cross-flavor branch:** the file exists in the OTHER flavor's canon. Possible flavor mismatch.
  - **Name-reference branch:** the file exists in cwd only (no canon counterpart in either flavor) but is name-referenced from a divergent target file (e.g., `rel.sh` references `./cmd/rel/color.go` and color.go has no canon presence).

  Both branches surface the file in `## Director Review` with `keep / delete / migrate-to-canon` options. Migrating a file into canon is a separate governa-side workflow, not a drift-scan resolution; the consumer agent surfaces the file via `keep` and the Director coordinates with the governa maintainer if upstream migration is desired.

For every divergent file, the AC stub references the sister diffs file by H2 anchor; the full `diff -u` hunk lives in `docs/ac<N>-drift-scan-v<X.Y.Z>-diffs.md`.

## Reachability of canon-only branches

**Scope.** This rule applies to canon code (code-flavor consumers); doc consumers don't have branching canon and the rule is structurally inapplicable to them.

Canon code files may carry branches gated on governa's host shape (e.g., `cmd/<repo>/main.go` presence, `internal/templates/` tree). Such branches are dormant on consumers — byte-divergence on those lines is benign by construction; drift-scan does not flag them as action-requiring sync gaps, though consumers may still voluntarily adopt them for operational reasons (e.g., maintaining canonical shape across periodic baseline syncs). Before routing a divergent canon-code file as drift, verify the divergent code path is reachable in the consumer's structure.

The gate sentence is the verbatim value of the exported `ReachabilityHeaderReminder` constant in `internal/driftscan/driftscan.go`, also emitted in every code-flavor AC stub's `## Implementation Notes`:

```
Reachability check: verify divergent canon-code branches reach this consumer's structure before treating as drift.
```

**Known limit.** This rule assumes canon-shaped branches; sync-omitted branches that look dormant are real drift. Consumer agents distinguish case-by-case based on the gate condition.

Structurally unreachable branches are not drift.

## Format-defining files

The `formatDefiningCanonPaths` registry lists canon files whose content defines the form the consumer Operator's AC instantiates. Divergence on these files is surfaced as a sync item regardless of raw classification (commits or markers do not override the routing for format-defining files).

**Initial registry:**

- `docs/ac-template.md` (defines every AC's section shape)
- `docs/critique-protocol.md` (round-append structure + four-field terminator)
- `AGENTS.md` (Objective Fit form, AT-label convention)

**Inclusion criterion:** a canon file belongs in this registry iff syncing it is the only way to keep the consumer Operator's AC consistent with canon's section shape. Importance, frequency-of-edit, or being-a-template are not sufficient on their own.

## Expected-divergence registry

The `ExpectedDivergencePaths` registry lists canon files that are per-repo stubs by design — files whose canon content is a placeholder and whose target content is expected to diverge. The tool skips the byte-compare for these paths and classifies them as `expected-divergence`.

**Initial registry:** `plan.md`; consumer-repo-root `arch.md` (governa-templated architecture stub — consumer fills in repo-specific architecture).

**Extension:** when a future canon file is introduced as a per-repo stub, the contributing AC MUST add the file's path to `ExpectedDivergencePaths` in the same code change. The registry MAY be per-flavor if a stub is flavor-specific.

**Failure mode:** introducing a per-repo stub without registering it in `ExpectedDivergencePaths` causes drift-scan to byte-compare the stub against target's filled content on every run, producing a stream of `clear-sync` (or worse, `ambiguity`) classifications that route to `## In Scope` as "sync to canon" — silently overwriting target's per-repo content with the canon stub.

## Canon-coherence precondition

Before any canon→cwd walk, the tool checks canon for internal coherence on a set of registered cross-file rules. The check is canon-only — it does not read the cwd.

**Authoritative source:** `AGENTS.md` (governa root and base overlay) is authoritative for any rule it describes. Overlay templates and other canon files must conform.

**Behavior on incoherence:** the tool **hard-fails** — refuses to emit, exits non-zero, and writes nothing to the cwd.

- **Channel:** the structured report replaces what would have been the stdout summary. H1 reads `# Canon-Coherence Precondition Failed` so consumer agents reading drift-scan stdout route on H1.
- **Report content:** for each incoherent rule — the rule name, every conflicting canon path with line numbers and conflicting text, the authoritative source per AGENTS.md, the canonical wording the conflicting sites must conform to.
- **Framing:** the report opens with one line stating this is a **governa-side** defect requiring canon reconciliation, not a consumer-side routing decision. Consumer Director's action is "ping governa maintainer," not "route a divergence."
- **Enumerate, don't bail:** when multiple rules are simultaneously incoherent, the precondition surfaces all of them in one report. Failing at the first hit forces reconcile-rerun thrash.
- **Fire early:** the precondition runs canon-only and does not need the cwd. It runs before the canon→cwd walk so canon-side defects surface in seconds, not after a full target enumeration.
- **No cwd writes:** nothing under `<cwd>/docs/` is emitted, no IE inserted into `<cwd>/plan.md`, no sister diffs file. The precondition runs before any cwd write, so nothing to roll back.

The check is registry-driven: rule definitions live next to `FormatDefiningCanonPaths`, so adding future cross-file rules extends the check.

## Preserve-marker phrase set

The tool recognizes exactly the four phrases below in `<cwd>/CHANGELOG.md` table rows or `<cwd>/docs/ac*.md` content. Implicit AC references locking the local form (e.g., `migrate <x> to <path>`, `<path> from governa overlay`) **do not** count — the tool will misclassify those files as `ambiguity` until the row is backfilled with an explicit marker.

| Phrase | Example |
|---|---|
| `preserve <path> <qualifier>` | `preserve docs/release.md customization` |
| `do not sync <path>` | `do not sync docs/local-overrides.md` |
| `intentional divergence: <path>` | `intentional divergence: rel.sh` |
| `<path>: keep local` | `docs/team-rituals.md: keep local` |

When shipping an AC that locks a local form against canon, include one of these phrases in the CHANGELOG row alongside whatever else the row says. See `docs/build-release.md` for the release-flow rule.

## Match evidence

For every `match`-classified file, the canon walk records `byte-equal (canon @ v<version> vs <relpath>)` internally; the AC stub does not enumerate match files by default. Files whose canon is a per-repo stub appear in `## Out Of Scope` under the `expected-divergence` classification.

## Ownership and consumer response

- Apply these rules whenever reviewing the emitted AC stub.
- Treat canon-owned violations as governa feedback.
- Report canon-owned violations upstream to the governa maintainer.
- Skip local patches of canon-owned text.
- Treat repo-owned violations as local repo work.
- Fix repo-owned violations directly in the next AC.
- Pause when a canon update introduces an Instruction Style violation.
- Report the violation upstream.
- Skip local rewrites of canon-owned text unless an explicit AC declares intentional divergence.

Note: drift-scan provides the diff payload; the consumer agent's review is the classifier that decides what to do with each divergence. Local patches of canon-owned text create drift. Inside the governa repo itself, both ownership paths apply: canon-owned template/source files need canon fixes; governa-local docs can be edited as local repo work.

## Consumer Operator workflow

The consumer-repo Operator handles drift-scan as a self-contained loop. The Director's role is to authorize the scan, review the summary, and resolve the Director Review items that the emission surfaces.

- **Trigger phrase.** When the Director says "do a new drift-scan" or similar wording in a consumer repo, the Operator treats that as authorization to run `governa drift-scan` from the repo root. No additional authorization request is needed for the run itself.
- **Post-emission review.** Immediately after the two files are emitted, the Operator performs a high-level review of both the AC stub and the sister diffs file, and reports a concise summary to the Director — without waiting for a separate "review the drift-scan report" request.
- **Summary content.** The summary names: main drift categories (counts of `clear-sync`, `ambiguity`, `preserve`, `expected-divergence`, etc.); Director Review questions surfaced by the emission; obvious canon-owned issues that warrant upstream feedback rather than local sync; whether the emitted AC stub appears ready for critique and iteration.
- **Scope discipline.** The summary is high-level; the emitted AC stub remains the source of truth. The Operator's role is to surface the report at a glance, not to recapitulate it.
- **Not a governa-source handoff.** This is consumer-repo Operator behavior after a local scan. It does not require any action on the governa source side.
- **Lifecycle symmetry.** When the Director wants to extricate Governa rather than refresh canon, run `governa rm` from the consumer repo root; it emits a cleanup AC stub plus a targeted sister diffs file for hybrid-file removal decisions.
