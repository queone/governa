# Forge Verify Closure Audit

Use this template once, after Forge implementation, validation, adversarial verification, and defect correction, and before reporting Forge complete. Keep the audit non-mutating. Record the exact commands or checks used and preserve the completed artifact with the Forge completion report.

## Audit Identity

- Active AC: `<AC number and path>`
- Audit date: `<YYYY-MM-DD>`
- Repository: `<repository path or identifier>`
- Forge completion report: `<path or link>`
- Audit rerun: `<initial audit or correction rerun>`

## Non-Mutating Scan Procedure

Run and record the exact read-only checks used for this audit.

```bash
git status --short
rg --files
rg -n '<user-facing entry-point patterns>' <in-scope paths>
rg -n '<provider/API fetch patterns>' <in-scope paths>
rg -n '<normalized-table write and durable-snapshot patterns>' <in-scope paths>
rg -n '<stale-fallback, freshness-gate, and reconciliation patterns>' <in-scope paths>
rg -n '^## (In Scope|Out Of Scope|Acceptance Tests)$' <active AC path>
```

Add read-only file inspection, test-result inspection, or repository-specific search commands as needed. Do not edit files, generate artifacts in the repository, call external providers, or rerun mutating commands during the audit.

## Command/Data-Path Matrix

Record every discovered path in active AC scope. Record `Not applicable` and repository evidence for an absent category; do not use `Not applicable` for an unsearched or unmapped path.

| Category | Discovered path | Active-AC mapping | Evidence command or test | Disposition | Finding |
| --- | --- | --- | --- | --- | --- |
| Command entry point | `<path or Not applicable>` | `<In Scope / Out Of Scope / Acceptance Test / none>` | `<command>` | `<Verified / Not applicable / Unmapped / Unverified>` | `<finding or none>` |
| Provider/API fetch | `<path or Not applicable>` | `<mapping>` | `<command>` | `<disposition>` | `<finding>` |
| Normalized-table write | `<path or Not applicable>` | `<mapping>` | `<command>` | `<disposition>` | `<finding>` |
| Durable snapshot | `<path or Not applicable>` | `<mapping>` | `<command>` | `<disposition>` | `<finding>` |
| Stale fallback | `<path or Not applicable>` | `<mapping>` | `<command>` | `<disposition>` | `<finding>` |
| Freshness gate | `<path or Not applicable>` | `<mapping>` | `<command>` | `<disposition>` | `<finding>` |
| Complete-snapshot reconciliation | `<path or Not applicable>` | `<mapping>` | `<command>` | `<disposition>` | `<finding>` |

## Acceptance-Test Disposition

Record every acceptance test from the active AC.

| Test | Source axis | Timing axis | Execution or reasoning evidence | Result | Gap |
| --- | --- | --- | --- | --- | --- |
| `<AT identifier>` | `<Automated / Manual>` | `<Pre-release gate / Post-release verification>` | `<command, report, or reason>` | `<Passed / Accepted / Pending Director review / Blocked>` | `<gap, pending review, or none>` |

## Scope Check

- Compare every matrix path with the active AC `## In Scope` entries: `<result>`
- Compare every matrix path with the active AC `## Out Of Scope` entries: `<result>`
- Compare every matrix path with the active AC `## Acceptance Tests` entries: `<result>`
- Record omitted, conflicting, or newly discovered scope: `<result>`

## Residual Risks

List accepted non-implementation risks separately from implementation findings.

| Risk | Owner | Impact | Required follow-up | Accepted by |
| --- | --- | --- | --- | --- |
| `<risk or None>` | `<owner>` | `<impact>` | `<follow-up>` | `<decision>` |

## Closure Decision

- Unresolved implementation findings: `<zero required>`
- Unmapped required paths: `<zero required>`
- Unverified required paths: `<zero required>`
- Pending manual acceptance reviews: `<AT identifiers or None>`
- Closure result: `<Forge complete / Return to Forge / Return to Shape>`

Report Forge complete only when all required implementation counts are zero, pending manual reviews are explicitly recorded, the artifact path is linked in the Forge completion report, and residual risks are listed separately.
