import { Page, expect } from '@playwright/test';
import { readFileSync } from 'fs';
import { BACKEND_LOG } from './logs';

const API_URL = process.env.API_URL || 'http://localhost:8080';

// Extracts the 6-digit code from a log line already known to contain
// "fake email sent" and the target email. Works for both text/pretty and JSON
// logs regardless of field order.
const FAKE_EMAIL_CODE_REGEX = /\bcode["\s:=]*(\d{6})/i;

export type UserInfo = {
  id: string;
  phone: string;
  role: string;
};

export function generatePhone(): string {
  const suffix = Math.floor(100000000 + Math.random() * 900000000);
  return `+79${suffix.toString().slice(0, 9)}`;
}

function digitsAfterPlus7(phone: string): string {
  return phone.replace(/^\+7/, '').replace(/\D/g, '');
}

function generateEmail(phone: string): string {
  return `${phone.replace(/\D/g, '')}@example.com`;
}

export async function extractCodeForEmail(email: string, logPath: string = BACKEND_LOG): Promise<string> {
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    try {
      const log = readFileSync(logPath, 'utf-8');
      const lines = log.split('\n');
      for (let i = lines.length - 1; i >= 0; i--) {
        const line = lines[i];
        if (!line.includes('fake email sent') || !line.includes(email)) {
          continue;
        }
        // Strip ANSI colour codes (used by the tint/pretty handler) before parsing.
        const cleanLine = line.replace(/\x1b\[[0-9;]*m/g, '');
        const match = cleanLine.match(FAKE_EMAIL_CODE_REGEX);
        if (match) {
          return match[1];
        }
      }
    } catch (err) {
      // The log may not exist yet; retry in that case. Any other read error
      // should surface immediately.
      if (err && typeof err === 'object' && 'code' in err && err.code === 'ENOENT') {
        // retry
      } else {
        throw err;
      }
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(`Could not extract email code for ${email} from ${logPath}`);
}

export async function login(page: Page, phone: string = generatePhone(), email: string = generateEmail(phone)): Promise<UserInfo> {
  await page.goto('/login');
  await page.waitForLoadState('networkidle');

  const phoneInput = page.getByRole('textbox', { name: /телефон/i });
  await phoneInput.click();
  await phoneInput.fill('');
  await phoneInput.pressSequentially(digitsAfterPlus7(phone), { delay: 50 });

  await expect(page.getByRole('button', { name: /войти/i })).toBeEnabled();
  await page.getByRole('button', { name: /войти/i }).click();

  const emailInput = page.getByRole('textbox', { name: /email/i });
  await emailInput.fill('');
  await emailInput.fill(email);

  await expect(page.getByRole('button', { name: /получить код/i })).toBeEnabled();
  await page.getByRole('button', { name: /получить код/i }).click();

  const code = await extractCodeForEmail(email);
  const codeInput = page.getByRole('textbox', { name: /код/i });
  await codeInput.fill('');
  await codeInput.pressSequentially(code, { delay: 50 });

  await page.waitForURL('/dashboard');
  await page.waitForLoadState('networkidle');

  const user: UserInfo = await page.request.fetch(`${API_URL}/me`, { method: 'GET' }).then((r) => r.json());
  return user;
}
