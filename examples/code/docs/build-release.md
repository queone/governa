# Build And Release

## Build And Test Rules

- define one canonical local build command and keep it current here
- run formatting, linting, tests, and packaging through that canonical command or documented sequence
- do not trigger release work during routine implementation

If this repo is Go-based and includes `cmd/build/main.go`, use it for the build/test action itself rather than embedding build behavior in shell scripts.
If this repo is Go-based and includes `cmd/rel/main.go`, use it for the release action itself rather than embedding release behavior in shell scripts.
If this repo also includes `cmd/bootstrap/main.go`, treat it as a `go run` maintenance entrypoint rather than an installed binary.

## Minimum Validation

- formatting passes
- lint or static checks pass if present
- automated tests pass
- changed docs match actual behavior

## Release Trigger

Only perform release work when the user explicitly asks for release, publish, or version preparation.

## Release Checklist

1. run the canonical build and validation flow
2. confirm user-visible docs and architecture notes are current
3. update release notes or changelog if the repo uses them
4. prepare version or publish artifacts only within the explicit release request

## Go Build Tool

If this repo includes `cmd/build/main.go`, the expected build invocation is:

```bash
go run ./cmd/build
```

If the repo also includes `build.sh`, that shell wrapper is a convenience alias for Unix, Linux, and Git-Bash environments:

```bash
./build.sh
```

To scope the run to selected commands:

```bash
go run ./cmd/build bootstrap rel
```

or:

```bash
./build.sh bootstrap rel
```

If you pass `build`, `bootstrap`, or `rel` as targets, the command will run checks for those entrypoints but will not install binaries for them.

Use `-v` or `--verbose` to run tests in verbose mode.

## Go Release Tool

If this repo includes `cmd/rel/main.go`, the expected release invocation is:

```bash
go run ./cmd/rel vX.Y.Z "release message"
```

The command always asks for interactive confirmation before it runs the release git steps.
The only supported option is help: `-h`, `-?`, or `--help`.

If the repo includes `build.sh`, the same release can be triggered through the shell convenience wrapper:

```bash
./build.sh vX.Y.Z "release message"
```
