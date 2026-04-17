# AC56 Acknowledged Drift

Introduce a mechanism for consumer repos to declare intentional divergence from the governa template so `governa sync` stops re-flagging stable carve-outs on every run. Code + doc; feature-class; MINOR release.

## Summary

Every `governa sync` in a consumer repo re-evaluates each file against the template. A file with intentional, documented repo-specific drift — e.g., skout's preserved `### Project Rules` subsections in `AGENTS.md` — stays flagged as `adopt` on every future sync, forcing the consumer to re-evaluate the same carve-out each cycle. AC56 adds a manifest-based `acknowledged` section with dual SHA-scoping (consumer file state + template file state) so once a drift is intentionally recorded, sync omits it from the `adopt` list until either side of the SHA binding changes.

## Objective Fit

1. **Which part of the primary objective?** Sync usability — reduce recurring review cost for stable repo-specific carve-outs so consumers focus attention on actual template changes.
2. **Why not advance a higher-priority task instead?** Sync noise is the concrete bottleneck observed in skout AC58 (14 files flagged `adopt`, many with stable carve-outs the consumer keeps re-documenting). Each sync cycle repeats the same evaluation work. AC55 reduces authoring tax; AC56 reduces review tax. Complementary, independent.
3. **What existing decision does it depend on or risk contradicting?** Depends on AC55's `.governa/` metadata consolidation (the `acknowledged` section lives in `.governa/manifest`). Extends rather than contradicts the existing sync-review `adopted / kept / needs director judgment` vocabulary — `acknowledged` is a fourth disposition meaning "kept, and don't re-ask".
4. **Intentional pivot?** No — extension of the sync-quality trajectory established by AC44–AC55.

## In Scope

### Manifest `acknowledged` section (per-file, dual-SHA-scoped)

Extend `.governa/manifest` with a structured section listing acknowledged drift items. Each entry binds to a specific consumer-file state AND a specific template-file state. If either side changes, the acknowledgment is invalidated and the file returns to the `adopt` flow.

**Entry schema (v1, per-file granularity):**

```
acknowledged:
  - path: <repo-relative path>
    consumer-sha: <sha256 of consumer file at acknowledgment time>
    template-sha: <sha256 of template file at acknowledgment time>
    template-version: <template version at acknowledgment time>
    reason: <single-line justification>
```

`consumer-sha`, `template-sha`, `template-version`, and `reason` are required. `path` keys the entry. Date of acknowledgment is derivable from git history on `.governa/manifest` — not stored in the entry.

The dual-SHA scoping is the load-bearing design choice:
- Consumer's file changes → re-flag (consumer made a new change; re-evaluate).
- Template's version of the file changes → re-flag (upstream improvements deserve reconsideration).
- Both unchanged → acknowledgment remains valid; file omitted from sync's adopt list.

### `governa ack` CLI command

Add an explicit command for recording acknowledgments. The director invokes it after deciding a flagged `adopt` is actually a stable, intentional carve-out.

**Usage:**

```
governa ack <path> --reason "<single-line justification>"
```

Behavior:

- Reads the current consumer file SHA and the current template file SHA.
- Appends or updates the entry in `.governa/manifest` acknowledged section.
- Refuses to run without `--reason`.
- Refuses to acknowledge a file not currently in the `adopt` list (nothing to acknowledge).
- Emits a one-line confirmation with the path and reason.

### Sync-time filter

`governa sync` reads the `acknowledged` section. For each entry where both SHAs still match current state, the file is omitted from the `## Adoption Items` list and from the `adopt` count in the summary. Stale acknowledgments (either SHA mismatched) are surfaced as advisory notes and the file returns to normal adopt-flow treatment.

## Out Of Scope

- Per-region / per-hunk acknowledgment granularity. File-level is v1; sub-file granularity is a v2 consideration.
- Bulk acknowledgment via single command (`governa ack --all` or similar) — intentional omission to force per-file rationale.
- Automatic acknowledgment by heuristic. Acknowledgment must be explicit.
- `CHANGELOG.md` release row — added at release prep, not during implementation.

## Implementation Notes

- The `acknowledged` section lives inside `.governa/manifest` (single file, consistent with AC55's consolidation).
- Parsing: hand-rolled stdlib, matching the format the rest of `.governa/manifest` already uses. No external Go dependencies (per repo constraint).
- SHA algorithm: `sha256` over raw file contents.
- CLI flag parsing: follow existing `cmd/governa/main.go` patterns.

## Open Design Questions

### Review-doc rendering for acknowledged items

Three options for how `.governa/sync-review.md` presents them:

1. **Hidden entirely** — cleanest signal; review doc only shows items that need action. Risk: acknowledgments drift out of the director's view over time.
2. **Dedicated `## Acknowledged Drift` section** — auditable; list with reasons visible each sync.
3. **Inline `[acknowledged]` tag** in the Adoption Items table — keeps in context but clutters the list.

Lean: 2 (dedicated section). Decide before critique.

### Handoff with `promoteStandingDrift`

The existing mechanism surfaces stable drift as a director hint. Acknowledgment is the resolution pathway — standing drift gets either adopted or acknowledged. Review doc should suggest both outcomes explicitly when drift is promoted: "adopt template OR run `governa ack <path> --reason "..."`".

### Re-acknowledgment workflow when template changes

When the template version of a file changes in a later release, existing acknowledgments become stale (template-sha mismatch). Director re-acknowledges after reviewing what changed upstream. Open question: should `governa ack <path>` support a `--renew-if-unchanged-reason` shortcut that updates SHAs while preserving the reason, or should every re-acknowledgment require a fresh `--reason`? Tension: friction (fresh reason = more deliberate) vs pragmatism (stable carve-outs need the same reason forever).

### Stale-entry hygiene for deleted files

If a consumer deletes a file that has an acknowledgment entry, the entry becomes orphaned. Options: auto-prune at sync time (lower friction) vs explicit `governa ack --remove <path>` (more auditable). Lean auto-prune with a one-line sync-output note about which orphans were cleaned.

## Acceptance Tests

**AT1** [Automated] — `governa ack <path> --reason "..."` on an `adopt`-flagged file appends a well-formed entry to `.governa/manifest` containing `path`, `consumer-sha`, `template-sha`, `template-version`, and `reason`.

**AT2** [Automated] — `governa ack <path>` without `--reason` exits non-zero with a clear error.

**AT3** [Automated] — `governa ack <path>` on a file not currently in the `adopt` list exits non-zero with a "nothing to acknowledge" error.

**AT4** [Automated] — `governa sync` omits acknowledged files from `## Adoption Items` and from the adopt count when both SHAs still match current state. Unit test seeds a manifest with a valid acknowledged entry and asserts the file is absent from the review output's adopt list.

**AT5** [Automated] — `governa sync` re-flags an acknowledged file when the consumer SHA changes. Unit test seeds a manifest, mutates the consumer file, re-runs sync, asserts the file appears in `## Adoption Items` and an advisory notes the stale acknowledgment.

**AT6** [Automated] — `governa sync` re-flags an acknowledged file when the template SHA changes. Similar unit test with template-side mutation.

**AT7** [Automated] — `./build.sh` exits 0.

(Additional ATs added as open design questions are settled: review-doc rendering, standing-drift handoff, re-acknowledgment shortcut, orphan-entry pruning.)

## Documentation Updates

- `docs/governance-model.md` — new section describing the acknowledgment mechanism and SHA-binding semantics.
- `docs/build-release.md` + overlay + example — reference `governa ack` in the template-upgrade workflow.
- `docs/roles/dev.md` + overlay + example — add acknowledgment usage to the DEV templating-maintenance section.
- `cmd/governa/main.go` — new `ack` subcommand wiring.
- `internal/governance/governance.go` — acknowledged-section parsing, sync-time filter, `ack` command implementation, review-doc rendering (pending rendering decision).
- `internal/governance/governance_test.go` — coverage for all of the above.
- `CHANGELOG.md` — release row added at release prep, not during implementation.

## Status

`PENDING` — growing. Currently scoped: manifest `acknowledged` section with dual SHA-scoping, `governa ack` CLI command, sync-time filter. Open design questions for review-doc rendering, standing-drift handoff, re-acknowledgment workflow, and orphan-entry pruning — to be settled before critique initiation.
