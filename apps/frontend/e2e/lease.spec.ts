import { test, expect } from '@playwright/test';
import { login, generatePhone } from './helpers/auth';
import { assertDbState, assertDbRows } from './helpers/db';
import { assertNoBackendErrors, assertNoFrontendErrors, getLogOffset, BACKEND_LOG, FRONTEND_LOG } from './helpers/logs';

function isoDate(monthsAhead: number, dayOfMonth: number): string {
  const date = new Date();
  date.setMonth(date.getMonth() + monthsAhead);
  date.setDate(dayOfMonth);
  return date.toISOString().slice(0, 10);
}

test.describe('Lease creation', () => {
  test('creates a lease from property page', async ({ page }) => {
    const backendOffset = getLogOffset(BACKEND_LOG);
    const frontendOffset = getLogOffset(FRONTEND_LOG);
    const phone = generatePhone();
    const user = await login(page, phone);

    // Create a property first
    await page.goto('/properties/new');
    await page.waitForLoadState('networkidle');
    await page.getByRole('button', { name: /квартира/i }).click();
    await page.getByRole('button', { name: /продолжить/i }).click();
    await page.getByLabel(/адрес/i).fill('г Москва, ул Ленина, д 3');
    await page.waitForTimeout(500);
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: /продолжить/i }).click();
    const propertyName = `E2E Lease Property ${Date.now()}`;
    await page.getByLabel(/название/i).fill(propertyName);
    await page.getByRole('button', { name: /создать объект/i }).click();
    await expect(page.getByText(/объект создан/i)).toBeVisible();

    await assertDbRows<{ id: string }>(
      `SELECT id FROM properties WHERE owner_id = '${user.id}' AND name = '${propertyName}'`,
      (rows) => rows.length === 1,
      'Property should be created'
    );

    // Open the property from the list
    await page.goto('/properties');
    await page.waitForLoadState('networkidle');
    await page.getByText(propertyName).click();
    await page.waitForLoadState('networkidle');

    await page.getByRole('link', { name: /создать аренду/i }).click();
    await page.waitForLoadState('networkidle');

    // Step 1: price and deposit
    await page.getByLabel(/арендная плата/i).fill('50000');
    await page.getByLabel(/залог/i).fill('100000');
    await page.getByRole('button', { name: /продолжить/i }).click();

    // Step 2: use sessionStorage to bypass HeroUI drawer/date-picker animations in tests
    const draft = {
      step: 2,
      rentAmount: '50000',
      depositAmount: '100000',
      paymentDay: 1,
      startDate: isoDate(1, 15),
      endDate: isoDate(7, 15),
    };
    await page.evaluate((d) => sessionStorage.setItem('lease-create-draft', JSON.stringify(d)), draft);
    await page.reload();
    await page.waitForLoadState('networkidle');

    await page.getByRole('button', { name: /создать аренду/i }).click();

    await expect(page.getByText(/аренда создана/i)).toBeVisible();

    await assertDbState(
      `SELECT count(*)::text FROM leases l JOIN properties p ON l.property_id = p.id WHERE p.owner_id = '${user.id}' AND p.name = '${propertyName}'`,
      '1',
      'Lease should be created in DB'
    );

    assertNoBackendErrors(backendOffset);
    assertNoFrontendErrors(frontendOffset);
  });
});
