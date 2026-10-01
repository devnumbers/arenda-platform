import {
  captureScreen,
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Машина «Действий аренды» на странице объекта (#986, карта #984):
// аренды нет → «Начать аренду», «Ожидает начала» → «Удалить аренду»
// (бэк #985: DELETE = 204, платёж сносится вместе с арендой), «идёт» →
// «Завершить аренду» (#627). Гард статуса #628 у «Ожидает начала» —
// составное «Удалить аренду + применить статус» (решение владельца
// 30.09: завершение будущей аренды не существует); гард удаления #632
// ведёт в шит той же машины. Правки аренды на объекте нет — только со
// страницы аренды (#987).
//
// Объект сценария — отдельная строка в properties (сид-квартира занята
// ассертами «Аренда не добавлена» в property-viewer-mode): арендные пары
// rent/(rentals) сеются SQL-контрактом бэка (integration_test.go
// seedCompletedRental), уборка — rentals, затем объект (каскад уносит
// платежи, операции и журнал).

const PROPERTY_ID = '98600000-9860-4000-8000-000000000986';
const OWNER_SQL = `(SELECT id FROM users WHERE email = '${process.env.E2E_USER_EMAIL}')`;

const RENT_AMOUNT_KOPECKS = '4731000'; // уникальная сумма — материал уборки

test.describe('машина «Действий аренды» на странице объекта', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  /** Чистый объект сценария: снос прошлых прогонов и создание строки.
   * rentals сносятся до объекта (RESTRICT от payments по каскаду). */
  async function seedProperty(): Promise<void> {
    await execE2eSql(`DELETE FROM rentals WHERE property_id = '${PROPERTY_ID}'`);
    await execE2eSql(`DELETE FROM properties WHERE id = '${PROPERTY_ID}'`);
    await execE2eSql(
      `INSERT INTO properties (id, owner_id, name, type, address, status)
       VALUES ('${PROPERTY_ID}', ${OWNER_SQL}, 'Дом арендных состояний', 'house', 'ул. Состояний, 9', 'active')`,
    );
  }

  /** Арендная пара SQL-контрактом бэка: управляемый Платёж «Арендная
   * плата» + аренда с датами относительно серверного current_date. */
  async function seedRental(startOffsetDays: number, endOffsetDays: number): Promise<void> {
    await execE2eSql(
      `INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
                             recurrence, since, end_date, auto_pay, category_slug)
       VALUES ('98600000-9860-4000-8000-000000000987', ${OWNER_SQL}, '${PROPERTY_ID}',
               'income', 'Арендная плата', ${RENT_AMOUNT_KOPECKS},
               '{"kind":"monthly","daysOfMonth":[15]}'::jsonb,
               current_date + ${startOffsetDays}, current_date + ${endOffsetDays},
               false, 'rent')`,
    );
    await execE2eSql(
      `INSERT INTO rentals (id, owner_id, property_id, payment_id, start_date,
                            planned_end_date, completed_date, utilities)
       VALUES ('98600000-9860-4000-8000-000000000988', ${OWNER_SQL}, '${PROPERTY_ID}',
               '98600000-9860-4000-8000-000000000987',
               current_date + ${startOffsetDays}, current_date + ${endOffsetDays},
               NULL, 'included')`,
    );
  }

  async function openDetail(
    page: Parameters<typeof openCabinetWithSeededSession>[0],
    user: Parameters<typeof openCabinetWithSeededSession>[1],
  ): Promise<void> {
    await openCabinetWithSeededSession(page, user);
    // Путь пользователя (канон #698): список → ряд. Deep-link здесь не
    // годится: при холодном входе гидрация Next.js на ~100мс держит в
    // DOM два дерева (стрим RSC до замены гидрацией), строгий локатор
    // ловит в этом окне дубль testid — strict не ретраится.
    await page.goto('/properties');
    await page.getByRole('link', { name: 'Дом арендных состояний' }).click();
    await expect(page.getByTestId('property-manage-list')).toBeVisible();
  }

  /** Шит статуса через кебаб: пункты — menuitem'ы канона Modal-шита. */
  async function openStatusSheet(
    page: Parameters<typeof openCabinetWithSeededSession>[0],
  ): Promise<void> {
    await page.getByRole('button', { name: 'Действия с объектом' }).click();
    await page.getByRole('menuitem', { name: 'Изменить статус' }).click();
    const menu = page.getByRole('menu', { name: 'Изменить статус' });
    await expect(menu).toBeVisible();
  }

  test.afterEach(async () => {
    await execE2eSql(`DELETE FROM rentals WHERE property_id = '${PROPERTY_ID}'`);
    await execE2eSql(`DELETE FROM properties WHERE id = '${PROPERTY_ID}'`);
  });

  test('аренды нет: «Начать аренду» в «Управлении» и шите, пустой блок с CTA', async ({
    page,
    seededUser,
  }, testInfo) => {
    await seedProperty();
    await openDetail(page, seededUser);

    const manage = page.getByTestId('property-manage-list');
    await expect(manage.getByRole('button', { name: 'Начать аренду' })).toBeVisible();
    await expect(manage.getByRole('button', { name: 'Завершить аренду' })).toHaveCount(0);
    await expect(manage.getByRole('button', { name: 'Удалить аренду' })).toHaveCount(0);

    await openStatusSheet(page);
    const menu = page.getByRole('menu', { name: 'Изменить статус' });
    await expect(menu.getByRole('menuitem', { name: 'Начать аренду' })).toBeVisible();
    await expect(menu.getByRole('menuitem', { name: 'Завершить аренду' })).toHaveCount(0);
    await page.keyboard.press('Escape');

    // Пустой блок «Аренда»: копия CTA «Добавить» — канон #588, вопрос
    // «Начать аренду?» решается на приёмке #986 (карта: not yet specified).
    const rentCard = page.locator('section').filter({
      has: page.getByText('Аренда не добавлена'),
    });
    await expect(rentCard).toHaveCount(1);
    await expect(rentCard.getByRole('button', { name: 'Добавить' })).toBeVisible();

    await captureScreen(page, testInfo, 'rental-actions-none');
  });

  test('«Ожидает начала»: «Удалить аренду», гард #628 удаляет аренду и ставит ремонт', async ({
    page,
    seededUser,
  }, testInfo) => {
    await seedProperty();
    await seedRental(10, 740); // старт через 10 дней — upcoming
    await openDetail(page, seededUser);

    const manage = page.getByTestId('property-manage-list');
    await expect(manage.getByRole('button', { name: 'Удалить аренду' })).toBeVisible();
    await expect(manage.getByRole('button', { name: 'Завершить аренду' })).toHaveCount(0);
    await expect(manage.getByRole('button', { name: 'Начать аренду' })).toHaveCount(0);

    await openStatusSheet(page);
    const menu = page.getByRole('menu', { name: 'Изменить статус' });
    await expect(menu.getByRole('menuitem', { name: 'Удалить аренду' })).toBeVisible();
    await expect(menu.getByRole('menuitem', { name: 'Завершить аренду' })).toHaveCount(0);
    await page.keyboard.press('Escape');
    await expect(menu).toBeHidden();

    // Гард #628 (решение владельца 30.09): «Объект на ремонте» под
    // upcoming — составное «Удалить аренду + применить статус».
    await manage.getByRole('button', { name: 'Объект на ремонте' }).click();
    const guard = page.getByRole('dialog', { name: 'Нельзя изменить статус, пока аренда не началась' });
    await expect(guard).toBeVisible();
    await expect(guard.getByText('Удалите аренду, чтобы изменить статус объекта')).toBeVisible();

    await guard.getByRole('button', { name: 'Удалить аренду', exact: true }).click();

    // Композит исполнен: аренда снесена (204), статус применён — шапка
    // «На ремонте», блок «Аренда» перетекает в пустое состояние.
    await expect(page.getByText('На ремонте')).toBeVisible();
    await expect(page.getByText('Аренда не добавлена')).toBeVisible();
    await expect(page.getByTestId('property-rental-block')).toHaveCount(0);

    // Server-truth (#860): rentals 0, объект maintenance.
    const rentalsCount = await execE2eSql(
      `SELECT count(*) FROM rentals WHERE property_id = '${PROPERTY_ID}'`,
    );
    expect(rentalsCount).toBe('0');
    const status = await execE2eSql(`SELECT status FROM properties WHERE id = '${PROPERTY_ID}'`);
    expect(status).toBe('maintenance');

    await captureScreen(page, testInfo, 'rental-actions-upcoming-guard');
  });

  test('«Ожидает начала»: гард удаления #632 ведёт в шит «Удалить аренду?»', async ({
    page,
    seededUser,
  }) => {
    await seedProperty();
    await seedRental(10, 740);
    await openDetail(page, seededUser);

    await page
      .getByTestId('property-manage-list')
      .getByRole('button', { name: 'Удалить объект' })
      .click();

    const guard = page.getByRole('dialog', { name: 'Нельзя удалить объект, пока аренда не началась' });
    await expect(guard).toBeVisible();
    await expect(guard.getByText('Удалите аренду, чтобы удалить объект')).toBeVisible();
    await guard.getByRole('button', { name: 'Удалить аренду', exact: true }).click();

    // Шит той же машины — канон удаления завершённой аренды (#535);
    // «Отмена» ничего не сносит.
    const deleteSheet = page.getByRole('dialog', { name: 'Удалить аренду?' });
    await expect(deleteSheet).toBeVisible();
    await expect(
      deleteSheet.getByText('Аренда и её платеж будут удалены. Операции останутся в истории объекта'),
    ).toBeVisible();
    await deleteSheet.getByRole('button', { name: 'Отмена' }).click();
    await expect(deleteSheet).toHaveCount(0);

    const rentalsCount = await execE2eSql(
      `SELECT count(*) FROM rentals WHERE property_id = '${PROPERTY_ID}'`,
    );
    expect(rentalsCount).toBe('1');
  });

  test('«Ожидает начала»: прямое удаление строкой — шит канона, блок пустеет', async ({
    page,
    seededUser,
  }) => {
    await seedProperty();
    await seedRental(10, 740);
    await openDetail(page, seededUser);

    // Прямой путь (мимо гарда): строка «Управления» → шит канона #535 →
    // «Удалить» — DELETE уносит аренду вместе с платём, объект остаётся.
    await page
      .getByTestId('property-manage-list')
      .getByRole('button', { name: 'Удалить аренду' })
      .click();
    const sheet = page.getByRole('dialog', { name: 'Удалить аренду?' });
    await expect(sheet).toBeVisible();
    await sheet.getByRole('button', { name: 'Удалить', exact: true }).click();

    await expect(page.getByText('Аренда не добавлена')).toBeVisible();
    await expect(page.getByTestId('property-rental-block')).toHaveCount(0);
    const manage = page.getByTestId('property-manage-list');
    await expect(manage.getByRole('button', { name: 'Начать аренду' })).toBeVisible();

    const rentalsCount = await execE2eSql(
      `SELECT count(*) FROM rentals WHERE property_id = '${PROPERTY_ID}'`,
    );
    expect(rentalsCount).toBe('0');
    const paymentsCount = await execE2eSql(
      `SELECT count(*) FROM payments WHERE property_id = '${PROPERTY_ID}'`,
    );
    expect(paymentsCount).toBe('0');
  });

  test('идёт: «Завершить аренду» — шит #627 завершает, блок пустеет', async ({
    page,
    seededUser,
  }, testInfo) => {
    await seedProperty();
    await seedRental(-60, 305); // старт 60 дней назад — active
    await openDetail(page, seededUser);

    const manage = page.getByTestId('property-manage-list');
    await expect(manage.getByRole('button', { name: 'Завершить аренду' })).toBeVisible();
    await expect(manage.getByRole('button', { name: 'Удалить аренду' })).toHaveCount(0);
    await expect(page.getByTestId('property-rental-block')).toBeVisible();

    await manage.getByRole('button', { name: 'Завершить аренду' }).click();
    const completeSheet = page.getByRole('dialog', { name: 'Завершить аренду?' });
    await expect(completeSheet).toBeVisible();
    await completeSheet.getByRole('button', { name: 'Завершить', exact: true }).click();

    await expect(page.getByText('Аренда завершена')).toBeVisible();
    await expect(page.getByText('Аренда не добавлена')).toBeVisible();

    // Server-truth: completed_date = сегодня (POST /complete прошёл).
    const completedDate = await execE2eSql(
      `SELECT completed_date FROM rentals WHERE property_id = '${PROPERTY_ID}'`,
    );
    expect(completedDate).toBeTruthy();

    await captureScreen(page, testInfo, 'rental-actions-active-complete');
  });
});
