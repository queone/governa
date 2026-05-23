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
- `governa/ac-template.md`
- `governa/build-release.md`
- `governa/development-cycle.md`
- `governa/development-guidelines.md`
- `governa/README.md`
- `governa/roles.md`
- `internal/preptool/preptool.go`
- `internal/preptool/preptool_test.go`
- `plan.md`
- `README.md`

Go-stack packages (`cmd/build`, `cmd/prep`, `cmd/rel`, `internal/preptool`) are included only when the stack suggests Go. Color helpers come from the `github.com/queone/governa-color` library. Build pipeline orchestration comes from `github.com/queone/governa-buildtool`, imported by the rendered `cmd/build`. Release orchestration (semver tag + git push) comes from `github.com/queone/governa-reltool`, imported by the rendered `cmd/rel`.

See `plan.md` for future overlay improvements.
