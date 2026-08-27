import { readFile } from 'node:fs/promises';
import { setTimeout as sleep } from 'node:timers/promises';
import { test as base, expect, type Page, type TestInfo } from '@playwright/test';

// Shared fixtures of the frontend e2e suite. The environment contract is
// filled by tools/e2e/frontend/run-frontend-e2e.sh (`make frontend-e2e`):
// the seeded owner user, a pre-authenticated session token (cookie value),
// and the backend log the fake email sender writes login codes to.

export interface SeededUser {
  /** Phone digits without the +7 prefix, as typed into the login field. */
  readonly phoneDigits: string;
  readonly email: string;
  /** Raw session token; its hash sits in the seeded sessions row. */
  readonly sessionToken: string;
  /** Path of the backend log (JSON lines, contains login codes). */
  readonly backendLogPath: string;
}

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (value === undefined || value === '') {
    throw new Error(
      `${name} is not set — run the suite via 'make frontend-e2e' `
        + '(tools/e2e/frontend/run-frontend-e2e.sh exports the whole contract).',
    );
  }
  return value;
}

export const test = base.extend<{ seededUser: SeededUser }>({
  seededUser: async ({}, use) => {
    await use({
      phoneDigits: requiredEnv('E2E_USER_PHONE'),
      email: requiredEnv('E2E_USER_EMAIL'),
      sessionToken: requiredEnv('E2E_SESSION_TOKEN'),
      backendLogPath: requiredEnv('E2E_BACKEND_LOG'),
    });
  },
});
export { expect };

/** Seeded property names (tools/e2e/frontend/seed.sql). */
export const SEEDED_PROPERTIES = ['Квартира на Ленина', 'Гараж на Садовой'] as const;

/** Seeded property ids (tools/e2e/frontend/seed.sql): on the apartment live
 * the payments of the «Платежи объекта» screen (#463); the garage is
 * intentionally paymentless for the empty states; the studio carries the
 * 55-overdue rule for the overdue sub-screen scroll test (#466). */
export const SEEDED_APARTMENT_PROPERTY_ID = '33333333-3333-4333-8333-333333333333';
export const SEEDED_GARAGE_PROPERTY_ID = '44444444-4444-4444-8444-444444444444';
export const SEEDED_STUDIO_PROPERTY_ID = '46464646-4646-4646-8646-464646464646';

/** Session cookie of the non-secure local backend (httpsupport.SessionCookieName). */
const SESSION_COOKIE_NAME = 'session_id';
const BASE_URL = process.env.E2E_BASE_URL ?? 'http://127.0.0.1:3010';

/**
 * Opens the cabinet without touching the login screen: the browser gets the
 * seeded session cookie, and the middleware /me check accepts it.
 */
export async function openCabinetWithSeededSession(page: Page, user: SeededUser): Promise<void> {
  await page.context().addCookies([
    {
      name: SESSION_COOKIE_NAME,
      value: user.sessionToken,
      url: BASE_URL,
    },
  ]);
}

/**
 * Extracts the last login code emailed for the seeded user. The fake email
 * sender logs the message body ("Код для входа в Рентли: NNNNNN"); polling
 * covers the send→log latency. Bruno API-e2e precedent (grep of the same
 * log, tools/e2e/run-e2e-with-db-checks.sh).
 */
export async function extractLoginCode(user: SeededUser): Promise<string> {
  const codePattern = new RegExp(`${user.email.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}[^\\n]*?Рентли: (\\d{6})`, 'g');
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    try {
      const log = await readFile(user.backendLogPath, 'utf8');
      const codes = [...log.matchAll(codePattern)].map((match) => match[1] ?? '');
      const code = codes.at(-1);
      if (code) {
        return code;
      }
    } catch {
      // The log appears after the first send; retry until the deadline.
    }
    await sleep(250);
  }
  throw new Error(`login code not found in backend log (${user.backendLogPath})`);
}

/**
 * Full-page screenshot as a run artifact: written into the test output dir
 * and attached to the HTML report — the material for the design comparison
 * against the Figma frame (uploaded from CI by the frontend-e2e job).
 */
export async function captureScreen(page: Page, testInfo: TestInfo, name: string): Promise<void> {
  const path = testInfo.outputPath(`${name}.png`);
  await page.screenshot({ path, fullPage: true });
  await testInfo.attach(name, { path, contentType: 'image/png' });
}

/**
 * Full login through the real UI: phone step → code step (the code comes
 * from the backend log) → redirect into the cabinet. Ends on /properties
 * with the list heading visible.
 */
export async function loginViaUi(page: Page, user: SeededUser): Promise<void> {
  await page.goto('/login');
  // HeroUI TextField exposes the label as the accessible name (the placeholder
  // attribute renders as a single space), so locators go by role+name.
  await page.getByRole('textbox', { name: 'Телефон' }).fill(user.phoneDigits);
  await page.getByRole('button', { name: 'Войти' }).click();

  await expect(page.getByRole('heading', { name: 'Введите код' })).toBeVisible();
  const code = await extractLoginCode(user);
  // The code field auto-verifies as soon as all six digits are in.
  await page.getByRole('textbox', { name: '6-значный код' }).fill(code);

  await page.waitForURL('**/properties');
  await expect(page.getByRole('heading', { name: 'Мои объекты' })).toBeVisible();
}
