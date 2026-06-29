import { test, expect, Page } from '@playwright/test';
import { login, generatePhone } from './helpers/auth';
import { createProperty } from './helpers/property';
import { assertDbState } from './helpers/db';
import { assertNoBackendErrors, assertNoFrontendErrors, getLogOffset, BACKEND_LOG, FRONTEND_LOG } from './helpers/logs';

async function createOperation(
  page: Page,
  type: 'income' | 'expense',
  propertyId: string,
  name: string
): Promise<void> {
  await page.goto(`/finance/create-operation?type=${type}&propertyId=${propertyId}`);
  await page.waitForLoadState('networkidle');

  await page.getByLabel(/сумма операции/i).fill('15000');
  await page.getByLabel(/название операции/i).fill(name);

  const categoryLabel = type === 'income' ? /категория дохода/i : /категория расхода/i;
  await page.getByLabel(categoryLabel).click();
  const optionText = type === 'income' ? 'Арендная плата' : 'ЖКХ';
  await page.getByRole('option', { name: optionText }).click();

  await page.getByRole('button', { name: /далее/i }).click();

  await page.getByLabel(/дата операции/i).fill('2026-06-15');

  await page.getByRole('button', { name: /далее/i }).click();

  // Reminder step: keep disabled and submit
  await page.getByRole('button', { name: /создать операцию/i }).click();
}

test.describe('Operation lifecycle', () => {
  test('create income operation', async ({ page }) => {
    const backendOffset = getLogOffset(BACKEND_LOG);
    const frontendOffset = getLogOffset(FRONTEND_LOG);
    const phone = generatePhone();
    const user = await login(page, phone);
    const { propertyId } = await createProperty(page, user.id, 'E2E Income Property');

    const operationName = `E2E Income ${Date.now()}`;
    await createOperation(page, 'income', propertyId, operationName);

    await expect(page.getByText(/операция создана/i)).toBeVisible();

    await assertDbState(
      `SELECT count(*)::text FROM operations WHERE property_id = '${propertyId}' AND name = '${operationName}' AND type = 'income' AND amount_kopecks = 1500000`,
      '1',
      'Income operation should be created'
    );

    assertNoBackendErrors(backendOffset);
    assertNoFrontendErrors(frontendOffset);
  });

  test('create expense operation', async ({ page }) => {
    const backendOffset = getLogOffset(BACKEND_LOG);
    const frontendOffset = getLogOffset(FRONTEND_LOG);
    const phone = generatePhone();
    const user = await login(page, phone);
    const { propertyId } = await createProperty(page, user.id, 'E2E Expense Property');

    const operationName = `E2E Expense ${Date.now()}`;
    await createOperation(page, 'expense', propertyId, operationName);

    await expect(page.getByText(/операция создана/i)).toBeVisible();

    await assertDbState(
      `SELECT count(*)::text FROM operations WHERE property_id = '${propertyId}' AND name = '${operationName}' AND type = 'expense' AND amount_kopecks = 1500000`,
      '1',
      'Expense operation should be created'
    );

    assertNoBackendErrors(backendOffset);
    assertNoFrontendErrors(frontendOffset);
  });

  test('create recurring monthly operation', async ({ page }) => {
    const backendOffset = getLogOffset(BACKEND_LOG);
    const frontendOffset = getLogOffset(FRONTEND_LOG);
    const phone = generatePhone();
    const user = await login(page, phone);
    const { propertyId } = await createProperty(page, user.id, 'E2E Recurring Property');

    await page.goto(`/finance/create-operation?type=income&propertyId=${propertyId}`);
    await page.waitForLoadState('networkidle');

    const operationName = `E2E Recurring ${Date.now()}`;
    await page.getByLabel(/сумма операции/i).fill('12000');
    await page.getByLabel(/название операции/i).fill(operationName);

    await page.getByLabel(/категория дохода/i).click();
    await page.getByRole('option', { name: /арендная плата/i }).click();

    await page.getByRole('button', { name: /далее/i }).click();

    await page.getByRole('radio', { name: /каждый месяц/i }).click();
    await page.getByLabel(/дата первого повтора/i).fill('2026-07-05');
    await page.getByLabel(/день операции/i).fill('5');

    await page.getByRole('button', { name: /далее/i }).click();

    // Reminder step
    await page.getByRole('button', { name: /создать операцию/i }).click();

    await assertDbState(
      `SELECT count(*)::text FROM recurring_operations WHERE property_id = '${propertyId}' AND name = '${operationName}' AND type = 'income' AND amount_kopecks = 1200000 AND periodicity = 'monthly'`,
      '1',
      'Recurring operation should be created'
    );

    assertNoBackendErrors(backendOffset);
    assertNoFrontendErrors(frontendOffset);
  });
});
