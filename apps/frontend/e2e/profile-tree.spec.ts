import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Дерево профиля на едином хроме (карта #556, тикет #566): все /profile*
// переехали из (cabinet) в (screens) без смены URL. Подэкраны — TopNav с
// ведущим «Назад» (history-first goBack, фолбэк — родительская страница),
// /profile/notifications — хаб-страница таба «Уведомления» (анатомия #567),
// активность — по нав-модели #558 без правок.

/** Подэкраны дерева: путь → заголовок в шапке. /profile/tariff/payments/[id]
 * не входит — требует id платежа, канон его шапки совпадает со списком. */
const SUBPAGE_TITLES: ReadonlyArray<readonly [string, string]> = [
  ['/profile', 'Профиль'],
  ['/profile/personal', 'Мои данные'],
  ['/profile/account', 'Аккаунт'],
  ['/profile/account/phone', 'Изменение телефона'],
  ['/profile/info', 'Информация'],
  ['/profile/info/privacy', 'Политика конфиденциальности'],
  ['/profile/info/terms', 'Пользовательское соглашение'],
  ['/profile/tariff', 'Тариф'],
  ['/profile/tariff/change', 'Сменить тариф'],
  ['/profile/tariff/change/success', 'Тариф изменён'],
  ['/profile/tariff/payment-methods', 'Способы оплаты'],
  ['/profile/tariff/payment-methods/add', 'Добавить карту'],
  ['/profile/tariff/payments', 'История платежей'],
];

const SCREEN_HEADER = 'header[aria-label="Навигация экрана"]';
const BOTTOM_NAV = 'nav[aria-label="Нижняя навигация"]';
// Сид-юзер «Иван Иванов»; до загрузки useMe кнопка показывает плейсхолдер.
const USER_WING = /Пользователь|Иван/;

test.describe('дерево профиля — хром #566', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('хаб «Уведомления»: крылья на мобайле, таб активен, без «Назад»', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/notifications');

    // Хаб-анатомия: крылья и на мобайле, ведущего «Назад» нет —
    // страница таба, а не подэкран профиля.
    const header = page.locator(SCREEN_HEADER);
    await expect(header.getByRole('link', { name: 'Объекты' })).toBeVisible();
    await expect(header.getByRole('button', { name: USER_WING })).toBeVisible();
    await expect(header.getByRole('button', { name: 'Назад' })).toHaveCount(0);
    await expect(
      page.getByRole('heading', { level: 1, name: 'Уведомления', exact: true }),
    ).toBeVisible();
    // Состав прежний: настройка каналов уведомлений.
    await expect(
      page.getByText('Письма об оплате подписки приходят на вашу почту', {
        exact: false,
      }),
    ).toBeVisible();

    // Активность по нав-модели: средний таб TabBar подсвечен.
    await expect(
      page.locator(BOTTOM_NAV).getByRole('link', { name: 'Уведомления' }),
    ).toHaveAttribute('aria-current', 'page');

    await captureScreen(page, testInfo, 'profile-notifications-hub-mobile');
  });

  test('подэкраны: «Назад» по истории, заголовок в шапке, вход из меню', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile');

    // Подэкран: «Назад» есть, крыльев на мобайле нет, заголовок в TopNav.
    const header = page.locator(SCREEN_HEADER);
    await expect(header.getByRole('button', { name: 'Назад' })).toBeVisible();
    await expect(header.getByRole('link', { name: 'Объекты' })).toHaveCount(0);
    await expect(header.getByText('Профиль', { exact: true })).toBeVisible();

    // Вглубь дерева по меню профиля; «Назад» возвращается по истории.
    await page.getByRole('link', { name: 'Перейти в раздел «Мои данные»' }).click();
    await expect(page).toHaveURL(/\/profile\/personal$/);
    await expect(header.getByText('Мои данные', { exact: true })).toBeVisible();
    await header.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(/\/profile$/);
  });

  test('все маршруты дерева отвечают канонной шапкой подэкрана', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);

    for (const [path, title] of SUBPAGE_TITLES) {
      await page.goto(path);
      const header = page.locator(SCREEN_HEADER);
      await expect(header.getByRole('button', { name: 'Назад' })).toBeVisible();
      await expect(header.getByText(title, { exact: true })).toBeVisible();
    }
  });

  test('планшет 561: подэкран без крыльев, хаб с крыльями', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.setViewportSize({ width: 561, height: 900 });
    const header = page.locator(SCREEN_HEADER);

    // Подэкран с «Назад»: крыльев нет и на планшете — колонка 560 начинается
    // там же, где кончается лого, крыло наложилось бы на неё (аудит #563).
    await page.goto('/profile');
    await expect(header.getByRole('button', { name: 'Назад' })).toBeVisible();
    await expect(header.getByRole('link', { name: 'Объекты' })).toHaveCount(0);

    // Хаб «Уведомления»: крылья и на планшете (канон хаб-экранов).
    await page.goto('/profile/notifications');
    await expect(header.getByRole('link', { name: 'Объекты' })).toBeVisible();
    await expect(header.getByRole('button', { name: 'Назад' })).toHaveCount(0);
  });
});

test.describe('дерево профиля — ПК ≥1024', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('хаб «Уведомления»: пилюля активна, TabBar скрыт', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/notifications');

    // Пилюля «Уведомления» (левый-низ) подсвечена, TabBar на ПК не рендерится.
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

    // «Крыло» UserButton ведёт на /profile.
    await page
      .locator(SCREEN_HEADER)
      .getByRole('button', { name: USER_WING })
      .click();
    await expect(page).toHaveURL(/\/profile$/);
    await expect(
      page.locator(SCREEN_HEADER).getByText('Профиль', { exact: true }),
    ).toBeVisible();
  });
});
