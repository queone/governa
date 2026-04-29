# AC100 preptool split attempt

Skeleton stub. Attempts to split `internal/preptool` into a `governa-preptool` library core plus a template-side adapter under the policy in [`docs/library-policy.md`](library-policy.md). The convention-coupling test is the load-bearing primitive: preptool today encodes governance conventions (AC file shape, CHANGELOG row format, AGENTS.md governed sections, programVersion bump semantics, the AC critique protocol). The split is real only if the library core can be expressed in convention-free terms; otherwise preptool stays template. The library is also the canonical-fix venue for the `programVersion` advisory ([`docs/advisories/program-version-bump.md`](advisories/program-version-bump.md)) — designed with per-utility-vs-repo-tracked semantics from scratch (mode contract: auto-detect / explicit config / hybrid — open question for the AC). First-Consumer Self-Test runs in `cmd/prep/main.go` (already imports `internal/preptool`).

## Summary

Stub awaiting full draft. When activated, the AC will be drafted in full per `docs/ac-template.md`, critiqued, and authorized before implementation. The split attempt itself is the diagnostic — a failed split is signal that preptool stays template, not failure to ship. See AC96 for policy context and the convention-coupling test.

## Status

`PENDING` — stub registered as IE10 in `plan.md`. Awaiting Director scope-up and full draft.
