---
name: ui-walkthrough
description: Live UI acceptance in a visible browser — the agent raises the seeded frontend stack and clicks through a ticket's screens itself, black-box, verifying server truth (API responses, backend log, database rows) alongside the screens, reporting a screenshot-backed checklist. Use before shipping any frontend ticket with screens or interactions, when the user asks to "протестировать в браузере", "прокликать", "провести живую приёмку", "проверить, что в базе создалось", or wants to watch the testing live; also the entry for watching existing e2e specs headed. Audit tickets of map #862 additionally gate every page in scope through the page acceptance checklist (DESIGN.md §14).
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

2. **Plan from the ticket** — turn the ticket's acceptance criteria into the tester's plan tiers: P0 main flow → P1 interaction feedback → P2 input boundaries → P3 layout/visual. Publish the plan in chat before the first click. Done when every acceptance criterion maps to at least one numbered check. Audit tickets of map #862 plan from the «Page acceptance checklist» below — its four sections form the tier skeleton, ticket-specific criteria ride on top.

3. **Open and log in** — `browser_navigate` to the frontend URL; the user watches the session's own browser window. Log in through the real login screen: type the seeded phone, take the code from the backend log (`.tmp/e2e-frontend/backend.log`, pattern «Код для входа в Рентли» — parse the 6 digits from the log line's `text` field, not from anywhere else in the line: its timestamp also contains six-digit runs), type it. Re-requesting a code too soon hits the product's resend cooldown (429; the button shows «Отправить новый код MM:SS») — wait out the timer. The in-memory profile holds no state between browser closes, so every walkthrough logs in fresh; `--storage-state` (a per-session config override, not the committed one) is the optional shortcut when a storage-state file exists. Done when the cabinet's «Мои объекты» is on screen.

4. **Walk the plan** — one action per observation cycle: act, then capture the cheapest proof (`browser_snapshot` for state and locators; `browser_take_screenshot` whenever vision decides the verdict — pass the full artifacts path as the filename, `.playwright-mcp/artifacts/<ticket>-NN-<slug>.png`: a bare filename is written to the session cwd, not the artifacts dir (0.0.80 behavior, #517) — and view it before judging the check passed; copying a per-ticket bundle to `.scratch/ui-walkthroughs/<ticket>/` is an optional extra, never the primary location). A blocked path gets recorded and skipped, never forced. Width-varying checks from the P3 tier go through «Adaptive widths» below. Every screen that renders query data additionally carries the loading-stability checks of «Loading stability» below, and every page of an audit ticket (map #862) carries the «Page acceptance checklist» below across the full adaptive-width sweep (default 375 / 768 / 1440, on audit tickets add 1100; ticket/mockup widths override). Every mutation check (create, edit, delete, archive, any state flip) additionally carries the server-truth layers of «Server truth» below — the UI verdict alone does not close a mutation. Done when every numbered check carries a viewed screenshot, its server-truth verdicts, and a verdict.

5. **Report and gate** — post the checklist to chat: per tier, passed/failed/blocked, each item linked to its screenshot; failed items become Issues. Acceptance gate: the ticket ships only with **P0 + P1 green**; P2/P3 findings report without blocking. Loading-stability failures are **P1** (blocking) — a page that jumps when data arrives does not ship. Server truth: a database mismatch is **P0** (blocking); an unexpected API status, console error, or backend-log error is **P1**. Close with `make frontend-e2e-live-down` — only after every DB assertion has run (the teardown wipes the volume). Done when the commit message can honestly carry «живая приёмка N/N».

## Loading stability (скелетоны и layout shift)

Data surfaces are judged not only on their loaded state but on the **transition** into it. A walkthrough check group (P1, blocking) for every screen with query data — and for every navigation leading to it:

1. **No blank frame**: navigating to the screen shows its skeleton/loading state immediately — not a white/empty flash before the skeleton mounts.
2. **Skeleton parity**: the skeleton mirrors the final layout — same blocks, heights, spacing; when data arrives it occupies the skeleton's place instead of pushing content around. Judge by a screenshot pair (loading frame vs loaded frame), not by eyeballing one frame.
3. **No naked data surface**: every query has a loading state at all (CODING_STANDARDS «Naked data surface») — nothing pops in unannounced.
4. **In-place updates stay in place**: typing in search, switching filter chips or period keeps previous results on screen (keepPreviousData) — a skeleton flash per keystroke or per chip is a failure.

Mechanics: local stacks answer in tens of milliseconds — too fast to judge a skeleton honestly, so slow the API down and meter the shifts. Pitfalls verified live on #605 (2026-09-10):

- **Never busy-wait inside a `page.route` handler** — it blocks the runner's event loop, so goto/timers/screenshots all stall until the wait is over and the "loading" frame is shot after data has already arrived. Delay with `page.waitForTimeout` inside the handler (there is no `setTimeout` in the JS sandbox, but Playwright's own timer works).
- **Neutralize the service worker first** — the PWA `sw.js` intercepts navigations and its controlled fetches bypass `page.route` («route.continue: already handled»), and an already-registered SW survives re-armed routes on a reused tab. The arm snippet below fulfills `sw.js` **and unregisters existing registrations** — run it on every freeze, not once per session; the next hard `goto` loads uncontrolled.
- **Never `unroute` mid-flight** — requests parked in a removed handler hang forever. Release by time (release-at deadline inside the handler) and unroute only after the loaded screenshot.
- **Release deadline ≥12 s, and shoot the loading frame the moment `goto` returns** — `browser_navigate` comes back on the load event, before the delayed API resolves, and the MCP round-trip to the screenshot tool eats more seconds: with the old 4 s deadline the first screenshot landed on the loaded frame, twice now (#868, #873).
- **Keep the `**/sw.js` arm route active until ALL verification on the stand is done** — unroute the API delay when the transition is shot, but re-arming `sw.js` is mandatory before every later navigation: a session where the neutralization was dropped lets the real service worker re-register and serve stale chunks on the next hard reload, so a shipped fix reads as «not applied» (hit on #873; cost a rebuild cycle). If a fix seems absent: `navigator.serviceWorker.getRegistrations()` → unregister + `caches.keys()` → delete each, then re-judge before touching the build.
- **SPA navigation can be legitimately instant** (react-query `staleTime`), so shoot the loading frame on a cold hard `goto`, not an in-app click.

```js
// Arm a freeze: fulfill sw.js AND unregister existing registrations — a
// controlled SW's fetches bypass page.route and survive re-armed routes on
// a reused tab. Then delay every API response (~12 s release deadline; the
// load event returns before it, MCP round-trips eat the rest):
await page.route('**/sw.js', (r) => r.fulfill({ body: '', contentType: 'application/javascript' }));
await page.evaluate(async () => {
  const regs = await navigator.serviceWorker.getRegistrations();
  await Promise.all(regs.map((r) => r.unregister()));
});
const releaseAt = Date.now() + 12000;
await page.route('**/api/**', async (route) => {
  const wait = releaseAt - Date.now();
  if (wait > 0) await page.waitForTimeout(wait);
  await route.continue();
});
// … goto (hard load), screenshot the loading frame at ~700 ms, waitForTimeout past
// the deadline, screenshot loaded, then: await page.unrouteAll({ behavior: 'ignoreErrors' });

// Shift meter (install before the transition, read after; buffered replays earlier shifts).
// For a cold hard goto install it via page.addInitScript — an evaluate before
// goto dies with the document, and the metric reads undefined on the new one:
window.__cls = 0;
new PerformanceObserver((list) => {
  for (const e of list.getEntries()) if (!e.hadRecentInput) window.__cls += e.value;
}).observe({ type: 'layout-shift', buffered: true });
```

Caveat: on a fast local stack a swap can land inside the platform's 500 ms `hadRecentInput` window after the click and be filtered out of the score — the screenshot pair (loading vs loaded) is the primary evidence, the CLS number is corroborating. Record per check: pass / fail with both screenshots; a fail is a P1 finding and blocks the ticket (step 5).

## Page acceptance checklist (гейт аудит-тикетов карты #862)

The canonical text is `DESIGN.md` §14 «Чеклист приёмки страницы» — four sections (каркас / стабильность / данные / мобильный UX) with items anchored to §1–§13; this section is the operational wrapper. It gates every audit ticket of map #862 («Фронт в порядке»): each page in the ticket's scope walks the checklist, and the ticket fixes its P0/P1 findings on the spot («карта несёт исполнение»). Other frontend tickets: the P3 tier plus «Loading stability» already cover most of it — pulling in the full checklist is welcome, not mandatory.

Run it **per page**: Каркас gets a verdict on each sweep width (Adaptive widths below; default sweep 375 / 768 / 1440, on audit tickets add 1100; the Figma mockup widths override), Стабильность and Данные once per page, Мобильный UX on the mobile width:

- **Каркас** — judged on each width against the mockup of its tier: hub anatomy (`mobileWings` + `HubTitle` + `HubCollapseAnchor` compact + search pill, where the mockup has one) vs subscreen anatomy (`SubScreenShell`); column cap 560/24; on desktop nothing permanently overlaps the sidebar or pills — the PC chrome is permanent (an open fullscreen surface keeps it alive: its header draws the wings, sidebar and pills stay visible and clickable above it; a bottom bar's sheet sits in the 560 column and never mutes the pills — решение 25.09, правка #561); bottom chrome (TabBar / StickyBottomBar / safe-area) conflict-free — a mounted bar mutes the TabBar on mobile/tablet (§14).
- **Стабильность** — one verdict per page; the evidence comes from the «Loading stability» group above (screenshot pair per transition): header never jumps, content never shifts on data arrival, skeleton parity (#604) including composition parity for pinned feeds, and the segment's loading strategy matches DESIGN.md §7 — `loading.tsx` with the page archetype where the route renders the header, in-component skeleton where the screen assembles the header itself.
- **Данные** — a cold entry fires each request exactly once (the network sweep of «Server truth» layer 1 doubles as the evidence); hub warm-up intact: a warmed hub section opens on data with zero blocking API requests (#626) — prefetch and screen must read the same query-keys; empty/error states per canon (ErrorCard / EmptyState / section empties). A bare request in the sweep can belong to the shell's hub warm-up, not the screen: `hub-prefetch-provider` prefetches the properties list on every cabinet page (#626) and react-query dedupes it with any screen subscription — check the warm registry before calling a request «лишним» (false positive on #873; the screen-level fix is an `enabled` gate on invisible data, the request itself legitimately stays).
- **Мобильный UX** — judged on the mobile width: tap targets ≥44px (visual may be smaller, the tap zone may not), safe-area respected top and bottom, sheets on canonical timings (§8 curve; MoreSheet fade 250 / slide 400 / close 300), native pull-to-refresh works on normally scrolling pages (a full-height screen with inner scroll deliberately keeps the gesture — pattern contract, not a finding), no hover-only affordances, scroll and sticky behavior intact (sticky day pills latch and release; body is `overflow-x: clip`, never hidden).

Scope guard (charter decision): server prefetch has landed (#887; canon DESIGN.md §15) — the «Данные» item gates the first frame with data; transition animations (#867) were rejected by the owner on 25.09 — the through-pass rollout is cancelled, and a page does not fail for their absence; the loading-strategy item checks the current §7 canon.

Findings keep the standard tiers: **P0** — the page is unusable or shows wrong data; **P1** — a §14 item broken in a user-visible way (frame off the mockup, jitter, broken sticky/safe-area, non-canonical states). P0/P1 are fixed in the audit ticket before it closes; P2/P3 report without blocking. The ticket-level gate stays P0 + P1 green (step 5).

Reconcile by code before closing, not by pages visited (found on #872): the live sweep only shows what the seeded data loaded — a generic skeleton behind an empty state or a cached query never surfaces. Before closing an audit ticket, grep the subtree for the known-bad loading shapes (`Skeleton className="h-14 w-full"` generic rows and friends) and reconcile every hit against the findings list; a hit with no finding is a missed one. Grep by name **and** by geometry: a skeleton component you just fixed can have near-twins in other widgets under the same name (ObjectRowsSkeleton lived in tasks and contacts — the second copy surfaced only in code review, #873), so also sweep `apps/frontend` for the touched component's name and its old class signatures before the commit — the reconcile runs at fix time, not as a code-review afterthought.

## Server truth (серверная правда)

A green screenshot is the UI's claim; server truth is what the server actually recorded. The black-box discipline governs the driving — the server-truth layers read the aftermath after the UI step is judged. Three layers over every mutation check, gated in step 5:

1. **API layer (P1)** — after each mutation and again at scenario end, sweep `browser_network_requests`: a `/api/*` status outside the ticket's intent (an unexpected 4xx/5xx) is a finding; `browser_console_messages` rides the same sweep. The login screen's 429 resend cooldown (step 3) is expected noise. Response bodies double as the evidence source: capture the `id` of each entity the flow created — the DB layer anchors on it.
2. **Log layer (P1)** — at scenario end, grep the backend JSON log (`.tmp/e2e-frontend/backend.log` of the walkthrough's checkout) for `"level":"error"` and read any `"level":"warn"`. Successful requests are not logged, so every hit gets a verdict; the 429-cooldown warn is expected.
3. **Database layer (P0)** — each mutation check carries 1–3 SQL assertions stating what the row should look like, run right after the UI claims success. A success that did not persist, persisted wrong, or left duplicates is a failed check.

DB mechanics:

- Tool: `mcp__postgres__pg_execute_query` — SELECT-only by construction (`operation`: `select`/`count`/`exists`; a non-SELECT query is refused, and every mutating tool is unregistered by the allowlist `.zcode/postgres-mcp.tools.json`). The connection string is passed **per call** — `postgresql://arenda:arenda@localhost:<PG port>/arenda?sslmode=disable` — with the port live-up printed (default 5436; slots override via `E2E_PG_PORT`). DSNs never live in configs.
- Fallback: the `mcp__postgres__*` tools absent from the session → `docker exec "${E2E_COMPOSE_PROJECT:-arenda-e2e}-postgres-1" psql -U arenda -d arenda -tAc "<sql>"` and continue.
- Anchors: assert by ids and names. Phone and email are encrypted at rest (`users.phone` holds a deterministic ciphertext), so the seeded owner anchors as `owner_id = '11111111-1111-4111-8111-111111111111'` — the fixed UUID from `tools/e2e/frontend/seed.sql`.
- Money: assert integer kopecks — the UI's «1 200,00 ₽» is `amount_kopecks = 120000`.
- Ticket-specific assertions are written from the schema sources (migrations, sqlc queries) in the shape of these common invariants:

```sql
-- created and owned: the new row exists, in the expected state
SELECT count(*) FROM properties
WHERE owner_id = '11111111-1111-4111-8111-111111111111'
  AND name = '<имя из сценария>' AND status = 'active';   -- expect 1

-- exact money: the formatted «1 200,00 ₽» is stored as integer kopecks
SELECT amount_kopecks FROM operations WHERE id = '<id>';   -- expect 120000

-- soft delete: the row survives, the flag flipped
SELECT deleted_at IS NOT NULL FROM operations WHERE id = '<id>';  -- expect t
```

The tool output (or pasted psql row) is the assertion's evidence, attached to the check like a screenshot.

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

**Lane B — widths < 500 (mobile): viewport emulation in a throwaway tab.** A real window physically cannot go there, so emulate: open a fresh tab (`browser_tabs` action `new`), call `browser_resize` with the mobile size, run the checks, close the tab. `browser_resize` calls `page.setViewportSize()`, which flips a tab into permanent viewport emulation — the site re-lays-out but the window keeps its old size, and the tab never returns to window-sized rendering (`Emulation.clearDeviceMetricsOverride` does not help). That is why this lane never touches the walkthrough's main tab: a contaminated main tab keeps failing after the sweep is over. CSS, media queries and screenshots are honest under emulation — mark the check as emulated in the report. `browser_navigate`, `browser_run_code_unsafe` and every other driving tool ride the **current** tab: after the Lane B sweep, select the main tab back (`browser_tabs` action `select`) before resuming desktop work — otherwise desktop checks silently shoot inside the 375 emulation (hit on #872; benign only because the mobile evidence was valid too).

Default sweep: 375 (lane B) / 768 / 1440 (lane A) — mobile / tablet / desktop; on audit tickets add **1100** (lane A): arbitrary breakpoints in code (`min-[1200px]`, found on #870) branch inside the desktop tier, so a layout hack can sleep at 1440 and fire at 1100 — the three Figma tiers alone do not cover every branch. Exact widths from the ticket or its Figma mockup override the defaults. Judge widths by `window.innerWidth` (the snippet's returned `viewport`, lane A) or `browser_resize`'s applied size (lane B), never by the OS window size you set — docked DevTools keeps the viewport hundreds of px narrower than the window (#870: "1440 window" measured 1150 viewport). Capture a screenshot per width (`.playwright-mcp/artifacts/<ticket>-NN-w375.png`) — each width carries its own P3 verdict.

## Mode: headed run

`make frontend-e2e-headed [TESTS="…"]` rebuilds the disposable stack and replays the specs in visible windows (`--headed`); pass a Playwright filter through `TESTS` (e.g. `TESTS="-g платежи"`) to watch one feature, leave it empty for the full suite. The runner launches its own browsers per process — independent of the MCP browser, safe next to a parallel session's walkthrough. For single-test debugging with no rebuild pressure: keep the stack alive with `E2E_KEEP_STACK=1 make frontend-e2e`, then run `npx playwright test <spec> --headed` directly against it from `apps/frontend`.

## Environment facts

- Stack URLs: frontend `http://127.0.0.1:3010`, backend `http://127.0.0.1:8081/healthz`; postgres owns port 5436. Override ports via `E2E_PG_PORT` / `E2E_BACKEND_PORT` / `E2E_FRONTEND_PORT`.
- Seed (tools/e2e/frontend/seed.sql): owner «Иван Иванов» (+7 915 000 0001, e2e@example.com), properties «Квартира на Ленина» (payments of #463), «Гараж на Садовой» (paymentless — empty states) and the studio «Студия на Полевой» (55-overdue rule, #466). **The seed has no rentals** (#872): for rental-subtree walkthroughs create them through the wizard (`/properties/<id>/rentals/new` — start date «today» is the button with `aria-current="date"`; on an empty contact book the picker route is hidden by design — drive the `?pick=rental` contact branch). Completion wizard gotchas: the step button appears only after the field fills (tap the «0 ₽» chip), and the success screen's «Хорошо» runs `history.back()` — after a full page load onto another object's `/new` it lands there by design.
- Walkthrough artifacts live under `.playwright-mcp/` — gitignored and disposable; the durable artifact is the chat checklist.
