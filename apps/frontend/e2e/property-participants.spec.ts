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

// Экран «Участники объекта» (карта #692, тикет #700): замена легаси-
// модалки PropertySharingModal — список с владельцем отдельным рядом,
// поиск, чип-фильтр ролей, кебаб «Отозвать доступ всем», CTA приглашения
// от объекта. Сид: владелец «Квартиры на Ленина» (33333333-…) Иван Иванов
// с двумя участниками — Мария Петрова (full_access, member …931) и Сергей
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

test('manage-участник: своя строка «(Вы)» инертная, чужие ряды ведут на агрегат (#770)', async ({ page }) => {
  // Мария — full_access на чужой «Квартире на Ленина»: manage-права есть,
  // но своей ноги в её manage-скоупе нет — /participants/{себя} был бы 404.
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

  // Чужие ряды manage-скоупа кликабельны, как и раньше.
  await page.getByRole('button', { name: /Сергей Сидоров/ }).click();
  await expect(header.getByText('Участник', { exact: true }).first()).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Сергей Сидоров' })).toBeVisible();
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
    await expect(page.getByText(INVITE_EMAIL).first()).toBeVisible();

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

    // Серверная правда: членовств на объекте нет; сид возвращается.
    expect(
      await execE2eSql(
        `SELECT count(*) FROM property_members WHERE property_id = '${APARTMENT_ID}'`,
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

test('зритель: список без manage-контролов, свой ряд с «(Вы)»', async ({ page }) => {
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
  await expect(
    header.getByRole('button', { name: 'Еще — действия со списком' }),
  ).toHaveCount(0);
});
