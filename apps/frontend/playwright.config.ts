import { defineConfig, devices } from '@playwright/test';

// The whole run lives in UTC so every clock in the suite agrees — the seed
// (CURRENT_DATE in the Etc/UTC postgres container), the browser and the
// node workers of the specs; otherwise a nightly run between 00:00 and 03:00
// MSK saw «сегодня» render as «Вчера» (#796). TZ is set before workers fork,
// so they inherit it; the invariant is guarded live by e2e/utc-clock.spec.ts.
process.env.TZ = 'UTC';

// Frontend e2e suite (ticket #456, spec #453). The whole stack — postgres
// (compose project arenda-e2e), the backend (fake email sender, fake payment
// provider, migrations on boot) and a production standalone build of this
// app — is brought up and torn down by tools/e2e/frontend/run-frontend-e2e.sh
// (`make frontend-e2e`). Running `npx playwright test` directly only works
// against that orchestrator's env (E2E_* variables below).
//
// New screens ship with e2e specs and screenshot artifacts for Figma
// comparison — see docs/testing-strategy.md, "Экранные e2e (Playwright)".
export default defineConfig({
  testDir: './e2e',
  // Только спеки: дефолтный testMatch цепляет и *.test.ts, а vitest-тесты
  // хелперов (e2e/realtime-bridge.test.ts) живут рядом и коллекционером
  // playwright импортироваться не должны.
  testMatch: '**/*.spec.ts',
  // One browser, sequential: the suite shares a single seeded backend, and
  // UI-login scenarios consume per-send login codes from its log.
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: [['html', { outputFolder: 'playwright-report' }], ['list']],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? 'http://127.0.0.1:3010',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    timezoneId: 'UTC',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      // Мобильный проект (карта #1416, тикет #1421): тач-события и
      // мобильный вьюпорт для смоука хрома. В дефолтный прогон не входит
      // (раннер пинит проект через E2E_PROJECT) — workers:1 и общий
      // засеянный бэкенд удвоили бы каждый прогон; полный мобильный
      // прогон — решение владельца после приёмки смоука.
      name: 'mobile-chromium',
      use: { ...devices['Pixel 7'] },
    },
    {
      // Движковый замок (план 049): WebKit — движок телефона владельца,
      // и семьи багов позиции колеса ловились на нём трижды (#1429 —
      // реснап промахивается при перекройке месяцев; план 042 — Safari
      // без scroll-anchoring; план 049 — реснапа нет вовсе после
      // доращивки, коррекция после пейнта). На Chromium это не
      // воспроизводится никогда. План 051 добавил календарный якорь
      // (calendar-anchor): нативного scroll anchoring в WebKit нет до
      // Safari 27 — приписка ленты без ручной компенсации телепортирует
      // контент, на Chromium не воспроизводится. Скоуп узкий — колесные
      // и якорные спеки; в дефолтный прогон не входит (E2E_PROJECT, как
      // mobile-chromium):
      // E2E_PROJECT=webkit make frontend-e2e \
      //   TESTS="e2e/wheel-picker.spec.ts e2e/payments-wizard.spec.ts e2e/calendar-anchor.spec.ts"
      name: 'webkit',
      testMatch: /(wheel-picker|payments-wizard|calendar-anchor)\.spec\.ts$/,
      // Десктопный Safari: mouse.wheel (флик в спеках) в мобильном WebKit
      // Playwright не поддержан вовсе; движок тот же, мобильные вьюпорты
      // нужные спеки ставят сами (setViewportSize). Вьюпорт как у
      // дефолтного chromium (1280×720), чтобы тиры/layout совпадали.
      // Полисы CSP в этом прогоне без upgrade-insecure-requests — раннер
      // стартует фронт с E2E_OMIT_UPGRADE_INSECURE=1 (webkit эскалирует
      // по директиве ВСЕ http-подресурсы на https и остаётся без
      // CSS/JS; на https-проде директива инертна, Chromium localhost не
      // эскалирует). webkit-прогон — замок поведения колеса, не
      // CSP-комплаенс (он живёт на chromium): прогон эмулирует
      // https-контекст прод-страницы.
      use: {
        ...devices['Desktop Safari'],
        viewport: { width: 1280, height: 720 },
      },
    },
  ],
});
