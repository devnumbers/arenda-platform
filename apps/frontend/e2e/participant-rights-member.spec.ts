import {
  captureScreen,
  execE2eSql,
  expect,
  openCabinetWithSessionToken,
  screenHeader,
  seededMemberSessionToken,
  seededViewerSessionToken,
  test,
} from './fixtures';

// «Права участника» в member-режиме (#1362, решение владельца 12.10.2026):
// приглашённый участник объекта открывает страницу прав любого ряда
// «Участников объекта», включая свою строку и ряд владельца — read-only
// по контракту чтения строки #1357 (маршрут
// /properties/{id}/participants/{memberId}; агрегат /participants/{id}
// остаётся owner-only — D7-лок в participants-page.spec.ts). Сид: владелец
// «Квартиры на Ленина» (33333333-…) Иван Иванов, участники Мария Петрова
// (full_access, member …931) и Сергей Сидоров (viewer, member …932);
// сессии Марии и Сергея — E2E_MEMBER_ / E2E_VIEWER_SESSION_TOKEN.
// Pending-приглашение сеется с собственным id (утечка строк между спеками
// исключена, workers=1).

const APARTMENT_ID = '33333333-3333-4333-8333-333333333333';
const GARAGE_ID = '44444444-4444-4444-8444-444444444444';
const MARIA_MEMBER_ID = '99999999-9999-4999-8999-999999999931';
const SERGEY_MEMBER_ID = '99999999-9999-4999-8999-999999999932';
const OWNER_USER_ID = '11111111-1111-4111-8111-111111111111';
// Собственное pending-приглашение спека (не из participants-page.spec.ts:
// утечка строки одного теста не подменяет данные другого).
const PENDING_MEMBER_INVITE_ID = '99999999-9999-4999-8999-999999999974';
const PENDING_MEMBER_EMAIL = 'e2e-member-rights-pending@example.com';

function seedPendingMemberInvitation(role: 'viewer' | 'full_access'): Promise<string> {
  return execE2eSql(
    `INSERT INTO property_member_invitations (id, property_id, email, role, invited_by, last_sent_at) ` +
      `VALUES ('${PENDING_MEMBER_INVITE_ID}', '${APARTMENT_ID}', '${PENDING_MEMBER_EMAIL}', '${role}', '${OWNER_USER_ID}', now()) ` +
      `ON CONFLICT (id) DO UPDATE SET role = EXCLUDED.role, property_id = EXCLUDED.property_id, email = EXCLUDED.email`,
  );
}

test('member-режим прав: тап по чужому ряду — карточка объекта с ролью, строка истории, мутаций нет; история ведёт на пару', async ({ page }, testInfo) => {
  await openCabinetWithSessionToken(page, seededMemberSessionToken());
  await page.goto(`/properties/${APARTMENT_ID}/participants`);

  // Мария (full_access) тапает ряд Сергея (viewer) — member-режим прав.
  await page.getByRole('button', { name: /Сергей Сидоров/ }).click();

  await expect(screenHeader(page).getByText('Права участника')).toBeVisible();
  await expect(page).toHaveURL(
    new RegExp(`/properties/${APARTMENT_ID}/participants/${SERGEY_MEMBER_ID}$`),
  );

  // Карточка объекта с бейджем роли Сергея.
  await expect(page.getByText('Квартира на Ленина')).toBeVisible();
  await expect(page.getByText('Просмотр', { exact: true })).toBeVisible();

  // Read-only: сегмента роли и красного отзыва нет (мутации остаются
  // у владельца), инфо-блока D7 нет (копия адресована владельцу).
  await expect(page.getByRole('radio', { name: 'Просмотр' })).toHaveCount(0);
  await expect(page.getByRole('radio', { name: 'Управление' })).toHaveCount(0);
  await expect(page.getByText('Отозвать доступ')).toHaveCount(0);
  await expect(page.getByText('Пользователь еще не создал аккаунт')).toHaveCount(0);

  // Строка «Действия участника в объекте» — вход в пару Сергея.
  await expect(page.getByTestId('participant-property-actions')).toBeVisible();
  await page.getByTestId('participant-property-actions').click();
  await expect(page).toHaveURL(
    new RegExp(
      `/history/participants/13111111-1111-4111-8111-111111111131/properties/${APARTMENT_ID}`,
    ),
  );

  await captureScreen(page, testInfo, 'participant-rights-member-viewer');
});

test('своя строка: тап по «Мария (Вы)» ведёт на read-only права с ролью «Управление» (быстрый переход в историю)', async ({ page }) => {
  await openCabinetWithSessionToken(page, seededMemberSessionToken());
  await page.goto(`/properties/${APARTMENT_ID}/participants`);

  await page.getByRole('button', { name: /Мария \(Вы\)/ }).click();

  await expect(screenHeader(page).getByText('Права участника')).toBeVisible();
  await expect(page).toHaveURL(
    new RegExp(`/properties/${APARTMENT_ID}/participants/${MARIA_MEMBER_ID}$`),
  );
  await expect(page.getByText('Управление', { exact: true })).toBeVisible();
  await expect(page.getByTestId('participant-property-actions')).toBeVisible();
  await expect(page.getByRole('radio', { name: 'Управление' })).toHaveCount(0);
});

test('ряд владельца: бейдж «Владелец», строка истории жива — журнал действий владельца существует', async ({ page }) => {
  await openCabinetWithSessionToken(page, seededMemberSessionToken());
  await page.goto(`/properties/${APARTMENT_ID}/participants`);

  // Ряд владельца для Марии — «Иван Иванов» (серый суффикс «(Вы)» несёт
  // только её собственная строка).
  await page.getByRole('button', { name: /Иван Иванов/ }).click();

  await expect(screenHeader(page).getByText('Права участника')).toBeVisible();
  await expect(page.getByText('Владелец', { exact: true })).toBeVisible();
  await expect(page.getByTestId('participant-property-actions')).toBeVisible();
  await expect(page.getByRole('radio', { name: 'Управление' })).toHaveCount(0);
  await expect(page.getByText('Отозвать доступ')).toHaveCount(0);
});

test('deep-link и refresh у участника работают: прямой вход и перезагрузка показывают read-only права', async ({ page }) => {
  await openCabinetWithSessionToken(page, seededMemberSessionToken());
  // Критерий тикета: deep-link и refresh — чтение строки адресуется URL.
  await page.goto(`/properties/${APARTMENT_ID}/participants/${SERGEY_MEMBER_ID}`);

  await expect(screenHeader(page).getByText('Права участника')).toBeVisible();
  await expect(page.getByText('Просмотр', { exact: true })).toBeVisible();

  await page.reload();
  await expect(screenHeader(page).getByText('Права участника')).toBeVisible();
  await expect(page.getByText('Просмотр', { exact: true })).toBeVisible();
  await expect(page.getByRole('radio', { name: 'Просмотр' })).toHaveCount(0);
});

test('pending-ряд: карточка «Аккаунт не создан» без строк действий; сид восстанавливается', async ({ page }) => {
  await seedPendingMemberInvitation('viewer');
  try {
    await openCabinetWithSessionToken(page, seededMemberSessionToken());
    await page.goto(`/properties/${APARTMENT_ID}/participants`);

    await page.getByRole('button', { name: new RegExp(PENDING_MEMBER_EMAIL) }).click();

    await expect(screenHeader(page).getByText('Права участника')).toBeVisible();
    // Карточка та же, что у D7: анонимный титул #1105, почта, бейдж.
    const card = page.getByTestId('participant-rights-card');
    await expect(card.getByText('Пользователь', { exact: true })).toBeVisible();
    await expect(card.getByText(PENDING_MEMBER_EMAIL)).toBeVisible();
    await expect(card.getByText('Аккаунт не создан')).toBeVisible();

    // Строк действий нет: uuid юзера не существует, роли member не меняет,
    // отзывать нечем; чеврона на страницу участника нет (агрегат
    // owner-only).
    await expect(page.getByTestId('participant-property-actions')).toHaveCount(0);
    await expect(page.getByRole('radio', { name: 'Просмотр' })).toHaveCount(0);
    await expect(page.getByText('Отозвать доступ')).toHaveCount(0);
  } finally {
    await execE2eSql(
      `DELETE FROM property_member_invitations WHERE id = '${PENDING_MEMBER_INVITE_ID}'`,
    );
  }
});

test('карточка объекта — ссылка на объект (амендмент владельца 13.10.2026): тап ведёт в хаб объекта, вид прежний', async ({ page }, testInfo) => {
  await openCabinetWithSessionToken(page, seededMemberSessionToken());
  await page.goto(`/properties/${APARTMENT_ID}/participants/${SERGEY_MEMBER_ID}`);

  await expect(screenHeader(page).getByText('Права участника')).toBeVisible();

  // Карточка объекта — настоящая ссылка на хаб объекта (без шеврона:
  // решение владельца — вид карточки не меняется).
  const card = page.getByRole('link', { name: /Квартира на Ленина/ });
  await expect(card).toBeVisible();
  await expect(card).toHaveAttribute('href', `/properties/${APARTMENT_ID}`);
  await expect(card.getByText('Просмотр', { exact: true })).toBeVisible();
  await captureScreen(page, testInfo, 'participant-rights-member-card-link');

  await card.click();
  await page.waitForURL(`**/properties/${APARTMENT_ID}`);
  await expect(page.locator('header')).toBeVisible();
});

test('мусорный memberId при читаемом объекте — «Участник не найден»', async ({ page }) => {
  await openCabinetWithSessionToken(page, seededMemberSessionToken());
  await page.goto(
    `/properties/${APARTMENT_ID}/participants/00000000-0000-4000-8000-000000000000`,
  );

  await expect(page.getByText('Участник не найден')).toBeVisible();
  // Назад в шапке жив (§7), мутаций нет.
  await expect(screenHeader(page).getByRole('button', { name: 'Назад' })).toBeVisible();
  await expect(page.getByRole('radio', { name: 'Просмотр' })).toHaveCount(0);
});

test('чужой объект (Мария не участник гаража) — «Объект не найден или у вас нет к нему доступа»', async ({ page }) => {
  await openCabinetWithSessionToken(page, seededMemberSessionToken());
  // Строка Сергея адресована в скоупе гаража, к которому у Марии доступа
  // нет: бэк отвечает приватным 404 на оба чтения — канон #719.
  await page.goto(`/properties/${GARAGE_ID}/participants/${SERGEY_MEMBER_ID}`);

  await expect(
    page.getByText('Объект не найден или у вас нет к нему доступа'),
  ).toBeVisible();
});

test('зритель ведёт ряды туда же: тап по Марии — read-only права «Управление» без мутаций', async ({ page }) => {
  await openCabinetWithSessionToken(page, seededViewerSessionToken());
  await page.goto(`/properties/${APARTMENT_ID}/participants`);

  await page.getByRole('button', { name: /Мария Петрова/ }).click();

  await expect(screenHeader(page).getByText('Права участника')).toBeVisible();
  await expect(page.getByText('Управление', { exact: true })).toBeVisible();
  await expect(page.getByRole('radio', { name: 'Управление' })).toHaveCount(0);
  await expect(page.getByText('Отозвать доступ')).toHaveCount(0);
});
