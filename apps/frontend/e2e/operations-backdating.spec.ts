import {
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
  todayIso,
  passOperationDateStep,
} from './fixtures';
import type { Locator, Page } from '@playwright/test';

// Шаг «Выберите дату» визарда операции (#1394, карта #1390): между суммой
// и названием, экран целиком CalendarDatePicker — «сегодня владельца»
// предвыбрано, будущее закрыто (maxDate), прошлое открыто без пола
// (#1393). Сид-владелец живёт в UTC (canon #796) — календарь профиля
// совпадает с todayIso(). Задне-датированный день берём в предыдущем
// месяце (лента дорисовывается назад по доскроллу — стартовая позиция
// уже на «сегодня»), 15-е число — всегда строго раньше сегодняшнего. Сумма 600 ₽
// и DELETE в finally — как в double-submit-guard.spec.ts: соседние спеки
// операций держат точные суммы сводки. SQL-правда: выбранная дата
// пишется в обе колонки факта (date = paid_date, #1392).

const MONTH_LABELS = [
  'Январь',
  'Февраль',
  'Март',
  'Апрель',
  'Май',
  'Июнь',
  'Июль',
  'Август',
  'Сентябрь',
  'Октябрь',
  'Ноябрь',
  'Декабрь',
];

function monthShifted(today: string, shift: number): { heading: string; iso15: string } {
  const date = new Date(`${today}T00:00:00Z`);
  const month0 = (date.getUTCMonth() + shift + 12) % 12;
  const year =
    date.getUTCFullYear() + Math.floor((date.getUTCMonth() + shift) / 12);
  return {
    heading: `${MONTH_LABELS[month0]}, ${year}`,
    iso15: `${year}-${String(month0 + 1).padStart(2, '0')}-15`,
  };
}

const CALENDAR_NAME = 'Выберите дату';

/** Открывает шаг даты после суммы и выбирает 15-е число предыдущего
 * месяца (доскоролл ленты вверх дорисовывает месяцы назад). */
async function pickPreviousMonth15th(page: Page, calendar: Locator): Promise<void> {
  await expect(calendar).toBeVisible();

  // Дефолт — «сегодня владельца» предвыбрано (ячека today выделена), а
  // будущее закрыто: 1-е число следующего месяца погашено.
  const todayCell = calendar.locator('button[aria-current="date"]');
  await expect(todayCell).toBeVisible();
  await expect(todayCell).toHaveClass(/bg-primary/);
  const previous = monthShifted(todayIso(), -1);
  const next = monthShifted(todayIso(), 1);
  await expect(
    calendar
      .locator('section')
      .filter({ hasText: next.heading })
      .getByRole('button', { name: '1', exact: true }),
  ).toBeDisabled();

  // Лента стартует порцией раньше «сегодня» (решение владельца 13.10):
  // прокрутка вверх работает первым же движением колеса — дорисовка назад
  // не требует «качания».
  await calendar.locator('.overflow-y-auto').hover();
  await page.mouse.wheel(0, -800);
  await expect(calendar.getByText(previous.heading)).toBeVisible();
  await calendar
    .locator('section')
    .filter({ hasText: previous.heading })
    .getByRole('button', { name: '15', exact: true })
    .click();
  await passOperationDateStep(page);
}

async function sqlOperationDates(title: string): Promise<{ date: string; paidDate: string }> {
  const row = await execE2eSql(
    `SELECT date::text || ' ' || paid_date::text FROM operations WHERE title = '${title}'`,
  );
  const [date, paidDate] = row.split(' ');
  if (date === undefined || paidDate === undefined) {
    throw new Error(`unexpected SQL row for «${title}»: ${row}`);
  }
  return { date, paidDate };
}

test.describe('операция задним числом — шаг «Выберите дату» (#1394)', () => {
  test('глобальный вход: задне-датированная операция пишет выбранную дату в date и paid_date', async ({
    page,
    seededUser,
  }) => {
    test.setTimeout(120_000);
    await openCabinetWithSeededSession(page, seededUser);
    const title = `Задне-операция глобально ${Date.now()}`;
    try {
      await page.goto('/operations/new');
      await page.getByRole('textbox', { name: 'Сумма' }).fill('600');
      await page.getByRole('button', { name: 'Продолжить' }).click();
      const calendar = page.getByRole('dialog', { name: CALENDAR_NAME });
      await expect(calendar).toBeVisible();

      // Жёсткий край ленты (решение владельца 13.10): за парой приглушённых
      // месяцев после «сегодня» ленты нет.
      await expect(calendar.getByText(monthShifted(todayIso(), 2).heading)).toBeVisible();
      await expect(calendar.getByText(monthShifted(todayIso(), 3).heading)).toHaveCount(0);

      // Колесо годов зажато по «сегодня»: будущего в шите месяца нет
      // (шит — отдельный диалог-портал, не внутри пикера).
      const current = monthShifted(todayIso(), 0);
      const year = Number(todayIso().slice(0, 4));
      await calendar
        .getByRole('button', { name: current.heading.replace(', ', ' ') })
        .click();
      const sheet = page.getByRole('dialog', { name: 'Месяц и год' });
      await expect(sheet).toBeVisible();
      await expect(sheet.getByRole('option', { name: String(year) })).toBeVisible();
      await expect(sheet.getByRole('option', { name: String(year + 1) })).toHaveCount(0);
      await sheet.getByRole('button', { name: 'Отменить' }).click();

      await pickPreviousMonth15th(page, calendar);

      // Название шага названия — уникальный title для SQL-правды и уборки.
      await page.getByRole('textbox').fill(title);
      await page.getByRole('button', { name: 'Продолжить' }).click();
      const rows = page.locator('div.flex.flex-col.pt-6').getByRole('button');
      await expect(rows.last()).toHaveAccessibleName('Другое');
      await rows.last().click();
      await page.getByRole('button', { name: 'Продолжить' }).click();
      await page.getByRole('radio', { name: 'Квартира на Ленина' }).click();
      await page.getByRole('button', { name: 'Добавить операцию' }).click();

      await expect(page.getByRole('heading', { name: title })).toBeVisible();
      const expected = monthShifted(todayIso(), -1).iso15;
      const { date, paidDate } = await sqlOperationDates(title);
      expect(date).toBe(expected);
      expect(paidDate).toBe(expected);
    } finally {
      await execE2eSql(`DELETE FROM operations WHERE title = '${title}'`);
    }
  });

  test('вход с объекта: та же дата уходит в команду и рождает факт обеими колонками', async ({
    page,
    seededUser,
  }) => {
    test.setTimeout(120_000);
    await openCabinetWithSeededSession(page, seededUser);
    const title = `Задне-операция с объекта ${Date.now()}`;
    try {
      await page.goto(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}/operations/new`);
      await page.getByRole('textbox', { name: 'Сумма' }).fill('600');
      await page.getByRole('button', { name: 'Продолжить' }).click();
      await pickPreviousMonth15th(page, page.getByRole('dialog', { name: CALENDAR_NAME }));

      // Шаг названия, затем категория — последний шаг входа с объекта:
      // на ней сабмит.
      await page.getByRole('textbox').fill(title);
      await page.getByRole('button', { name: 'Продолжить' }).click();
      const rows = page.locator('div.flex.flex-col.pt-6').getByRole('button');
      await expect(rows.last()).toHaveAccessibleName('Другое');
      await rows.last().click();
      await page.getByRole('button', { name: 'Добавить операцию' }).click();

      await expect(page.getByRole('heading', { name: title })).toBeVisible();
      const expected = monthShifted(todayIso(), -1).iso15;
      const { date, paidDate } = await sqlOperationDates(title);
      expect(date).toBe(expected);
      expect(paidDate).toBe(expected);
    } finally {
      await execE2eSql(`DELETE FROM operations WHERE title = '${title}'`);
    }
  });
});
