import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  SEEDED_PROPERTIES,
  test,
} from './fixtures';

// Screen smoke of the properties list (ticket #456): the canonical example
// every future screen spec follows — enter the cabinet on the seeded
// session, assert the seeded data on screen, capture a screenshot artifact
// for the Figma comparison (decision of 2026-08-25, spec #453 revision).
// The UI-login path into the same screen lives in auth.spec.ts.

test('список объектов: карточки сид-объектов и скриншот экрана', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties');

  // Хаб нового хрома (карта #556, тикет #562): заголовок 28, «+» создания.
  await expect(page.getByRole('heading', { name: 'Объекты', exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Создать объект' })).toBeVisible();
  for (const name of SEEDED_PROPERTIES) {
    await expect(page.getByText(name, { exact: true })).toBeVisible();
  }

  await captureScreen(page, testInfo, 'properties');
});

test('архив объектов: шапка подэкрана с «Назад», не 404', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties/archive');

  // Подэкран: ведущая «Назад» на список, заголовок в хедере, «+» в trailing.
  await expect(page.getByRole('button', { name: 'Назад' })).toBeVisible();
  await expect(page.getByText('Архивные объекты')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Создать объект' })).toBeVisible();

  await captureScreen(page, testInfo, 'properties-archive');
});
