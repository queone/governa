# AC162 Swift Build Presentation

## Summary

Rudimentary stub — requires further scoping before critique gate or implementation authorization. Align Swift CODE build, prep, and release presentation with the established Go and Rust interaction contract.

## In Scope

### Candidate files to modify

- `internal/templates/overlays/code/stacks/swift/build.sh.tmpl` — align phases, colors, command previews, help, errors, prep output, and release output.
- `internal/governance/governance_test.go` — verify plain, colored, verbose, failure, prep, and release presentation.
- `governa/build-release.md` — document any stack-neutral presentation rule clarified by Swift adoption.
- `internal/templates/overlays/code/files/governa/build-release.md.tmpl` — propagate any consumer-facing release rule.
- `README.md` — document Swift build presentation where user-visible behavior requires it.

## Out Of Scope

- Change Go or Rust behavior except for an explicitly approved stack-neutral correction.
- Add scoped product selection.
- Add a separate Swift utility repository.
- TBD — resolve the exact parity matrix during scoping.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — TBD — define phase-order, wording, and command-preview assertions during scoping.

**AT2** [Automated] [Pre-release gate] — TBD — define color-policy and plain-output parity assertions during scoping.

**AT3** [Automated] [Pre-release gate] — TBD — define help, failure, prep, and release golden assertions during scoping.

## Status

`PENDING` — requires scoping before critique gate or implementation authorization.
