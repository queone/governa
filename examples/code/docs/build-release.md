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

## Pre-Release Checklist

Do not start this checklist unless the user explicitly asks to prep for release or equivalent.

Before offering a release commit or release command:

1. run the canonical build and validation flow and fix failures until clean
2. ask the user whether any required manual or live acceptance checks were run
3. audit `arch.md` and any affected reference docs against the actual behavior
4. update `CHANGELOG.md` or the repo's release-history artifact
5. confirm the repo's version artifact matches the intended release version
6. remove or reprioritize completed roadmap items in `plan.md`
7. remove completed AC files — consolidate their decisions into durable docs and delete the AC files before release; release prep is not complete while completed AC files remain (keep `ac-template.md` and designated teaching artifacts such as `ac-example.md`)
8. prepare the exact tag and release message — the release message should be a single concise line, 80 characters or fewer
9. present the canonical release command for the user to run or approve

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

## Release Artifacts

- `TEMPLATE_VERSION` is the template contract version written at bootstrap time. It records which template version this repo was generated from.
- `CHANGELOG.md`, if the repo maintains one, is the human-readable release history. Keep it current as the canonical record of what shipped in each version.

## Template Upgrade

This repo was generated from a governance template. To check for template updates:

1. Compare `TEMPLATE_VERSION` in this repo against the source template's current version.
2. Diff changed files manually against the template's current overlays.
3. `.repokit-manifest`, if present, records SHA-256 checksums of each file at bootstrap time. This enables tooling-assisted comparison to distinguish your customizations from stale template content.

Template refresh is an operator-driven review process. The template maintainer may use enhance mode from the template repo to identify improvements, but generated repos do not run enhance directly.
