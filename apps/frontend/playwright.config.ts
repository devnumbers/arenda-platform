import { defineConfig, devices } from '@playwright/test';

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
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
