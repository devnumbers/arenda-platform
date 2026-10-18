import type { Locator, Page } from '@playwright/test';
import {
  captureScreen,
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  SEEDED_STUDIO_PROPERTY_ID,
  test,
} from './fixtures';

// Экран «Отключение тарифа» (#622) — выбор сохраняемого объекта (#1518):
// клик по не-первому ряду обязан доехать до POST /subscription/cancel
// (keepPropertyId = выбранный, и на проводе, и в БД), а приглашённые
// объекты (чужие, доступ по membership) в выбор не попадают вовсе —
// бэк валидирует keepPropertyId по владению (409 invalid_keep_property
// на чужой), и приглашённый объект не сохраняемый: при истечении
// архиваются только свои, чужие ноги гасит recipient-slots без выбора.
// «Истечение → выжил выбранный» на этом проводе не дублируется: его
// покрывает интеграционный тест бэка на реальном Postgres
// (TestWorkers_Integration_CancelledExpiryCarriesKeepChoice) — ожидание
// тика воркера в e2е было бы флейтейм.
//
// Порядок списка бэка — updated_at DESC; сид задаёт свой порядок разгоном
// updated_at (Студия → Гараж → Квартира), послеAll возвращает «как в сиде»
// (одинаковый now()). Подписка сида — service/active: юзер не может
// отменить неоплачиваемую (гвардия paid-source), сценарии отмены живут на
// перевёрнутой в paid+active подписке, afterAll возвращает сид. Подписка
// общая, от неё зависят лимиты объектов в других спеках.

const SEEDED_SUBSCRIPTION_ID = '99999999-9999-4999-8999-999999999901';
const SEEDED_OWNER_ID = '11111111-1111-4111-8111-111111111111';
// Чужой объект: Дача Марии (владелец — сид-участница квартиры) + активное
// членство Ивана — GET /properties вернёт её четвёртой с ролью full_access.
const INVITED_PROPERTY_ID = '35353535-3535-4535-8535-353535353535';
const SEEDED_MEMBER_ID = '12111111-1111-4111-8111-111111111121';

const SEED_ROW_ORDER = `
UPDATE properties SET updated_at = now() - interval '1 hour' WHERE id = '${SEEDED_STUDIO_PROPERTY_ID}';
UPDATE properties SET updated_at = now() - interval '2 hours' WHERE id = '${SEEDED_GARAGE_PROPERTY_ID}';
UPDATE properties SET updated_at = now() - interval '3 hours' WHERE id = '${SEEDED_APARTMENT_PROPERTY_ID}';`;

const SEED_INVITED_PROPERTY = `
INSERT INTO properties (id, owner_id, name, type, address, status)
VALUES ('${INVITED_PROPERTY_ID}', '${SEEDED_MEMBER_ID}', 'Дача в Переделкино', 'house', 'Москва, Переделкино, 1', 'active')
ON CONFLICT (id) DO NOTHING;
INSERT INTO property_members (id, property_id, user_id, role, granted_by)
VALUES ('99999999-9999-4999-8999-999999999936', '${INVITED_PROPERTY_ID}', '${SEEDED_OWNER_ID}', 'full_access', '${SEEDED_MEMBER_ID}')
ON CONFLICT (property_id, user_id) WHERE status = 'active' DO NOTHING;`;

const CLEANUP_INVITED_PROPERTY = `
DELETE FROM property_members WHERE property_id = '${INVITED_PROPERTY_ID}';
DELETE FROM properties WHERE id = '${INVITED_PROPERTY_ID}';`;

// Сид-эквивалент трёх своих: активные, одинаковый now() (в сиде они
// вставлены одним стейтментом); лишние объекты чужих спек возвращаются
// отдельно по захваченным id — их статус вне ведома этой спеки.
const RESTORE_ROWS = `
UPDATE properties SET status = 'active', updated_at = now()
WHERE owner_id = '${SEEDED_OWNER_ID}'
  AND id IN ('${SEEDED_APARTMENT_PROPERTY_ID}', '${SEEDED_GARAGE_PROPERTY_ID}', '${SEEDED_STUDIO_PROPERTY_ID}');`;

const SEED_PAID_ACTIVE = `
UPDATE user_subscriptions
SET source = 'paid',
    status = 'active',
    auto_renew_enabled = true,
    valid_until = now() + interval '20 days',
    current_period = 'month',
    pending_tariff_id = NULL,
    pending_change_at = NULL,
    pending_period = NULL,
    keep_property_id = NULL
WHERE id = '${SEEDED_SUBSCRIPTION_ID}'`;

const RESTORE_SEED_SUBSCRIPTION = `
UPDATE user_subscriptions
SET source = 'service',
    status = 'active',
    auto_renew_enabled = false,
    valid_until = NULL,
    current_period = 'month',
    pending_tariff_id = NULL,
    pending_change_at = NULL,
    pending_period = NULL,
    keep_property_id = NULL
WHERE id = '${SEEDED_SUBSCRIPTION_ID}'`;

const ACTIVE_OWN_EXTRAS = `SELECT string_agg(id::text, ',')
FROM properties
WHERE owner_id = '${SEEDED_OWNER_ID}'
  AND status = 'active'
  AND id NOT IN ('${SEEDED_APARTMENT_PROPERTY_ID}', '${SEEDED_GARAGE_PROPERTY_ID}', '${SEEDED_STUDIO_PROPERTY_ID}')`;

const cancelledTransitionsCount = (): Promise<string> =>
  execE2eSql(
    `SELECT count(*) FROM subscription_transitions WHERE subscription_id = '${SEEDED_SUBSCRIPTION_ID}' AND reason = 'cancelled'`,
  );

/** Счётчик фаеров POST /subscription/cancel — page.on('request'), не route:
 * грабля #1113 — широкий route-паттерн валит вкладку. */
function trackCancels(page: Page): string[] {
  const bodies: string[] = [];
  page.on('request', (request) => {
    if (
      request.method() === 'POST'
      && request.url().endsWith('/api/subscription/cancel')
    ) {
      bodies.push(request.postData() ?? '');
    }
  });
  return bodies;
}

/** Два клика одной кнопкой в одном JS-таске — окно до перерисовки из
 * проб #1113; гард мутации обязан дать ровно один фаер. */
async function doubleClickInOneTask(button: Locator): Promise<void> {
  await button.evaluate((element) => {
    const el = element as HTMLElement;
    el.click();
    el.click();
  });
}

type CancelBody = { readonly keepPropertyId?: string };

test.describe('отключение тарифа: выбор сохраняемого объекта #1518', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  let archivedExtras = '';

  test.beforeAll(async () => {
    // Полный прогон мог оставить Ивану лишние активные объекты (визарды
    // других спек) — счётчик и порядок списка должны быть ровно сидовыми:
    // лишнее архивируем, в afterAll возвращаем статус.
    archivedExtras = await execE2eSql(ACTIVE_OWN_EXTRAS);
    if (archivedExtras !== '') {
      const ids = archivedExtras.split(',').map((id) => `'${id}'`).join(', ');
      await execE2eSql(`UPDATE properties SET status = 'archived' WHERE id IN (${ids})`);
    }
    await execE2eSql(SEED_ROW_ORDER);
    await execE2eSql(SEED_INVITED_PROPERTY);
  });

  test.afterAll(async () => {
    await execE2eSql(RESTORE_SEED_SUBSCRIPTION);
    await execE2eSql(`${CLEANUP_INVITED_PROPERTY};${RESTORE_ROWS}`);
    if (archivedExtras !== '') {
      const ids = archivedExtras.split(',').map((id) => `'${id}'`).join(', ');
      await execE2eSql(`UPDATE properties SET status = 'active' WHERE id IN (${ids})`);
    }
  });

  test('приглашённый объект не попадает в радиосписок: свои считаются без чужих', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/tariff/disable');

    // Карточка считает только СВОИ активные (3), а не всё GET /properties
    // (4 с приглашённой дачей).
    await expect(
      page.getByRole('heading', { name: 'У вас 3 объекта' }),
    ).toBeVisible();

    // Радиосписок — три своих ряда; дачи в списке нет ни текстом, ни радио.
    await expect(page.getByRole('radio')).toHaveCount(3);
    await expect(page.getByText('Дача в Переделкино')).toHaveCount(0);
    await expect(page.getByRole('radio', { name: 'Студия на Полевой' })).toBeVisible();
    await expect(page.getByRole('radio', { name: 'Гараж на Садовой' })).toBeVisible();
    await expect(page.getByRole('radio', { name: 'Квартира на Ленина' })).toBeVisible();
    await captureScreen(page, testInfo, 'tariff-disable-own-only-picker');
  });

  test('клик по не-первому ряду доезжает до запроса и БД; двойной клик — ровно один фаер', async ({
    page,
    seededUser,
  }, testInfo) => {
    expect(await execE2eSql(SEED_PAID_ACTIVE)).toBe('UPDATE 1');
    const cancelBodies = trackCancels(page);
    const transitionsBefore = await cancelledTransitionsCount();

    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/tariff/disable');

    // Порядок сида: Студия (дефолт, новее всех) → Гараж → Квартира.
    await expect(
      page.getByRole('radio', { name: 'Студия на Полевой' }),
    ).toBeChecked();

    // Сценарий тикета: клик по ВТОРОМУ ряду (по кнопке-строке, не по
    // самому радио), пауза (проникновение из репро владельца), диалог,
    // подтверждение.
    await page.getByRole('button', { name: 'Гараж на Садовой Москва, ул. Садовая, 2' }).click();
    await expect(
      page.getByRole('radio', { name: 'Гараж на Садовой' }),
    ).toBeChecked();
    await expect(
      page.getByRole('radio', { name: 'Студия на Полевой' }),
    ).not.toBeChecked();
    await page.waitForTimeout(250);
    await captureScreen(page, testInfo, 'tariff-disable-second-row-selected');

    await page.getByRole('button', { name: 'Отключить тариф', exact: true }).click();
    const dialog = page.getByRole('dialog');
    await expect(
      dialog.getByText('Уверены, что хотите отключить тариф Про?'),
    ).toBeVisible();

    // Канон мутаций #1113: два клика одним таском — гард даёт один фаер.
    await doubleClickInOneTask(
      dialog.getByRole('button', { name: 'Отключить', exact: true }),
    );

    await expect(page.getByText('Тариф Про отключен')).toBeVisible();

    // На проводе и в БД — именно выбранный Гараж, не дефолтная Студия;
    // фаер один, переход `cancelled` один.
    expect(cancelBodies).toHaveLength(1);
    expect(
      (JSON.parse(cancelBodies[0] ?? '{}') as CancelBody).keepPropertyId,
    ).toBe(SEEDED_GARAGE_PROPERTY_ID);
    expect(await cancelledTransitionsCount()).toBe(
      String(Number(transitionsBefore) + 1),
    );
    expect(
      await execE2eSql(
        `SELECT keep_property_id::text FROM user_subscriptions WHERE id = '${SEEDED_SUBSCRIPTION_ID}'`,
      ),
    ).toBe(SEEDED_GARAGE_PROPERTY_ID);
  });

  test('репро прогона тикета: 2 объекта, дефолт Гараж, выбор Квартиры — доезжает Квартира', async ({
    page,
    seededUser,
  }) => {
    // Точный расклад первого прогона #1518: два своих (Студия в архиве),
    // дефолт — Гараж, выбрана Квартира.
    expect(await execE2eSql(SEED_PAID_ACTIVE)).toBe('UPDATE 1');
    expect(
      await execE2eSql(
        `UPDATE properties SET status = 'archived' WHERE id = '${SEEDED_STUDIO_PROPERTY_ID}'`,
      ),
    ).toBe('UPDATE 1');
    const cancelBodies = trackCancels(page);

    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/tariff/disable');

    await expect(
      page.getByRole('heading', { name: 'У вас 2 объекта' }),
    ).toBeVisible();
    await expect(
      page.getByRole('radio', { name: 'Гараж на Садовой' }),
    ).toBeChecked();

    await page.getByRole('button', { name: 'Квартира на Ленина Москва, ул. Ленина, 1' }).click();
    await expect(
      page.getByRole('radio', { name: 'Квартира на Ленина' }),
    ).toBeChecked();
    await page.waitForTimeout(250);

    await page.getByRole('button', { name: 'Отключить тариф', exact: true }).click();
    const dialog = page.getByRole('dialog');
    await expect(
      dialog.getByText('Уверены, что хотите отключить тариф Про?'),
    ).toBeVisible();
    await dialog.getByRole('button', { name: 'Отключить', exact: true }).click();

    await expect(page.getByText('Тариф Про отключен')).toBeVisible();

    expect(cancelBodies).toHaveLength(1);
    expect(
      (JSON.parse(cancelBodies[0] ?? '{}') as CancelBody).keepPropertyId,
    ).toBe(SEEDED_APARTMENT_PROPERTY_ID);
    expect(
      await execE2eSql(
        `SELECT keep_property_id::text FROM user_subscriptions WHERE id = '${SEEDED_SUBSCRIPTION_ID}'`,
      ),
    ).toBe(SEEDED_APARTMENT_PROPERTY_ID);
  });
});
