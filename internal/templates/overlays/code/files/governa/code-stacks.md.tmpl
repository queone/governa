# First-Class CODE Stacks

Use this reference for stack selection, canonical validation, artifacts, installation, release prep, and scoped-build behavior.

## Go

- Infer Go from `go.mod`; select it explicitly with `--stack Go`.
- Require the Go toolchain and the pinned staticcheck version installed by `build.sh`.
- Run dependency tidying, formatting, fixes, vetting, tests with coverage, staticcheck, and compilation.
- Install command binaries into `$(go env GOPATH)/bin`.
- Bump the single detected `programVersion` during release prep; skip ambiguous multi-utility version bumps.
- Accept command names for scoped builds while retaining package-wide shared validation.

## Rust

- Infer Rust from `Cargo.toml`; select it explicitly with `--stack Rust`.
- Require Cargo, rustfmt, and Clippy.
- Run formatting, Clippy, tests, and release compilation.
- Keep compilation in an invocation-owned external Cargo target.
- Install binaries into `$CARGO_HOME/bin`, or `$HOME/.cargo/bin` when `CARGO_HOME` is unset.
- Bump the root package version and refresh `Cargo.lock` during release prep.
- Accept declared binary names for scoped builds and preserve package-wide shared validation.

## Terraform

- Infer Terraform from `.terraform.lock.hcl` or root Terraform files; select it explicitly with `--stack Terraform`.
- Require the Terraform CLI.
- Run recursive formatting checks and module validation.
- Keep Terraform working data in repository-local ignored artifact directories.
- Derive release versions from Git tags without a source version bump.
- Reject scoped builds because Terraform validation is repository-wide.

## Swift

- Infer Swift from a root `Package.swift`; select it explicitly with `--stack Swift`.
- Prefer Go, Terraform, and Rust manifests over Swift; prefer Swift over Node, Python, and Java manifests.
- Require Swift 6.0 or newer, Git, and one root SwiftPM package on macOS or Linux.
- Run strict toolchain formatting, debug compilation, tests, and release compilation with compiler warnings as errors.
- Keep SwiftPM artifacts in one invocation-owned external scratch directory and clean it on success, failure, and handled signals.
- Keep project-level `.swiftpm/` configuration trackable.
- Keep `Package.resolved` tracked for leaf packages with dependencies; treat it as optional for dependency libraries.
- Derive release versions from Git tags and leave `Package.swift` unchanged during release prep.
- Install executable products into `${SWIFT_BIN_HOME:-$HOME/.local/bin}` by atomically replacing regular destination files and refusing unsafe entries.
- Let library-only packages complete without installation.
- Use the canonical color and plain-text presentation policy for build, prep, and release output.
- Accept executable-product names for scoped builds while retaining package-wide formatting and tests.
- Build and install only selected executable products during scoped builds.
- Treat native Xcode projects and Apple application bundles as a possible future backend.
