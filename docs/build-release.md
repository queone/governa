# Build And Release

## Build And Test Rules

- use one canonical local build command and keep this document current
- run formatting, static checks, tests, and packaging through that command or documented sequence
- do not trigger release work during routine implementation

This repo is Go-based and keeps the real implementation in:

- `cmd/build/main.go`
- `cmd/rel/main.go`

The root `build.sh` script is a convenience wrapper for Unix, Linux, and Git-Bash environments.

## Minimum Validation

- formatting passes
- static checks pass
- automated tests pass
- changed docs match actual behavior

## Canonical Build Commands

```bash
go run ./cmd/build
```

Convenience wrapper:

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

If you pass `build`, `bootstrap`, or `rel` as targets, the command will validate those entrypoints but will not install binaries for them.

## Canonical Release Commands

```bash
go run ./cmd/rel vX.Y.Z "release message"
```

Convenience wrapper:

```bash
./build.sh vX.Y.Z "release message"
```

The release command always asks for interactive confirmation before it runs the git steps.

## Release Trigger

Only perform release work when the user explicitly asks for release, publish, or version preparation.

## Release Checklist

1. run the canonical build and validation flow
2. confirm root docs and architecture notes are current
3. update `CHANGELOG.md` for the release
4. confirm `TEMPLATE_VERSION` matches the template release version being prepared
5. prepare version or publish artifacts only within the explicit release request

## Pre-Release Checklist

Do not start this checklist unless the user explicitly asks to prep for release or equivalent.

Before offering a release commit or release command:

1. run the canonical build and validation flow and fix failures until clean
2. ask the user whether any required manual or live acceptance checks were run
3. audit `arch.md` and any affected reference docs against the actual behavior
4. move the current `Unreleased` changelog summary into the release entry and leave a fresh `Unreleased` row
5. confirm `TEMPLATE_VERSION` matches the intended template release version
6. remove or reprioritize completed roadmap items in `plan.md`
7. remove completed AC files — consolidate their decisions into durable docs and delete the AC files before release; release prep is not complete while completed AC files remain (keep `ac-template.md` and designated teaching artifacts such as `ac-example.md`)
8. prepare the exact tag and release message — the release message must be a single concise line, 80 characters or fewer
9. present the canonical release command for the user to run or approve

## Release Artifacts

- `CHANGELOG.md` is the human-readable release history
- `TEMPLATE_VERSION` is the machine-readable template contract version used by generated repos and future refresh tooling
- for this repo, keep `TEMPLATE_VERSION` aligned with the released template version unless there is a deliberate reason to version them separately
