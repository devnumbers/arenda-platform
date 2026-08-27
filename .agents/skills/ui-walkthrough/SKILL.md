---
name: ui-walkthrough
description: Live UI acceptance in a visible browser — the agent raises the seeded frontend stack and clicks through a ticket's screens itself, black-box, reporting a screenshot-backed checklist. Use before shipping any frontend ticket with screens or interactions, when the user asks to "протестировать в браузере", "прокликать", "провести живую приёмку", or wants to watch the testing live; also the entry for watching existing e2e specs headed.
---

# Live UI walkthrough («живая приёмка»)

Two modes, one entry point:

- **Live acceptance (основной)** — you drive a visible browser through the ticket's acceptance criteria as a real user, black-box, with a screenshot per check. Mandatory before shipping every frontend ticket that touches screens, forms, flows, or widgets (`apps/frontend/AGENTS.md`, Quality Gates); available on demand anytime.
- **Headed run** — the user watches your *existing* Playwright specs play in visible Chromium windows. Reach it when the ask is about already-written tests rather than exploratory acceptance.

## Mode: live acceptance

Work through five steps. Testing is **black-box** while it runs: interact only with what the page shows, verify from what the page reveals — code edits wait until you declare testing complete (methodology and rules: invoke `web-gui-tester`; browser mechanics: `control-browser` governs how you open tabs, aim locators, and emit screenshots).

1. **Raise the stack** — `make frontend-e2e-live-up`. Done when the command prints the frontend URL (default `http://127.0.0.1:3010`) plus the seeded session facts (token, phone digits, email). The stack stays up until `make frontend-e2e-live-down` — between checks the seed keeps data deterministic.

2. **Plan from the ticket** — turn the ticket's acceptance criteria into the tester's plan tiers: P0 main flow → P1 interaction feedback → P2 input boundaries → P3 layout/visual. Publish the plan in chat before the first click. Done when every acceptance criterion maps to at least one numbered check.

3. **Open and log in** — open the frontend URL in the visible browser pane; the pane activates on every action so the user watches each step. Log in by seeding first (inject the seeded session cookie if the browser tooling supports it), falling back to the real login screen: type the seeded phone, take the code from the backend log (`.tmp/e2e-frontend/backend.log`, pattern «Код для входа в Рентли»). Done when the cabinet's «Мои объекты» is on screen.

4. **Walk the plan** — one action per observation cycle: act, then capture the cheapest proof (targeted state read, fresh DOM snapshot, and a screenshot whenever vision decides the verdict). Save every verdict-bearing PNG under `.scratch/ui-walkthroughs/<ticket>/` and view it before judging the check passed. A blocked path gets recorded and skipped, never forced. Done when every numbered check carries a viewed screenshot and a verdict.

5. **Report and gate** — post the checklist to chat: per tier, passed/failed/blocked, each item linked to its screenshot; failed items become Issues. Acceptance gate: the ticket ships only with **P0 + P1 green**; P2/P3 findings report without blocking. Close with `make frontend-e2e-live-down`. Done when the commit message can honestly carry «живая приёмка N/N».

## Mode: headed run

`make frontend-e2e-headed [TESTS="…"]` rebuilds the disposable stack and replays the specs in visible windows (`--headed`); pass a Playwright filter through `TESTS` (e.g. `TESTS="-g платежи"`) to watch one feature, leave it empty for the full suite. For single-test debugging with no rebuild pressure: keep the stack alive with `E2E_KEEP_STACK=1 make frontend-e2e`, then run `npx playwright test <spec> --headed` directly against it from `apps/frontend`.

## Environment facts

- Stack URLs: frontend `http://127.0.0.1:3010`, backend `http://127.0.0.1:8081/healthz`; postgres owns port 5436. Override ports via `E2E_PG_PORT` / `E2E_BACKEND_PORT` / `E2E_FRONTEND_PORT`.
- Seed (tools/e2e/frontend/seed.sql): owner «Иван Иванов» (+7 915 000 0001, e2e@example.com), properties «Квартира на Ленина» (payments of #463) and «Гараж на Садовой» (paymentless — empty states).
- Screenshots dir is `.scratch/…` — never committed; the durable artifact is the chat checklist.
