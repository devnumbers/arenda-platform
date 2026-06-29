import { test, expect } from '@playwright/test';
import { login, generatePhone } from './helpers/auth';
import { assertDbState } from './helpers/db';
import { assertNoBackendErrors, assertNoFrontendErrors, getLogOffset, BACKEND_LOG, FRONTEND_LOG } from './helpers/logs';

test.describe('Property lifecycle', () => {
  test('create a property', async ({ page }) => {
    const backendOffset = getLogOffset(BACKEND_LOG);
    const frontendOffset = getLogOffset(FRONTEND_LOG);
    const phone = generatePhone();
    const user = await login(page, phone);

    await page.goto('/properties/new');
    await page.waitForLoadState('networkidle');

    await page.getByRole('button', { name: /квартира/i }).click();
    await page.getByRole('button', { name: /продолжить/i }).click();

    await page.getByLabel(/адрес/i).fill('г Москва, ул Ленина, д 1');
    await page.waitForTimeout(500);
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: /продолжить/i }).click();

    const propertyName = `E2E Property ${Date.now()}`;
    await page.getByLabel(/название/i).fill(propertyName);
    await page.getByLabel(/описание/i).fill('E2E test property description');
    await page.getByRole('button', { name: /создать объект/i }).click();

    await expect(page.getByText(/объект создан/i)).toBeVisible();

    await assertDbState(
      `SELECT count(*)::text FROM properties WHERE owner_id = '${user.id}' AND name = '${propertyName}' AND status = 'active'`,
      '1',
      'Property should be created in DB'
    );

    assertNoBackendErrors(backendOffset);
    assertNoFrontendErrors(frontendOffset);
  });

  test('archive a property', async ({ page }) => {
    const backendOffset = getLogOffset(BACKEND_LOG);
    const frontendOffset = getLogOffset(FRONTEND_LOG);
    const phone = generatePhone();
    const user = await login(page, phone);

    await page.goto('/properties/new');
    await page.waitForLoadState('networkidle');

    await page.getByRole('button', { name: /квартира/i }).click();
    await page.getByRole('button', { name: /продолжить/i }).click();
    await page.getByLabel(/адрес/i).fill('г Москва, ул Ленина, д 2');
    await page.waitForTimeout(500);
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: /продолжить/i }).click();
    const propertyName = `E2E Archive ${Date.now()}`;
    await page.getByLabel(/название/i).fill(propertyName);
    await page.getByRole('button', { name: /создать объект/i }).click();
    await expect(page.getByText(/объект создан/i)).toBeVisible();

    await page.goto('/properties');
    await page.waitForLoadState('networkidle');
    await page.getByText(propertyName).click();
    await page.waitForLoadState('networkidle');
    await page.getByLabel(/действия/i).click();
    await page.getByRole('button', { name: /перевести в архив/i }).click();

    await assertDbState(
      `SELECT count(*)::text FROM properties WHERE owner_id = '${user.id}' AND name = '${propertyName}' AND status = 'archived'`,
      '1',
      'Property should be archived'
    );

    assertNoBackendErrors(backendOffset);
    assertNoFrontendErrors(frontendOffset);
  });
});
