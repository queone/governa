# AC101 governa shrinkage

Skeleton stub. Once the upstream extraction ACs (AC97 color, AC98 reltool, AC99 buildtool, AC100 preptool split attempt) settle, governa's repo trims to `internal/templates/` + `docs/` + a thin `cmd/governa` that imports the libs as their first consumer. Codifies the convention-archive role established by AC96. Removes extracted `internal/<x>` directories and their tests; updates `cmd/<x>/main.go` imports if not already migrated by the per-extraction ACs.

## Summary

Stub awaiting full draft. Lands after the extraction ACs above have completed (or definitively stayed template, in the case of AC100). When activated, the AC will be drafted in full per `docs/ac-template.md`, critiqued, and authorized before implementation. See AC96 for policy context.

## Status

`PENDING` — stub registered as IE11 in `plan.md`. Awaiting Director scope-up and full draft. Sequencing dependency: do not activate until prior extraction ACs have shipped or been definitively deferred.
