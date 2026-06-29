import { test, expect } from '@playwright/test';
import { login, generatePhone } from './helpers/auth';
import { assertDbState, assertDbRows } from './helpers/db';
import { assertNoBackendErrors, assertNoFrontendErrors, getLogOffset, BACKEND_LOG, FRONTEND_LOG } from './helpers/logs';

test.describe('Tenant creation', () => {
  test('creates a tenant from property page', async ({ page }) => {
    const backendOffset = getLogOffset(BACKEND_LOG);
    const frontendOffset = getLogOffset(FRONTEND_LOG);
    const phone = generatePhone();
    const user = await login(page, phone);

    // Create property
    await page.goto('/properties/new');
    await page.waitForLoadState('networkidle');
    await page.getByRole('button', { name: /квартира/i }).click();
    await page.getByRole('button', { name: /продолжить/i }).click();
    await page.getByLabel(/адрес/i).fill('г Москва, ул Ленина, д 4');
    await page.waitForTimeout(500);
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: /продолжить/i }).click();
    const propertyName = `E2E Tenant Property ${Date.now()}`;
    await page.getByLabel(/название/i).fill(propertyName);
    await page.getByRole('button', { name: /создать объект/i }).click();
    await expect(page.getByText(/объект создан/i)).toBeVisible();

    await assertDbRows<{ id: string }>(
      `SELECT id FROM properties WHERE owner_id = '${user.id}' AND name = '${propertyName}'`,
      (rows) => rows.length === 1,
      'Property should be created'
    );

    // Open tenant creation form (global tenant contacts)
    await page.goto('/tenants/new');
    await page.waitForLoadState('networkidle');

    await page.getByLabel(/имя/i).fill('Петр');
    await page.getByLabel(/фамилия/i).fill('Петров');
    await page.getByLabel(/отчество/i).fill('Петрович');
    await page.getByLabel(/телефон/i).fill('');
    await page.getByLabel(/телефон/i).pressSequentially('9234567890', { delay: 50 });
    await page.getByRole('button', { name: /добавить арендатора/i }).click();

    await expect(page.getByText(/арендатор добавлен/i)).toBeVisible();

    await assertDbState(
      `SELECT count(*)::text FROM tenant_contacts WHERE owner_id = '${user.id}' AND name = 'Петр' AND surname = 'Петров' AND patronymic = 'Петрович'`,
      '1',
      'Tenant contact should be created in DB'
    );

    assertNoBackendErrors(backendOffset);
    assertNoFrontendErrors(frontendOffset);
  });
});
