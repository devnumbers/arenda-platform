import { execFile } from 'node:child_process';
import { readFile } from 'node:fs/promises';
import { promisify } from 'node:util';
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

/** Session tokens of the seeded co-members of the apartment (#467 role
 * matrix): full access (edits without delete) and viewer (read only). Lazy
 * on purpose — specs that never use them run without the extra env. */
export function seededMemberSessionToken(): string {
  return requiredEnv('E2E_MEMBER_SESSION_TOKEN');
}

export function seededViewerSessionToken(): string {
  return requiredEnv('E2E_VIEWER_SESSION_TOKEN');
}

/** Seeded property names (tools/e2e/frontend/seed.sql). */
export const SEEDED_PROPERTIES = ['Квартира на Ленина', 'Гараж на Садовой'] as const;

/** Seeded property ids (tools/e2e/frontend/seed.sql): on the apartment live
 * the payments of the «Платежи объекта» screen (#463); the garage is
 * intentionally paymentless for the empty states; the studio carries the
 * 55-overdue rule for the overdue sub-screen scroll test (#466). */
export const SEEDED_APARTMENT_PROPERTY_ID = '33333333-3333-4333-8333-333333333333';
export const SEEDED_GARAGE_PROPERTY_ID = '44444444-4444-4444-8444-444444444444';
export const SEEDED_STUDIO_PROPERTY_ID = '46464646-4646-4646-8646-464646464646';

const execFileAsync = promisify(execFile);

/**
 * Runs one SQL statement against the e2e database — the orchestrator's own
 * channel (it seeds and polls migrations through `docker exec … psql`).
 * Serves the lifecycle scenario (#468): the wizard can only create
 * future-dated rules, so its overdue leg seeds past-dated occurrences
 * mid-test the same way seed.sql does (status stays 'planned' — the
 * overdue projection is computed server-side against the owner's today).
 * Returns the trimmed psql stdout (e.g. "UPDATE 1") for row-count asserts.
 */
export async function execE2eSql(sql: string): Promise<string> {
  const container = requiredEnv('E2E_PG_CONTAINER');
  try {
    const { stdout } = await execFileAsync('docker', [
      'exec',
      container,
      'psql',
      '-U',
      'arenda',
      '-d',
      'arenda',
      '-v',
      'ON_ERROR_STOP=1',
      '-tAc',
      sql,
    ]);
    return stdout.trim();
  } catch (error) {
    throw new Error(`e2e SQL failed: ${sql}\n${error instanceof Error ? error.message : String(error)}`);
  }
}

/** Session cookie of the non-secure local backend (httpsupport.SessionCookieName). */
const SESSION_COOKIE_NAME = 'session_id';
const BASE_URL = process.env.E2E_BASE_URL ?? 'http://127.0.0.1:3010';

/**
 * Opens the cabinet without touching the login screen: the browser gets the
 * seeded session cookie, and the middleware /me check accepts it.
 */
export async function openCabinetWithSeededSession(page: Page, user: SeededUser): Promise<void> {
  await openCabinetWithSessionToken(page, user.sessionToken);
}

/** Same entry, but for any pre-authenticated seeded session (co-members of
 * the apartment — the #467 role matrix). */
export async function openCabinetWithSessionToken(page: Page, sessionToken: string): Promise<void> {
  await page.context().addCookies([
    {
      name: SESSION_COOKIE_NAME,
      value: sessionToken,
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
  return extractCodeSentTo(user, user.email);
}

function codePatternFor(email: string): RegExp {
  return new RegExp(`${email.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}[^\\n]*?Рентли: (\\d{6})`, 'g');
}

/**
 * Counts the codes already logged for the address. Serves as the baseline
 * for extractCodeSentTo: an earlier send to the same address (login spec,
 * the previous leg of the email-change flow) must not be mistaken for the
 * code that has not been logged yet.
 */
export async function countCodesSentTo(user: SeededUser, email: string): Promise<number> {
  const log = await readFile(user.backendLogPath, 'utf8');
  return [...log.matchAll(codePatternFor(email))].length;
}

/**
 * Extracts the last code emailed to the given address, but only once at
 * least minCount + 1 sends are in the log — the caller snapshots the count
 * before triggering the send. All identity codes (login, phone change,
 * email change #722) share the fake sender and the log format.
 */
export async function extractCodeSentTo(
  user: SeededUser,
  email: string,
  minCount = 0,
): Promise<string> {
  const codePattern = codePatternFor(email);
  const deadline = Date.now() + 15_000;
  while (Date.now() < deadline) {
    try {
      const log = await readFile(user.backendLogPath, 'utf8');
      const codes = [...log.matchAll(codePattern)].map((match) => match[1] ?? '');
      const code = codes.at(-1);
      if (code !== undefined && codes.length > minCount) {
        return code;
      }
    } catch {
      // The log appears after the first send; retry until the deadline.
    }
    await sleep(250);
  }
  throw new Error(`code for ${email} not found in backend log (${user.backendLogPath})`);
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
  // The design-layer TextField (titleIn) exposes the floating label as the
  // accessible name, so locators go by role+name.
  await page.getByRole('textbox', { name: 'Телефон' }).fill(user.phoneDigits);
  await page.getByRole('button', { name: 'Войти' }).click();

  await expect(page.getByRole('heading', { name: 'Введите код' })).toBeVisible();
  const code = await extractLoginCode(user);
  // The code field auto-verifies as soon as all six digits are in.
  await page.getByRole('textbox', { name: 'Код' }).fill(code);

  await page.waitForURL('**/properties');
  await expect(page.getByRole('heading', { name: 'Объекты' })).toBeVisible();
}
