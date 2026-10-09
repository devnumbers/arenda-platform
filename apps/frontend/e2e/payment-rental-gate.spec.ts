import type { Page } from '@playwright/test';
import {
  captureScreen,
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
  todayIso,
} from './fixtures';

// Гейт мутаций Платежа арендной платы (#818): платёж, на который ссылается
// аренда, создаётся, правится и удаляется только через аренду
// (rentals/CONTEXT.md, ADR 0053). Экран платежа скрывает «На паузу» и
// «Изменить», у незавершённой аренды вместо них «Изменить аренду» (#1158) —
// «Оплатить» остаётся каноном отметки оплаты месяца; экран правки
// деградирует к карточке недоступности; прямые API-мутации правила отвечают
// 409 с доменной подсказкой. Звезда избранного и платёжные факты гейту не
// подчиняются. Сид: аренды в seed.sql нет — фикстура создаёт её API-постом
// (и переиспользует при retry, инвариант №12 — одна незавершённая на
// объект).
//
// Сид: «Страхование» …552 на квартире — обычное (неуправляемое) правило
// для регрессии.

const PROPERTY = SEEDED_APARTMENT_PROPERTY_ID;
const ORDINARY_PAYMENT = `${PROPERTY}/payments/55555555-5555-4555-8555-555555555552`;

interface RentalFromApi {
  id: string;
  completedDate: string | null;
  rentPayment: { paymentId: string };
}

/** Аренды фикстуры: существующая незавершённая переиспользуется (retry),
 * иначе создаётся с началом «сегодня по TZ собственника» — сид держит
 * владельца в UTC (canon fixtures, #796), так что календарь владельца и
 * браузера совпадают и у границы суток. Все созданные/переиспользованные
 * аренды попадают в список уборки: ретрай теста завершения создаёт вторую
 * аренду (завершённая не переиспользуется), и без полного списка она
 * оставалась бы призраком на общей квартире — afterAll обещает соседним
 * спекам квартиру без аренд. */
let fixtureRentals: ReadonlyArray<{ rentalId: string; paymentId: string }> = [];

async function ensureRental(page: Page): Promise<{ rentalId: string; paymentId: string }> {
  const list = await page.request.get(`/api/properties/${PROPERTY}/rentals`);
  expect(list.ok()).toBe(true);
  const { items } = (await list.json()) as { items: ReadonlyArray<RentalFromApi> };
  const unfinished = items.find((rental) => rental.completedDate === null);
  if (unfinished !== undefined) {
    const found = { rentalId: unfinished.id, paymentId: unfinished.rentPayment.paymentId };
    if (!fixtureRentals.some((fixture) => fixture.rentalId === found.rentalId)) {
      fixtureRentals = [...fixtureRentals, found];
    }
    return found;
  }
  const today = todayIso();
  const created = await page.request.post(`/api/properties/${PROPERTY}/rentals`, {
    data: {
      amountKopecks: 3_000_000,
      paymentDay: Number(today.slice(8, 10)),
      startDate: today,
      plannedEndDate: null,
      utilities: 'included',
      autoPay: false,
    },
  });
  expect(created.status()).toBe(201);
  const rental = (await created.json()) as RentalFromApi;
  const made = { rentalId: rental.id, paymentId: rental.rentPayment.paymentId };
  fixtureRentals = [...fixtureRentals, made];
  return made;
}

test.describe('гейт мутаций платежа аренды', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  // Уборка фикстур в порядке rentals → operations → payments — как в
  // cleanupRental rentals-wizard: следующая спека (визард, режимы объекта)
  // ждёт квартиру без аренды.
  test.afterAll(async () => {
    for (const { rentalId, paymentId } of fixtureRentals) {
      await execE2eSql(`DELETE FROM rentals WHERE id = '${rentalId}'`);
      await execE2eSql(`DELETE FROM operations WHERE payment_id = '${paymentId}'`);
      await execE2eSql(`DELETE FROM payment_pauses WHERE payment_id = '${paymentId}'`);
      await execE2eSql(`DELETE FROM payments WHERE id = '${paymentId}'`);
    }
  });

  test('флаг isRentalManaged в контракте; на экране платежа — «Изменить аренду» + «Оплатить»', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    const { paymentId } = await ensureRental(page);

    const response = await page.request.get(`/api/properties/${PROPERTY}/payments`);
    expect(response.ok()).toBe(true);
    const { items } = await response.json() as {
      items: ReadonlyArray<{ id: string; isRentalManaged: boolean; isRentalCompleted: boolean }>;
    };
    const rentPayment = items.find((payment) => payment.id === paymentId);
    expect(rentPayment?.isRentalManaged).toBe(true);
    // Аренда фикстуры незавершённая — условия правятся (#1158).
    expect(rentPayment?.isRentalCompleted).toBe(false);

    await page.goto(`/properties/${PROPERTY}/payments/${paymentId}`);
    await expect(page.getByText('Арендная плата').first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Оплатить' })).toBeVisible();
    // #1158: у незавершённой аренды вместо «Изменить» — «Изменить аренду»,
    // ведёт на экран правки условий.
    await expect(page.getByRole('button', { name: 'Изменить аренду' })).toBeVisible();
    await page.getByRole('button', { name: 'Изменить аренду' }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/rentals/terms/edit`));
    await page.goBack();
    await expect(page.getByRole('button', { name: 'На паузу' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Возобновить' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Изменить', exact: true })).toHaveCount(0);

    await captureScreen(page, testInfo, 'rental-payment-gated-actions');
  });

  test('экран правки управляемого платежа деградирует, удаления нет', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    const { paymentId } = await ensureRental(page);

    await page.goto(`/properties/${PROPERTY}/payments/${paymentId}/edit`);
    await expect(page.getByText('Правка недоступна')).toBeVisible();
    await expect(
      page.getByText('Платёж управляется арендой — изменить его можно только в аренде'),
    ).toBeVisible();
    await expect(page.getByRole('button', { name: 'Удалить платеж' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Сохранить', exact: true })).toHaveCount(0);

    await captureScreen(page, testInfo, 'rental-payment-gated-edit');
  });

  test('API: пауза/правка/удаление правила — 409 с доменной подсказкой', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    const { rentalId, paymentId } = await ensureRental(page);
    const base = `/api/properties/${PROPERTY}/payments/${paymentId}`;

    const pause = await page.request.post(`${base}/pause`);
    expect(pause.status()).toBe(409);
    expect(((await pause.json()) as { detail: string }).detail)
      .toContain('Платёж управляется арендой');

    const resume = await page.request.post(`${base}/resume`);
    expect(resume.status()).toBe(409);

    const patch = await page.request.patch(base, {
      data: { title: 'Не прошло и не тут' },
    });
    expect(patch.status()).toBe(409);

    const del = await page.request.delete(`${base}?keep_overdue=true`);
    expect(del.status()).toBe(409);

    // Звезда — не условие аренды: гейт её не трогает (и возвращается назад).
    const favorite = await page.request.put(`${base}/favorite`, { data: { favorite: true } });
    expect(favorite.ok()).toBe(true);
    await expect(await page.request.put(`${base}/favorite`, { data: { favorite: false } }))
      .toBeOK();

    // Server-truth: ничего не произошло — паузы нет, название серверное,
    // аренда на месте.
    expect(await execE2eSql(
      `SELECT COUNT(*) FROM payment_pauses WHERE payment_id = '${paymentId}'`,
    )).toBe('0');
    expect(await execE2eSql(
      `SELECT title FROM payments WHERE id = '${paymentId}'`,
    )).toBe('Арендная плата');
    expect(await execE2eSql(
      `SELECT COUNT(*) FROM rentals WHERE id = '${rentalId}' AND completed_date IS NULL`,
    )).toBe('1');
  });

  test('обычное правило не затронуто: кнопки на месте, флаг false', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);

    const response = await page.request.get(`/api/properties/${PROPERTY}/payments`);
    expect(response.ok()).toBe(true);
    const { items } = (await response.json()) as {
      items: ReadonlyArray<{ id: string; isRentalManaged: boolean }>;
    };
    expect(
      items.find((payment) => payment.id === '55555555-5555-4555-8555-555555555552')
        ?.isRentalManaged,
    ).toBe(false);

    await page.goto(`/properties/${ORDINARY_PAYMENT}`);
    await expect(page.getByRole('button', { name: 'Изменить' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'На паузу' })).toBeVisible();
  });

  test('карточка правки ведёт в условия аренды — переход вместо тупика (#988)', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    const { paymentId } = await ensureRental(page);

    await page.goto(`/properties/${PROPERTY}/payments/${paymentId}/edit`);
    await expect(page.getByText('Правка недоступна')).toBeVisible();
    await page.getByRole('button', { name: 'Условия аренды' }).click();

    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/rentals/terms`));
    await expect(page.getByText('Условия аренды').first()).toBeVisible();

    await captureScreen(page, testInfo, 'rental-payment-edit-to-terms');
  });

  test('завершение аренды: платёж остановлен и завершён без противоречивых действий (#988)', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    const { rentalId, paymentId } = await ensureRental(page);

    // Конвейер аренды: платёж остановлен на дате завершения, будущие
    // плановые снесены (ADR 0053 §3), действий противоречия на экране нет.
    const stopDate = todayIso();
    const completed = await page.request.post(
      `/api/properties/${PROPERTY}/rentals/${rentalId}/complete`,
      { data: { completedDate: stopDate } },
    );
    expect(completed.ok()).toBe(true);

    // Остановка: у выживших операций дата не позже стопа — будущие плановые
    // снесены, просрочки до стопа остались (серверная правда, не только UI).
    for (const status of ['planned', 'overdue'] as const) {
      const left = await page.request.get(
        `/api/properties/${PROPERTY}/payments/${paymentId}/operations?status=${status}`,
      );
      expect(left.ok()).toBe(true);
      const { items } = (await left.json()) as { items: ReadonlyArray<{ date: string }> };
      expect(items.every((operation) => operation.date <= stopDate)).toBe(true);
    }

    await page.goto(`/properties/${PROPERTY}/payments/${paymentId}`);
    await expect(page.getByText('Арендная плата').first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Оплатить' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Оплатить' })).toBeEnabled();
    await expect(page.getByRole('button', { name: 'На паузу' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Возобновить' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Изменить', exact: true })).toHaveCount(0);
    // #1158: аренда завершена — правки условий нет, «Изменить аренду» ушла.
    await expect(page.getByRole('button', { name: 'Изменить аренду' })).toHaveCount(0);

    // Гасим всё неоплаченное до стопа — серверный isCompleted становится
    // true («Завершённый платёж» вычисляемый, CONTEXT.md).
    for (const status of ['planned', 'overdue'] as const) {
      const list = await page.request.get(
        `/api/properties/${PROPERTY}/payments/${paymentId}/operations?status=${status}`,
      );
      expect(list.ok()).toBe(true);
      const { items } = (await list.json()) as { items: ReadonlyArray<{ id: string }> };
      for (const operation of items) {
        const paid = await page.request.post(
          `/api/properties/${PROPERTY}/operations/${operation.id}/pay`,
        );
        expect(paid.ok()).toBe(true);
      }
    }

    await page.goto(`/properties/${PROPERTY}/payments/${paymentId}`);
    // Deep-link даёт транзиентное двойное дерево гидрации (~100мс, грабля
    // #987): строгий локатор ловит обе копии — берём первую видимую.
    await expect(page.getByText('Платеж завершен').filter({ visible: true }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Оплатить' }).first()).toBeDisabled();
    await expect(page.getByRole('button', { name: 'На паузу' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Изменить', exact: true })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Изменить аренду' })).toHaveCount(0);

    await captureScreen(page, testInfo, 'rental-payment-completed-after-rental');
  });
});
