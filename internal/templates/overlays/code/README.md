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
- `internal/color/color.go`
- `internal/color/color_test.go`
- `internal/preptool/preptool.go`
- `internal/preptool/preptool_test.go`
- `internal/reltool/reltool.go`
- `internal/reltool/reltool_test.go`
- `plan.md`
- `README.md`

Go-stack packages (`cmd/build`, `cmd/prep`, `cmd/rel`, `internal/buildtool`, `internal/preptool`, `internal/reltool`, `internal/color`) are included only when the stack suggests Go.

See `plan.md` for future overlay improvements.
