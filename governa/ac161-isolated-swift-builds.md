# AC161 Isolated Swift Builds

## Summary

Rudimentary stub — requires further scoping before critique gate or implementation authorization. Isolate SwiftPM build artifacts outside consumer repositories and install executable products safely after successful validation.

## In Scope

### Candidate files to modify

- `internal/templates/overlays/code/stacks/swift/build.sh.tmpl` — create and clean an invocation-owned external SwiftPM scratch directory.
- `internal/templates/overlays/code/stacks/swift/build.sh.tmpl` — discover executable products and install resolved release artifacts outside the repository.
- `internal/governance/governance_test.go` — verify isolation, cleanup, product discovery, installation, and failure behavior.
- `README.md` — document Swift build isolation and executable installation.
- `arch.md` — document Swift artifact and installation boundaries.

## Out Of Scope

- Add scoped product selection.
- Add a separate Swift utility repository.
- Finalize destination, collision, signal-cleanup, or library-only behavior before toolchain research.
- TBD — resolve detailed isolation and installation behavior during scoping.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — TBD — define scratch-directory lifecycle and cleanup assertions during scoping.

**AT2** [Automated] [Pre-release gate] — TBD — define executable-product discovery and installation assertions during scoping.

**AT3** [Automated] [Pre-release gate] — TBD — define unsafe-path, collision, interruption, and failure assertions during scoping.

## Status

`PENDING` — requires scoping before critique gate or implementation authorization.
