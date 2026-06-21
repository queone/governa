# AC150 DOC Shell Release and Prep Parity

## Summary

DOC-flavor repos ship `rel.sh` + `cmd/rel/main.go` (stdlib-only, not governa-reltool despite what `overlay-scope.md` says), forcing a non-code content repo to keep a Go toolchain installed just to git-tag a release. Removing these takes un-lockable `missing-in-target` drift noise every cycle. This AC replaces them with a pure-shell `build.sh` carrying both validate and `vX.Y.Z "message"` release subcommands (reusing AC147 color/usage/release helper patterns), and adds a `build.sh prep` subcommand that gives DOC repos the same deterministic release bookkeeping CODE repos get.

Two-part delivery:
- **Part A** — DOC shell release: new `build.sh.tmpl`, remove `rel.sh.tmpl` + `cmd/rel/` from canon, fix `overlay-scope.md`
- **Part B** — DOC prep parity: `build.sh prep` subcommand (CHANGELOG row insertion, AC-file deletion, `plan.md` IE sweep, pre/post validation)

## In Scope

### Part A — DOC shell release

- `internal/templates/overlays/doc/files/build.sh.tmpl` — new: shell build.sh with validate and `vX.Y.Z "message"` release subcommands; reuse AC147 color/usage/release helper functions
- `internal/templates/overlays/doc/files/rel.sh.tmpl` — delete (superseded by build.sh)
- `internal/templates/overlays/doc/files/cmd/rel/main.go.tmpl` — delete
- `internal/templates/overlays/doc/files/cmd/rel/color.go.tmpl` — delete
- `governa/overlay-scope.md` — fix line 36: replace stale `rel.sh`, `cmd/rel/main.go`, `governa-reltool` description with shell `build.sh` release path
- `internal/templates/overlays/doc/files/governa/release.md.tmpl` — update release instructions to reference `./build.sh vX.Y.Z "message"` instead of `./rel.sh`
- `internal/governance/governance_test.go` — add test: DOC apply emits `build.sh`; does not emit `rel.sh` or `cmd/rel/`

### Part B — DOC prep parity

- `internal/templates/overlays/doc/files/build.sh.tmpl` — add `prep` subcommand: CHANGELOG `| Unreleased |` row sealing, AC-file deletion matched by commit message AC-refs, `plan.md` IE sweep, pre/post `./build.sh` validation gate
- `internal/templates/overlays/doc/files/governa/release.md.tmpl` — update pre-release checklist to use `./build.sh prep vX.Y.Z "message"` (mirrors CODE `build.sh prep` from AC147)
- `internal/governance/governance_test.go` — add test: DOC apply emits build.sh containing `prep` subcommand

## Out of Scope

- Stack seam and Terraform stack (AC148)
- CODE prep changes (complete in AC147)
- Migrating existing DOC consumer repos

## Acceptance Tests

### Part A

- [Automated] [Pre-release gate] `governa apply --type doc` on a scratch target emits `build.sh`; does not emit `rel.sh`, `cmd/rel/main.go`, or `cmd/rel/color.go`.
- [Manual] [Pre-release gate] Emitted DOC `build.sh validate` exits 0 on a minimal repo with valid markdown; exits non-zero when `mdcheck` finds a lint error.
- [Manual] [Pre-release gate] Emitted DOC `build.sh vX.Y.Z "message"` produces a signed git tag `vX.Y.Z` with the given message; exits non-zero when the working tree is dirty.
- [Automated] [Pre-release gate] `governa/overlay-scope.md` no longer references `rel.sh`, `cmd/rel/main.go`, or `governa-reltool` in its DOC section.

### Part B

- [Manual] [Pre-release gate] Emitted DOC `build.sh prep vX.Y.Z "message"` seals the `| Unreleased |` CHANGELOG row, deletes AC files referenced in the commit message, sweeps `plan.md` IE entries, and runs pre/post validation.
- [Automated] [Pre-release gate] `governa/release.md.tmpl` (DOC flavor) references `./build.sh prep` in its pre-release checklist.

## Status

`PENDING` — scoped; awaiting director critique gate before implementation authorization.
