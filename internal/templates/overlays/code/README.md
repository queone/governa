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

`build.sh` is a self-contained Bash script that handles build, release-prep, and release orchestration. It carries its stack-specific pipeline inline, requires no external governa tools, and targets Bash 3.2+ so macOS system Bash is supported. Rust CODE builds validate independent utility declarations and compiled `--version` output, run their rendered build CLI regression suite, and report each installed utility with its version. Swift CODE builds require Swift 6 and a root `Package.swift`; they isolate SwiftPM artifacts outside the repository, keep formatting and tests package-wide, build full or selected (scoped) executable products, and install release executables into `${SWIFT_BIN_HOME:-$HOME/.local/bin}` with canonical color or plain-text presentation. Library-only Swift packages validate without installation.

See `plan.md` for future overlay improvements.
