# AC153 Require Canonical Build First

## Summary

Amend the Governa build-verification contract so Operators run the repository's canonical `./build.sh` before any routine formatter, test, vet, or static-analysis command. Reserve direct tool invocations for diagnosis of a failure reported by the latest canonical build, and require a final canonical rerun after diagnosis.

## In Scope

### Files to modify

- `AGENTS.md` — strengthen `## Base Rules` → `### Build Verification`.
- `internal/templates/base/AGENTS.md` — mirror the CODE consumer build-verification rules.
- `internal/templates/overlays/doc/files/AGENTS.md.tmpl` — mirror the DOC consumer build-verification rules.
- `internal/governance/governance_test.go` — verify rendered CODE and DOC contracts contain the strengthened rules.

### Build-verification contract

- Run `./build.sh` first after every implementation change.
- Let `./build.sh` perform routine formatting, testing, vetting, and static analysis.
- Do not invoke direct formatting, testing, vetting, or static-analysis commands before the first canonical build.
- Use a direct tool command only to diagnose a corresponding failure reported by the latest `./build.sh`.
- Scope each diagnostic command to the reported failure.
- Rerun `./build.sh` after every diagnostic or corrective pass.
- Treat work as unverified until the final `./build.sh` succeeds.
- Preserve direct inspection commands and isolated binary smoke commands already allowed by the contract.

## Out Of Scope

- Change `build.sh`.
- Change which checks `build.sh` performs.
- Ban direct commands used for read-only inspection.
- Ban focused commands used to diagnose a canonical-build failure.
- Change approval, release, or commit gates.
- Modify AC152 implementation behavior.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — Confirm source `AGENTS.md` instructs the Operator to run `./build.sh` first after implementation changes and forbids routine direct validation before that run.

**AT2** [Automated] [Pre-release gate] — Confirm source `AGENTS.md` permits a direct tool command only for a corresponding failure reported by the latest canonical build and requires a subsequent `./build.sh` rerun.

**AT3** [Automated] [Pre-release gate] — Render CODE and DOC consumers and confirm both generated `AGENTS.md` files contain the same strengthened build-verification rules.

**AT4** [Automated] [Pre-release gate] — Confirm the source, CODE base template, and DOC overlay retain identical build-verification instructions while preserving their existing project-specific sections.

**AT5** [Automated] [Pre-release gate] — Run `./build.sh` first and confirm the full Governa validation passes.

## Status

`PENDING` — awaiting user authorization to begin implementation.
