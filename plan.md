# governa Plan

## Product Direction

Provide a narrow, usable governance template that bootstraps new repos. Governa is a one-time apply — after bootstrap, the consumer repo owns all governance files and evolves them independently. Template improvements are adopted by having consumer-repo agents read governa's source and cherry-pick what's useful.

## Ideas To Explore

Ideas captured for future reference. Prefix each with `IE<N>:` (sequential N) for stable references. Two kinds: (a) **pre-rubric IE** — `IE<N>: <one-liner>`, awaiting director discussion and the objective-fit rubric (see `AGENTS.md` Approval Boundaries); (b) **AC-pointer** — `IE<N>: <one-liner> → docs/ac<N>-<slug>.md`, pointing at a drafted AC stub not yet through critique. A pre-rubric entry that clears the rubric converts to an AC-pointer at AC-draft time, keeping its `IE<N>` number. Remove entries when the idea is rejected, retired, or (for AC-pointers) the AC has shipped and its file deleted. Not a historical record.

- IE14: `governa drift-scan resolve <ac-path> <Q#> <decision>` subcommand with fixtures — promotes R10.1–R10.4 to code (sync/preserve/defer/target-has-no-canon resolutions). Per-section mutation logic: move file between In Scope/Out Of Scope, append AT for sync, append CHANGELOG marker-backfill action for preserve, append plan.md IE entry for defer, attribute `(Director-set)` inline. New CLI surface in `cmd/governa/main.go` dispatch + new test fixtures per resolution path. Depends on IE12 (resolve must know the post-pre-fill emission shape).
