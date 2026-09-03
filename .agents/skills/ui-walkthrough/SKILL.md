---
name: ui-walkthrough
description: Live UI acceptance in a visible browser — the agent raises the seeded frontend stack and clicks through a ticket's screens itself, black-box, reporting a screenshot-backed checklist. Use before shipping any frontend ticket with screens or interactions, when the user asks to "протестировать в браузере", "прокликать", "провести живую приёмку", or wants to watch the testing live; also the entry for watching existing e2e specs headed.
---

# Live UI walkthrough («живая приёмка»)

Two modes, one entry point:

- **Live acceptance (основной)** — you drive a visible browser through the ticket's acceptance criteria as a real user, black-box, with a screenshot per check. Mandatory before shipping every frontend ticket that touches screens, forms, flows, or widgets (`apps/frontend/AGENTS.md`, Quality Gates); available on demand anytime.
- **Headed run** — the user watches your *existing* Playwright specs play in visible Chromium windows. Reach it when the ask is about already-written tests rather than exploratory acceptance.

## Browser: per-session Playwright MCP

All walkthrough browsing goes through the `playwright` MCP server (`mcp__playwright__browser_*` tools), wired by the repo workspace config `.zcode/config.json`: each session (any chat, any worktree) gets its own stdio server process and its own visible browser — the installed Google Chrome (chrome channel; #517) — headed by default, in-memory profile (`--isolated`), auto-artifacts (snapshots, console logs) under `.playwright-mcp/artifacts/`. The rule is identical in a single session and when several sessions run in parallel — no shared cookies, no shared tabs, no "am I running parallel" branching (decision [#515](https://github.com/devnumbers/arenda-platform/issues/515); the rule for all repo browser work: `docs/agents/parallel-dev.md`; live two-session check: [#517](https://github.com/devnumbers/arenda-platform/issues/517)). Browser Use (the built-in pane) is not used for walkthroughs. If the `mcp__playwright__*` tools are absent from the session, the workspace config has not loaded — stop and tell the user (Settings → MCP, restart the session); do not substitute another browser mechanism.

Methodology and rules: invoke `web-gui-tester` — it governs the black-box discipline, the action → observation cycle, and the screenshot-evidence rules (its principle 5 defers to the tooling's own rules, which here are Playwright MCP mechanics: act on `browser_snapshot` accessibility refs, never on guessed selectors or raw pixels).

## Mode: live acceptance

Work through five steps. Testing is **black-box** while it runs: interact only with what the page shows, verify from what the page reveals — code edits wait until you declare testing complete.

1. **Raise the stack** — `make frontend-e2e-live-up`. Done when the command prints the frontend URL (default `http://127.0.0.1:3010`) plus the seeded session facts (token, phone digits, email). The stack stays up until `make frontend-e2e-live-down` — between checks the seed keeps data deterministic. In a worktree the ports are the slot's own — use the URL the command actually printed.

2. **Plan from the ticket** — turn the ticket's acceptance criteria into the tester's plan tiers: P0 main flow → P1 interaction feedback → P2 input boundaries → P3 layout/visual. Publish the plan in chat before the first click. Done when every acceptance criterion maps to at least one numbered check.

3. **Open and log in** — `browser_navigate` to the frontend URL; the user watches the session's own browser window. Log in through the real login screen: type the seeded phone, take the code from the backend log (`.tmp/e2e-frontend/backend.log`, pattern «Код для входа в Рентли» — parse the 6 digits from the log line's `text` field, not from anywhere else in the line: its timestamp also contains six-digit runs), type it. Re-requesting a code too soon hits the product's resend cooldown (429; the button shows «Отправить новый код MM:SS») — wait out the timer. The in-memory profile holds no state between browser closes, so every walkthrough logs in fresh; `--storage-state` (a per-session config override, not the committed one) is the optional shortcut when a storage-state file exists. Done when the cabinet's «Мои объекты» is on screen.

4. **Walk the plan** — one action per observation cycle: act, then capture the cheapest proof (`browser_snapshot` for state and locators; `browser_take_screenshot` whenever vision decides the verdict — pass the full artifacts path as the filename, `.playwright-mcp/artifacts/<ticket>-NN-<slug>.png`: a bare filename is written to the session cwd, not the artifacts dir (0.0.80 behavior, #517) — and view it before judging the check passed; copying a per-ticket bundle to `.scratch/ui-walkthroughs/<ticket>/` is an optional extra, never the primary location). A blocked path gets recorded and skipped, never forced. Width-varying checks from the P3 tier go through «Adaptive widths» below. Done when every numbered check carries a viewed screenshot and a verdict.

5. **Report and gate** — post the checklist to chat: per tier, passed/failed/blocked, each item linked to its screenshot; failed items become Issues. Acceptance gate: the ticket ships only with **P0 + P1 green**; P2/P3 findings report without blocking. Close with `make frontend-e2e-live-down`. Done when the commit message can honestly carry «живая приёмка N/N».

## Adaptive widths

Two lanes, split by a hard physical limit of desktop Chrome: the OS window is never narrower than ~500 px and never taller than the screen's work area (verified live on 0.0.80; on the dev MacBook that caps the viewport at ~816 px tall).

**Lane A — widths ≥ 500 (tablet, desktop): real window.** Resize the actual OS window via `browser_run_code_unsafe` and CDP `Browser.setWindowBounds` — the window moves and the viewport follows it. The snippet measures `innerWidth`/`innerHeight`, compensates for Chrome's toolbar height and corrects once (heights beyond the screen cap out — trust the returned `viewport`, width is the primary axis):

```js
async (page) => {
  const target = { width: 1440, height: 900 }; // desired viewport in CSS px
  const session = await page.context().newCDPSession(page);
  const { windowId } = await session.send('Browser.getWindowForTarget');
  const set = (width, height) => session.send('Browser.setWindowBounds', { windowId, bounds: { windowState: 'normal', width, height } });
  const inner = () => page.evaluate(() => ({ w: window.innerWidth, h: window.innerHeight }));
  await set(target.width, target.height);
  await page.waitForTimeout(300);
  let v = await inner();
  if (v.w !== target.width || v.h !== target.height) {
    const { bounds } = await session.send('Browser.getWindowForTarget');
    await set(bounds.width + target.width - v.w, bounds.height + target.height - v.h);
    await page.waitForTimeout(300);
    v = await inner();
  }
  return { target, viewport: v };
}
```

**Lane B — widths < 500 (mobile): viewport emulation in a throwaway tab.** A real window physically cannot go there, so emulate: open a fresh tab (`browser_tabs` action `new`), call `browser_resize` with the mobile size, run the checks, close the tab. `browser_resize` calls `page.setViewportSize()`, which flips a tab into permanent viewport emulation — the site re-lays-out but the window keeps its old size, and the tab never returns to window-sized rendering (`Emulation.clearDeviceMetricsOverride` does not help). That is why this lane never touches the walkthrough's main tab: a contaminated main tab keeps failing after the sweep is over. CSS, media queries and screenshots are honest under emulation — mark the check as emulated in the report.

Default sweep: 375 (lane B) / 768 / 1440 (lane A) — mobile / tablet / desktop; exact widths from the ticket or its Figma mockup override the defaults. Trust the snippet's returned `viewport` (lane A) or `browser_resize`'s applied size (lane B), never an assumption, and capture a screenshot per width (`.playwright-mcp/artifacts/<ticket>-NN-w375.png`) — each width carries its own P3 verdict.

## Mode: headed run

`make frontend-e2e-headed [TESTS="…"]` rebuilds the disposable stack and replays the specs in visible windows (`--headed`); pass a Playwright filter through `TESTS` (e.g. `TESTS="-g платежи"`) to watch one feature, leave it empty for the full suite. The runner launches its own browsers per process — independent of the MCP browser, safe next to a parallel session's walkthrough. For single-test debugging with no rebuild pressure: keep the stack alive with `E2E_KEEP_STACK=1 make frontend-e2e`, then run `npx playwright test <spec> --headed` directly against it from `apps/frontend`.

## Environment facts

- Stack URLs: frontend `http://127.0.0.1:3010`, backend `http://127.0.0.1:8081/healthz`; postgres owns port 5436. Override ports via `E2E_PG_PORT` / `E2E_BACKEND_PORT` / `E2E_FRONTEND_PORT`.
- Seed (tools/e2e/frontend/seed.sql): owner «Иван Иванов» (+7 915 000 0001, e2e@example.com), properties «Квартира на Ленина» (payments of #463) and «Гараж на Садовой» (paymentless — empty states).
- Walkthrough artifacts live under `.playwright-mcp/` — gitignored and disposable; the durable artifact is the chat checklist.
