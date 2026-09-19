import type { Page } from '@playwright/test';
import {
  captureScreen,
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Карточки хаба «Объекты» карты «Совместный доступ 2.0» (тикет #702; поля
// карточек пересмотрены решением владельца по итогам обхода #756 — ряд
// участников своих объектов снесён, ряд владельца носят только ЧУЖИЕ
// карточки) и блюр-карточки подвесших чужих объектов с шитом причины
// «Превышен лимит объектов». Suspended-доступ владельца Ивана на чужой
// «Даче у Марии» грантуется через execE2eSql (workers=1) и убирается в
// конце каждого теста. Выход с подвесшего доступа разрешён правилом #702
// (раньше ErrCannotLeaveSuspended).

const OWNER_ID = '11111111-1111-4111-8111-111111111111';
const FOREIGN_OWNER_ID = '12111111-1111-4111-8111-111111111121';
const SUSPENDED_PROPERTY_ID = '34444444-4444-4444-8444-444444444444';
const SUSPENDED_MEMBERSHIP_ID = '99999999-9999-4999-8999-999999999939';

async function grantSuspendedAccess(): Promise<void> {
  await execE2eSql(
    "INSERT INTO properties (id, owner_id, name, type, address, description, attributes, status) VALUES " +
      `('${SUSPENDED_PROPERTY_ID}', '${FOREIGN_OWNER_ID}', 'Дача у Марии', 'house', 'Москва, ул. Дачная, 5', '', '{}', 'active') ` +
      'ON CONFLICT (id) DO NOTHING',
  );
  await execE2eSql(
    "INSERT INTO property_members (id, property_id, user_id, role, granted_by, status, suspended_at) VALUES " +
      `('${SUSPENDED_MEMBERSHIP_ID}', '${SUSPENDED_PROPERTY_ID}', '${OWNER_ID}', 'viewer', '${FOREIGN_OWNER_ID}', 'suspended', now()) ` +
      'ON CONFLICT (id) DO NOTHING',
  );
}

async function revokeSuspendedAccess(): Promise<void> {
  await execE2eSql(
    `DELETE FROM property_members WHERE id = '${SUSPENDED_MEMBERSHIP_ID}'`,
  );
  await execE2eSql(`DELETE FROM properties WHERE id = '${SUSPENDED_PROPERTY_ID}'`);
}

async function openHub(
  page: Page,
  seededUser: Parameters<typeof openCabinetWithSeededSession>[1],
): Promise<void> {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties');
}

test('свои карточки без ряда участников и без бейджа роли', async ({ page, seededUser }, testInfo) => {
  await openHub(page, seededUser);

  // Решение владельца по итогам обхода #756: на своих карточках ряда нет
  // вовсе — ни участников (#702 снесён), ни владельца, ни бейджа роли.
  const apartmentCard = page
    .locator('li')
    .filter({ has: page.getByRole('heading', { name: 'Квартира на Ленина' }) });
  await expect(apartmentCard.getByTestId('property-participants')).toHaveCount(0);
  await expect(apartmentCard.getByTestId('property-owner')).toHaveCount(0);
  await expect(apartmentCard.getByText('Мария Петрова')).toHaveCount(0);
  await expect(apartmentCard.getByText('Редактирование')).toHaveCount(0);

  await captureScreen(page, testInfo, 'properties-own-cards-plain');
});

test('блюр-карточка подвесшего объекта и шит причины', async ({ page, seededUser }, testInfo) => {
  await grantSuspendedAccess();
  try {
    await openHub(page, seededUser);

    // Блюр-карточка после своих карточек: настоящая карточка объекта под
    // блюром с теми же полями, что у любой чужой карточки (название, адрес
    // и ряд владельца — Figma 2213-99113 + решение по обходу #756), и
    // плашка «Узнать причину».
    const blurCard = page.getByTestId('suspended-property-card');
    await expect(blurCard.getByText('Дача у Марии')).toBeVisible();
    await expect(blurCard.getByText('Москва, ул. Дачная, 5')).toBeVisible();
    await expect(blurCard.getByTestId('property-owner').getByText('Мария Петрова')).toBeVisible();
    await expect(blurCard.getByText('Объект недоступен')).toBeVisible();
    await expect(blurCard.getByText('Узнать причину')).toBeVisible();

    await blurCard.click();
    await expect(page.getByTestId('suspended-reason-sheet').getByText('Превышен лимит объектов')).toBeVisible();
    await expect(page.getByTestId('suspended-reason-sheet').getByText('Владелец объекта')).toBeVisible();
    await expect(page.getByTestId('suspended-reason-sheet').getByText('Мария Петрова')).toBeVisible();
    // Почта владельца — сознательная экспозиция шита причины (2229-100002).
    await expect(page.getByTestId('suspended-reason-sheet').getByText('e2e-member@example.com')).toBeVisible();
    await expect(page.getByTestId('suspended-reason-sheet').getByText('Выбрать тариф')).toBeVisible();

    await captureScreen(page, testInfo, 'properties-suspended-reason');

    await page.getByTestId('suspended-close').click();
    await expect(page.getByTestId('suspended-reason-sheet')).toBeHidden();
  } finally {
    await revokeSuspendedAccess();
  }
});

test('«Покинуть объект» из шита: suspended-доступ снят (серверная правда)', async ({ page, seededUser }) => {
  await grantSuspendedAccess();
  try {
    await openHub(page, seededUser);

    await page.getByTestId('suspended-property-card').click();
    await page.getByTestId('suspended-leave').click();

    // Плейсхолдер уходит после инвалидации (useLeaveProperty, #701).
    await expect(page.getByTestId('suspended-property-card')).toBeHidden();

    const count = await execE2eSql(
      `SELECT count(*) FROM property_members WHERE id = '${SUSPENDED_MEMBERSHIP_ID}'`,
    );
    expect(count).toBe('0');
  } finally {
    await revokeSuspendedAccess();
  }
});

test('«Выбрать тариф» ведёт на смену тарифа', async ({ page, seededUser }) => {
  await grantSuspendedAccess();
  try {
    await openHub(page, seededUser);

    await page.getByTestId('suspended-property-card').click();
    await page.getByTestId('suspended-choose-tariff').click();

    await expect(page).toHaveURL(/\/profile\/tariff\/change$/);
  } finally {
    await revokeSuspendedAccess();
  }
});
