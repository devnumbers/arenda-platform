# MCP servers

Registry of the MCP servers this repo's agents rely on, where each is configured, and what to do when one is unavailable. Read before relying on an `mcp__<name>__*` tool; re-check here when a server is missing from the tool set. The serena/lean-ctx boundary line lives in the root `AGENTS.md` (Repo rules).

When a task depends on MCP tools, check that the server's tools are present in the agent tool set before relying on them. If a server is unavailable, tell the user (they can inspect `/mcp` themselves), state the fallback clearly, and continue with standard repository tools where safe.

## Servers

- `lean-ctx` — compressed reads and noisy-output compression (incl. file trees and shell output), semantic search by meaning, dependency/diff-impact graph, session intelligence. Use it for broad repository exploration, large or generated files, noisy command output, and repeated reads; before editing a file or relying on exact line-level behavior, read the relevant source in full or in precise raw ranges. Fallback when unavailable: native `Read`, `Grep`, `Glob`, `Bash`.
- `context7` — official library and framework documentation. Fallback when unavailable: official docs via `FetchURL` or `WebSearch`.
- `playwright` — browser automation and UI verification; one stdio server per session, so each session gets its own visible Chromium (headed by default, `--isolated` in-memory profile). All repository browser work — `/ui-walkthrough` live acceptance, headed spec runs, browser debugging — goes through it; the rule and the Browser Use ban: `docs/agents/parallel-dev.md`. Fallback when unavailable: stop and tell the user (Settings → MCP, restart the session), then manual inspection and build logs.
- `figma` — Figma design data and image exports. Fallback when unavailable: manual design references.
- `heroui-react` — HeroUI v3 docs for the **legacy widget layer only** (ADR 0050: new UI is built on shadcn/ui over Radix; do not use HeroUI in new code). Fallback when unavailable: official docs at https://v3.heroui.com via `FetchURL` or `WebSearch`. For new components, the source is shadcn/ui docs (https://ui.shadcn.com/docs) and Radix.
- `serena` — code semantics for the whole stack (TS + Go): symbol navigation, references, rename, diagnostics, symbol-level editing. Pinned `serena-agent` 1.7.0 (`uv` + managed Python 3.13); committed project config `.serena/project.yml`, one project at the repo root. If the `mcp__serena__*` tools are unavailable, stop and tell the user (they can inspect `/mcp`) — no silent substitution with text search or other tools.

## Config locations

The harness MCP config is `~/.kimi-code/mcp.json` for Kimi Code and the ZCode user config for ZCode; playwright additionally comes from the committed repo workspace config `.zcode/config.json` (ZCode; auto-connects in every session and worktree).
