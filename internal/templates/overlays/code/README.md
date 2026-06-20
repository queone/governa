# CODE Overlay

This overlay will own code-repo artifacts and rules only.

Current concrete templates live under `files/`.

Current contents:

- `.gitignore`
- `arch.md`
- `build.sh`
- `CHANGELOG.md`
- `governa/ac-template.md`
- `governa/build-release.md`
- `governa/development-cycle.md`
- `governa/development-guidelines.md`
- `governa/README.md`
- `governa/roles.md`
- `plan.md`
- `README.md`

`build.sh` is a self-contained Bash script that handles build, release-prep, and release orchestration. It carries its pipeline and color helpers inline, requires no external governa tools, and targets Bash 3.2+ so macOS system Bash is supported.

See `plan.md` for future overlay improvements.
