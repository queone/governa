## Rust Practices

- Run all repository validation through `./build.sh`.
- Format Rust code with rustfmt.
- Treat Clippy warnings as build failures.
- Test all targets and all features before handoff.
- Document public Rust items with rustdoc comments.
- Return contextual errors instead of discarding error sources.
- Confine `unsafe` code to the smallest practical scope.
- Document the safety invariant for every `unsafe` block.
- Prefer the standard library when it provides equivalent capability.
- Justify every added crate in the governing AC.
- Pin direct dependencies to explicit compatible versions in `Cargo.toml`.
- Keep `Cargo.lock` tracked for application repositories.
