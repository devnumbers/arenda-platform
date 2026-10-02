import {
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_GARAGE_PROPERTY_ID,
  test,
} from './fixtures';
import { formatDayMonth } from '@/shared/lib/date-format';
import type { Page, TestInfo } from '@playwright/test';

// Следующая дата оплаты — серверная истина на «Платежах объекта» (#993,
// баг #967): предоплата ближайшего вхождения переставляет плановую на
// следующую, и хаб со страницей секции показывают уже новую дату — ту же,
// что «Ближайшая операция» страницы платежа. До #993 хаб проецировал
// правило клиентским портом от «сегодня» устройства: оплаченное будущее
// вхождение он рисовал бы заново (старая дата), ADR 0048.
//
// Правило создаётся визардом «каждый месяц, 1-е число» на Гараже на
// Садовой — сидово без платежей, поэтому единственная строка правила
// заведомо видна в лимите секции хаба (3) и на странице секции. Ожидаемые
// даты не считаются на клиенте: первая (F) и следующая за предоплатой (F')
// читаются из API плановых операций того же сеанса — спецификация
// проверяет равенство показанной даты серверной в обоих моментах, без
// привязки к календарю прогона и к часам устройства.

const PROPERTY = SEEDED_GARAGE_PROPERTY_ID;
const PAYMENTS_URL = `/properties/${PROPERTY}/payments`;

interface PaymentFromApi {
  readonly id: string;
  readonly title: string;
}

interface OperationFromApi {
  readonly id: string;
  readonly date: string;
}

/** Строчка правила на поверхности: подзаголовок-дата — единственная дата
 * строки (PaymentRow не несёт description), точное совпадение текста. */
function rowDate(page: Page, dateLabel: string): ReturnType<Page['getByText']> {
  return page.getByTestId('section-payments').getByText(dateLabel, { exact: true });
}

test.describe('следующая дата оплаты из серверного nearestDate', () => {
  test.use({ viewport: { width: 390, height: 844 } });
  test.setTimeout(240_000);

  test('предоплата ближайшего вхождения двигает дату на хабе, странице секции и странице платежа одинаково', async ({
    page,
    seededUser,
  }, testInfo: TestInfo) => {
    const run = String(testInfo.retry);
    const title = `E2E следующая дата ${run}`;

    // Гараж — сидово без платежей: правило убирается в любом исходе, иначе
    // оно переживает прогон и ломает пустые состояния payments.spec.
    try {
    // ── Создание правила «каждый месяц, 1-е число» визардом с хаба гаража ──
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(PAYMENTS_URL);
    await expect(page.getByRole('button', { name: 'Добавить' })).toBeVisible();
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
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeDisabled();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('1990');
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);
    await page.getByRole('button', { name: 'Хорошо, закрыть' }).click();
    await expect(page).toHaveURL(new RegExp(`${PAYMENTS_URL}$`));

    const response = await page.request.get(`/api/properties/${PROPERTY}/payments`);
    expect(response.ok()).toBe(true);
    const { items } = (await response.json()) as { items: ReadonlyArray<PaymentFromApi> };
    const created = items.find((payment) => payment.title === title);
    if (created === undefined) {
      throw new Error(`created payment «${title}» is missing from the API list`);
    }

    // Ближайшее вхождение F — из API: мутационный тик материализовал его
    // при создании; хаб показывает ровно эту серверную дату (#993).
    const plannedOperationsUrl =
      `/api/properties/${PROPERTY}/payments/${created.id}/operations?status=planned&order=asc`;
    let firstOccurrence = '';
    await expect.poll(async () => {
      const planned = await page.request.get(plannedOperationsUrl);
      const { items: plannedItems } = (await planned.json()) as {
        items: ReadonlyArray<OperationFromApi>;
      };
      firstOccurrence = plannedItems[0]?.date ?? '';
      return plannedItems.length;
    }).toBeGreaterThanOrEqual(1);

    const beforeLabel = formatDayMonth(firstOccurrence);
    await expect(rowDate(page, beforeLabel)).toBeVisible();

    // ── Предоплата ближайшего (будущего) вхождения: «Оплатить» на странице
    // платежа гасит F, мутационный тик материализует следующее ──
    await rowDate(page, beforeLabel).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/payments/${created.id}$`));
    await page.getByRole('button', { name: 'Оплатить' }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/operations/[0-9a-f-]+(\\?.*)?$`));
    await page.getByRole('button', { name: 'Отметить оплаченной' }).click();
    await expect(page.getByText('Платеж оплачен')).toBeVisible();
    await page.getByRole('button', { name: 'Хорошо', exact: true }).click();
    // Вход был строкой ближайшего (без ?returnTo=) — фолбэк #1072 ведёт
    // на страницу правила, контекст операции.
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+$`));

    // Новое ближайшее F' — серверная истина после предоплаты: тик
    // переставил плановую строго вперёд от оплаченной.
    let nextOccurrence = '';
    await expect.poll(async () => {
      const planned = await page.request.get(plannedOperationsUrl);
      const { items: plannedItems } = (await planned.json()) as {
        items: ReadonlyArray<OperationFromApi>;
      };
      nextOccurrence = plannedItems[0]?.date ?? '';
      return nextOccurrence > firstOccurrence;
    }).toBe(true);
    const afterLabel = formatDayMonth(nextOccurrence);

    // ── Страница платежа («Ближайшая операция») и хаб — одна и та же новая
    // дата; старой (оплаченной) на поверхностях нет — регрессия #967 ──
    await page.goto(`/properties/${PROPERTY}/payments/${created.id}`);
    // Секция — по её уникальному видимому заголовку-кнопке, дата — внутри
    // этой секции: getByRole/has не считают скрытые поддеревья (гонка
    // двойного DOM оставляла фантомные aria-hidden копии, строгие
    // page-wide getByText падали на двух совпадениях, прогоны 30.09);
    // реальный дубль секции дал бы два видимых и упал бы так же.
    const nearestSection = page.locator('section').filter({
      has: page.getByRole('button', { name: 'Открыть график платежей' }),
    });
    await expect(
      nearestSection.getByRole('button', { name: 'Открыть график платежей' }),
    ).toBeVisible();
    await expect(nearestSection.getByText(afterLabel, { exact: true })).toBeVisible();

    await page.goto(PAYMENTS_URL);
    await expect(rowDate(page, afterLabel)).toBeVisible();
    await expect(rowDate(page, beforeLabel)).toHaveCount(0);

    await page.goto(`${PAYMENTS_URL}/all`);
    await expect(page.getByText(afterLabel, { exact: true })).toBeVisible();
    } finally {
      await execE2eSql(
        `DELETE FROM operations WHERE payment_id IN ` +
          `(SELECT id FROM payments WHERE property_id = '${SEEDED_GARAGE_PROPERTY_ID}' AND title = '${title}')`,
      );
      await execE2eSql(
        `DELETE FROM payments WHERE property_id = '${SEEDED_GARAGE_PROPERTY_ID}' AND title = '${title}'`,
      );
    }
  });
});
