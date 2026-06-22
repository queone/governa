# DOC Overlay

Governance + planning + release tooling for documentation repos. Editorial structure (voice guides, style guides, publishing workflows) is the repo owner's domain.

Current concrete templates live under `files/`.

Current contents:

- `.gitignore`
- `AGENTS.md`
- `build.sh`
- `CHANGELOG.md`
- `README.md`
- `governa/ac-template.md`
- `governa/canon-cycle.md`
- `governa/drift-scan.md`
- `governa/editing-cycle.md`
- `governa/editing-guidelines.md`
- `governa/operator-contract-rationale.md`
- `governa/README.md`
- `governa/release.md`
- `governa/roles.md`
- `plan.md`

`build.sh` is a self-contained Bash 3.2+ script for release preparation and
annotated-tag release orchestration. Generated DOC repos require no Go
toolchain for those workflows and define no automated content validation.
