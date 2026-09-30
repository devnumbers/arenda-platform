import { MONTH_SHORT } from '@/shared/lib/date-format';
import type { Page } from '@playwright/test';
import {
  captureScreen,
  execE2eSql,
  expect,
  memberTaskEntry,
  openCabinetWithSeededSession,
  paymentCreatedEntry,
  SEEDED_GARAGE_PROPERTY_ID,
  seedJournalEntry,
  test,
  todayAt,
  type SeededUser,
} from './fixtures';

// Шит «Настройки» — фильтры ленты «История действий» (#711, макеты
// 2177-60527 / 2067-163528 / 2067-162950 / 2050-158280): группы-чекбоксы
// со счётчиками N/M, период через канон CalendarRangePicker, «Сбросить» /
// «Применить фильтры». Состояние живёт в адресе ленты (?from=&to=&actions=
// &kinds=&actors=&objects=): применение — push, «назад» возвращает без
// фильтров. Семантика группы: все — параметра нет, выбор — CSV, ни одного —
// пустое значение и пустой результат без запроса.

// Названия месяцев в заголовках секций пикера периода («Сентябрь, 2026»).
const PICKER_MONTH_LABELS = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
] as const;

/** Четыре записи «сегодня» под фильтры Истории. */
async function seedHistory(user: SeededUser): Promise<void> {
  await execE2eSql('DELETE FROM action_journal;');
  await seedJournalEntry(paymentCreatedEntry('b0000000-0000-4000-8000-000000000001', 4, 'Аренда за сентябрь'), user);
  await seedJournalEntry(
    {
      id: 'b0000000-0000-4000-8000-000000000002',
      createdAtSql: todayAt(3),
      kind: 'operation',
      action: 'operation.paid',
      baseAction: 'completed',
      text: 'Операция оплачена: Вода',
    },
    user,
  );
  await seedJournalEntry(memberTaskEntry('b0000000-0000-4000-8000-000000000003', 2), user);
  await seedJournalEntry(
    {
      id: 'b0000000-0000-4000-8000-000000000004',
      createdAtSql: todayAt(1),
      propertyId: SEEDED_GARAGE_PROPERTY_ID,
      kind: 'property',
      action: 'property.pinned',
      baseAction: 'changed',
      text: 'Гараж на Садовой закреплён',
    },
    user,
  );
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

  // Отступ чекбокса шапки от края карточки — 20 (макет 2177-60528;
  // регресс-гард: margin на кнопке чекбокса съедает preflight, #753).
  const masterInset = await page.evaluate(() => {
    const card = document.querySelector('[aria-label="Фильтры истории"] section');
    const box = card?.querySelector('button[role="checkbox"]')?.getBoundingClientRect();
    if (!card || !box) {
      return -1;
    }
    return Math.round(box.left - card.getBoundingClientRect().left);
  });
  expect(masterInset).toBe(20);

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

  // Мастер-чекбокс — переключатель (решение владельца 23.09): из «все»
  // снимает всю группу, повторный тап возвращает «все».
  await dialog.getByRole('checkbox', { name: 'Выбрать все: Основные действия' }).click();
  await expect(dialog.getByRole('checkbox', { name: 'Выбрать все: Основные действия' })).toHaveAttribute('data-state', 'unchecked');
  await expect(dialog.getByText('0/4')).toBeVisible();
  for (const label of ['Добавление', 'Изменение', 'Выполнение', 'Удаление']) {
    await expect(dialog.getByRole('checkbox', { name: label })).toHaveAttribute('data-state', 'unchecked');
  }
  await dialog.getByRole('checkbox', { name: 'Выбрать все: Основные действия' }).click();
  await expect(dialog.getByRole('checkbox', { name: 'Выбрать все: Основные действия' })).toHaveAttribute('data-state', 'checked');
  await expect(dialog.getByText('4/4')).toBeVisible();
  await expect(dialog.getByRole('checkbox', { name: 'Добавление' })).toHaveAttribute('data-state', 'checked');

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

  // Цвет текста невыбранной опции не меняется (макет 2050-158281:
  // content #171a1c, не tertiary).
  const uncheckedColor = await dialog
    .getByText('Добавление', { exact: true })
    .evaluate((el) => getComputedStyle(el).color);
  expect(uncheckedColor).toBe('rgb(23, 26, 28)');

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

  // Вся строка кликабельна: тап по тексту переключает опцию (макет
  // 2050-158281), не только чекбокс.
  await sheet(page).getByText('Добавление', { exact: true }).click();
  await expect(sheet(page).getByRole('checkbox', { name: 'Добавление' })).toHaveAttribute('data-state', 'checked');
  await expect(sheet(page).getByText('4/4')).toBeVisible();
  await sheet(page).getByText('Добавление', { exact: true }).click();
  await expect(sheet(page).getByRole('checkbox', { name: 'Добавление' })).toHaveAttribute('data-state', 'unchecked');
  await expect(sheet(page).getByText('3/4')).toBeVisible();
});

test('период «сегодня» через CalendarRangePicker: синий чип, дальняя запись уходит из ленты', async ({ page, seededUser }, testInfo) => {
  await seedHistory(seededUser);
  // Запись 12 дней назад — за пределами периода «сегодня».
  await seedJournalEntry(
    {
      id: 'b0000000-0000-4000-8000-000000000005',
      createdAtSql: "(now() - interval '12 days')",
      kind: 'payment',
      action: 'payment.deleted',
      baseAction: 'deleted',
      text: 'Платёж удалён: Старый платёж',
    },
    seededUser,
  );
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
  const monthLabel = `${PICKER_MONTH_LABELS[now.getMonth()]}, ${now.getFullYear()}`;
  const monthSection = page.locator('section').filter({ has: page.getByRole('heading', { name: monthLabel }) });
  // «Сегодня» читаем из маркера aria-current — как в однодатных пикерах
  // (прецедент payment-edit-delete), а не из календарного числа.
  const todayButton = monthSection.locator('button[aria-current="date"]');
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

test('участники: своя строка «Иван (Вы)» с замком владельца, чужая — иконка роли (макеты 2067-163528, 2184-94261)', async ({ page, seededUser }) => {
  await openHistoryWithFeed(page, seededUser);

  await page.getByRole('button', { name: 'Настройки' }).click();
  const dialog = sheet(page);
  await dialog.getByRole('button', { name: 'Участники' }).click();

  // Своя строка: имя без фамилии + серый суффикс «(Вы)»; владелец
  // объектов области — замок перед почтой (#711).
  const ownRow = page
    .locator('.min-h-14')
    .filter({ has: page.getByRole('checkbox', { name: 'Иван (Вы)' }) });
  await expect(ownRow).toHaveCount(1);
  await expect(ownRow.getByText('(Вы)', { exact: true })).toBeVisible();
  await expect(ownRow.getByRole('img', { name: 'Роль: Владелец' })).toHaveCount(1);

  // Приглашённая без своих объектов: канон имени + иконка роли
  // full_access (#840, макет 2184-94261).
  const memberRow = page
    .locator('.min-h-14')
    .filter({ has: page.getByRole('checkbox', { name: 'Мария Петрова' }) });
  await expect(memberRow).toHaveCount(1);
  await expect(memberRow.getByRole('img', { name: 'Роль: Редактирование' })).toHaveCount(1);
  await expect(memberRow.getByRole('img', { name: 'Роль: Владелец' })).toHaveCount(0);
});

test('прямая ссылка с фильтром (?kinds=task) открывает отфильтрованную ленту', async ({ page, seededUser }) => {
  await seedHistory(seededUser);
  await openCabinetWithSeededSession(page, seededUser);

  await page.goto('/history?kinds=task');
  await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();
  await expect(page.getByText('Платёж создан: Аренда за сентябрь')).toHaveCount(0);
  await expect(page.getByText('Операция оплачена: Вода')).toHaveCount(0);
});

// Кейс «опции не загрузились: ErrorCard с „Повторить"» (бывший тест ниже)
// снят: с серверным префетчем опций (#887, history/page.tsx — «опции шита
// фильтров … в первом кадре всегда») клиентский запрос /history/filters при
// открытом шите не случается вовсе — опции приходят в hydration-состоянии,
// так что перехват сети с 500 больше не приводит запрос в ошибку, и ветка
// ErrorCard в e2e недостижима. Ветка остаётся страховкой на провал
// серверного префетча; его нельзя уронить рычагами e2e (page.route не
// видит серверный fetch, тарифный гейт блокирует саму страницу).

