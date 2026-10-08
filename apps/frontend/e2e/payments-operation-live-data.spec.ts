import {
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';
import type { Page } from '@playwright/test';

// Данные операции на странице операции (#1196, решение владельца — поправка
// 08.10 к #1190): страница показывает снимок операции — название, категорию
// и иконку, с которыми операция материализовалась; правка правила её не
// меняет. Живое правило представлено только плашкой «Платеж» в «Данных
// операции» — актуальные название и картинка из живой детали платежа.
// Экран успеха «Платеж оплачен» показывает название самой операции (снимок).
// Ручная операция — свои данные (строки «Платеж» нет); сравнение плановой и
// фактической дат («Задержана на») считается по датам операции.
//
// Правило создаётся визардом «на сегодня» — первое вхождение материализуется
// тиком (канон payment-lifecycle); названия уникальны за попытку (суффикс —
// номер retry). Просрочку сеет SQL середины теста — execE2eSql: статус
// просрочки считает сервер по «сегодня» собственника (ADR 0048).

const PROPERTY = SEEDED_APARTMENT_PROPERTY_ID;
const PAYMENTS_URL = `/properties/${PROPERTY}/payments`;

interface NamedFromApi {
  readonly id: string;
  readonly title: string;
}

/** Создаёт месячное правило «на сегодня» (первое вхождение материализуется
 * тиком) и возвращает его id из API. Канон payment-lifecycle: путь
 * пользователя через шит «Добавить», уникальное название за попытку. */
async function createMonthlyPaymentToday(
  page: Page,
  seededUser: Parameters<typeof openCabinetWithSeededSession>[1],
  title: string,
): Promise<string> {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(PAYMENTS_URL);
  await expect(page.getByRole('button', { name: 'Добавить' })).toBeVisible();
  await page.getByRole('button', { name: 'Добавить' }).click();
  await page.getByRole('button', { name: /Платеж Отмечайте оплату/ }).click();
  await expect(page.getByRole('heading', { name: 'Выберите категорию платежа' })).toBeVisible();

  // Шаг 1 — категория, шаг 2 — название.
  await page.getByRole('button', { name: 'Интернет', exact: true }).click();
  await page.getByRole('button', { name: 'Продолжить' }).click();
  await expect(page.getByRole('heading', { name: 'Назовите платеж' })).toBeVisible();
  await page.getByRole('textbox').fill(title);
  await page.getByRole('button', { name: 'Продолжить' }).click();

  // Шаг 3 — ежемесячно, день = сегодня (31-е — маркером «последний день»).
  const day = String(new Date().getDate());
  await page.getByRole('button', { name: 'Каждый месяц' }).click();
  await expect(page.getByRole('heading', { name: 'Выберите день', exact: true })).toBeVisible();
  if (day === '31') {
    await page.getByRole('button', { name: 'Последний день месяца' }).click();
  } else {
    await page.getByRole('button', { name: day, exact: true }).first().click();
  }
  await page.getByRole('button', { name: 'Продолжить' }).click();

  // Шаг 4 — напоминание не задаём; шаг 5 — сумма и направление.
  await expect(page.getByRole('heading', { name: 'За сколько напомнить об оплате' })).toBeVisible();
  await page.getByRole('button', { name: 'Далее' }).click();
  await page.getByRole('textbox', { name: 'Сумма' }).fill('1990');
  await page.getByRole('radio', { name: 'Расход' }).click();
  await page.getByRole('button', { name: 'Создать платеж' }).click();
  await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(
    `«${title}»`,
  );
  await page.getByRole('button', { name: 'Хорошо, закрыть' }).click();
  await expect(page).toHaveURL(new RegExp(`${PAYMENTS_URL}$`));

  const response = await page.request.get(`/api/properties/${PROPERTY}/payments`);
  expect(response.ok()).toBe(true);
  const { items } = (await response.json()) as { items: ReadonlyArray<NamedFromApi> };
  const created = items.find((payment) => payment.title === title);
  if (created === undefined) {
    throw new Error(`created payment «${title}» is missing from the API list`);
  }
  return created.id;
}

test.describe('данные операции на странице операции (#1196)', () => {
  test.use({ viewport: { width: 390, height: 844 } });
  test.setTimeout(240_000);

  test('операция держит снимок после правки правила; плашка «Платеж» живая; успех — снимок; «Задержана на» на месте', async ({
    page,
    seededUser,
  }, testInfo) => {
    const run = String(testInfo.retry);
    const oldTitle = `E2E снимок операции было ${run}`;
    const newTitle = `E2E снимок операции стало ${run}`;
    const induceOverdue = (paymentId: string, daysAgo: number): string =>
      `UPDATE operations SET date = CURRENT_DATE - ${daysAgo} WHERE id = (`
      + `SELECT id FROM operations WHERE payment_id = '${paymentId}' `
      + `AND status = 'planned' ORDER BY date ASC LIMIT 1)`;

    const id = await createMonthlyPaymentToday(page, seededUser, oldTitle);
    const paymentUrl = `${PAYMENTS_URL}/${id}`;

    // ── Правка правила: новое название и другая категория. Вход через
    // «Изменить» со страницы платежа — сохранение возвращает назад по
    // истории (goBack), как в payment-lifecycle ──
    await page.goto(paymentUrl);
    await page.getByRole('button', { name: 'Изменить' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/edit$`));
    await page.getByRole('textbox', { name: 'Название платежа' }).fill(newTitle);
    await page.getByRole('button', { name: 'Категория' }).click();
    await expect(page.getByRole('heading', { name: 'Выберите категорию платежа' })).toBeVisible();
    await page.getByRole('button', { name: 'Страхование', exact: true }).click();
    await page.getByRole('button', { name: 'Готово' }).click();
    await expect(page.getByRole('button', { name: 'Категория' })).toHaveText(/Страхование/);
    await page.getByRole('button', { name: 'Сохранить изменения' }).click();
    await expect(page.getByText('Изменения сохранены')).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`${paymentUrl}$`));

    // ── Страница операции: снимок операции (старые название и категория),
    // живое правило — только в плашке «Платеж» ──
    await page.goto(paymentUrl);
    await page.getByRole('button', { name: 'Оплатить' }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/operations/[0-9a-f-]+(\\?.*)?$`));
    await expect(page.getByText(oldTitle, { exact: true })).toHaveCount(1); // hero — снимок
    await expect(page.getByText('Интернет', { exact: true })).toBeVisible(); // чип-снимок
    await expect(page.getByRole('button', { name: `${newTitle} Платеж` })).toBeVisible(); // живая плашка
    await expect(page.getByText(newTitle, { exact: true })).toHaveCount(1);
    await expect(page.getByText('Страхование')).toHaveCount(0); // живая категория не утекает
    await expect(page.getByText('Запланирована', { exact: true })).toBeVisible();

    // ── Оплата: экран успеха показывает название самой операции (снимок) ──
    await page.getByRole('button', { name: 'Отметить оплаченной' }).click();
    await expect(page.getByText('Платеж оплачен')).toBeVisible();
    await expect(page.getByText(`«${oldTitle}»`)).toBeVisible();
    await page.getByRole('button', { name: 'Хорошо', exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+$`));

    // ── Просрочка: расхождение плановой и фактической дат видно — подпись
    // под суммой, строка «Задержана на», статус ──
    expect(await execE2eSql(induceOverdue(id, 3))).toBe('UPDATE 1');
    await page.goto(paymentUrl);
    await page.getByRole('button', { name: 'Оплатить' }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/operations/[0-9a-f-]+(\\?.*)?$`));
    await expect(page.getByText('Просрочена', { exact: true })).toBeVisible();
    await expect(page.getByText('просрочена на 3 дня')).toBeVisible();
    await expect(page.getByText('Задержана на')).toBeVisible();
    await expect(page.getByText('3 дня', { exact: true })).toBeVisible();
  });

  test('ручная операция показывает свои данные — без строки «Платеж»', async ({
    page,
    seededUser,
  }, testInfo) => {
    const run = String(testInfo.retry);
    const title = `E2E ручная операция ${run}`;

    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`/properties/${PROPERTY}/operations/new`);
    await expect(page.getByText('Добавить операцию')).toBeVisible();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('1200');
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await expect(page.getByRole('heading', { name: 'Что хотите добавить?' })).toBeVisible();
    await page.getByRole('textbox', { name: 'Название операции' }).fill(title);
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await expect(page.getByRole('heading', { name: 'Категория операции' })).toBeVisible();
    await page.getByRole('button', { name: 'Интернет', exact: true }).click();
    await page.getByRole('button', { name: 'Добавить операцию' }).click();
    await expect(page.getByRole('heading', { name: title })).toBeVisible();
    await page.getByRole('button', { name: 'Готово' }).click();

    // Страница операции: ручной факт рождается оплаченным, правила у него
    // нет — строки «Платеж» в «Данных операции» нет, в «Подробнее» —
    // «Дата операции» и статус.
    const response = await page.request.get(`/api/properties/${PROPERTY}/operations`);
    expect(response.ok()).toBe(true);
    const { items } = (await response.json()) as { items: ReadonlyArray<NamedFromApi> };
    const created = items.find((operation) => operation.title === title);
    if (created === undefined) {
      throw new Error(`created manual operation «${title}» is missing from the API list`);
    }
    await page.goto(`/properties/${PROPERTY}/operations/${created.id}`);
    await expect(page.getByText(title)).toBeVisible();
    await expect(page.getByText('Платеж', { exact: true })).toHaveCount(0);
    await expect(page.getByText('Дата операции')).toBeVisible();
    await expect(page.getByText('Выполнена', { exact: true })).toBeVisible();
  });
});
