# AC163 Scoped Swift Builds

## Summary

Rudimentary stub — requires further scoping before critique gate or implementation authorization. Add single- and multi-executable Swift build selection using declared SwiftPM executable products while preserving required package-wide validation.

## In Scope

### Candidate files to modify

- `internal/templates/overlays/code/stacks/swift/build.sh.tmpl` — accept, validate, build, test, and install selected executable products.
- `internal/governance/governance_test.go` — verify single-product, multi-product, invalid-product, library-only, color, verbose, and target-isolation behavior.
- `governa/build-release.md` — document Swift scoped-build behavior where the contract is stack-neutral.
- `governa/development-guidelines.md` — document Swift product-selection conventions where required.
- `internal/templates/overlays/code/files/governa/build-release.md.tmpl` — propagate affected consumer release guidance.
- `internal/templates/overlays/code/files/governa/development-guidelines.md.tmpl` — propagate affected consumer development guidance.
- `internal/templates/stack-guidelines/swift.md` — document Swift-specific selector and test conventions.
- `README.md` — document scoped Swift build usage.

## Out Of Scope

- Select arbitrary internal Swift targets that are not executable products.
- Add multi-package workspace support.
- Add a separate Swift utility repository.
- TBD — resolve SwiftPM product-to-test mapping and shared-validation limits during scoping.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — TBD — define valid single- and multi-product routing assertions during scoping.

**AT2** [Automated] [Pre-release gate] — TBD — define invalid, duplicate, library-only, and malformed selector assertions during scoping.

**AT3** [Automated] [Pre-release gate] — TBD — define package-wide validation, selected installation, color, and verbose parity assertions during scoping.

## Status

`PENDING` — requires scoping before critique gate or implementation authorization.
