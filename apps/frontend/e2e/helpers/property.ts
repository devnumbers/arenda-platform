import { Page, expect } from '@playwright/test';
import { queryValue } from './db';

export async function createProperty(
  page: Page,
  userId: string,
  baseName: string
): Promise<{ propertyId: string; propertyName: string }> {
  await page.goto('/properties/new');
  await page.waitForLoadState('networkidle');

  await page.getByRole('button', { name: /квартира/i }).click();
  await page.getByRole('button', { name: /продолжить/i }).click();
  await page.getByLabel(/адрес/i).fill('г Москва, ул Лесная, д 5');
  await page.waitForTimeout(500);
  await page.keyboard.press('Escape');
  await page.getByRole('button', { name: /продолжить/i }).click();

  const propertyName = `${baseName} ${Date.now()}`;
  await page.getByLabel(/название/i).fill(propertyName);
  await page.getByRole('button', { name: /создать объект/i }).click();
  await expect(page.getByText(/объект создан/i)).toBeVisible();

  const propertyId = queryValue(
    `SELECT id FROM properties WHERE owner_id = '${userId}' AND name = '${propertyName}'`
  );
  if (!propertyId) {
    throw new Error(`Property ${propertyName} was not created`);
  }

  return { propertyId, propertyName };
}
