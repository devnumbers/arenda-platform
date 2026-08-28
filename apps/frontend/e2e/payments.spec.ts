import { paymentDraftStorageKey } from '@/features/payments';
import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  test,
} from './fixtures';

// Экран «Платежи объекта» (#463, Figma 1043:57610/1043:62920): три секции —
// «Просроченные операции / Платежи / Автоплатежи», максимум 3 строки в
// секции, паузные правила не выводятся, поиск фильтрует секции независимо
// (серверный search), заголовки-стрелки ведут на страницы секций.
// Скриншоты — материал для сверки с Figma.

const APARTMENT_PAYMENTS_URL = `/properties/${SEEDED_APARTMENT_PROPERTY_ID}/payments`;
const GARAGE_PAYMENTS_URL = `/properties/${SEEDED_GARAGE_PROPERTY_ID}/payments`;

// Доступное имя карточки выбора — весь её текст; якорим к описанию, чтобы не
// путать с «Автоплатеж» (подстрока) и строкой черновика «Платеж Черновик…».
const PAYMENT_CARD = /Платеж Напомним, когда нужно/;
const AUTOPAYMENT_CARD = /Автоплатеж Предупредим о платеже/;

test.describe('экран «Платежи объекта»', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('вход со страницы объекта через секцию-ссылку «Платежи»', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/properties');
    // Карточка объекта — ссылка-оверлей поверх контента карточки.
    await page.getByRole('link', { name: 'Открыть объект Квартира на Ленина' }).click();
    await page.getByRole('link', { name: /Платежи/ }).click();

    await expect(page).toHaveURL(new RegExp(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}/payments$`));
    await expect(page.getByText('Платежи объекта', { exact: true })).toBeVisible();
  });

  test('три секции, максимум 3 строки, паузные скрыты', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(APARTMENT_PAYMENTS_URL);

    await expect(page.getByRole('heading', { name: 'Просроченные операции' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Платежи', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Автоплатежи' })).toBeVisible();

    // Строки секций считаем по кнопкам с суммой (заголовок-стрелка — тоже
    // кнопка, но без «₽»). Содержимое секций зависит от параллельных
    // сценариев, создающих платежи на той же квартире, — проверяем форму,
    // а не состав: у каждой секции от 1 до 3 строк с красным сроком у
    // просроченных.
    const overdueRows = page.getByTestId('section-overdue').getByRole('button').filter({ hasText: '₽' });
    await expect(overdueRows.first()).toBeVisible();
    expect(await overdueRows.count()).toBeLessThanOrEqual(3);
    await expect(page.getByTestId('section-overdue').getByText(/\d+ (день|дня|дней)/).first()).toBeVisible();

    const paymentRows = page.getByTestId('section-payments').getByRole('button').filter({ hasText: '₽' });
    await expect(paymentRows.first()).toBeVisible();
    expect(await paymentRows.count()).toBeLessThanOrEqual(3);

    const autoRows = page.getByTestId('section-auto').getByRole('button').filter({ hasText: '₽' });
    await expect(autoRows.first()).toBeVisible();
    expect(await autoRows.count()).toBeLessThanOrEqual(3);

    // Паузное правило не выводится: строк «На паузе» нет ни в одной секции
    // (его просроченный долг при этом остаётся в секции просроченных).
    await expect(page.getByTestId('section-payments').getByText('На паузе')).toHaveCount(0);
    await expect(page.getByTestId('section-auto').getByText('На паузе')).toHaveCount(0);

    // Завершённое правило (вхождений нет) — в конце сортировки, за лимитом.
    await expect(page.getByText('Техосмотр')).toHaveCount(0);

    await expect(page.getByRole('button', { name: 'Добавить' })).toBeVisible();

    await captureScreen(page, testInfo, 'payments-filled-mobile');
  });

  test('заголовок секции ведёт на страницу секции', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(APARTMENT_PAYMENTS_URL);

    await page.getByRole('button', { name: 'Открыть все платежи' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/all$`));
    await expect(page.getByText('Платежи объекта', { exact: true })).toBeVisible();
    // На странице секции паузные видны («там уже всё видно»).
    await expect(page.getByText('Домофон')).toBeVisible();
    await expect(page.getByText('На паузе')).toBeVisible();
  });

  test('страница просроченных операций открывает полный список', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(APARTMENT_PAYMENTS_URL);

    await page.getByRole('button', { name: 'Открыть просроченные операции' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/overdue$`));
    // Состав строк зависит от параллельных сценариев (гасят сидовые
    // просрочки), проверяем что список непуст и строки красные.
    await expect(
      page.getByRole('button').filter({ hasText: '₽' }).filter({ hasText: /\d+ (день|дня|дней)/ }).first(),
    ).toBeVisible();
  });

  test('поиск фильтрует каждую секцию независимо', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(APARTMENT_PAYMENTS_URL);
    // Ищем оверлейное правило: параллельные сценарии его не трогают.
    await page.getByRole('button', { name: 'Поиск' }).click();
    await page.getByRole('searchbox', { name: 'Поиск по названиям' }).fill('клининг');

    await expect(page.getByTestId('section-payments').getByText('Клининг холла')).toBeVisible();
    await expect(page.getByTestId('section-payments').getByRole('button').filter({ hasText: '₽' })).toHaveCount(1);
    await expect(page.getByTestId('section-auto').getByRole('button').filter({ hasText: '₽' })).toHaveCount(0);
    await expect(page.getByTestId('section-overdue').getByRole('button').filter({ hasText: '₽' })).toHaveCount(0);
  });

  test('поиск без совпадений — «Ничего не нашлось» в каждой секции', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(APARTMENT_PAYMENTS_URL);

    await page.getByRole('button', { name: 'Поиск' }).click();
    await page.getByRole('searchbox', { name: 'Поиск по названиям' }).fill('ипотека');

    await expect(page.getByText('Ничего не нашлось').first()).toBeVisible();
  });

  test('пустые состояния секций на объекте без платежей; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(GARAGE_PAYMENTS_URL);

    await expect(page.getByText('У вас нет просроченных операций')).toBeVisible();
    await expect(
      page.getByText('Напомним, когда нужно будет отметить оплату, вы вручную отметите платеж'),
    ).toBeVisible();
    await expect(
      page.getByText('Предупредим о платеже, потом автоматически отметим оплату'),
    ).toBeVisible();

    await captureScreen(page, testInfo, 'payments-empty-mobile');
  });

  test('страницы секций гаража — пустые состояния с иллюстрацией', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);

    await page.goto(`${GARAGE_PAYMENTS_URL}/all`);
    await expect(page.getByText('Нет платежей')).toBeVisible();
    await expect(page.locator('img[src*="empty-payments"]')).toBeVisible();

    await page.goto(`${GARAGE_PAYMENTS_URL}/auto`);
    await expect(page.getByText('Нет автоплатежей')).toBeVisible();

    await page.goto(`${GARAGE_PAYMENTS_URL}/overdue`);
    await expect(page.getByText('Нет просроченных операций')).toBeVisible();
  });

  test('шит выбора «Платёж / Автоплатёж»; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(APARTMENT_PAYMENTS_URL);

    await page.getByRole('button', { name: 'Добавить' }).click();
    await expect(page.getByRole('button', { name: PAYMENT_CARD })).toBeVisible();
    await expect(page.getByRole('button', { name: AUTOPAYMENT_CARD })).toBeVisible();
    // Скриншот после оседания анимации шита (~450ms на --dl-ease).
    await page.waitForTimeout(700);
    await captureScreen(page, testInfo, 'payments-sheet-mobile');
  });

  test('черновик в шите: строка «Черновик» и подтверждение «Создать новый»', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    // Черновик визарда живёт в localStorage per объект+тип; сеем валидный
    // payload до загрузки экрана — обёртка usePaymentWizardDraft его подхватит.
    await page.addInitScript(
      ([key, value]) => {
        window.localStorage.setItem(key ?? '', value ?? '');
      },
      [
        paymentDraftStorageKey(SEEDED_APARTMENT_PROPERTY_ID, 'payment'),
        JSON.stringify({ categorySlug: 'rent', title: 'Арендная плата' }),
      ],
    );
    await page.goto(APARTMENT_PAYMENTS_URL);

    await page.getByRole('button', { name: 'Добавить' }).click();
    await expect(page.getByText('Черновик')).toBeVisible();
    await expect(page.getByText('Продолжить')).toBeVisible();

    // Карточка типа с черновиком переключает шит в подтверждение (837:21349).
    await page.getByRole('button', { name: PAYMENT_CARD }).click();
    await expect(page.getByRole('button', { name: /Создать новый/ })).toBeVisible();
    await page.waitForTimeout(700);
    await captureScreen(page, testInfo, 'payments-sheet-draft-mobile');
  });
});

test.describe('экран «Платежи объекта» — десктоп', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('глобальный хром, колонка 560, секции и модалка выбора; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(APARTMENT_PAYMENTS_URL);

    await expect(page.getByRole('heading', { name: 'Просроченные операции' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Платежи', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Добавить' })).toBeVisible();

    await captureScreen(page, testInfo, 'payments-filled-desktop');

    await page.getByRole('button', { name: 'Добавить' }).click();
    await expect(page.getByRole('button', { name: PAYMENT_CARD })).toBeVisible();
    // Радикс-модалка появляется за 400ms — ждём оседания.
    await page.waitForTimeout(600);
    await captureScreen(page, testInfo, 'payments-sheet-desktop');
  });
});
