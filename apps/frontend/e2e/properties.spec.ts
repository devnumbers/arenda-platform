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
  // Поверхностей создания в хабе три (#586); на десктопном ярусе кликабельна
  // «+» пилюли — компакт-бар шапки скрыт до скролла, якоримся к пилюле.
  await expect(page.getByRole('heading', { name: 'Объекты', exact: true })).toBeVisible();
  await expect(
    page.getByTestId('properties-search-pill').getByRole('button', { name: 'Создать объект' }),
  ).toBeVisible();
  for (const name of SEEDED_PROPERTIES) {
    await expect(page.getByText(name, { exact: true })).toBeVisible();
  }

  await captureScreen(page, testInfo, 'properties');
});

test('архив объектов: шапка подэкрана с «Назад», не 404', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties/archive');

  // Подэкран: ведущая «Назад» на список, заголовок в хедере; «+» создания
  // в архиве нет — принятый канон #587.
  await expect(page.getByRole('button', { name: 'Назад' })).toBeVisible();
  await expect(page.getByText('Архивные объекты')).toBeVisible();

  await captureScreen(page, testInfo, 'properties-archive');
});

test('без подписки (404 /subscription): кнопки создания живые, ведут на смену тарифа #768', async ({ page, seededUser }) => {
  // Состояние сид-Марии из обхода #760 воспроизводим перехватом: до фикса
  // 404 держал react-query в вечном pending, и кнопки создания хаба были
  // disabled навсегда.
  await page.route('**/api/subscription', (route) =>
    route.fulfill({
      status: 404,
      contentType: 'application/problem+json',
      body: JSON.stringify({ code: 'not_found', detail: 'Подписка не найдена' }),
    }),
  );
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties');

  // Список рисуется, «+» пилюли не disabled и носит защитную подписку
  // (canAdd=false при «подписки нет» — тот же путь, что лимит тарифа).
  await expect(page.getByRole('heading', { name: 'Объекты', exact: true })).toBeVisible();
  const pillAdd = page
    .getByTestId('properties-search-pill')
    .getByRole('button', { name: 'Достигнут лимит объектов по тарифу — сменить тариф' });
  await expect(pillAdd).toBeVisible();
  await expect(pillAdd).toBeEnabled();

  // Защитный путь: тап ведёт на «Выбрать тариф», где пикер рисуется
  // (фолбэк «подписки нет ≡ базовый»), а не ошибка загрузки.
  await pillAdd.click();
  await expect(page).toHaveURL(/\/profile\/tariff\/change$/);
  const tariffRadio = page.getByRole('radiogroup', { name: 'Тариф' });
  await expect(tariffRadio.getByText('Базовый', { exact: true })).toBeVisible();
  await expect(page.getByText('Текущий', { exact: true })).toBeVisible();
  await expect(page.getByText('Не удалось загрузить данные тарифов')).toHaveCount(0);
});
