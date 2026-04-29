# Advisory: programVersion Bump Regex + Semantics

**Status:** Archived in governa as the canonical record. Origin: surfaced in the `utils` consumer repo during AC25 release prep on 2026-04-28. Tracked in `utils` as IE9 / AC26. This advisory is design input for governa AC96 (the library policy) and the downstream preptool-extraction AC that will ship `governa-preptool` under that policy. AC96 itself does not ship the library; the canonical fix venue is the library, designed under AC96 and landed via the extraction AC. This advisory is also forwardable to other consumer repos that applied governa before the structural pivot away from sync.

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

## Canonical Fix (governa-side)

The `governa-preptool` library, when designed under AC96, should distinguish two modes and let consumers select per-repo:

- **Repo-tracked mode** — `programVersion` is bumped to the repo release version at prep time. Suitable for single-utility repos where `programVersion == repo tag` is the convention.
- **Per-utility-independent mode** — `programVersion` is owned by each utility's own AC; prep does not touch `cmd/*/main.go`. Suitable for multi-utility repos.

Mode selection options (open question, to be decided in the preptool-extraction AC):

- Auto-detect: one `cmd/*/main.go` → repo-tracked; multiple → per-utility-independent.
- Explicit config: a flag in a repo-level governa/preptool config file.
- Hybrid: auto-detect with explicit override.

Whichever mode is active, the regex must support both inline and grouped const forms — `internal/buildtool/buildtool.go` already demonstrates the dual-regex pattern (`programVersionInlineRe` + `programVersionBlockRe`) and is a clean reference.

## Per-Consumer Guidance

- **Single-utility repos:** likely affected only if the utility uses the grouped const form. If on inline form, the bug never triggers. Verify by grepping `cmd/*/main.go` for the grouped form. Recommended posture: repo-tracked mode (status quo intent), pick up the dual-regex fix when `governa-preptool` ships.
- **Multi-utility repos:** affected. Current silent-skip behavior is correct-by-accident; do not "fix" the regex locally without first removing the bump path or selecting per-utility-independent mode. Recommended posture: codify per-utility-independent doctrine in repo's release docs, add a guard test that pins the silent-skip behavior, then either (a) wait for `governa-preptool` and adopt per-utility-independent at migration time, or (b) remove the `cmd/*/main.go` scan from local preptool now (note: any local code change is throwaway when migrating to the library).

## Recommended Steward Actions

1. Fold this advisory into the design work for `governa-preptool` extraction. The library is the canonical-fix venue; the dual-regex + mode-contract decision is a design input there, not a patch to governa's current monolith preptool.
2. Decide and document mode selection in `governa-preptool` (auto-detect via `cmd/*/main.go` count vs. explicit per-repo config vs. hybrid).
3. Once `governa-preptool` ships, each former-governa consumer drafts its own AC to migrate from the locally-owned preptool copy to the imported library, picking the mode that matches its shape (single-utility → repo-tracked; multi-utility → per-utility-independent).
4. Maintain a steward-side ledger of which consumer repos have migrated and which still carry the broken regex in their local copy.
5. Until the library ships, consumer repos discovering this trap should land doctrine + a guard test only (see `utils` AC26 for the reference shape) and avoid local code changes that will be discarded at migration time.
