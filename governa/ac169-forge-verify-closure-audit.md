# AC169 Forge Verify Closure Audit

## Summary

Strengthen the Forge Verify contract so every implementation pass ends with one exhaustive, non-mutating closure audit before Ratify. The audit must discover all in-scope command paths, provider reads, persistence paths, fallback behavior, freshness decisions, acceptance tests, and residual risks, then block completion when any required path is unmapped or unverified.

## In Scope

### Contract changes

- Update the Forge Verify rules in `AGENTS.md` to require one exhaustive, non-mutating closure audit before reporting Forge complete.
- Define the audit outputs as a command/data-path matrix, acceptance-test disposition, scope check, and explicit residual-risk list.
- Require the audit to inspect all user-facing command entry points, provider/API fetches, normalized-table writes, durable snapshots, stale fallbacks, freshness gates, and complete-snapshot reconciliation paths within the active AC scope.
- Require the audit to compare discovered paths against the active AC `## In Scope`, `## Out Of Scope`, and `## Acceptance Tests` sections.
- Require Forge to return to implementation or Shape before Ratify when the closure audit finds an unmapped required path, missing persistence/fallback behavior, missing test coverage, or an unresolved contract decision.
- Require the Forge completion report to include the closure-audit artifact or path and report zero unresolved implementation findings before Ratify.

### Verification artifact

- Create a reusable Forge Verify closure-audit template or checklist in `governa/`.
- Define a non-mutating repository scan procedure that can be repeated after corrections and records the exact commands or checks used.

## Out Of Scope

- Change the four-phase workflow ordering.
- Permit implementation during Verify or Ratify.
- Replace the Director's Ratify decision.
- Require external-provider access when repository evidence or automated tests can establish the result.
- Change release-preparation behavior.

## Acceptance Tests

**AT1** [Automated] — The Forge Verify contract names the closure audit as a required pre-Ratify gate and requires a non-mutating audit.

**AT2** [Automated] — The closure-audit checklist requires command/data-path mapping, provider-fetch discovery, persistence/fallback verification, freshness/reconciliation verification, AC scope comparison, and acceptance-test disposition.

**AT3** [Automated] — A Forge completion report template or documented example includes the closure-audit artifact path, zero unresolved implementation findings, and explicit residual risks.

**AT4** [Manual] — The Director reviews one Forge completion report produced under the updated contract and confirms that Ratify does not reveal an implementation path omitted by Verify.

## Status

`PENDING` — awaiting Governa-agent Shape and implementation.
