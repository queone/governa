# Drift Scan

When the user invokes `drift-scan <repo-path>`, follow this protocol.

## Protocol

- Scan the named adopted repo against governa canon.
- Stage findings as an IE in the target's `plan.md` (shape (a) or (b) per `plan.md`'s docstring) and an AC stub in its `docs/`.
- One repo per invocation. No commits in the target repo.
- Assume the user has asserted the path is an adopted-governa repo.

## AC content requirements

The AC stub must be implementable standalone — the target's Operator should not need governa access.

### Canonical text

- Inline canonical replacement text verbatim under `## Implementation Notes` in fenced code blocks (```` ``` ````), one block per replacement.
- Pin canon by governa commit SHA + path (e.g., `governa @ d87e003: internal/templates/overlays/<flavor>/files/docs/ac-template.md.tmpl`). SHA, not version.
- Quote the full enclosing section (heading + every line) when the replacement covers only part of a section.
- Include the disclaimer: `paste exactly the content between the fence markers — the `` ``` `` markers are AC presentation only`.
- Never use blockquotes (`>`) for canonical text.
- Do not paraphrase.

### Structural call-outs

- Explicitly name what stays unchanged when the change touches a heading, section name, or top-level shape.
- Pair every "unchanged" call-out with an Acceptance Test using:
  - Full-literal patterns covering the entire line/sentence/heading (not prefixes).
  - Count assertions when uniqueness is implied: `[ "$(rg -c '<pattern>' <file>)" = N ]`.
- Add a list AT pinning the file's top-level heading sequence (`rg '^## ' <file>`) when scoping changes within named sections.

### Divergence classification

For every divergent file:

1. Grep target's `CHANGELOG.md` and `docs/ac*.md` for: `preserve`, `do not sync`, `intentional divergence`, or AC references locking the local form.
2. Run `git log -n 5 --follow -- <file>` in the target. Cite every returned commit verbatim under `## Implementation Notes`. Do not abridge.

Route by exactly one outcome:

- **Preserve marker found** → frame as intentional in `## Out Of Scope`, cite the row or AC verbatim.
- **Local commits, no preserve marker** → stage a separate IE in the target's `plan.md` (shape (a)): `IE<N>: drift-scan ambiguity in <path> — sync to canon or keep local? <one-line rationale>`. Do not add to this AC's `## Director Review`. Do not soften with "could be intentional" in `## Out Of Scope`. When the rationale characterizes canon (e.g., "canon now requires X"), anchor it with a SHA-pinned excerpt of the relevant canon line(s) under `## Implementation Notes`.
- **Neither** → include in this AC's `## In Scope` or stage a follow-up AC. Do not bury in `## Out Of Scope`.

### Match evidence

For every file classified as `match` (canon and target identical), name the comparison command used under `## Implementation Notes` (e.g., `diff -u <canon-template-path> <target-path>` returning empty). Do not assert `match` without naming the check.

### Refinement tracing

When canonical text overwrites a section the target touched recently (check `git log -n 5 --follow -- <file>`), call out in `## Implementation Notes` which local wording is preserved verbatim in canon vs which is superseded.

### Post-merge coherence audit

Before staging, mentally apply the canonical replacement and read the post-merge state. Surface contradictions, redundancies, or self-references under a `Post-merge coherence audit` subsection of `## Implementation Notes`. Attribute each as either pre-existing in canon (point at a follow-up governa-side AC) or introduced by this change (resolve before staging).

## Small-drift simplification

When drift is one or two lines across one or two files: state this in the Summary, keep sections proportional (terse Objective Fit, literal In Scope, `None` or omitted Out Of Scope / Director Review, minimal ATs). The AC content requirements above still apply. Do not pad to look complete.

When every divergent file routes to a preserve marker or an ambiguity IE — `In Scope` is `None` — state in the Summary that the AC ships only itself plus the staged `plan.md` IE entries (no file edits).

