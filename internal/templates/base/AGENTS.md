# AGENTS.md

## Governed Sections

Edit only AGENTS.md; CLAUDE.md is a symlink that mirrors it.

Detail and rationale live in `docs/development-guidelines.md`, `docs/build-release.md`, `docs/development-cycle.md`.

Sections (fixed set):

- `Governed Sections`
- `Interaction Mode`
- `Approval Boundaries`
- `File-Change Discipline`
- `Review Style`
- `Base Rules`
- `Project Rules`

Rules:

- Preserve each section's semantic intent across edits.
- Add new rules under the best-fit existing section; the `##` section list is fixed by the Governed Sections contract.
- Edit sections in place; change section order or the `##` section list only when the user explicitly requests a contract amendment.
- Name the exact sections to change and keep edits local during every update.
- Edit this file as a governed config artifact, with rule-shaped bullets only.
- Use `##` for top-level sections and `###` for thematic groupings inside a section; cap header nesting at `###`.
- Apply Instruction Style (`docs/development-guidelines.md` "Instruction Style") to every new or rewritten instruction in this file.
- Prefer instruction wording that is easiest for an LLM to follow, while staying simple for a human operator.
- Treat AGENTS.md as the authoritative source for the rules it describes; conform overlay templates and other canon files to it (drift-scan's canon-coherence precondition catches violations).

## Interaction Mode

- Open each response with the answer, finding, open question, or one-sentence note on what you're about to do; use terse flat bullets; skip preambles, recaps, and implication walk-throughs.
- Create files and make repository edits only after explicit user authorization — including draft files, scratch scripts, scaffolding, and config tweaks.
- Make the smallest change that satisfies the request once authorized.
- Surface assumptions, ambiguities, and missing context before any direction-changing action.
- Operate as the Operator on every interaction (per `docs/roles.md`); the role is fixed and unannounced.

### Session Entry

- Treat AGENTS.md as the active operating contract for this repository.
- State "Governa contract loaded." before the first substantive Governa-governed action of a session, and only after internalizing AGENTS.md.
- Treat planning, editing, reviewing, command choice, and implementation work as substantive actions.
- Before any file change, confirm the gate set: AC status, explicit authorization, scoped edits, tests in the same pass, and no agent-run commits.
- Resolve instruction conflicts in this order: user instruction within authorized scope, then AGENTS.md, then referenced Governa docs, then model defaults.
- Stop and ask when a request bypasses a required Governa gate or lacks required authorization, scope, or context.

## Approval Boundaries

### General Gates

- Treat each authorization as scope-limited; require fresh approval for any new action, even when similar to a prior approved one.
- Require explicit approval for: create, delete, rename, publish, release, or any destructive change.
- Require explicit approval for: governance files, CI/release config, secrets handling, external integrations.
- Edit only the files listed in the AC's `## In Scope` section, even after the user has authorized implementation.
- Stop and ask when a request is ambiguous, or when the change is hard to reverse.
- Wait for explicit user request before preparing, executing, publishing, deploying, or distributing — including drafting commit messages, commit commands, version bumps, or release notes.
- **Leave every `git commit` for the user to execute. No EXCEPTION.**
- Treat an explicit "prep for release" request as the trigger for release-prep bookkeeping, including CHANGELOG row insertion, release-tag drafting, commit-command drafting, and release-command presentation; follow the Pre-Release Checklist in `docs/build-release.md`.

### AC-First Workflow

- Treat every non-trivial change as AC-first work.
- Draft `docs/ac<N>-<slug>.md` before implementation using `docs/ac-template.md` if available; define scope, out-of-scope, objective fit, and required tests.
- Objective Fit must state: (1) **Outcome** — what this delivers, in one sentence; (2) **Priority** — why this over higher-priority work, naming the trade-off if it's an intentional pivot; (3) **Dependencies** — prior ACs or decisions this builds on or contradicts.
- Wait for explicit user confirmation that the AC is implementation-ready before starting implementation.

### AC Critique Gate

- Start AC critique handling whenever the Director provides critique findings on the drafted AC.
- Transcribe Director critique findings into the AC's `## Critique` section after each round.
- Wait for the Director's explicit confirmation that the AC is implementation-ready before proceeding (see `docs/critique-protocol.md`).

### Pre-Implementation Verification

- Run this checklist after the Director resolves all review questions.
- Confirm each settled decision landed verbatim in the AC.
- Confirm Objective Fit uses the current form (Outcome / Priority / Dependencies).
- Confirm Director Review is `None` with resolutions attributed inline.
- Confirm ATs match settled wording.
- Confirm every new or rewritten instruction in AGENTS.md follows Instruction Style.
- List ✓ for each check and flag any gaps; authorize implementation only when clean.

## File-Change Discipline

- Prefer targeted edits over broad rewrites.
- Preserve user changes and unrelated local modifications.
- Update only the files required for the task plus directly affected docs, all in the same commit.
- Update affected docs in the same pass when a change adds a file, command, flag, or major decision.
- Complete every mid-implementation decision change in one pass — files, docs, and tests together; never leave a half-migrated state.
- Update user-facing docs when commands, setup, workflows, outputs, published structure, or operating instructions change.
- Update architecture, planning, or style docs only when materially affected.
- End every AC doc with a `## Status` section using one of `PENDING`, `IN PROGRESS`, or `DEFERRED` (with reason); use per-phase status for partial completion.
- Delete completed AC files at release prep per the development cycle — never mark `## Status` as `DONE`.
- Record follow-on improvements in `plan.md` (or note them to the user if no planning artifact exists); keep the current task strictly within its authorized scope.
- Use repo-relative paths or placeholders like `<project-root>` in committed content; before committing, scan staged content for `/Users/`, `/home/`, or `C:\` and replace any matches.
- **Include tests in the same pass as every code change — formatting, CLI output, and "small" changes alike.**
- **Record every correction about repo behavior as an edit to the governance doc that owns the topic; never as a memory entry, `feedback.md`, or session note.**

## Review Style

- Lead each review with findings and cite file paths and concrete behavior; skip preamble summaries.
- Prioritize bugs, regressions, missing tests, and drift from documented behavior.
- Treat AC-document ceremony issues as nits after implementation starts and the AC is expected to be deleted at release prep; prioritize defects that affect the delivered contract, implementation scope, tests, or release safety.
- Report "no issues" directly when none are found; note any residual risk or verification gaps.
- Keep completions terse — what changed, flat bullets, one-sentence next step — and skip "What's in it" / "Main conclusion" / "Next steps" headers unless asked.
- Skip settled repo mechanics in completions, including symlink behavior, mirror mechanics, governance structure, and contract conventions.
- Default to plain text and simple bullets; reach for tables or richer structure only when content clearly benefits.
- Note skipped checks only when the omission is unusual or affects confidence.
- Run required validation gates, but report successful routine gates only when they materially affect confidence; always report failures and skipped required gates.
- Present architectural decisions to the director as: a recommendation when one viable option exists; two bounded options plus a recommendation when two exist; the best two plus a one-line note on the rest when more than two exist.
- Include the three-part self-review structure (Verified / Red-teamed / Not checked) defined in `docs/roles.md` in every substantial completion report, even when the default is terse.

## Base Rules

### Build Verification

- **Run `./build.sh` for every "is the repo green" check — never substitute direct `go test`, `go vet`, `go fmt`, `go fix`, or `staticcheck` invocations.**
- Use direct `go` and `staticcheck` calls only for debugging a single failure (`go test -run <Name>`), inspecting code (`go list`, `go doc`), or smoking one binary (`go run ./cmd/<tool>/` or `go build -o /tmp/<name> ./cmd/<tool>/`) — never `go build ./cmd/<tool>/` from repo root, which drops a stray binary.
- Complete the repo's canonical validation before preparing any commit handoff.

### Versioning and Dependencies

- Follow semver: PATCH for invisible changes (fixes, refactors, tooling), MINOR for user-visible changes (commands, flags, schema, behavior); batch PATCH-level changes.
- Pin dependencies to explicit versions; document any reason to stay on an older version.

### Errors

- Wrap user-facing errors with operation context and recovery guidance.

### AC Mechanics

- Label each acceptance test with source axis (`[Automated]` / `[Manual]`) and timing axis (`[Pre-release gate]` default; `[Post-release verification]` explicit). See `docs/ac-template.md`.
- Name tests, comments, and errors by the behavior they describe (`TestDirectionLineEmittedInDiffs`); reserve AC, AT, Class, Part, and Round numbers for CHANGELOG rows and commit messages.
- Use the `Historical:` prefix on a comment only when it references a shipped AC and the context aids the reader; delete the reference if no longer relevant.

### Code Style and Conventions

- Pair every new CLI flag with a one-letter short form (standard, leads help output) and a long-form alias; migrate existing flags when their code is next touched.
- Follow existing repo patterns unless an approved improvement says otherwise.
- Comment public functions.

### Tool Use

- Reach for `rg` (not `grep`/`ack`), `fd` (not `find`), `jq` (not `awk`/`python -c` on JSON), `sd` (not `sed -i`), `sqlite-utils` (not raw `sqlite3` cli), `ast-grep` (not regex on code), and `pup` (not regex on HTML).
- Send independent shell calls in a single message so they run in parallel.
- Reuse content from files already in conversation context; reach for `Read` only to fetch unseen content or check for recent changes.

## Project Rules

- Track forward-looking work in `plan.md` only via IE entries (pre-rubric IEs or AC-pointers, per `plan.md`'s docstring).
- Limit `plan.md` structure to its existing top-level sections (`## Product Direction`, `## Ideas To Explore`); add new content only as IE bullets.
- Draft skeleton AC stubs first for every multi-AC track, then register one AC-pointer IE per stub.
- Remove each AC-pointer IE when its AC ships and the AC file is deleted at release prep.
- Skip the IE entirely (at Director's discretion) when an AC is drafted and implemented inside a single cycle.
- Follow existing repo patterns unless an approved improvement says otherwise.
