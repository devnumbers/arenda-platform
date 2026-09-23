import {
  captureScreen,
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  test,
  type SeededUser,
} from './fixtures';

// Экран «История действий» (карта #704, тикет #709): мессенджерская лента
// по всем доступным объектам — новые снизу, прокрутка вверх догружает
// старое (двусторонний keyset #708, порция 50). Группировка «дата →
// объект → актёр → строки»; вход — строка на хабе «Совместный доступ»
// (решение владельца 22.09). Журнал сеется прямым INSERT'ом в
// action_journal (запись журнала идёт в транзакциях мутаций — сиду
// проще класть строки тем же контрактом, что миграция 000136).

/** Чип дня в JS — зеркально formatDayMonthWithYear (год вне текущего). */
function expectedDayChip(daysAgo: number): string {
  const day = new Date();
  day.setUTCHours(12, 0, 0, 0);
  day.setUTCDate(day.getUTCDate() - daysAgo);
  const base = day.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', timeZone: 'UTC' });
  const today = new Date();
  return day.getUTCFullYear() === today.getUTCFullYear() ? base : `${base}, ${day.getUTCFullYear()}`;
}

function ownerActorId(user: SeededUser): string {
  return `(SELECT id FROM users WHERE email = '${user.email}')`;
}

function memberActorId(): string {
  return `(SELECT id FROM users WHERE email = 'e2e-member@example.com')`;
}

interface SeededEntry {
  readonly id: string;
  readonly createdAt: string;
  readonly propertyId?: string;
  readonly actorIdSql?: string;
  readonly actorName?: string;
  readonly actorRole?: string;
  readonly baseAction?: string;
  readonly action?: string;
  readonly kind?: string;
  readonly segments: string;
  readonly searchable: string;
}

async function seedEntry(entry: SeededEntry, user: SeededUser): Promise<string> {
  return execE2eSql(`
    INSERT INTO action_journal
      (id, property_id, actor_id, actor_role, actor_name, actor_email, kind, action, base_action, segments, searchable, created_at)
    VALUES (
      '${entry.id}',
      '${entry.propertyId ?? SEEDED_APARTMENT_PROPERTY_ID}',
      ${entry.actorIdSql ?? ownerActorId(user)},
      '${entry.actorRole ?? 'owner'}',
      '${entry.actorName ?? 'Иван Иванов'}',
      '${user.email}',
      '${entry.kind ?? 'payment'}',
      '${entry.action ?? 'payment.created'}',
      '${entry.baseAction ?? 'added'}',
      $j$${entry.segments}$j$::jsonb,
      '${entry.searchable.replace(/'/g, "''")}',
      '${entry.createdAt}'
    );
  `);
}

test('вход с хаба «Совместный доступ», пустая лента — «Действий не было» и кнопка «Настройки»', async ({ page, seededUser }, testInfo) => {
  await execE2eSql('DELETE FROM action_journal;');
  await openCabinetWithSeededSession(page, seededUser);

  // Точка входа — строка на хабе (решение владельца 22.09).
  await page.goto('/participants');
  await page.getByRole('button', { name: /История действий/ }).click();
  await page.waitForURL('**/history');

  await expect(page.getByText('Действий не было')).toBeVisible();
  // Кнопка «Настройки» стоит и на пустой ленте (макет 2050-158499);
  // контент шита — тикет #711.
  await expect(page.getByRole('button', { name: 'Настройки' })).toBeVisible();

  await captureScreen(page, testInfo, 'history-feed-empty');
});

test('лента из сида: чипы дней, группы «объект → актёр», иконки и время строк', async ({ page, seededUser }, testInfo) => {
  await execE2eSql('DELETE FROM action_journal;');
  await openCabinetWithSeededSession(page, seededUser);

  // Сегодня: два объекта и два актёра сериями (канон мессенджера).
  await seedEntry(
    {
      id: 'a0000000-0000-4000-8000-000000000001',
      createdAt: new Date(Date.now() - 3 * 60_000).toISOString(),
      segments: '[{"text": "Платёж создан: "}, {"text": "Аренда за сентябрь", "link": {"kind": "payment", "id": "11111111-1111-4111-8111-111111111111"}}]',
      searchable: 'Платёж создан: Аренда за сентябрь Иван Иванов',
    },
    seededUser,
  );
  await seedEntry(
    {
      id: 'a0000000-0000-4000-8000-000000000002',
      createdAt: new Date(Date.now() - 2 * 60_000).toISOString(),
      segments: '[{"text": "Название объекта изменено: "}, {"text": "Гараж на Садовой", "link": {"kind": "property", "id": "' + SEEDED_GARAGE_PROPERTY_ID + '"}}]',
      searchable: 'Название объекта изменено: Гараж на Садовой Иван Иванов',
    },
    seededUser,
  );
  await seedEntry(
    {
      id: 'a0000000-0000-4000-8000-000000000003',
      createdAt: new Date(Date.now() - 60_000).toISOString(),
      propertyId: SEEDED_GARAGE_PROPERTY_ID,
      segments: '[{"text": "Гараж на Садовой закреплён"}]',
      searchable: 'Гараж на Садовой закреплён Иван Иванов',
      action: 'property.pinned',
      baseAction: 'changed',
      kind: 'property',
    },
    seededUser,
  );
  // Сегодня, другим актёром — серия внутри того же объекта.
  await seedEntry(
    {
      id: 'a0000000-0000-4000-8000-000000000004',
      createdAt: new Date().toISOString(),
      actorIdSql: memberActorId(),
      actorName: 'Мария Петрова',
      actorRole: 'full_access',
      segments: '[{"text": "Задача выполнена: Заменить кран"}]',
      searchable: 'Задача выполнена: Заменить кран Мария Петрова',
      action: 'task.completed',
      baseAction: 'completed',
      kind: 'task',
    },
    seededUser,
  );
  // Вчера и 12 дней назад.
  await seedEntry(
    {
      id: 'a0000000-0000-4000-8000-000000000005',
      createdAt: new Date(Date.now() - 24 * 3600_000).toISOString(),
      segments: '[{"text": "Операция оплачена: Вода"}]',
      searchable: 'Операция оплачена: Вода Иван Иванов',
      action: 'operation.paid',
      baseAction: 'completed',
      kind: 'operation',
    },
    seededUser,
  );
  await seedEntry(
    {
      id: 'a0000000-0000-4000-8000-000000000006',
      createdAt: new Date(Date.now() - 12 * 24 * 3600_000).toISOString(),
      segments: '[{"text": "Платёж удалён: Старый платёж"}]',
      searchable: 'Платёж удалён: Старый платёж Иван Иванов',
      action: 'payment.deleted',
      baseAction: 'deleted',
    },
    seededUser,
  );

  await page.goto('/history');

  // Чипы дней: сегодня, вчера, дальняя дата (канон подписи).
  await expect(page.getByText('Сегодня', { exact: true })).toBeVisible();
  await expect(page.getByText('Вчера', { exact: true })).toBeVisible();
  await expect(page.getByText(expectedDayChip(12), { exact: true })).toBeVisible();

  // Шапки объектов — живые названия области, кликабельны (страница
  // объекта); у сидовой квартиры виден и адрес (вторая строка шапки по
  // макету 2157-56876, из /history/filters).
  const apartmentHeader = page.getByRole('link', { name: /Квартира на Ленина/ }).first();
  await expect(apartmentHeader).toBeVisible();
  await expect(apartmentHeader).toHaveAttribute('href', new RegExp(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}`));
  await expect(page.getByText('Москва, ул. Ленина, 1').first()).toBeVisible();
  await expect(page.getByRole('link', { name: /Гараж на Садовой/ }).first()).toBeVisible();

  // Шапки актёров — имена в серых карточках; роль экран не показывает
  // (макет 2157-56876). Шапка с actor_id — вход в «Действия участника»
  // (#712): кликабельны и участник, и владелец (страница действий не
  // читает участников, 404 не бывает), обезличенные записи не ссылки.
  await expect(page.getByRole('heading', { name: 'Иван Иванов' }).first()).toBeVisible();
  await expect(page.getByRole('link', { name: 'Мария Петрова' }).first()).toHaveAttribute(
    'href',
    /\/history\/participants\//,
  );
  await expect(page.getByRole('link', { name: 'Иван Иванов' }).first()).toHaveAttribute(
    'href',
    /\/history\/participants\//,
  );

  // Строки: сегменты дословно, связанные фрагменты — синие ссылки (#713
  // проложит переходы), иконка основного действия с подписью группы.
  await expect(page.getByText('Платёж создан:').first()).toBeVisible();
  await expect(page.getByText('Аренда за сентябрь')).toBeVisible();
  await expect(page.locator('svg[aria-label="Добавление"]').first()).toBeVisible();
  await expect(page.locator('svg[aria-label="Изменение"]').first()).toBeVisible();
  await expect(page.locator('svg[aria-label="Выполнение"]').first()).toBeVisible();
  await expect(page.locator('svg[aria-label="Удаление"]').first()).toBeVisible();
  // Время строки «ЧЧ:ММ».
  await expect(page.getByText(/^\d{2}:\d{2}$/).first()).toBeVisible();

  // Плашка дня липнет к верхнему краю (макет 2157-56876 — sticky, как в
  // мессенджерах): на дне ленты секция «Вчера» уже прошла верхнюю кромку —
  // её плашка прижата под закреплённой шапкой 72 (вьюпорт e2e — Desktop
  // Chrome), а не висит в потоке на своей позиции.
  await page.evaluate(() => {
    window.scrollTo(0, document.documentElement.scrollHeight - window.innerHeight);
  });
  const yesterdayPill = page.getByText('Вчера', { exact: true });
  await expect(yesterdayPill).toBeVisible();
  const pillBox = await yesterdayPill.boundingBox();
  expect(pillBox?.y).toBeGreaterThanOrEqual(60);
  expect(pillBox?.y).toBeLessThanOrEqual(96);

  await captureScreen(page, testInfo, 'history-feed-groups');
});

test('прокрутка вверх догружает старое: prepend 55 записей поверх порции 50', async ({ page, seededUser }, testInfo) => {
  await execE2eSql('DELETE FROM action_journal;');
  await openCabinetWithSeededSession(page, seededUser);

  // 55 записей одного дня: порция 50 несёт самые новые (06..55), пять
  // самых старых (01..05) приезжают prepend'ом при прокрутке вверх.
  await execE2eSql(`
    INSERT INTO action_journal
      (id, property_id, actor_id, actor_role, actor_name, actor_email, kind, action, base_action, segments, searchable, created_at)
    SELECT
      ('a0000000-0000-4000-8000-' || lpad(g::text, 12, '0'))::uuid,
      '${SEEDED_APARTMENT_PROPERTY_ID}',
      ${ownerActorId(seededUser)},
      'owner',
      'Иван Иванов',
      '${seededUser.email}',
      'payment',
      'payment.created',
      'added',
      jsonb_build_array(jsonb_build_object('text', 'Платёж создан: платёж-' || lpad(g::text, 2, '0'))),
      'Платёж создан: платёж-' || lpad(g::text, 2, '0') || ' Иван Иванов',
      date_trunc('day', now()) + interval '9 hours' - (55 - g) * interval '1 minute'
    FROM generate_series(1, 55) AS g;
  `);

  await page.goto('/history');
  const rows = page.getByText(/^Платёж создан: платёж-\d+$/);
  const oldest = page.getByText('Платёж создан: платёж-01');
  const newest = page.getByText('Платёж создан: платёж-55');

  // Первая порция: самая старая запись ещё не загружена, самая новая
  // видна внизу (мессенджер).
  await expect(rows).toHaveCount(50);
  await expect(oldest).toHaveCount(0);
  await expect(newest).toBeInViewport();

  // Прокрутка вверх: sentinel просит предыдущую порцию — prepend 5 старых.
  await page.evaluate(() => window.scrollTo(0, 0));
  await expect(oldest).toBeVisible();
  await expect(rows).toHaveCount(55);
  // Хронология prepend'а: старейшая строка встала НАД старейшей первой
  // порции (платёж-06) и обе в кадре — страница старых рисуется над
  // загруженными, а не в низу ленты (баг живой приёмки #709: реверс
  // постранично вместо плоского клал prepend-страницу в низ).
  const order = await page.evaluate(() => {
    const y = (needle: string) => {
      const row = [...document.querySelectorAll('p')].find((p) => p.textContent.includes(needle));
      return row ? row.getBoundingClientRect().top : null;
    };
    return { first: y('платёж-01'), sixth: y('платёж-06') };
  });
  expect(order.first).not.toBeNull();
  expect(order.sixth).not.toBeNull();
  expect(order.first as number).toBeLessThan(order.sixth as number);
  await expect(oldest).toBeInViewport();
  // Удержание позиции: верхняя до prepend'а строка (платёж-06 — старейшая
  // первой порции) возвращается в кадр.
  await expect(page.getByText('Платёж создан: платёж-06')).toBeInViewport();

  await captureScreen(page, testInfo, 'history-feed-prepend');
});
