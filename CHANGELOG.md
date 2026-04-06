# Changelog

| Version | Summary |
|---------|---------|
| Unreleased | |
| 0.1.0 | Deterministic Go bootstrap tooling for `new`, `adopt`, and `enhance`; `CODE` and `DOC` overlays with rendered examples; Go-based `cmd/build` and `cmd/rel` workflows with thin shell wrappers; self-hosted root governance artifacts so this repo operates as a governed `CODE` repo; path-safe enhancement reporting and path-hygiene rules; terminal coloring in build and release tooling; QA review fixes: Go-stack detection uses word-boundary matching, `color.go.tmpl` skipped for non-Go stacks, release tool shows `git status` before staging, `build.sh` routes single-arg semver to `cmd/rel`, root `AGENTS.md` symlinked to `base/AGENTS.md`, `go vet` and `staticcheck` now fail the build on errors, `.gitignore` added for template and generated repos |
