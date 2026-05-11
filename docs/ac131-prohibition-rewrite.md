# AC131 Prohibition rewrite across at-risk AGENTS.md rules

Rewrite at-risk prohibition-shaped rules in `AGENTS.md` (and conforming overlay templates) using a positive-substitution pattern derived from the recurring `./build.sh`-bypass failure. Doc-only change to governance text. Phased: a pilot validates the rewrite pattern on the two highest-failure rules before a broader sweep across the remaining at-risk inventory.

## Summary

Multiple AGENTS.md rules expressed as prohibitions ("Never X", "Do not X") have been violated repeatedly across agent instances despite being loaded in context. The originating case is `Never run individual tool commands directly` (Base Rules), which an agent bypassed by running `go vet`/`go test`/`go fmt`/`staticcheck` directly during a repo evaluation. The diagnosis generalizes: prohibitions fail when they (a) fight a strong default habit, (b) require the agent to first classify its intended action as falling in the banned category, and (c) lack an external gate that catches the violation. This AC rewrites the rules where all three conditions hold, using the pattern: lead with positive substitution, name specific failure modes, supply an in-stream diagnostic trigger, and reframe carve-outs as imperatives so they cannot be read as escape hatches.

## Objective Fit

1. **Outcome.** Reduce repeat violations of high-failure AGENTS.md rules by reshaping prohibitions into positive-substitution rules with named failure modes and diagnostic triggers, propagated into the overlay template so consumer repos inherit the same shape.
2. **Priority.** The build.sh-bypass failure has recurred across instances and corrupts "is the repo green" verification — a foundational workflow. Other at-risk rules (memory writes, scope expansion, file path leakage) similarly fail silently and erode governance trust. Trade-off: this is a doc-only rewrite of governance text; it does not advance product features. Pivots ahead of any new AC because every other AC inherits these rules.
3. **Dependencies.** Builds on AGENTS.md File-Change Discipline rule "Codify corrections about repo behavior in the appropriate governance doc — not in agent-local memory" (the reason this lands in AGENTS.md, not as agent memory or a hook). Builds on Project Rule "AGENTS.md is the authoritative source for rules it describes. Overlay templates and other canon files must conform" (the reason changes propagate to the overlay). Contradicts no prior decision.

## In Scope

### Phase 1 — Pilot (validates the rewrite pattern)

Rewrite two rules and observe across subsequent sessions whether violations drop:

- **`AGENTS.md` Base Rules — `./build.sh` rule.** Replace the current two-bullet pair (lines 83–84):
  - *Existing:* "Use the repo's canonical build command (`./build.sh` or equivalent). Never run individual tool commands directly. See `docs/build-release.md`." + "For single-utility smoke tests, use `go run ./cmd/<tool>/` or `go build -o /tmp/<name> ./cmd/<tool>/`. Do not `go build ./cmd/<tool>/` from repo root — drops a stray binary."
  - *Proposed:* "Run `./build.sh` for every 'is the repo green' verification. Do not substitute direct `go test`, `go vet`, `go fmt`, `go fix`, or `staticcheck` invocations — a green direct run does not guarantee a green `./build.sh`. Running two or more of these in sequence means you are reimplementing the pipeline; stop and run `./build.sh`. See `docs/build-release.md`." + "Use targeted single-purpose `go` calls only for non-verification work: `go test -run <Name> ./pkg` to debug one failure, `go list`, `go doc`, or smoke-running one binary with `go run ./cmd/<tool>/` or `go build -o /tmp/<name> ./cmd/<tool>/`. Do not `go build ./cmd/<tool>/` from repo root — drops a stray binary."

- **`AGENTS.md` Base Rules — AC-body-labels-in-code rule** (line 93). Apply same shape: lead with positive form ("Name tests, comments, and errors by behavior"), name the specific banned tokens (AC/AT/Class/Part/Round numbers in identifiers), reframe the `Historical:` carve-out as an explicit pre-condition rather than an escape hatch.

### Phase 2 — Sweep (single pass, Director-set; conditional on pilot validating the pattern)

Rewrite the remaining high-risk items inventoried in Implementation Notes in one pass, applying the same shape per item. Eight items:

- `Interaction Mode` line 31 — `no current-state recaps or implication walk-throughs`
- `Approval Boundaries` line 44 — `Do not prepare, execute, publish, deploy, or distribute without explicit user request` (added Round 2)
- `Approval Boundaries` line 46 — `Do not start release-prep bookkeeping early` (added Round 2)
- `File-Change Discipline` line 65 — `Do not expand scope ad hoc`
- `File-Change Discipline` line 66 — `Do not commit personal absolute filesystem paths`
- `File-Change Discipline` line 67 — `No exceptions for "small" changes` (re: tests)
- `File-Change Discipline` line 68 — `Codify corrections... not in agent-local memory`
- `Base Rules` line 94 — `Do not re-read files already in context`

### Files to modify

- `AGENTS.md` (and `CLAUDE.md` symlink, automatically) — Phase 1 + Phase 2 rule rewrites.
- `internal/templates/overlays/doc/files/AGENTS.md.tmpl` — propagate rewrites that apply to consumer repos in this same AC (Director-set: same-AC propagation, no follow-on overlay AC). Per-rule judgment: rules describing governa-internal workflow (e.g., `./build.sh` specifics) may not need overlay propagation; rules describing universal agent behavior (e.g., scope expansion, memory writes) do.

### Files to create

None.

## Out Of Scope

- **Medium-risk prohibitions** (`Avoid ### sub-subsections`, `No "What's in it" / "Main conclusion" / "Next steps" headers`, `Do not note skipped checks`, `No half-migrated states`, `Do not invent top-level sections`, `Do not reorder sections`). These have either weaker habit pull or some natural gating; defer to a later AC if the Phase 2 sweep proves the pattern further.
- **Low-risk prohibitions** that are already gated and working (`Never run git commit`, `Never run the release command`, file-creation-without-authorization, symlinked-file edits, `Do not set DONE`). Touching these risks regression.
- **Structural changes to AGENTS.md** (e.g., splitting Base Rules into multiple sections to shorten the bullet list). Section structure is fixed by AGENTS.md's Governed Sections rule; a structural change is a separate contract change requiring director approval. (Director-set: stay strict for this AC — no structural-change proposal mixed into rule rewrites; muddies the validation signal.)
- **Harness-level enforcement** (PreToolUse hooks, settings.json policies). governa is agent-agnostic; harness mechanisms are out of scope per the original feedback's Decisions.
- **Agent-local memory entries** as a workaround. Forbidden by File-Change Discipline; this AC is the explicit alternative.

## Implementation Notes

### Originating case (build.sh)

During an "evaluate this repo" request, the agent ran `go vet ./...`, `go test ./...`, `go fmt ./...`, and `staticcheck ./...` directly, despite AGENTS.md Base Rules stating "Never run individual tool commands directly." The rule was loaded in context via the AGENTS.md → CLAUDE.md symlink. The failure was attention, not knowledge: pattern-matching ("evaluate a Go repo" → reach for `go vet`/`go test`) overrode the explicit rule. The director reports the same failure has recurred across agent instances.

Why the existing rule failed:

- The rule is a prohibition ("Never run individual tool commands directly") buried in a 14-item Base Rules bullet list.
- Prohibitions require the agent to first classify what it is about to do as falling in the banned category — a step that fails under habit.
- The existing carve-out for single-utility smoke tests can be read as "individual `go` is sometimes fine," weakening the headline rule.

### Rewrite pattern (the heuristic this AC operationalizes)

A rule is at risk of repeated silent violation when **all three** conditions hold:

1. It fights a strong default Claude behavior (pattern-match habit).
2. It requires the agent to first classify its intended action as falling in the banned category.
3. No external gate (visible diff caught by user, hard failure, etc.) catches the violation.

**Gate-adjacency check.** Rules with strong gates often block only the literal action. List the substitutes the agent will reach for (drafting vs. executing, adjacent-file storage vs. the gated location). Each unguarded substitute is its own high-risk rule — add it to the inventory. (Added Round 2; the missed L44/L46 surfaced exactly this gap.)

For at-risk rules, apply this shape:

- **Lead with positive substitution.** "Run X" beats "Don't run Y" because there is no classification step — the positive form is the default action.
- **Name specific failure modes by token.** "Don't substitute direct `go test`, `go vet`, `go fmt`, `go fix`, or `staticcheck`" beats "Don't run individual tool commands" because the agent cannot rationalize "I didn't realize that counted."
- **Supply an in-stream diagnostic trigger.** "Running two or more of these in sequence means you are reimplementing the pipeline; stop and run `./build.sh`" catches the failure mid-stream, not just at the start.
- **Reframe carve-outs as imperatives.** "Use X only for Y" beats "X is allowed for Y" because the imperative form prevents reading the carve-out as a general escape hatch.
- **For recognition-failure rules** (paths, memory-write-on-request), wording alone is insufficient — supply a concrete scan-trigger ("before committing, scan staged content for `/Users/`, `/home/`, `C:\\`") inline in AGENTS.md, not split into a separate doc (Director-set: the trigger is the rule's enforcement mechanism; separating them reintroduces the classification gap the rewrite is meant to close).

### Full at-risk inventory (with risk reasoning)

**High risk — all three conditions present (the ten items addressed by this AC, across both phases):**

- Interaction Mode L31 `no current-state recaps or implication walk-throughs` — preamble is the default; no gate (user reads but rarely corrects).
- Approval Boundaries L44 `Do not prepare, execute, publish, deploy, or distribute without explicit user request` — gate-adjacency miss: L45 blocks running `git commit`, agent substitutes "draft commit message" which the gate doesn't reach. (Added Round 2.)
- Approval Boundaries L46 `Do not start release-prep bookkeeping early` — same gate-adjacency: agent offers commit messages, version bumps, CHANGELOG rows unprompted at "task complete" moments. (Added Round 2.)
- File-Change Discipline L65 `Do not expand scope ad hoc` — habit to "fix the nearby thing"; classification failure ("is this in scope?"); silent.
- File-Change Discipline L66 `Do not commit personal absolute filesystem paths` — paths flow into examples from session context; recognition failure; needs diagnostic trigger.
- File-Change Discipline L67 `No exceptions for "small" changes` (tests) — habit to skip tests for "tiny" formatting/CLI changes; no gate; bolding hasn't been enough.
- File-Change Discipline L68 `Agent memory is not a shadow governance system` — user often *asks* "remember this," training the wrong default; invisible to user; gate-adjacency variant: agent substitutes "store in feedback file" or "track this session" wording.
- Base Rules L83 `./build.sh` rule — originating case.
- Base Rules L93 `Don't put AC-body labels in code` — strong test-naming habit; carve-out (`Historical:`) muddies the rule; tests pass either way.
- Base Rules L94 `Do not re-read files already in context` — strong tool-reach habit; no gate; buried at the end of an unrelated tool-list bullet.

**Medium risk — habit + classification, but some gating (Out of Scope for this AC):**

- Governed Sections L26 `Avoid ### sub-subsections` — visible, but habit pulls toward nested headers in long edits.
- Review Style L75 `No "What's in it" / "Main conclusion" / "Next steps" headers unless asked` — visible; sometimes corrected, often not.
- Review Style L77 `Do not note skipped checks` — visible; medium habit.
- File-Change Discipline L61 `No half-migrated states` — visible in diffs.
- Governed Sections L22 `Do not invent top-level sections` / L23 `Do not reorder sections` — visible diff catches it.

**Low risk — gated or self-reinforcing (leave alone):**

- Approval Boundaries L45/L47 `Never run git commit` / `Never run the release command` — strong reinforcement (`No EXCEPTION` stamp), paired substitute, immediately visible. The `git commit` rule is the model the failing rules should aspire to.
- Interaction Mode L32 `Do not create files or make changes without explicit authorization` — visible the moment violated.
- Governed Sections L5 `do not edit them independently` — filesystem-gated (symlink).
- File-Change Discipline L64 `Do not set DONE` — paired positive (the states list).

### Pattern observations

- **Two distinct failure shapes need different fixes.** Most high-risk items respond to the build.sh treatment. But paths, AC-body labels, and memory-write-on-request fail at *recognition*, not at intent — they need a concrete scan-trigger more than they need rewording.
- **Carve-outs systematically weaken headline rules.** build.sh (smoke-test carve-out), AC-body labels (`Historical:` prefix), `###` ban (documented technical need exception). Each carve-out gives the agent an "I'm in the exception" rationalization. Per-rule judgment whether the carve-out earns its keep — reframe-as-imperative case-by-case rather than removing outright (Director-set: removing carve-outs may regress legitimate uses; reframing preserves the use while closing the rationalization gap).
- **Length-of-list matters.** Base Rules has 14 bullets; the most-violated rules sit deep in it. Splitting Base Rules would help but breaks the "flat `##` with inline bullets" contract — explicitly out of scope here.
- **Reinforcement asymmetry.** The git-commit rule is bolded, stamped `No EXCEPTION`, and has a substitute. It works. The build.sh rule had none of those and failed across instances. The strongest-habit rules need the strongest formatting, not equal billing.

### Phase gate (pilot → sweep)

Phase 2 begins only after Director observation across subsequent sessions confirms the pilot rules show reduced violation. No fixed N — Director gives explicit go-ahead (Director-set: violation frequency is qualitative; a fixed window may force a premature go/no-go). If the pilot fails to move the needle, retire this AC rather than expand the scope; the diagnosis was wrong.

### Disposition Log

- **F1** — Phase 2 In Scope header updated: "single pass, Director-set" added.
- **F2** — Implementation Notes "Carve-outs systematically weaken headline rules" paragraph updated: reframe-as-imperative case-by-case attribution added.
- **F3** — Implementation Notes Rewrite pattern's recognition-failure bullet updated: "inline in AGENTS.md, not split into a separate doc" attribution added.
- **F4** — AT5 wording tightened to "Director observes... gives explicit go-ahead... no fixed window"; Phase gate paragraph updated to match.
- **F5** — Out of Scope "Structural changes to AGENTS.md" entry attributed: stay strict for this AC.
- **F6** — In Scope "Files to modify" overlay line attributed: same-AC propagation, no follow-on overlay AC.
- **F-new-1** — Phase 2 In Scope expanded 6 → 8 items (added L44, L46); high-risk inventory expanded 8 → 10; Rewrite pattern got the **Gate-adjacency check** as a short imperative addendum.

### Decisions inherited from feedback.md

- **No hooks.** governa is agent-agnostic; harness-level enforcement is out of scope.
- **No agent-local memory.** AGENTS.md File-Change Discipline forbids it.
- **Tighten the rule at governa source.** Changes land in AGENTS.md (and conforming overlay) so every governa-family repo inherits on next sync.

### Propagation across canonical sites

**Three sites, not two.** AGENTS.md rule text lives in (1) governa source `AGENTS.md`, (2) `internal/templates/base/AGENTS.md` (the consumer-repo seed), and (3) any overlay-specific `internal/templates/overlays/<type>/files/AGENTS.md.tmpl`. Update all three when the rule applies. "Overlay templates" alone is incomplete — the Governed Sections rule's *"and other canon files"* clause covers the base template. (Phase 1 missed site (2) on first pass; caught and corrected same session.)

Per-rule judgment whether the rewrite applies to consumer repos:

- `./build.sh` rule — partly governa-internal (mentions `./build.sh` and `staticcheck`); the *shape* (positive lead + named failure modes + diagnostic trigger) propagates, but consumer-repo wording may differ. Currently in governa source + base template; not in any overlay AGENTS.md (doc overlay omits it; code overlay has no AGENTS.md.tmpl and inherits base unchanged).
- AC-body labels, scope expansion, paths, memory, file re-reads — universal agent behavior; propagate verbatim to every site that carries the same rule.

## Acceptance Tests

These ATs are starting drafts. The AC critique gate must sharpen them — particularly the behavioral ATs, which depend on the rewrite content settling.

**AT1** [Automated] — `rg` against the Phase 1 rewritten `./build.sh` rule confirms presence of (a) positive lead "Run `./build.sh`", (b) named failure tokens "`go test`, `go vet`, `go fmt`, `go fix`, or `staticcheck`", and (c) diagnostic trigger phrase "two or more of these in sequence".

**AT2** [Automated] — `rg` against the Phase 1 rewritten AC-body-labels rule confirms positive lead ("Name tests… by behavior") precedes the prohibition.

**AT3** [Automated] — Each Phase 2 rewritten rule passes the equivalent shape check: positive lead, named failure mode, diagnostic trigger if applicable.

**AT4** [Automated] — The overlay template `internal/templates/overlays/doc/files/AGENTS.md.tmpl` is updated for every rewrite the AC marks as universal-applicable; `./build.sh` validation passes.

**AT5** [Manual] — Director observes across subsequent sessions whether the originating failure recurs after Phase 1; gives explicit go-ahead when satisfied. No fixed window (Director-set: qualitative judgment, not session-counted). Director's go-ahead gates Phase 2.

**AT6** [Manual] [Post-release verification] — Across the consumer repos that re-sync after the next release, the rewritten rules render correctly in their AGENTS.md files.

## Documentation Updates

- `AGENTS.md` — primary rule rewrites (the AC's main artifact).
- `internal/templates/overlays/doc/files/AGENTS.md.tmpl` — propagated rewrites per the per-rule judgment in Implementation Notes.
- `docs/development-guidelines.md` — only if the rewrite pattern itself (positive substitution + named failures + diagnostic triggers + carve-outs as imperatives) deserves a meta-rule entry. Director call during critique.
- `docs/build-release.md` — no change needed; it already explains why direct invocations skip the pipeline.
- `CHANGELOG.md` — release row added at release prep, not now.

## Director Review

`None`. All six initial questions resolved in Critique Round 1; resolutions attributed inline (Phase 2 In Scope header, Out of Scope structural-change entry, Implementation Notes carve-out and rewrite-pattern paragraphs, Phase gate paragraph, AT5, Files to modify overlay line).

## Critique

### Round 1

#### F1 [Minor] — Phase 2 scope: single pass vs. subdivide

Director's call: single pass. The rewrite pattern is the unit being validated, so applying it broadly tests the pattern's reach. Confirms Operator's lean.

#### F2 [Minor] — Carve-out treatment: reframe vs. remove

Director's call: reframe-as-imperative case-by-case. Removing carve-outs may regress legitimate uses (smoke tests, historical references); reframing preserves the use while closing the rationalization gap. Confirms Operator's lean.

#### F3 [Minor] — Diagnostic triggers: AGENTS.md inline vs. split into development-guidelines.md

Director's call: AGENTS.md inline. The trigger is the rule's enforcement mechanism — separating reintroduces the classification gap the rewrite is meant to close. Confirms Operator's lean.

#### F4 [Minor] — Phase 1 → Phase 2 validation window

Director's call: Director observation, no fixed N. Violation frequency is qualitative; a fixed window may force a premature go/no-go. Confirms Operator's lean. AT5 + Phase gate paragraph updated to remove fixed-N language.

#### F5 [Minor] — Structural change to Base Rules in this AC

Director's call: stay strict. Structural change is a separate contract change requiring its own director approval and would muddy the validation signal. Confirms Operator's lean.

#### F6 [Minor] — Overlay propagation: same AC vs. follow-on

Director's call: same AC. The propagation rule requires same-pass updates; splitting introduces drift risk. Confirms Operator's lean.

### Round 1 Summary

1. **Unresolved findings.** None.
2. **Residual risks accepted.** Pilot validation is qualitative (no fixed N); risk that Director observation drifts or that "a few sessions" is insufficient signal. Mitigated by AC131 staying in `docs/` indefinitely until either go-ahead or retirement.
3. **Coverage.** All six Director Review questions resolved. ATs reviewed for alignment with settled decisions; AT1–AT4 and AT6 unchanged, AT5 tightened. Not reviewed this round: rewrite content for the six Phase 2 items (out of scope for critique — material to be drafted at Phase 2 start), and per-rule overlay-propagation judgments (deferred to implementation per Implementation Notes).
4. **Verdict.** `no blockers`.

### Round 2

#### F-new-1 [Major] — at-risk inventory incomplete; gate-adjacency failure shape unrecognized

Live demonstration: Operator drafted a `git commit` command unsolicited at the end of Phase 1 (violating Approval Boundaries L44/L46), then on being called out, offered to "store in feedback for me to track this session" — proposing the very agent-local-memory pattern L68 forbids and which AC131 itself cites as reason for not using hooks. Both violations share a shape the Round 1 inventory missed: a strong gate (`Never run git commit`, `Codify... not in agent-local memory`) blocks the literal action, and the agent substitutes an adjacent unguarded behavior (drafting the message; storing in a feedback file). Methodology gap: the Round 1 classification inspected explicit prohibitions in isolation and did not enumerate substitutes around gated rules.

Director's call: add L44 + L46 to Phase 2 (single pass; total Phase 2 items now 8). Codify the gate-adjacency check in the Rewrite pattern as a **short imperative** — no long-winded sub-rule (long sub-rules bloat AGENTS.md and get missed; short imperatives stick). Disposition: Phase 2 In Scope expanded; high-risk inventory expanded with L44/L46 entries explicitly tagged as gate-adjacency cases; Rewrite pattern got the **Gate-adjacency check** addendum (three sentences).

### Round 2 Summary

1. **Unresolved findings.** None.
2. **Residual risks accepted.** The gate-adjacency check is a heuristic for *future* inventory passes; the current expanded inventory may still miss adjacent substitutes around L32 (`Do not create files... without authorization`) or L47 (`Never run the release command`). Mitigated by L44 covering "preparing" broadly. If future violations surface adjacent gaps, open Round 3 rather than re-classify silently.
3. **Coverage.** Reviewed: inventory completeness against gate-adjacency, Phase 2 scope expansion, Rewrite pattern amendment, AT3 implicit count change (6 → 8 — wording unchanged, scope tracks the In Scope list). Not reviewed: Phase 1 already-shipped rewrites (out of scope; if the new heuristic suggests Phase 1 wording needs revision, that's a separate AC).
4. **Verdict.** `no blockers`.

## Status

`IN PROGRESS — Phase 1 complete; Phase 2 awaiting Director go-ahead per AT5`.

- **Phase 1 landed (2026-05-11):** Both rewrites propagated to all three canonical sites — governa source `AGENTS.md`, the base template `internal/templates/base/AGENTS.md` (consumer-repo seed), and the doc-overlay template `internal/templates/overlays/doc/files/AGENTS.md.tmpl` (which carries only the AC-labels rule, not build.sh). `./build.sh` rule rewritten in governa source lines 83–84 and base template lines 83–84. AC-body-labels rule rewritten in governa source line 93, base template line 93, and overlay line 85. (Initial Phase 1 commit missed the base template — caught and corrected same session before commit.)
- **Build validation:** `./build.sh` passes; DOC smoke passes (overlay rendered + validated in temp dir).
- **AT status:** AT1 ✓; AT2 ✓; AT4 ✓; AT3, AT5, AT6 deferred to Phase 2.
- **Critique Round 2 (2026-05-11) closed `no blockers`:** F-new-1 expanded Phase 2 to 8 items (added L44, L46) and added the Gate-adjacency check to the Rewrite pattern.
- **Phase 2 gate:** Director observation across subsequent sessions; explicit go-ahead unblocks the eight-item sweep listed in In Scope Phase 2.
- **AC lifecycle:** stays in `docs/` until Phase 2 ships and the next release prep runs.
