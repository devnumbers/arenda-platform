import {
  captureScreen,
  execE2eSql,
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

test('чужие карточки: ряд владельца — имя, у безымянного «Пользователь» (карта #1105, аменд #1123, #1109)', async ({ page, seededUser }, testInfo) => {
  // Ряд «чей объект» на чужих карточках с активным доступом строит бекенд
  // по канону карты #1105 — «Имя Фамилия», иначе «Пользователь» (#1106,
  // аменд #1123, OwnerDisplayNameResolver → access.DisplayNameOf); фронт
  // строку не пере-маскирует. Сид инлайновый (workers=1), восстанавливается.
  const MARIA_PROPERTY_ID = '35555555-5555-4555-8555-555555555551';
  const NAMELESS_PROPERTY_ID = '35555555-5555-4555-8555-555555555552';
  const NAMELESS_ID = '15111111-1111-4111-8111-111111111152';
  const NAMELESS_PHONE = '+79137654325';
  const MARIA_MEMBER_ID = '99999999-9999-4999-8999-999999999971';
  const NAMELESS_MEMBER_ID = '99999999-9999-4999-8999-999999999972';
  await execE2eSql(
    // Телефон безымянного сидится плейнтекстом при phone_encrypted = false
    // (паттерн #1107): decryptPhone читает как есть.
    `INSERT INTO users (id, phone, role, name, surname, email, phone_encrypted, timezone) ` +
      `VALUES ('${NAMELESS_ID}', '${NAMELESS_PHONE}', 'owner', NULL, NULL, 'e2e-nameless-cards@example.com', FALSE, 'UTC') ` +
      `ON CONFLICT (id) DO NOTHING; ` +
      `INSERT INTO properties (id, owner_id, name, type, address, status) VALUES ` +
      `('${MARIA_PROPERTY_ID}', '12111111-1111-4111-8111-111111111121', 'Офис у Марии', 'office', 'Москва, ул. Обручева, 9', 'active'), ` +
      `('${NAMELESS_PROPERTY_ID}', '${NAMELESS_ID}', 'Хата без имени', 'house', 'Москва, ул. Анонимная, 8', 'active') ` +
      `ON CONFLICT (id) DO NOTHING; ` +
      `INSERT INTO property_members (id, property_id, user_id, role, granted_by) VALUES ` +
      `('${MARIA_MEMBER_ID}', '${MARIA_PROPERTY_ID}', '11111111-1111-4111-8111-111111111111', 'viewer', '12111111-1111-4111-8111-111111111121'), ` +
      `('${NAMELESS_MEMBER_ID}', '${NAMELESS_PROPERTY_ID}', '11111111-1111-4111-8111-111111111111', 'full_access', '${NAMELESS_ID}') ` +
      `ON CONFLICT (property_id, user_id) WHERE status = 'active' DO NOTHING`,
  );
  try {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/properties');

    const officeCard = page
      .locator('li')
      .filter({ has: page.getByRole('heading', { name: 'Офис у Марии' }) });
    await expect(officeCard.getByTestId('property-owner').getByText('Мария Петрова')).toBeVisible();

    const namelessCard = page
      .locator('li')
      .filter({ has: page.getByRole('heading', { name: 'Хата без имени' }) });
    await expect(namelessCard.getByTestId('property-owner').getByText('Пользователь')).toBeVisible();
    // Телефона в ряду владельца нет (аменд #1123).
    await expect(namelessCard.getByTestId('property-owner').getByText(NAMELESS_PHONE)).toHaveCount(0);

    // Масок нет ни на одной карточке хаба.
    await expect(page.getByText(/\*{3}/)).toHaveCount(0);

    await captureScreen(page, testInfo, '1109-foreign-cards-owner-row');
  } finally {
    await execE2eSql(
      `DELETE FROM property_members WHERE id IN ('${MARIA_MEMBER_ID}', '${NAMELESS_MEMBER_ID}'); ` +
        `DELETE FROM properties WHERE id IN ('${MARIA_PROPERTY_ID}', '${NAMELESS_PROPERTY_ID}'); ` +
        `DELETE FROM users WHERE id = '${NAMELESS_ID}'`,
    );
  }
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
