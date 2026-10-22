import type { Page } from '@playwright/test';
import {
  captureScreen,
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  openCabinetWithSessionToken,
  screenHeader,
  seedJournalEntry,
  seededMemberSessionToken,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  SEEDED_STUDIO_PROPERTY_ID,
  test,
} from './fixtures';

// Страница участника и её подэкраны (карта #692, тикет #698): блок
// «аватар — имя — почта — чип агрегата», «Доступные объекты» с бейджами
// ролей, экран «Права участника» (сегмент роли, отзыв из объекта) и
// «Пригласить в объект» (мультичек, POST /participants/{id}/properties).
// Сид: владелец «Квартиры на Ленина» (33333333-…) с двумя участниками —
// Мария Петрова (full_access, 12111111-…21) и Сергей Сидоров (viewer,
// 13111111-…31), оба active на квартире; гараж (44444444-…) свободен.
// Разрушающие тесты восстанавливают сид через execE2eSql (workers=1).

const MARIA_ID = '12111111-1111-4111-8111-111111111121';
const SERGEY_ID = '13111111-1111-4111-8111-111111111131';
// Свободный сид-юзер (без подписки, лимит исчерпан) для suspended/slot-
// сценариев приглашений — у Марии и Сергея подписки pro для гейтов #997.
const LIMITED_ID = '14111111-1111-4111-8111-111111111141';
const APARTMENT_ID = '33333333-3333-4333-8333-333333333333';
const GARAGE_ID = '44444444-4444-4444-8444-444444444444';

test('страница участника: шапка, чип агрегата, ряды объектов с бейджами ролей, кебаб', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/participants/${MARIA_ID}`);

  const header = screenHeader(page);

  // exact+first: в переходный момент в шапке встречается дубль-спан.
  await expect(header.getByText('Участник', { exact: true }).first()).toBeVisible();

  // Блок участника: имя, почта; у partial-агрегата бейджа нет — статусы
  // «частично/все объекты» ушли с экранов (D3 #1313).
  await expect(page.getByRole('heading', { name: 'Мария Петрова' })).toBeVisible();
  await expect(page.getByText('e2e-member@example.com')).toBeVisible();
  await expect(page.getByText(/Доступно \d+/)).toHaveCount(0);
  await expect(page.getByText('Доступ ко всем объектам')).toHaveCount(0);

  await expect(page.getByText('Доступные объекты')).toBeVisible();
  const apartmentRow = page.getByRole('button', { name: /Квартира на Ленина/ });
  await expect(apartmentRow).toBeVisible();
  await expect(apartmentRow.getByText('Управление')).toBeVisible();

  // Кебаб шапки: приглашение, история действий (карта #1582 — лента
  // одного участника по его объектам) и отзыв; пункта «Действия
  // участника» в меню нет — имя носит сама страница, пункт называется
  // «История действий» (22.10.2026).
  await header.getByRole('button', { name: 'Еще — действия с участником' }).click();
  await expect(page.getByRole('menuitem', { name: 'Пригласить в объект' })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'История действий' })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'Отозвать доступ к объектам' })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'Действия участника' })).toHaveCount(0);
  await page.keyboard.press('Escape');

  await captureScreen(page, testInfo, 'participant-page');
});

test('кебаб: «История действий» — лента одного участника по его объектам (карта #1582, переработка 22.10.2026); сид восстанавливается', async ({ page, seededUser }, testInfo) => {
  // Четыре строки журнала разводят обе оси скоупа страницы: строки МАРИИ
  // на её объектах (квартира — сид-нога, гараж — досажен SQL'ем) —
  // остаются; строка ВЛАДЕЛЬЦА на квартире (чужой человек для ленты
  // участника) и строка МАРИИ в студии (объект без её ноги — «чужой
  // объект») — обе за бортом. Вторая нога и четыре строки — разрушающие
  // правки, finally восстанавливает сид (workers=1).
  const OWNER_ID = '11111111-1111-4111-8111-111111111111';
  const GARAGE_LEG_ID = '99999999-9999-4999-8999-999999999941';
  const ROW_MARIA_APARTMENT_ID = '9b111111-1111-4111-8111-111111111191';
  const ROW_MARIA_GARAGE_ID = '9b111111-1111-4111-8111-111111111192';
  const ROW_MARIA_STUDIO_ID = '9b111111-1111-4111-8111-111111111193';
  const ROW_OWNER_APARTMENT_ID = '9b111111-1111-4111-8111-111111111195';
  const NOW_TEN_AM_SQL = "date_trunc('day', now()) + interval '10 hours'";
  const MARIA_ACTOR_SQL = `'${MARIA_ID}'`;
  try {
    await seedJournalEntry(
      {
        id: ROW_MARIA_APARTMENT_ID,
        propertyId: SEEDED_APARTMENT_PROPERTY_ID,
        text: 'Запись Марии на квартире',
        createdAtSql: NOW_TEN_AM_SQL,
        actorIdSql: MARIA_ACTOR_SQL,
        actorName: 'Мария Петрова',
        actorRole: 'full_access',
      },
      seededUser,
    );
    await seedJournalEntry(
      {
        id: ROW_MARIA_GARAGE_ID,
        propertyId: SEEDED_GARAGE_PROPERTY_ID,
        text: 'Запись Марии в гараже',
        createdAtSql: NOW_TEN_AM_SQL,
        actorIdSql: MARIA_ACTOR_SQL,
        actorName: 'Мария Петрова',
        actorRole: 'viewer',
      },
      seededUser,
    );
    await seedJournalEntry(
      {
        id: ROW_MARIA_STUDIO_ID,
        propertyId: SEEDED_STUDIO_PROPERTY_ID,
        text: 'Запись Марии в студии',
        createdAtSql: NOW_TEN_AM_SQL,
        actorIdSql: MARIA_ACTOR_SQL,
        actorName: 'Мария Петрова',
        actorRole: 'viewer',
      },
      seededUser,
    );
    await seedJournalEntry(
      { id: ROW_OWNER_APARTMENT_ID, propertyId: SEEDED_APARTMENT_PROPERTY_ID, text: 'Запись владельца на квартире', createdAtSql: NOW_TEN_AM_SQL },
      seededUser,
    );
    await execE2eSql(
      `INSERT INTO property_members (id, property_id, user_id, role, granted_by) ` +
        `VALUES ('${GARAGE_LEG_ID}', '${SEEDED_GARAGE_PROPERTY_ID}', '${MARIA_ID}', 'viewer', '${OWNER_ID}') ` +
        `ON CONFLICT (property_id, user_id) WHERE status = 'active' DO NOTHING`,
    );

    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`/participants/${MARIA_ID}`);

    // Скоуп выводит сервер: запрос уходит на под-ресурс участника БЕЗ
    // actor_ids/property_ids (их выводит агрегат, контракт #1583).
    const participantFeed = page.waitForRequest(
      (request) => request.url().includes(`/participants/${MARIA_ID}/history`),
    );
    const header = screenHeader(page);
    await header.getByRole('button', { name: 'Еще — действия с участником' }).click();
    await page.getByRole('menuitem', { name: 'История действий' }).click();
    await page.waitForURL(new RegExp(`/participants/${MARIA_ID}/history$`));

    const feedUrl = (await participantFeed).url();
    expect(feedUrl).not.toContain('property_ids=');
    expect(feedUrl).not.toContain('actor_ids=');

    // Экран: заголовок «Действия участника», объекты участника — шапками
    // (first: в общей БД e2e секций одного объекта бывает несколько — по
    // одной на день); чужая строка владельца и объект без ноги Марии —
    // ни шапкой, ни строкой.
    await expect(screenHeader(page).getByText('Действия участника', { exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Квартира на Ленина' }).first()).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Гараж на Садовой' }).first()).toBeVisible();
    await expect(page.getByText('Запись Марии на квартире')).toBeVisible();
    await expect(page.getByText('Запись Марии в гараже')).toBeVisible();
    await expect(page.getByText('Запись владельца на квартире')).toHaveCount(0);
    await expect(page.getByText('Запись Марии в студии')).toHaveCount(0);
    await expect(page.getByText('Студия на Полевой')).toHaveCount(0);

    await captureScreen(page, testInfo, 'participant-history-entry');

    // Шит «Настройки» — аменда владельца 22.10.2026 («стандартная кнопка
    // настроек и фильтр как и на всех остальных страницах; фильтр по
    // пользователю заблокирован»): группа «Участник» прибита серым
    // (1/1, чекбоксы disabled), «Объекты» предлагает только ноги
    // агрегата (студии без ноги нет).
    await expect(page.getByRole('button', { name: 'Настройки' })).toBeVisible();
    await page.getByRole('button', { name: 'Настройки' }).click();
    const sheet = page.getByRole('dialog', { name: 'Фильтры истории' });
    await expect(sheet.getByText('Действия участника', { exact: true })).toBeVisible();
    await expect(sheet.getByText('Участник', { exact: true })).toBeVisible();
    const participantMaster = sheet.getByRole('checkbox', { name: 'Выбрать все: Участник' });
    await expect(participantMaster).toBeDisabled();
    const mariaRow = sheet.getByRole('checkbox', { name: 'Мария Петрова' });
    await expect(mariaRow).toBeDisabled();
    await expect(mariaRow).toBeChecked();
    await sheet.getByRole('button', { name: /Объекты/ }).click();
    await expect(sheet.getByText('Квартира на Ленина')).toBeVisible();
    await expect(sheet.getByText('Гараж на Садовой')).toBeVisible();
    await expect(sheet.getByText('Студия на Полевой')).toHaveCount(0);

    // Выбор «Объектов» сужает ленту пересечением с ногами на сервере:
    // в адрес уходит objects=, в запрос под-ресурса — property_ids.
    await sheet.getByRole('checkbox', { name: 'Выбрать все: Объекты' }).click();
    await sheet.getByRole('checkbox', { name: 'Гараж на Садовой' }).click();
    const narrowedFeed = page.waitForRequest(
      (request) =>
        request.url().includes(`/participants/${MARIA_ID}/history`) &&
        request.url().includes(`property_ids=${SEEDED_GARAGE_PROPERTY_ID}`),
    );
    await sheet.getByRole('button', { name: 'Применить фильтры' }).click();
    expect((await narrowedFeed).url()).toContain(`property_ids=${SEEDED_GARAGE_PROPERTY_ID}`);
    expect(new URL(page.url()).searchParams.get('objects')).toBe(SEEDED_GARAGE_PROPERTY_ID);
    await expect(page.getByText('Запись Марии в гараже')).toBeVisible();
    await expect(page.getByText('Запись Марии на квартире')).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Квартира на Ленина' })).toHaveCount(0);

    // «Назад» снимает применённые фильтры (канон §3: применение — push,
    // «назад» возвращает без фильтров), второй — живым переходом на
    // страницу участника.
    await header.getByRole('button', { name: 'Назад' }).click();
    await page.waitForURL(
      (url) =>
        url.pathname.endsWith(`/participants/${MARIA_ID}/history`) &&
        url.searchParams.get('objects') === null,
    );
    await expect(page.getByText('Запись Марии на квартире')).toBeVisible();
    await header.getByRole('button', { name: 'Назад' }).click();
    await page.waitForURL(new RegExp(`/participants/${MARIA_ID}$`));
  } finally {
    await execE2eSql(
      `DELETE FROM property_members WHERE id = '${GARAGE_LEG_ID}'; ` +
        `DELETE FROM action_journal WHERE id IN ('${ROW_MARIA_APARTMENT_ID}', '${ROW_MARIA_GARAGE_ID}', '${ROW_MARIA_STUDIO_ID}', '${ROW_OWNER_APARTMENT_ID}')`,
    );
  }
});

test('pending-участник: «История действий» — пустая лента (своих действий не бывает); сид восстанавливается', async ({ page, seededUser }, testInfo) => {
  // Pending-нога (приглашение по email) — тоже «объект, в который
  // добавили»: пункт кебаба живёт, страница открывается. Своих действий у
  // pending не бывает (actor_id журнала — uuid юзера, его ещё нет,
  // ADR 0061 §3) — страница отвечает пустой лентой «Действий не было»,
  // строки приглашённых объектов туда не попадают (это действия
  // читающего, не участника). Идентификатор агрегата pending составляет
  // бэкенд — открываем страницу кликом по ряду списка.
  const PENDING_EMAIL = 'e2e-pending-history@example.com';
  const INVITATION_ID = '99999999-9999-4999-8999-999999999951';
  const JOURNAL_ID = '9b111111-1111-4111-8111-111111111194';
  const OWNER_ID = '11111111-1111-4111-8111-111111111111';
  const NOW_TEN_AM_SQL = "date_trunc('day', now()) + interval '10 hours'";
  try {
    await execE2eSql(
      `INSERT INTO property_member_invitations (id, property_id, email, role, invited_by, last_sent_at) ` +
        `VALUES ('${INVITATION_ID}', '${SEEDED_APARTMENT_PROPERTY_ID}', '${PENDING_EMAIL}', 'viewer', '${OWNER_ID}', now()) ` +
        `ON CONFLICT (property_id, lower(email)) DO NOTHING`,
    );
    await seedJournalEntry(
      { id: JOURNAL_ID, propertyId: SEEDED_APARTMENT_PROPERTY_ID, text: 'Тест-запись приглашения', createdAtSql: NOW_TEN_AM_SQL },
      seededUser,
    );

    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/participants');
    // Ряд pending-агрегата (аменда владельца 22.10.2026, макет
    // 3313-77128): титул «Пользователь», почта подзаголовком, бейдж
    // «Аккаунт не создан» — почта больше не титул (#693 амендован).
    const pendingRow = page.getByRole('button', { name: new RegExp(PENDING_EMAIL) });
    await expect(pendingRow.getByText('Пользователь', { exact: true })).toBeVisible();
    await expect(pendingRow.getByText(PENDING_EMAIL)).toBeVisible();
    await expect(pendingRow.getByText('Аккаунт не создан')).toBeVisible();
    await pendingRow.click();
    await page.waitForURL(/\/participants\/[^/]+$/);
    const participantPath = new URL(page.url()).pathname;
    // Своё pending-состояние на странице: бейдж «Аккаунт не создан» (D3
    // #1313) — значит, открыт именно pending-агрегат.
    await expect(page.getByText('Аккаунт не создан')).toBeVisible();

    // Страница pending-участника: /participants/{агрегат-id}/history.
    // Статус отдельно не ловим: предикат запроса цепляет и RSC-префетч
    // Next (у отменённого префетча .response() не settles) — пустое
    // состояние и есть доказательство честного пустого 200 (ошибка
    // рисовала бы ErrorCard). Путь сравниваем на двойном decode:
    // клиентская навигация Next может отдать сегмент с email в другом
    // числе процент-кодирований, чем исходный адрес страницы (квирк
    // параметров клиент-навигации; приложение при этом резолвит агрегат
    // корректно — квирк кодирования в пути существует и у самой
    // страницы участника).
    const header = screenHeader(page);
    await header.getByRole('button', { name: 'Еще — действия с участником' }).click();
    await page.getByRole('menuitem', { name: 'История действий' }).click();
    const participantPathDecoded = decodeURIComponent(participantPath);
    await page.waitForURL((url) => {
      try {
        return decodeURIComponent(decodeURIComponent(url.pathname)) === `${participantPathDecoded}/history`;
      } catch {
        return url.pathname === `${participantPath}/history`;
      }
    });

    // Пустая лента: «Действий не было» (канон пустой книги, без
    // иллюстрации); чужих строк и шапок объектов нет. Кнопка «Настройки»
    // стоит и на пустой ленте (канон шита #711); в шите группа
    // «Участник» не рисуется (pending нет в опциях — гарда прибитой
    // строки), «Объекты» показывает приглашённый объект.
    await expect(page.getByText('Действий не было')).toBeVisible();
    await expect(page.getByText('Тест-запись приглашения')).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Квартира на Ленина' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Настройки' })).toBeVisible();
    await page.getByRole('button', { name: 'Настройки' }).click();
    const pendingSheet = page.getByRole('dialog', { name: 'Фильтры истории' });
    await expect(pendingSheet.getByText('Участник', { exact: true })).toHaveCount(0);
    await pendingSheet.getByRole('button', { name: /Объекты/ }).click();
    await expect(pendingSheet.getByText('Квартира на Ленина')).toBeVisible();
    await pendingSheet.getByRole('button', { name: 'Закрыть фильтры' }).click();
    await expect(page.getByText('Действий не было')).toBeVisible();

    await captureScreen(page, testInfo, 'participant-history-pending');

    // «Назад» возвращает на страницу pending-участника (путь — тоже на
    // decode, симметрично пути туда).
    await header.getByRole('button', { name: 'Назад' }).click();
    await page.waitForURL((url) => {
      try {
        return decodeURIComponent(url.pathname) === participantPathDecoded;
      } catch {
        return url.pathname === participantPath;
      }
    });
  } finally {
    await execE2eSql(
      `DELETE FROM property_member_invitations WHERE id = '${INVITATION_ID}'; ` +
        `DELETE FROM action_journal WHERE id = '${JOURNAL_ID}'`,
    );
  }
});

test('безымянный участник: «Пользователь» — заголовок шапки, почта подзаголовком (карта #1105, аменд #1123); сид восстанавливается', async ({ page, seededUser }, testInfo) => {
  // Тот же канон карты #1105, что и в ряду списка: display_name
  // безымянного собирает бекенд (#1106) — «Пользователь» (аменд #1123:
  // телефон больше не фолбэк; из схемы он ушёл совсем — ADR 0067,
  // миграция 000162, пин «телефона нет» — страховка от остаточных масок).
  // Идентификаторы
  // свои (не из participants-list.spec.ts): утечка строки одного теста
  // не должна подменять данные другого (ON CONFLICT (id) DO NOTHING).
  const NAMELESS_ID = '16111111-1111-4111-8111-111111111162';
  const NAMELESS_MEMBER_ID = '99999999-9999-4999-8999-999999999962';
  const NAMELESS_PHONE = '+79137654322';
  const NAMELESS_EMAIL = 'e2e-nameless-page@example.com';
  await execE2eSql(
    `INSERT INTO users (id, role, name, surname, email, timezone) ` +
      `VALUES ('${NAMELESS_ID}', 'owner', NULL, NULL, '${NAMELESS_EMAIL}', 'UTC') ` +
      `ON CONFLICT (id) DO NOTHING; ` +
      `INSERT INTO property_members (id, property_id, user_id, role, granted_by) ` +
      `VALUES ('${NAMELESS_MEMBER_ID}', '${APARTMENT_ID}', '${NAMELESS_ID}', 'viewer', '11111111-1111-4111-8111-111111111111') ` +
      `ON CONFLICT (property_id, user_id) WHERE status = 'active' DO NOTHING`,
  );
  try {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`/participants/${NAMELESS_ID}`);

    // Шапка (2008-81468): имени нет — «Пользователь» H1, почта
    // подзаголовком (не прячется); у partial-агрегата бейджа нет (D3
    // #1313).
    await expect(page.getByRole('heading', { name: 'Пользователь' })).toBeVisible();
    await expect(page.getByText(NAMELESS_EMAIL)).toBeVisible();
    await expect(page.getByText(/Доступно \d+/)).toHaveCount(0);
    await expect(page.getByText(NAMELESS_PHONE)).toHaveCount(0);
    await expect(page.getByText(/\*{3}/)).toHaveCount(0);

    await captureScreen(page, testInfo, 'participant-page-nameless');
  } finally {
    await execE2eSql(
      `DELETE FROM property_members WHERE id = '${NAMELESS_MEMBER_ID}'; ` +
        `DELETE FROM users WHERE id = '${NAMELESS_ID}'`,
    );
  }
});

test('deep-link на отозванного/чужого участника — «Участник не найден»', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  // Существующий пользователь, но вне сцопа читающего — приватный 404.
  await page.goto('/participants/00000000-0000-4000-8000-000000000000');

  const header = screenHeader(page);

  await expect(page.getByText('Участник не найден')).toBeVisible();
  // Кебаб не показывается без данных (§7), назад в шапке жив.
  await expect(header.getByRole('button', { name: 'Еще — действия с участником' })).toHaveCount(0);
  await expect(header.getByRole('button', { name: 'Назад' })).toBeVisible();
});

test('права участника: смена роли сегментом, тост «Права изменены»; сид восстанавливается', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/participants/${MARIA_ID}`);
  const header = screenHeader(page);

  await page.getByRole('button', { name: /Квартира на Ленина/ }).click();
  await expect(header.getByText('Права участника')).toBeVisible();
  await expect(page.getByRole('radio', { name: 'Управление' })).toBeChecked();

  // Карточка объекта — ссылка на объект и у владельца (амендмент
  // владельца 13.10.2026, общий с member-режимом #1362; без шеврона).
  const objectCard = page.getByRole('link', { name: /Квартира на Ленина/ });
  await expect(objectCard).toBeVisible();
  await expect(objectCard).toHaveAttribute('href', `/properties/${APARTMENT_ID}`);

  try {
    await page.getByRole('radio', { name: 'Просмотр' }).click();

    // Тост «Права изменены» (2177-59799; тост вместо успех-попапа,
    // решение владельца 19.10.2026); закрываем кнопкой, чтобы не
    // перехватывал клики (канон contacts-спек).
    const toast = page.locator('.Toastify__toast', { hasText: 'Права изменены' });
    await expect(toast).toBeVisible();
    await toast.getByRole('button', { name: 'Закрыть' }).click();

    // Бейдж на странице участника перечитан (инвалидация агрегатов).
    await header.getByRole('button', { name: 'Назад' }).click();
    await expect(
      page.getByRole('button', { name: /Квартира на Ленина/ }).getByText('Просмотр'),
    ).toBeVisible();
  } finally {
    await execE2eSql(
      `UPDATE property_members SET role = 'full_access' WHERE id = '99999999-9999-4999-8999-999999999931'`,
    );
  }
  await page.reload();
  await expect(
    page.getByRole('button', { name: /Квартира на Ленина/ }).getByText('Управление'),
  ).toBeVisible();
});

/** CDP-задержка сети: полёт мутации удлиняется — погашенное состояние
 * переключателя роли наблюдаемо (канон double-submit-guard; route-задержка
 * ненадёжна — грабля #1113). Возвращает функцию снятия эмуляции. */
async function holdNetworkLatency(page: Page, latencyMs: number): Promise<() => Promise<void>> {
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Network.enable');
  await cdp.send('Network.emulateNetworkConditions', {
    offline: false,
    latency: latencyMs,
    downloadThroughput: -1,
    uploadThroughput: -1,
  });
  return async () => {
    await cdp.send('Network.emulateNetworkConditions', {
      offline: false,
      latency: 0,
      downloadThroughput: -1,
      uploadThroughput: -1,
    });
  };
}

test('переключатель роли: курсор-пойнтер на кнопках, клип тени капсулы (#1395); в полёте мутации курсор default, сид восстанавливается', async ({ page, seededUser }) => {
  test.setTimeout(120_000);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/participants/${MARIA_ID}`);

  await page.getByRole('button', { name: /Квартира на Ленина/ }).click();
  await expect(page.getByRole('radio', { name: 'Управление' })).toBeChecked();

  // Курсор переключателю задаёт глобальное base-правило
  // button:not(:disabled) (#1395): сам сегмент локального cursor-класса
  // не несёт, поэтому снятие base-правила валит этот ассерт — радио и есть
  // канарейка глобальности.
  const manage = page.getByRole('radio', { name: 'Управление' });
  const view = page.getByRole('radio', { name: 'Просмотр' });
  await expect(manage).toHaveCSS('cursor', 'pointer');
  await expect(view).toHaveCSS('cursor', 'pointer');

  // Тень активной пилюли не выходит за капсулу: «Clip content» макета
  // 3298-75577, тот же CSS-аналог, что на тарифах лендинга (#1010).
  await expect(
    page.getByRole('radiogroup', { name: 'Роль участника на объекте' }),
  ).toHaveCSS('overflow', 'clip');

  // Погашенный сегмент в полёте мутации — курсор остаётся default:
  // base-правило с оговоркой :not(:disabled) (слово владельца).
  const releaseLatency = await holdNetworkLatency(page, 2000);
  try {
    await view.click();
    await expect(view).toBeDisabled();
    await expect(view).toHaveCSS('cursor', 'default');
  } finally {
    await releaseLatency();
  }

  await expect(
    page.locator('.Toastify__toast', { hasText: 'Права изменены' }),
  ).toBeVisible();
  await page
    .locator('.Toastify__toast', { hasText: 'Права изменены' })
    .getByRole('button', { name: 'Закрыть' })
    .click();

  try {
    // Сид восстанавливается — тот же член ряда, что в сценарии смены роли.
    await execE2eSql(
      `UPDATE property_members SET role = 'full_access' WHERE id = '99999999-9999-4999-8999-999999999931'`,
    );
  } finally {
    await page.reload();
  }
  await expect(manage).toBeChecked();
});

test('страница участника: отступы шапки по макету 2036-84861, h1 без легаси-маржина (#1402)', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/participants/${LIMITED_ID}`);

  const h1 = page.getByRole('heading', { name: 'Анна Лимитова' });

  // Легаси-маржин h1 (0.67em ≈ 18.76px на кегле 28) снесён глобально
  // (#1402, слово владельца): без этого base-правила ассерт красный.
  await expect(h1).toHaveCSS('margin-top', '0px');
  await expect(h1).toHaveCSS('margin-bottom', '0px');

  // Геометрия шапки: аватар / имя+почта / (чип) с зазорами 24 и 8 внутри
  // блока; контент страницы разнесён на 48 (2036-84861). Канарейка
  // глобальности маржина — сам h1: локального margin-класса он не несёт.
  // Ассерт 48 жив, пока сида Анны partial (бейджа и notice в шапке нет —
  // при дрейфе сида между почтой и секцией встанут чип+карточка).
  const avatar = page.locator('.h-24.rounded-pill').first();
  const email = page.getByText('e2e-limited@example.com');
  const section = page.getByRole('heading', { name: 'Доступные объекты' });
  const [avatarBox, h1Box, emailBox, sectionBox] = await Promise.all([
    avatar.boundingBox(),
    h1.boundingBox(),
    email.boundingBox(),
    section.boundingBox(),
  ]);
  if (!avatarBox || !h1Box || !emailBox || !sectionBox) {
    throw new Error('боксы шапки не измерились');
  }
  expect(Math.round(h1Box.y - avatarBox.y - avatarBox.height)).toBe(24);
  expect(Math.round(emailBox.y - h1Box.y - h1Box.height)).toBe(8);
  expect(Math.round(sectionBox.y - emailBox.y - emailBox.height)).toBe(48);
});

test('пригласить в объект: мультичек гаража, шит роли, «Доступ выдан»; сид восстанавливается', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  // Приглашаем Анну (без подписки — тарифный слот превышен, сценарий
  // suspended-гранта); у Марии и Сергея подписки pro для гейтов #997.
  await page.goto(`/participants/${LIMITED_ID}`);

  const header = screenHeader(page);

  await header.getByRole('button', { name: 'Еще — действия с участником' }).click();
  await page.getByRole('menuitem', { name: 'Пригласить в объект' }).click();

  await expect(header.getByText('Пригласить в объект')).toBeVisible();

  // Квартира уже выдана — в списке гараж и студия; «Все объекты» unchecked.
  const garageRow = page.getByRole('checkbox', { name: /Гараж на Садовой/ });
  await expect(garageRow).toBeVisible();
  await expect(page.getByRole('button', { name: /Квартира на Ленина/ })).toHaveCount(0);
  const allRow = page.getByRole('checkbox', { name: /Все объекты/ });
  await expect(allRow).toHaveAttribute('aria-checked', 'false');

  // Tri-state (2010-131721): выбран один из двух — «Все объекты» mixed.
  await garageRow.click();
  await expect(allRow).toHaveAttribute('aria-checked', 'mixed');

  const inviteButton = page.getByRole('button', { name: 'Пригласить' }).filter({ hasText: 'Пригласить' });
  await expect(inviteButton).toBeEnabled();
  await inviteButton.click();

  // Шит роли (макет 2010-131724): сегмент + кнопка «Пригласить».
  const sheet = page.getByRole('dialog');
  await sheet.getByRole('radio', { name: 'Управление' }).click();
  try {
    await sheet.getByRole('button', { name: 'Пригласить' }).click();

    // Тост «Доступ выдан» (2010-131859) на странице участника; грант
    // пришёл suspended — у сид-получателя нет подписки, тарифный слот
    // превышен (второй объект). Статус и пояснение — на уровне участника
    // (макет 2036-84861, правка приёмки #756): чип в шапке + жёлтая
    // карточка; ноги несут бейджи ролей. Тост закрываем кнопкой, чтобы
    // не перехватывал клики (канон contacts-спек).
    const grantedToast = page.locator('.Toastify__toast', { hasText: 'Доступ выдан' });
    await expect(grantedToast).toBeVisible();
    await grantedToast.getByRole('button', { name: 'Закрыть' }).click();
    await expect(page.getByText('Превышен лимит объектов').first()).toBeVisible();
    await expect(page.getByTestId('participant-limit-notice')).toBeVisible();
    await expect(
      page
        .getByTestId('participant-limit-notice')
        .getByText('Пользователь пока не может пользоваться вашим объектом'),
    ).toBeVisible();
    await expect(page.getByTestId('participant-limit-notice').getByText(/Попросите его освободить слот/)).toBeVisible();
    await expect(
      page.getByRole('button', { name: /Гараж на Садовой/ }).getByText('Управление'),
    ).toBeVisible();
    await expect(page.getByRole('button', { name: /Квартира на Ленина/ }).getByText('Управление')).toBeVisible();
  } finally {
    await execE2eSql(
      `DELETE FROM property_members WHERE property_id = '${GARAGE_ID}' AND user_id = '${LIMITED_ID}'`,
    );
  }
  await page.reload();
  // Сид восстановлен (нога гаража удалена) — агрегат снова partial, бейджа
  // лимита нет (D3 #1313: у partial бейджа нет).
  await expect(page.getByText('Превышен лимит объектов')).toHaveCount(0);
});

test('отзыв из объекта: подтверждение, тост успеха, ряд исчезает; сид восстанавливается', async ({ page, seededUser }) => {
  // Нога на гараже — временная, создаётся и убирается этим тестом.
  await execE2eSql(
    `INSERT INTO property_members (id, property_id, user_id, role, granted_by) ` +
      `VALUES ('99999999-9999-4999-8999-999999999933', '${GARAGE_ID}', '${SERGEY_ID}', 'viewer', '11111111-1111-4111-8111-111111111111') ` +
      `ON CONFLICT (id) DO NOTHING`,
  );

  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/participants/${SERGEY_ID}`);

  await page.getByRole('button', { name: /Гараж на Садовой/ }).click();
  await expect(screenHeader(page).getByText('Права участника')).toBeVisible();

  await page.getByRole('button', { name: 'Отозвать доступ к объекту' }).click();

  const dialog = page.getByRole('dialog');
  await expect(dialog.getByText('Уверены, что хотите отозвать доступ?')).toBeVisible();
  try {
    await dialog.getByRole('button', { name: 'Отозвать' }).click();

    // Тост (2008-83135) на странице участника; нога на гараже исчезла.
    // Тост закрываем кнопкой, чтобы не перехватывал клики.
    const toast = page.locator('.Toastify__toast', {
      hasText: 'У участника больше нет доступа к объекту',
    });
    await expect(toast).toBeVisible();
    await toast.getByRole('button', { name: 'Закрыть' }).click();
    await expect(page.getByRole('button', { name: /Гараж на Садовой/ })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /Квартира на Ленина/ })).toBeVisible();
  } finally {
    await execE2eSql(
      `DELETE FROM property_members WHERE id = '99999999-9999-4999-8999-999999999933'`,
    );
  }
});

test('кебаб «Отозвать доступ к объектам»: подтверждение, тост «Участник удален» на списке; сид восстанавливается', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  // Путь пользователя: раздел «Участники» → ряд Сергея (нужна история —
  // успех возврата по канону goBack ведёт именно на раздел).
  await page.goto('/participants');

  const header = screenHeader(page);

  await page.getByRole('button', { name: /Сергей Сидоров/ }).click();
  // Данные экрана сели до меню (заголовок шапки не матчим: у хаба-раздела
  // он рендерится дважды — inline и mobile-клон компакт-бара, strict
  // violation на substring «Участник»; возврат проверяет heading ниже).
  await expect(page.getByRole('heading', { name: 'Сергей Сидоров' })).toBeVisible();
  await expect(page.getByRole('button', { name: /Квартира на Ленина/ })).toBeVisible();

  // Radix-меню под нагрузкой полного прогона может не открыться с первого
  // клика — повторяем до появления пункта, затем выбираем его.
  await expect(async () => {
    await header.getByRole('button', { name: 'Еще — действия с участником' }).click();
    await expect(page.getByRole('menuitem', { name: 'Отозвать доступ к объектам' })).toBeVisible();
  }).toPass({ timeout: 10_000 });
  await page.getByRole('menuitem', { name: 'Отозвать доступ к объектам' }).click();

  const dialog = page.getByRole('dialog');
  await expect(
    dialog.getByText('Отозвать у пользователя доступ ко всем вашим объектам?'),
  ).toBeVisible();
  try {
    await dialog.getByRole('button', { name: 'Отозвать и удалить' }).click();

    // Возврат в раздел «Участники» с тостом «Участник удален»
    // (2008-83716; тост вместо успех-попапа, решение владельца
    // 19.10.2026); URL с якорем конца строки отличает раздел от страницы
    // участника. Тост закрываем кнопкой, чтобы не перехватывал клики.
    await expect(page).toHaveURL(/\/participants$/);
    const toast = page.locator('.Toastify__toast', { hasText: 'Участник удален' });
    await expect(toast).toBeVisible();
    await toast.getByRole('button', { name: 'Закрыть' }).click();
    await expect(page.getByRole('button', { name: /Сергей Сидоров/ })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /Мария Петрова/ })).toBeVisible();
  } finally {
    await execE2eSql(
      `INSERT INTO property_members (id, property_id, user_id, role, granted_by) ` +
        `VALUES ('99999999-9999-4999-8999-999999999932', '${APARTMENT_ID}', '${SERGEY_ID}', 'viewer', '11111111-1111-4111-8111-111111111111') ` +
        `ON CONFLICT (id) DO NOTHING`,
    );
  }
  await page.reload();
  await expect(page.getByRole('button', { name: /Сергей Сидоров/ })).toBeVisible();
});

test('pending-участник: deep-link по почте, бейдж «Аккаунт не создан», смена роли приглашения; сид восстанавливается', async ({ page, seededUser }) => {
  // Pending-приглашение на гараж (гейт тикета: состояния pending на тех же
  // экранах; права уходят в PATCH/DELETE /access/invitations/{id}).
  await execE2eSql(
    `INSERT INTO property_member_invitations (id, property_id, email, role, invited_by, last_sent_at) ` +
      `VALUES ('88888888-8888-4888-8888-888888888881', '${GARAGE_ID}', 'e2e-pending@example.com', 'viewer', '11111111-1111-4111-8111-111111111111', now()) ` +
      `ON CONFLICT (id) DO NOTHING`,
  );
  try {
    await openCabinetWithSeededSession(page, seededUser);
    // Идентификатор pending-участника — почта (контракт #693).
    await page.goto('/participants/e2e-pending%40example.com');

    const header = screenHeader(page);

    await expect(header.getByText('Участник', { exact: true }).first()).toBeVisible();
    // Имени нет — титул «Пользователь», почта строкой под ним (аменда
    // владельца 22.10.2026, макет 3313-77128: почта больше не титул,
    // #693 амендован); бейдж в шапке — «Аккаунт не создан» (D3 #1313: у
    // pending-агрегата ровно этот статус). У pending-ноги гаража свой чип
    // «Приглашён» (ассерт ниже) — он-role бейдж, не статусный.
    await expect(page.getByRole('heading', { name: 'Пользователь', exact: true })).toBeVisible();
    await expect(page.getByText('e2e-pending@example.com')).toBeVisible();
    await expect(page.getByText('Аккаунт не создан')).toBeVisible();

    // Чип шапки pending-участника: почта → 24 + 6 внутреннего паддинга
    // инстанса бейджа (#1402, 2036-84861; с амендой 22.10.2026 между
    // титулом и бейджем строка почты — зазор 8 по gap-2 блока,
    // меряется от неё).
    const emailBox = await page.getByText('e2e-pending@example.com').boundingBox();
    const chipBox = await page.getByText('Аккаунт не создан').boundingBox();
    if (!emailBox || !chipBox) {
      throw new Error('боксы почты и чипа не измерились');
    }
    expect(Math.round(chipBox.y - emailBox.y - emailBox.height)).toBe(30);

    const garageRow = page.getByRole('button', { name: /Гараж на Садовой/ });
    await expect(garageRow.getByText('Приглашён')).toBeVisible();

    await garageRow.click();
    await expect(header.getByText('Права участника')).toBeVisible();
    await expect(page.getByRole('radio', { name: 'Просмотр' })).toBeChecked();

    await page.getByRole('radio', { name: 'Управление' }).click();
    const rightsToast = page.locator('.Toastify__toast', { hasText: 'Права изменены' });
    await expect(rightsToast).toBeVisible();
    await rightsToast.getByRole('button', { name: 'Закрыть' }).click();

    // Серверная правда: роль приглашения реально сменилась.
    expect(
      await execE2eSql(
        `SELECT count(*) FROM property_member_invitations WHERE id = '88888888-8888-4888-8888-888888888881' AND role = 'full_access'`,
      ),
    ).toBe('1');
  } finally {
    await execE2eSql(
      `DELETE FROM property_member_invitations WHERE id = '88888888-8888-4888-8888-888888888881'`,
    );
  }
});

// D7 #1314 (макет 3298-75576): «Права участника» для приглашённого без
// аккаунта. Сид — собственное pending-приглашение на квартиру (id не из
// других спек: утечка строки одного теста не подменяет данные другого).
const PENDING_RIGHTS_INVITE_ID = '99999999-9999-4999-8999-999999999973';
const PENDING_RIGHTS_EMAIL = 'e2e-pending-rights@example.com';

function seedPendingRightsInvitation(role: 'viewer' | 'full_access'): Promise<string> {
  return execE2eSql(
    `INSERT INTO property_member_invitations (id, property_id, email, role, invited_by, last_sent_at) ` +
      `VALUES ('${PENDING_RIGHTS_INVITE_ID}', '${APARTMENT_ID}', '${PENDING_RIGHTS_EMAIL}', '${role}', '11111111-1111-4111-8111-111111111111', now()) ` +
      `ON CONFLICT (id) DO UPDATE SET role = EXCLUDED.role, property_id = EXCLUDED.property_id, email = EXCLUDED.email`,
  );
}

test('права участника без аккаунта (D7): карточка, инфо-блок, сегмент, «Отозвать доступ»; скриншот', async ({ page, seededUser }, testInfo) => {
  await seedPendingRightsInvitation('viewer');
  try {
    await openCabinetWithSeededSession(page, seededUser);
    // Вход — из страницы участника: pending-нога «Приглашён» → права.
    await page.goto('/participants/e2e-pending-rights%40example.com');
    const legRow = page.getByRole('button', { name: /Квартира на Ленина/ });
    await expect(legRow.getByText('Приглашён')).toBeVisible();
    await legRow.click();

    const header = screenHeader(page);
    await expect(header.getByText('Права участника')).toBeVisible();

    // Карточка участника (3298-75576): анонимный титул #1105, почта,
    // бейдж «Аккаунт не создан»; контекста объекта на экране нет.
    const card = page.getByTestId('participant-rights-card');
    await expect(card.getByText('Пользователь', { exact: true })).toBeVisible();
    await expect(card.getByText(PENDING_RIGHTS_EMAIL)).toBeVisible();
    await expect(card.getByText('Аккаунт не создан')).toBeVisible();
    await expect(page.getByText('Квартира на Ленина')).toHaveCount(0);

    // Инфо-блок: кто это и что будет после создания аккаунта.
    const notice = page.getByTestId('participant-pending-notice');
    await expect(notice.getByText('Пользователь еще не создал аккаунт')).toBeVisible();
    await expect(
      notice.getByText('После создания аккаунта он сможет пользоваться вашим объектом'),
    ).toBeVisible();

    // Сегмент роли и красная строка отзыва; строки «Действия участника
    // в объекте» у pending нет — журнал адресуется uuid юзера (actor_id),
    // которого у приглашённого ещё не существует.
    await expect(page.getByRole('radio', { name: 'Просмотр' })).toBeChecked();
    await expect(page.getByTestId('participant-rights-revoke')).toBeVisible();
    await expect(page.getByText('Действия участника')).toHaveCount(0);

    await captureScreen(page, testInfo, 'participant-rights-pending');
  } finally {
    await execE2eSql(`DELETE FROM property_member_invitations WHERE id = '${PENDING_RIGHTS_INVITE_ID}'`);
  }
});

test('отзыв pending-приглашения с D7-экрана: подтверждение, тост, приглашение снято; сид восстанавливается', async ({ page, seededUser }) => {
  await seedPendingRightsInvitation('viewer');
  try {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/participants/e2e-pending-rights%40example.com');
    await page.getByRole('button', { name: /Квартира на Ленина/ }).click();
    await expect(screenHeader(page).getByText('Права участника')).toBeVisible();

    await page.getByTestId('participant-rights-revoke').click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText('Уверены, что хотите отозвать доступ?')).toBeVisible();
    await dialog.getByRole('button', { name: 'Отозвать' }).click();

    // Тост (2008-83135) на странице участника; единственная нога снята —
    // агрегат честно отвечает «Участник не найден» (приватный 404 #693).
    // Тост закрываем кнопкой, чтобы не перехватывал клики.
    const toast = page.locator('.Toastify__toast', {
      hasText: 'У участника больше нет доступа к объекту',
    });
    await expect(toast).toBeVisible();
    await toast.getByRole('button', { name: 'Закрыть' }).click();
    await expect(page.getByText('Участник не найден')).toBeVisible();

    // Серверная правда: приглашение отменено.
    expect(
      await execE2eSql(
        `SELECT count(*) FROM property_member_invitations WHERE id = '${PENDING_RIGHTS_INVITE_ID}'`,
      ),
    ).toBe('0');
  } finally {
    await seedPendingRightsInvitation('viewer');
  }
  await page.reload();
  await expect(page.getByRole('button', { name: /Квартира на Ленина/ })).toBeVisible();
});

test('D7 owner-only: manage-участник по deep-link видит «Участник не найден» — мутаций нет', async ({ page }) => {
  // Мария (full_access на чужой квартире) открывает D7-экран напрямую:
  // агрегат GET /participants/{id} — owner-скоуп (D5 #1304), не-собственнику
  // приватный 404; зрителя проверяет та же ветка (404 не зависит от роли).
  // #1362 открыл участнику страницы прав по member-маршруту
  // /properties/{id}/participants/{memberId} (контракт чтения строки
  // #1357, спек participant-rights-member.spec.ts) — агрегатный маршрут
  // владельческого экрана остаётся owner-only, этот лок стоит.
  await seedPendingRightsInvitation('viewer');
  try {
    await openCabinetWithSessionToken(page, seededMemberSessionToken());
    await page.goto(`/participants/e2e-pending-rights%40example.com/properties/${APARTMENT_ID}`);

    await expect(page.getByText('Участник не найден')).toBeVisible();
    await expect(page.getByRole('radio', { name: 'Просмотр' })).toHaveCount(0);
    await expect(page.getByRole('radio', { name: 'Управление' })).toHaveCount(0);
    await expect(page.getByTestId('participant-rights-revoke')).toHaveCount(0);
    // Назад в шапке жив (§7).
    await expect(screenHeader(page).getByRole('button', { name: 'Назад' })).toBeVisible();
  } finally {
    await execE2eSql(`DELETE FROM property_member_invitations WHERE id = '${PENDING_RIGHTS_INVITE_ID}'`);
  }
});
