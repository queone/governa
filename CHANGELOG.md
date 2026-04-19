# Changelog

| Version | Summary |
|---------|---------|
| Unreleased | |
| 0.41.0 | AC64: flag convention + mdcheck step + critique ownership rule |
| 0.40.0 | AC62+AC63: consumer-feedback cleanup + sync feedback-closure advisor |
| 0.39.0 | AC61: slim README template + arch.md Core Files section |
| 0.38.0 | AC60: ./prep.sh release-staging tool + 2-step checklist |
| 0.37.0 | AC58+AC59: section-order advisor + governance doc canonicalization — `detectSectionOrderDrift` emits Advisory Notes on `keep` files showing current vs template order (rename-aware). `plan.md` reordered (IE ↔ Deferred) and `Current Platform` removed (dup arch.md). Role-assignment rule consolidated in `AGENTS.md`; adversarial-check principle consolidated in `docs/roles/README.md` (dev/qa pointers; maintainer unchanged). Brittle step-N refs replaced with stable section refs. |
| 0.36.0 | AC57: monotonic AC numbering across release-prep deletions — `nextACNumber` consults `git log --all --pretty=%B` alongside `docs/` so deleted ACs still contribute to the max. Two-layer seam: `gitACMaxFn` + `extractACNumbersFromGitOutput` parser (handles `AC53+AC54` composites). Disk-only fallback + stderr warning on git-unavailable. `docs/ac-template.md` preamble + `docs/development-cycle.md` step 2 rewritten (overlay + example copies) to codify the rule for consumer-repo DEVs. |
| 0.35.1 | PATCH: codify write-surface boundary across role files — DEV owns the AC file and implementation/content files; QA's write surface is limited to findings (chat or `docs/ac<N>-<slug>-critique.md` per Companion Artifacts). Clarification added to `docs/roles/dev.md` and `docs/roles/qa.md` plus overlay + example copies (10 files total). |
| 0.35.0 | AC56: acknowledged drift for sync review — add `governa ack` with `-m/--reason` and `-x/--remove`, manifest-backed dual-SHA acknowledgments, `## Acknowledged Drift` review rendering, stale-ack re-flagging, and orphan pruning. |
| 0.34.0 | AC55: governance refinement — step 5 compacted (+consumer-credit propagated to overlay); bolded-title checklist convention codified; AC-template `### New files`→`### Files to create` + `## Companion Artifacts` preamble (critique/feedback/dispositions); `.governa/` metadata layout (manifest, proposed/, sync-review.md, feedback/) w/ auto-migration; `-feedback.md` persists to `.governa/feedback/` at release prep; `enhance -r` emits `## Consumer Feedback`; template-owned godoc; README rewrite. |
| 0.33.0 | AC53+AC54: sync quality bundle (utils v0.32.1 feedback) — `plan.md` skeleton-section recommender downgrades adopt→keep on shape match; per-section bullet-removal advisory on adopt; `closes <consumer>:IE<N>` CHANGELOG convention with go.mod-aware identity; `## Template Changes` truncation 300→500 to match cap; AGENTS.md two-bounded-options bullet; AC54 `t.Setenv` race fix in buildtool_test.go (addresses utils AC4 AT11 flake); `## Sandboxed Execution` doc note in build-release.md. |
| 0.32.1 | PATCH: codify two review-output rules across governance docs and propagate to overlay+example copies — `docs/build-release.md` step 9: present only the release command (no trailing commentary about wrapper routing or prompts); `AGENTS.md` Review Style: do not note skipped checks when the skip is already implied by repo rules or the review scope. |
| 0.32.0 | AC52: template emits extracted `internal/buildtool` and `internal/reltool` packages with delegator entrypoints, matching canonical and closing day-one template drift; renderer skip list extended for non-Go-stack CODE; `docs/development-cycle.md` step 3 codifies QA-critique iteration loop; propagation table updated. (addresses utils feedback from v0.31.0 sync) |
| 0.31.0 | AC51: sync quality bundle — manifest sha reflects on-disk state, stack-aware `.gitignore` (Go block), step 5 code-block example restored, Adoption Items surfaces new sections, per-sync feedback obligation codified in template, `## Template Changes` summary, `.governa-proposed/` cleanup at sync start, consumer-feedback CHANGELOG credit convention (addresses utils feedback from v0.30.0 sync) |
| 0.30.0 | AC48+AC49+AC50: signal hygiene (skip `.governa-proposed/` and governa-owned paths), context-aware `## Status`, CHANGELOG stub template + format spec with ≤ 500 char summaries, role files name counterparts explicitly via intro + `## Counterparts` section |
| 0.29.0 | AC47: sync polish bundle — `type: (inferred)` provenance line, suppressed redundant `collisions:`, `.governa-proposed/` covers keep-with-advisory via shared predicate, truthful ABOUT.md, AGENTS.md Purpose rewording |
| 0.28.0 | AC46: operator clarity bundle — consistent first-sync/re-sync assessment, dropped `recommendation:` line, structured conflict descriptions (heading + numbered steps), merge hint for `.gitignore`, `AGENTS.md` intent note, `## Next Steps` block |
| 0.27.0 | AC45: sync conflict follow-through — non-zero exit via `ErrConflictsPresent`, safe migration guidance in conflict description, truthful ABOUT.md, repo-relative stderr paths, `disposition:` label distinct from pre-sync assessment |
| 0.26.0 | AC44: agent-agnostic invariant — symlink-vs-regular-file conflict detection, `## Conflicts` review doc section, post-transform status line, manifest filters blocked symlinks, adopt count in drift summary, first-sync wording |
| 0.25.0 | AC43: scoring false positive fixes — `.governa-proposed/ABOUT.md` collision fix, scaffold file demotion, extracted-package demotion |
| 0.24.1 | Remove stale `bootstrap` entry from `scriptOnlyCommands` in template and example |
| 0.24.0 | AC42: module path migration from `github.com/kquo/governa` to `github.com/queone/governa` |
| 0.23.1 | Fail-safe: `governa sync` refuses to run inside the governa repo itself |
| 0.23.0 | AC41: rename `internal/bootstrap` to `internal/governance`, `docs/bootstrap-model.md` to `docs/governance-model.md` |
| 0.22.0 | AC40: imperative sync review — 2-category model (`keep`/`adopt`), imperative methodology language, structural observation promotion, consolidated Adoption Items section, repo-relative diff paths |
| 0.21.1 | Lean review doc (no inline diffs, points to `.governa-proposed/`), dead code cleanup, doc alignment |
| 0.21.0 | Proposed files in `.governa-proposed/`, standing drift promoted to recommendation, stronger adoption language |
| 0.20.0 | Standing drift: inline diffs for un-adopted changes, director reporting instruction |
| 0.19.1 | Advisory notes: instruct agents to report standing drift to director |
| 0.19.0 | Standing drift detection: surfaces un-adopted template changes from previous sync rounds; sync lifecycle guidance in DEV role |
| 0.18.2 | Sync review: version transition header, AC workflow nudge for cherry-picks |
| 0.18.1 | Sync review and DEV role: nudge AC workflow for cherry-picks before applying |
| 0.18.0 | AC39: preamble scoring, keep-with-missing-sections advisory, section rename detection |
| 0.17.1 | Sync review: show proposed content for non-markdown content-changed files |
| 0.17.0 | AC38: section-level overlay scoring, compact diffs for small deltas, enhance subsection drill-down; flattening rule in Governed Sections |
| 0.16.1 | Fix: remove governa-specific propagation rule from consumer maintainer template; add `governa-sync-review.md` to .gitignore templates |
| 0.16.0 | AC37: skout enhance — session persistence, AC critique gate, terse completion guidance, `ac<N>-<slug>.md` path format, planning-artifact fallback, partial-completion status |
| 0.15.7 | Fix: TEMPLATE_VERSION now updated on re-sync instead of skipped; bookkeeping note clarifies version markers are not cherry-picks |
| 0.15.6 | Sync review: soften review artifact disposition to respect repo governance |
| 0.15.5 | Sync review: version coherence note and review artifact disposition in bookkeeping section |
| 0.15.4 | QA role: build validation scope rule, rule ordering aligned |
| 0.15.3 | DEV role: rename Using Sync/Enhance to Governa Templating Maintenance, distinguish consumer sync from governa enhance |
| 0.15.2 | Sync review doc: bookkeeping note clarifying TEMPLATE_VERSION and manifest are not review items |
| 0.15.1 | Rename review doc to `governa-sync-review.md`; role traceability wording across AGENTS.md and all role files |
| 0.15.0 | AC36: sync review classifies changed sections as structural or cosmetic for triage |
| 0.14.0 | CLI help: version header, description, `h` alias; bold test-gate rule in AGENTS.md |
| 0.13.4 | Pre-release checklist: tag and working-tree check moved to step 1 |
| 0.13.3 | Sync review methodology: report and feedback steps for director visibility and governa improvement loop |
| 0.13.2 | Sync review doc: 7-step evaluation methodology with report and feedback loop, IE1-IE2 in plan |
| 0.13.1 | AC35: governance-first latch rule, tag-check and AC-to-file rules codified |
| 0.13.0 | AC34: roles directory rename, delivery-model integration, director reference role |
| 0.12.0 | AC33: sync content-change detection, AC deletion timing aligned to release prep |
| 0.11.1 | TEMPLATE_VERSION semantic clarified, development-cycle wording aligned |
| 0.11.0 | AC32: governance baseline from skout — rubric inline, default-to-maintainer, terse output, release command rule |
| 0.10.2 | Build-release template restructured, stale doc refs fixed, release message limit enforced in checklist |
| 0.10.1 | Enhance: removed -a/--apply and -t flags, deprecated .template-proposed |
| 0.10.0 | AC31: enhance detects existing enhance ACs and prompts replace/update/new on collision |
| 0.9.2 | Color: 256-color capability detection with basic ANSI fallback, test isolation fixes, FormatUsage coverage 93.3%, stale overlay/propagation docs fixed |
| 0.9.1 | Shared color package: 256-color ANSI standardization, superset palette (13 functions), ShowPalette() for agent troubleshooting, escape code regression tests |
| 0.9.0 | AC30: `sync` replaces `new` and `adopt` — single subcommand with auto-detection and interactive prompts, per-subcommand help output, repo-wide command migration |
| 0.8.2 | AC29: AGENTS.md preamble and Project Rules section, AC-first workflow rule, ac-example.md removed, ac-template improvements from skout, adopt knowledge/ skip, review doc verbosity and structural comparison, cherry-pick false positive fix |
| 0.8.1 | AC28: overlay color.go replaced with shared internal/color import, review doc always at repo root, finer review categories (cherry-pick vs no action likely), identical file detection, DEV role AC-file-first rule |
| 0.8.0 | AC25–AC27: build-time drift check, zero-flag adopt with auto-inferred params, content-aware adopt review document replacing .template-proposed files |
| 0.7.2 | AC26: zero-flag adopt with auto-inferred params, manifest stores adopt parameters for idempotent re-adopt |
| 0.7.1 | AC25: build-time governance drift check via enhance self-review, advisory summary after binary install |
| 0.7.0 | AC24: renamed repokit to governa — module path, binary, docs, templates; legacy manifest backward compatible |
| 0.6.4 | AC23: Using Adopt workflow in CODE overlay DEV role, drift summary for enhance and adopt output |
| 0.6.3 | AC22: portable governance rules from skout, enhance workflow in DEV role doc, fixed dangling AGENTS.md symlink |
| 0.6.2 | README: enhance purpose clarified (templating set + self-hosted governance), CLI description updated |
| 0.6.1 | AC21: enriched ac-template.md and ac-example.md with inline coaching, sub-headed scope, numbered ATs |
| 0.6.0 | AC20: self-review enhance without `-r`, retired `cmd/bootstrap` |
| 0.5.0 | AC19: installable `repokit` CLI binary with embedded templates, subcommands, GitHub version check, module path github.com/kquo/repokit |
| 0.4.0 | AC18: templates moved to internal/templates/, bootstrap refactored to fs.FS abstraction |
| 0.3.4 | AC17: ## Why section in generated READMEs, adopt advisory for missing Why, IE cleanup rule |
| 0.3.3 | AC16: README consolidated — Quick Start, Intended Use, Operating Model, Operator Guide collapsed into Modes and Design |
| 0.3.2 | AC15: AC filename convention surfaced in ac-template.md, development-cycle.md, and docs/README.md files (stale ac_<id> guidance corrected) |
| 0.3.1 | AC14: Ideas To Explore section in plan.md, pre-rubric vs Priorities boundary |
| 0.3.0 | AC13: AC naming convention simplified to acN-slug.md and # ACN titles (hard cutover) |
| 0.2.10 | Build enforces programVersion const on installable binaries |
| 0.2.9 | Release message limit raised from 60 to 80 characters |
| 0.2.8 | Release governance: concrete change-based messages, canonical command only by default |
| 0.2.7 | Cross-platform absolute-path detection in enhance marker rules (added Linux /home/ coverage) |
| 0.2.6 | README cleanup: current-state wording, single Quick Start command reference, removed stale v0.1.1 block |
| 0.2.5 | AC cleanup: completed AC files removed; hard-gate cleanup rule in dev cycle, pre-release checklist, and role docs |
| 0.2.4 | AC-012: DOC overlay enrichment — platform-specific publishing notes, voice.md and calendar.md variants, DOC-specific agent roles |
| 0.2.3 | AC-011: CODE overlay enrichment — template upgrade guidance, release artifacts section, worked AC example |
| 0.2.2 | AC-010: go fmt failures now build-breaking; release tool recovery guidance; doc drift fixes; reltool coverage 57.8% |
| 0.2.1 | AC-009: maintainer role for single-agent repos combining DEV and QA with self-review requirement |
| 0.2.0 | AC-008: agent role bootstrap pattern with `docs/agent-roles/` for DEV and QA roles; bootstrap `--help` exits cleanly; consistent CLI usage formatting across all commands; `-?` alias for bootstrap |
| 0.1.8 | AC-007: adopt mode section-level patching for AGENTS.md; missing governed sections appended in template order; existing content never modified; domain coverage 71.4% |
| 0.1.7 | Release message 60-char limit enforced in reltool, overlay templates, and rendered examples; docs updated |
| 0.1.6 | AC-006: safe refresh path improvements — Phase 1: constraint-level governance comparison and section-level file diffing replace keyword signals and whole-file diffs; Phase 2: `.repokit-manifest` written at bootstrap with dual checksums enables three-way enhance comparison (user vs template vs both changed); Phase 3: classification, marker, and signal logic refactored to data-driven rule tables; Phase 4: `--apply` flag writes `.template-proposed` files for assisted merge without overwriting live targets; `planRender` refactored into `planCanonical` + `applyAdoptTransforms`; domain coverage 63% → 70.7% |
| 0.1.5 | AC-005: development knowledge layer — new `docs/development-guidelines.md` template with 8 reusable engineering guidance sections (identifier strategy, migrations, external integration, propagation, error handling, testing, dependency hygiene, doc alignment); new `docs/knowledge/` directory for deeper supporting notes that expand guidelines topics; repokit seeded with project-specific guidelines and template-propagation knowledge entry; CODE example regenerated |
| 0.1.4 | AC-004: plan.md template cleanup — CODE plan template now has six sections (Product Direction, Current Platform, Objective-Fit Rubric, Priorities, Deferred, Constraints); removed process guidance and Notes that duplicated `docs/development-cycle.md`; repokit's own `plan.md` cleaned of completed items, given real Deferred and Constraints entries; examples regenerated |
| 0.1.3 | Renamed module from `repo-governance-template` to `repokit` across go.mod, all Go imports, README, arch, plan, overlay templates, and rendered examples; AC-003 test coverage push: bootstrap 75%, buildtool 36%, reltool 39%, domain coverage 63%; AC-003 acceptance targets adjusted to reflect subprocess-dependent coverage ceiling |
| 0.1.2 | Operator guidance (R2): Quick Start section in README with CODE, DOC, and enhance examples; refreshed milestone text to reflect shipped state; corrected enhance wording across all operator docs; fixed enhance flow step ordering in bootstrap-model; reduced `scripts/README.md` to pointer; migrated overlay wishlists to `plan.md` |
| 0.1.1 | Enhance mode now produces AC docs instead of standing `docs/enhance-report.md` report; deterministic candidate ranking (`accept`+`portable` > `adapt`+`needs-review`); no file created when no actionable improvements found; dry-run support; `RunEnhance` exported for testability; bootstrap coverage improved from 44% to 57% |
| 0.1.0 | Deterministic Go bootstrap tooling for `new`, `adopt`, and `enhance`; `CODE` and `DOC` overlays with rendered examples; Go-based `cmd/build` and `cmd/rel` workflows with thin shell wrappers; self-hosted root governance artifacts so this repo operates as a governed `CODE` repo; path-safe enhancement reporting and path-hygiene rules; terminal coloring in build and release tooling; QA review fixes: Go-stack detection uses word-boundary matching, `color.go.tmpl` skipped for non-Go stacks, release tool shows `git status` before staging, `build.sh` routes single-arg semver to `cmd/rel`, root `AGENTS.md` symlinked to `base/AGENTS.md`, `go vet` and `staticcheck` now fail the build on errors, `.gitignore` added for template and generated repos |
