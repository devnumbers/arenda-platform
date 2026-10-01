import {
  captureScreen,
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  openCabinetWithSessionToken,
  seededViewerSessionToken,
  test,
} from './fixtures';

// Машина «Действий аренды» на странице аренды (#987, карта #984):
// страница ест тот же доменный селектор rentalActionState, что и объект
// (#986), — расхождение поверхностей баг, а не вариант UX. «Ожидает
// начала» → «Удалить аренду» (бэк #985: DELETE = 204, платёж сносится
// вместе с арендой; подтверждение — канон удаления завершённой аренды
// #535), после удаления — возврат на страницу объекта. «Идёт» →
// «Завершить аренду» (#534, полный мастер) — не трогаем, сверяем
// синхронность условий. «Продлить» — у любой незавершённой (#804), в
// тройку синхронизации не входит. У круглой тройки левый слот —
// «Завершить» — у upcoming по-прежнему скрыт (#802): деструктивное
// действие живёт строкой «Управления», как на обеих принятых
// поверхностях удаления (#535, #986).
//
// Объект сценария — отдельная строка в properties (сид-квартира занята
// ассертами других спек): арендная пара rent/(rentals) сеется
// SQL-контрактом бэка (integration_test.go seedCompletedRental), уборка —
// rentals, затем объект (каскад уносит платежи, операции и журнал).

const PROPERTY_ID = '98700000-9870-4000-8000-000000000987';
const OWNER_SQL = `(SELECT id FROM users WHERE email = '${process.env.E2E_USER_EMAIL}')`;

const RENT_AMOUNT_KOPECKS = '4872000'; // уникальная сумма — материал уборки

test.describe('машина «Действий аренды» на странице аренды', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  /** Чистый объект сценария: снос прошлых прогонов и создание строки.
   * rentals сносятся до объекта (RESTRICT от payments по каскаду). */
  async function seedProperty(): Promise<void> {
    await execE2eSql(`DELETE FROM rentals WHERE property_id = '${PROPERTY_ID}'`);
    await execE2eSql(`DELETE FROM properties WHERE id = '${PROPERTY_ID}'`);
    await execE2eSql(
      `INSERT INTO properties (id, owner_id, name, type, address, status)
       VALUES ('${PROPERTY_ID}', ${OWNER_SQL}, 'Дом аренды страниц', 'house', 'ул. Страниц, 7', 'active')`,
    );
  }

  /** Арендная пара SQL-контрактом бэка: управляемый Платёж «Арендная
   * плата» + аренда с датами относительно серверного current_date. */
  async function seedRental(startOffsetDays: number, endOffsetDays: number): Promise<void> {
    await execE2eSql(
      `INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
                             recurrence, since, end_date, auto_pay, category_slug)
       VALUES ('98700000-9870-4000-8000-000000000988', ${OWNER_SQL}, '${PROPERTY_ID}',
               'income', 'Арендная плата', ${RENT_AMOUNT_KOPECKS},
               '{"kind":"monthly","daysOfMonth":[15]}'::jsonb,
               current_date + ${startOffsetDays}, current_date + ${endOffsetDays},
               false, 'rent')`,
    );
    await execE2eSql(
      `INSERT INTO rentals (id, owner_id, property_id, payment_id, start_date,
                            planned_end_date, completed_date, utilities)
       VALUES ('98700000-9870-4000-8000-000000000989', ${OWNER_SQL}, '${PROPERTY_ID}',
               '98700000-9870-4000-8000-000000000988',
               current_date + ${startOffsetDays}, current_date + ${endOffsetDays},
               NULL, 'included')`,
    );
  }

  /** Путь пользователя (канон #698): список → ряд объекта → секция
   * «Аренда». Deep-link здесь не годится: при холодном входе гидрация
   * Next.js на ~100мс держит в DOM два дерева (стрим RSC до замены
   * гидрацией), строгий локатор ловит в этом окне дубль testid. */
  async function openRentalPage(
    page: Parameters<typeof openCabinetWithSeededSession>[0],
    user: Parameters<typeof openCabinetWithSeededSession>[1],
  ): Promise<void> {
    await openCabinetWithSeededSession(page, user);
    await page.goto('/properties');
    await page.getByRole('link', { name: 'Дом аренды страниц' }).click();
    await expect(page.getByTestId('property-manage-list')).toBeVisible();
    await page.getByRole('link', { name: 'Аренда' }).click();
    // Маркер детализации — пара «Продлить аренду» (круглая тройка + строка
    // «Управления»): на объекте таких кнопок нет, а count дождётся ровно
    // двух — голый toBeVisible в окне перехода страниц ловит строгим
    // локатором чужие элементы.
    await expect(page.getByRole('button', { name: 'Продлить аренду' })).toHaveCount(2);
  }

  test.afterEach(async () => {
    await execE2eSql(`DELETE FROM rentals WHERE property_id = '${PROPERTY_ID}'`);
    await execE2eSql(`DELETE FROM properties WHERE id = '${PROPERTY_ID}'`);
  });

  test('«Ожидает начала»: «Удалить аренду» в «Управлении», «Завершить» нет нигде', async ({
    page,
    seededUser,
  }, testInfo) => {
    await seedProperty();
    await seedRental(10, 740); // старт через 10 дней — upcoming
    await openRentalPage(page, seededUser);

    // Машина (#987): завершения у upcoming не существует — ни круглой
    // тройки, ни строки; удаление — красная строка «Управления» (канон
    // #535); «Продлить» остаётся (#804). Состав тройки — по канону
    // деградации: у этого сида будущего вхождения платежа нет (бэк не
    // даёт плановый платёж до старта) — «Оплатить» скрыт, слот «Завершить»
    // пуст, тройка вырождается в одиночную «Продлить».
    await expect(page.getByRole('button', { name: 'Удалить аренду' })).toHaveCount(1);
    await expect(page.getByRole('button', { name: 'Завершить аренду' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Продлить аренду' })).toHaveCount(2);
    await expect(page.getByRole('button', { name: 'Оплатить платеж' })).toHaveCount(0);

    await captureScreen(page, testInfo, 'rental-page-upcoming');
  });

  test('«Ожидает начала»: шит канона #535, «Отмена» ничего не сносит', async ({
    page,
    seededUser,
  }) => {
    await seedProperty();
    await seedRental(10, 740);
    await openRentalPage(page, seededUser);

    await page.getByRole('button', { name: 'Удалить аренду' }).click();
    const sheet = page.getByRole('dialog', { name: 'Удалить аренду?' });
    await expect(sheet).toBeVisible();
    await expect(
      sheet.getByText('Аренда и её платеж будут удалены. Операции останутся в истории объекта'),
    ).toBeVisible();
    await sheet.getByRole('button', { name: 'Отмена' }).click();
    await expect(sheet).toHaveCount(0);

    const rentalsCount = await execE2eSql(
      `SELECT count(*) FROM rentals WHERE property_id = '${PROPERTY_ID}'`,
    );
    expect(rentalsCount).toBe('1');
  });

  test('«Ожидает начала»: удаление возвращает на объект, аренда и платёж снесены', async ({
    page,
    seededUser,
  }, testInfo) => {
    await seedProperty();
    await seedRental(10, 740);
    await openRentalPage(page, seededUser);

    await page.getByRole('button', { name: 'Удалить аренду' }).click();
    const sheet = page.getByRole('dialog', { name: 'Удалить аренду?' });
    await expect(sheet).toBeVisible();
    await sheet.getByRole('button', { name: 'Удалить', exact: true }).click();

    // Возврат на страницу объекта (тикет #987): аренды нет — блок
    // «Аренда» пуст, «Управление» показывает «Начать аренду» (синхронность
    // поверхностей по одному селектору).
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY_ID}$`));
    await expect(page.getByText('Аренда не добавлена')).toBeVisible();
    await expect(
      page
        .getByTestId('property-manage-list')
        .getByRole('button', { name: 'Начать аренду' }),
    ).toBeVisible();

    // Server-truth (#860): DELETE унёс аренду вместе с платём (#985).
    const rentalsCount = await execE2eSql(
      `SELECT count(*) FROM rentals WHERE property_id = '${PROPERTY_ID}'`,
    );
    expect(rentalsCount).toBe('0');
    const paymentsCount = await execE2eSql(
      `SELECT count(*) FROM payments WHERE property_id = '${PROPERTY_ID}'`,
    );
    expect(paymentsCount).toBe('0');

    await captureScreen(page, testInfo, 'rental-page-upcoming-deleted');
  });

  test('идёт: «Завершить аренду» на месте (тройка и строка), «Удалить аренду» нет', async ({
    page,
    seededUser,
  }, testInfo) => {
    await seedProperty();
    await seedRental(-60, 305); // старт 60 дней назад — active
    await openRentalPage(page, seededUser);

    // Синхронность с объектом (#986): у «идёт» действие — Завершение,
    // удаления нет. Тройка и строка «Управления» открывают полный мастер
    // #534 (здесь не исполняем — мастер покрыт своими спеками).
    await expect(page.getByRole('button', { name: 'Завершить аренду' })).toHaveCount(2);
    await expect(page.getByRole('button', { name: 'Удалить аренду' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Продлить аренду' })).toHaveCount(2);

    await captureScreen(page, testInfo, 'rental-page-active');
  });

  test('смотрящему: ни одной мутирующей строки, чтение аренды цело', async ({ page }) => {
    await seedProperty();
    // Зритель сида (Сергей, #467) получает viewer-членство на объекте
    // сценария; спадёт вместе с объектом (каскад) в afterEach.
    await execE2eSql(
      `INSERT INTO property_members (id, property_id, user_id, role, granted_by)
       VALUES ('98700000-9870-4000-8000-000000000990', '${PROPERTY_ID}',
               '13111111-1111-4111-8111-111111111131', 'viewer',
               '11111111-1111-4111-8111-111111111111')`,
    );
    await seedRental(10, 740);

    // Спека #987: «удаление — мутаторам (как остальные действия
    // страницы)» — вся секция «Управление» под тем же canMutate, что и
    // до тикета; зритель читает аренду, но не видит ни одного действия.
    await openCabinetWithSessionToken(page, seededViewerSessionToken());
    await page.goto('/properties');
    await page.getByRole('link', { name: 'Дом аренды страниц' }).click();
    await page.getByRole('link', { name: 'Аренда' }).click();
    await expect(page.getByRole('heading', { name: 'Условия аренды' })).toBeVisible();

    await expect(page.getByRole('button', { name: 'Удалить аренду' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Завершить аренду' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Продлить аренду' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Редактировать аренду' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Открыть платеж арендной платы' })).toBeVisible();
  });
});
