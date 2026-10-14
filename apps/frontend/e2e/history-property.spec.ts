import {
  captureScreen,
  expect,
  execE2eSql,
  memberTaskEntry,
  openCabinetWithSeededSession,
  paymentCreatedEntry,
  propertyRenamedEntry,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  seedJournalEntry,
  screenHeader,
  test,
  todayAt,
  trackHistoryScope,
  type SeededUser,
} from './fixtures';

// «История объекта» (карта #1324, тикет #1321; макет 3258-76984): та же
// лента «Истории действий», прибитая к одному объекту — property_ids =
// один uuid (ADR 0061 §7), поиск и фильтры переиспользуются, группа
// «Объекты» шита прибита серым. Объект — предмет страницы и живёт в
// сабтайтле шапки; над лентой сразу плашка дня и карточки актёров. Вход —
// кебаб «Участников объекта» (макет 1980-139712).

const APARTMENT_ID = SEEDED_APARTMENT_PROPERTY_ID;
const GARAGE_ID = SEEDED_GARAGE_PROPERTY_ID;

/** Три записи квартиры (владелец и участница Мария) и одна запись гаража —
 * чужой объект не должен попадать в прибитую ленту; все сегодня. */
async function seedPropertyJournal(user: SeededUser): Promise<void> {
  await execE2eSql('DELETE FROM action_journal;');
  await seedJournalEntry(
    paymentCreatedEntry('d0000000-0000-4000-8000-000000000001', 5, 'Аренда за сентябрь', {
      segments: '[{"text": "Платёж создан: "}, {"text": "Аренда за сентябрь", "link": {"kind": "payment", "id": "11111111-1111-4111-8111-111111111111"}}]',
    }),
    user,
  );
  await seedJournalEntry(propertyRenamedEntry('d0000000-0000-4000-8000-000000000002', 4, 'Квартира на Ленина'), user);
  await seedJournalEntry(memberTaskEntry('d0000000-0000-4000-8000-000000000003', 3), user);
  await seedJournalEntry(
    {
      id: 'd0000000-0000-4000-8000-000000000004',
      createdAtSql: todayAt(1),
      propertyId: GARAGE_ID,
      text: 'Платёж удалён: Аренда гаража',
      action: 'payment.deleted',
      baseAction: 'deleted',
    },
    user,
  );
}

test('вход из кебаба «Участников объекта»: лента только этого объекта, имя объекта в сабтайтле шапки', async ({ page, seededUser }, testInfo) => {
  await seedPropertyJournal(seededUser);
  const properties = await trackHistoryScope(page);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/properties/${APARTMENT_ID}/participants`);
  // TopNavTitle — span, не heading: ждём текст шапки.
  await expect(page.getByText('Участники объекта').first()).toBeVisible();

  await screenHeader(page).getByRole('button', { name: 'Еще — действия со списком' }).click();
  await page.getByRole('menuitem', { name: 'История объекта' }).click();
  await page.waitForURL(`**/history/properties/${APARTMENT_ID}`);

  // Прибитый объект: запрос ленты несёт property_ids = один id (полл —
  // фетч стартует с маунтом экрана, позже смены адреса).
  await expect.poll(() => properties.lastProperties()).toBe(APARTMENT_ID);

  // Заголовок страницы; записи квартиры на месте — обеих актёров, чужой
  // объект (гараж) не приезжает.
  await expect(page.getByText('История объекта').first()).toBeVisible();
  await expect(page.getByText('Платёж создан:')).toBeVisible();
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();
  await expect(page.getByText('Платёж удалён: Аренда гаража')).toHaveCount(0);

  // Объект — предмет страницы и живёт в сабтайтле шапки (макет
  // 3258-76984), серым под заголовком; шапки-карточки над лентой больше
  // нет, имя в шапке не ссылка. Скоуп к шапке: то же имя звучит и в
  // строке «Название объекта изменено» ленты.
  await expect(screenHeader(page).getByText('Квартира на Ленина')).toBeVisible();
  await expect(screenHeader(page).getByRole('link', { name: 'Квартира на Ленина' })).toHaveCount(0);
  await expect(page.getByText('Москва, ул. Ленина, 1')).toHaveCount(0);

  // Группировка вырождена: внутри дня сразу карточки актёров; шапка
  // актёра — вход в «Действия участника в объекте» (#1361: прибитая
  // пара человек+объект, глобальной страницы «Действия участника»
  // больше нет).
  await expect(page.getByText('Сегодня', { exact: true })).toBeVisible();
  await page.getByRole('link', { name: 'Мария Петрова' }).first().click();
  await page.waitForURL(`**/history/participants/**/properties/${APARTMENT_ID}`);
  await expect(page.getByText('Действия участника в объекте').first()).toBeVisible();

  // «Настройки» на месте (макет 3258-76984).
  await page.goBack();
  await expect(page.getByRole('button', { name: 'Настройки' })).toBeVisible();

  await captureScreen(page, testInfo, 'history-property-from-kebab');
});

test('поиск ищет в пределах объекта, чужой объект не находится', async ({ page, seededUser }, testInfo) => {
  await seedPropertyJournal(seededUser);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/history/properties/${APARTMENT_ID}`);
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();

  await page.getByRole('button', { name: 'Поиск по истории' }).click();
  const field = page.getByPlaceholder('Поиск действий');
  await expect(field).toBeFocused();

  // Своё — находится.
  await field.fill('Заменить кран');
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();
  await expect(page.getByText('Платёж создан:')).toHaveCount(0);

  // Чужое (запись гаража) — «Ничего не найдено» (канон без иллюстрации).
  await field.fill('Аренда гаража');
  await expect(page.getByText('Ничего не найдено')).toBeVisible();
  await expect(page.getByText('Задача выполнена: Заменить кран')).toHaveCount(0);

  // Очистка возвращает ленту объекта.
  await page.getByRole('button', { name: 'Очистить поиск' }).click();
  await expect(page.getByText('Платёж создан:')).toBeVisible();

  await captureScreen(page, testInfo, 'history-property-search');
});

test('фильтры переиспользуются, «Объект» прибит серым; «в ноль» — пусто без запроса', async ({ page, seededUser }, testInfo) => {
  await seedPropertyJournal(seededUser);
  const properties = await trackHistoryScope(page);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/history/properties/${APARTMENT_ID}`);
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();

  await page.getByRole('button', { name: 'Настройки' }).click();
  const dialog = page.getByRole('dialog', { name: 'Фильтры истории' });
  await expect(dialog.getByText('Основные действия')).toBeVisible();
  await expect(dialog.getByText('Участники')).toBeVisible();

  // Группа «Объект» — только прибитый объект, 1/1, серым и незабираемым
  // (#840, макет 2184-94176, решение владельца 24.09; подпись прибитой
  // группы в единственном числе — решение владельца 10.10.2026,
  // доработка #1321).
  await expect(dialog.getByText('Объект', { exact: true })).toBeVisible();
  await expect(dialog.getByText('1/1')).toBeVisible();
  const master = dialog.getByRole('checkbox', { name: 'Выбрать все: Объект' });
  await expect(master).toBeDisabled();
  await dialog.getByRole('button', { name: /Объект/ }).click();
  const pinned = dialog.getByRole('checkbox', { name: 'Квартира на Ленина' });
  await expect(pinned).toBeChecked();
  await expect(pinned).toBeDisabled();
  await expect(dialog.getByText('Москва, ул. Ленина, 1')).toBeVisible();

  // «Виды действий» в ноль мастер-чекбоксом, применить — лента пуста
  // (семантика «ни одного», запроса нет), параметр kinds= в адресе; пин
  // в адрес не пишется.
  await dialog.getByRole('button', { name: /Виды действий/ }).click();
  await dialog.getByRole('checkbox', { name: 'Выбрать все: Виды действий' }).click();
  properties.reset();
  await dialog.getByRole('button', { name: 'Применить фильтры' }).click();
  await expect(page.getByText('Ничего не найдено')).toBeVisible();
  await expect(page).toHaveURL(/kinds=/);
  await expect(page).not.toHaveURL(/objects=/);
  expect(properties.lastProperties()).toBeNull(); // «в ноль» — запроса не было вовсе

  // Сброс возвращает ленту (ключ запроса возвращается к исходному —
  // данные свежи в кэше, staleTime, рефетча нет), kinds уходит из адреса.
  await page.getByRole('button', { name: 'Настройки' }).click();
  await page.getByRole('button', { name: 'Сбросить фильтры' }).click();
  await page.getByRole('button', { name: 'Применить фильтры' }).click();
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();
  await expect(page).not.toHaveURL(/kinds=/);

  await captureScreen(page, testInfo, 'history-property-filters');
});

test('объект без действий — «Действий не было», лупы нет (канон пустой книги)', async ({ page, seededUser }) => {
  await execE2eSql('DELETE FROM action_journal;');
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/history/properties/${GARAGE_ID}`);

  await expect(page.getByText('Действий не было')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Поиск по истории' })).toHaveCount(0);
});

test('открытие «с конца»: последние записи объекта в вьюпорте (карта #1324, D8)', async ({ page, seededUser }) => {
  test.setTimeout(60_000);
  await execE2eSql('DELETE FROM action_journal;');
  // Серия из 30 записей квартиры — лента прибитого скоупа заведомо выше
  // вьюпорта; открытие строго на последних записях, без флэша верха.
  for (let index = 0; index < 30; index += 1) {
    await seedJournalEntry(
      paymentCreatedEntry(
        `e0000000-0000-4000-8000-${String(index + 1).padStart(12, '0')}`,
        60 - index,
        `Заготовка ${index + 1}`,
      ),
      seededUser,
    );
  }
  await openCabinetWithSeededSession(page, seededUser);

  await page.goto(`/history/properties/${APARTMENT_ID}`);
  await expect(page.getByText('Платёж создан: Заготовка 30', { exact: true })).toBeVisible();
  const metrics = await page.evaluate(() => ({
    distanceToBottom:
      document.documentElement.scrollHeight - (window.scrollY + window.innerHeight),
  }));
  expect(metrics.distanceToBottom).toBeLessThan(120);
});
