import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_STUDIO_PROPERTY_ID,
  test,
} from './fixtures';
import type { Locator, Page } from '@playwright/test';

// Подэкраны страницы платежа (#466): «График платежей» (материализованное
// ближайшее + клиентская проекция, порции по 50, пустые состояния «На
// паузе»/«Платеж завершен»), «История платежей» (paid, группы «Сегодня»/
// «Вчера»/дата с годом, чип «Сначала новые», серверная пагинация 50 +
// скролл-догрузка) и
// полный список просроченных (red-стилизация, пагинация 50). Скриншоты —
// материал для сверки с Figma (671:7358 график, 1302:52209 история — канон
// #802; полный
// список просроченных спроектирован по решению владельца — резолюция #452).
//
// Сид (#466): квартира — …551 аренда (просрочка + плановое), …554 «Домофон»
// на паузе, …555 завершённый, …556 «Интернет» (55 paid — догрузка истории),
// …557 «Парковка» (ежедневный до +120 дней — догрузка проекции); студия —
// …558 «Аренда студии» (55 просрочек — догрузка полного списка). Файл не
// мутирует сид (оплата аренды происходит в payment-detail.spec, который
// выполняется раньше): порядок проверок устойчив.

const PROPERTY = SEEDED_APARTMENT_PROPERTY_ID;
const PAYMENT_URLS = {
  rent: `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555551`,
  intercomPaused: `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555554`,
  completed: `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555555`,
  internet: `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555556`,
  parking: `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555557`,
};
const STUDIO_RENT = `/properties/${SEEDED_STUDIO_PROPERTY_ID}/payments/55555555-5555-4555-8555-555555555558`;

/** Год даты, отстоящей на `monthsAhead` месяцев от «сейчас» браузера —
 * подписи дальних строк проекции и старых групп истории содержат год. */
function yearShiftedBy(monthsAhead: number): number {
  const now = new Date();
  return new Date(now.getFullYear(), now.getMonth() + monthsAhead, 1).getFullYear();
}

/** Первая порция списка — ровно 50 (правило платформы). */
async function expectFirstPage(rows: Locator): Promise<void> {
  await expect(rows.first()).toBeVisible();
  await expect.poll(() => rows.count(), { timeout: 15_000 }).toBe(50);
}

/** Скролл к sentinel-догрузке: строк становится не меньше `atLeast`. */
async function expectMoreAfterScroll(
  page: Page,
  rows: Locator,
  atLeast: number,
): Promise<void> {
  await page.mouse.wheel(0, 200_000);
  await expect
    .poll(() => rows.count(), { timeout: 15_000 })
    .toBeGreaterThanOrEqual(atLeast);
}

test.describe('подэкран «График платежей»', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('вход по плитке; «Ближайший» из операций и проекция «Следующих»; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(PAYMENT_URLS.rent);

    await page.getByRole('button', { name: 'График платежей', exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/schedule$`));
    await expect(page.getByText('График платежей').first()).toBeVisible();

    // «Ближайший» — материализованное плановое вхождение (1-е число
    // следующего месяца по сиду аренды); сервер — источник истины.
    await expect(page.getByText('Ближайший').first()).toBeVisible();
    await expect(
      page.getByText(/1 (сентября|октября|ноября|декабря|января|февраля|марта|апреля|мая|июня|июля|августа)/).first(),
    ).toBeVisible();

    // «Следующие» — проекция; дальние строки содержат год.
    await expect(page.getByText('Следующие').first()).toBeVisible();
    const rentRows = page.getByText('Арендная плата');
    await expect(rentRows.first()).toBeVisible();

    await captureScreen(page, testInfo, 'payment-schedule-mobile');
  });

  test('скролл-догрузка проекции: 50 → больше 50', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(PAYMENT_URLS.parking);

    await page.getByRole('button', { name: 'График платежей', exact: true }).click();
    const parkingRows = page.getByText('Парковка');
    await expectFirstPage(parkingRows);
    await expectMoreAfterScroll(page, parkingRows, 51);
  });

  test('правило на паузе показывает «На паузе»; завершённое — «Платеж завершен»', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`${PAYMENT_URLS.intercomPaused}/schedule`);
    await expect(page.getByText('На паузе', { exact: true })).toBeVisible();
    await captureScreen(page, testInfo, 'payment-schedule-paused-mobile');

    await page.goto(`${PAYMENT_URLS.completed}/schedule`);
    await expect(page.getByText('Платеж завершен')).toBeVisible();
  });
});

test.describe('подэкран «История платежей»', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('группы «Сегодня»/«Вчера»/дата, минус у расходов; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(PAYMENT_URLS.internet);

    await page.getByRole('button', { name: 'История операций', exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/history$`));

    await expect(page.getByText('Сегодня', { exact: true })).toBeVisible();
    await expect(page.getByText('Вчера', { exact: true })).toBeVisible();
    // Порция 50 из 55: дальше «Вчера» — группы-даты (день совпадает с датой
    // прогона, сид режет от CURRENT_DATE).
    await expect(
      page.getByText(/\d{1,2} (января|февраля|марта|апреля|мая|июня|июля|августа|сентября|октября|ноября|декабря)/).first(),
    ).toBeVisible();

    // Сумма расхода — с минусом (Figma 671:7776). Подписей-дат в строках
    // больше нет (канон 1302:52209, решение #802) — «Заранее/Задержан»
    // живут на странице операции.
    await expect(page.getByText('-1 000 ₽').first()).toBeVisible();

    await captureScreen(page, testInfo, 'payment-history-mobile');
  });

  test('скролл-догрузка истории: 50 → 55', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`${PAYMENT_URLS.internet}/history`);

    const internetRows = page.getByText('Интернет');
    await expectFirstPage(internetRows);
    await expectMoreAfterScroll(page, internetRows, 55);
  });

  test('чип «Сначала новые» переключает сортировку на «сначала старые»', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`${PAYMENT_URLS.internet}/history`);

    const chip = page.getByRole('button', {
      name: 'Сортировка: сначала новые — переключить на «сначала старые»',
    });
    await expect(chip).toHaveText('Сначала новые');
    await expect(page.getByText('Сегодня', { exact: true })).toBeVisible();

    await chip.click();
    const oldestYear = String(yearShiftedBy(-53));
    const oldestChip = page.getByRole('button', {
      name: 'Сортировка: сначала старые — переключить на «сначала новые»',
    });
    await expect(oldestChip).toHaveText('Сначала старые');
    // Первая группа — самая старая дата (53 месяца назад, год в подписи);
    // «Сегодня» уходит за пределы первой порции.
    await expect(page.getByText(new RegExp(`, ${oldestYear}$`)).first()).toBeVisible();
    await expect(page.getByText('Сегодня', { exact: true })).toHaveCount(0);
  });
});

test.describe('полный список просроченных', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('вход по стрелке секции, red-стилизация строк; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(STUDIO_RENT);

    await page
      .getByRole('button', { name: 'Открыть полный список просроченных' })
      .click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/overdue$`));

    // Строки «Графика» с red-стилизацией: срок «N дней», сумма, danger-бейдж
    // (кнопка строки одна — срок просрочки; у просрочки нет своей страницы).
    await expect(page.getByText(/\d+ (день|дня|дней)/).first()).toBeVisible();
    await expect(page.getByText('30 000 ₽').first()).toBeVisible();

    await captureScreen(page, testInfo, 'payment-overdue-mobile');
  });

  test('скролл-догрузка просроченных: 50 → больше 50', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`${STUDIO_RENT}/overdue`);

    const overdueTitles = page.getByText('Аренда студии');
    await expectFirstPage(overdueTitles);
    await expectMoreAfterScroll(page, overdueTitles, 51);
  });

  test('без просрочек — иллюстрированное «Нет просроченных платежей»', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(
      `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555553/overdue`,
    );

    await expect(page.getByText('Нет просроченных платежей')).toBeVisible();
    await expect(page.getByText('Когда платеж просрочится, он будет здесь')).toBeVisible();
  });
});

test.describe('подэкраны — десктоп', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('график и история: тот же контент в колонке 560; скриншоты', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);

    await page.goto(`${PAYMENT_URLS.parking}/schedule`);
    const parkingRows = page.getByText('Парковка');
    await expectFirstPage(parkingRows);
    await expectMoreAfterScroll(page, parkingRows, 51);
    await captureScreen(page, testInfo, 'payment-schedule-desktop');

    await page.goto(`${PAYMENT_URLS.internet}/history`);
    await expect(page.getByText('Сегодня', { exact: true })).toBeVisible();
    await expect(page.getByText('-1 000 ₽').first()).toBeVisible();
    await captureScreen(page, testInfo, 'payment-history-desktop');
  });

  test('история: догрузка и группы дат; просроченные в колонке 560', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);

    await page.goto(`${PAYMENT_URLS.internet}/history`);
    await expect(page.getByText('Вчера', { exact: true })).toBeVisible();
    const internetRows = page.getByText('Интернет');
    await expectFirstPage(internetRows);
    await expectMoreAfterScroll(page, internetRows, 55);

    await page.goto(`${STUDIO_RENT}/overdue`);
    await expect(page.getByText(/\d+ (день|дня|дней)/).first()).toBeVisible();
    await expect(page.getByText('30 000 ₽').first()).toBeVisible();
    await captureScreen(page, testInfo, 'payment-overdue-desktop');
  });
});
