import {
  captureScreen,
  expect,
  execE2eSql,
  memberActorIdSql,
  openCabinetWithSeededSession,
  SEEDED_GARAGE_PROPERTY_ID,
  seedJournalEntry,
  test,
  trackHistoryScope,
  type SeededUser,
} from './fixtures';

// «Действия участника» (карта #704, тикет #712; макет 2184-94731): та же
// лента «Истории действий», прибитая к одному человеку — actor_ids = один
// uuid (ADR 0061 §7), группировка «день → объект → актёр» и поиск/фильтры
// переиспользуются; группы «Участники» в шите нет — человек прибит
// страницей. Вход — тап по актёру в общей ленте и кебаб страницы
// участника.

const MARIA_ID = '12111111-1111-4111-8111-111111111121';
const GARAGE_ID = SEEDED_GARAGE_PROPERTY_ID;

/** Четыре записи: две владельца (квартира) и две участницы Марии —
 * квартира и гараж, оба сегодня: на странице участницы объекты остаются
 * секциями внутри дня (макет 2184-94731). */
async function seedMemberJournal(user: SeededUser): Promise<void> {
  await execE2eSql('DELETE FROM action_journal;');
  await seedJournalEntry(
    {
      id: 'c0000000-0000-4000-8000-000000000001',
      createdAt: new Date(Date.now() - 4 * 60_000).toISOString(),
      segments: '[{"text": "Платёж создан: "}, {"text": "Аренда за сентябрь", "link": {"kind": "payment", "id": "11111111-1111-4111-8111-111111111111"}}]',
      searchable: `Платёж создан: Аренда за сентябрь Иван Иванов ${user.email}`,
    },
    user,
  );
  await seedJournalEntry(
    {
      id: 'c0000000-0000-4000-8000-000000000002',
      createdAt: new Date(Date.now() - 3 * 60_000).toISOString(),
      segments: '[{"text": "Название объекта изменено: Квартира на Ленина"}]',
      searchable: 'Название объекта изменено: Квартира на Ленина Иван Иванов',
      action: 'property.renamed',
      baseAction: 'changed',
      kind: 'property',
    },
    user,
  );
  await seedJournalEntry(
    {
      id: 'c0000000-0000-4000-8000-000000000003',
      createdAt: new Date(Date.now() - 2 * 60_000).toISOString(),
      actorIdSql: memberActorIdSql(),
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
  await seedJournalEntry(
    {
      id: 'c0000000-0000-4000-8000-000000000004',
      createdAt: new Date(Date.now() - 60_000).toISOString(),
      propertyId: GARAGE_ID,
      actorIdSql: memberActorIdSql(),
      actorName: 'Мария Петрова',
      actorRole: 'full_access',
      segments: '[{"text": "Платёж удалён: Аренда гаража"}]',
      searchable: 'Платёж удалён: Аренда гаража Мария Петрова e2e-member@example.com',
      action: 'payment.deleted',
      baseAction: 'deleted',
    },
    user,
  );
}

test('вход из общей ленты по тапу на актёра: только его действия, объекты внутри дня', async ({ page, seededUser }, testInfo) => {
  await seedMemberJournal(seededUser);
  const actors = await trackHistoryScope(page);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/history');

  // Тап по шапке актёра в общей ленте (#712) ведёт на страницу его
  // действий с uuid юзера в пути.
  await page.getByRole('link', { name: 'Мария Петрова' }).first().click();
  await page.waitForURL(`**/history/participants/${MARIA_ID}`);

  // Прибитый человек: запрос ленты несёт actor_ids = один id (полл —
  // фетч стартует с маунтом экрана, позже смены адреса).
  await expect.poll(() => actors.lastActors()).toBe(MARIA_ID);

  // Заголовок страницы и лента только его действий; записи владельца
  // не приезжают.
  await expect(page.getByText('Действия участника').first()).toBeVisible();
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();
  await expect(page.getByText('Платёж удалён: Аренда гаража')).toBeVisible();
  await expect(page.getByText('Платёж создан:')).toHaveCount(0);
  await expect(page.getByText('Название объекта изменено: Квартира на Ленина')).toHaveCount(0);

  // Группировка переиспользована: чип дня, обе объектные секции внутри
  // дня (макет 2184-94731), шапки объектов кликабельны.
  await expect(page.getByText('Сегодня', { exact: true })).toBeVisible();
  await expect(page.getByRole('link', { name: /Квартира на Ленина/ }).first()).toBeVisible();
  await expect(page.getByRole('link', { name: /Гараж на Садовой/ }).first()).toBeVisible();

  // Шапки актёров на самой странице статичны — человек уже её предмет.
  await expect(page.getByRole('link', { name: 'Мария Петрова' })).toHaveCount(0);
  await expect(page.getByRole('heading', { name: 'Мария Петрова' }).first()).toBeVisible();

  // «Настройки» на месте (макет 2184-94731).
  await expect(page.getByRole('button', { name: 'Настройки' })).toBeVisible();

  await captureScreen(page, testInfo, 'history-member-from-feed');
});

test('вход со страницы участника: кебаб ведёт на действия', async ({ page, seededUser }) => {
  await seedMemberJournal(seededUser);
  const actors = await trackHistoryScope(page);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/participants/${MARIA_ID}`);
  await expect(page.getByRole('heading', { name: 'Мария Петрова' })).toBeVisible();

  await page.locator('header[aria-label="Навигация экрана"]').getByRole('button', { name: 'Еще — действия с участником' }).click();
  await page.getByRole('menuitem', { name: 'Действия участника' }).click();
  await page.waitForURL(`**/history/participants/${MARIA_ID}`);

  await expect.poll(() => actors.lastActors()).toBe(MARIA_ID);
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();

  // «Назад» возвращает на страницу участника (источник входа).
  await page.locator('header[aria-label="Навигация экрана"]').getByRole('button', { name: 'Назад' }).click();
  await page.waitForURL(`**/participants/${MARIA_ID}`);
});

test('поиск переиспользуется: сужает действия человека, чужой текст не находится', async ({ page, seededUser }, testInfo) => {
  await seedMemberJournal(seededUser);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/history/participants/${MARIA_ID}`);
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();

  await page.getByRole('button', { name: 'Поиск по истории' }).click();
  const field = page.getByPlaceholder('Поиск действий');
  await expect(field).toBeFocused();

  // Своё — находится.
  await field.fill('Заменить кран');
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();
  await expect(page.getByText('Платёж удалён: Аренда гаража')).toHaveCount(0);

  // Чужое (текст владельца) внутри действий человека — «Ничего не
  // найдено» (канон без иллюстрации, макет 2092-166866).
  await field.fill('Аренда за сентябрь');
  await expect(page.getByText('Ничего не найдено')).toBeVisible();
  await expect(page.getByText('Задача выполнена: Заменить кран')).toHaveCount(0);

  // Очистка возвращает ленту действий.
  await page.getByRole('button', { name: 'Очистить поиск' }).click();
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();
  await expect(page.getByText('Платёж удалён: Аренда гаража')).toBeVisible();

  await captureScreen(page, testInfo, 'history-member-search');
});

test('фильтры переиспользуются, «Участники» прибит серым; «в ноль» — пусто без запроса', async ({ page, seededUser }, testInfo) => {
  await seedMemberJournal(seededUser);
  const actors = await trackHistoryScope(page);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/history/participants/${MARIA_ID}`);
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();

  await page.getByRole('button', { name: 'Настройки' }).click();
  const dialog = page.getByRole('dialog', { name: 'Фильтры истории' });
  await expect(dialog.getByText('Виды действий')).toBeVisible();
  await expect(dialog.getByText('Объекты')).toBeVisible();

  // Группа «Участники» показывает только прибитого человека — 1/1, серым
  // и незабираемым (#712, макет 2184-92510, решение владельца 23.09).
  await expect(dialog.getByText('Участники')).toBeVisible();
  await expect(dialog.getByText('1/1')).toBeVisible();
  const master = dialog.getByRole('checkbox', { name: 'Выбрать все: Участники' });
  await expect(master).toBeDisabled();
  await dialog.getByRole('button', { name: /Участники/ }).click();
  const pinned = dialog.getByRole('checkbox', { name: 'Мария Петрова' });
  await expect(pinned).toBeChecked();
  await expect(pinned).toBeDisabled();
  await expect(dialog.getByText('e2e-member@example.com')).toBeVisible();

  // «Виды действий» в ноль мастер-чекбоксом, применить — лента пуста
  // (семантика «ни одного», запроса нет), параметр kinds= в адресе; пин
  // в адрес не пишется.
  await dialog.getByRole('button', { name: /Виды действий/ }).click();
  await dialog.getByRole('checkbox', { name: 'Выбрать все: Виды действий' }).click();
  actors.reset();
  await dialog.getByRole('button', { name: 'Применить фильтры' }).click();
  await expect(page.getByText('Ничего не найдено')).toBeVisible();
  await expect(page).toHaveURL(/kinds=/);
  await expect(page).not.toHaveURL(/actors=/);
  expect(actors.lastActors()).toBeNull(); // «в ноль» — запроса не было вовсе

  // Сброс возвращает ленту: виды снова «все» (ключ запроса возвращается к
  // исходному — данные свежи в кэше, staleTime, рефетча нет), kinds уходит
  // из адреса. Прибитость человека к запросам здесь не проверяем — кэш
  // отдаёт без запроса; серверная правда actor_ids — в тестах входа выше.
  await page.getByRole('button', { name: 'Настройки' }).click();
  await page.getByRole('button', { name: 'Сбросить фильтры' }).click();
  await page.getByRole('button', { name: 'Применить фильтры' }).click();
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();
  await expect(page).not.toHaveURL(/kinds=/);

  await captureScreen(page, testInfo, 'history-member-filters');
});

test('участник без действий — «Действий не было», лупы нет (канон пустой книги)', async ({ page, seededUser }) => {
  await execE2eSql('DELETE FROM action_journal;');
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/history/participants/${MARIA_ID}`);

  await expect(page.getByText('Действий не было')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Поиск по истории' })).toHaveCount(0);
});

test('pending-участник: пункта «Действия участника» в кебабе нет', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  // Pending-приглашение на гараж (идентификатор — почта; действий у
  // приглашённого не бывает, userId не известен).
  await execE2eSql(
    `INSERT INTO property_member_invitations (id, property_id, email, role, invited_by, last_sent_at) ` +
      `VALUES ('88888888-8888-4888-8888-888888888882', '${GARAGE_ID}', 'e2e-pending@example.com', 'viewer', '11111111-1111-4111-8111-111111111111', now()) ` +
      `ON CONFLICT (id) DO NOTHING`,
  );
  try {
    await page.goto('/participants/e2e-pending%40example.com');
    await expect(page.getByText('e2e-pending@example.com').first()).toBeVisible();

    await page.locator('header[aria-label="Навигация экрана"]').getByRole('button', { name: 'Еще — действия с участником' }).click();
    await expect(page.getByRole('menuitem', { name: 'Пригласить в объект' })).toBeVisible();
    await expect(page.getByRole('menuitem', { name: 'Действия участника' })).toHaveCount(0);
  } finally {
    await execE2eSql(`DELETE FROM property_member_invitations WHERE id = '88888888-8888-4888-8888-888888888882'`);
  }
});
