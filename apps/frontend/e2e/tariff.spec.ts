import { test, expect } from '@playwright/test';
import { login, generatePhone } from './helpers/auth';
import { assertDbState } from './helpers/db';
import { assertNoBackendErrors, assertNoFrontendErrors, getLogOffset, BACKEND_LOG, FRONTEND_LOG } from './helpers/logs';

test.describe('Tariff change', () => {
  test('change tariff via UI', async ({ page }) => {
    const backendOffset = getLogOffset(BACKEND_LOG);
    const frontendOffset = getLogOffset(FRONTEND_LOG);
    const phone = generatePhone();
    const user = await login(page, phone);

    await page.goto('/profile/tariff/change');
    await page.waitForLoadState('networkidle');

    await page.getByRole('radio', { name: /год/i }).click();
    await page.getByRole('button', { name: /выбрать/i }).nth(1).click();

    // Fake provider redirects to an external confirmation URL; intercept it.
    await page.waitForURL(/fake-subscription-payment/);
    const paymentUrl = page.url();
    const paymentId = paymentUrl.match(/fake-subscription-payment\/([a-f0-9-]+)\/confirm/)?.[1];
    expect(paymentId).toBeDefined();

    // Confirm payment via backend API directly (external URL is not reachable locally).
    const confirmResp = await page.request.post(`http://localhost:8080/internal/fake-subscription-payment/${paymentId}/confirm`);
    expect(confirmResp.status()).toBe(200);

    await assertDbState(
      `SELECT count(*)::text FROM subscription_payments WHERE user_id = '${user.id}' AND provider = 'fake'`,
      '1',
      'Subscription payment should be created'
    );

    assertNoBackendErrors(backendOffset);
    assertNoFrontendErrors(frontendOffset);
  });
});
