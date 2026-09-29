import type { Locator, Page } from '@playwright/test';
import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  screenHeader,
  test,
} from './fixtures';

// Дерево профиля на едином хроме (карта #556, тикет #566): все /profile*
// переехали из (cabinet) в (screens) без смены URL. Подэкраны — TopNav с
// ведущим «Назад» (history-first goBack, фолбэк — родительская страница).
// /profile/notifications с #746 — саб-экран «Настроить уведомления»,
// раздел «Уведомления» он подсвечивает по нав-модели #558.

/** Подэкраны дерева: путь → заголовок в шапке. /profile — хаб (без «Назад»,
 * крылья), в таблицу не входит. /profile/tariff/payments/[id] не входит —
 * требует id платежа, канон его шапки совпадает со списком.
 * /profile/personal снесён в #593 — поглощён экраном /profile/account.
 * /profile/tariff/change/success не входит — канонный полноэкранный успех
 * (#623) без SubScreenShell. Info-маршруты в таблицу не входят: после
 * редизайна ad04300b их канон другой — в баре только «Назад», тайтла нет
 * (INFO_ROUTES ниже). */
const SUBPAGE_TITLES: ReadonlyArray<readonly [string, string]> = [
  ['/profile/account', 'Аккаунт'],
  ['/profile/account/phone', 'Изменение телефона'],
  ['/profile/devices', 'Устройства'],
  ['/profile/notifications', 'Настроить уведомления'],
  ['/profile/tariff', 'Тариф'],
  ['/profile/tariff/about', 'О тарифе'],
  ['/profile/tariff/disable', 'Отключение тарифа'],
  ['/profile/tariff/change', 'Выбрать тариф'],
  ['/profile/tariff/payment-methods', 'Способы оплаты'],
  ['/profile/tariff/payment-methods/add', 'Добавить карту'],
  ['/profile/tariff/payments', 'Операции'],
];

/** Info-группа (редизайн ad04300b): в баре только «Назад», заголовок живёт
 * в контенте — у privacy/terms это крупный h1 (LegalDocument), у самой
 * /profile/info h1 нет вовсе: смысл экрана держат бренд-локап и nav
 * «Правовая информация» строк-документов. */
const INFO_ROUTES: ReadonlyArray<readonly [string, (page: Page) => Locator]> = [
  ['/profile/info', (page) => page.getByRole('navigation', { name: 'Правовая информация' })],
  [
    '/profile/info/privacy',
    (page) => page.getByRole('heading', { name: 'Политика обработки персональных данных' }),
  ],
  [
    '/profile/info/terms',
    (page) => page.getByRole('heading', { name: 'Пользовательское соглашение' }),
  ],
];

const BOTTOM_NAV = 'nav[aria-label="Нижняя навигация"]';
// Сид-юзер «Иван Иванов» — имя в крыле после загрузки useMe; в pending
// кнопка — скелетон с aria-label «Профиль» (аудит #876), «Пользователь» —
// текстовый плейсхолдер вне провайдера.
const USER_WING = /Пользователь|Иван|Профиль/;

test.describe('дерево профиля — хром #566', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('экран «Настроить уведомления»: саб-экран с «Назад», мастер и матрица', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/notifications');

    // Саб-экран #746 (макеты 1789-100250/2329-150165): ведущий «Назад»,
    // заголовок в шапке; крыльев нет — экран дерева профиля.
    const header = screenHeader(page);
    await expect(header.getByRole('button', { name: 'Назад' })).toBeVisible();
    // Фильтр visible — Next держит в body скрытый клон дерева,
    // текстовые локаторы без него ресолвят обе копии шапки.
    await expect(
      header.getByText('Настроить уведомления', { exact: true }).filter({ visible: true }),
    ).toBeVisible();

    // Матрица: мастер-тумблер и четыре группы × два канала. Фильтр visible —
    // Next держит в body скрытый клон дерева (div#S:0[hidden]), текстовые
    // локаторы без него ресолвят обе копии.
    await expect(
      page.getByText('Получать пуш-уведомления', { exact: true }).filter({ visible: true }),
    ).toBeVisible();
    for (const group of ['Аренда', 'Платежи и операции', 'Задачи', 'Совместный доступ']) {
      await expect(
        page.getByRole('heading', { level: 2, name: group }).filter({ visible: true }),
      ).toBeVisible();
    }
    await expect(
      page.getByText('Электронная почта', { exact: true }).filter({ visible: true }),
    ).toHaveCount(4);
    await expect(
      page.getByText('Пуш-уведомления', { exact: true }).filter({ visible: true }),
    ).toHaveCount(4);

    // Активность по нав-модели: раздел «Уведомлений» — средний таб TabBar.
    await expect(
      page.locator(BOTTOM_NAV).getByRole('link', { name: 'Уведомления' }),
    ).toHaveAttribute('aria-current', 'page');

    await captureScreen(page, testInfo, 'profile-notifications-settings-mobile');
  });

  test('дерево: хаб без «Назад», строка «Аккаунт» вглубь, «Назад» по истории', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile');

    // Хаб профиля (#592): крылья на мобайле, ведущего «Назад» нет.
    const header = screenHeader(page);
    await expect(header.getByRole('button', { name: 'Назад' })).toHaveCount(0);
    await expect(header.getByRole('link', { name: 'Объекты' })).toBeVisible();
    await expect(header.getByText('Профиль', { exact: true })).toBeVisible();

    // Вглубь дерева строкой «Аккаунт» (#593); «Назад» возвращается по истории.
    await page.getByRole('button', { name: 'Аккаунт' }).click();
    await expect(page).toHaveURL(/\/profile\/account$/);
    await expect(header.getByText('Аккаунт', { exact: true })).toBeVisible();
    await header.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(/\/profile$/);
  });

  test('хаб: строка «Устройства» ведёт в дерево устройств (#724, #729)', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile');

    // Строка из мока хаба (Figma 1786-31288, между «Уведомления» и
    // «Информация»); сам экран /profile/devices — тикет #730, здесь шов —
    // переход.
    await page.getByRole('button', { name: 'Устройства' }).click();
    await expect(page).toHaveURL(/\/profile\/devices$/);
  });

  test('хаб: «Выйти» — шит «Вы уверены, что хотите выйти?» (мок 2004-45981)', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile');

    await page.getByRole('button', { name: 'Выйти' }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText('Вы уверены, что хотите выйти?')).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Отменить' })).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Выйти' })).toBeVisible();
    await captureScreen(page, testInfo, 'profile-hub-logout-sheet-mobile');

    // Отмена закрывает шит, сессия жива — хаб на месте. Подтверждённый
    // логаут не гоняем: он угасил бы сид-сессию для параллельных воркеров.
    await dialog.getByRole('button', { name: 'Отменить' }).click();
    await expect(page.getByRole('dialog')).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Аккаунт' })).toBeVisible();
  });

  test('все маршруты дерева отвечают канонной шапкой подэкрана', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);

    for (const [path, title] of SUBPAGE_TITLES) {
      await page.goto(path);
      const header = screenHeader(page);
      await expect(header.getByRole('button', { name: 'Назад' })).toBeVisible();
      await expect(header.getByText(title, { exact: true })).toBeVisible();
    }

    // Info-группа: в шапке проверяется только «Назад», канонный заголовок —
    // в контенте (редизайн ad04300b).
    for (const [path, content] of INFO_ROUTES) {
      await page.goto(path);
      const header = screenHeader(page);
      await expect(header.getByRole('button', { name: 'Назад' })).toBeVisible();
      await expect(content(page)).toBeVisible();
    }
  });

  test('планшет 561: подэкран без крыльев, хаб с крыльями', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.setViewportSize({ width: 561, height: 900 });
    const header = screenHeader(page);

    // Хаб профиля: крылья и на планшете (канон хаб-экранов), без «Назад».
    await page.goto('/profile');
    await expect(header.getByRole('link', { name: 'Объекты' })).toBeVisible();
    await expect(header.getByRole('button', { name: 'Назад' })).toHaveCount(0);

    // «Настроить уведомления» (#746): саб-экран — «Назад» вместо крыльев
    // и на планшете (канон подэкранов).
    await page.goto('/profile/notifications');
    await expect(header.getByRole('button', { name: 'Назад' })).toBeVisible();
    await expect(header.getByRole('link', { name: 'Объекты' })).toHaveCount(0);
  });
});

test.describe('дерево профиля — ПК ≥1024', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('«Настроить уведомления»: пилюля активна, TabBar скрыт', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/notifications');

    // Пилюля «Уведомления» (левый-низ) подсвечена и на саб-экране настроек
    // (правило пилюль #561, #746), TabBar на ПК не рендерится.
    await expect(
      page
        .getByRole('navigation', { name: 'Дополнительная навигация' })
        .getByRole('link', { name: 'Уведомления' }),
    ).toHaveAttribute('aria-current', 'page');
    await expect(page.locator(BOTTOM_NAV)).toBeHidden();
  });

  test('вход в дерево по кнопке юзера в хедере', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/properties');
    const header = screenHeader(page);

    // «Крыло» UserButton ведёт на /profile.
    await header.getByRole('button', { name: USER_WING }).click();
    await expect(page).toHaveURL(/\/profile$/);
    await expect(header.getByText('Профиль', { exact: true })).toBeVisible();
  });
});
