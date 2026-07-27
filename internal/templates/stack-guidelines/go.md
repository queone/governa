## Go Practices

- Add single-line godoc comments to exported functions in shared Go packages.
- Declare a non-empty `const programVersion` string literal in every installable `cmd/<name>/main.go`.
- Validate every `programVersion` declaration through `build.sh` before compiling installable binaries.
- Scan all `.go` and `.go.tmpl` files for stale import paths after a module rename.
