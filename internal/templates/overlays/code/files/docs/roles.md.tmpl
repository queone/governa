# Roles

Each `docs/role-<name>.md` file is a behavioral contract that an agent reads and follows alongside `AGENTS.md`.

**Instruction traceability:** `AGENTS.md` is the shared repo contract loaded every session. Each role file adds role-specific behavior for the assigned role. When both apply, follow `AGENTS.md` plus the assigned role file together. If a role file conflicts with `AGENTS.md`, `AGENTS.md` wins unless the repo intentionally says otherwise.

## Delivery Model

- **Director (human).** Owns intent, priorities, and irreversible decisions: defining success criteria, approving requirements and acceptance criteria, prioritizing the backlog, making architectural bets, approving releases, and deciding what "done" means. Agents will otherwise either gold-plate or cut corners. The director also owns the meta-loop — reviewing how agents perform and adjusting their instructions.
- **DEV.** Owns everything inside the code boundary: translating requirements into designs, writing code and unit tests, running builds, managing dependencies, fixing bugs, refactoring, and writing technical documentation. DEV does not self-certify quality or decide when something ships.
- **QA.** Owns verification and release safety: turning acceptance criteria into test plans, writing integration and end-to-end tests, exploratory testing, filing structured bug reports, regression testing, gating releases with a pass/fail recommendation, and red-teaming DEV's work.
- **Maintainer.** Combined DEV and QA for single-agent repos. Carries the inherent conflict of interest between implementation and review — the self-review requirement exists specifically to mitigate this.

## Critical Principle

On anything substantive, the director must never let DEV and QA negotiate directly without being in the loop. The value of two agents is the adversarial check — if they collude or defer to each other, that is one agent with extra steps. Route disagreements through the director, even if it's slower.

## Caveat

This split assumes agents are capable enough to hold these responsibilities across long horizons. If an agent loses context on architecture or misses obvious bugs, the answer is not to reshuffle roles — it is to give them better tools (persistent docs, checklists, test harnesses) and tighter scoped tasks.

## Role Assignment

See `AGENTS.md` Interaction Mode for the full role-assignment rule — default to maintainer when `role-maintainer.md` is present, explicit assignment otherwise, case-insensitive lookup, `role-director.md` is reference-only.

## Available Roles

| File | Role | Type | Focus |
|------|------|------|-------|
| `role-director.md` | Director | Reference (human) | Intent, priorities, irreversible decisions |
| `role-dev.md` | DEV | Agent | Implementation, testing, build process |
| `role-qa.md` | QA | Agent | Review, verification, finding-first reporting |
| `role-maintainer.md` | Maintainer | Agent | Combined implementation and review for single-agent repos |

## Adding Custom Roles

Create a new `docs/role-<name>.md` file. Keep it concise — role docs supplement `AGENTS.md`, they do not replace it. Each file should contain short, actionable rules that the agent follows after role assignment.
