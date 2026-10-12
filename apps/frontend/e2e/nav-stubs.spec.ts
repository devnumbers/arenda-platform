import {
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Раздел «Участники» (карта #692, D1 #1313): единая страница списка —
// хаб-карточки и «Объекты пользователей» снесены. Покрытие экрана живёт
// в participants-list.spec.ts; здесь — входы навигации.

test('вход в раздел «Участники» с ПК — сайдбар; в профиле строки нет', async ({
  page,
  seededUser,
}) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/profile');
  await expect(page.getByRole('button', { name: 'Участники' })).toHaveCount(0);

  const sidebar = page.getByRole('navigation', { name: 'Основная навигация' });
  await sidebar.getByRole('link', { name: 'Участники' }).click();
  await page.waitForURL('**/participants');
  await expect(page.getByRole('heading', { level: 1, name: 'Участники' })).toBeVisible();
});

test.describe('мобайл', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('вход в раздел «Участники» с мобайла — шит «Еще»; в профиле строки нет', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile');
    await expect(page.getByRole('button', { name: 'Участники' })).toHaveCount(0);

    await page.getByRole('button', { name: 'Еще' }).click();
    const moreSheet = page.getByRole('dialog', { name: 'Еще' });
    await moreSheet.getByRole('link', { name: 'Участники' }).click();
    await page.waitForURL('**/participants');
    await expect(page.getByRole('heading', { level: 1, name: 'Участники' })).toBeVisible();
  });
});
