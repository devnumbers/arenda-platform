import {
  captureScreen,
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  screenHeader,
  test,
} from './fixtures';

// Раздел «Участники» — единая страница списка (D1–D4 #1313; макеты
// 3313-77128/3313-78626/3315-79334): ряды агрегатов GET /participants с
// бейджами ровно двух статусов, пилюля поиска, клиентская сортировка
// «Имя», кебаб раздела «Пригласить участника» / «История действий» /
// «Отозвать доступ всем» (глобальный DELETE /participants, #1310).
// Сид: владелец «Квартиры на Ленина» с тремя участниками — Анна Лимитова
// (full_access), Мария Петрова (full_access) и Сергей Сидоров (viewer),
// все active на одном объекте из трёх в скоупе владельца. Анна —
// свободный сид-юзер suspended/slot-сценариев (у Марии и Сергея подписки
// pro для гейтов #997).

const SECTION_KEBAB = 'participants-section-kebab-row';

test('снесённые маршруты хаба — 308 на живые экраны (канон /dashboard, #975)', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);

  // /participants/list был списком «Ваших участников» — им /participants
  // и остался (D1 #1313). goto следует за 308 — статус проверяем у
  // финального ответа, как в каноне /dashboard (#975).
  const list = await page.goto('/participants/list');
  expect(list?.status()).toBe(200);
  await expect(page).toHaveURL(/\/participants$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Участники' })).toBeVisible();

  // «Объекты пользователей» снесены без наследника — чужие объекты живут
  // в общем списке.
  const props = await page.goto('/participants/properties');
  expect(props?.status()).toBe(200);
  await expect(page).toHaveURL(/\/properties$/);
});

test('список участников: ряды сида без счётчиков, чип «Имя», пилюля, кебаб', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants');

  // Заголовок раздела в ряду хаб-шапки.
  await expect(page.getByRole('heading', { name: 'Участники' })).toBeVisible();

  // Ряды агрегатов: имя + почта; статусы «частично/все объекты» и
  // счётчики объектов ушли (D3 #1313) — у активных участников бейджа нет.
  const mariaRow = page.getByRole('button', { name: /Мария Петрова/ });
  await expect(mariaRow).toBeVisible();
  await expect(mariaRow.getByText('e2e-member@example.com')).toBeVisible();
  await expect(page.getByText(/Доступно \d+/)).toHaveCount(0);
  await expect(page.getByText('Доступ ко всем объектам')).toHaveCount(0);

  const sergeyRow = page.getByRole('button', { name: /Сергей Сидоров/ });
  await expect(sergeyRow).toBeVisible();
  await expect(sergeyRow.getByText('e2e-viewer@example.com')).toBeVisible();

  // Сервер отдаёт name ASC — Анна первая.
  const names = page.getByRole('button', { name: /@example\.com/ });
  await expect(names).toHaveCount(3);
  await expect(names.first()).toContainText('Анна Лимитова');

  // Пилюля поиска и чип сортировки; кебаб — в ряду заголовка (в шапке
  // его больше нет — хаб-паттерн).
  await expect(page.getByTestId('participants-search-pill')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Имя' })).toBeVisible();
  await expect(page.getByTestId(SECTION_KEBAB).getByRole('button', { name: 'Еще — действия со списком' })).toBeVisible();

  await captureScreen(page, testInfo, 'participants-list');
});

test('поиск: пилюля ведёт на /participants/search, фильтр, пустой результат, выход', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants');

  await page.getByTestId('participants-search-pill').click();
  await page.waitForURL('**/participants/search');

  const searchField = page.getByRole('searchbox', { name: 'Поиск участников' });
  await expect(searchField).toBeFocused();

  await searchField.fill('сер');
  await expect(page.getByRole('button', { name: /Сергей Сидоров/ })).toBeVisible();
  await expect(page.getByRole('button', { name: /Мария Петрова/ })).toHaveCount(0);

  await searchField.fill('example.com');
  await expect(page.getByRole('button', { name: /@example\.com/ })).toHaveCount(3);

  await searchField.fill('александр');
  await expect(page.getByText('Участник не найден')).toBeVisible();

  // Выход из поиска возвращает раздел целиком.
  await screenHeader(page).getByRole('button', { name: 'Закрыть поиск' }).click();
  await page.waitForURL('**/participants');
  await expect(page.getByRole('button', { name: /Мария Петрова/ })).toBeVisible();
  await expect(page.getByRole('button', { name: /Сергей Сидоров/ })).toBeVisible();
});

test('сортировка «Имя»: «Убывание» переворачивает список', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants');

  await page.getByRole('button', { name: 'Имя' }).click();
  // Сюит гоняется на десктопном вьюпорте — PickerMenu открывает меню
  // (menuitem), а не мобильный шит с radio.
  await page.getByRole('menuitem', { name: 'Убывание' }).click();

  const rows = page.getByRole('button', { name: /@example\.com/ });
  await expect(rows).toHaveCount(3);
  await expect(rows.first()).toContainText('Сергей Сидоров');
});

test('кебаб: трио пунктов по макету 3313-78243, «История действий» ведёт в ленту', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants');

  await page.getByTestId(SECTION_KEBAB).getByRole('button', { name: 'Еще — действия со списком' }).click();

  // Состав кебаба (D4 #1313): «Пригласить участника», «История действий»
  // и красное «Отозвать доступ всем».
  await expect(page.getByRole('menuitem', { name: 'Пригласить участника' })).toBeVisible();
  const historyItem = page.getByRole('menuitem', { name: 'История действий' });
  await expect(historyItem).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'Отозвать доступ всем' })).toBeVisible();

  await historyItem.click();
  await page.waitForURL('**/history');
});

test('безымянный участник: «Пользователь» в титуле ряда, почта подзаголовком (карта #1105, аменд #1123); сид восстанавливается', async ({ page, seededUser }, testInfo) => {
  // Зарегистрированный юзер без имени: display_name собирает бекенд по
  // канону карты #1105 — «Пользователь» вместо маски «+7***» (#1106);
  // аменд #1123: телефон больше не фолбэк, из схемы он ушёл совсем
  // (ADR 0067, миграция 000162) — пин «телефона в ряду нет» остаётся
  // страховкой от любых остаточных масок в UI.
  const NAMELESS_ID = '16111111-1111-4111-8111-111111111161';
  const NAMELESS_MEMBER_ID = '99999999-9999-4999-8999-999999999961';
  const NAMELESS_PHONE = '+79137654321';
  const NAMELESS_EMAIL = 'e2e-nameless-list@example.com';
  await execE2eSql(
    `INSERT INTO users (id, role, name, surname, email, timezone) ` +
      `VALUES ('${NAMELESS_ID}', 'owner', NULL, NULL, '${NAMELESS_EMAIL}', 'UTC') ` +
      `ON CONFLICT (id) DO NOTHING; ` +
      `INSERT INTO property_members (id, property_id, user_id, role, granted_by) ` +
      `VALUES ('${NAMELESS_MEMBER_ID}', '33333333-3333-4333-8333-333333333333', '${NAMELESS_ID}', 'viewer', '11111111-1111-4111-8111-111111111111') ` +
      `ON CONFLICT (property_id, user_id) WHERE status = 'active' DO NOTHING`,
  );
  try {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/participants');

    // Титул ряда — «Пользователь», почта — подзаголовком под ним; безымянный
    // в списке один — ряд уникален по титулу.
    const namelessRow = page.getByRole('button', { name: 'Пользователь' });
    await expect(namelessRow).toBeVisible();
    await expect(namelessRow.getByText(NAMELESS_EMAIL)).toBeVisible();
    // Телефона в ряду нет (аменд #1123), маски в списке нет ни в каком виде.
    await expect(namelessRow.getByText(NAMELESS_PHONE)).toHaveCount(0);
    await expect(page.getByText(/\*{3}/)).toHaveCount(0);

    await captureScreen(page, testInfo, 'participants-list-nameless');
  } finally {
    await execE2eSql(
      `DELETE FROM property_members WHERE id = '${NAMELESS_MEMBER_ID}'; ` +
        `DELETE FROM users WHERE id = '${NAMELESS_ID}'`,
    );
  }
});

test('кебаб: «Отозвать доступ всем» открывает подтверждение, «Отмена» ничего не делает', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants');

  await page.getByTestId(SECTION_KEBAB).getByRole('button', { name: 'Еще — действия со списком' }).click();
  await page.getByRole('menuitem', { name: 'Отозвать доступ всем' }).click();

  const dialog = page.getByRole('dialog');
  await expect(
    dialog.getByText('Отозвать доступ всем пользователям к вашим объектам?'),
  ).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Отозвать и удалить' })).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Отмена' })).toBeVisible();

  await dialog.getByRole('button', { name: 'Отмена' }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByRole('button', { name: /Мария Петрова/ })).toBeVisible();
});

test('«Отозвать и удалить»: глобальный отзыв, попап успеха, пустое состояние с CTA; сид восстанавливается', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants');

  await page.getByTestId(SECTION_KEBAB).getByRole('button', { name: 'Еще — действия со списком' }).click();
  await page.getByRole('menuitem', { name: 'Отозвать доступ всем' }).click();

  const dialog = page.getByRole('dialog');
  try {
    await dialog.getByRole('button', { name: 'Отозвать и удалить' }).click();

    // Попап успеха по макету 2008-84101; список под ним перечитан
    // (sr-only заголовок несёт тот же текст — проверяем видимый <p>).
    await expect(
      page.getByRole('dialog').locator('p', { hasText: 'Все участники удалены' }),
    ).toBeVisible();
    await expect(page.getByRole('button', { name: /Мария Петрова/ })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /Сергей Сидоров/ })).toHaveCount(0);

    await page.keyboard.press('Escape');
    // Пустой список платного тарифа (макет 3313-78626): CTA «Пригласить»
    // в форме, кебаб остаётся — но без красного пункта (макет 3314-79248).
    await expect(page.getByText('Участников нет')).toBeVisible();
    await expect(page.getByTestId('participants-empty-cta')).toBeVisible();
    await expect(page.getByTestId('participants-search-pill')).toHaveCount(0);
    await page.getByTestId(SECTION_KEBAB).getByRole('button', { name: 'Еще — действия со списком' }).click();
    await expect(page.getByRole('menuitem', { name: 'Пригласить участника' })).toBeVisible();
    await expect(page.getByRole('menuitem', { name: 'Отозвать доступ всем' })).toHaveCount(0);
  } finally {
    // Восстановление сида (воркеры=1 — позже по сюиту сид нужен
    // payment-edit-delete: кабинет сид-участников).
    await execE2eSql(
      "INSERT INTO property_members (id, property_id, user_id, role, granted_by) VALUES " +
        "('99999999-9999-4999-8999-999999999931', '33333333-3333-4333-8333-333333333333', '12111111-1111-4111-8111-111111111121', 'full_access', '11111111-1111-4111-8111-111111111111'), " +
        "('99999999-9999-4999-8999-999999999932', '33333333-3333-4333-8333-333333333333', '13111111-1111-4111-8111-111111111131', 'viewer', '11111111-1111-4111-8111-111111111111'), " +
        "('99999999-9999-4999-8999-999999999935', '33333333-3333-4333-8333-333333333333', '14111111-1111-4111-8111-111111111141', 'full_access', '11111111-1111-4111-8111-111111111111') " +
        'ON CONFLICT (id) DO NOTHING',
    );
  }
  await page.reload();
  await expect(page.getByRole('button', { name: /Мария Петрова/ })).toBeVisible();
  await expect(page.getByRole('button', { name: /Сергей Сидоров/ })).toBeVisible();
});

test('basic: апселл-пустота, кебаб и пилюля скрыты, CTA ведёт на интерстишал шаринга; тариф восстанавливается', async ({ page, seededUser }) => {
  const OWNER_SUBSCRIPTION_ID = '99999999-9999-4999-8999-999999999901';
  // Перевод владельца на basic (не удаление подписки: «подписки нет» —
  // 404 → null, и optimistic-умолчание фронта считает это платным).
  const previousTariff = await execE2eSql(
    `SELECT t.name FROM user_subscriptions s JOIN tariffs t ON t.id = s.tariff_id WHERE s.id = '${OWNER_SUBSCRIPTION_ID}'`,
  );
  await execE2eSql(
    `UPDATE user_subscriptions SET tariff_id = (SELECT id FROM tariffs WHERE name = 'basic') ` +
      `WHERE id = '${OWNER_SUBSCRIPTION_ID}'`,
  );
  try {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/participants');

    // Пустое состояние-апселл (D2 #1313, макет 3315-79334): список скрыт
    // до оплаты — даже с живыми участниками сида (downgrade-семантика),
    // данные не пропадают.
    await expect(page.getByText('Участников нет')).toBeVisible();
    await expect(page.getByText('Пригласите пользователей в свои объекты и управляйте вместе')).toBeVisible();
    await expect(page.getByRole('button', { name: /Мария Петрова/ })).toHaveCount(0);

    // Кебаб раздела скрыт целиком, пилюли и чипа нет.
    await expect(page.getByTestId(SECTION_KEBAB).getByRole('button', { name: 'Еще — действия со списком' })).toHaveCount(0);
    await expect(page.getByTestId('participants-search-pill')).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Имя' })).toHaveCount(0);

    // CTA «Пригласить» — на интерстишал шаринга (#1470, решение
    // владельца 15.10); выбор тарифа — следующей кнопкой с экрана.
    await page.getByTestId('participants-upsell-cta').click();
    await page.waitForURL('**/profile/tariff/sharing');
    await expect(page.getByRole('heading', { name: 'Совместный доступ недоступен' })).toBeVisible();
  } finally {
    await execE2eSql(
      `UPDATE user_subscriptions SET tariff_id = (SELECT id FROM tariffs WHERE name = '${previousTariff}') ` +
        `WHERE id = '${OWNER_SUBSCRIPTION_ID}'`,
    );
  }
});
