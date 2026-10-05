import type { Page } from '@playwright/test';
import {
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  screenHeader,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  test,
  todayIso,
} from './fixtures';

// Возврат после оплаты (#1072, Т4 карты #1068): закрытие success «Платеж
// оплачен» («Хорошо» и крестик) возвращает на страницу, С КОТОРОЙ перешли
// к оплате — точки входа прокидывают sanitized ?returnTo= (страница
// платежа «Оплатить», «Оплатить платеж» аренды); «Посмотреть платеж» по-
// прежнему ведёт на правило. Правило сценария создаётся визардом на
// Гараже на Садовой — сидово без платежей, прогон не трогает сидовые
// правила квартиры; аренда для второго сценария — API-постом на квартире
// (инвариант №12: одна незавершённая, переиспользование при retry),
// уборка возвращает квартиру без аренд — как в payment-rental-gate
// (workers: 1, общий сид на прогон).

const GARAGE = SEEDED_GARAGE_PROPERTY_ID;
const APARTMENT = SEEDED_APARTMENT_PROPERTY_ID;

interface PaymentFromApi {
  readonly id: string;
  readonly title: string;
}

interface RentalFromApi {
  readonly id: string;
  readonly completedDate: string | null;
  readonly rentPayment: { readonly paymentId: string };
}

/** Адрес операции; без хвостового якоря — переход несёт ?returnTo= (#1072). */
function operationUrlPattern(propertyId: string): RegExp {
  return new RegExp(`/properties/${propertyId}/operations/[0-9a-f-]+`);
}

/** Шаг оплаты из видимого флоу: на странице операции нажать «Отметить
 * оплаченной» и дождаться success. */
async function payFromOperationScreen(page: Page): Promise<void> {
  await page.getByRole('button', { name: 'Отметить оплаченной' }).click();
  await expect(page.getByText('Платеж оплачен')).toBeVisible();
}

test.describe('возврат на исходную страницу после оплаты (#1072)', () => {
  test.use({ viewport: { width: 390, height: 844 } });
  test.setTimeout(240_000);

  test('оплата со страницы платежа — «Хорошо» возвращает на неё', async ({
    page,
    seededUser,
  }, testInfo) => {
    const run = String(testInfo.retry);
    const title = `E2E возврат после оплаты ${run}`;

    // Гараж — сидово без платежей: правило убирается в любом исходе.
    try {
      // ── Правило «каждый месяц, 1-е число» визардом (канон payment-next-date) ──
      await openCabinetWithSeededSession(page, seededUser);
      await page.goto(`/properties/${GARAGE}/payments`);
      await page.getByRole('button', { name: 'Добавить' }).click();
      await page.getByRole('button', { name: /Платеж Отмечайте оплату/ }).click();
      await expect(page.getByRole('heading', { name: 'Категория платежа' })).toBeVisible();
      await page.getByRole('button', { name: 'Интернет', exact: true }).click();
      await page.getByRole('button', { name: 'Продолжить' }).click();
      await expect(page.getByRole('heading', { name: 'Назовите платеж' })).toBeVisible();
      await page.getByRole('textbox').fill(title);
      await page.getByRole('button', { name: 'Продолжить' }).click();
      await page.getByRole('button', { name: 'Каждый месяц' }).click();
      await expect(page.getByRole('heading', { name: 'Выберите день', exact: true })).toBeVisible();
      await page.getByRole('button', { name: '1', exact: true }).first().click();
      await page.getByRole('button', { name: 'Продолжить' }).click();
      await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();
      await page.getByRole('button', { name: 'Далее' }).click();
      await page.getByRole('textbox', { name: 'Сумма' }).fill('1990');
      await page.getByRole('button', { name: 'Создать платеж' }).click();
      await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);
      await page.getByRole('button', { name: 'Хорошо, закрыть' }).click();

      const response = await page.request.get(`/api/properties/${GARAGE}/payments`);
      expect(response.ok()).toBe(true);
      const { items } = (await response.json()) as { items: ReadonlyArray<PaymentFromApi> };
      const created = items.find((payment) => payment.title === title);
      if (created === undefined) {
        throw new Error(`created payment «${title}» is missing from the API list`);
      }
      const paymentPageUrl = `/properties/${GARAGE}/payments/${created.id}`;

      // ── «Оплатить» страницы платежа ведёт на операцию с ?returnTo= неё ──
      await page.goto(paymentPageUrl);
      await page.getByRole('button', { name: 'Оплатить' }).click();
      await expect(page).toHaveURL(operationUrlPattern(GARAGE));
      expect(new URL(page.url()).searchParams.get('returnTo')).toBe(paymentPageUrl);

      await payFromOperationScreen(page);
      await page.getByRole('button', { name: 'Хорошо', exact: true }).click();

      // ── Закрытие success — на странице платежа, не операции; тик
      // материализовал следующее вхождение — «Оплатить» снова активна ──
      await expect(page).toHaveURL(new RegExp(`${paymentPageUrl}$`));
      await expect(page.getByRole('button', { name: 'Оплатить' })).toBeEnabled();
    } finally {
      await execE2eSql(
        `DELETE FROM operations WHERE payment_id IN ` +
          `(SELECT id FROM payments WHERE property_id = '${GARAGE}' AND title = '${title}')`,
      );
      await execE2eSql(
        `DELETE FROM payments WHERE property_id = '${GARAGE}' AND title = '${title}'`,
      );
    }
  });

  test('оплата со страницы аренды — крестик возвращает на аренду; «Посмотреть платеж» ведёт на правило', async ({
    page,
    seededUser,
  }) => {
    // Сессия раньше API-фикстуры: кука сессии общая с page.request.
    await openCabinetWithSeededSession(page, seededUser);

    // Аренды в seed.sql нет: переиспользуем существующую незавершённую
    // (retry), иначе создаём с началом «сегодня по TZ собственника» —
    // календарь владельца и браузера совпадают (канон fixtures, #796).
    const list = await page.request.get(`/api/properties/${APARTMENT}/rentals`);
    expect(list.ok()).toBe(true);
    const { items } = (await list.json()) as { items: ReadonlyArray<RentalFromApi> };
    let rental = items.find((item) => item.completedDate === null);
    if (rental === undefined) {
      const today = todayIso();
      const created = await page.request.post(`/api/properties/${APARTMENT}/rentals`, {
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
      rental = (await created.json()) as RentalFromApi;
    }

    const rentalPageUrl = `/properties/${APARTMENT}/rentals`;
    const rentPaymentId = rental.rentPayment.paymentId;

    try {
      // ── «Оплатить платеж» аренды ведёт на операцию с ?returnTo= аренды ──
      await page.goto(rentalPageUrl);
      await page.getByRole('button', { name: 'Оплатить платеж' }).click();
      await expect(page).toHaveURL(operationUrlPattern(APARTMENT));
      expect(new URL(page.url()).searchParams.get('returnTo')).toBe(rentalPageUrl);

      await payFromOperationScreen(page);

      // Крестик шапки success — тот же возврат, что «Хорошо» (#1072);
      // локатор скоуплен шапкой: «Закрыть» есть и у push-алертов.
      await screenHeader(page).getByRole('button', { name: 'Закрыть' }).click();
      await expect(page).toHaveURL(new RegExp(`${rentalPageUrl}$`));
      await expect(page.getByText('Оплачено', { exact: true })).toBeVisible();

      // ── «Посмотреть платеж» остаётся каноном: success ведёт на правило ──
      await page.goto(rentalPageUrl);
      await page.getByRole('button', { name: 'Оплатить платеж' }).click();
      await expect(page).toHaveURL(operationUrlPattern(APARTMENT));
      await payFromOperationScreen(page);
      await page.getByRole('button', { name: 'Посмотреть платеж' }).click();
      await expect(page).toHaveURL(
        new RegExp(`/properties/${APARTMENT}/payments/${rentPaymentId}$`),
      );
    } finally {
      await execE2eSql(`DELETE FROM rentals WHERE id = '${rental.id}'`);
      await execE2eSql(`DELETE FROM operations WHERE payment_id = '${rentPaymentId}'`);
      await execE2eSql(`DELETE FROM payment_pauses WHERE payment_id = '${rentPaymentId}'`);
      await execE2eSql(`DELETE FROM payments WHERE id = '${rentPaymentId}'`);
    }
  });
});
