QA has reviewed this entire project, and here are its findings:

---

# QA Review: repo-governance-template

**Reviewer**: Claude Opus 4.6 (QA pass)
**Date**: 2026-04-06
**Scope**: Full codebase review -- all Go code, templates, rendered examples, docs, scripts, and governance artifacts
**Revision 3**: Final consolidated review. All parties (DEV, QA, owner) aligned on findings and fix order.

---

## Confirmed Bugs

### 1. `stackSuggestsGo` has a false-positive substring match

**Location**: `stackSuggestsGo` in `internal/bootstrap/bootstrap.go`

```go
func stackSuggestsGo(stack string) bool {
    value := strings.ToLower(strings.TrimSpace(stack))
    return strings.Contains(value, "go")
}
```

This matches any stack string containing "go" as a substring. Stacks like `"Django service"`, `"Google Cloud Functions"`, `"Cargo workspace"`, or `"Hugo site"` would incorrectly trigger the Go-specific code path, causing Go-only template files to be rendered into non-Go repos. A word-boundary check or explicit keyword list is needed.

**Propagation**: This affects the bootstrap implementation directly. Generated repos receive incorrect files. The bug does not propagate into overlay templates or examples themselves, but its output does -- any repo bootstrapped with a false-positive stack string will contain broken Go artifacts.

### 2. `color.go.tmpl` files are not skipped for non-Go stacks

**Location**: `planRender` in `internal/bootstrap/bootstrap.go`

The skip logic checks for `cmd/rel/main.go.tmpl` and `cmd/build/main.go.tmpl` when the stack does not suggest Go, but does **not** check for `cmd/build/color.go.tmpl` or `cmd/rel/color.go.tmpl`. A non-Go CODE repo (e.g. Rust) will get orphaned `cmd/build/color.go` and `cmd/rel/color.go` files with no corresponding `main.go`.

**Propagation**: Same as #1 -- the bug is in the bootstrap implementation, but its effect lands in the generated repo. The overlay templates themselves are correct; the rendering logic fails to skip them.

### 3. Release tool does `git add .` unconditionally

**Location**: `Run` in `internal/reltool/reltool.go` (the `steps` slice, first entry)

The release flow hardcodes `git add .` as its first step. This stages everything in the working tree, including unrelated WIP files, `.env` files, credentials, or anything not in `.gitignore`. The confirmation prompt only asks "Have you validated the build?" -- it does not show what files are about to be staged.

**Propagation**:

Source-of-truth locations (fix here first):
- `internal/reltool/reltool.go` (the template repo's own release tool)
- `overlays/code/files/cmd/rel/main.go.tmpl` (CODE overlay template)
- `overlays/doc/files/cmd/rel/main.go.tmpl` (DOC overlay template)

Rendered copies (regenerate after fixing sources):
- `examples/code/cmd/rel/main.go`
- `examples/doc/cmd/rel/main.go`

### 4. `build.sh` pattern match is fragile for single-arg semver

**Location**: `build.sh` at the repo root

```bash
if [[ $# -ge 2 && "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  exec go run ./cmd/rel "$@"
fi
```

If a user runs `./build.sh v1.0.0` (one arg, no message), it falls through to the build command instead of the release command. The build command will treat `v1.0.0` as a target name and fail with a confusing error. The shell wrapper should match on the first arg pattern alone and let `cmd/rel` own argument validation.

**Propagation**:

Source-of-truth locations (fix here first):
- `build.sh` at the repo root
- `overlays/code/files/build.sh.tmpl` (CODE overlay template)

Rendered copies (regenerate after fixing sources):
- `examples/code/build.sh`

### 5. Duplicated canonical governance content with no enforcement mechanism

**Location**: `AGENTS.md` (repo root) and `base/AGENTS.md`

The root `AGENTS.md` and `base/AGENTS.md` are byte-identical regular files that serve different roles -- the root copy exists for self-hosting this repo as a governed CODE repo, while `base/AGENTS.md` exists as the template source rendered into generated repos. There is no mechanism (symlink, generation step, CI check, or documented convention) to keep them in sync. If someone edits one, the other will silently drift.

### 6. Overlay READMEs are stale -- missing `color.go.tmpl` entries

**Location**: `overlays/code/README.md` and `overlays/doc/README.md`

`overlays/code/README.md` lists the current templates but omits `cmd/build/color.go.tmpl` and `cmd/rel/color.go.tmpl`. Similarly, `overlays/doc/README.md` omits `cmd/rel/color.go.tmpl`. These files exist and are rendered into generated repos but are not documented in the overlay's own inventory.

---

## Unresolved Behavior-Policy Issues

These are not necessarily defects, but the current behavior does not match what a user would reasonably expect from a "build" or "validation" command. They should be resolved by either changing the behavior or documenting the policy explicitly.

### 7. `runCapturedSoft` treats `go vet` / `staticcheck` failures as informational

**Location**: `runCapturedSoft` in `internal/buildtool/buildtool.go`

When `go vet` or `staticcheck` fail with a non-zero exit code but produce output, the build continues -- output is printed but the error is not propagated. Users running the canonical build command will see static analysis warnings scroll past but may not realize the build "passed" despite them.

This behavior was inherited from the original `skout/build.sh` where these steps were intentionally informational. Either preserving the inherited workflow or adopting strict validation is defensible, but the current state is undocumented. A user who runs the canonical build command and sees it exit successfully has no way to know whether static analysis actually passed. Current behavior may violate user expectations unless explicitly documented.

**Propagation**:

Source-of-truth locations:
- `runCapturedSoft` in `internal/buildtool/buildtool.go`
- `overlays/code/files/cmd/build/main.go.tmpl` (CODE overlay template)

Rendered copies:
- `examples/code/cmd/build/main.go`

### 8. Targeted build scopes skip `internal/` package validation

**Location**: `packageScopes` in `internal/buildtool/buildtool.go`

When a user scopes the build to selected commands (e.g. `build bootstrap rel`), `packageScopes` returns only `["./cmd/bootstrap", "./cmd/rel"]`, so `go vet`, `go test`, `go fmt`, and `staticcheck` run only against those cmd packages. The `internal/` packages -- where the domain logic lives -- are skipped.

This matches the original shell behavior and is arguably reasonable for targeted runs. But it is not documented. A user who runs `build bootstrap` expecting it to validate the full dependency chain of the bootstrap command will be surprised when `internal/bootstrap` is not checked. Current behavior may violate user expectations unless explicitly documented.

**Propagation**:

Source-of-truth locations:
- `packageScopes` in `internal/buildtool/buildtool.go`
- `overlays/code/files/cmd/build/main.go.tmpl` (CODE overlay template)

Rendered copies:
- `examples/code/cmd/build/main.go`

---

## Quality Gaps

### 9. Test coverage is low for critical-path functions

| Package | Coverage |
|---------|----------|
| `internal/bootstrap` | 44.1% |
| `internal/buildtool` | 14.3% |
| `internal/reltool` | 23.0% |
| `internal/color` | 81.8% |
| **Overall domain** | **35.3%** |

Key untested functions: `ParseArgs` in bootstrap (0%), `Run` in buildtool (0%), `Run` in reltool (0%), `validateConfig` (0%), `planRender` (0%), `applyOperations` (0%), `runNewOrAdopt` (0%).

The functions that are tested are mostly leaf helpers. A test calling `Run` with a temp directory and dry-run mode would cover most of the rendering pipeline without side effects. This is a quality gap, not evidence that the implementation is currently broken.

### 10. Bootstrap output is hard-wired to stdout

**Location**: `printAssessment`, `printEnhancementReport`, `runEnhance`, and `applyOperations` in `internal/bootstrap/bootstrap.go`

These functions use `fmt.Printf` directly instead of accepting `io.Writer` parameters. This makes them untestable and inconsistent with the build/release tools which accept injected writers. Technical debt, not urgent.

### 11. `docs/bootstrap-model.md` references `scripts/bootstrap` as an entrypoint

**Location**: `docs/bootstrap-model.md` (lines referencing `scripts/bootstrap` as "conceptual entrypoint" and listing it under "Why This Fits The Template Repo")

`scripts/` only contains a `README.md` placeholder. The actual entrypoint is `cmd/bootstrap/main.go`. The doc should be updated to reflect reality.

### 12. No `.gitignore` in the repository

The repo has no `.gitignore`. Generated repos also don't get one from the bootstrap. Common artifacts like coverage files, editor swap files, `.DS_Store`, etc. can easily get committed. The absence has not caused problems yet, but adding one is sensible for a reusable template repo and for generated repos.

---

## Minor Notes

### 13. `proposalPath` produces surprising names for extensionless files

**Location**: `proposalPath` in `internal/bootstrap/bootstrap.go`

For `TEMPLATE_VERSION` (no extension), the result is `TEMPLATE_VERSION.template-proposed`. For `CLAUDE.md`, the result is `CLAUDE.template-proposed.md`. The inconsistent suffix position (before vs after extension) may confuse users.

### 14. Duplicate validation logic between `new` and `adopt` modes

**Location**: `validateConfig` in `internal/bootstrap/bootstrap.go`

Nearly identical validation blocks for `ModeNew` and `ModeAdopt`. The only difference is that `adopt` allows empty `cfg.Type`. Could be a single code path with a conditional.

### 15. Enhancement mapping has a redundant `scripts/bootstrap` entry

**Location**: Enhancement `mappings` slice in `ReviewEnhancement` in `internal/bootstrap/bootstrap.go`

`scripts/bootstrap` appears in two mappings. If the reference repo has this file, it matches the first mapping and the second redundantly re-reads the same file.

### 16. `color` package `enabled` is determined at init time and is not overridable

**Location**: `enabled` var in `internal/color/color.go`

Set once via a package-level init closure. No way to force color on or off for testing, nor to respect per-writer TTY detection. Color decisions are based on `os.Stdout`, regardless of which writer is actually being used.

---

## Items Reviewed And Determined Not To Be Issues

- **CHANGELOG table format**: User-directed and intentionally chosen. Not a defect.
- **`templateRoot()` using `runtime.Caller`**: Acceptable under the current design constraint that `bootstrap` is a `go run` entrypoint, not a distributed binary.
- **Example repos compile during `go test ./...`**: This is a feature. Verifying that generated Go examples remain compilable is useful signal.
- **`-v` flag means different things across commands**: Normal CLI design. Separate commands have separate flag surfaces.

---

## Positive Observations

- Clean separation between base governance, overlays, and tooling
- Conservative adoption mode with proposal files instead of overwrites
- Enhancement mode is report-first by design -- good safety posture
- Zero external dependencies (pure stdlib Go)
- Symlinks for CLAUDE.md -> AGENTS.md are correctly implemented
- Good use of `t.Parallel()` in all tests
- Template placeholder rendering is deterministic and sorted
- The `dry-run` mode is consistently supported across all bootstrap operations
- The architecture doc (`arch.md`) accurately reflects the codebase
- Rendered examples compile and are maintained as current rendered outputs of the template

---

## Recommended Fix Order

1. Fix `stackSuggestsGo` false positives (#1)
2. Skip `color.go.tmpl` for non-Go stacks (#2)
3. Tighten release staging safety around `git add .` across `reltool.go` and both overlay templates (#3)
4. Fix the `build.sh` single-tag routing edge case across root and CODE overlay template (#4)
5. Establish an enforcement mechanism for root `AGENTS.md` vs `base/AGENTS.md` sync (#5)
6. Update overlay README inventories (#6)
7. Resolve `runCapturedSoft` policy: either fail hard on static analysis errors or document the informational-only behavior (#7)
8. Document targeted-build scoping behavior (#8)
9. Clean up `scripts/bootstrap` documentation drift (#11)
10. Add `.gitignore` for both the template repo and generated repos (#12)

---

## Appendix: Triage History

This review went through three rounds. Supporting artifacts: `review-response.md` (DEV triage), `review-critique.md` (QA critique of revisions).

Key reclassifications across rounds:
- `runCapturedSoft` and targeted build scopes: initially classified as design decisions, reopened as unresolved behavior-policy issues
- Go 1.25.0 and `strings.SplitSeq` findings: retracted (Go 1.26.1 is current stable)
- CHANGELOG format, `runtime.Caller`, example compilation, `-v` flag: reviewed and determined not to be issues
- `docs/skout-build-migration.md`: dropped (file removed from repo)
