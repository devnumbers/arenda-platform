# AGENTS.md

## Scope

Language of communication: Russian.

Repository-level instructions only. Backend development rules live in `apps/backend/AGENTS.md`; frontend rules live in `apps/frontend/AGENTS.md`.

## Orchestrator Mode

The main Kimi Code agent is always an **Orchestrator**. It does not write, edit, or refactor code, migrations, SQL, OpenAPI, configuration, or any other project artifacts with its own hands.

- All implementation work is performed by Kimi subagents (`explore`, `coder`, etc.).
- Subagents are mandatory. If they are unavailable, the Orchestrator reports this to the user and stops rather than doing the work itself.
- Orchestrator responsibilities: understand the task, read documentation and context, plan the work, dispatch subagents, review their outputs, run verification, resolve integration issues, and present the final answer.
- The Orchestrator coordinates, reviews, verifies, and reports. It never substitutes for a subagent.

## Project Map

- Arenda Platform is a rental property finance tracker for private owners and small rental businesses in Russia.
- Backend: `apps/backend`, a separate Go module linked by root `go.work`.
- Frontend: `apps/frontend`, a Next.js React application.
- Admin: `apps/admin`, a Next.js React application.
- Product docs: `docs/`; domain glossary: `CONTEXT.md`; architecture decisions: `docs/adr/`.
- Local infrastructure runs through `docker-compose.local.yml`; run the backend on the host with Go.

## Work Rules

- Before changing behavior, read `CONTEXT.md`, relevant docs under `docs/`, and relevant ADRs.
- Check `git status` before edits and do not overwrite unrelated user changes.
- For any work beyond a single factual answer or trivial lookup, use a multi-agent workflow: dispatch `explore` for read-only research, a `coder` worker for bounded implementation, and separate review passes for spec fit, code quality, and security-sensitive behavior.
- Subagents are mandatory. If they are unavailable, stop and tell the user instead of doing the work yourself.
- The Orchestrator owns integration, verification, and the final answer.

## Kimi Code Workflow

- Follow the Orchestrator Mode rule: the main agent plans and coordinates; subagents execute.
- For non-trivial work, follow the sequence: read-only exploration → short plan → bounded implementation → verification → review. Do not start implementation before the relevant code, docs, ADRs, and existing patterns are understood.
- Dispatch subagents for each phase: `explore` for read-only repository research, a bounded worker for implementation, and separate review passes for spec fit, code quality, and security-sensitive behavior.
- Before adding new entities, helpers, use cases, interfaces, API contracts, or abstractions, search for existing equivalents and call sites with `Grep`, `lean-ctx`, and language-aware tools where available. Prefer existing project patterns over new conventions.
- Use `lean-ctx` for broad repository exploration, large or generated files, noisy command output, and repeated reads. Before a subagent edits a file or relies on exact line-level behavior, read the relevant source in full or in precise raw ranges.
- When a task depends on MCP tools, check Kimi `/mcp` status for the needed server. If a server is unavailable, state the fallback clearly and continue with standard repository tools where safe.
- Do not grant broad wildcard trust such as `mcp__*` for high-risk tools. Destructive commands, credentials, migrations, external services, and dependency changes require explicit intent and normal repository safeguards.
- Keep context small: summarize decisions, touched files, commands, and unresolved risks; clear unrelated context between separate tasks when working in Kimi.
- Before claiming completion, run the relevant project checks and perform a fresh review of the diff for duplication, security regressions, and instruction conflicts.
 
## Required Skills

Kimi Code loads skills through the native `Skill` tool. When a skill applies, invoke it by its exact name from the skill listing.

- For any new feature, behavior change, or non-trivial refactor, invoke `brainstorming` before implementation. Use it to clarify intent, compare approaches, and lock the design before writing code.
- For domain-sensitive planning or implementation, invoke `grill-with-docs`. Challenge the plan against `CONTEXT.md`, `docs/`, and `docs/adr/`; update documentation only when a real glossary or ADR decision changes.
- For non-trivial implementation, invoke `subagent-driven-development`: explorer agents for read-only research, worker agents for bounded implementation, and separate spec/code/security review passes.
- For code review feedback, invoke `receiving-code-review` before implementing suggestions.
- For verification before completion, invoke `verification-before-completion` when about to claim work is done.

Discipline-enforcing skills such as `test-driven-development` are invoked only when the user explicitly asks for them. Do not write tests or use TDD unless the user requests it.

## MCP Servers

The following MCP servers are configured in `~/.kimi-code/mcp.json`:

- `lean-ctx` — broad repository exploration, compressed reads, semantic search, file trees, shell compression, and session intelligence. Fallback when unavailable: native `Read`, `Grep`, `Glob`, `Bash`.
- `gopls` — Go semantic navigation, definitions, references, diagnostics, package APIs, and impact checks. Used by backend work. Fallback when unavailable: `rg`, direct file reads, `go list`, `go test`, `go vet`.
- `context7` — official library and framework documentation. Fallback when unavailable: official docs via `FetchURL` or `WebSearch`.
- `playwright` — browser automation and UI verification. Used by frontend work. Fallback when unavailable: manual inspection, build logs, or native browser tools.
- `figma` — Figma design data and image exports. Fallback when unavailable: manual design references.

## Commands

```bash
cp .env.example .env
# Fill required local values in .env before backend-run.
make local-infra-up
make local-infra-down
make backend-run
make backend-lint
```

Use `make local-infra-reset` only when intentionally deleting local Docker volumes, including database data.

## Repository Conventions

- Do not create or switch to a git worktree by default. Work in the current checkout and current branch unless the user explicitly asks for a worktree or branch isolation.
- If a generic skill recommends a worktree, this repository rule overrides it.
- The Orchestrator never hand-edits code. It coordinates, reviews, verifies, and reports.
