import {
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
  }) => {
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

    // Бесплатный путь: POST /subscription/resume → экран успеха возобновления.
    await resume.click();
    await expect(page.getByText('Тариф Про возобновлен')).toBeVisible();

    // Подписка снова активна: после «Хорошо» хаб показывает активный тариф.
    await page.getByRole('button', { name: 'Хорошо' }).click();
    await expect(page.getByText('Про остановлен')).toHaveCount(0);
    await expect(page.getByText(/Вы платите/)).toBeVisible();
  });
});
