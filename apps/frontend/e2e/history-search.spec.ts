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

// Поиск по истории (карта #704, тикет #710; макеты 2089-165788,
// 2092-166284/166621/166866): иконка в шапке ленты меняет шапку на
// поисковую (прецедент «Ваших участников» #697), лента под пустым полем
// остаётся собой (макет 2089-165788); ввод ищет серверно по q (канон
// поиска #601: дебаунс 300, трим, пустой ввод запроса не порождает).
// Найденное группируется как лента — чипы дней сохраняются; без
// совпадений — «Ничего не найдено» (макет 2092-166866). Журнал сеется
// прямым INSERT'ом в action_journal; searchable контракта рекордера —
// текст сегментов + имя + почта актёра.

function ownerActorId(user: SeededUser): string {
  return `(SELECT id FROM users WHERE email = '${user.email}')`;
}

function memberActorId(): string {
  return `(SELECT id FROM users WHERE email = 'e2e-member@example.com')`;
}

async function seedEntry(entry: {
  readonly id: string;
  readonly createdAt: string;
  readonly propertyId?: string;
  readonly actorIdSql?: string;
  readonly actorName?: string;
  readonly actorRole?: string;
  readonly action?: string;
  readonly baseAction?: string;
  readonly kind?: string;
  readonly segments: string;
  readonly searchable: string;
}, user: SeededUser): Promise<string> {
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

/** Три записи: две сегодняшних (владелец) и одна вчерашняя; у записи
 * участника searchable несёт почту — лег trgm-поиска по почте. */
async function seedSearchJournal(user: SeededUser): Promise<void> {
  await execE2eSql('DELETE FROM action_journal;');
  await seedEntry(
    {
      id: 'b0000000-0000-4000-8000-000000000001',
      createdAt: new Date(Date.now() - 120_000).toISOString(),
      segments: '[{"text": "Платёж создан: "}, {"text": "Аренда за сентябрь", "link": {"kind": "payment", "id": "11111111-1111-4111-8111-111111111111"}}]',
      searchable: `Платёж создан: Аренда за сентябрь Иван Иванов ${user.email}`,
    },
    user,
  );
  await seedEntry(
    {
      id: 'b0000000-0000-4000-8000-000000000002',
      createdAt: new Date(Date.now() - 60_000).toISOString(),
      propertyId: SEEDED_GARAGE_PROPERTY_ID,
      segments: '[{"text": "Название объекта изменено: Гараж на Садовой"}]',
      searchable: 'Название объекта изменено: Гараж на Садовой Иван Иванов',
      action: 'property.renamed',
      baseAction: 'changed',
      kind: 'property',
    },
    user,
  );
  await seedEntry(
    {
      id: 'b0000000-0000-4000-8000-000000000003',
      createdAt: new Date(Date.now() - 24 * 3600_000).toISOString(),
      segments: '[{"text": "Операция оплачена: Вода"}]',
      searchable: 'Операция оплачена: Вода Иван Иванов',
      action: 'operation.paid',
      baseAction: 'completed',
      kind: 'operation',
    },
    user,
  );
  // Сегодня, запись участника: searchable несёт почту (контракт
  // рекордера — текст + имя + почта), находку по адресу даёт trgm-нога.
  await seedEntry(
    {
      id: 'b0000000-0000-4000-8000-000000000004',
      createdAt: new Date(Date.now() - 30_000).toISOString(),
      actorIdSql: memberActorId(),
      actorName: 'Мария Петрова',
      actorRole: 'full_access',
      segments: '[{"text": "Задача выполнена: Заменить кран"}]',
      searchable: 'Задача выполнена: Заменить кран Мария Петрова e2e-member@example.com',
      action: 'task.completed',
      baseAction: 'completed',
      kind: 'task',
    },
    user,
  );
}

test('вход с ленты: шапка меняется на поисковую, лента под пустым полем', async ({ page, seededUser }, testInfo) => {
  await seedSearchJournal(seededUser);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/history');

  // Точка входа — лупа в шапке ленты (макет 2157-56876).
  await page.getByRole('button', { name: 'Поиск по истории' }).click();

  // Шапка сменилась: поле «Поиск действий» с программным фокусом (канон
  // поисков), выход — «Назад»; лента под пустым полем осталась собой
  // (макет 2089-165788 — тот же контент без перезапроса).
  const field = page.getByPlaceholder('Поиск действий');
  await expect(field).toBeVisible();
  await expect(field).toBeFocused();
  await expect(page.getByRole('button', { name: 'Закрыть поиск' })).toBeVisible();
  await expect(page.getByText('Платёж создан:').first()).toBeVisible();
  await expect(page.getByText('Операция оплачена: Вода')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Настройки' })).toBeVisible();

  // Выход: «Назад» возвращает заголовок ленты и лупу.
  await page.getByRole('button', { name: 'Закрыть поиск' }).click();
  await expect(page.getByText('История действий')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Поиск по истории' })).toBeVisible();

  await captureScreen(page, testInfo, 'history-search-entry');
});

test('живой поиск: слова и почта сужают ленту, чипы дней сохраняются', async ({ page, seededUser }, testInfo) => {
  await seedSearchJournal(seededUser);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/history');
  await page.getByRole('button', { name: 'Поиск по истории' }).click();
  const field = page.getByPlaceholder('Поиск действий');

  // Слова (FTS-нога): только совпавшая запись, несовпавшие уходят,
  // чип дня на месте (макет 2092-166284).
  await field.fill('Платёж создан');
  await expect(page.getByText('Платёж создан:').first()).toBeVisible();
  await expect(page.getByText('Название объекта изменено: Гараж на Садовой')).toHaveCount(0);
  await expect(page.getByText('Операция оплачена: Вода')).toHaveCount(0);
  await expect(page.getByText('Сегодня', { exact: true })).toBeVisible();

  // Почта (trgm-нога): запись участника находится по адресу в searchable.
  await field.fill('e2e-member@');
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();
  await expect(page.getByText('Платёж создан:').first()).toHaveCount(0);

  // Запрос по вчерашней записи: секция «Сегодня» уходит вместе со своим
  // чипом — дни сохраняются только у найденного (макет 2092-166621 —
  // найденное разносится по дням, как в ленте).
  await field.fill('Операция оплачена');
  await expect(page.getByText('Операция оплачена: Вода')).toBeVisible();
  await expect(page.getByText('Вчера', { exact: true })).toBeVisible();
  await expect(page.getByText('Сегодня', { exact: true })).toHaveCount(0);

  await captureScreen(page, testInfo, 'history-search-live');
});

test('префиксы и словоформы «как в Telegram»: «петр» находит Петрову (тикет #842)', async ({ page, seededUser }, testInfo) => {
  await seedSearchJournal(seededUser);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/history');
  await page.getByRole('button', { name: 'Поиск по истории' }).click();
  const field = page.getByPlaceholder('Поиск действий');

  const paymentRow = page.getByText('Платёж создан:').first();
  const renameRow = page.getByText('Название объекта изменено: Гараж на Садовой');
  const operationRow = page.getByText('Операция оплачена: Вода');
  const taskRow = page.getByText('Задача выполнена: Заменить кран');

  // Инкремент недопечатанного слова: «и» (ILIKE-нога широкого префикса —
  // лента целиком, FTS-нога молчит на стоп-слове) → «ив» → «ива» — запись
  // участника уходит, три строки Иванова остаются на каждой ступени.
  await field.fill('и');
  await expect(paymentRow).toBeVisible();
  await expect(operationRow).toBeVisible();
  await expect(taskRow).toBeVisible();

  await field.fill('ив');
  await expect(taskRow).toHaveCount(0);
  await expect(paymentRow).toBeVisible();
  await expect(renameRow).toBeVisible();
  await expect(operationRow).toBeVisible();

  await field.fill('ива');
  await expect(taskRow).toHaveCount(0);
  await expect(paymentRow).toBeVisible();
  await expect(operationRow).toBeVisible();

  // Морфология: «Иванова» находит записи про «Иванов» ('иванов' после
  // нормализации живёт как префикс 'ива':*).
  await field.fill('Иванова');
  await expect(taskRow).toHaveCount(0);
  await expect(paymentRow).toBeVisible();
  await expect(operationRow).toBeVisible();

  // Префикс чужой фамилии: «петр» находит «Петрову», строки владельца уходят.
  await field.fill('петр');
  await expect(taskRow).toBeVisible();
  await expect(paymentRow).toHaveCount(0);
  await expect(operationRow).toHaveCount(0);

  // Фрагмент внутри слова — «трова» видит только ILIKE-нога.
  await field.fill('трова');
  await expect(taskRow).toBeVisible();
  await expect(paymentRow).toHaveCount(0);

  // Регресс полной почты: не только обрубок «e2e-member@».
  await field.fill('e2e-member@example.com');
  await expect(taskRow).toBeVisible();
  await expect(paymentRow).toHaveCount(0);

  await captureScreen(page, testInfo, 'history-search-telegram-like');
});

test('без совпадений — «Ничего не найдено», очистка возвращает ленту', async ({ page, seededUser }, testInfo) => {
  await seedSearchJournal(seededUser);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/history');
  await page.getByRole('button', { name: 'Поиск по истории' }).click();
  const field = page.getByPlaceholder('Поиск действий');

  await field.fill('ничего-подобного-в-журнале-нет');
  await expect(page.getByText('Ничего не найдено')).toBeVisible();
  await expect(page.getByText('Платёж создан:')).toHaveCount(0);

  // Крестик поля чистит ввод — лента возвращается (макет 2089-165788).
  await page.getByRole('button', { name: 'Очистить поиск' }).click();
  await expect(field).toHaveValue('');
  await expect(page.getByText('Платёж создан:').first()).toBeVisible();
  await expect(page.getByText('Операция оплачена: Вода')).toBeVisible();

  await captureScreen(page, testInfo, 'history-search-empty-result');
});

test('гвард пустого запроса: пробелы запрос с q= не порождают', async ({ page, seededUser }) => {
  await seedSearchJournal(seededUser);
  await openCabinetWithSeededSession(page, seededUser);

  // Счётчик запросов ленты с непустым q= (канон #601: пустой ввод —
  // чтения нет).
  let searchRequests = 0;
  await page.route(/\/history\?/, (route) => {
    if (new URL(route.request().url()).searchParams.get('q')) {
      searchRequests += 1;
    }
    return route.continue();
  });

  await page.goto('/history');
  await page.getByRole('button', { name: 'Поиск по истории' }).click();
  await page.getByPlaceholder('Поиск действий').fill('   ');

  // Дебаунс 300 мс истёк — запросов с q= нет, лента остаётся собой.
  await page.waitForTimeout(700);
  expect(searchRequests).toBe(0);
  await expect(page.getByText('Платёж создан:').first()).toBeVisible();

  // Санити счётчика: настоящий ввод уходит в q= (полл — дебаунс 300 мс)
  // и сужает ленту; до ответа держится прежняя выдача (keepPreviousData),
  // поэтому отсутствие чужих строк — только признак свершившегося поиска.
  await page.getByPlaceholder('Поиск действий').fill('Операция оплачена');
  await expect.poll(() => searchRequests, { timeout: 5_000 }).toBeGreaterThan(0);
  await expect(page.getByText('Операция оплачена: Вода')).toBeVisible();
  await expect(page.getByText('Платёж создан:')).toHaveCount(0);
});

test('пустой журнал: лупы в шапке нет (канон пустой книги)', async ({ page, seededUser }) => {
  await execE2eSql('DELETE FROM action_journal;');
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/history');

  await expect(page.getByText('Действий не было')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Поиск по истории' })).toHaveCount(0);
});
