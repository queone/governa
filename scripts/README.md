# Scripts

This directory will hold the stable bootstrap entrypoint definition.

The implementation should be Go-based rather than shell-based.

Current invocation shape:

```bash
go run <template-root>/cmd/bootstrap ...
```

Expected responsibilities:

- accept a target path or reference path plus explicit metadata inputs
- support `new`, `adopt`, and `enhance` modes
- assess template fit before modifying an existing repo
- assess enhancement candidates before modifying this template repo
- write a deterministic enhancement review artifact before any template-maintenance edits are considered
- select `CODE` or `DOC`
- fill placeholders
- write concrete files into a target repo
- create `CLAUDE.md -> AGENTS.md`
- write `TEMPLATE_VERSION`
- support `--dry-run`
- support single-letter short flags plus long-form aliases for every argument

Recommended implementation shape:

- `cmd/bootstrap/` for the Go program
- this directory for bootstrap-facing docs and any thin entrypoint wrapper if needed
