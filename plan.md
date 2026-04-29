# governa Plan

## Product Direction

Provide a narrow, usable governance template that bootstraps new repos. Governa is a one-time apply — after bootstrap, the consumer repo owns all governance files and evolves them independently. Template improvements are adopted by having consumer-repo agents read governa's source and cherry-pick what's useful.

## Ideas To Explore

Ideas captured for future reference. Prefix each with `IE<N>:` (sequential N) for stable references. Entries come in two shapes: (a) **pre-rubric idea** — `IE<N>: <one-liner>` awaiting director discussion and the objective-fit rubric (see `AGENTS.md` Approval Boundaries); (b) **pointer to a drafted AC stub** not yet scoped through the critique cycle — `IE<N>: <one-liner> → docs/ac<N>-<slug>.md`. A shape (a) entry that clears the rubric converts to shape (b) at AC-draft time (keeping the same `IE<N>` number) rather than being removed, so the entry persists as a pointer until the pointed-to AC ships. Remove entries when the underlying idea is closed: rejected, retired, or (for AC pointers) the pointed-to AC has shipped and its file has been deleted. This section is not a historical record.

- IE6: formalize advisory-log mechanism for `docs/advisories/` — intake process, severity tiers, per-consumer ledger, lifecycle rules.
- IE10: split `internal/preptool` → `github.com/queone/governa-preptool` library core + template-side adapter, *if* the convention-coupling test admits a clean split; canonical-fix venue for the programVersion advisory → [docs/ac100-preptool-split-attempt.md](docs/ac100-preptool-split-attempt.md)
- IE11: governa shrinkage — trim to `internal/templates/` + `docs/` + thin `cmd/governa` after extraction ACs settle → [docs/ac101-governa-shrinkage.md](docs/ac101-governa-shrinkage.md)
