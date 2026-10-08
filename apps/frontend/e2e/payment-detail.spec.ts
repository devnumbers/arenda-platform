import {
  captureScreen,
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';
import { formatDayMonth } from '@/shared/lib/date-format';
import { dateToIsoLocal } from '@/shared/lib/calendar';

// Страница платежа (#465): карточка правила (иконка/цвет категории,
// повторяемость, бейдж паузы), круглые кнопки «На паузу» (confirm-шторка)
// ↔ «Возобновить», «Изменить», «Оплатить» — гасит старейшее неоплаченное
// вхождение, звезда избранного в шапке, секции «Ближайшая операция»,
// «Просроченные платежи» и «История платежа» (макет 3214:76417, #1194).
// Скриншоты — материал для сверки с Figma (3214:76417 активный,
// 850:15410 на паузе).
//
// Сид (#465): …551 аренда с одной просрочкой (-5 дней), …553 автоплатёж,
// …554 «Домофон» на активной бессрочной паузе, …555 завершённое правило
// (окно расписания нулевой ширины — операций нет детерминированно).

const PROPERTY = SEEDED_APARTMENT_PROPERTY_ID;
const PAYMENT_URLS = {
  rent: `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555551`,
  insurance: `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555552`,
  electricity: `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555553`,
  intercomPaused: `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555554`,
  completed: `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555555`,
  internet: `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555556`,
};
const PROPERTY_PAYMENTS_URL = `/properties/${PROPERTY}/payments`;

test.describe('страница платежа', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  // Тесты файла выполняются последовательно и мутируют сидовые данные
  // (оплата аренды гасит её единственную просрочку) — порядок важен:
  // сначала скриншот наполненного экрана, затем мутации.

  test('карточка, кнопки и секции наполненного экрана; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(PAYMENT_URLS.rent);

    // Шапка: заголовок правила не-автоплатежа и его звезда (в сиде — в избранном).
    await expect(page.getByText('Платеж', { exact: true })).toBeVisible();
    const star = page.getByRole('button', { name: 'Убрать из избранного' });
    await expect(star).toBeVisible();
    await expect(star).toHaveAttribute('aria-pressed', 'true');

    // Карточка: название, сумма, направление, регулярность (671:6171).
    await expect(page.getByText('Арендная плата').first()).toBeVisible();
    await expect(page.getByText('45 000 ₽').first()).toBeVisible();
    await expect(page.getByText('Расход', { exact: true })).toBeVisible();
    await expect(page.getByText('Каждый месяц 1 числа')).toBeVisible();

    // Круглые кнопки мутаций (Full Access+, у владельца видны все три).
    await expect(page.getByRole('button', { name: 'На паузу' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Изменить' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Оплатить' })).toBeEnabled();

    // Секции: ближайший плановый день месяца («1 сентября»), одна
    // просрочка красным. Заголовок ближайшего — кнопка со стрелкой на
    // график (макет 3214:76417, #1194), клик по строке — тоже график
    // (решение #1073).
    await expect(page.getByText('Ближайшая операция')).toBeVisible();
    await expect(
      page.getByRole('button', { name: 'Открыть график платежей' }),
    ).toBeVisible();
    await expect(
      page.getByText(
        /\d{1,2} (января|февраля|марта|апреля|мая|июня|июля|августа|сентября|октября|ноября|декабря)/,
      ).first(),
    ).toBeVisible();
    await expect(page.getByText('Просроченные платежи')).toBeVisible();
    await expect(page.getByText(/\d+ (день|дня|дней)/).first()).toBeVisible();

    // Плиток подэкранов нет: вход на график — заголовок и строка
    // ближайшего, вход в историю — стрелка секции. У свежесидовой аренды
    // paid-операций нет — секция истории скрыта, «Графика платежей» на
    // странице не существует вовсе.
    await expect(page.getByText('График платежей')).toHaveCount(0);
    await expect(page.getByText('История платежа')).toHaveCount(0);

    await captureScreen(page, testInfo, 'payment-detail-filled-mobile');
  });

  test('секция истории: превью 3 новейших, строка — операция, стрелка — подэкран', async ({
    page,
    seededUser,
  }) => {
    // «Интернет» …556: 55 paid-операций в сиде — секция с превью
    // (решение владельца #1073, макет 1096:37793).
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(PAYMENT_URLS.internet);

    const historySection = page.locator('section').filter({
      has: page.getByRole('heading', { name: 'История платежа' }),
    });
    await expect(historySection).toBeVisible();

    // Превью — новейшие по фактической дате (сид: paid «сегодня» и
    // «вчера»), в строке — дата факта (групп-заголовков «Сегодня» здесь
    // нет, это канон подэкрана) и знак расхода. TZ прогона UTC — тот же
    // «сегодня», что у CURRENT_DATE сидового postgres.
    const todayLabel = formatDayMonth(dateToIsoLocal(new Date()));
    const yesterdayLabel = formatDayMonth(
      dateToIsoLocal(new Date(Date.now() - 24 * 60 * 60 * 1000)),
    );
    await expect(historySection.getByText(todayLabel, { exact: true })).toBeVisible();
    await expect(historySection.getByText(yesterdayLabel, { exact: true })).toBeVisible();
    await expect(historySection.getByText('-1 000 ₽').first()).toBeVisible();

    // Строка превью — страница операции (кнопки секции: стрелка заголовка
    // и строки — кликаем по строке операции по имени).
    await historySection.getByRole('button', { name: /Интернет/ }).first().click();
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/operations/[0-9a-f-]+$`));

    // Стрелка секции — подэкран истории.
    await page.goBack();
    await page.getByRole('button', { name: 'Открыть историю платежа' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/history$`));
  });

  test('клик по строке ближайшего ведёт на график — проекция и материализованная', async ({
    page,
    seededUser,
  }) => {
    // Секция ближайшего (#1073, макет 3214:76417): строка кликабельна при
    // обоих видах ближайшего и всегда открывает график — не страницу
    // операции; заголовок-стрелка (#1194) ведёт туда же. Строки секции —
    // div role="button", заголовок — нативный button: фильтр по role-селектору
    // отделяет строки от кнопки заголовка.
    const nearestSection = page
      .locator('section')
      .filter({ has: page.getByRole('heading', { name: 'Ближайшая операция' }) });
    const nearestRow = nearestSection.locator('[role="button"]');
    const nearestHeader = nearestSection.getByRole('button', {
      name: 'Открыть график платежей',
    });

    await openCabinetWithSeededSession(page, seededUser);

    // Проекция: у «Страхования» материализованные плановые в прошлом
    // (просрочка), будущих нет — ближайшее рисует клиентский порт.
    await page.goto(PAYMENT_URLS.insurance);
    await expect(nearestRow).toBeVisible();
    await nearestRow.click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/schedule$`));

    // Заголовок секции со стрелкой — тот же график (#1194).
    await page.goBack();
    await nearestHeader.click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/schedule$`));

    // Материализованная плановая: SQL-вставка будущего вхождения —
    // ближайшее становится операцией, клик по-прежнему ведёт на график.
    expect(
      await execE2eSql(`
        INSERT INTO operations (id, owner_id, property_id, payment_id, origin, date,
                                paid_date, status, type, title, amount_kopecks,
                                category_label, category_slug)
        VALUES ('77777777-7777-4777-8777-777777777799',
                '11111111-1111-4111-8111-111111111111',
                '33333333-3333-4333-8333-333333333333',
                '55555555-5555-4555-8555-555555555552',
                'payment', CURRENT_DATE + 1, NULL, 'planned', 'expense',
                'Страхование', 320000, 'Страхование', 'insurance')
        ON CONFLICT (id) DO NOTHING
      `),
      // Ровно одна вставленная строка: «INSERT 0 0» — молчаливый пропуск
      // конфликта id (остаток прошлого прогона), дальше проверять нечего.
    ).toBe('INSERT 0 1');
    try {
      // Убеждаемся по API, что ближайшее — вставленная операция, и сверяем
      // её дату с подзаголовком строки (секция переехала с проекции).
      // expect.poll резолвится в void — значение выносим замыканием.
      const plannedUrl =
        `/api/properties/${PROPERTY}/payments/` +
        `55555555-5555-4555-8555-555555555552/operations?status=planned&order=asc`;
      let materializedDate = '';
      await expect.poll(async () => {
        const response = await page.request.get(plannedUrl);
        const { items } = (await response.json()) as {
          items: ReadonlyArray<{ readonly date: string }>;
        };
        materializedDate = items[0]?.date ?? '';
        return materializedDate;
      }).not.toBe('');

      await page.goto(PAYMENT_URLS.insurance);
      await expect(
        nearestRow.getByText(formatDayMonth(materializedDate), { exact: true }),
      ).toBeVisible();
      await nearestRow.click();
      await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/schedule$`));
    } finally {
      await execE2eSql(
        `DELETE FROM operations WHERE id = '77777777-7777-4777-8777-777777777799'`,
      );
    }

    // Паузная строка «На паузе» — тоже ближайший платеж: клик ведёт на
    // график (решение владельца #1073).
    await page.goto(PAYMENT_URLS.intercomPaused);
    await expect(nearestRow).toBeVisible();
    await nearestRow.click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/schedule$`));
  });

  test('отмена в шторке паузы ничего не меняет', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(PAYMENT_URLS.electricity);

    // Автоплатёж называет страницу иначе (резолюция #452).
    await expect(page.getByText('Автоплатёж', { exact: true })).toBeVisible();

    await page.getByRole('button', { name: 'На паузу' }).click();
    await expect(page.getByText('Поставить платеж на паузу?')).toBeVisible();
    await expect(
      page.getByText('Данные платежа сохранятся, вы сможете возобновить его в любое время'),
    ).toBeVisible();

    await page.getByRole('button', { name: 'Отменить' }).click();
    await expect(page.getByText('Поставить платеж на паузу?')).not.toBeVisible();
    // Мутации не было: кнопка паузы на месте, тоста нет.
    await expect(page.getByRole('button', { name: 'На паузу' })).toBeVisible();
    await expect(page.getByText('Платеж поставлен на паузу')).toHaveCount(0);
  });

  test('возобновление без подтверждения и пауза через шторку; скриншоты', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(PAYMENT_URLS.intercomPaused);

    // Правило уже на паузе: карточка приглушена, тип с суффиксом,
    // ближайший платеж заменён подписью.
    await expect(page.getByText('Расход • На паузе')).toBeVisible();
    await expect(page.getByText('На паузе', { exact: true }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Возобновить' })).toBeVisible();
    await captureScreen(page, testInfo, 'payment-detail-paused-mobile');

    // Возобновление подтверждения не требует (история 20).
    await page.getByRole('button', { name: 'Возобновить' }).click();
    await expect(page.getByText('Платеж возобновлен')).toBeVisible();
    await expect(page.getByRole('button', { name: 'На паузу' })).toBeVisible();
    await expect(page.getByText('Расход • На паузе')).toHaveCount(0);

    // Обратно на паузу — только через confirm-шторку (история 21).
    await page.getByRole('button', { name: 'На паузу' }).click();
    await expect(page.getByText('Поставить платеж на паузу?')).toBeVisible();
    await page.waitForTimeout(700);
    await captureScreen(page, testInfo, 'payment-detail-pause-confirm-mobile');
    await page.getByRole('button', { name: 'Пауза', exact: true }).click();
    await expect(page.getByText('Платеж поставлен на паузу')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Возобновить' })).toBeVisible();
    await expect(page.getByText('Расход • На паузе')).toBeVisible();
    await expect(page.getByText('На паузе', { exact: true }).first()).toBeVisible();
  });

  test('оплата уходит на страницу операции; отметка гасит просрочку, дальше — досрочно', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(PAYMENT_URLS.rent);

    await expect(page.getByText(/\d+ (день|дня|дней)/).first()).toBeVisible();

    // Строка просрочки ведёт на страницу операции: статус «Просрочена» и
    // кнопка «Отметить оплаченной» (макеты 1419:25859).
    await page.getByRole('button', { name: /\d+ (день|дня|дней)/ }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/operations/[0-9a-f-]+$`));
    await expect(page.getByText('Просрочена', { exact: true })).toBeVisible();

    // Строки «Данных операции» — ссылки (1386:67731): объект ведёт на
    // страницу объекта, правило — на страницу платежа.
    await page.getByRole('button', { name: 'Квартира на Ленина Объект' }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}$`));
    await page.goBack();
    await page.getByRole('button', { name: 'Арендная плата Платеж' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+$`));
    await page.goBack();
    await expect(page.getByText('Просрочена', { exact: true })).toBeVisible();

    await page.goBack();
    await page.getByRole('button', { name: 'Оплатить' }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/operations/[0-9a-f-]+(\\?.*)?$`));
    await page.getByRole('button', { name: 'Отметить оплаченной' }).click();

    // Экран успеха (1444:65733); «Посмотреть платеж» ведёт на страницу
    // правила — долг закрыт, тик материализовал следующее вхождение и
    // «Оплатить» снова активна (оплатить можно досрочно, история 24).
    await expect(page.getByText('Платеж оплачен')).toBeVisible();
    await page.getByRole('button', { name: 'Посмотреть платеж' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+$`));
    await expect(page.getByText('У вас нет просроченных платежей')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Оплатить' })).toBeEnabled();

    // Назад по истории — операция в состоянии «Выполнена», кнопки нет.
    await page.goBack();
    await expect(page.getByText('Выполнена')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Отметить оплаченной' })).toHaveCount(0);

    // Удаление оплаченной операции (1510:77505): факт стирается, история
    // пустеет, правило живёт дальше.
    await page.goto(PAYMENT_URLS.rent);
    await page.getByRole('button', { name: 'Открыть историю платежа' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/history$`));
    await page.getByRole('button', { name: /Арендная плата/ }).first().click();
    await expect(page).toHaveURL(new RegExp(`/operations/[0-9a-f-]+$`));
    await page.getByRole('button', { name: 'Удалить операцию' }).click();
    await expect(page.getByText('Удалить операцию?')).toBeVisible();
    await page.getByRole('button', { name: 'Удалить', exact: true }).click();
    await expect(page.getByText('Операция удалена')).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/history$`));
    await expect(page.getByText('Платежей еще не было')).toBeVisible();

    await page.goto(PAYMENT_URLS.rent);
    await captureScreen(page, testInfo, 'payment-detail-paid-out-mobile');
  });

  test('звезда избранного переключает состояние с тостом', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(PAYMENT_URLS.insurance);

    const star = page.getByRole('button', { name: 'Добавить в избранное' });
    await expect(star).toHaveAttribute('aria-pressed', 'false');
    await star.click();

    await expect(page.getByText('Платеж добавлен в избранное')).toBeVisible();
    const starOn = page.getByRole('button', { name: 'Убрать из избранного' });
    await expect(starOn).toHaveAttribute('aria-pressed', 'true');

    // Состояние переживает перезагрузку — прочитано с сервера.
    await page.reload();
    await expect(
      page.getByRole('button', { name: 'Убрать из избранного' }),
    ).toHaveAttribute('aria-pressed', 'true');

    await page.getByRole('button', { name: 'Убрать из избранного' }).click();
    await expect(page.getByText('Платеж больше не в избранном')).toBeVisible();
  });

  test('регулярность в hero — множественная форма дней месяца', async ({
    page,
    seededUser,
  }) => {
    // Строка повторяемости hero (3214:76429, #1194): месячное правило с
    // несколькими днями рендерит форму «Каждый месяц 1, 10 и 15 числа».
    // Сидовое «Интернет» …556 — одиночное «15 числа»: переводим правило
    // SQL-ом на три дня и возвращаем точный сидовый литерал в finally.
    await openCabinetWithSeededSession(page, seededUser);
    expect(
      await execE2eSql(`
        UPDATE payments
        SET recurrence = '{"kind": "monthly", "daysOfMonth": [1, 10, 15]}'::jsonb
        WHERE id = '55555555-5555-4555-8555-555555555556'
      `),
    ).toBe('UPDATE 1');
    try {
      await page.goto(PAYMENT_URLS.internet);
      await expect(page.getByText('Каждый месяц 1, 10 и 15 числа')).toBeVisible();
    } finally {
      await execE2eSql(`
        UPDATE payments
        SET recurrence = '{"kind": "monthly", "dayOfMonth": 15}'::jsonb
        WHERE id = '55555555-5555-4555-8555-555555555556'
      `);
    }
  });

  test('завершённое правило: без паузы, «Оплатить» отключена; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(PAYMENT_URLS.completed);

    await expect(page.getByText('Техосмотр').first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'На паузу' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Возобновить' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Оплатить' })).toBeDisabled();
    await expect(page.getByText('Платеж завершен')).toBeVisible();
    await expect(page.getByText('У вас нет просроченных платежей')).toBeVisible();

    await captureScreen(page, testInfo, 'payment-detail-completed-mobile');
  });

  test('вход со списка платежей по строке правила', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(PROPERTY_PAYMENTS_URL);

    // Секции показывают максимум 3 ближайших платежа: кликаем по строке,
    // которая всегда в топе, — ежедневная «Парковка». Строки операций
    // (просрочек) кликов не имеют — правило ведёт на страницу платежа.
    await page.getByText('Парковка', { exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/payments/[0-9a-f-]+$`));
    await expect(page.getByText('Парковка').first()).toBeVisible();
  });
});

test.describe('страница платежа — десктоп', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('тот же контент в колонке 560; скриншот', async ({ page, seededUser }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    // Мобильные тесты выше мутируют сид (аренда оплачена), поэтому состояние
    // кнопки здесь не проверяем — только состав и колонка 560.
    await page.goto(PAYMENT_URLS.rent);

    await expect(page.getByText('45 000 ₽').first()).toBeVisible();
    await expect(page.getByRole('button', { name: 'Оплатить' })).toBeVisible();
    await captureScreen(page, testInfo, 'payment-detail-desktop');
  });
});
