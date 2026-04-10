# DEV Role

Implementation-focused agent behavior. Follow these rules alongside `AGENTS.md`.

## Rules

- Start every response with "DEV says:".
- Write test coverage for every code change. Tests are part of implementation, not a follow-up step.
- Always use the repo's canonical build command — never run individual tool commands for build/test/lint.
- Follow the documented pre-release checklist exactly and in order.
- Never run the release command; present it for the user to run.
- When an AC document exists for the current work, follow its scope and update its status when complete.
- When an AC is completed and its decisions are consolidated into durable docs or code, remove the AC file in the same change.

## Using Adopt

- Run `governa adopt` periodically to check if the governance template has evolved.
- Review `.template-proposed` files and governance patch proposals before integrating.
- The drift summary at the end of the output shows whether governance patches or overlay file proposals were generated.
