import {
  expect,
  openCabinetWithSeededSession,
  SEEDED_STUDIO_PROPERTY_ID,
  test,
} from './fixtures';
import { formatDayMonth, formatDayMonthWithYear, formatDayMonthYear } from '@/shared/lib/date-format';
import type { Page, TestInfo } from '@playwright/test';

// Операционные ленты читаются по фактической дате оплаты (#933/#994):
// платёж, оплаченный сегодня за будущий плановый период, стоит в группе
// дня оплаты («Сегодня»), а не плановой даты — банковский порядок
// «последний выполненный — первым». До #994 сортировка и группировка шли
// по плановой дате: оплаченные наперёд месяцы висели над сегодняшними.
//
// Правило «каждый месяц, 1-е число» создаётся визардом на Студии на
// Полевой: сидово у студии нет оплаченных операций (только просрочки
// аренды, лента paid-only их не показывает), поэтому лента объекта
// содержит ровно одну операцию прогона и группа заведомо одна; прогон не
// трогает гараж — его «Операций еще не было» (#478) читают другие спеки
// (workers: 1, общий сид на прогон). Даты не считаются на клиенте:
// плановая (F) и факт оплаты читаются из API того же сеанса; негативное
// «группы плановой даты нет» проверяется только когда F строго в будущем
// — спецификация не зависит от календаря прогона.

const PROPERTY = SEEDED_STUDIO_PROPERTY_ID;
const OPERATIONS_URL = `/properties/${PROPERTY}/operations`;

interface PaymentFromApi {
  readonly id: string;
  readonly title: string;
}

interface OperationFromApi {
  readonly id: string;
  readonly date: string;
  readonly paidDate: string | null;
}

/** Сегодня глазами браузера: группировка ленты — клиентская (та же
 * оговорка о TZ, что во всех спецификациях зоны). Это не сидовый todayAt
 * (SQL-выражение для seeds) — здесь читается ровно тот «сегодня», от
 * которого лента строит «Сегодня»/«Вчера». */
async function browserToday(page: Page): Promise<string> {
  return page.evaluate(() => {
    const now = new Date();
    const month = String(now.getMonth() + 1).padStart(2, '0');
    const day = String(now.getDate()).padStart(2, '0');
    return `${now.getFullYear()}-${month}-${day}`;
  });
}

test.describe('операционные ленты по факту оплаты', () => {
  test.use({ viewport: { width: 390, height: 844 } });
  test.setTimeout(240_000);

  test('оплаченный вперёд платёж за будущий период виден по дате оплаты', async ({
    page,
    seededUser,
  }, testInfo: TestInfo) => {
    const run = String(testInfo.retry);
    const title = `E2E оплата по факту ${run}`;

    // ── Создание правила «каждый месяц, 1-е число» визардом с хаба гаража ──
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`/properties/${PROPERTY}/payments`);
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

    const response = await page.request.get(`/api/properties/${PROPERTY}/payments`);
    expect(response.ok()).toBe(true);
    const { items } = (await response.json()) as { items: ReadonlyArray<PaymentFromApi> };
    const created = items.find((payment) => payment.title === title);
    if (created === undefined) {
      throw new Error(`created payment «${title}» is missing from the API list`);
    }

    // Ближайшее плановое вхождение F — из API того же сеанса.
    const plannedOperationsUrl =
      `/api/properties/${PROPERTY}/payments/${created.id}/operations?status=planned&order=asc`;
    let plannedDate = '';
    await expect.poll(async () => {
      const planned = await page.request.get(plannedOperationsUrl);
      const { items: plannedItems } = (await planned.json()) as {
        items: ReadonlyArray<OperationFromApi>;
      };
      plannedDate = plannedItems[0]?.date ?? '';
      return plannedItems.length;
    }).toBeGreaterThanOrEqual(1);

    // ── Предоплата ближайшего вхождения: факт оплаты = сегодня ──
    await page.goto(`/properties/${PROPERTY}/payments/${created.id}`);
    await page.getByRole('button', { name: 'Оплатить' }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/operations/[0-9a-f-]+$`));
    await page.getByRole('button', { name: 'Отметить оплаченной' }).click();
    await expect(page.getByText('Платеж оплачен')).toBeVisible();
    await page.getByRole('button', { name: 'Хорошо', exact: true }).click();

    const paidOperationsUrl =
      `/api/properties/${PROPERTY}/payments/${created.id}/operations?status=paid&order=desc`;
    let paidOperation: OperationFromApi | undefined;
    await expect.poll(async () => {
      const paid = await page.request.get(paidOperationsUrl);
      const { items: paidItems } = (await paid.json()) as {
        items: ReadonlyArray<OperationFromApi>;
      };
      paidOperation = paidItems.find((operation) => operation.date === plannedDate);
      return paidOperation !== undefined && paidOperation.paidDate !== null;
    }).toBe(true);
    const paidDate = paidOperation?.paidDate ?? '';
    expect(paidDate).not.toBe('');

    const today = await browserToday(page);

    // Групповые лейблы проверяются по роли заголовка: getByRole не считает
    // скрытые поддеревья (гонка двойного DOM оставляет фантомные копии
    // страницы — строгие page-wide getByText на них падают, прогон 30.09).
    // Лейблы лент — «Сегодня, <дата>», истории — «Сегодня» (канон истории,
    // без даты через запятую).
    const feedGroupLabel = paidDate === today
      ? `Сегодня, ${formatDayMonth(paidDate)}`
      : formatDayMonthWithYear(paidDate, today);
    const historyGroupLabel = paidDate === today
      ? 'Сегодня'
      : formatDayMonthYear(paidDate);
    const plannedGroupHeading = formatDayMonthWithYear(plannedDate, today);

    // ── Лента объекта: операция — в группе дня оплаты, не плановой даты ──
    await page.goto(OPERATIONS_URL);
    const row = page.getByText(title).first();
    await expect(row).toBeVisible();
    // Оплаченный наперёд платёж лежит в группе факта; когда плановая дата
    // строго в будущем (F ≠ сегодня), группы плановой даты в ленте нет.
    if (plannedDate > today) {
      await expect(
        page.getByRole('heading', { name: plannedGroupHeading, exact: true }),
      ).toHaveCount(0);
    }
    await expect(
      page.getByRole('heading', { name: feedGroupLabel, exact: true }),
    ).toBeVisible();

    // ── Глобальная лента (#933 — surfaces владельца): та же группа факта ──
    await page.goto('/operations');
    await expect(page.getByText(title).first()).toBeVisible();
    if (plannedDate > today) {
      await expect(
        page.getByRole('heading', { name: plannedGroupHeading, exact: true }),
      ).toHaveCount(0);
    }
    await expect(
      page.getByRole('heading', { name: feedGroupLabel, exact: true }),
    ).toBeVisible();

    // ── История платежа: та же операция — в группе дня оплаты (дополнение
    // #994, отмена «учёт, не кассы» #466): сорт сервера sort=paid_date,
    // группировка по факту; плановой группы нет ──
    await page.goto(`/properties/${PROPERTY}/payments/${created.id}/history`);
    await expect(page.getByText(title).first()).toBeVisible();
    if (plannedDate > today) {
      await expect(
        page.getByRole('heading', { name: plannedGroupHeading, exact: true }),
      ).toHaveCount(0);
    }
    await expect(
      page.getByRole('heading', { name: historyGroupLabel, exact: true }),
    ).toBeVisible();
  });
});
