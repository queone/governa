# CODE Overlay

This overlay will own code-repo artifacts and rules only.

Current concrete templates live under `files/`.

Current contents:

- `.gitignore`
- `arch.md`
- `build.sh`
- `CHANGELOG.md`
- `cmd/build/main.go`
- `cmd/prep/main.go`
- `cmd/rel/main.go`
- `docs/ac-template.md`
- `docs/build-release.md`
- `docs/critique-protocol.md`
- `docs/development-cycle.md`
- `docs/development-guidelines.md`
- `docs/README.md`
- `docs/roles.md`
- `internal/buildtool/buildtool.go`
- `internal/buildtool/buildtool_test.go`
- `internal/preptool/preptool.go`
- `internal/preptool/preptool_test.go`
- `plan.md`
- `README.md`

Go-stack packages (`cmd/build`, `cmd/prep`, `cmd/rel`, `internal/buildtool`, `internal/preptool`) are included only when the stack suggests Go. Color helpers come from the `github.com/queone/governa-color` library, imported by the rendered `buildtool` package and by the rendered `cmd/rel`. Release orchestration (semver tag + git push) comes from the `github.com/queone/governa-reltool` library, imported by the rendered `cmd/rel`.

See `plan.md` for future overlay improvements.
