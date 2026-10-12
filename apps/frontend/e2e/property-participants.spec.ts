import {
  captureScreen,
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  openCabinetWithSessionToken,
  seededMemberSessionToken,
  seededViewerSessionToken,
  screenHeader,
  test,
} from './fixtures';

// Экран «Участники объекта» (D6 #1312, макет 3298-75434): полный состав
// с почтами и pending-приглашениями — read-only всем; у владельца кебаб
// «Пригласить/История/Отозвать доступ всем» и синий CTA приглашения, у
// приглашённого — кебаб только с «Историей объекта» и красный CTA
// «Покинуть объект»; бейджи «Аккаунт не создан»/«Превышен лимит
// объектов»; D15 — на basic кебаб объекта без пунктов шаринга. Сид:
// владелец «Квартиры на Ленина» (33333333-…) Иван Иванов с двумя
// участниками — Мария Петрова (full_access, member …931) и Сергей
// Сидоров (viewer, member …932); их собственные сессии — E2E_MEMBER_/
// E2E_VIEWER_SESSION_TOKEN. Разрушающие тесты восстанавливают сид через
// execE2eSql (workers=1).

const APARTMENT_ID = '33333333-3333-4333-8333-333333333333';
const MARIA_ID = '12111111-1111-4111-8111-111111111121';
const MARIA_MEMBER_ID = '99999999-9999-4999-8999-999999999931';
const SERGEY_ID = '13111111-1111-4111-8111-111111111131';
const SERGEY_MEMBER_ID = '99999999-9999-4999-8999-999999999932';
const INVITE_EMAIL = 'e2e-property-invite@example.com';

test('вход из детали объекта: «Управление» → «Совместный доступ», ряды списка', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/properties/${APARTMENT_ID}`);

  // Путь пользователя: секция «Управление» → пункт «Совместный доступ».
  await page.getByRole('button', { name: 'Совместный доступ' }).click();
  await expect(screenHeader(page).getByText('Участники объекта')).toBeVisible();

  // Владелец — отдельный первый ряд с «(Вы)» (статичный, без шеврона),
  // участники — с почтой.
  await expect(page.getByText('Иван Иванов (Вы)')).toBeVisible();
  const mariaRow = page.getByRole('button', { name: /Мария Петрова/ });
  await expect(mariaRow).toBeVisible();
  await expect(mariaRow.getByText('e2e-member@example.com')).toBeVisible();
  const sergeyRow = page.getByRole('button', { name: /Сергей Сидоров/ });
  await expect(sergeyRow).toBeVisible();
  await expect(sergeyRow.getByText('e2e-viewer@example.com')).toBeVisible();

  // CTA приглашения от объекта — постоянная нижняя панель.
  await expect(
    page.getByRole('button', { name: 'Пригласить участника' }).filter({ hasText: 'Пригласить участника' }),
  ).toBeVisible();

  await captureScreen(page, testInfo, 'property-participants');
});

test('тап по ряду — страница участника агрегата', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/properties/${APARTMENT_ID}/participants`);

  await page.getByRole('button', { name: /Мария Петрова/ }).click();
  await expect(screenHeader(page).getByText('Участник', { exact: true }).first()).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Мария Петрова' })).toBeVisible();
});

test('manage-участник (D8): состав read-only — своя строка «(Вы)» и чужие ряды инертные; CTA выхода на месте', async ({ page }) => {
  // Мария — full_access на чужой «Квартире на Ленина»: D8 (#1304) забрал у
  // неё member-management (canManageMembers — только владелец), D6 оставил
  // чтение состава, «Историю объекта» и «Покинуть объект».
  await openCabinetWithSessionToken(page, seededMemberSessionToken());
  await page.goto(`/properties/${APARTMENT_ID}/participants`);
  const header = screenHeader(page);

  await expect(header.getByText('Участники объекта')).toBeVisible();

  // Своя строка с «(Вы)» — статичный ряд: не role=button, без шеврона.
  await expect(page.getByText('Мария Петрова (Вы)')).toBeVisible();
  await expect(page.getByRole('button', { name: /Мария Петрова/ })).toHaveCount(0);

  // Тап по своей строке никуда не ведёт — экран остаётся на месте.
  await page.getByText('Мария Петрова (Вы)').click();
  await expect(header.getByText('Участники объекта')).toBeVisible();
  await expect(page.getByText('Участник не найден')).toHaveCount(0);
  await expect(page).toHaveURL(new RegExp(`/properties/${APARTMENT_ID}/participants$`));

  // D8: чужие ряды тоже статичные — страницы участника вне скоупа
  // не-владельца (404-политика #693).
  await expect(page.getByRole('button', { name: /Сергей Сидоров/ })).toHaveCount(0);

  // D6: у приглашённого вместо приглашения — красный CTA выхода (кебаб
  // «История объекта» покрыт зрительским тестом).
  await expect(page.getByRole('button', { name: 'Покинуть объект' })).toBeVisible();
});

test('поиск: подсказка пустого, находка по имени/почте, «Участник не найден»', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/properties/${APARTMENT_ID}/participants`);

  await screenHeader(page).getByRole('button', { name: 'Поиск участников' }).click();
  const field = page.getByRole('searchbox', { name: 'Поиск участников' });

  // Пустой запрос — подсказка макета 1980-108531.
  await expect(page.getByText('Введите имя или почту участника')).toBeVisible();

  await field.fill('мария');
  await expect(page.getByRole('button', { name: /Мария Петрова/ })).toBeVisible();
  await expect(page.getByRole('button', { name: /Сергей Сидоров/ })).toHaveCount(0);

  await field.fill('e2e-viewer@example.com');
  await expect(page.getByRole('button', { name: /Сергей Сидоров/ })).toBeVisible();
  await expect(page.getByRole('button', { name: /Мария Петрова/ })).toHaveCount(0);

  await field.fill('александр');
  await expect(page.getByText('Участник не найден')).toBeVisible();
});

test('чип-фильтр ролей: «Просмотр» оставляет зрителя, «Все роли» возвращает', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/properties/${APARTMENT_ID}/participants`);

  // Desktop-канон PickerMenu: меню с menuitem (не мобильный шит).
  await page.getByRole('button', { name: /Все роли/ }).click();
  await page.getByRole('menuitem', { name: 'Просмотр' }).click();

  await expect(page.getByText('Сергей Сидоров')).toBeVisible();
  await expect(page.getByRole('button', { name: /Мария Петрова/ })).toHaveCount(0);
  // Владелец под фильтр роли не попадает — строка скрыта тоже.
  await expect(page.getByText('Иван Иванов (Вы)')).toHaveCount(0);

  await page.getByRole('button', { name: /Просмотр/ }).click();
  await page.getByRole('menuitem', { name: 'Все роли' }).click();
  await expect(page.getByRole('button', { name: /Мария Петрова/ })).toBeVisible();
  await expect(page.getByText('Иван Иванов (Вы)')).toBeVisible();
});

test('приглашение от объекта: свой email — 400, свежая почта — попап и SQL; сид восстанавливается', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/properties/${APARTMENT_ID}/participants`);

  await page
    .getByRole('button', { name: 'Пригласить участника' })
    .filter({ hasText: 'Пригласить участника' })
    .click();
  // Заголовок приглашения — в контенте (канон #699), шапка без титула.
  await expect(page.getByRole('heading', { name: 'Пригласите участника' })).toBeVisible();
  // Приглашение от объекта — без выбора объектов (макет 1978-103001).
  await expect(page.getByRole('button', { name: /Выбрать объект/ })).toHaveCount(0);

  const emailField = page.getByRole('textbox', { name: 'Электронная почта' });

  // Своя почта — семантический 400 с текстом бэка под полем.
  await emailField.fill(seededUser.email);
  await page.getByRole('button', { name: 'Пригласить' }).filter({ hasText: 'Пригласить' }).click();
  await expect(page.getByText('Нельзя добавить себя участником')).toBeVisible();

  // Свежая почта — pending-приглашение, возврат на список с попапом.
  await emailField.fill(INVITE_EMAIL);
  await page.getByRole('radio', { name: 'Редактирование' }).click();
  try {
    await page.getByRole('button', { name: 'Пригласить' }).filter({ hasText: 'Пригласить' }).click();

    await expect(screenHeader(page).getByText('Участники объекта')).toBeVisible();
    await expect(
      page.getByRole('dialog').locator('p', { hasText: 'Участник приглашен' }),
    ).toBeVisible();
    await page.keyboard.press('Escape');

    // Pending-строка по D6 (3298-75434): титул «Пользователь» (бекенд
    // display_name pending не наполняет — лейбл #1105 ставит VM), почта
    // подзаголовком, info-бейдж «Аккаунт не создан».
    const pendingRow = page.getByRole('button', { name: 'Пользователь' });
    await expect(pendingRow).toBeVisible();
    await expect(pendingRow.getByText(INVITE_EMAIL)).toBeVisible();
    await expect(pendingRow.getByText('Аккаунт не создан')).toBeVisible();

    // Серверная правда: приглашение именно на этот объект с выбранной ролью.
    expect(
      await execE2eSql(
        `SELECT count(*) FROM property_member_invitations WHERE property_id = '${APARTMENT_ID}' AND email = '${INVITE_EMAIL}' AND role = 'full_access'`,
      ),
    ).toBe('1');
  } finally {
    await execE2eSql(
      `DELETE FROM property_member_invitations WHERE property_id = '${APARTMENT_ID}' AND email = '${INVITE_EMAIL}'`,
    );
  }
});

test('suspended-участник: warning-чип в ряду; сид восстанавливается', async ({ page, seededUser }) => {
  await execE2eSql(
    `UPDATE property_members SET status = 'suspended' WHERE id = '${MARIA_MEMBER_ID}'`,
  );
  try {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`/properties/${APARTMENT_ID}/participants`);

    const mariaRow = page.getByRole('button', { name: /Мария Петрова/ });
    await expect(mariaRow.getByText('Превышен лимит объектов')).toBeVisible();
  } finally {
    await execE2eSql(
      `UPDATE property_members SET status = 'active' WHERE id = '${MARIA_MEMBER_ID}'`,
    );
  }
});

test('безымянный участник: «Пользователь» в титуле ряда, почта подзаголовком (карта #1105, аменд #1123); сид восстанавливается', async ({ page, seededUser }) => {
  // Зарегистрированный юзер без имени: display_name собирает бекенд по
  // канону карты #1105 — «Пользователь» вместо маски «+7***» (#1106);
  // аменд #1123: телефон больше не фолбэк. Телефон сидится плейнтекстом
  // при phone_encrypted = false — decryptPhone читает как есть,
  // шифрование нужно только логину.
  const NAMELESS_ID = '15111111-1111-4111-8111-111111111151';
  const NAMELESS_MEMBER_ID = '99999999-9999-4999-8999-999999999951';
  const NAMELESS_PHONE = '+79131234567';
  const NAMELESS_EMAIL = 'e2e-nameless@example.com';
  await execE2eSql(
    `INSERT INTO users (id, phone, role, name, surname, email, phone_encrypted, timezone) ` +
      `VALUES ('${NAMELESS_ID}', '${NAMELESS_PHONE}', 'owner', NULL, NULL, '${NAMELESS_EMAIL}', FALSE, 'UTC') ` +
      `ON CONFLICT (id) DO NOTHING; ` +
      `INSERT INTO property_members (id, property_id, user_id, role, granted_by) ` +
      `VALUES ('${NAMELESS_MEMBER_ID}', '${APARTMENT_ID}', '${NAMELESS_ID}', 'viewer', '11111111-1111-4111-8111-111111111111') ` +
      `ON CONFLICT (property_id, user_id) WHERE status = 'active' DO NOTHING`,
  );
  try {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`/properties/${APARTMENT_ID}/participants`);

    // Титул ряда — «Пользователь», почта — подзаголовком под ним; безымянный
    // в списке один — ряд уникален по титулу.
    const namelessRow = page.getByRole('button', { name: 'Пользователь' });
    await expect(namelessRow).toBeVisible();
    await expect(namelessRow.getByText(NAMELESS_EMAIL)).toBeVisible();
    // Телефона в ряду нет (аменд #1123), маски в ряду нет ни в каком виде.
    await expect(namelessRow.getByText(NAMELESS_PHONE)).toHaveCount(0);
    await expect(page.getByText(/\*{3}/)).toHaveCount(0);
  } finally {
    await execE2eSql(
      `DELETE FROM property_members WHERE id = '${NAMELESS_MEMBER_ID}'; ` +
        `DELETE FROM users WHERE id = '${NAMELESS_ID}'`,
    );
  }
});

test('кебаб «Отозвать доступ всем»: подтверждение, попап, владелец остаётся; сид восстанавливается', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/properties/${APARTMENT_ID}/participants`);

  // Отзыв снимает и pending-приглашения: садим одно SQL-ом рядом с
  // членствами (AC #1312 — «все членства и приглашения объекта»).
  await execE2eSql(
    `INSERT INTO property_member_invitations (id, property_id, email, role, invited_by, last_sent_at) ` +
      `VALUES ('99999999-9999-4999-8999-999999999971', '${APARTMENT_ID}', '${INVITE_EMAIL}', 'viewer', '11111111-1111-4111-8111-111111111111', now()) ` +
      `ON CONFLICT (property_id, lower(email)) DO NOTHING`,
  );
  await page.reload();

  await screenHeader(page).getByRole('button', { name: 'Еще — действия со списком' }).click();
  await page.getByRole('menuitem', { name: 'Отозвать доступ всем' }).click();

  const dialog = page.getByRole('dialog');
  await expect(
    dialog.getByText('Отозвать доступ всем пользователям к вашему объекту?'),
  ).toBeVisible();
  try {
    await dialog.getByRole('button', { name: 'Отозвать и удалить' }).click();

    // Попап успеха (2035-82834): владелец в списке, участников нет.
    await expect(
      page.getByRole('dialog').locator('p', { hasText: 'Все участники удалены' }),
    ).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByText('Иван Иванов (Вы)')).toBeVisible();
    await expect(page.getByRole('button', { name: /Мария Петрова/ })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /Сергей Сидоров/ })).toHaveCount(0);
    await expect(page.getByText(INVITE_EMAIL)).toHaveCount(0);

    // Серверная правда: членств и приглашений на объекте нет; сид возвращается.
    expect(
      await execE2eSql(
        `SELECT count(*) FROM property_members WHERE property_id = '${APARTMENT_ID}'`,
      ),
    ).toBe('0');
    expect(
      await execE2eSql(
        `SELECT count(*) FROM property_member_invitations WHERE property_id = '${APARTMENT_ID}'`,
      ),
    ).toBe('0');
  } finally {
    await execE2eSql(
      `INSERT INTO property_members (id, property_id, user_id, role, granted_by) VALUES ` +
        `('${MARIA_MEMBER_ID}', '${APARTMENT_ID}', '${MARIA_ID}', 'full_access', '11111111-1111-4111-8111-111111111111'), ` +
        `('${SERGEY_MEMBER_ID}', '${APARTMENT_ID}', '${SERGEY_ID}', 'viewer', '11111111-1111-4111-8111-111111111111') ` +
        `ON CONFLICT (id) DO NOTHING`,
    );
  }
});

test('зритель: состав read-only, кебаб только с «Историей объекта», красный CTA «Покинуть объект» (D6)', async ({ page }, testInfo) => {
  await openCabinetWithSessionToken(page, seededViewerSessionToken());
  await page.goto(`/properties/${APARTMENT_ID}/participants`);
  const header = screenHeader(page);

  await expect(header.getByText('Участники объекта')).toBeVisible();
  // Ряды без manage-прав статичные (страница участника вне скоупа
  // зрителя) — проверяем текстами, не role=button.
  await expect(page.getByText('Иван Иванов')).toBeVisible();
  // Зритель видит себя с отметкой «(Вы)»; manage-контролов нет.
  await expect(page.getByText('Сергей Сидоров (Вы)')).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Пригласить участника' }).filter({ hasText: 'Пригласить участника' }),
  ).toHaveCount(0);

  // Кебаб приглашённого (D6): только «История объекта».
  await header.getByRole('button', { name: 'Еще — действия со списком' }).click();
  await expect(page.getByRole('menuitem', { name: 'История объекта' })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'Пригласить участника' })).toHaveCount(0);
  await expect(page.getByRole('menuitem', { name: 'Отозвать доступ всем' })).toHaveCount(0);
  await page.keyboard.press('Escape');

  // «Покинуть объект» вместо кнопки приглашения.
  await expect(page.getByRole('button', { name: 'Покинуть объект' })).toBeVisible();

  // Скриншот-сверка вида приглашённого (AC #1312: captureScreen).
  await captureScreen(page, testInfo, 'property-participants-invited');
});

test('«Покинуть объект» (D6): подтверждение, самовыход, серверная правда; сид восстанавливается', async ({ page }) => {
  await openCabinetWithSessionToken(page, seededViewerSessionToken());
  await page.goto(`/properties/${APARTMENT_ID}/participants`);

  await page.getByRole('button', { name: 'Покинуть объект' }).click();
  const dialog = page.getByRole('dialog');
  await expect(dialog.getByText('Уверены, что хотите покинуть объект?')).toBeVisible();
  await expect(
    dialog.getByText('Вы потеряете доступ к объекту пользователя. Попросить доступ можно будет снова'),
  ).toBeVisible();
  try {
    await dialog.getByRole('button', { name: 'Покинуть', exact: true }).click();

    // Возврат с экрана (канон #701) и серверная правда: нога Сергея снята.
    await expect(page).not.toHaveURL(new RegExp(`/properties/${APARTMENT_ID}/participants`));
    expect(
      await execE2eSql(
        `SELECT count(*) FROM property_members WHERE property_id = '${APARTMENT_ID}' AND user_id = '${SERGEY_ID}'`,
      ),
    ).toBe('0');
  } finally {
    await execE2eSql(
      `INSERT INTO property_members (id, property_id, user_id, role, granted_by) ` +
        `VALUES ('${SERGEY_MEMBER_ID}', '${APARTMENT_ID}', '${SERGEY_ID}', 'viewer', '11111111-1111-4111-8111-111111111111') ` +
        `ON CONFLICT (id) DO NOTHING`,
    );
  }
});

test('D15: на basic кебаб объекта без «Совместного доступа», paywall-редирект экрана жив; сид восстанавливается', async ({ page, seededUser }) => {
  const OWNER_SUBSCRIPTION_ID = '99999999-9999-4999-8999-999999999901';
  // Перевод владельца на basic (не удаление подписки: «подписки нет» —
  // 404 → null, и optimistic-умолчание фронта считает это платным).
  // Прежний тариф запоминаем — восстановление честное, не литерал 'pro'.
  const previousTariff = await execE2eSql(
    `SELECT t.name FROM user_subscriptions s JOIN tariffs t ON t.id = s.tariff_id WHERE s.id = '${OWNER_SUBSCRIPTION_ID}'`,
  );
  await execE2eSql(
    `UPDATE user_subscriptions SET tariff_id = (SELECT id FROM tariffs WHERE name = 'basic') ` +
      `WHERE id = '${OWNER_SUBSCRIPTION_ID}'`,
  );
  try {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`/properties/${APARTMENT_ID}`);

    // /subscription разрешился basic'ом: секция «Управление» отрисована,
    // строка пина скрыта (canPin=false) — кебаб считается тем же isPaid,
    // гонки нет.
    await expect(page.getByRole('button', { name: 'Редактировать объект' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Сделать основным' })).toHaveCount(0);

    // Кебаб владельца на basic: статусные мутации живы, шаринг скрыт
    // полностью (2235-100924 минус пункты шаринга).
    await screenHeader(page).getByRole('button', { name: 'Действия с объектом' }).click();
    await expect(page.getByRole('menuitem', { name: 'Об объекте' })).toBeVisible();
    await expect(page.getByRole('menuitem', { name: 'Изменить статус' })).toBeVisible();
    await expect(page.getByRole('menuitem', { name: 'Совместный доступ' })).toHaveCount(0);
    await page.keyboard.press('Escape');

    // Paywall-редиректы per-object экранов остаются как есть (ADR 0064):
    // deep-link на «Участники объекта» ведёт на выбор тарифа.
    await page.goto(`/properties/${APARTMENT_ID}/participants`);
    await expect(page).toHaveURL(/\/profile\/tariff\/change/);
  } finally {
    await execE2eSql(
      `UPDATE user_subscriptions SET tariff_id = (SELECT id FROM tariffs WHERE name = '${previousTariff}') ` +
        `WHERE id = '${OWNER_SUBSCRIPTION_ID}'`,
    );
  }
});
