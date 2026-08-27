import { paymentDraftStorageKey } from '@/features/payments';
import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  test,
} from './fixtures';

// Экран «Платежи объекта» (#463): вход секцией-ссылкой со страницы объекта,
// секции «Просроченные / Платежи / Автоплатежи», поиск, шит выбора
// «Платёж / Автоплатёж» с индикацией черновика. Скриншоты — материал
// для сверки с Figma (784:13393, 837:21349, 654:6778, 853:17208).

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

  test('секции: просроченные карточки, платежи, автоплатежи; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(APARTMENT_PAYMENTS_URL);

    await expect(page.getByRole('heading', { name: 'Просроченные' })).toBeVisible();
    // Две просроченные операции с красным сроком; точное число дней зависит
    // от даты прогона.
    await expect(page.getByText('Арендная плата').first()).toBeVisible();
    await expect(page.getByText(/\d+ (день|дня|дней)/).first()).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Платежи', exact: true })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Автоплатежи' })).toBeVisible();
    await expect(page.getByText('Страхование').first()).toBeVisible();
    await expect(page.getByText('Электроэнергия')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Добавить' })).toBeVisible();

    await captureScreen(page, testInfo, 'payments-filled-mobile');
  });

  test('поиск по названиям фильтрует секции', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(APARTMENT_PAYMENTS_URL);
    await expect(page.getByText('Страхование').first()).toBeVisible();

    await page.getByRole('button', { name: 'Поиск' }).click();
    await page.getByRole('searchbox', { name: 'Поиск по названиям' }).fill('страх');

    await expect(page.getByText('Страхование').first()).toBeVisible();
    await expect(page.getByText('Арендная плата')).toHaveCount(0);
    await expect(page.getByText('Электроэнергия')).toHaveCount(0);
  });

  test('поиск без совпадений показывает «Ничего не нашлось»', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(APARTMENT_PAYMENTS_URL);

    await page.getByRole('button', { name: 'Поиск' }).click();
    await page.getByRole('searchbox', { name: 'Поиск по названиям' }).fill('ипотека');

    await expect(page.getByText('Ничего не нашлось')).toBeVisible();
  });

  test('пустые состояния секций на объекте без платежей; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(GARAGE_PAYMENTS_URL);

    await expect(page.getByText('Нет просроченных платежей')).toBeVisible();
    await expect(page.getByText('Нет платежей')).toBeVisible();
    await expect(page.getByText('Нет автоплатежей')).toBeVisible();

    await captureScreen(page, testInfo, 'payments-empty-mobile');
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

    await expect(page.getByRole('heading', { name: 'Просроченные' })).toBeVisible();
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
