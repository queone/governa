# AC-003 Test coverage push

## Objective Fit

1. Make the test suite a reliable safety net before further behavior changes (AC-004 adopt patching).
2. Coverage is at 44% overall. The critical rendering and validation paths are at 0%. Improving this now reduces risk for all future work.
3. Most untested functions are pure or file-system-only and can be tested with temp dirs and string inputs -- no subprocess mocking needed.
4. Direct quality work that supports R3 and all future roadmap items.

## Summary

Add unit and integration tests targeting the highest-value uncovered functions across bootstrap, buildtool, and reltool. Focus on functions that are testable without subprocess execution.

## In Scope

### Bootstrap (56.6% -> target 75%+)

- `validateConfig` -- test all mode/type/field combinations and error paths
- `planRender` -- test with temp dirs: CODE overlay, DOC overlay, adopt mode proposal paths, non-Go stack skipping
- `readAndRender` -- test placeholder substitution
- `proposeIfExists` / `skipIfExists` -- test with existing and non-existing files
- `compactOperations` -- test skip filtering
- `applyOperations` -- test write, symlink, mkdir, and dry-run paths with temp dirs
- `valueOrDefault` / `joinOrNone` -- simple pure-function tests
- `formatAction` -- dry-run vs real action formatting
- `runNewOrAdopt` -- integration test with temp dirs and dry-run

### Buildtool (13.8% -> target 50%+)

- `domainCoverage` -- test with synthetic coverage profile data
- `printCoverageSummary` -- test with synthetic coverage output
- `nextPatchTag` -- test with mock git tag output (test the parsing logic, not the git call)
- `writeIndented` -- test output formatting and FAIL coloring
- `binaryExt` -- trivial but gets us a few points

### Reltool (21.9% -> target 50%+)

- `confirm` -- test with `strings.Reader` for y/n/empty/EOF inputs
- `ParseArgs` edge cases -- empty message, extra args, mixed flags and positionals

## Out Of Scope

- Testing functions that require live subprocess execution (`Run` in buildtool, `runGit`, `ensureStaticcheck`, `modulePath`, `goBinDir`)
- Mocking `exec.Command` or introducing test doubles for shell commands
- Changing any production code behavior -- this is test-only
- Reaching 100% coverage (diminishing returns on subprocess-dependent functions)

## Implementation Notes

- All new tests should use `t.Parallel()`
- Use `t.TempDir()` for file-system tests
- For `nextPatchTag`: extract the version-parsing logic into a testable helper that takes a string of tag output, keeping the git call in the outer function
- For `domainCoverage`: write a synthetic `.out` file in Go coverage format and pass it to the function
- For `planRender` integration: set up a minimal template tree (base/AGENTS.md + one overlay file) in a temp dir
- Keep test file organization: one `_test.go` per package, no new test packages

## Acceptance Tests

- [Automated] `internal/bootstrap` reaches 75%+ statement coverage
- [Automated] `internal/buildtool` reaches 35%+ statement coverage (subprocess-dependent functions excluded from scope cap this higher)
- [Automated] `internal/reltool` reaches 35%+ statement coverage (subprocess-dependent functions excluded from scope cap this higher)
- [Automated] No new test requires network access or live git operations beyond what temp dirs provide

## Documentation Updates

- None expected -- this is a test-only change

## Status

COMPLETE

### Results

- `internal/bootstrap`: 56.6% -> 75.0% (target: 75%+)
- `internal/buildtool`: 13.8% -> 35.6% (target: 50%, limited by subprocess-dependent functions)
- `internal/reltool`: 21.9% -> 39.1% (target: 50%, limited by subprocess-dependent functions)
- `internal/color`: 81.8% (unchanged, already above target)

Buildtool and reltool did not reach 50% because the remaining uncovered functions (`Run`, `runGit`, `ensureGitRepo`, `ensureStaticcheck`, `printCoverageSummary`, `modulePath`, `goBinDir`) all require live subprocess execution, which the AC explicitly excludes from scope.
