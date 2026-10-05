import {
  captureScreen,
  expect,
  execE2eSql,
  memberActorIdSql,
  memberTaskEntry,
  openCabinetWithSeededSession,
  openCabinetWithSessionToken,
  ownerActorIdSql,
  paymentCreatedEntry,
  propertyRenamedEntry,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  seedJournalEntry,
  screenHeader,
  seededMemberSessionToken,
  test,
  todayAt,
} from './fixtures';

// Экран «История действий» (карта #704, тикет #709): мессенджерская лента
// по всем доступным объектам — новые снизу, прокрутка вверх догружает
// старое (двусторонний keyset #708, порция 50). Группировка «дата →
// объект → актёр → строки»; вход — кебаб «Ваших участников» (#843,
// макет 2008-47514; решение владельца 22.09 о строке на хабе заменено —
// #843). Журнал сеется прямым INSERT'ом в
// action_journal (запись журнала идёт в транзакциях мутаций — сиду
// проще класть строки тем же контрактом, что миграция 000140).

/** Чип дня в JS — зеркально formatDayMonthWithYear (год вне текущего). */
function expectedDayChip(daysAgo: number): string {
  const day = new Date();
  day.setUTCHours(12, 0, 0, 0);
  day.setUTCDate(day.getUTCDate() - daysAgo);
  const base = day.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', timeZone: 'UTC' });
  const today = new Date();
  return day.getUTCFullYear() === today.getUTCFullYear() ? base : `${base}, ${day.getUTCFullYear()}`;
}

test('вход с кебаба «Ваших участников», пустая лента — «Действий не было» и кнопка «Настройки»', async ({ page, seededUser }, testInfo) => {
  await execE2eSql('DELETE FROM action_journal;');
  await openCabinetWithSeededSession(page, seededUser);

  // Точка входа — кебаб «Ваших участников» (#843, макет 2008-47514).
  await page.goto('/participants/list');
  await screenHeader(page).getByRole('button', { name: 'Еще — действия со списком' }).click();
  await page.getByRole('menuitem', { name: 'История действий' }).click();
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
  await seedJournalEntry(
    paymentCreatedEntry('a0000000-0000-4000-8000-000000000001', 3, 'Аренда за сентябрь', {
      segments: '[{"text": "Платёж создан: "}, {"text": "Аренда за сентябрь", "link": {"kind": "payment", "id": "11111111-1111-4111-8111-111111111111"}}]',
    }),
    seededUser,
  );
  await seedJournalEntry(
    propertyRenamedEntry('a0000000-0000-4000-8000-000000000002', 2, 'Гараж на Садовой', {
      segments: '[{"text": "Название объекта изменено: "}, {"text": "Гараж на Садовой", "link": {"kind": "property", "id": "' + SEEDED_GARAGE_PROPERTY_ID + '"}}]',
    }),
    seededUser,
  );
  await seedJournalEntry(
    {
      id: 'a0000000-0000-4000-8000-000000000003',
      createdAtSql: todayAt(1),
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
  await seedJournalEntry(memberTaskEntry('a0000000-0000-4000-8000-000000000004', 0), seededUser);
  // Вчера и 12 дней назад.
  await seedJournalEntry(
    {
      id: 'a0000000-0000-4000-8000-000000000005',
      createdAtSql: todayAt(0, 1),
      segments: '[{"text": "Операция оплачена: Вода"}]',
      searchable: 'Операция оплачена: Вода Иван Иванов',
      action: 'operation.paid',
      baseAction: 'completed',
      kind: 'operation',
    },
    seededUser,
  );
  await seedJournalEntry(
    {
      id: 'a0000000-0000-4000-8000-000000000006',
      createdAtSql: todayAt(0, 12),
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

  // Строки: сегменты дословно, связанные фрагменты — синие ссылки на
  // страницы сущностей (#713), иконка основного действия с подписью группы.
  await expect(page.getByText('Платёж создан:').first()).toBeVisible();
  await expect(page.getByText('Аренда за сентябрь')).toBeVisible();
  await expect(page.locator('svg[aria-label="Добавление"]').first()).toBeVisible();
  await expect(page.locator('svg[aria-label="Изменение"]').first()).toBeVisible();
  await expect(page.locator('svg[aria-label="Выполнение"]').first()).toBeVisible();
  await expect(page.locator('svg[aria-label="Удаление"]').first()).toBeVisible();
  // Время строки «ЧЧ:ММ».
  await expect(page.getByText(/^\d{2}:\d{2}$/).first()).toBeVisible();

  // Плашка дня липнет к верхнему краю (макет 2157-56876 — sticky, как в
  // мессенджерах): прокручиваем в середину секции «Сегодня» — плашка
  // прижимается под закреплённой шапкой 72, на tablet-вьюпорте это 96
  // (tablet:top-96). Вьюпорт ниже стандарта, чтобы сид из шести записей
  // позволял докрутить секцию до кромки (иначе контент ниже короче
  // вьюпорта и страница кончается раньше пиннинга).
  await page.setViewportSize({ width: 1280, height: 560 });
  const todaySection = page.getByText('Сегодня', { exact: true }).locator('xpath=ancestor::section[1]');
  const todayTop = await todaySection.evaluate(
    (el) => el.getBoundingClientRect().top + window.scrollY,
  );
  await page.evaluate((top) => {
    window.scrollTo(0, top + 300);
  }, todayTop);
  const todayPill = page.getByText('Сегодня', { exact: true });
  await expect(todayPill).toBeVisible();
  const pillBox = await todayPill.boundingBox();
  expect(pillBox?.y).toBeGreaterThanOrEqual(60);
  expect(pillBox?.y).toBeLessThanOrEqual(96);

  await captureScreen(page, testInfo, 'history-feed-groups');
});

test('синие переходы #713: связанные сегменты ведут на страницы сущностей, удалённая — без перехода', async ({ page, seededUser }) => {
  await execE2eSql('DELETE FROM action_journal;');
  await openCabinetWithSeededSession(page, seededUser);

  // Живой платёж сидовой квартиры (сид #463) — адресат платежной ссылки.
  const paymentId = await execE2eSql(
    `SELECT id FROM payments WHERE property_id = '${SEEDED_APARTMENT_PROPERTY_ID}' LIMIT 1`,
  );
  const memberUserId = await execE2eSql(`SELECT id FROM users WHERE email = 'e2e-member@example.com'`);

  // Платёжная и участникская ссылки на живые сущности + объектная.
  await seedJournalEntry(
    {
      id: 'a0000000-0000-4000-8000-000000000011',
      createdAtSql: todayAt(3),
      segments: `[{"text": "Платёж изменён: "}, {"text": "Аренда за сентябрь", "link": {"kind": "payment", "id": "${paymentId}"}}]`,
      searchable: 'Платёж изменён: Аренда за сентябрь Иван Иванов',
      action: 'payment.updated',
      baseAction: 'changed',
    },
    seededUser,
  );
  await seedJournalEntry(
    {
      id: 'a0000000-0000-4000-8000-000000000012',
      createdAtSql: todayAt(2),
      propertyId: SEEDED_GARAGE_PROPERTY_ID,
      segments: `[{"text": "Гараж на Садовой закреплён", "link": {"kind": "property", "id": "${SEEDED_GARAGE_PROPERTY_ID}"}}]`,
      searchable: 'Гараж на Садовой закреплён Иван Иванов',
      action: 'property.pinned',
      baseAction: 'changed',
      kind: 'property',
    },
    seededUser,
  );
  await seedJournalEntry(
    {
      id: 'a0000000-0000-4000-8000-000000000013',
      createdAtSql: todayAt(1),
      segments: `[{"text": "Добавлен участник: "}, {"text": "Мария Петрова", "link": {"kind": "member", "id": "${memberUserId}"}}]`,
      searchable: 'Добавлен участник: Мария Петрова Иван Иванов',
      action: 'member.added',
      baseAction: 'added',
      kind: 'member',
      actorIdSql: memberActorIdSql(),
      actorName: 'Мария Петрова',
      actorRole: 'full_access',
    },
    seededUser,
  );
  // Удалённая сущность: сервер пишет строку без ссылки — снапшот названия
  // остаётся, перехода нет (ADR 0061 §3).
  await seedJournalEntry(
    {
      id: 'a0000000-0000-4000-8000-000000000014',
      createdAtSql: todayAt(0),
      segments: '[{"text": "Платёж удалён: Старый платёж"}]',
      searchable: 'Платёж удалён: Старый платёж Иван Иванов',
      action: 'payment.deleted',
      baseAction: 'deleted',
    },
    seededUser,
  );

  await page.goto('/history');

  // Сегмент-ссылка платежа ведёт на страницу платежа по его живому id.
  const paymentLink = page.getByRole('link', { name: 'Аренда за сентябрь' });
  await expect(paymentLink).toHaveAttribute(
    'href',
    `/properties/${SEEDED_APARTMENT_PROPERTY_ID}/payments/${paymentId}`,
  );
  await paymentLink.click();
  await page.waitForURL(`**/properties/${SEEDED_APARTMENT_PROPERTY_ID}/payments/${paymentId}`);

  // Объектная ссылка строки — на деталь своего объекта.
  await page.goto('/history');
  const propertyLink = page.getByRole('link', { name: 'Гараж на Садовой' }).last();
  await expect(propertyLink).toHaveAttribute('href', `/properties/${SEEDED_GARAGE_PROPERTY_ID}`);

  // Ссылка участника — на страницу участника хаба (uuid юзера из ссылки).
  // Локатор по href: шапка актёра той же группы тоже «Мария Петрова», но
  // ведёт в «Действия участника» (#712).
  const memberLink = page.locator(`a[href="/participants/${memberUserId}"]`);
  await expect(memberLink).toHaveCount(1);
  await memberLink.click();
  await page.waitForURL(`**/participants/${memberUserId}`);

  // Удалённая сущность — текст строки без ссылки.
  await page.goto('/history');
  await expect(page.getByText('Старый платёж')).toBeVisible();
  await expect(page.getByRole('link', { name: 'Старый платёж' })).toHaveCount(0);
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
      ${ownerActorIdSql(seededUser)},
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

test('короткая лента прижата к низу вьюпорта — страница не прокручивается (мессенджер)', async ({ page, seededUser }) => {
  await execE2eSql('DELETE FROM action_journal;');
  await openCabinetWithSeededSession(page, seededUser);

  // Две записи за сегодня — лента заведомо короче вьюпорта: как одно
  // сообщение в мессенджере, она висит у нижнего края, а не у верхнего
  // (решение владельца 24.09). Пустые состояния — не «сообщения», их
  // прижатие не касается.
  await seedJournalEntry(paymentCreatedEntry('a0000000-0000-4000-8000-000000000021', 5, 'Первая запись'), seededUser);
  await seedJournalEntry(paymentCreatedEntry('a0000000-0000-4000-8000-000000000022', 1, 'Вторая запись'), seededUser);

  await page.goto('/history');
  await expect(page.getByText('Сегодня', { exact: true })).toBeVisible();

  // Страница ровно вьюпорт: узел контента ScreenLayout — flex-1 от
  // min-h-screen оболочки, лента с flex-1 тянется до нижнего края на
  // всех ярусах (высота шапки и safe-area учитываются сами). Иначе —
  // не якорь.
  const metrics = await page.evaluate(() => ({
    scrollHeight: document.documentElement.scrollHeight,
    innerHeight: window.innerHeight,
  }));
  expect(Math.abs(metrics.scrollHeight - metrics.innerHeight)).toBeLessThanOrEqual(1);

  // Последняя строка прижата к низу: под ней только резерв плавающей
  // кнопки «Настройки» — pb-136 PageContent (кнопка сама fixed и высоты
  // страницы не занимает).
  const lastRow = page.getByText('Платёж создан: Вторая запись');
  await expect(lastRow).toBeVisible();
  const box = await lastRow.boundingBox();
  const boxBottom = (box?.y ?? 0) + (box?.height ?? 0);
  const gapToBottom = metrics.innerHeight - boxBottom;
  expect(gapToBottom).toBeGreaterThan(96);
  expect(gapToBottom).toBeLessThanOrEqual(176);
});

test('свой актор подписан «(Вы)»: в ленте и на прибитой странице про себя', async ({ page, seededUser }) => {
  await execE2eSql('DELETE FROM action_journal;');
  await openCabinetWithSeededSession(page, seededUser);

  // Свой актор (владелец сеанса) и чужой — метка только у своего
  // (actor_id = id сессии, семантика «как в Telegram», решение
  // владельца 24.09); канон подписи — серый суффикс шита фильтров (#710).
  await seedJournalEntry(paymentCreatedEntry('a0000000-0000-4000-8000-000000000031', 5, 'Аренда за сентябрь'), seededUser);
  await seedJournalEntry(memberTaskEntry('a0000000-0000-4000-8000-000000000032', 1), seededUser);

  await page.goto('/history');

  // Своя шапка — имя с суффиксом; чужая — имя без него.
  await expect(page.getByRole('heading', { name: 'Иван Иванов (Вы)', exact: true }).first()).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Мария Петрова', exact: true }).first()).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Мария Петрова (Вы)' })).toHaveCount(0);

  // Метка не меняет кликабельность (#712): своя шапка в общей ленте —
  // по-прежнему ссылка в «Действия участника».
  await expect(page.getByRole('link', { name: 'Иван Иванов (Вы)' }).first()).toHaveAttribute(
    'href',
    /\/history\/participants\//,
  );

  // На прибитой странице про себя (#712) та же метка — шапка там статична
  // (человек — предмет страницы), ссылок из шапок нет.
  const meId = await execE2eSql(`SELECT id FROM users WHERE email = '${seededUser.email}'`);
  await page.goto(`/history/participants/${meId}`);
  await expect(page.getByRole('heading', { name: 'Иван Иванов (Вы)', exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: 'Иван Иванов (Вы)' })).toHaveCount(0);

  // Глазами приглашённого — семантика Telegram: своя группа подписана
  // «(Вы)», владелец — без метки (решение владельца 24.09).
  await openCabinetWithSessionToken(page, seededMemberSessionToken());
  await page.goto('/history');
  await expect(page.getByRole('heading', { name: 'Мария Петрова (Вы)', exact: true }).first()).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Иван Иванов', exact: true }).first()).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Иван Иванов (Вы)' })).toHaveCount(0);
});

test('безымянный актёр: «Пользователь» в шапке группы (карта #1105, аменд #1123); старый снимок с маской не перезаписывается (Q6=А)', async ({ page, seededUser }) => {
  await execE2eSql('DELETE FROM action_journal;');

  // Зарегистрированный юзер без имени: канон карты #1105 («Имя Фамилия»,
  // иначе «Пользователь» — аменд #1123, телефон больше не фолбэк)
  // собирает бекенд одним швом display_name (#1106).
  // Телефон сидится плейнтекстом при phone_encrypted = false —
  // decryptPhone читает как есть, шифрование нужно только логину.
  const NAMELESS_ID = '16111111-1111-4111-8111-111111111171';
  const NAMELESS_PHONE = '+79137654327';
  const NAMELESS_EMAIL = 'e2e-nameless-history@example.com';
  await execE2eSql(
    `INSERT INTO users (id, phone, role, name, surname, email, phone_encrypted, timezone) ` +
      `VALUES ('${NAMELESS_ID}', '${NAMELESS_PHONE}', 'owner', NULL, NULL, '${NAMELESS_EMAIL}', FALSE, 'UTC') ` +
      `ON CONFLICT (id) DO NOTHING`,
  );
  try {
    // Две строки журнала безымянного вокруг строки владельца — две серии
    // (вернувшийся актёр открывает новую серию, шапка несёт имя первой
    // строки серии): СТАРЫЙ снимок с маской maskPhone (как писалось до
    // #1106) и НОВЫЙ — «Пользователь» (как пишет рекордер после #1123).
    // Старые снимки журнал не перезаписывает (решение Q6=А, 2026-10-05):
    // лента отображает снимок строки как есть.
    await seedJournalEntry(
      memberTaskEntry('a0000000-0000-4000-8000-000000000041', 9, {
        actorIdSql: `(SELECT id FROM users WHERE email = '${NAMELESS_EMAIL}')`,
        actorName: '+7********27',
      }),
      seededUser,
    );
    await seedJournalEntry(paymentCreatedEntry('a0000000-0000-4000-8000-000000000042', 6, 'Аренда за сентябрь'), seededUser);
    await seedJournalEntry(
      memberTaskEntry('a0000000-0000-4000-8000-000000000043', 1, {
        actorIdSql: `(SELECT id FROM users WHERE email = '${NAMELESS_EMAIL}')`,
        actorName: 'Пользователь',
      }),
      seededUser,
    );

    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/history');

    // Старый снимок — маска как есть (журнал историчен), новый —
    // «Пользователь»; шапка безымянного без суффикса «(Вы)» (актёр чужой).
    await expect(page.getByRole('heading', { name: '+7********27', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Пользователь', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Пользователь (Вы)' })).toHaveCount(0);
    // Шапка с actor_id кликабельна в «Действия участника» (#712) —
    // безымянный титул ничего не ломает.
    await expect(page.getByRole('link', { name: 'Пользователь' }).first()).toHaveAttribute(
      'href',
      new RegExp(`/history/participants/${NAMELESS_ID}`),
    );
    // Маска на ленте ровно одна — в шапке старой серии: лента не
    // пере-маскирует новые снимки и не переписывает старые (Q6=А).
    await expect(page.getByText(/\*{3}/)).toHaveCount(1);
  } finally {
    await execE2eSql(
      `DELETE FROM action_journal WHERE actor_id = '${NAMELESS_ID}'; ` +
        `DELETE FROM users WHERE id = '${NAMELESS_ID}'`,
    );
  }
});
