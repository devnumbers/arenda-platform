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
  ],
});
