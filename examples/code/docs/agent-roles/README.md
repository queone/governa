# Agent Roles

Role-specific behavior docs that supplement the shared governance contract in `AGENTS.md`.

## How It Works

1. At session start, the agent checks whether a role has been explicitly assigned.
2. If no role is assigned, the agent asks which role to assume before doing substantive work.
3. Role assignment requires an explicit instruction: "act as DEV", "use docs/agent-roles/qa.md", or "you are QA".
4. The role name is case-insensitive: "DEV", "Dev", and "dev" all resolve to `dev.md`.
5. If the requested role file does not exist, the agent says so and continues under shared governance only.

For deterministic role selection, assign the role explicitly at session start rather than waiting for the agent to prompt.

## Available Roles

| File | Role | Focus |
|------|------|-------|
| `dev.md` | DEV | Implementation, testing, build process |
| `qa.md` | QA | Review, verification, finding-first reporting |

## Adding Custom Roles

Create a new `<role>.md` file in this directory. Keep it concise — role docs supplement `AGENTS.md`, they do not replace it. Each file should contain short, actionable rules that the agent follows after role assignment.
