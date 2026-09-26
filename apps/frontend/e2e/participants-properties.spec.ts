import type { Page } from '@playwright/test';
import {
  captureScreen,
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  openCabinetWithSessionToken,
  screenHeader,
  seededMemberSessionToken,
  test,
} from './fixtures';

// Экран «Объекты пользователей» (карта #692, тикет #701): чужие объекты
// читающего из GET /properties (access.role ≠ owner), бейджи ролей, чип
// «Название», кебаб карточки → шит «Действия с объектом» → «Покинуть
// объект», кебаб шапки → «История действий» (#843) и «Покинуть все
// объекты». Сид: Мария Петрова
// (e2e-member) — full_access на «Квартире на Ленина» владельца Ивана
// Иванова; у самого Ивана чужих объектов нет (пустое состояние).

const APARTMENT_ROW = /Квартира на Ленина/;
const MEMBER_MEMBERSHIP_ID = '99999999-9999-4999-8999-999999999931';

async function openAsMember(page: Page): Promise<void> {
  await openCabinetWithSessionToken(page, seededMemberSessionToken());
  await page.goto('/participants/properties');
}

/** Сид после разрушающих тестов: membership Марии возвращается (workers=1 —
 * дальше сид нужен payment-edit-delete: кабинет сид-участников). */
async function restoreMemberMembership(): Promise<void> {
  await execE2eSql(
    "INSERT INTO property_members (id, property_id, user_id, role, granted_by) VALUES " +
      `('${MEMBER_MEMBERSHIP_ID}', '33333333-3333-4333-8333-333333333333', '12111111-1111-4111-8111-111111111121', 'full_access', '11111111-1111-4111-8111-111111111111') ` +
      'ON CONFLICT (id) DO NOTHING',
  );
}

test('список: чужой объект с бейджем роли «Редактирование», чип «Название»', async ({ page }, testInfo) => {
  await openAsMember(page);

  const header = screenHeader(page);

  await expect(header.getByText('Доступные объекты')).toBeVisible();

  const apartmentRow = page.getByText(APARTMENT_ROW).first();
  await expect(apartmentRow).toBeVisible();
  await expect(page.getByText('Москва, ул. Ленина, 1')).toBeVisible();
  await expect(page.getByText('Редактирование')).toBeVisible();

  await expect(page.getByRole('button', { name: 'Название' })).toBeVisible();
  await expect(header.getByRole('button', { name: 'Еще — действия со списком' })).toBeVisible();
  await expect(
    page.getByRole('button', { name: 'Действия с объектом «Квартира на Ленина»' }),
  ).toBeVisible();

  await captureScreen(page, testInfo, 'participants-properties');
});

test('владелец без чужих объектов: пустое состояние без чипа и кебабов', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants/properties');

  await expect(page.getByText('Вас не пригласили в объекты')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Название' })).toHaveCount(0);
  await expect(
    screenHeader(page).getByRole('button', { name: 'Еще — действия со списком' }),
  ).toHaveCount(0);
});

test('кебаб карточки: шит действий, «Отмена» в подтверждении ничего не делает', async ({ page }) => {
  await openAsMember(page);
  await page.getByRole('button', { name: 'Действия с объектом «Квартира на Ленина»' }).click();

  // Шит (2010-133846): карточка объекта, ряд владельца, статус доступа.
  const sheet = page.getByRole('dialog');
  await expect(sheet.getByText('Действия с объектом')).toBeVisible();
  await expect(sheet.getByText(APARTMENT_ROW).first()).toBeVisible();
  await expect(sheet.getByText('Иван Иванов')).toBeVisible();
  await expect(sheet.getByText('Владелец объекта')).toBeVisible();
  await expect(sheet.getByText('Вам доступно редактирование')).toBeVisible();

  await sheet.getByRole('button', { name: 'Покинуть объект', exact: true }).click();

  // Подтверждение (2010-132970): кнопки в ряд, «Отмена» закрывает без DELETE.
  const confirm = page.getByRole('dialog');
  await expect(confirm.getByText('Уверены, что хотите покинуть объект?')).toBeVisible();
  await confirm.getByRole('button', { name: 'Отмена', exact: true }).click();
  await expect(page.getByRole('dialog')).toHaveCount(0);
  await expect(page.getByText(APARTMENT_ROW).first()).toBeVisible();

  const count = await execE2eSql(
    `SELECT count(*) FROM property_members WHERE id = '${MEMBER_MEMBERSHIP_ID}'`,
  );
  expect(count).toBe('1');
});

test('«Покинуть объект»: доступ снят, попап успеха, сид восстанавливается', async ({ page }) => {
  await openAsMember(page);
  await page.getByRole('button', { name: 'Действия с объектом «Квартира на Ленина»' }).click();

  try {
    const sheet = page.getByRole('dialog');
    await sheet.getByRole('button', { name: 'Покинуть объект', exact: true }).click();

    const confirm = page.getByRole('dialog');
    await confirm.getByRole('button', { name: 'Покинуть', exact: true }).click();

    // Попап (2010-133204); список под ним перечитан инвалидацией properties.
    await expect(
      page.getByRole('dialog').locator('p', { hasText: 'Вы покинули объект' }),
    ).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByText('Вас не пригласили в объекты')).toBeVisible();

    // Серверная правда: membership удалён самовыходом.
    const count = await execE2eSql(
      `SELECT count(*) FROM property_members WHERE id = '${MEMBER_MEMBERSHIP_ID}'`,
    );
    expect(count).toBe('0');
  } finally {
    await restoreMemberMembership();
  }
  await page.reload();
  await expect(page.getByText(APARTMENT_ROW).first()).toBeVisible();
});

test('кебаб шапки: «История действий» ведёт в общую ленту, «Покинуть все объекты» — подтверждение', async ({ page }) => {
  await openAsMember(page);

  const header = screenHeader(page);

  try {
    await header.getByRole('button', { name: 'Еще — действия со списком' }).click();

    // «История действий» (#843) — вход приглашённого участника в общую
    // ленту (решение владельца 24.09); красный пункт остаётся в кебабе.
    await expect(page.getByRole('menuitem', { name: 'Покинуть все объекты' })).toBeVisible();
    await page.getByRole('menuitem', { name: 'История действий' }).click();
    await page.waitForURL('**/history');
    await page.goBack();

    await header.getByRole('button', { name: 'Еще — действия со списком' }).click();
    await page.getByRole('menuitem', { name: 'Покинуть все объекты' }).click();

    // Подтверждение (2010-133563): кнопки столбиком, подтверждение сверху.
    const confirm = page.getByRole('dialog');
    await expect(confirm.getByText('Уверены, что хотите покинуть все объекты?')).toBeVisible();
    const leaveAllButton = confirm.getByRole('button', { name: 'Покинуть все объекты', exact: true });
    const cancelButton = confirm.getByRole('button', { name: 'Отмена', exact: true });
    await expect(leaveAllButton).toBeVisible();
    await expect(cancelButton).toBeVisible();
    const leaveBox = await leaveAllButton.boundingBox();
    const cancelBox = await cancelButton.boundingBox();
    expect(leaveBox?.y ?? 0).toBeLessThan(cancelBox?.y ?? 0);
    await leaveAllButton.click();

    await expect(
      page.getByRole('dialog').locator('p', { hasText: 'Вы покинули все объекты пользователей' }),
    ).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByText('Вас не пригласили в объекты')).toBeVisible();

    const count = await execE2eSql(
      `SELECT count(*) FROM property_members WHERE id = '${MEMBER_MEMBERSHIP_ID}'`,
    );
    expect(count).toBe('0');
  } finally {
    await restoreMemberMembership();
  }
  await page.reload();
  await expect(page.getByText(APARTMENT_ROW).first()).toBeVisible();
});
