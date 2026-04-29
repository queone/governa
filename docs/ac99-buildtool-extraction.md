# AC99 buildtool extraction

Skeleton stub. Extracts `internal/buildtool` to `github.com/queone/governa-buildtool` under the policy in [`docs/library-policy.md`](library-policy.md). Light convention coupling expected (validates `programVersion` non-empty, etc.); convention-coupling test at draft time decides whether the library can be expressed in convention-free terms with a small config surface, or whether it stays template. First-Consumer Self-Test runs in `cmd/build/main.go` (already imports `internal/buildtool`).

## Summary

Stub awaiting full draft. When activated, the AC will be drafted in full per `docs/ac-template.md`, critiqued, and authorized before implementation. See AC96 for policy context.

## Status

`PENDING` — stub registered as IE9 in `plan.md`. Awaiting Director scope-up and full draft.
