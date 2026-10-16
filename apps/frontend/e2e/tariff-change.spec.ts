import {
  captureScreen,
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// «Остановлен»-флоу экрана «Выбрать тариф» (#691): платная реактивация
// того же тарифа и того же периода при живом остатке закрыта — на своём
// тарифе и периоде экран показывает бесплатную «Возобновить {тариф}»
// вместо кнопки цены, после неё — экран успеха возобновления (#617),
// подписка снова активна. Состояние «Остановлен» сеет SQL середины теста
// поверх сида (подписка сид-владельца — service/active без срока); сид
// восстанавливается в afterAll: подписка общая, от неё зависят лимиты
// объектов в других спеках.

const SEEDED_SUBSCRIPTION_ID = '99999999-9999-4999-8999-999999999901';

const SEED_STOPPED = `
UPDATE user_subscriptions
SET source = 'paid',
    status = 'cancelled',
    auto_renew_enabled = false,
    valid_until = now() + interval '20 days',
    current_period = 'month'
WHERE id = '${SEEDED_SUBSCRIPTION_ID}'`;

const RESTORE_SEED = `
UPDATE user_subscriptions
SET source = 'service',
    status = 'active',
    auto_renew_enabled = false,
    valid_until = NULL,
    current_period = 'month'
WHERE id = '${SEEDED_SUBSCRIPTION_ID}'`;

test.describe('выбор тарифа в «Остановлен» — бесплатное возобновление #691', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test.afterAll(async () => {
    await execE2eSql(RESTORE_SEED);
  });

  test('своя подписка и период: «Возобновить Про» вместо цены, успех возобновления', async ({
    page,
    seededUser,
  }, testInfo) => {
    expect(await execE2eSql(SEED_STOPPED)).toBe('UPDATE 1');

    await openCabinetWithSeededSession(page, seededUser);

    // Хаб «Тариф» в «Остановлен»: герой остановленного тарифа.
    await page.goto('/profile/tariff');
    await expect(page.getByText('Про остановлен')).toBeVisible();

    // Экран выбора: дефолт — свой тариф и период (бейдж «Текущий»),
    // футер — бесплатная «Возобновить Про», кнопки цены нет.
    await page.goto('/profile/tariff/change');
    await expect(page.getByText('Текущий')).toBeVisible();
    const resume = page.getByRole('button', { name: 'Возобновить Про' });
    await expect(resume).toBeVisible();
    await expect(page.getByRole('button', { name: /Подключить за/ })).toHaveCount(0);
    await captureScreen(page, testInfo, 'tariff-change-stopped-resume');

    // Бесплатный путь: POST /subscription/resume → экран успеха возобновления.
    await resume.click();
    await expect(page.getByText('Тариф Про возобновлен')).toBeVisible();

    // Подписка снова активна: после «Хорошо» хаб показывает активный тариф.
    await page.getByRole('button', { name: 'Хорошо' }).click();
    await expect(page.getByText('Про остановлен')).toHaveCount(0);
    await expect(page.getByText(/Вы платите/)).toBeVisible();
  });
});

// Отложенный даунгрейд и повторное отключение на «Остановлен» и grace
// (#1452 поверх бекенда #1451, ADR 0066): более дешёвый тариф получает
// подтверждение с датой применения (конец оплаченного остатка / дедлайн
// grace-окна) и после планирования — существующий экран «Тариф изменен»
// с датой вступления; повторное «Отключить тариф» на отменённой —
// идемпотентный no-op без сырого 409. Тариф подписки сеется в Бизнес,
// послеAll возвращает сид (и тариф, и назначение даунгрейда): подписка
// общая, от неё зависят лимиты объектов в других спеках.

const SEED_BUSINESS_STOPPED = `
UPDATE user_subscriptions
SET source = 'paid',
    status = 'cancelled',
    tariff_id = (SELECT id FROM tariffs WHERE name = 'business'),
    auto_renew_enabled = false,
    valid_until = now() + interval '20 days',
    current_period = 'month',
    pending_tariff_id = NULL,
    pending_change_at = NULL,
    pending_period = NULL
WHERE id = '${SEEDED_SUBSCRIPTION_ID}'`;

const SEED_BUSINESS_GRACE = `
UPDATE user_subscriptions
SET source = 'paid',
    status = 'grace',
    tariff_id = (SELECT id FROM tariffs WHERE name = 'business'),
    auto_renew_enabled = true,
    valid_until = now() + interval '20 days',
    current_period = 'month',
    pending_tariff_id = NULL,
    pending_change_at = NULL,
    pending_period = NULL
WHERE id = '${SEEDED_SUBSCRIPTION_ID}'`;

test.describe('отложенный даунгрейд и повторное отключение в «Остановлен» и grace #1452', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  let previousTariff = 'pro';

  test.beforeAll(async () => {
    // Исходный тариф сида читаем один раз до первого переворота —
    // afterAll возвращает именно его.
    previousTariff = await execE2eSql(currentTariffName());
  });

  test.afterAll(async () => {
    await execE2eSql(
      `${RESTORE_SEED};
       UPDATE user_subscriptions
       SET tariff_id = (SELECT id FROM tariffs WHERE name = '${previousTariff}'),
           pending_tariff_id = NULL,
           pending_change_at = NULL,
           pending_period = NULL
       WHERE id = '${SEEDED_SUBSCRIPTION_ID}'`,
    );
  });

  const currentTariffName = (): string =>
    `SELECT t.name FROM user_subscriptions s JOIN tariffs t ON t.id = s.tariff_id
     WHERE s.id = '${SEEDED_SUBSCRIPTION_ID}'`;

  const pendingTariffName = (): string =>
    `SELECT t.name FROM user_subscriptions s JOIN tariffs t ON t.id = s.pending_tariff_id
     WHERE s.id = '${SEEDED_SUBSCRIPTION_ID}'`;

  test('«Остановлен» Бизнес × более дешёвый Про: подтверждение с датой и планирование', async ({
    page,
    seededUser,
  }, testInfo) => {
    expect(await execE2eSql(SEED_BUSINESS_STOPPED)).toBe('UPDATE 1');

    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/tariff/change');

    // Свой Бизнес выбран по умолчанию («Текущий»); выбираем более дешёвый
    // Про — футер открывает подтверждение, кнопки цены нет.
    await page.getByRole('radio', { name: 'Про' }).click();
    const change = page.getByRole('button', { name: 'Сменить тариф' });
    await expect(change).toBeVisible();
    await expect(page.getByRole('button', { name: /Подключить за/ })).toHaveCount(0);
    await change.click();

    // Подтверждение (канон ConfirmDialog): дата применения — конец
    // оплаченного периода, списание целевого тарифа на границе раскрыто.
    await expect(page.getByText('Сменить тариф на Про?')).toBeVisible();
    await expect(page.getByText(/в конце оплаченного периода/)).toBeVisible();
    await expect(page.getByText(/Тогда же спишем 490 ₽ в месяц за тариф Про/)).toBeVisible();
    await expect(page.getByText('Сейчас ничего не списываем.')).toBeVisible();
    await captureScreen(page, testInfo, 'tariff-change-stopped-downgrade-confirm');

    // Подтверждение — планирование без оплаты: существующий экран успеха
    // с датой вступления, подписка пока на Бизнесе.
    await page.getByRole('dialog').getByRole('button', { name: 'Сменить тариф' }).click();
    await expect(page.getByText('Тариф изменен')).toBeVisible();
    await expect(page.getByText(/Изменения вступят в силу/)).toBeVisible();
    expect(await execE2eSql(pendingTariffName())).toBe('pro');
  });

  test('grace Бизнес × более дешёвый Про: подтверждение с дедлайном окна', async ({
    page,
    seededUser,
  }, testInfo) => {
    expect(await execE2eSql(SEED_BUSINESS_GRACE)).toBe('UPDATE 1');

    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/tariff/change');

    await page.getByRole('radio', { name: 'Про' }).click();
    await page.getByRole('button', { name: 'Сменить тариф' }).click();

    // В grace дата применения — дедлайн окна, «конец оплаченного периода»
    // не называется.
    await expect(page.getByText('Сменить тариф на Про?')).toBeVisible();
    await expect(page.getByText(/Изменения вступят в силу/)).toBeVisible();
    await expect(page.getByText(/в конце оплаченного периода/)).toHaveCount(0);
    await expect(page.getByText('Сейчас ничего не списываем.')).toBeVisible();
    await captureScreen(page, testInfo, 'tariff-change-grace-downgrade-confirm');

    await page.getByRole('dialog').getByRole('button', { name: 'Сменить тариф' }).click();
    await expect(page.getByText('Тариф изменен')).toBeVisible();
    await expect(page.getByText(/Изменения вступят в силу/)).toBeVisible();
    expect(await execE2eSql(pendingTariffName())).toBe('pro');
  });

  test('повторное «Отключить тариф» на отменённой — идемпотентный no-op без 409', async ({
    page,
    seededUser,
  }) => {
    expect(await execE2eSql(SEED_BUSINESS_STOPPED)).toBe('UPDATE 1');

    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/tariff/change');

    // Выбор базового ведёт во флоу отключения (#622); повторное отключение
    // на отменённой отвечает успехом (ADR 0066), сырого 409 нет.
    await page.getByRole('radio', { name: 'Базовый' }).click();
    // exact: у FAQ на экране есть аккордеон «Как отключить тариф».
    await page.getByRole('button', { name: 'Отключить тариф', exact: true }).click();
    await expect(
      page.getByRole('heading', { name: 'Не теряйте доступ к своим объектам' }),
    ).toBeVisible();
    await page.getByRole('button', { name: 'Отключить тариф', exact: true }).click();

    const dialog = page.getByRole('dialog');
    await expect(
      dialog.getByText('Уверены, что хотите отключить тариф Бизнес?'),
    ).toBeVisible();
    await dialog.getByRole('button', { name: 'Отключить' }).click();

    await expect(page.getByText('Тариф Бизнес отключен')).toBeVisible();
  });
});
