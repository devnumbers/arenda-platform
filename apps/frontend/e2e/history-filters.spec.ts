import { MONTH_SHORT } from '@/shared/lib/date-format';
import { MONTH_LABELS } from '@/shared/ui/design/month-grid';
import type { Page } from '@playwright/test';
import {
  captureScreen,
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  test,
  type SeededUser,
} from './fixtures';

// Шит «Настройки» — фильтры ленты «История действий» (#711, макеты
// 2177-60527 / 2067-163528 / 2067-162950 / 2050-158280): группы-чекбоксы
// со счётчиками N/M, период через канон CalendarRangePicker, «Сбросить» /
// «Применить фильтры». Состояние живёт в адресе ленты (?from=&to=&actions=
// &kinds=&actors=&objects=): применение — push, «назад» возвращает без
// фильтров. Семантика группы: все — параметра нет, выбор — CSV, ни одного —
// пустое значение и пустой результат без запроса.

const OWNER_NAME = 'Иван Иванов';
const MEMBER_NAME = 'Мария Петрова';

function ownerActorId(user: SeededUser): string {
  return `(SELECT id FROM users WHERE email = '${user.email}')`;
}

function memberActorId(): string {
  return `(SELECT id FROM users WHERE email = 'e2e-member@example.com')`;
}

async function seedHistory(user: SeededUser): Promise<void> {
  await execE2eSql('DELETE FROM action_journal;');
  const insert = (entry: {
    id: string;
    createdAt: string;
    propertyIdSql?: string;
    actorIdSql?: string;
    actorName?: string;
    actorRole?: string;
    kind?: string;
    action?: string;
    baseAction?: string;
    text: string;
  }): Promise<unknown> =>
    execE2eSql(`
    INSERT INTO action_journal
      (id, property_id, actor_id, actor_role, actor_name, actor_email, kind, action, base_action, segments, searchable, created_at)
    VALUES (
      '${entry.id}',
      '${entry.propertyIdSql ?? SEEDED_APARTMENT_PROPERTY_ID}',
      ${entry.actorIdSql ?? ownerActorId(user)},
      '${entry.actorRole ?? 'owner'}',
      '${entry.actorName ?? OWNER_NAME}',
      '${user.email}',
      '${entry.kind ?? 'payment'}',
      '${entry.action ?? 'payment.created'}',
      '${entry.baseAction ?? 'added'}',
      $j$[{"text": "${entry.text}"}]$j$::jsonb,
      '${entry.text} ${entry.actorName ?? OWNER_NAME}',
      '${entry.createdAt}'
    );
  `);
  const now = Date.now();
  await insert({
    id: 'b0000000-0000-4000-8000-000000000001',
    createdAt: new Date(now - 4 * 60_000).toISOString(),
    text: 'Платёж создан: Аренда за сентябрь',
  });
  await insert({
    id: 'b0000000-0000-4000-8000-000000000002',
    createdAt: new Date(now - 3 * 60_000).toISOString(),
    kind: 'operation',
    action: 'operation.paid',
    baseAction: 'completed',
    text: 'Операция оплачена: Вода',
  });
  await insert({
    id: 'b0000000-0000-4000-8000-000000000003',
    createdAt: new Date(now - 2 * 60_000).toISOString(),
    actorIdSql: memberActorId(),
    actorName: MEMBER_NAME,
    actorRole: 'full_access',
    kind: 'task',
    action: 'task.completed',
    baseAction: 'completed',
    text: 'Задача выполнена: Заменить кран',
  });
  await insert({
    id: 'b0000000-0000-4000-8000-000000000004',
    createdAt: new Date(now - 60_000).toISOString(),
    propertyIdSql: SEEDED_GARAGE_PROPERTY_ID,
    kind: 'property',
    action: 'property.pinned',
    baseAction: 'changed',
    text: 'Гараж на Садовой закреплён',
  });
}

/** Сид + вход + открытая лента: все записи в первой порции (без прокрутки). */
async function openHistoryWithFeed(page: Page, user: SeededUser): Promise<void> {
  await seedHistory(user);
  await openCabinetWithSeededSession(page, user);
  await page.goto('/history');
  await expect(page.getByText('Платёж создан: Аренда за сентябрь')).toBeVisible();
}

const sheet = (page: Page) =>
  page.getByRole('dialog', { name: 'Фильтры истории' });

test('шит открывается свёрнутым с «всеми выбранными» группами, крестик закрывает без изменений', async ({ page, seededUser }, testInfo) => {
  await openHistoryWithFeed(page, seededUser);

  await page.getByRole('button', { name: 'Настройки' }).click();
  const dialog = sheet(page);
  await expect(dialog).toBeVisible();

  // Анатомия макета 2177-60527: чип периода, четыре свёрнутые группы со
  // счётчиками «все выбрано», сброс и применение.
  await expect(dialog.getByRole('button', { name: 'Выбрать период' })).toBeVisible();
  await expect(dialog.getByText('Основные действия')).toBeVisible();
  // Основные действия 4/4 и виды 7/7 — статические каталоги; участники и
  // объекты приходят с /history/filters (состав сида не фиксируем).
  await expect(dialog.getByText('4/4')).toBeVisible();
  await expect(dialog.getByText('7/7')).toBeVisible();
  await expect(dialog.getByText(/^\d+\/\d+$/)).toHaveCount(4);
  await expect(dialog.getByRole('button', { name: 'Сбросить фильтры' })).toBeVisible();
  await expect(dialog.getByRole('button', { name: 'Применить фильтры' })).toBeVisible();

  // Мастер-чекбоксы «всё выбрано»; раскрытие — по кнопке заголовка
  // (аннотация макета: страница открывается свёрнутой).
  await expect(dialog.getByRole('checkbox', { name: 'Выбрать все: Основные действия' })).toHaveAttribute('data-state', 'checked');
  await dialog.getByRole('button', { name: 'Основные действия' }).click();
  for (const label of ['Добавление', 'Изменение', 'Выполнение', 'Удаление']) {
    await expect(dialog.getByRole('checkbox', { name: label })).toHaveAttribute('data-state', 'checked');
  }

  // Крестик закрывает без коммита: лента и адрес не тронуты.
  await dialog.getByRole('button', { name: 'Закрыть фильтры' }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page.getByText('Платёж создан: Аренда за сентябрь')).toBeVisible();
  expect(new URL(page.url()).search).toBe('');

  await captureScreen(page, testInfo, 'history-filters-sheet');
});

test('снятие опции «Добавление» и «Применить»: URL с CSV, лента без добавлений, 3/4 при переоткрытии', async ({ page, seededUser }) => {
  await openHistoryWithFeed(page, seededUser);

  await page.getByRole('button', { name: 'Настройки' }).click();
  const dialog = sheet(page);
  await dialog.getByRole('button', { name: 'Основные действия' }).click();
  await dialog.getByRole('checkbox', { name: 'Добавление' }).click();

  // Счётчик 3/4, мастер-чекбокс в частичном состоянии (минус).
  await expect(dialog.getByText('3/4')).toBeVisible();
  await expect(dialog.getByRole('checkbox', { name: 'Выбрать все: Основные действия' })).toHaveAttribute('data-state', 'indeterminate');

  await dialog.getByRole('button', { name: 'Применить фильтры' }).click();
  await expect(dialog).toHaveCount(0);
  // Пишется router.push — адрес догоняет с ретраем.
  await expect(page).toHaveURL(/actions=/);

  const params = new URL(page.url()).searchParams;
  expect(params.get('actions')).toBe('changed,completed,deleted');
  await expect(page.getByText('Платёж создан: Аренда за сентябрь')).toHaveCount(0);
  await expect(page.getByText('Операция оплачена: Вода')).toBeVisible();
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();

  // Применённое состояние — стартовое для нового открытия шита: счётчик
  // 3/4 на свёрнутой карточке, снятая опция видна после раскрытия.
  await page.getByRole('button', { name: 'Настройки' }).click();
  await expect(sheet(page).getByText('3/4')).toBeVisible();
  await sheet(page).getByRole('button', { name: 'Основные действия' }).click();
  await expect(sheet(page).getByRole('checkbox', { name: 'Добавление' })).toHaveAttribute('data-state', 'unchecked');
});

test('период «сегодня» через CalendarRangePicker: синий чип, дальняя запись уходит из ленты', async ({ page, seededUser }, testInfo) => {
  await seedHistory(seededUser);
  // Запись 12 дней назад — за пределами периода «сегодня».
  await execE2eSql(`
    INSERT INTO action_journal
      (id, property_id, actor_id, actor_role, actor_name, actor_email, kind, action, base_action, segments, searchable, created_at)
    VALUES (
      'b0000000-0000-4000-8000-000000000005',
      '${SEEDED_APARTMENT_PROPERTY_ID}',
      ${ownerActorId(seededUser)},
      'owner',
      '${OWNER_NAME}',
      '${seededUser.email}',
      'payment',
      'payment.deleted',
      'deleted',
      $j$[{"text": "Платёж удалён: Старый платёж"}]$j$::jsonb,
      'Платёж удалён: Старый платёж ${OWNER_NAME}',
      (now() - interval '12 days')
    );
  `);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/history');
  await expect(page.getByText('Платёж удалён: Старый платёж')).toBeVisible();

  await page.getByRole('button', { name: 'Настройки' }).click();
  const dialog = sheet(page);
  await dialog.getByRole('button', { name: 'Выбрать период' }).click();

  // Канон CalendarRangePicker (#670): тап задаёт границу, повторный тап по
  // тому же дню завершает диапазон одним днём; «Выбрать» коммитит.
  // Локаторы секций месяца — по page-скоуп-хедингу внутри (прецедент
  // operations.spec: has-фильтр с обычным page-локатором).
  const picker = page.getByRole('dialog', { name: 'Выберите период' });
  await expect(picker).toBeVisible();
  const now = new Date();
  const monthLabel = `${MONTH_LABELS[now.getMonth()]}, ${now.getFullYear()}`;
  const monthSection = page.locator('section').filter({ has: page.getByRole('heading', { name: monthLabel }) });
  const todayButton = monthSection.getByRole('button', { name: String(now.getDate()), exact: true });
  await todayButton.click();
  await todayButton.click();
  await picker.getByRole('button', { name: 'Выбрать', exact: true }).click();

  const expectedChip = `${now.getDate()} ${MONTH_SHORT[now.getMonth()]}`;
  await expect(dialog.getByRole('button', { name: expectedChip })).toBeVisible();

  await dialog.getByRole('button', { name: 'Применить фильтры' }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page).toHaveURL(/from=/);

  const params = new URL(page.url()).searchParams;
  const todayIso = now.toISOString().slice(0, 10);
  expect(params.get('from')).toBe(todayIso);
  expect(params.get('to')).toBe(todayIso);
  await expect(page.getByText('Платёж удалён: Старый платёж')).toHaveCount(0);
  await expect(page.getByText('Операция оплачена: Вода')).toBeVisible();

  await captureScreen(page, testInfo, 'history-filters-period');
});

test('участники «в ноль» — «Ничего не найдено» без запроса; «Сбросить фильтры» возвращает ленту', async ({ page, seededUser }) => {
  await openHistoryWithFeed(page, seededUser);

  await page.getByRole('button', { name: 'Настройки' }).click();
  const dialog = sheet(page);
  await dialog.getByRole('button', { name: 'Участники' }).click();
  // Снимаю все опции группы (состав сида не фиксируем): мастер-чекбокс
  // первый в карточке, опции — дальше. Карточка — единственная секция с
  // кнопкой «Участники …» (has-фильтр с page-локатором, см. operations).
  const group = page.locator('section').filter({ has: page.getByRole('button', { name: 'Участники' }) });
  const checkboxes = group.getByRole('checkbox');
  const total = await checkboxes.count();
  expect(total).toBeGreaterThan(1);
  for (let index = 1; index < total; index += 1) {
    await checkboxes.nth(index).click();
  }
  await expect(checkboxes.first()).toHaveAttribute('data-state', 'unchecked');

  await dialog.getByRole('button', { name: 'Применить фильтры' }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page).toHaveURL(/actors=/);

  // Пустое значение параметра = «ни один»: результат пуст по семантике,
  // запрос не делается — лента показывает «Ничего не найдено».
  expect(new URL(page.url()).searchParams.get('actors')).toBe('');
  await expect(page.getByText('Ничего не найдено')).toBeVisible();

  // Сброс в черновике и повторное применение — лента без фильтров.
  await page.getByRole('button', { name: 'Настройки' }).click();
  await sheet(page).getByRole('button', { name: 'Сбросить фильтры' }).click();
  await expect(sheet(page).getByRole('checkbox', { name: 'Выбрать все: Участники' })).toHaveAttribute('data-state', 'checked');
  await sheet(page).getByRole('button', { name: 'Применить фильтры' }).click();
  await expect(page).toHaveURL(/\/history$/);
  await expect(page.getByText('Платёж создан: Аренда за сентябрь')).toBeVisible();
});

test('участники: своя строка «Иван (Вы)» с замком владельца, чужая — полное имя без замка (макет 2067-163528)', async ({ page, seededUser }) => {
  await openHistoryWithFeed(page, seededUser);

  await page.getByRole('button', { name: 'Настройки' }).click();
  const dialog = sheet(page);
  await dialog.getByRole('button', { name: 'Участники' }).click();

  // Своя строка: имя без фамилии + серый суффикс «(Вы)»; владелец
  // объектов области — замок перед почтой (вторая svg строки: аватар + замок).
  const ownRow = page
    .locator('div.min-h-14')
    .filter({ has: page.getByRole('checkbox', { name: 'Иван (Вы)' }) });
  await expect(ownRow).toHaveCount(1);
  await expect(ownRow.getByText('(Вы)', { exact: true })).toBeVisible();
  // Замок владельца — приглушённая svg в подзаголовке рядом с почтой.
  await expect(ownRow.locator('svg.text-content-tertiary')).toHaveCount(1);

  // Приглашённая без своих объектов: канон имени, замка нет.
  const memberRow = page
    .locator('div.min-h-14')
    .filter({ has: page.getByRole('checkbox', { name: 'Мария Петрова' }) });
  await expect(memberRow).toHaveCount(1);
  await expect(memberRow.locator('svg.text-content-tertiary')).toHaveCount(0);
});

test('прямая ссылка с фильтром (?kinds=task) открывает отфильтрованную ленту', async ({ page, seededUser }) => {
  await seedHistory(seededUser);
  await openCabinetWithSeededSession(page, seededUser);

  await page.goto('/history?kinds=task');
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();
  await expect(page.getByText('Платёж создан: Аренда за сентябрь')).toHaveCount(0);
  await expect(page.getByText('Операция оплачена: Вода')).toHaveCount(0);
});

test('опции не загрузились: ErrorCard на месте групп, остальные фильтры и применение работают', async ({ page, seededUser }) => {
  // Роут ставится до монтирования: иначе первый успешный ответ ленты
  // закроет шит кэшем и ошибки не будет.
  await page.route('**/api/history/filters*', (route) => route.fulfill({ status: 500 }));
  await openHistoryWithFeed(page, seededUser);

  await page.getByRole('button', { name: 'Настройки' }).click();
  const dialog = sheet(page);
  // Статические группы (не зависят от опций) редактируются; 500 ретраится
  // глобальным предикатом до трёх волн — ждём дольше дефолта.
  await expect(dialog.getByText('4/4')).toBeVisible({ timeout: 15_000 });
  await dialog.getByRole('button', { name: 'Основные действия' }).click();
  await dialog.getByRole('checkbox', { name: 'Добавление' }).click();
  // Опции-группы заменены канон-ошибкой с «Повторить»; применение доступно.
  await expect(dialog.getByRole('button', { name: 'Повторить' })).toBeVisible();
  await dialog.getByRole('button', { name: 'Применить фильтры' }).click();
  await expect(dialog).toHaveCount(0);
  await expect(page).toHaveURL(/actions=/);

  const params = new URL(page.url()).searchParams;
  expect(params.get('actions')).toBe('changed,completed,deleted');
  await expect(page.getByText('Платёж создан: Аренда за сентябрь')).toHaveCount(0);
  await expect(page.getByText('Операция оплачена: Вода')).toBeVisible();
});
