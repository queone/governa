# Changelog

| Version | Summary |
|---------|---------|
| Unreleased | |
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
