import {
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';
import type { Page } from '@playwright/test';

// Сквозной сценарий жизни платежа (#468, замыкающий tracer-bullet спеки
// #453): одним прогоном против живого стека — создание визардом → появление
// в списке и на странице → материализация мутационным тиком («График»
// показывает серверное «Ближайший», «Оплатить» остаётся активной после
// оплаты — тик материализовал следующее вхождение) → оплата и запись в
// «Истории» → пауза и возобновление → просрочка и гашение из просроченных →
// правка → удаление без «удалить просроченные» (долг остаётся) и с ним
// (просрочки сносятся вместе с правилом).
//
// Просрочку сеет SQL середины теста — execE2eSql (fixtures.ts): docker exec
// в контейнер postgres e2e-стека, тот же канал, которым оркестратор применяет
// seed.sql. Визард создаёт только будущее (since = сегодня), поэтому прошлое
// правилу приносит только сид: status остаётся 'planned', проекцию просрочки
// считает сервер по «сегодня» собственника (ADR 0048) — тот же приём, что в
// сидовых просрочках seed.sql. Сдвигается ровно одна старейшая planned-
// операция на N дней назад: (payment_id, date) уникален, поэтому глубины
// подобраны так, чтобы не попасть на занятые даты истории оплат.
//
// Названия платежей уникальны за попытку (суффикс — номер retry): повтор
// упавшей попытки в CI создаёт платежи заново, а остатки прошлой попытки
// с другим суффиксом не попадают в проверки по названиям. Сценарий идёт по
// квартире и не трогает сидовые правила — порядок относительно остальных
// спек не важен.

const PROPERTY = SEEDED_APARTMENT_PROPERTY_ID;
const PAYMENTS_URL = `/properties/${PROPERTY}/payments`;

interface PaymentFromApi {
  readonly id: string;
  readonly title: string;
}

/**
 * Создаёт месячное правило с днём месяца = `day`: первое вхождение наступает
 * сразу и материализуется тиком вместе с ровно одним следующим (ADR 0048).
 * Идёт путём пользователя — список объекта → шит «Добавить» → карточка
 * «Платёж» (#463) → визард (#464), — чтобы закрытие экрана успеха вернулось
 * назад по истории на список. Возвращает id созданного правила (из API тем
 * же сеансом).
 */
async function createMonthlyPaymentToday(
  page: Page,
  seededUser: Parameters<typeof openCabinetWithSeededSession>[1],
  title: string,
  day: string,
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

  // Шаг 3 — ежемесячно, день = сегодня. Грид несёт 1..30, поэтому 31-е
  // выбирается маркером «Последний день месяца» (в 31-дневном месяце это
  // тот же сегодняшний день).
  await page.getByRole('button', { name: 'Каждый месяц' }).click();
  await expect(page.getByRole('heading', { name: 'Выберите день', exact: true })).toBeVisible();
  if (day === '31') {
    await page.getByRole('button', { name: 'Последний день месяца' }).click();
  } else {
    await page.getByRole('button', { name: day, exact: true }).first().click();
  }
  await page.getByRole('button', { name: 'Продолжить' }).click();

  // Шаг 4 — напоминание и настройки не задаём.
  await expect(page.getByRole('heading', { name: 'За сколько напомнить об оплате' })).toBeVisible();
  await page.getByRole('button', { name: 'Далее' }).click();

  // Шаг 5 — сумма и направление. Чип-переключатель (макеты суммы карты
  // #1005) показывает текущее значение — по умолчанию «Доход»; клик по
  // чипу ставит альтернативное. «Формы оплаты» на шаге нет — поле
  // снесено из продукта (карта #1005, тикеты #1008/#1009).
  await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeDisabled();
  await page.getByRole('textbox', { name: 'Сумма' }).fill('1990');
  await page.getByRole('radio', { name: 'Расход' }).click(); // Доход → Расход: шаг 5 — радиогруппа «Направление платежа» (0105cee5, канон шага платежа один-в-один с операцией)
  await page.getByRole('button', { name: 'Создать платеж' }).click();

  // Экран успеха с первым вхождением из серверного ответа; закрытие — на
  // список. Канон успеха (1858:105544/105549): описание и сумма —
  // отдельные узлы, сумма со знаком (правка 6, a41617f9).
  await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(
    `«${title}»`,
  );
  await expect(
    page.getByText(new RegExp(day === '31' ? 'Первый платеж .*, далее последний день каждого месяца' : `Первый платеж .*, далее каждый месяц ${day} числа`)),
  ).toBeVisible();
  await expect(page.getByText(`-1\u00A0990 ₽`)).toBeVisible();
  await page.getByRole('button', { name: 'Хорошо, закрыть' }).click();
  await expect(page).toHaveURL(new RegExp(`${PAYMENTS_URL}$`));

  const response = await page.request.get(`/api/properties/${PROPERTY}/payments`);
  expect(response.ok()).toBe(true);
  const { items } = (await response.json()) as { items: ReadonlyArray<PaymentFromApi> };
  const created = items.find((payment) => payment.title === title);
  if (created === undefined) {
    throw new Error(`created payment «${title}» is missing from the API list`);
  }
  return created.id;
}

test.describe('сквозная жизнь платежа', () => {
  test.use({ viewport: { width: 390, height: 844 } });
  test.setTimeout(240_000);

  test('визард → список/страница → оплата/история → пауза → просрочка → правка → удаление', async ({
    page,
    seededUser,
  }, testInfo) => {
    const run = String(testInfo.retry);
    const title1 = `E2E жизнь платежа ${run}`;
    const title2 = `E2E жизнь платежа снос ${run}`;
    // День месяца считаем один раз на весь сценарий: правило создаётся
    // «на сегодня», и регулярность на странице сверяется с тем же числом.
    const day = String(new Date().getDate());
    // Просрочка: старейшая planned-операция правила уезжает на daysAgo дней
    // в прошлое (одна строка — (payment_id, date) уникален).
    const induceOverdue = (paymentId: string, daysAgo: number): string =>
      `UPDATE operations SET date = CURRENT_DATE - ${daysAgo} WHERE id = (`
      + `SELECT id FROM operations WHERE payment_id = '${paymentId}' `
      + `AND status = 'planned' ORDER BY date ASC LIMIT 1)`;

    // ── Создание визардом; появление в API; на страницу — напрямую ──
    // (секции списка показывают максимум 3 ближайших платежа, созданное
    // правило в топ не обязано попадать).
    const id1 = await createMonthlyPaymentToday(page, seededUser, title1, day);
    await page.goto(`/properties/${PROPERTY}/payments/${id1}`);
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/payments/[0-9a-f-]+$`));
    const paymentUrl = page.url();

    // ── Страница платежа: карточка правила + материализация тиком ──
    await expect(page.getByText('Платеж', { exact: true })).toBeVisible();
    await expect(page.getByText(title1).first()).toBeVisible();
    await expect(page.getByText('1 990 ₽').first()).toBeVisible();
    await expect(
      page.getByText(
        day === '31'
          ? 'Последний день каждого месяца'
          : `Каждый месяц ${day} числа`,
      ),
    ).toBeVisible();
    await expect(page.getByText('Ближайший платеж')).toBeVisible();

    // Клик по строке ближайшего ведёт на график (#1073) — плановая
    // материализована тиком создания, страница операции не открывается.
    await page
      .locator('section')
      .filter({ has: page.getByRole('heading', { name: 'Ближайший платеж' }) })
      .getByRole('button')
      .first()
      .click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/schedule$`));
    await page.goBack();
    await expect(page.getByText('Ближайший платеж')).toBeVisible();

    // Избранное: звезда переключается с тостом (путь страницы #465).
    const star = page.getByRole('button', { name: 'Добавить в избранное' });
    await expect(star).toHaveAttribute('aria-pressed', 'false');
    await star.click();
    await expect(page.getByText('Платеж добавлен в избранное')).toBeVisible();
    await expect(
      page.getByRole('button', { name: 'Убрать из избранного' }),
    ).toHaveAttribute('aria-pressed', 'true');
    // Тост добавления (top-center на мобайле) висит поверх звезды в шапке
    // и перехватывает клик, автоухол ставится на паузу без фокуса окна —
    // закрываем его кнопкой (канон contacts-спек), иначе клик звезды не
    // проходит вовсе (гонка прогона).
    await page
      .locator('.Toastify__toast', { hasText: 'Платеж добавлен в избранное' })
      .getByRole('button', { name: 'Закрыть' })
      .click();
    await page.getByRole('button', { name: 'Убрать из избранного' }).click();
    await expect(page.getByText('Платеж больше не в избранном')).toBeVisible();

    // «Ближайший» на графике — материализованная операция с сервера;
    // «Следующие» — клиентская проекция (#466). Вход — строкой ближайшего
    // (плитки подэкранов снесены, #1073).
    await page
      .locator('section')
      .filter({ has: page.getByRole('heading', { name: 'Ближайший платеж' }) })
      .getByRole('button')
      .first()
      .click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/schedule$`));
    await expect(page.getByText('Ближайший').first()).toBeVisible();
    await expect(page.getByText('Следующие').first()).toBeVisible();
    await expect(page.getByText(title1).first()).toBeVisible();

    // Любая строка графика открывает операцию (решение владельца):
    // материализованная — по id, проекция — вью без кнопки оплаты.
    await page.getByRole('button', { name: new RegExp(`${title1}`) }).first().click();
    await expect(page).toHaveURL(new RegExp(`/operations/[0-9a-f-]+$`));
    await expect(page.getByText('Запланирована', { exact: true })).toBeVisible();
    await page.goBack();
    // Хвост списка «Следующие» — проекции (материализованных две: сегодня и
    // следующий месяц), последняя строка заведомо кликает проекционный вью.
    await page.getByRole('button', { name: new RegExp(`${title1}`) }).last().click();
    await expect(page).toHaveURL(new RegExp(`/operations/projected/[0-9-]+$`));
    await expect(page.getByText('Запланирована', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Отметить оплаченной' })).toHaveCount(0);
    await page.goBack();
    await expect(page.getByText('Следующие').first()).toBeVisible();

    // ── «Оплатить» ведёт на страницу операции; «Отметить оплаченной» гасит
    // старейшее неоплаченное; тик тут же материализует следующее вхождение —
    // на странице платежа кнопка остаётся активной ──
    await page.goto(paymentUrl);
    await page.getByRole('button', { name: 'Оплатить' }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/operations/[0-9a-f-]+(\\?.*)?$`));
    await page.getByRole('button', { name: 'Отметить оплаченной' }).click();
    await expect(page.getByText('Платеж оплачен')).toBeVisible();
    await page.getByRole('button', { name: 'Хорошо', exact: true }).click();
    // Закрытие success возвращает на страницу, с которой платили (#1072).
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+$`));
    await page.goto(paymentUrl);
    await expect(page.getByRole('button', { name: 'Оплатить' })).toBeEnabled();

    // ── История: запись «Сегодня» с минусом у расхода ──
    await page.getByRole('button', { name: 'Открыть историю операций' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/history$`));
    await expect(page.getByText('Сегодня', { exact: true })).toBeVisible();
    await expect(page.getByText('-1 990 ₽').first()).toBeVisible();
    await expect(page.getByText(title1).first()).toBeVisible();

    // ── Пауза через confirm-шторку → возобновление без подтверждения ──
    await page.goto(paymentUrl);
    await page.getByRole('button', { name: 'На паузу' }).click();
    await expect(page.getByText('Поставить платеж на паузу?')).toBeVisible();
    await page.getByRole('button', { name: 'Пауза', exact: true }).click();
    await expect(page.getByText('Платеж поставлен на паузу')).toBeVisible();
    await expect(page.getByText('Расход • На паузе')).toBeVisible();
    await expect(page.getByText('На паузе', { exact: true }).first()).toBeVisible();

    await page.getByRole('button', { name: 'Возобновить' }).click();
    await expect(page.getByText('Платеж возобновлен')).toBeVisible();
    await expect(page.getByRole('button', { name: 'На паузу' })).toBeVisible();
    await expect(page.getByText('Расход • На паузе')).toHaveCount(0);

    // ── Просрочка (сид прошлого через SQL) → гашение из просроченных ──
    expect(await execE2eSql(induceOverdue(id1, 7))).toBe('UPDATE 1');
    await page.goto(paymentUrl);
    await expect(page.getByText('Просроченные платежи')).toBeVisible();
    await expect(page.getByText(/\d+ (день|дня|дней)/).first()).toBeVisible();

    // Полный список просроченных открывается стрелкой секции (#466).
    await page.getByRole('button', { name: 'Открыть полный список просроченных' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/overdue$`));
    await expect(page.getByText(title1).first()).toBeVisible();
    await expect(page.getByText(/\d+ (день|дня|дней)/).first()).toBeVisible();

    await page.goto(paymentUrl);
    await page.getByRole('button', { name: 'Оплатить' }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/operations/[0-9a-f-]+(\\?.*)?$`));
    await page.getByRole('button', { name: 'Отметить оплаченной' }).click();
    await expect(page.getByText('Платеж оплачен')).toBeVisible();
    await page.getByRole('button', { name: 'Хорошо', exact: true }).click();
    // Закрытие success возвращает на страницу, с которой платили (#1072).
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+$`));
    await page.goto(paymentUrl);
    await expect(page.getByText('У вас нет просроченных платежей')).toBeVisible();

    // ── Удаление операции: новая просрочка стирается корзиной, тик не
    // воскресает её (надгробие cancelled) ──
    expect(await execE2eSql(induceOverdue(id1, 2))).toBe('UPDATE 1');
    await page.goto(paymentUrl);
    await page.getByRole('button', { name: new RegExp(`${title1} .*(день|дня|дней)`) }).first().click();
    await expect(page).toHaveURL(new RegExp(`/operations/[0-9a-f-]+$`));
    await page.getByRole('button', { name: 'Удалить операцию' }).click();
    await expect(page.getByText('Удалить операцию?')).toBeVisible();
    await page.getByRole('button', { name: 'Удалить', exact: true }).click();
    await expect(page.getByText('Операция удалена')).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+$`));
    await expect(page.getByText('У вас нет просроченных платежей')).toBeVisible();

    // ── Правка: сумма меняется, прошлое не тронуто ──
    await page.getByRole('button', { name: 'Изменить' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/edit$`));
    await page.getByRole('textbox', { name: 'Сумма' }).fill('2500');
    await page.getByRole('button', { name: 'Сохранить изменения' }).click();
    await expect(page.getByText('Изменения сохранены')).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+$`));
    await expect(page.getByText('2 500 ₽').first()).toBeVisible();
    await expect(
      page.getByText(
        day === '31'
          ? 'Последний день каждого месяца'
          : `Каждый месяц ${day} числа`,
      ),
    ).toBeVisible();

    // ── Просрочка #2 → удаление БЕЗ «удалить просроченные»: правило
    // сносится, долг остаётся ──
    // Глубина 3 дня: «−7» у этого правила занята уже оплаченной просрочкой.
    expect(await execE2eSql(induceOverdue(id1, 3))).toBe('UPDATE 1');
    // Секция «Просроченные операции» списка несёт лимит 3 строки от старых
    // к новым — сидовые долги старше и вытесняют свежую просрочку правила;
    // строку правила ищем в полном списке просроченных.
    await page.goto(PAYMENTS_URL);
    await expect(page.getByRole('heading', { name: 'Просроченные операции' })).toBeVisible();
    await page.getByRole('button', { name: 'Открыть просроченные операции' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/overdue$`));
    await expect(page.getByText(title1).first()).toBeVisible();

    await page.goto(`${paymentUrl}/edit`);
    await page.getByRole('button', { name: 'Удалить платеж' }).click();
    await expect(page.getByText('Удалить платеж?')).toBeVisible();
    const checkbox = page.getByRole('checkbox', { name: 'Удалить просроченные операции' });
    await expect(checkbox).toBeVisible();
    await expect(checkbox).not.toBeChecked();
    await page.getByRole('button', { name: 'Удалить', exact: true }).click();
    await expect(page.getByText('Платеж удален')).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`${PAYMENTS_URL}$`));

    // Правило снесено; остаток долга живёт — полный список просроченных
    // показывает его без лимита секции (позиция долга в лимите секции зависит
    // от дат сидовых долгов — там не проверяем).
    await page.goto(`${PAYMENTS_URL}/overdue`);
    await expect(page.getByText(title1).first()).toBeVisible();

    // ── Удаление С «удалить просроченные»: сносится всё ──
    const id2 = await createMonthlyPaymentToday(page, seededUser, title2, day);
    expect(await execE2eSql(induceOverdue(id2, 7))).toBe('UPDATE 1');

    await page.goto(PAYMENTS_URL);
    await expect(page.getByText(title2).first()).toBeVisible();

    await page.goto(`/properties/${PROPERTY}/payments/${id2}/edit`);
    await page.getByRole('button', { name: 'Удалить платеж' }).click();
    await expect(page.getByText('Удалить платеж?')).toBeVisible();
    await page.getByText('Удалить просроченные операции').click();
    await expect(
      page.getByRole('checkbox', { name: 'Удалить просроченные операции' }),
    ).toBeChecked();
    await page.getByRole('button', { name: 'Удалить', exact: true }).click();
    await expect(page.getByText('Платеж удален')).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`${PAYMENTS_URL}$`));

    // Ни правила, ни просрочек.
    await expect(page.getByText(title2)).toHaveCount(0);
  });
});
