import {
  captureScreen,
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  screenHeader,
  test,
} from './fixtures';

// Экран «Ваши участники» (карта #692, тикет #697): список агрегатов
// GET /participants, чипы агрегат-статуса, клиентские поиск и сортировка
// «Имя», кебаб-трио «Пригласить участника» / «История действий» /
// «Отозвать доступ всем» (#843, макет 2008-47514). Сид: владелец
// «Квартиры на Ленина»
// с тремя участниками — Анна Лимитова (full_access), Мария Петрова
// (full_access) и Сергей Сидоров (viewer), все active на одном объекте
// из трёх в скоупе владельца → агрегат-статус partial, чип «Доступно
// 1 объект». Анна — свободный сид-юзер suspended/slot-сценариев
// (у Марии и Сергея подписки pro для гейтов #997).

test('список участников: ряды сида с чипами статусов, чип «Имя», CTA', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants/list');

  const header = screenHeader(page);
  await expect(header.getByText('Ваши участники')).toBeVisible();

  // Ряды агрегатов: имя + почта + чип «Доступно 1 объект» (у обоих
  // активный доступ к одному объекту из трёх в скоупе — partial).
  const mariaRow = page.getByRole('button', { name: /Мария Петрова/ });
  await expect(mariaRow).toBeVisible();
  await expect(mariaRow.getByText('e2e-member@example.com')).toBeVisible();
  await expect(mariaRow.getByText('Доступно 1 объект')).toBeVisible();

  const sergeyRow = page.getByRole('button', { name: /Сергей Сидоров/ });
  await expect(sergeyRow).toBeVisible();
  await expect(sergeyRow.getByText('e2e-viewer@example.com')).toBeVisible();
  await expect(sergeyRow.getByText('Доступно 1 объект')).toBeVisible();

  // Сервер отдаёт name ASC — Анна первая.
  const names = page.getByRole('button', { name: /@example\.com/ });
  await expect(names).toHaveCount(3);
  await expect(names.first()).toContainText('Анна Лимитова');

  // Чип сортировки и служебные иконки шапки.
  await expect(page.getByRole('button', { name: 'Имя' })).toBeVisible();
  await expect(header.getByRole('button', { name: 'Поиск участников' })).toBeVisible();
  await expect(header.getByRole('button', { name: 'Еще — действия со списком' })).toBeVisible();

  // CTA нижней панели — текстовая кнопка (вне шапки).
  await expect(
    page
      .getByRole('button', { name: 'Пригласить участника' })
      .filter({ hasText: 'Пригласить участника' }),
  ).toBeVisible();

  await captureScreen(page, testInfo, 'participants-list');
});

test('поиск: фильтрует по имени и почте, пустой результат, выход из поиска', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants/list');

  const header = screenHeader(page);
  await header.getByRole('button', { name: 'Поиск участников' }).click();

  const searchField = page.getByRole('searchbox', { name: 'Поиск участников' });
  await expect(searchField).toBeFocused();

  // Чип сортировки в режиме поиска спрятан (макет 2008-84003).
  await expect(page.getByRole('button', { name: 'Имя' })).toHaveCount(0);

  await searchField.fill('сер');
  const sergeyRow = page.getByRole('button', { name: /Сергей Сидоров/ });
  await expect(sergeyRow).toBeVisible();
  await expect(page.getByRole('button', { name: /Мария Петрова/ })).toHaveCount(0);

  await searchField.fill('example.com');
  await expect(page.getByRole('button', { name: /@example\.com/ })).toHaveCount(3);

  await searchField.fill('александр');
  await expect(page.getByText('Участник не найден')).toBeVisible();

  // Выход из поиска возвращает список целиком.
  await header.getByRole('button', { name: 'Закрыть поиск' }).click();
  await expect(page.getByRole('button', { name: /Мария Петрова/ })).toBeVisible();
  await expect(page.getByRole('button', { name: /Сергей Сидоров/ })).toBeVisible();
});

test('сортировка «Имя»: «Убывание» переворачивает список', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants/list');

  await page.getByRole('button', { name: 'Имя' }).click();
  // Сюит гоняется на десктопном вьюпорте — PickerMenu открывает меню
  // (menuitem), а не мобильный шит с radio.
  await page.getByRole('menuitem', { name: 'Убывание' }).click();

  const rows = page.getByRole('button', { name: /@example\.com/ });
  await expect(rows).toHaveCount(3);
  await expect(rows.first()).toContainText('Сергей Сидоров');
});

test('кебаб: трио пунктов по макету 2008-47514, «История действий» ведёт в ленту', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants/list');

  await screenHeader(page).getByRole('button', { name: 'Еще — действия со списком' }).click();

  // Состав кебаба (#843): «Пригласить участника», «История действий» и
  // красное «Отозвать доступ всем» — анатомия макета 2008-47514.
  await expect(page.getByRole('menuitem', { name: 'Пригласить участника' })).toBeVisible();
  const historyItem = page.getByRole('menuitem', { name: 'История действий' });
  await expect(historyItem).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'Отозвать доступ всем' })).toBeVisible();

  await historyItem.click();
  await page.waitForURL('**/history');
});

test('безымянный участник: полный телефон в титуле ряда, почта подзаголовком (карта #1105); сид восстанавливается', async ({ page, seededUser }, testInfo) => {
  // Зарегистрированный юзер без имени: display_name собирает бекенд по
  // канону карты #1105 — полный телефон вместо маски «+7***» (#1106).
  // Телефон сидится плейнтекстом при phone_encrypted = false —
  // decryptPhone читает как есть, шифрование нужно только логину.
  const NAMELESS_ID = '16111111-1111-4111-8111-111111111161';
  const NAMELESS_MEMBER_ID = '99999999-9999-4999-8999-999999999961';
  const NAMELESS_PHONE = '+79137654321';
  const NAMELESS_EMAIL = 'e2e-nameless-list@example.com';
  await execE2eSql(
    `INSERT INTO users (id, phone, role, name, surname, email, phone_encrypted, timezone) ` +
      `VALUES ('${NAMELESS_ID}', '${NAMELESS_PHONE}', 'owner', NULL, NULL, '${NAMELESS_EMAIL}', FALSE, 'UTC') ` +
      `ON CONFLICT (id) DO NOTHING; ` +
      `INSERT INTO property_members (id, property_id, user_id, role, granted_by) ` +
      `VALUES ('${NAMELESS_MEMBER_ID}', '33333333-3333-4333-8333-333333333333', '${NAMELESS_ID}', 'viewer', '11111111-1111-4111-8111-111111111111') ` +
      `ON CONFLICT (property_id, user_id) WHERE status = 'active' DO NOTHING`,
  );
  try {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/participants/list');

    // Титул ряда — полный телефон, почта — подзаголовком под ним.
    // Доступное имя ряда — титул + подзаголовок разом, матч регуляркой;
    // ведущий «+» в регэксп не нужен (в регэкспе это квантификатор).
    const namelessRow = page.getByRole('button', { name: new RegExp(NAMELESS_PHONE.slice(1)) });
    await expect(namelessRow).toBeVisible();
    await expect(namelessRow.getByText(NAMELESS_EMAIL)).toBeVisible();
    // Маски в списке нет ни в каком виде.
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
  await page.goto('/participants/list');

  await screenHeader(page).getByRole('button', { name: 'Еще — действия со списком' }).click();
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

test('«Отозвать и удалить»: ряды исчезают, попап успеха; сид восстанавливается', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants/list');

  const header = screenHeader(page);
  await header.getByRole('button', { name: 'Еще — действия со списком' }).click();
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
    // Пустой список: служебные иконки шапки спрятаны, иллюстрация на месте.
    await expect(page.getByText('Участников пока нет')).toBeVisible();
    await expect(header.getByRole('button', { name: 'Поиск участников' })).toHaveCount(0);
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
