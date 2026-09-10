import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Страница-заглушка единой навигации хрома (карта #556, тикет #559): пункт
// меню «Участники» ведёт на живой маршрут — 404 под пунктом меню и есть баг,
// который гасит тикет. Макета у заглушки нет: это канон хаба (TopNav с
// «крыльями» #523, заголовок раздела 28, канонный EmptyState на PageContent).
// Заглушка «Платежи» заменена настоящим разделом (карта #573) — её покрытие
// переехало в payments.spec.ts.

test('заглушка «Участники»: хаб-шапка и EmptyState, не 404', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants');

  const header = page.locator('header[aria-label="Навигация экрана"]');
  await expect(header.getByRole('link', { name: 'Объекты' })).toBeVisible();
  await expect(
    page.getByRole('heading', { level: 1, name: 'Участники', exact: true }),
  ).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Участники появятся здесь' })).toBeVisible();

  await captureScreen(page, testInfo, 'participants-stub');
});
