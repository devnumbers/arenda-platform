---
name: ui-walkthrough
description: Live UI acceptance in a visible browser — the agent raises the seeded frontend stack and clicks through a ticket's screens itself, black-box, reporting a screenshot-backed checklist. Use before shipping any frontend ticket with screens or interactions, when the user asks to "протестировать в браузере", "прокликать", "провести живую приёмку", or wants to watch the testing live; also the entry for watching existing e2e specs headed.
---

# Live UI walkthrough («живая приёмка»)

Two modes, one entry point:

- **Live acceptance (основной)** — you drive a visible browser through the ticket's acceptance criteria as a real user, black-box, with a screenshot per check. Mandatory before shipping every frontend ticket that touches screens, forms, flows, or widgets (`apps/frontend/AGENTS.md`, Quality Gates); available on demand anytime.
- **Headed run** — the user watches your *existing* Playwright specs play in visible Chromium windows. Reach it when the ask is about already-written tests rather than exploratory acceptance.

## Browser: per-session Playwright MCP

All walkthrough browsing goes through the `playwright` MCP server (`mcp__playwright__browser_*` tools), wired by the repo workspace config `.zcode/config.json`: each session (any chat, any worktree) gets its own stdio server process and its own visible Chromium — headed by default, in-memory profile (`--isolated`), artifacts under `.playwright-mcp/artifacts/`. The rule is identical in a single session and when several sessions run in parallel — no shared cookies, no shared tabs, no "am I running parallel" branching (decision [#515](https://github.com/devnumbers/arenda-platform/issues/515); the rule for all repo browser work: `docs/agents/parallel-dev.md`). Browser Use (the built-in pane) is not used for walkthroughs. If the `mcp__playwright__*` tools are absent from the session, the workspace config has not loaded — stop and tell the user (Settings → MCP, restart the session); do not substitute another browser mechanism.

Methodology and rules: invoke `web-gui-tester` — it governs the black-box discipline, the action → observation cycle, and the screenshot-evidence rules (its principle 5 defers to the tooling's own rules, which here are Playwright MCP mechanics: act on `browser_snapshot` accessibility refs, never on guessed selectors or raw pixels).

## Mode: live acceptance

Work through five steps. Testing is **black-box** while it runs: interact only with what the page shows, verify from what the page reveals — code edits wait until you declare testing complete.

1. **Raise the stack** — `make frontend-e2e-live-up`. Done when the command prints the frontend URL (default `http://127.0.0.1:3010`) plus the seeded session facts (token, phone digits, email). The stack stays up until `make frontend-e2e-live-down` — between checks the seed keeps data deterministic. In a worktree the ports are the slot's own — use the URL the command actually printed.

2. **Plan from the ticket** — turn the ticket's acceptance criteria into the tester's plan tiers: P0 main flow → P1 interaction feedback → P2 input boundaries → P3 layout/visual. Publish the plan in chat before the first click. Done when every acceptance criterion maps to at least one numbered check.

3. **Open and log in** — `browser_navigate` to the frontend URL; the user watches the session's own Chromium window. Log in through the real login screen: type the seeded phone, take the code from the backend log (`.tmp/e2e-frontend/backend.log`, pattern «Код для входа в Рентли»), type it. The in-memory profile holds no state between browser closes, so every walkthrough logs in fresh; `--storage-state` (a per-session config override, not the committed one) is the optional shortcut when a storage-state file exists. Done when the cabinet's «Мои объекты» is on screen.

4. **Walk the plan** — one action per observation cycle: act, then capture the cheapest proof (`browser_snapshot` for state and locators; `browser_take_screenshot` whenever vision decides the verdict — the PNG lands in the canonical artifacts dir `.playwright-mcp/artifacts/`, name files `<ticket>-NN-<slug>.png`, and view it before judging the check passed; copying a per-ticket bundle to `.scratch/ui-walkthroughs/<ticket>/` is an optional extra, never the primary location). A blocked path gets recorded and skipped, never forced. Done when every numbered check carries a viewed screenshot and a verdict.

5. **Report and gate** — post the checklist to chat: per tier, passed/failed/blocked, each item linked to its screenshot; failed items become Issues. Acceptance gate: the ticket ships only with **P0 + P1 green**; P2/P3 findings report without blocking. Close with `make frontend-e2e-live-down`. Done when the commit message can honestly carry «живая приёмка N/N».

## Mode: headed run

`make frontend-e2e-headed [TESTS="…"]` rebuilds the disposable stack and replays the specs in visible windows (`--headed`); pass a Playwright filter through `TESTS` (e.g. `TESTS="-g платежи"`) to watch one feature, leave it empty for the full suite. The runner launches its own browsers per process — independent of the MCP browser, safe next to a parallel session's walkthrough. For single-test debugging with no rebuild pressure: keep the stack alive with `E2E_KEEP_STACK=1 make frontend-e2e`, then run `npx playwright test <spec> --headed` directly against it from `apps/frontend`.

## Environment facts

- Stack URLs: frontend `http://127.0.0.1:3010`, backend `http://127.0.0.1:8081/healthz`; postgres owns port 5436. Override ports via `E2E_PG_PORT` / `E2E_BACKEND_PORT` / `E2E_FRONTEND_PORT`.
- Seed (tools/e2e/frontend/seed.sql): owner «Иван Иванов» (+7 915 000 0001, e2e@example.com), properties «Квартира на Ленина» (payments of #463) and «Гараж на Садовой» (paymentless — empty states).
- Walkthrough artifacts live under `.playwright-mcp/` — gitignored and disposable; the durable artifact is the chat checklist.
