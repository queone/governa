## Swift Practices

- Run all repository validation through `./build.sh`.
- Require Swift 6.0 or newer.
- Declare public executables as explicit SwiftPM executable products.
- Keep `Package.resolved` tracked for leaf packages with dependencies.
- Treat `Package.resolved` as optional for dependency libraries.
- Keep project-level `.swiftpm/` configuration tracked.
- Format Swift code with the toolchain-provided formatter.
- Treat formatter findings and compiler warnings as failures.
- Document public APIs with documentation comments.
- Avoid Xcode-only assumptions in cross-platform SwiftPM code.
- Reserve `SWIFT_BIN_HOME` for Governa-managed executable installation.
