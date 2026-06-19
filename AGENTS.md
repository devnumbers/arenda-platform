# AGENTS.md

## Scope

Repository-level instructions only. Backend development rules live in `apps/backend/AGENTS.md`.

## Project Map

- Arenda Platform is a rental property finance tracker for private owners and small rental businesses in Russia.
- Backend: `apps/backend`, a separate Go module linked by root `go.work`.
- Product docs: `docs/`; domain glossary: `CONTEXT.md`; architecture decisions: `docs/adr/`.
- Local infrastructure runs through `docker-compose.local.yml`; run the backend on the host with Go.

## Work Rules

- Before changing behavior, read `CONTEXT.md`, relevant docs under `docs/`, and relevant ADRs.
- Check `git status` before edits and do not overwrite unrelated user changes.
- Every feature, bug fix, or meaningful change must be recorded in `CHANGELOG.md` under the current date before merging to the main branch. Write briefly and in product-friendly language, without technical details.
- For non-trivial work, use a multi-agent workflow: explorer for read-only research, worker for bounded implementation, and separate spec/code/security/test review passes. If subagents are unavailable, perform those roles sequentially and state the fallback.
- The orchestrator owns integration, verification, and the final answer.

## Required Skills And Workflow

- For any new feature, behavior change, or non-trivial refactor, invoke `superpowers:brainstorming` before implementation. Use it to clarify intent, compare approaches, and lock the design before writing code.
- For domain-sensitive planning or implementation, invoke `grill-with-docs`. Challenge the plan against `CONTEXT.md`, `docs/`, and `docs/adr/`; update documentation only when a real glossary or ADR decision changes.
- For non-trivial implementation, use `superpowers:subagent-driven-development`: explorer agents for read-only research, worker agents for bounded implementation, and separate spec/code/security/test review passes.
- Do not create or switch to a git worktree by default. Work in the current checkout and current branch unless the user explicitly asks for a worktree or branch isolation.
- If a generic skill recommends a worktree, this repository rule overrides it.
- The orchestrator does not hand-edit code when the user has requested subagents; it coordinates, reviews, verifies, and reports.

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
