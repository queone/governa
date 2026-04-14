# Roles

This directory defines the delivery model for this repo. Each role file is a behavioral contract that an agent reads and follows alongside `AGENTS.md`.

**Instruction traceability:** `AGENTS.md` is the shared repo contract loaded every session. Each file in this directory adds role-specific behavior for the assigned role. When both apply, follow `AGENTS.md` plus the assigned role file together. If a role file conflicts with `AGENTS.md`, `AGENTS.md` wins unless the repo intentionally says otherwise.

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

1. At session start, the agent checks whether a role has been explicitly assigned.
2. If no role is assigned and `maintainer.md` exists, the agent defaults to maintainer and announces it (e.g., "Operating as maintainer (default).").
3. If no role is assigned and no `maintainer.md` exists, the agent asks which role to assume.
4. Role assignment requires an explicit instruction: "act as DEV", "use docs/roles/qa.md", or "you are QA". Assignment overrides the default.
5. The role name is case-insensitive: "DEV", "Dev", and "dev" all resolve to `dev.md`.
6. `director.md` is a reference document, not an assignable role. If requested, the agent must decline and ask for a valid agent role.
7. If the requested role file does not exist, the agent says so and continues under shared governance only.

## Available Roles

| File | Role | Type | Focus |
|------|------|------|-------|
| `director.md` | Director | Reference (human) | Intent, priorities, irreversible decisions |
| `dev.md` | DEV | Agent | Implementation, testing, build process |
| `qa.md` | QA | Agent | Review, verification, finding-first reporting |
| `maintainer.md` | Maintainer | Agent | Combined implementation and review for single-agent repos |

## Adding Custom Roles

Create a new `<role>.md` file in this directory. Keep it concise — role docs supplement `AGENTS.md`, they do not replace it. Each file should contain short, actionable rules that the agent follows after role assignment.
