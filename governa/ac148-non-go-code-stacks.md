# AC148 First-Class Non-Go CODE Stacks (Terraform First)

## Summary

Rudimentary stub — requires further scoping before critique gate or implementation authorization.

governa's CODE flavor hard-bakes Go build/release tooling. Today `--stack` only (a) appends a `.gitignore` block (`stack-ignores/go.txt`), (b) substitutes the `{{STACK_OR_PLATFORM}}` string, and (c) for non-Go stacks skips exactly two files (`cmd/build/main.go.tmpl`, `cmd/rel/main.go.tmpl`) while still emitting `build.sh`, `cmd/prep`, and `internal/preptool` — an incoherent half-state. This AC introduces real per-stack canon so CODE repos in other languages (Terraform first) get language-appropriate, dependency-free validation and release tooling. It depends on AC147 (shell build/release/prep tooling) landing first; the Go-vs-shell tooling reconsideration is owned by AC147. Infrastructure/templating change.

Note: AC147 converts the CODE Go tooling to shell. References below to `cmd/build`/`cmd/rel`/`cmd/prep` describe the pre-AC147 state and must be re-read against AC147's shell `build.sh` at scoping time.

## In Scope

High-level direction only; the concrete file list is deferred to the scoping pass.

### Stack seam
- Replace the monolithic `internal/templates/overlays/code/files` tree with real per-stack canon selection (e.g. `overlays/code/stacks/<stack>/`), so the build/release tooling is chosen by `--stack`.
- Fix the incoherent non-Go branch at `internal/governance/governance.go:638` (it drops the Go-tooling files but keeps a Go-assuming `build.sh`, prep, and preptool — re-evaluate against AC147's shell tooling).

### Terraform stack
- Dependency-free validation gate (no `go.mod` required): `terraform fmt -check -recursive`, per-module `terraform init -backend=false && terraform validate`, `tflint` when present; optional native `terraform test`.
- `inferStack` manifest detection for `*.tf` / `.terraform.lock.hcl`; add `stack-ignores/terraform.txt` (`.terraform/`, `*.tfstate*`, `*.tfvars`).

### Repo-owned validation gates
- A non-Go consumer that adopts DOC today and adds its own validation script (e.g. a Terraform `build.sh`) gets it classified `target-has-no-canon` by drift-scan — a recurring keep/delete/migrate routing decision that no preserve marker can lock (the `target-has-no-canon` branch in `internal/driftscan/driftscan.go` never consults `PreserveMarkers`). Give first-class stacks a canonical gate, or let a repo register an expected-divergence/locked path for a target-only gate file. Reference adopter: `terraform-azure` AC2.

### DOC shell release (drop the Go release tool)
- DOC canon ships `rel.sh` → Go `cmd/rel/{main.go,color.go}`, forcing a non-code content repo to keep Go installed just to git-tag a release. Offer a pure-shell single-entry `build.sh` (validate + `vX.Y.Z "message"` release) as the DOC release path and make `cmd/rel` + `rel.sh` optional/non-canon. Until then, a consumer that removes them takes un-lockable `missing-in-target` drift-scan noise every cycle (deletion is classified before `PreserveMarkers`, `internal/driftscan/driftscan.go:665`). Reference adopter: `terraform-azure` AC2, which removed both in favor of a shell `build.sh`. Reuse the shell color/usage/release helper functions AC147 adds to `build.sh`.
- Correct `governa/overlay-scope.md` line ~36, which describes DOC release tooling as "`rel.sh`, `cmd/rel/main.go`, importing `github.com/queone/governa-reltool`". This is already inaccurate — DOC's `cmd/rel` is stdlib-only and does not import `governa-reltool` — and AC147 deliberately left it for this AC. Update it to the DOC shell-release form. (Deferred here from AC147 Part E.)

### DOC release-prep tool (parity with CODE prep)
- CODE flavor gets deterministic, reproducible release bookkeeping (CHANGELOG row insertion, AC-file deletion by message AC-refs, `plan.md` IE sweep, pre/post validation). DOC flavor ships **no** prep tool, so its `governa/release.md` checklist is hand-performed — non-reproducible and a fresh source of harness confirmation prompts for routine edits. Ship a DOC prep step (shell, e.g. a `build.sh prep` subcommand mirroring AC147's CODE shell prep) so non-code repos get the same determinism. Reference adopter: `terraform-azure` AC2, which added a shell `build.sh prep`.

## Out Of Scope

TBD — requires scoping before critique gate. Provisional exclusions:

- Shell conversion of the CODE Go tooling — owned by AC147.
- Per-module / independent submodule semver (separate future IE/AC); the default remains one repo-level version.
- Implementing every language stack in one pass.
- Migrating existing adopted Go repos.

## Acceptance Tests

TBD — requires scoping before critique gate.

## Status

`PENDING` — rudimentary stub; depends on AC147; awaiting a scoping pass and director critique gate before implementation authorization.
