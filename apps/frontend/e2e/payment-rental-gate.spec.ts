import type { Page } from '@playwright/test';
import {
  captureScreen,
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';

// Гейт мутаций Платежа арендной платы (#818): платёж, на который ссылается
// аренда, создаётся, правится и удаляется только через аренду
// (rentals/CONTEXT.md, ADR 0053). Экран платежа скрывает «На паузу» и
// «Изменить» — «Оплатить» остаётся каноном отметки оплаты месяца;
// экран правки деградирует к карточке недоступности; прямые API-мутации
// правила отвечают 409 с доменной подсказкой. Звезда избранного и платёжные
// факты гейту не подчиняются. Сид: аренды в seed.sql нет — фикстура создаёт
// её API-постом (и переиспользует при retry, инвариант №12 — одна
// незавершённая на объект).
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

/** Аренда фикстуры: существующая незавершённая переиспользуется (retry),
 * иначе создаётся с началом «сегодня по TZ собственника» (Europe/Moscow —
 * браузер живёт в UTC, у границы суток дата другая). Запоминается для
 * afterAll-уборки: квартиры ждёт и rentals-wizard — инвариант №12 (одна
 * незавершённая на объект) не должен ломать соседние спеки. */
let fixtureRental: { rentalId: string; paymentId: string } | null = null;

async function ensureRental(page: Page): Promise<{ rentalId: string; paymentId: string }> {
  const list = await page.request.get(`/api/properties/${PROPERTY}/rentals`);
  expect(list.ok()).toBe(true);
  const { items } = (await list.json()) as { items: ReadonlyArray<RentalFromApi> };
  const unfinished = items.find((rental) => rental.completedDate === null);
  if (unfinished !== undefined) {
    fixtureRental = { rentalId: unfinished.id, paymentId: unfinished.rentPayment.paymentId };
    return fixtureRental;
  }
  const today = new Intl.DateTimeFormat('en-CA', { timeZone: 'Europe/Moscow' })
    .format(new Date());
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
  fixtureRental = { rentalId: rental.id, paymentId: rental.rentPayment.paymentId };
  return fixtureRental;
}

test.describe('гейт мутаций платежа аренды', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  // Уборка фикстуры в порядке rentals → operations → payments — как в
  // cleanupRental rentals-wizard: следующая спека (визард, режимы объекта)
  // ждёт квартиру без аренды.
  test.afterAll(async () => {
    if (fixtureRental === null) {
      return;
    }
    const { rentalId, paymentId } = fixtureRental;
    await execE2eSql(`DELETE FROM rentals WHERE id = '${rentalId}'`);
    await execE2eSql(`DELETE FROM operations WHERE payment_id = '${paymentId}'`);
    await execE2eSql(`DELETE FROM payment_pauses WHERE payment_id = '${paymentId}'`);
    await execE2eSql(`DELETE FROM payments WHERE id = '${paymentId}'`);
  });

  test('флаг isRentalManaged в контракте; на экране платежа — только «Оплатить»', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    const { paymentId } = await ensureRental(page);

    const response = await page.request.get(`/api/properties/${PROPERTY}/payments`);
    expect(response.ok()).toBe(true);
    const { items } = (await response.json()) as {
      items: ReadonlyArray<{ id: string; isRentalManaged: boolean }>;
    };
    expect(items.find((payment) => payment.id === paymentId)?.isRentalManaged).toBe(true);

    await page.goto(`/properties/${PROPERTY}/payments/${paymentId}`);
    await expect(page.getByText('Арендная плата').first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Оплатить' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'На паузу' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Возобновить' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Изменить' })).toHaveCount(0);

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
    await expect(page.getByRole('button', { name: 'Сохранить изменения' })).toHaveCount(0);

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
});
