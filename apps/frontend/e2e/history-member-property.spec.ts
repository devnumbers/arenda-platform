import {
  captureScreen,
  expect,
  execE2eSql,
  memberActorIdSql,
  memberTaskEntry,
  openCabinetWithSeededSession,
  paymentCreatedEntry,
  propertyRenamedEntry,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  seedJournalEntry,
  test,
  trackHistoryScope,
  type SeededUser,
} from './fixtures';

// «Действия участника в объекте» (карта #838, тикет #841; макет
// 2177-59620 — вход): та же лента «Истории действий», прибитая к паре
// человек+объект — actor_ids и property_ids по одному uuid (ADR 0061 §7,
// сервер AND'ит обе группы, бэк #708 без изменений), группы шита
// «Участники» и «Объекты» обе прибиты серым «1/1». Вход — строка
// «Действия участника в объекте» на «Правах участника».

const APARTMENT_ID = SEEDED_APARTMENT_PROPERTY_ID;
const GARAGE_ID = SEEDED_GARAGE_PROPERTY_ID;

/** uuid юзера сидового участника (actor_id журнала, он же participantId
 * маршрута прав). */
const MEMBER_UUID_SQL = `SELECT id FROM users WHERE email = 'e2e-member@example.com'`;

/** Три записи: владелец на квартире (чужой актёр) и участница на гараже
 * (чужой объект) не должны попадать в прибитую пару «участница +
 * квартира»; все сегодня. */
async function seedPairJournal(user: SeededUser): Promise<void> {
  await execE2eSql('DELETE FROM action_journal;');
  await seedJournalEntry(paymentCreatedEntry('e0000000-0000-4000-8000-000000000001', 5, 'Аренда за сентябрь'), user);
  await seedJournalEntry(memberTaskEntry('e0000000-0000-4000-8000-000000000002', 3), user);
  await seedJournalEntry(
    propertyRenamedEntry('e0000000-0000-4000-8000-000000000003', 1, 'Гараж на Садовой', {
      propertyId: GARAGE_ID,
      actorIdSql: memberActorIdSql(),
      actorName: 'Мария Петрова',
      actorRole: 'full_access',
    }),
    user,
  );
}

test('вход с «Прав участника»: лента прибита к паре, чужой актёр и чужой объект не приезжают', async ({ page, seededUser }, testInfo) => {
  await seedPairJournal(seededUser);
  const memberUuid = await execE2eSql(MEMBER_UUID_SQL);
  const scope = await trackHistoryScope(page);
  await openCabinetWithSeededSession(page, seededUser);

  await page.goto(`/participants/${memberUuid}/properties/${APARTMENT_ID}`);
  await expect(page.getByText('Права участника').first()).toBeVisible();

  // Строка входа (макет 2177-59620) ведёт на прибитую пару.
  await page.getByTestId('participant-property-actions').click();
  await page.waitForURL(`**/history/participants/${memberUuid}/properties/${APARTMENT_ID}`);

  // Двойной пин на запросе: actor_ids = uuid участницы AND property_ids =
  // квартира (полл — фетч стартует с маунтом экрана).
  await expect.poll(() => scope.lastActors()).toBe(memberUuid);
  await expect.poll(() => scope.lastProperties()).toBe(APARTMENT_ID);

  // Заголовок; запись пары на месте, владелец на квартире (чужой актёр)
  // и участница на гараже (чужой объект) не приезжают.
  await expect(page.getByText('Действия участника в объекте').first()).toBeVisible();
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();
  await expect(page.getByText('Платёж создан:')).toHaveCount(0);
  await expect(page.getByText('Гараж на Садовой')).toHaveCount(0);

  // Шапка-карточка объекта — одна над всей лентой (зеркало #840),
  // статична; шапки актёров статичны — человек тоже предмет страницы
  // (зеркало #712).
  await expect(page.getByRole('heading', { name: 'Квартира на Ленина' })).toBeVisible();
  await expect(page.getByRole('link', { name: /Квартира на Ленина/ })).toHaveCount(0);
  await expect(page.getByRole('link', { name: 'Мария Петрова' })).toHaveCount(0);

  // «Настройки» на месте (primary по контенту).
  await expect(page.getByRole('button', { name: 'Настройки' })).toBeVisible();

  await captureScreen(page, testInfo, 'history-member-property-feed');
});

test('шит: «Участники» и «Объекты» обе прибиты серым «1/1»; «в ноль» без запроса, пины не пишутся в адрес', async ({ page, seededUser }, testInfo) => {
  await seedPairJournal(seededUser);
  const memberUuid = await execE2eSql(MEMBER_UUID_SQL);
  const scope = await trackHistoryScope(page);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/history/participants/${memberUuid}/properties/${APARTMENT_ID}`);
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();

  await page.getByRole('button', { name: 'Настройки' }).click();
  const dialog = page.getByRole('dialog', { name: 'Фильтры истории' });
  await expect(dialog.getByText('Участники')).toBeVisible();

  // Обе группы прибиты: счётчик «1/1» у каждой, мастер-чекбоксы серые
  // (решение владельца 24.09 — зеркала #712/#840).
  await expect(dialog.getByText('1/1')).toHaveCount(2);
  await expect(dialog.getByRole('checkbox', { name: 'Выбрать все: Участники' })).toBeDisabled();
  await expect(dialog.getByRole('checkbox', { name: 'Выбрать все: Объекты' })).toBeDisabled();

  // Внутри групп — по одному прибитому: участница и квартира, оба
  // выбраны и незабираемы.
  await dialog.getByRole('button', { name: /Участники/ }).click();
  const pinnedMember = dialog.getByRole('checkbox', { name: 'Мария Петрова' });
  await expect(pinnedMember).toBeChecked();
  await expect(pinnedMember).toBeDisabled();
  await dialog.getByRole('button', { name: /Объекты/ }).click();
  const pinnedProperty = dialog.getByRole('checkbox', { name: 'Квартира на Ленина' });
  await expect(pinnedProperty).toBeChecked();
  await expect(pinnedProperty).toBeDisabled();

  // «Виды действий» в ноль, применить: лента пуста БЕЗ запроса (семантика
  // «ни одного»); пины в адрес не пишутся, kinds= — да.
  await dialog.getByRole('button', { name: /Виды действий/ }).click();
  await dialog.getByRole('checkbox', { name: 'Выбрать все: Виды действий' }).click();
  scope.reset();
  await dialog.getByRole('button', { name: 'Применить фильтры' }).click();
  await expect(page.getByText('Ничего не найдено')).toBeVisible();
  await expect(page).toHaveURL(/kinds=/);
  await expect(page).not.toHaveURL(/actors=/);
  await expect(page).not.toHaveURL(/objects=/);
  expect(scope.lastActors()).toBeNull(); // «в ноль» — запроса не было вовсе
  expect(scope.lastProperties()).toBeNull();

  // Сброс возвращает ленту пары: ключ запроса возвращается к исходному —
  // данные свежи в кэше (staleTime), рефетча нет, пины в запрос не
  // возвращаются.
  await page.getByRole('button', { name: 'Настройки' }).click();
  await page.getByRole('button', { name: 'Сбросить фильтры' }).click();
  await page.getByRole('button', { name: 'Применить фильтры' }).click();
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();
  await expect(page).not.toHaveURL(/kinds=/);

  await captureScreen(page, testInfo, 'history-member-property-filters');
});
