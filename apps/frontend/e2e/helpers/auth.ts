import { Page, expect } from '@playwright/test';
import { readFileSync } from 'fs';
import path from 'path';

const BACKEND_LOG = process.env.BACKEND_LOG || path.resolve(process.cwd(), '../../.tmp/backend-e2e.log');

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

export async function extractCodeForPhone(phone: string, logPath: string = BACKEND_LOG): Promise<string> {
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    try {
      const log = readFileSync(logPath, 'utf-8');
      const match = log
        .split('\n')
        .filter((line) => line.includes(phone))
        .pop()
        ?.match(/Код подтверждения: (\d{6})/);
      if (match) return match[1];
    } catch {
      // log may not exist yet
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(`Could not extract SMS code for ${phone} from ${logPath}`);
}

export async function login(page: Page, phone: string): Promise<UserInfo> {
  await page.goto('/login');
  await page.waitForLoadState('networkidle');

  const phoneInput = page.getByRole('textbox', { name: /телефон/i });
  await phoneInput.click();
  await phoneInput.fill('');
  await phoneInput.pressSequentially(digitsAfterPlus7(phone), { delay: 50 });

  await expect(page.getByRole('button', { name: /войти/i })).toBeEnabled();
  await page.getByRole('button', { name: /войти/i }).click();

  const code = await extractCodeForPhone(phone);
  const codeInput = page.getByRole('textbox', { name: /код/i });
  await codeInput.fill('');
  await codeInput.pressSequentially(code, { delay: 50 });

  await page.waitForURL('/dashboard');
  await page.waitForLoadState('networkidle');

  const user: UserInfo = await page.request.fetch('http://localhost:8080/me', { method: 'GET' }).then((r) => r.json());
  return user;
}
