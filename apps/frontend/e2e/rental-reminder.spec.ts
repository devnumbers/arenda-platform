import {
  captureScreen,
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_STUDIO_PROPERTY_ID,
  test,
  type SeededUser,
} from './fixtures';
import type { Page } from '@playwright/test';

// Шаг «Настройки аренды» визарда (#826, карта #822; Figma 1428-58757):
// селект «За сколько напоминать» с предвыбранным «За 1 день», видимый
// независимо от тумблера автоплатежа, и тумблер «Включить уведомления об
// оплате на почту» — шоткат глобальной категории «Платежи и операции».
// Выбранный оффал протекает в создаваемый арендой платёж 1:1 — проверяем
// и по API read-back аренды, и по SQL-правде payments.reminder_offset_days.
// Объекты: создающие сценарии едут по студии СЕРИЙНО — вторая незавершённая
// аренда на объекте невозможна (409), а квартира держит пины лент платежей
// и гараж — пины пустых состояний; созданное сносится за собой, перед
// сценарием — предочистка хвостов упавшего прогона (DELETE аренды сносит
// и её платёж, доступен только незапущенной — начало завтра).

type RentalsFromApi = {
  items: Array<{
    id: string;
    status: string;
    rentPayment: {
      paymentId: string;
      autoPay: boolean;
      reminderOffsetDays?: number | null;
    };
  }>;
};

async function fetchRentals(page: Page, propertyId: string): Promise<RentalsFromApi['items']> {
  const response = await page.request.get(`/api/properties/${propertyId}/rentals`);
  expect(response.ok()).toBe(true);
  return ((await response.json()) as RentalsFromApi).items;
}

/** Завтра по UTC (пояс e2e-контура): дата начала будущего — DELETE аренды
 * доступен только незапущенной (ErrRentalStarted), очистка сценария на
 * ней строится. */
function tomorrow(): Date {
  return new Date(Date.now() + 24 * 60 * 60 * 1000);
}

/** Предочистка: падение предыдущего прогона оставляет незавершённую аренду
 * (вторая на объекте невозможна — 409), сносим её через API — DELETE
 * доступен незапущенной аренде и сносит её платёж. */
async function cleanupRentals(page: Page, propertyId: string): Promise<void> {
  for (const rental of await fetchRentals(page, propertyId)) {
    const response = await page.request.delete(
      `/api/properties/${propertyId}/rentals/${rental.id}`,
    );
    expect(response.ok()).toBe(true);
  }
}

/** Пройти шаги 1–2 визарда аренды до настроек: сумма, день оплаты 10-е,
 * начало — завтра. */
async function passToSettings(
  page: Page,
  user: SeededUser,
  propertyId: string,
): Promise<void> {
  await openCabinetWithSeededSession(page, user);
  await cleanupRentals(page, propertyId);
  await page.goto(`/properties/${propertyId}/rentals/new`);
  await expect(page.getByRole('heading', { name: 'Цена и число оплаты' })).toBeVisible();
  await page.getByRole('textbox', { name: 'Арендная плата, рублей' }).fill('45000');

  await page.getByRole('button', { name: 'День оплаты: Выбрать день' }).click();
  const dayDialog = page.getByRole('dialog', { name: 'Выбор дня' });
  await dayDialog.getByRole('button', { name: '10', exact: true }).click();
  await dayDialog.getByRole('button', { name: 'Выбрать' }).click();
  await page.getByRole('button', { name: 'Продолжить' }).click();

  await expect(page.getByRole('heading', { name: 'Условия аренды' })).toBeVisible();
  await page.getByRole('button', { name: 'Начало аренды: Выбрать дату' }).click();
  const date = tomorrow();
  const monthLabel = date
    .toLocaleDateString('ru-RU', { month: 'long' })
    .replace(/^./, (ch) => ch.toUpperCase());
  const monthSection = page
    .locator('section')
    .filter({ has: page.getByRole('heading', { name: `${monthLabel}, ${date.getFullYear()}` }) });
  await monthSection.getByRole('button', { name: String(date.getDate()), exact: true }).click();
  // exact: триггеры полей «…: Выбрать дату» мечатся подстрокой «Выбрать».
  await page.getByRole('button', { name: 'Выбрать', exact: true }).click();

  await page.getByRole('button', { name: 'Далее' }).click();
  await expect(page.getByRole('heading', { name: 'Настройки аренды' })).toBeVisible();
}

test.describe.serial('настройки аренды: «За сколько напоминать»', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('дефолт «За 1 день» предвыбран и протекает в создаваемый платёж', async ({
    page,
    seededUser,
  }, testInfo) => {
    await passToSettings(page, seededUser, SEEDED_STUDIO_PROPERTY_ID);

    // Селект предвыбран «За 1 день» (решение #823), тумблеры рядом:
    // автоплатёж выключен, почта видна — блоки сосуществуют по макету.
    await expect(
      page.getByRole('button', { name: 'За сколько напоминать: За 1 день' }),
    ).toBeVisible();
    await expect(
      page.getByRole('switch', { name: 'Сделать платеж автоматическим' }),
    ).toHaveAttribute('aria-checked', 'false');
    await expect(
      page.getByRole('switch', { name: 'Включить уведомления об оплате на почту' }),
    ).toBeVisible();
    await captureScreen(page, testInfo, 'rental-settings-reminder-default');

    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Создать аренду' }).click();
    await expect(page.getByRole('heading', { name: 'Вы создали аренду' })).toBeVisible();

    const rentals = await fetchRentals(page, SEEDED_STUDIO_PROPERTY_ID);
    const rental = rentals[0];
    expect(rental).toBeDefined();
    if (rental === undefined) return;
    expect(rental.rentPayment.reminderOffsetDays).toBe(1);
    expect(rental.rentPayment.autoPay).toBe(false);

    // SQL-правда: оффал лежит в правиле платежа аренды.
    await expect
      .poll(() =>
        execE2eSql(
          `SELECT reminder_offset_days FROM payments WHERE id = '${rental.rentPayment.paymentId}'`,
        ),
      )
      .toBe('1');

    const cleanup = await page.request.delete(
      `/api/properties/${SEEDED_STUDIO_PROPERTY_ID}/rentals/${rental.id}`,
    );
    expect(cleanup.ok()).toBe(true);
  });

  test('«За 3 дня» выбирается в шите, виден при включённом автоплатеже и доезжает до платежа', async ({
    page,
    seededUser,
  }) => {
    await passToSettings(page, seededUser, SEEDED_STUDIO_PROPERTY_ID);

    // Автоплатёж включён — селект остаётся на экране (напоминание живёт
    // независимо от auto_pay, решение #823).
    const autoPay = page.getByRole('switch', { name: 'Сделать платеж автоматическим' });
    await autoPay.click();
    await expect(autoPay).toHaveAttribute('aria-checked', 'true');
    await expect(
      page.getByRole('button', { name: 'За сколько напоминать: За 1 день' }),
    ).toBeVisible();

    // Мобильная канва — шит с радио-опциями; выбор применяется сразу,
    // шит закрывается Escape.
    await page.getByRole('button', { name: 'За сколько напоминать: За 1 день' }).click();
    const sheet = page.getByRole('radiogroup', { name: 'За сколько напоминать' });
    await expect(sheet.getByRole('radio', { name: 'За 1 день' })).toHaveAttribute(
      'aria-checked',
      'true',
    );
    await sheet.getByRole('radio', { name: 'За 3 дня' }).click();
    await page.keyboard.press('Escape');

    await expect(
      page.getByRole('button', { name: 'За сколько напоминать: За 3 дня' }),
    ).toBeVisible();

    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Создать аренду' }).click();
    await expect(page.getByRole('heading', { name: 'Вы создали аренду' })).toBeVisible();

    const rentals = await fetchRentals(page, SEEDED_STUDIO_PROPERTY_ID);
    const rental = rentals[0];
    expect(rental).toBeDefined();
    if (rental === undefined) return;
    expect(rental.rentPayment.reminderOffsetDays).toBe(3);
    expect(rental.rentPayment.autoPay).toBe(true);

    await expect
      .poll(() =>
        execE2eSql(
          `SELECT reminder_offset_days FROM payments WHERE id = '${rental.rentPayment.paymentId}'`,
        ),
      )
      .toBe('3');

    const cleanup = await page.request.delete(
      `/api/properties/${SEEDED_STUDIO_PROPERTY_ID}/rentals/${rental.id}`,
    );
    expect(cleanup.ok()).toBe(true);
  });

  test('тумблер «Включить уведомления об оплате на почту» — шоткат категории «Платежи и операции»', async ({
    page,
    seededUser,
  }) => {
    // Тот же шоткат, что на шаге 4 платежей (#825): значение и запись —
    // глобальная email-матрица аккаунта. Мок stateful: refetch после PUT
    // возвращает сохранённое состояние.
    let current = { rental: true, payments_operations: true, tasks: true, shared_access: true };
    let savedCategories: typeof current | undefined;
    await page.route('**/api/notification-preferences', async (route) => {
      if (route.request().method() === 'PUT') {
        savedCategories = (route.request().postDataJSON() as { email: typeof current }).email;
        current = { ...savedCategories };
        await route.fulfill({ json: { email: current } });
        return;
      }
      await route.fulfill({ json: { email: current } });
    });

    await passToSettings(page, seededUser, SEEDED_APARTMENT_PROPERTY_ID);

    const toggle = page.getByRole('switch', { name: 'Включить уведомления об оплате на почту' });
    await expect(toggle).toHaveAttribute('aria-checked', 'true');
    await toggle.click();
    await expect(toggle).toHaveAttribute('aria-checked', 'false');
    await expect
      .poll(() => savedCategories?.payments_operations, { timeout: 5_000 })
      .toBe(false);
  });
});
