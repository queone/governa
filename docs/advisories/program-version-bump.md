# Advisory: programVersion Bump Regex + Semantics

**Status: RESOLVED-AGAIN (AC110).** governa AC100 landed the in-place dual-form regex + safe auto-detect fix. governa AC110 extended the auto-detect with a primary-cmd convention to fix governa's own bump gap (AC100's "exactly 1 → bump, >1 → skip-all" rule treated `cmd/governa/main.go` + `cmd/driftscan/main.go` as multi-utility and froze governa's `programVersion` at `0.101.0` while the repo shipped through `0.106.0`). Under AC110, `cmd/<go-mod-basename>/main.go` is the primary and bumps with the repo; other `cmd/*/main.go` are secondaries with independent versioning. Utils-style multi-utility repos (no `cmd/utils/main.go`) keep the existing skip-all behavior unchanged.

**Status (historical): RESOLVED (AC100).** governa AC100 landed the in-place dual-form regex + safe auto-detect fix in `internal/preptool/`. preptool stays template (the convention-coupling test rejected library extraction); consumer repos that want the fix pull it manually by reading the updated source on their own AC. Origin: surfaced in the `utils` consumer repo during AC25 release prep on 2026-04-28. Tracked in `utils` as IE9 / AC26.

## Two valid mitigation patterns

Both patterns are recorded for future readers facing this issue. Pick whichever fits.

- **In-place patch** (the AC100 shape): single regex matches both inline `const programVersion = "..."` and grouped `const ( ... programVersion = "..." ... )` forms. Auto-detect skips bumps when more than one `programVersion` target is present (multi-utility repo → per-utility-independent default). For governa's source patch, see git history at AC100 commit; for application in a consumer repo, copy the regex change + the auto-detect filter from `internal/preptool/preptool.go` onto the consumer's local copy.
- **Doctrine + guard** (the utils AC26 shape): codify per-utility independence in the consumer's release docs (e.g. `docs/build-release.md`); add a guard test that asserts `cmd/*/main.go` bytes do not change after a prep dry-run with a sentinel target version. No preptool code change. Functionally equivalent for multi-utility consumers — the buggy single-regex silently skips the grouped form, which is correct-by-accident for utils-style repos. The guard test prevents anyone from "fixing" the regex naively and triggering a mass downgrade.

Consumers facing this issue pick whichever pattern fits — neither requires steward coordination. governa does not track adoption.

## Symptom

`internal/preptool/preptool.go` declares:

```go
programVersionRe = regexp.MustCompile(`(const\s+programVersion\s*=\s*)"([^"]+)"`)
```

This matches only the inline form `const programVersion = "x.y.z"`. Consumer code that uses the grouped form

```go
const (
    programName    = "tool"
    programVersion = "1.2.3"
)
```

is silently skipped: `detectVersionTargets` returns zero `programVersion` targets, `applyVersionBump` is never called for those files, and prep produces no error. The release ships without bumping per-utility versions.

## Trap

A naive fix that broadens the regex to match the grouped form will cause every per-utility `programVersion` to be overwritten with the repo release version on the next prep run. For multi-utility consumers whose utilities have diverged into independent SemVers (e.g. `brew-update 1.3.5`, `dl 2.0.0`, `pgen 1.2.3`), this triggers mass downgrades.

Do not land a regex-only fix without first deciding bump semantics.

## Root Cause (Two Layers)

1. **Regex layer:** `programVersionRe` does not match the grouped `const ( ... )` form, even though that form is idiomatic Go and widely used in consumer repos.
2. **Semantics layer:** preptool implicitly assumes per-utility `programVersion` should track the repo release version. That assumption is wrong for multi-utility repos where each utility evolves on its own SemVer line.

## Resolution (governa AC100)

The full library extraction (`governa-preptool`) was attempted under AC100 and rejected per the convention-coupling test in `docs/library-policy.md`: preptool encodes too much governance (AC file shape, CHANGELOG row format, `internal/templates/base/` detection, critique-companion shape) for a clean library API to exist. preptool stays template; the IE9 fix landed in-place in `internal/preptool/preptool.go`:

- **Single regex** matching both inline and grouped const forms: `(programVersion\s*(?:string\s*)?=\s*)"([^"]*)"`. The `const` keyword is intentionally not required by the regex — the grouped form has it on a different line.
- **Safe auto-detect filter** in `detectVersionTargets`: count `programVersion` matches across `cmd/*/main.go`. Exactly 1 → bump (single-utility, repo-tracked). >1 → drop all targets, log a multi-utility warning (per-utility-independent, each utility owns its own version per its own AC). The skip avoids the clobber risk that broadening the regex without mode handling would have introduced.

Auto-detect was chosen over explicit-config override because it produces correct behavior for both known consumer shapes (single-utility repos including governa = bump; multi-utility repos like `utils` = skip) without configuration. Explicit override may be added later as a minor preptool change if a consumer surfaces a need.
