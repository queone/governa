# governa Plan

## Product Direction

Provide a narrow, usable governance template that bootstraps new repos. Governa is a one-time apply — after bootstrap, the consumer repo owns all governance files and evolves them independently. Template improvements are adopted by having consumer-repo agents read governa's source and cherry-pick what's useful.

## Ideas To Explore

Ideas captured for future reference. Prefix each with `IE<N>:` (sequential N) for stable references. Two kinds: (a) **pre-rubric IE** — `IE<N>: <one-liner>`, awaiting director discussion and the objective-fit rubric (see `AGENTS.md` Approval Boundaries); (b) **AC-pointer** — `IE<N>: <one-liner> → docs/ac<N>-<slug>.md`, pointing at a drafted AC stub not yet through critique. A pre-rubric entry that clears the rubric converts to an AC-pointer at AC-draft time, keeping its `IE<N>` number. Remove entries when the idea is rejected, retired, or (for AC-pointers) the AC has shipped and its file deleted. Not a historical record.

IE6: Codify "fold small adjacent improvements into current AC/release" anti-piecemeal heuristic in `AGENTS.md`

IE7: Codify "draft new ACs as multi-part (Part A/B/C) by default" structure in `docs/ac-template.md`

IE8: Codify "bundle consumer-agent review findings into one multi-part AC per cycle, one part per class" alongside the multi-part default


IE10: Divergence-classification procedure for canon-code drift — covers (a) reachability-check taxonomy beyond filesystem-shape examples (config flag, runtime env, build tag) AND (b) distinguishing structurally-unreachable canon branches (dormant by host shape, not drift) from locally-absent canon branches (consumer skipped a sync, real drift). Extends AC125's gate with the procedure that turns it into actionable classification. Subsumes prior IE11.

IE12: Flavor capability predicate for drift-scan emission gates — replace `cfg.Flavor == "code"` literal checks with a flavor-capability predicate (e.g., `HasExecutableCanon()`) so new flavors don't trigger per-flavor edits to every gate. Currently relevant gate: AC125's reachability-reminder emission.

IE13: drift-scan report-header missing emission of the documented "scan-asymmetry note" — `docs/drift-scan.md` line 17 claims the header carries it, but `internal/driftscan/driftscan.go writeReport` never emits a corresponding line. Doc/code drift between two governa-internal surfaces; discovered during AC125 implementation. Either implement the asymmetry-note emission or amend `docs/drift-scan.md` to remove the claim.


IE15: Canonize Canon Baseline Sync doctrine in `docs/drift-scan.md` — rule (overlay-tracked files adopted as whole-file canon snapshots, not hunk-merged) plus pure-canon vs mixed-content carve-out (hunk-level merge for files with consumer-local content interleaved with canon structure: AGENTS.md, README.md, .gitignore, CHANGELOG.md, arch.md, docs/development-guidelines.md, docs/build-release.md, etc.). Source: utils note 2026-05-06 (re-route from utils when AC127 drafting begins). Target AC127 (deferred for post-AC126 focus per Director 2026-05-06).
