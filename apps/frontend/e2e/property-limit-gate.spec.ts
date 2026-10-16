import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Интерстишал «Превышен лимит объектов» (#1469, макет 3516-79702):
// промежуточный экран вместо мгновенного редиректа на выбор тарифа.
// Входы — «+» хаба объектов при исчерпанном конечном лимите и визард
// создания (закодированный 402 property_limit_reached, #1468). Текст
// описания адаптивен по тарифу (basic → «Про» или «Бизнес», pro →
// «Бизнес», нет подписки — basic); незакодированный 402 визарда не
// маскируется под гейт — канон #1452.
//
// Исчерпанный лимит сид-владельца воспроизводим перехватом подписки:
// у сида 3 активных объекта, мок pro с лимитом 2 закрывает вход.
// POST /properties мокается точечно — бюджет реальных созданий сида
// (два на прогон) не тратится.

const PRO_AT_LIMIT_SUBSCRIPTION = {
  id: 'e2e-property-limit-pro',
  status: 'active',
  tariff: {
    id: 'pro',
    name: 'pro',
    monthlyPriceKopecks: 49000,
    yearlyPriceKopecks: 440000,
    activePropertyLimit: 2,
  },
  autoRenewEnabled: true,
  currentPeriod: 'month',
};

function mockSubscription(page: Parameters<typeof openCabinetWithSeededSession>[0]): void {
  void page.route('**/api/subscription', (route) =>
    route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(PRO_AT_LIMIT_SUBSCRIPTION) }),
  );
}

test('хаб: «+» при исчерпанном лимите pro ведёт на интерстишал с текстом pro', async ({ page, seededUser }, testInfo) => {
  mockSubscription(page);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties');

  await page.getByTestId('properties-create').click();
  await expect(page).toHaveURL(/\/profile\/tariff\/property-limit$/);
  await expect(page.getByRole('heading', { name: 'Превышен лимит объектов' })).toBeVisible();
  // pro — следующий тариф только «Бизнес» (решение владельца, #1469).
  await expect(
    page.getByText('Повысьте тариф до «Бизнес», чтобы добавить больше объектов'),
  ).toBeVisible();

  await captureScreen(page, testInfo, 'property-limit-interstitial-pro');
});

test('интерстишал: «Отменить» возвращает назад на хаб объектов', async ({ page, seededUser }) => {
  mockSubscription(page);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties');
  await page.getByTestId('properties-create').click();
  await expect(page).toHaveURL(/\/profile\/tariff\/property-limit$/);

  await page.getByRole('button', { name: 'Отменить' }).click();
  await expect(page).toHaveURL(/\/properties$/);
});

test('интерстишал: defensive-фолбэк текста на неожиданном тарифе (business)', async ({ page, seededUser }) => {
  // Business (безлимит) на этот экран потоками не приводит — фолбэк
  // страховка глубоких ссылок и будущих тарифов: текст без имён.
  void page.route('**/api/subscription', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({
        ...PRO_AT_LIMIT_SUBSCRIPTION,
        tariff: { ...PRO_AT_LIMIT_SUBSCRIPTION.tariff, name: 'business', activePropertyLimit: -1 },
      }),
    }),
  );
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/profile/tariff/property-limit');

  await expect(page.getByText('Повысьте тариф, чтобы добавить больше объектов', { exact: true })).toBeVisible();
});

test('интерстишал: «Выбрать тариф» ведёт на смену тарифа', async ({ page, seededUser }) => {
  mockSubscription(page);
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/profile/tariff/property-limit');

  await page.getByRole('button', { name: 'Выбрать тариф' }).click();
  await expect(page).toHaveURL(/\/profile\/tariff\/change$/);
});

test('визард: закодированный 402 property_limit_reached уводит на интерстишал', async ({ page, seededUser }) => {
  mockSubscription(page);
  await page.route('**/api/properties', (route) => {
    if (route.request().method() !== 'POST') {
      void route.fallback();
      return;
    }
    void route.fulfill({
      status: 402,
      contentType: 'application/problem+json',
      body: JSON.stringify({
        type: 'about:blank',
        title: 'Limit exceeded',
        status: 402,
        detail: 'Превышен лимит активных объектов',
        code: 'property_limit_reached',
      }),
    });
  });
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties/new');

  await page.getByRole('group', { name: 'Категория объекта' }).getByRole('button', { name: 'Дом' }).click();
  await page.getByRole('textbox', { name: 'Введите адрес' }).fill('Ленина, 2');
  await page.getByRole('button', { name: 'Продолжить' }).click();
  await page.getByRole('button', { name: 'Создать объект' }).click();

  await expect(page).toHaveURL(/\/profile\/tariff\/property-limit$/);
  await expect(page.getByRole('heading', { name: 'Превышен лимит объектов' })).toBeVisible();
});

test('визард: незакодированный 402 остаётся генерик-ошибкой (канон #1452)', async ({ page, seededUser }) => {
  mockSubscription(page);
  await page.route('**/api/properties', (route) => {
    if (route.request().method() !== 'POST') {
      void route.fallback();
      return;
    }
    void route.fulfill({
      status: 402,
      contentType: 'application/problem+json',
      body: JSON.stringify({
        type: 'about:blank',
        title: 'Limit exceeded',
        status: 402,
        detail: 'Превышен лимит активных объектов',
      }),
    });
  });
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties/new');

  await page.getByRole('group', { name: 'Категория объекта' }).getByRole('button', { name: 'Дом' }).click();
  await page.getByRole('textbox', { name: 'Введите адрес' }).fill('Ленина, 2');
  await page.getByRole('button', { name: 'Продолжить' }).click();
  await page.getByRole('button', { name: 'Создать объект' }).click();

  await expect(page).toHaveURL(/\/properties\/new$/);
  await expect(
    page.getByText('Не удалось создать объект. Проверьте соединение и попробуйте ещё раз'),
  ).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Превышен лимит объектов' })).toHaveCount(0);
});
