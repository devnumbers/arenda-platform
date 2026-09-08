import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Страницы-заглушки единой навигации хрома (карта #556, тикет #559): пункты
// меню «Платежи» и «Участники» ведут на живые маршруты — 404 под пунктом
// меню и есть баг, который гасит тикет. Макетов у заглушек нет: каждая —
// канон хаба (TopNav с «крыльями» #523, заголовок раздела 28, канонный
// EmptyState на PageContent); настоящие экраны приедут отдельными усилиями
// владельца.

test('заглушка «Платежи»: хаб-шапка и EmptyState, не 404', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/payments');

  // Хаб-шапка с «крыльями» и на мобайле (лого — ссылка на объекты).
  await expect(page.getByRole('link', { name: 'Объекты' })).toBeVisible();
  await expect(page.getByRole('heading', { level: 1, name: 'Платежи', exact: true })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Платежи появятся здесь' })).toBeVisible();

  await captureScreen(page, testInfo, 'payments-stub');
});

test('заглушка «Участники»: хаб-шапка и EmptyState, не 404', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants');

  await expect(page.getByRole('link', { name: 'Объекты' })).toBeVisible();
  await expect(
    page.getByRole('heading', { level: 1, name: 'Участники', exact: true }),
  ).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Участники появятся здесь' })).toBeVisible();

  await captureScreen(page, testInfo, 'participants-stub');
});
