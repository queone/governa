# governa
Template repo that bootstraps governance into new repositories and helps existing ones adopt it with minimal disruption. Built from:

- a common base contract in `internal/templates/base/`
- a repo-type overlay in `internal/templates/overlays/code/` or `internal/templates/overlays/doc/`
- a deterministic Go CLI that renders templates into target repos

## Frozen

governa is feature-frozen at `v0.160.3` (this release) and superseded by [`govna`](https://github.com/queone/govna), a from-scratch Rust rewrite that has reached functional parity for governa's core commands (`apply`, `drift-scan`, `rm`, `render-canon`) and is now the canonical implementation. This repo takes no new features going forward.

Existing governa-managed consumer repos migrate automatically: running `govna apply` against a repo that still carries `governa/metadata.txt` auto-detects it, carries the legacy repo type and stack forward, and emits a migration-tracking AC under `govna/` — no manual bookkeeping required.

## Why

AI-assisted coding is here to stay. Teams that code alone, teams that work entirely with human contributors, and teams that work with a mix of humans and agents all continue to exist — often in the same repo across different phases. **governa** is not a prerequisite for any of them. If you prefer to code without agents, governa stays out of the way. What governa does is add a little order to the new paradigm: when you choose to bring a coding agent into a repo, the collaboration contract is already explicit, versioned, and reproducible — not reinvented prompt by prompt.

The contract covers what humans and agents agree on before work starts: who is authorized to make which changes, how proposals are reviewed, what governance files mean, and how the template itself evolves. File-based and deterministic; nothing depends on transient session context.

## Roles

governa ships a closed two-role model so agent sessions have a predictable starting point:

- **Operator** — LLM agent role. Owns implementation, tests, doc alignment, and mandatory self-review. Automatic and unannounced; it is the only agent role.
- **Director** — human role. Owns intent, priorities, irreversible decisions (releases, architectural bets, scope), and the meta-loop. Not assignable to an agent.

Full role definitions and the self-review contract live in [`governa/roles.md`](governa/roles.md). The shared `AGENTS.md` contract applies in every case. The reasoning behind the contract structure — particularly the session-entry rule — is in [`governa/operator-contract-rationale.md`](governa/operator-contract-rationale.md).

## Acceptance Criteria

Governa uses Acceptance Criteria (AC) as its central change-control artifact for non-trivial work: a bounded, executable contract that translates Director intent into the change the Operator implements and verifies. Every non-trivial change is AC-first.

An AC records the change summary, authoritative scope, exclusions, acceptance tests, review state, and implementation status. After critique and pre-implementation verification, the Director explicitly confirms that the AC is implementation-ready. Release prep deletes completed ACs after their durable decisions have landed in code or governing documentation.

Trivial changes may proceed without an AC when explicitly authorized; size alone does not make a change trivial. The direct path still requires scoped authorization, appropriate tests, documentation alignment, file-change discipline, and Operator self-review.

Here, “AC” names both the acceptance-criteria document—the change blueprint—and the governed change it tracks from Draft through Package.

## Workflow at a glance

Use the standalone action vocabulary `Draft → Audit → Refine → Implement → Ratify → Package` for an active AC. Draft creates the AC; Audit, Refine, Implement, and Ratify are the four AC phases; Package is post-Ratify release preparation. Accept lowercase forms for the phase actions and `package`, `pack`, or `prep` for Package. Ordinary coding phrases such as `build`, `prepare the build`, and `package the binary` do not advance the workflow.

## Usage

Install the binary:

```bash
go install github.com/queone/governa/cmd/governa@latest
```

### `apply`

One-time governance bootstrap. Run from a target repo or empty directory. Governa is read-only source — templates are embedded in the binary. After apply, all files are consumer-owned — modify freely to fit the repo's needs.

**New repo** (empty directory):

```bash
governa apply
```

Or with flags to skip prompts:

```bash
governa apply -k CODE -n my-service -s "Go"
```

Go, Rust, Terraform, and Swift have first-class CODE overlays. Each emits one
stack-specific canonical `build.sh`; Go, Rust, and Swift also emit stack-specific
development guidance. Other stack values are accepted but produce the generic
CODE scaffold without a build script.

Rust canonical builds run formatting, Clippy, tests, and release compilation
with a temporary Cargo target outside the repository, then remove that target
on success or failure. They install every Cargo-recognized package binary with
all features into `$CARGO_HOME/bin`, or `$HOME/.cargo/bin` when `CARGO_HOME` is
unset. Rust CODE consumers must declare at least one binary target. Existing
binary-name conflicts are reported rather than overwritten.

Swift canonical builds require Swift 6 and a root `Package.swift`, then run
strict formatting, debug compilation, tests, and release compilation with
warnings as errors. Builds use an invocation-owned external scratch directory,
clean it on handled exits, and install executable products into
`${SWIFT_BIN_HOME:-$HOME/.local/bin}` by atomically replacing regular files.
With product names, formatting and tests remain package-wide while debug builds,
release builds, and installation are limited to the selected products.
Library-only packages validate without installation. Swift build, prep, and
release output follows the canonical color and plain-text policy. See
[`governa/code-stacks.md`](governa/code-stacks.md) for stack contracts.

**Existing repo** (governance artifacts found): all template files are written directly. Repo name, type, and stack are inferred from the target directory (directory basename, manifest files). Explicit flags override inference: `-n`, `-k`, `-s`.

```bash
governa apply
```

Run `governa help` for available commands, or `governa apply --help` for apply-specific flags.

### `rm`

Run `governa rm` from an adopted consumer repo root to emit a cleanup AC stub plus a sister diffs file under `governa/`. The emitted AC lists whole-file removals, preserves repo-owned content, and routes hybrid files through Director Review before any deletion occurs.

### `deps`

Run `governa deps` from an adopted Go CODE consumer repo root, or from the governa source repo itself, to report direct Go dependency freshness without modifying `go.mod` or `go.sum`. `deps` is Go-only; other CODE stacks use their native dependency tools. Governa helper libraries (`github.com/queone/governa-*`) are grouped first.

### Self-service updates

To adopt future governa improvements, run `governa drift-scan` from the consumer repo root. The command compares the consumer repo against canon embedded in the installed binary and emits an AC stub under `governa/`. Explicit `--flavor` and `--stack` selectors override flavor and stack inference independently. Before a Rust consumer has `Cargo.toml`, run `governa drift-scan --flavor code --stack Rust`; `--stack` alone does not imply CODE. The consumer Operator iterates on the emitted stub under normal AC discipline; per-file diffs are inspected via `governa render-canon` + standard `diff -ru` (see [`AGENTS.md`](AGENTS.md) `### Drift-Scan Adoption`). See [`governa/drift-scan.md`](governa/drift-scan.md) for the full flow. Manual cherry-picking from governa's `AGENTS.md`, role files, and `CHANGELOG.md` remains a fallback.

## Design
The target repo stays self-contained. The template repo is read-only at bootstrap time and is not imported as a submodule, package, or runtime dependency. The bootstrap tool is Go-based so the template works across macOS, Linux, and Windows without requiring a specific shell.

## Current Stage

governa is early. Releases, commits, and pushes remain Director-controlled; `build.sh` provides validation, release prep, and interactive release orchestration without removing that human gate. There's no branch or PR workflow yet. These are phase choices while the governance contract stabilizes.

Scope is also deliberately narrow. governa aims to be a small, stable collaboration contract — not a full-stack generator, not an opinionated starter kit, not an attempt to be another [gstack](https://github.com/garrytan/gstack). The fewer primitives governa ships, the less there is to drift against.

The primary validation surface so far has been CLI-type coding agents — [Claude Code](https://github.com/anthropics/claude-code) and [Codex CLI](https://github.com/openai/codex). The contract is file-based and agent-agnostic in principle — desktop clients and IDE-integrated agents can read the same files — but their session and context-loading models differ, so expect rougher edges there until the patterns are exercised.

## Self-Hosting Status
This repo is itself governed as a `CODE` repo and carries the core artifacts at the root:

- [`AGENTS.md`](AGENTS.md)
- [`arch.md`](arch.md)
- [`plan.md`](plan.md)
- [`CHANGELOG.md`](CHANGELOG.md)
- [`governa/README.md`](governa/README.md)
- [`governa/roles.md`](governa/roles.md)

The `governa` CLI may print a quiet stderr notice when a newer governa release is available. Set `GOVERNA_NO_UPDATE_CHECK=1` to suppress that best-effort check.

## External Library

governa keeps generated repositories self-contained and retains one direct external Go dependency in the source CLI:

- [`governa-color`](https://github.com/queone/governa-color) — ANSI terminal color and usage-formatting helpers for the internal CLI surface.

Its independent versioning policy lives in [`governa/library-policy.md`](governa/library-policy.md). Build, release-prep, and release orchestration remain self-contained in generated `build.sh` files rather than external libraries.

## Rendered Examples

Run `governa render-canon --flavor code <dir>` (and `--flavor doc <dir>`) to render flavor-specific canon into a target directory for inspection or testing. CODE rendering infers the stack from the current directory; use `-s, --stack <name>` to override it. Go rendering reads the module path from `go.mod` unless `-m, --module-path <path>` is supplied. See `governa render-canon -h` for full usage.

See [`governa/governance-model.md`](governa/governance-model.md).
