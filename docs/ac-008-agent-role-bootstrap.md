# AC-008 Agent Role Bootstrap Pattern

## Objective Fit

1. Agents get role-specific behavior guidance without polluting the shared governance contract
2. Users can assign DEV, QA, or custom roles at session start; agents ask if no role is specified
3. The pattern is agent-agnostic — any LLM agent can read the role docs
4. Direct roadmap work — improves the interaction model for governed repos

## Summary

Add a role bootstrap mechanism to governed repos. When `docs/agent-roles/` exists, the agent checks whether a role has been assigned. If not, it asks the user which role to assume. After selection, it reads `docs/agent-roles/<role>.md` and follows it alongside `AGENTS.md`. This keeps `AGENTS.md` focused on repo-wide governance while role-specific behavior lives in dedicated docs.

## In Scope

- Add one bullet to `Interaction Mode` in `base/AGENTS.md` describing the role bootstrap behavior
- Create `docs/agent-roles/README.md` — index explaining the pattern
- Create `docs/agent-roles/dev.md` — DEV role: implementation-focused, follows build/test/release process
- Create `docs/agent-roles/qa.md` — QA role: review-focused, audits behavior against docs and tests
- Add CODE overlay templates: `docs/agent-roles/README.md.tmpl`, `docs/agent-roles/dev.md.tmpl`, `docs/agent-roles/qa.md.tmpl`
- Self-host the same pattern in this repo (root `docs/agent-roles/`)
- Regenerate rendered examples (`examples/code/docs/agent-roles/`)
- Update overlay README to list the new template files

## Out Of Scope

- Adding new governed sections to AGENTS.md (role bootstrap is one bullet in Interaction Mode)
- Runtime role switching mid-session
- Agent-specific memory or hooks (role docs are plain markdown)
- DOC overlay roles (can be added later; CODE overlay is the immediate need)

## Implementation Notes

- The AGENTS.md change is a single bullet in Interaction Mode: "If `docs/agent-roles/` exists and the user has not explicitly assigned a role, the agent's first substantive response must ask which role to assume. Role assignment requires an explicit instruction such as 'act as DEV', 'use docs/agent-roles/qa.md', or 'you are QA'. Tone conventions or prefixes (e.g. 'QA says...') are not role selectors. After assignment, read `docs/agent-roles/<role>.md` and follow it alongside this file. If the requested role file does not exist, say so and continue under shared governance only."
- Role name lookup is case-insensitive: the user input is lowercased before mapping to `docs/agent-roles/<role>.md` (e.g. "DEV", "Dev", "dev" all resolve to `dev.md`)
- Role docs should be concise (under 30 lines each) — they supplement AGENTS.md, not replace it
- `dev.md` seed rules: write test coverage for every code change; follow the repo's build/test/release process; propagate fixes to overlay templates and rendered examples. These are starting points — the file is expected to grow with project-specific instructions over time.
- `qa.md` seed rules: start every response with "QA says"; use objective QA language ("Observed", "Expected", "Verify that", "Requirement") instead of anthropomorphic phrasing; prioritize findings over summaries. These are starting points — the file is expected to grow over time.
- `README.md` explains the pattern, lists available roles, describes how to add custom roles, and documents the case-insensitive lookup rule

## Known Limitations

- Role bootstrap behavior is instruction-driven and may vary by agent or client. The template generates the correct governance rule and role docs, but cannot enforce compliance across every LLM runtime. For reliable behavior, explicitly assign the role at session start rather than relying only on the agent to ask.
- `docs/agent-roles/README.md` should include an operator-facing note: "For deterministic role selection, assign the role explicitly at session start (e.g. 'act as DEV') rather than waiting for the agent to prompt."

## Acceptance Tests

- [Manual] Fresh session with no role assigned: agent's first substantive response is the role question — no repo analysis, planning, or other work before asking
- [Manual] Fresh session with explicit role assignment (e.g. "act as DEV", "use docs/agent-roles/qa.md"): agent loads matching role doc and proceeds without asking
- [Manual] Tone conventions alone (e.g. "QA says...") do not trigger role loading — agent still asks for explicit assignment
- [Manual] Requested role file does not exist (e.g. "act as OPS"): agent states the file was not found and continues under shared governance only
- [Manual] Case-insensitive lookup: "act as dev", "act as DEV", "act as Dev" all resolve to `dev.md`
- [Manual] After explicit QA assignment: agent responses start with "QA says" and use objective QA language ("Observed", "Expected", "Verify that", "Requirement")
- [Manual] After explicit DEV assignment: agent includes test coverage when implementing code changes
- [Automated] Bootstrap `new` mode for CODE produces `docs/agent-roles/dev.md` and `docs/agent-roles/qa.md`
- [Automated] Bootstrap `adopt` mode proposes `docs/agent-roles/` files via existing overlay propose-if-exists machinery
- [Manual] Generated `dev.md` explicitly states the test coverage requirement
- [Manual] Generated `qa.md` explicitly states the "QA says" prefix and objective language requirements
- [Manual] This repo's own `docs/agent-roles/` matches the pattern

## Documentation Updates

- `base/AGENTS.md` — one bullet added to Interaction Mode
- `overlays/code/README.md` — list new template files
- `docs/bootstrap-model.md` — mention role docs as part of the generated repo structure

## Status

COMPLETE
