import {
  captureScreen,
  execE2eSql,
  expect,
  mockEmailCategoryShortcut,
  openCabinetWithSeededSession,
  pickCalendarDay,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_STUDIO_PROPERTY_ID,
  test,
  type SeededUser,
} from './fixtures';
import type { Page } from '@playwright/test';

// Шаг «Настройки аренды» визарда (#826, карта #822; Figma 1428-58757):
// селект «За сколько напоминать» — #1198 (решение владельца 07.10): опции
// «Не напоминать / За 1 день / За 3 дня / За 7 дней», дефолт «Не напоминать»
// (null едет в команду явно), видимый независимо от тумблера автоплатежа;
// и тумблер «Включить уведомления об оплате на почту» — шоткат глобальной
// категории «Платежи и операции». Выбранный оффсет протекает в создаваемый
// арендой платёж 1:1 — проверяем и по API read-back аренды, и по SQL-правде
// payments.reminder_offset_days; там же SQL-правда бекенд-хвоста #1198 —
// payments.notify_auto_paid ставится true безусловно.
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
      notifyEmail?: boolean;
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
  await pickCalendarDay(page, date);
  // exact: триггеры полей «…: Выбрать дату» мечатся подстрокой «Выбрать».
  await page.getByRole('button', { name: 'Выбрать', exact: true }).click();

  await page.getByRole('button', { name: 'Далее' }).click();
  await expect(page.getByRole('heading', { name: 'Настройки аренды' })).toBeVisible();
}

test.describe.serial('настройки аренды: «За сколько напоминать»', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('дефолт «Не напоминать» предвыбран, в платёж едут null и notify_auto_paid=true (#1198)', async ({
    page,
    seededUser,
  }, testInfo) => {
    await passToSettings(page, seededUser, SEEDED_STUDIO_PROPERTY_ID);

    // Селект предвыбран «Не напоминать» (#1198), тумблеры рядом:
    // автоплатёж выключен, почта видна — блоки сосуществуют по макету.
    await expect(
      page.getByRole('button', { name: 'За сколько напоминать: Не напоминать' }),
    ).toBeVisible();
    await expect(
      page.getByRole('switch', { name: 'Сделать платеж автоматическим' }),
    ).toHaveAttribute('aria-checked', 'false');
    await expect(
      page.getByRole('switch', { name: 'Включить уведомления об оплате на почту' }),
    ).toBeVisible();
    await captureScreen(page, testInfo, 'rental-settings-reminder-default');

    // Шит: «Не напоминать» первой и выбранной, за ней «за N дней» по макету.
    await page.getByRole('button', { name: 'За сколько напоминать: Не напоминать' }).click();
    const sheet = page.getByRole('radiogroup', { name: 'За сколько напоминать' });
    const labels = sheet.getByRole('radio');
    await expect(labels).toHaveText([
      'Не напоминать',
      'За 1 день',
      'За 3 дня',
      'За 7 дней',
    ]);
    await expect(sheet.getByRole('radio', { name: 'Не напоминать' })).toHaveAttribute(
      'aria-checked',
      'true',
    );
    await page.keyboard.press('Escape');

    // Дефолт не протекает в команду молча: сабмит без касания селекта шлёт
    // reminderOffsetDays: null — напоминаний нет.
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Создать аренду' }).click();
    await expect(page.getByRole('heading', { name: 'Вы создали аренду' })).toBeVisible();

    const rentals = await fetchRentals(page, SEEDED_STUDIO_PROPERTY_ID);
    const rental = rentals[0];
    expect(rental).toBeDefined();
    if (rental === undefined) return;
    // Ответ omitted-поле не несёт (omitempty) — «напоминаний нет» читается
    // как отсутствие значения, null-эквивалент.
    expect(rental.rentPayment.reminderOffsetDays ?? null).toBeNull();
    expect(rental.rentPayment.autoPay).toBe(false);

    // SQL-правда: оффсета в правиле нет (psql -tAc рисует NULL пустой
    // строкой), а гейт уведомления об автоплатеже стоит true безусловно —
    // бекенд-хвост #1198.
    await expect
      .poll(() =>
        execE2eSql(
          `SELECT reminder_offset_days FROM payments WHERE id = '${rental.rentPayment.paymentId}'`,
        ),
      )
      .toBe('');
    await expect
      .poll(() =>
        execE2eSql(
          `SELECT notify_auto_paid FROM payments WHERE id = '${rental.rentPayment.paymentId}'`,
        ),
      )
      .toBe('t');

    const cleanup = await page.request.delete(
      `/api/properties/${SEEDED_STUDIO_PROPERTY_ID}/rentals/${rental.id}`,
    );
    expect(cleanup.ok()).toBe(true);
  });

  test('«За 3 дня» выбирается в шите при включённом автоплатеже и доезжает до платежа', async ({
    page,
    seededUser,
  }) => {
    await passToSettings(page, seededUser, SEEDED_STUDIO_PROPERTY_ID);

    // Автоплатёж включён — селект остаётся на экране (напоминание живёт
    // независимо от auto_pay, решение #823), дефолт «Не напоминать» (#1198).
    const autoPay = page.getByRole('switch', { name: 'Сделать платеж автоматическим' });
    await autoPay.click();
    await expect(autoPay).toHaveAttribute('aria-checked', 'true');
    await expect(
      page.getByRole('button', { name: 'За сколько напоминать: Не напоминать' }),
    ).toBeVisible();

    // Мобильная канва — шит с радио-опциями; выбор применяется сразу,
    // шит закрывается Escape.
    await page.getByRole('button', { name: 'За сколько напоминать: Не напоминать' }).click();
    const sheet = page.getByRole('radiogroup', { name: 'За сколько напоминать' });
    await expect(sheet.getByRole('radio', { name: 'Не напоминать' })).toHaveAttribute(
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

    // SQL-правда: оффсет лежит в правиле платежа аренды; напоминание и при
    // автоплатеже не режется (арендный конвейер не форсит NULL, #823),
    // гейт уведомления об автоплатеже — true (#1198).
    await expect
      .poll(() =>
        execE2eSql(
          `SELECT reminder_offset_days FROM payments WHERE id = '${rental.rentPayment.paymentId}'`,
        ),
      )
      .toBe('3');
    await expect
      .poll(() =>
        execE2eSql(
          `SELECT notify_auto_paid FROM payments WHERE id = '${rental.rentPayment.paymentId}'`,
        ),
      )
      .toBe('t');

    const cleanup = await page.request.delete(
      `/api/properties/${SEEDED_STUDIO_PROPERTY_ID}/rentals/${rental.id}`,
    );
    expect(cleanup.ok()).toBe(true);
  });

  test('ряд «Включить уведомления об оплате на почту» — персональная галочка аренды, матрица не пишется (карта #1551)', async ({
    page,
    seededUser,
  }) => {
    // Двухуровневая модель (канон #1552): свитч — флаг формы этой аренды,
    // клик пишет только черновик; глобальная матрица аккаунта из шага 3
    // не пишется (шоткат #746 снят). Гейта от пикера напоминаний у аренды
    // нет — ряд жив при дефолтном «Не напоминать».
    const { puts } = await mockEmailCategoryShortcut(page);

    await passToSettings(page, seededUser, SEEDED_APARTMENT_PROPERTY_ID);

    const toggle = page.getByRole('switch', { name: 'Включить уведомления об оплате на почту' });
    await expect(toggle).toBeEnabled();
    await expect(toggle).toHaveAttribute('aria-checked', 'true');
    await expect(
      page.getByRole('button', { name: 'За сколько напоминать: Не напоминать' }),
    ).toBeVisible();

    await toggle.click();
    await expect(toggle).toHaveAttribute('aria-checked', 'false');
    // Матрица аккаунта из шага 3 не пишется.
    expect(puts()).toBe(0);

    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Создать аренду' }).click();
    await expect(page.getByRole('heading', { name: 'Вы создали аренду' })).toBeVisible();

    const rentals = await fetchRentals(page, SEEDED_APARTMENT_PROPERTY_ID);
    const rental = rentals[0];
    expect(rental).toBeDefined();
    if (rental === undefined) return;
    // Снятая галочка доезжает до контракта пер-арендным флагом (сервер
    // держит его на Платеже аренды, #1555).
    expect(rental.rentPayment.notifyEmail).toBe(false);
    await expect
      .poll(() =>
        execE2eSql(
          `SELECT notify_email FROM payments WHERE id = '${rental.rentPayment.paymentId}'`,
        ),
      )
      .toBe('f');

    const cleanup = await page.request.delete(
      `/api/properties/${SEEDED_APARTMENT_PROPERTY_ID}/rentals/${rental.id}`,
    );
    expect(cleanup.ok()).toBe(true);
  });

  test('мастер категории «Аренда» off — ряд скрыт, галочка в команду не едет', async ({
    page,
    seededUser,
  }) => {
    // Глобальный мастер — видимость ряда: off (или нечитаемый GET) — ряда
    // нет вовсе, команда сохранения галочку не пишет (дифф без поля,
    // канон #1552), сервер ставит дефолт true (#1555).
    await mockEmailCategoryShortcut(page, {
      initial: { rental: false, payments_operations: true, tasks: true, shared_access: true },
    });

    await passToSettings(page, seededUser, SEEDED_APARTMENT_PROPERTY_ID);

    // Ряд скрыт целиком — ни свитча, ни текстов (пока GET в полёте,
    // рендер прежний, после ответа схлопывается).
    const toggle = page.getByRole('switch', { name: 'Включить уведомления об оплате на почту' });
    await expect(toggle).toHaveCount(0);
    await expect(page.getByText('Включить уведомления об оплате на почту')).toHaveCount(0);

    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Создать аренду' }).click();
    await expect(page.getByRole('heading', { name: 'Вы создали аренду' })).toBeVisible();

    const rentals = await fetchRentals(page, SEEDED_APARTMENT_PROPERTY_ID);
    const rental = rentals[0];
    expect(rental).toBeDefined();
    if (rental === undefined) return;
    expect(rental.rentPayment.notifyEmail).toBe(true);
    await expect
      .poll(() =>
        execE2eSql(
          `SELECT notify_email FROM payments WHERE id = '${rental.rentPayment.paymentId}'`,
        ),
      )
      .toBe('t');

    const cleanup = await page.request.delete(
      `/api/properties/${SEEDED_APARTMENT_PROPERTY_ID}/rentals/${rental.id}`,
    );
    expect(cleanup.ok()).toBe(true);
  });

  test('правка условий меняет напоминание: 3 → 7 → «Не напоминать» (#1208)', async ({
    page,
    seededUser,
  }) => {
    // Аренда с «За 3 дня», затем правка через экран «Изменить условия»:
    // селект предзаполнен из read-back аренды, смена доезжает до платежа
    // (API + SQL) и пишет чип reminder_offset_days в журнал изменений
    // платежа; «Не напоминать» чистит оффсет явным null (tri-state).
    await passToSettings(page, seededUser, SEEDED_STUDIO_PROPERTY_ID);
    await page.getByRole('button', { name: 'За сколько напоминать: Не напоминать' }).click();
    await page
      .getByRole('radiogroup', { name: 'За сколько напоминать' })
      .getByRole('radio', { name: 'За 3 дня' })
      .click();
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Создать аренду' }).click();
    await expect(page.getByRole('heading', { name: 'Вы создали аренду' })).toBeVisible();

    const rentals = await fetchRentals(page, SEEDED_STUDIO_PROPERTY_ID);
    const rental = rentals[0];
    expect(rental).toBeDefined();
    if (rental === undefined) return;
    expect(rental.rentPayment.reminderOffsetDays).toBe(3);

    const paymentId = rental.rentPayment.paymentId;
    const chipCount = (from: string, to: string): string =>
      `SELECT count(*) FROM payment_change_log WHERE payment_id = '${paymentId}' ` +
      `AND changes @> '[{"field":"reminder_offset_days","old":${from},"new":${to}}]'::jsonb`;

    // Правка 3 → 7: селект предзаполнен «За 3 дня» (read-back #1208).
    await page.goto(`/properties/${SEEDED_STUDIO_PROPERTY_ID}/rentals/terms/edit`);
    await expect(page.getByRole('button', { name: 'За сколько напоминать: За 3 дня' })).toBeVisible();
    await page.getByRole('button', { name: 'За сколько напоминать: За 3 дня' }).click();
    await page
      .getByRole('radiogroup', { name: 'За сколько напоминать' })
      .getByRole('radio', { name: 'За 7 дней' })
      .click();
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: 'Сохранить изменения' }).click();
    // goBack — history.back(): после goto теста возврат уходит не на
    // «Условия аренды», ждём закрытия экрана правки, правду читаем по API.
    await page.waitForURL((url) => !url.pathname.endsWith('/rentals/terms/edit'));

    const afterSeven = await fetchRentals(page, SEEDED_STUDIO_PROPERTY_ID);
    expect(afterSeven[0]?.rentPayment.reminderOffsetDays).toBe(7);
    await expect
      .poll(() =>
        execE2eSql(`SELECT reminder_offset_days FROM payments WHERE id = '${paymentId}'`),
      )
      .toBe('7');
    await expect.poll(() => execE2eSql(chipCount('3', '7'))).toBe('1');

    // Правка 7 → «Не напоминать»: явный null чистит оффсет, чип несёт
    // old 7 → new null.
    await page.goto(`/properties/${SEEDED_STUDIO_PROPERTY_ID}/rentals/terms/edit`);
    await expect(page.getByRole('button', { name: 'За сколько напоминать: За 7 дней' })).toBeVisible();
    await page.getByRole('button', { name: 'За сколько напоминать: За 7 дней' }).click();
    await page
      .getByRole('radiogroup', { name: 'За сколько напоминать' })
      .getByRole('radio', { name: 'Не напоминать' })
      .click();
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: 'Сохранить изменения' }).click();
    await page.waitForURL((url) => !url.pathname.endsWith('/rentals/terms/edit'));

    const afterNone = await fetchRentals(page, SEEDED_STUDIO_PROPERTY_ID);
    expect(afterNone[0]?.rentPayment.reminderOffsetDays ?? null).toBeNull();
    await expect
      .poll(() =>
        execE2eSql(`SELECT reminder_offset_days FROM payments WHERE id = '${paymentId}'`),
      )
      .toBe('');
    await expect.poll(() => execE2eSql(chipCount('7', 'null'))).toBe('1');

    const cleanup = await page.request.delete(
      `/api/properties/${SEEDED_STUDIO_PROPERTY_ID}/rentals/${rental.id}`,
    );
    expect(cleanup.ok()).toBe(true);
  });

  test('правка условий меняет галочку почты: on → off → on, SQL-правда на Платеже (карта #1551)', async ({
    page,
    seededUser,
  }) => {
    // Ряд на правке условий (канон #1552: после пикера напоминания;
    // возвращённая часть решения #532): предзаполнение из флага аренды,
    // PATCH едет диффом — галочка меняется, напоминание в той же сессии
    // не трогается; сервер переносит флаг на Платёж (#1555).
    await mockEmailCategoryShortcut(page);
    await passToSettings(page, seededUser, SEEDED_STUDIO_PROPERTY_ID);
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Создать аренду' }).click();
    await expect(page.getByRole('heading', { name: 'Вы создали аренду' })).toBeVisible();

    const rentals = await fetchRentals(page, SEEDED_STUDIO_PROPERTY_ID);
    const rental = rentals[0];
    expect(rental).toBeDefined();
    if (rental === undefined) return;
    const paymentId = rental.rentPayment.paymentId;
    expect(rental.rentPayment.notifyEmail).toBe(true);

    const sqlFlag = (): Promise<string> =>
      execE2eSql(`SELECT notify_email FROM payments WHERE id = '${paymentId}'`);

    // Правка on → off: ряд предзаполнен включённым, переключение и
    // сохранение пишут PATCH (дифф без reminderOffsetDays).
    await page.goto(`/properties/${SEEDED_STUDIO_PROPERTY_ID}/rentals/terms/edit`);
    const toggle = page.getByRole('switch', { name: 'Включить уведомления об оплате на почту' });
    await expect(toggle).toBeEnabled();
    await expect(toggle).toHaveAttribute('aria-checked', 'true');
    await toggle.click();
    await expect(toggle).toHaveAttribute('aria-checked', 'false');
    await page.getByRole('button', { name: 'Сохранить изменения' }).click();
    await page.waitForURL((url) => !url.pathname.endsWith('/rentals/terms/edit'));

    const afterOff = await fetchRentals(page, SEEDED_STUDIO_PROPERTY_ID);
    expect(afterOff[0]?.rentPayment.notifyEmail).toBe(false);
    await expect.poll(sqlFlag).toBe('f');

    // Правка off → on: предзаполнение из флага (off), возврат едет тем же
    // диффом; напоминание аренды остаётся «Не напоминать» — ряд независим.
    await page.goto(`/properties/${SEEDED_STUDIO_PROPERTY_ID}/rentals/terms/edit`);
    const toggleAgain = page.getByRole('switch', { name: 'Включить уведомления об оплате на почту' });
    await expect(toggleAgain).toHaveAttribute('aria-checked', 'false');
    await toggleAgain.click();
    await expect(toggleAgain).toHaveAttribute('aria-checked', 'true');
    await page.getByRole('button', { name: 'Сохранить изменения' }).click();
    await page.waitForURL((url) => !url.pathname.endsWith('/rentals/terms/edit'));

    const afterOn = await fetchRentals(page, SEEDED_STUDIO_PROPERTY_ID);
    expect(afterOn[0]?.rentPayment.notifyEmail).toBe(true);
    expect(afterOn[0]?.rentPayment.reminderOffsetDays ?? null).toBeNull();
    await expect.poll(sqlFlag).toBe('t');

    const cleanup = await page.request.delete(
      `/api/properties/${SEEDED_STUDIO_PROPERTY_ID}/rentals/${rental.id}`,
    );
    expect(cleanup.ok()).toBe(true);
  });
});
