import { test, expect } from '@playwright/test';
import { generatePhone } from './helpers/auth';
import { assertNoBackendErrors, assertNoFrontendErrors, getLogOffset, BACKEND_LOG, FRONTEND_LOG } from './helpers/logs';

test.describe('Auth', () => {
  test('rejects invalid SMS code', async ({ page }) => {
    const backendOffset = getLogOffset(BACKEND_LOG);
    const frontendOffset = getLogOffset(FRONTEND_LOG);
    const phone = generatePhone();
    await page.goto('/login');
    await page.waitForLoadState('networkidle');

    const phoneInput = page.getByRole('textbox', { name: /телефон/i });
    await phoneInput.click();
    await phoneInput.fill('');
    await phoneInput.pressSequentially(phone.replace(/\D/g, '').slice(1), { delay: 50 });

    await expect(page.getByRole('button', { name: /войти/i })).toBeEnabled();
    await page.getByRole('button', { name: /войти/i }).click();

    const codeInput = page.getByRole('textbox', { name: /код/i });
    await codeInput.fill('');
    await codeInput.pressSequentially('000000', { delay: 50 });

    await expect(page.getByText(/неверный код/i)).toBeVisible();

    assertNoBackendErrors(backendOffset);
    assertNoFrontendErrors(frontendOffset);
  });
});
