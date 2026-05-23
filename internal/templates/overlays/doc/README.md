# DOC Overlay

Governance + planning + release tooling for documentation repos. Editorial structure (voice guides, style guides, publishing workflows) is the repo owner's domain.

Current concrete templates live under `files/`.

Current contents:

- `.gitignore`
- `AGENTS.md`
- `cmd/rel/main.go`
- `governa/ac-template.md`
- `governa/editing-cycle.md`
- `governa/editing-guidelines.md`
- `governa/README.md`
- `governa/release.md`
- `governa/roles.md`
- `plan.md`
- `rel.sh`

Color helpers and release orchestration come from `github.com/queone/governa-color` and `github.com/queone/governa-reltool`, imported by the rendered `cmd/rel`.
