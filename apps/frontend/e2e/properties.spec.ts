import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
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

test('hero-иконка объекта: круг 96, глиф 52 — Figma Category Icon 2973:52664 (карта #984)', async ({ page, seededUser }) => {
  // Габариты «лица объекта» в шапке детали (макет 1186:44996): круг 96×96,
  // дом-плейсхолдер 52×52 (inset 22.73%). Регресс на схлопывание: аватар —
  // inline-span, вне flex-родителя h-24 w-24 игнорировалось и круг жил
  // размером глифа (40×40, дом без «воздуха»).
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}`);

  // Стриминговый буфер Suspense держит копию контента в hidden-диве конца
  // body, пока $RC-скрипт не заберёт его (гонка холодного старта: счётчик
  // 1→2→1 за ~400мс) — строгий локатор канона deep-link берёт видимый
  // аватар приложения (карта #984).
  const avatar = page.getByTestId('property-hero-avatar').filter({ visible: true });
  await expect(avatar).toBeVisible();
  await expect(avatar).toHaveCSS('width', '96px');
  await expect(avatar).toHaveCSS('height', '96px');
  await expect(avatar.locator('svg')).toHaveCSS('width', '52px');
  await expect(avatar.locator('svg')).toHaveCSS('height', '52px');
});

test('секции детали: тап по контенту карточки ведёт в раздел (карта #984)', async ({ page, seededUser }) => {
  // Раньше ссылка секции жила только в строке заголовка — контент карточки
  // был мёртвым (карта #984). На сид-квартире с данными: тап по площади
  // «Регулярных платежей» ниже заголовка ведёт в платежи объекта; тап по
  // заголовку «Задачи» — прежний путь в раздел.
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}`);

  // Стриминговый буфер Suspense держит скрытый клон контента (канон
  // deep-link из hero-спеки выше) — строгие локаторы берут видимые секции.
  const paymentsSection = page
    .locator('section', { hasText: 'Регулярные платежи' })
    .filter({ visible: true });
  await expect(paymentsSection).toBeVisible();
  const paymentsBox = await paymentsSection.boundingBox();
  await paymentsSection.click({ position: { x: 24, y: (paymentsBox?.height ?? 200) - 16 } });
  await expect(page).toHaveURL(new RegExp(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}/payments$`));

  await page.goto(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}`);
  const tasksSection = page
    .locator('section', { hasText: 'Задачи' })
    .filter({ visible: true });
  await expect(tasksSection).toBeVisible();
  await tasksSection.getByText('Задачи', { exact: true }).click();
  await expect(page).toHaveURL(new RegExp(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}/tasks$`));
});

test('правка объекта: «Описание» — статичный бокс на 8 строк, скролл внутри (#1080)', async ({ page, seededUser }) => {
  // Решение владельца 02.10 (кадр 1218-54295): бокс описания сразу 8 строк,
  // без autoGrow; текст сверх восьми строк скроллится внутри бокса. Тот же
  // TextField в визаре создания покрывает property-create.spec (шаг 3).
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}/edit`);

  const description = page.getByRole('textbox', { name: 'Описание' });
  await expect(description).toBeVisible();
  await expect(description).toHaveAttribute('rows', '8');
  const descBox = await description.boundingBox();
  expect(descBox?.height ?? 0).toBeCloseTo(144, 1); // 8 строк × 18px
  await description.fill(Array.from({ length: 12 }, (_, i) => `Строка ${i + 1}`).join('\n'));
  const descFilledBox = await description.boundingBox();
  expect(descFilledBox?.height ?? 0).toBeCloseTo(144, 1); // не растёт
  expect(await description.evaluate((el) => el.scrollHeight > el.clientHeight)).toBe(true);
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
