# Drift Scan

When the user invokes `drift <repo-path>`, follow this protocol.

## Protocol

- Scan the named adopted repo against governa canon.
- Stage findings as an IE in the target repo's `plan.md` (shape (a) or (b) per `plan.md`'s docstring) and an AC stub in its `docs/`.
- One repo per invocation.
- No commits in the target repo.
- The user is responsible for asserting the path is an adopted-governa repo.

## Small-drift simplification

When the drift is small — one or two lines across one or two files — state that explicitly in the AC's Summary and keep every section proportional: terse Objective Fit, In Scope as the literal change, `None` or omitted Out Of Scope / Implementation Notes / Director Review, minimal Acceptance Tests. Do not pad sections to look complete. Ceremony is proportional to the change.
