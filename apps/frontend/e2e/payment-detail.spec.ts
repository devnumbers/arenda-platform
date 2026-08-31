import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';

// Страница платежа (#465): карточка правила (иконка/цвет категории,
// повторяемость, бейдж паузы), круглые кнопки «На паузу» (confirm-шторка)
// ↔ «Возобновить», «Изменить», «Оплатить» — гасит старейшее неоплаченное
// вхождение, звезда избранного в шапке, секции «Ближайший платеж» и
// «Просроченные», плитки подэкранов. Скриншоты — материал для сверки
// с Figma (671:5889 активный, 850:15410 на паузе).
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
    // просрочка красным.
    await expect(page.getByText('Ближайшая операция')).toBeVisible();
    await expect(
      page.getByText(
        /\d{1,2} (января|февраля|марта|апреля|мая|июня|июля|августа|сентября|октября|ноября|декабря)/,
      ).first(),
    ).toBeVisible();
    await expect(page.getByText('Просроченные операции')).toBeVisible();
    await expect(page.getByText(/\d+ (день|дня|дней)/).first()).toBeVisible();

    // Плитки подэкранов.
    await expect(page.getByText('График платежей')).toBeVisible();
    await expect(page.getByText('История операций')).toBeVisible();

    await captureScreen(page, testInfo, 'payment-detail-filled-mobile');
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

    await page.goBack();
    await page.getByRole('button', { name: 'Оплатить' }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${PROPERTY}/operations/[0-9a-f-]+$`));
    await page.getByRole('button', { name: 'Отметить оплаченной' }).click();

    // Экран успеха (1444:65733); «Посмотреть платеж» ведёт на страницу
    // правила — долг закрыт, тик материализовал следующее вхождение и
    // «Оплатить» снова активна (оплатить можно досрочно, история 24).
    await expect(page.getByText('Платеж оплачен')).toBeVisible();
    await page.getByRole('button', { name: 'Посмотреть платеж' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+$`));
    await expect(page.getByText('У вас нет просроченных операций')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Оплатить' })).toBeEnabled();

    // Назад по истории — операция в состоянии «Выполнена», кнопки нет.
    await page.goBack();
    await expect(page.getByText('Выполнена')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Отметить оплаченной' })).toHaveCount(0);

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
    await expect(page.getByText('У вас нет просроченных операций')).toBeVisible();

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
