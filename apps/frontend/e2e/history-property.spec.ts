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

test('правки объекта: пояснения по-русски, смена типа без «удалено», длинное описание не вылезает (карта #1444)', async ({ page, seededUser }, testInfo) => {
  test.setTimeout(60_000);
  await execE2eSql('DELETE FROM action_journal;');
  await openCabinetWithSeededSession(page, seededUser);

  // Реальные PATCH-и: дифф пишет бекенд при записи (детали — готовые
  // строки, ADR 0061 §5). Сценарий брифа владельца 15.10: квартира с
  // характеристиками меняет тип на дом — подмена набора не пишет
  // «удалено», значения enum-полей читаются по-русски.
  const withAttrs = await page.request.patch(`/api/properties/${APARTMENT_ID}`, {
    data: { attributes: { balcony: 'loggia', area_total: 50 } },
  });
  expect(withAttrs.status()).toBe(200);
  const toHouse = await page.request.patch(`/api/properties/${APARTMENT_ID}`, {
    data: { type: 'house', attributes: { area_total: 50, land_area: 6, material: 'brick' } },
  });
  expect(toHouse.status()).toBe(200);
  // Повторная смена — обратный путь тоже без шума удалений.
  const backToApartment = await page.request.patch(`/api/properties/${APARTMENT_ID}`, {
    data: { type: 'apartment', attributes: { area_total: 50, balcony: 'loggia' } },
  });
  expect(backToApartment.status()).toBe(200);
  // Длинное описание одним неразрывным куском — перенос, не вылезание.
  const longDescription = 'А'.repeat(200);
  const withDescription = await page.request.patch(`/api/properties/${APARTMENT_ID}`, {
    data: { description: longDescription },
  });
  expect(withDescription.status()).toBe(200);

  await page.goto(`/history/properties/${APARTMENT_ID}`);
  await expect(page.getByText('Объект изменён: Квартира на Ленина').first()).toBeVisible();

  // Баг 1 (карта #1444): значения атрибутов — русские метки каталога, не
  // wire-ключи («loggia»/«brick»).
  await expect(page.getByText('Балкон: «Лоджия»').first()).toBeVisible();
  await expect(page.getByText('Материал: «Кирпичный»')).toBeVisible();

  // Баг 3 (карта #1444): смена типа и повторная смена — строки «Тип», и
  // ни одного «удалено» от подмены набора характеристик.
  await expect(page.getByText('Тип: «Квартира» → «Дом»')).toBeVisible();
  await expect(page.getByText('Тип: «Дом» → «Квартира»')).toBeVisible();
  await expect(page.getByText(/удалено/)).toHaveCount(0);

  // Баг 2 (карта #1444): пояснение с длинным описанием переносится —
  // ни элемент, ни страница не вылезают за вьюпорт по горизонтали.
  const descriptionLine = page.getByText(new RegExp(`Описание: «${longDescription}»`));
  await expect(descriptionLine).toBeVisible();
  const wrap = await descriptionLine.evaluate((el) => ({
    element: el.scrollWidth - el.clientWidth,
    page: document.documentElement.scrollWidth - window.innerWidth,
  }));
  expect(wrap.element).toBeLessThanOrEqual(1);
  expect(wrap.page).toBeLessThanOrEqual(0);

  await captureScreen(page, testInfo, 'history-property-update-diff');
});
