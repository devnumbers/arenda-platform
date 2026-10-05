import type { Locator, Page } from '@playwright/test';
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
// секции, паузные правила не выводятся, заголовки-стрелки ведут на
// страницы секций.
// Скриншоты — материал для сверки с Figma.

const APARTMENT_PAYMENTS_URL = `/properties/${SEEDED_APARTMENT_PROPERTY_ID}/payments`;
const GARAGE_PAYMENTS_URL = `/properties/${SEEDED_GARAGE_PROPERTY_ID}/payments`;

// Доступное имя карточки выбора — весь её текст; якорим к описанию, чтобы не
// путать с «Автоплатеж» (подстрока) и строкой черновика «Платеж Черновик…».
const PAYMENT_CARD = /Платеж Отмечайте оплату/;
const AUTOPAYMENT_CARD = /Автоплатеж Отмечается автоматически/;

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
    // Секция карточки #583/#588: ссылка называется заголовком секции.
    await page.getByRole('link', { name: 'Регулярные платежи' }).click();

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

    // Строки секций считаем по текстам с суммой (заголовок-стрелка —
    // кнопка без «₽»; строки операций не кликабельны и роли button не
    // имеют). Содержимое секций зависит от параллельных сценариев,
    // создающих платежи на той же квартире, — проверяем форму, а не состав:
    // у каждой секции от 1 до 3 строк, у просроченных красный срок.
    const overdueRows = page.getByTestId('section-overdue').getByText('₽');
    await expect(overdueRows.first()).toBeVisible();
    expect(await overdueRows.count()).toBeLessThanOrEqual(3);
    await expect(page.getByTestId('section-overdue').getByText(/\d+ (день|дня|дней)/).first()).toBeVisible();

    const paymentRows = page.getByTestId('section-payments').getByText('₽');
    await expect(paymentRows.first()).toBeVisible();
    expect(await paymentRows.count()).toBeLessThanOrEqual(3);

    const autoRows = page.getByTestId('section-auto').getByText('₽');
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
    await expect(page.getByText('На паузе').first()).toBeVisible();
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
    // просрочки), проверяем что список непуст: строки операций не
    // кликабельны — ищем по тексту красного срока.
    await expect(page.getByText(/\d+ (день|дня|дней)/).first()).toBeVisible();
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

  // Якоря — сидовые правила, которые сценарные спеки не мутируют:
  // «Электроэнергия» …553 (автоплатёж) и «Домофон» …554 (ручной, на паузе);
  // «Арендную плату» трогает арендная фикстура (платёж аренды — автоплатёж).
  test('автоплатёж не в списке «Платежи», в списке «Автоплатежи» (#1071)', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);

    await page.goto(`${APARTMENT_PAYMENTS_URL}/all`);
    await expect(page.getByText('Электроэнергия')).toHaveCount(0);
    await expect(page.getByText('Домофон').first()).toBeVisible();

    await page.goto(`${APARTMENT_PAYMENTS_URL}/auto`);
    await expect(page.getByText('Электроэнергия').first()).toBeVisible();
    await expect(page.getByText('Домофон')).toHaveCount(0);
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

  test('черновик в шите: «У вас есть черновик» вместо выбора типа (1134:40451)', async ({
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
        JSON.stringify({ categorySlug: 'rent', title: 'Арендная плата', updatedAt: 1756400000000 }),
      ],
    );
    await page.goto(APARTMENT_PAYMENTS_URL);

    await page.getByRole('button', { name: 'Добавить' }).click();
    await expect(page.getByRole('heading', { name: 'У вас есть черновик' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Продолжить черновик' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Создать новый' })).toBeVisible();
    // Сценарии не объединяются: с черновиком выбора типа в шите нет.
    await expect(page.getByRole('button', { name: PAYMENT_CARD })).toHaveCount(0);
    await expect(page.getByRole('button', { name: AUTOPAYMENT_CARD })).toHaveCount(0);
    await page.waitForTimeout(700);
    await captureScreen(page, testInfo, 'payments-sheet-draft-mobile');

    // «Продолжить черновик» — визард восстанавливается на шаге периодичности:
    // категория и название уже в черновике.
    await page.getByRole('button', { name: 'Продолжить черновик' }).click();
    await expect(page).toHaveURL(`${APARTMENT_PAYMENTS_URL}/new?type=payment`);
    await expect(page.getByRole('heading', { name: 'Периодичность платежа' })).toBeVisible();
  });

  test('«Создать новый» стирает черновик и в той же модалке показывает выбор типа', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    const draftKey = paymentDraftStorageKey(SEEDED_APARTMENT_PROPERTY_ID, 'payment');
    await page.addInitScript(
      ([key, value]) => {
        window.localStorage.setItem(key ?? '', value ?? '');
      },
      [draftKey, JSON.stringify({ categorySlug: 'rent', title: 'Арендная плата', updatedAt: 1756400000000 })],
    );
    await page.goto(APARTMENT_PAYMENTS_URL);

    await page.getByRole('button', { name: 'Добавить' }).click();
    await page.getByRole('button', { name: 'Создать новый' }).click();
    // Модалка не переоткрывается: контент сменился на выбор типа,
    // черновик уже стёрт.
    await expect(page.getByRole('heading', { name: 'Выберите тип платежа' })).toBeVisible();
    await expect(page.getByRole('button', { name: PAYMENT_CARD })).toBeVisible();
    const storedAfterDiscard = await page.evaluate(
      (key) => window.localStorage.getItem(key),
      draftKey,
    );
    expect(storedAfterDiscard).toBeNull();

    // Дальше — как без черновика: карточка ведёт в визард с нуля (шаг 1).
    await page.getByRole('button', { name: PAYMENT_CARD }).click();
    await expect(page).toHaveURL(`${APARTMENT_PAYMENTS_URL}/new?type=payment`);
    await expect(page.getByRole('heading', { name: 'Категория платежа' })).toBeVisible();
  });
});

test.describe('входы в создание с проверкой черновика (#1066)', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  const GARAGE_PROPERTY_URL = `/properties/${SEEDED_GARAGE_PROPERTY_ID}`;
  const GARAGE_DRAFT_KEY = paymentDraftStorageKey(SEEDED_GARAGE_PROPERTY_ID, 'payment');
  const APARTMENT_AUTO_DRAFT_KEY = paymentDraftStorageKey(SEEDED_APARTMENT_PROPERTY_ID, 'autopayment');

  /** CTA «Добавить» пустой секции «Регулярные платежи» на странице объекта:
   * у гаража пустых секций несколько — якорим к заголовку пустой платежной. */
  const paymentsSectionCta = (page: Page): Locator =>
    page
      .locator('section')
      .filter({ hasText: 'Платежи не добавлены' })
      .getByRole('button', { name: 'Добавить' });

  /** Посеять валидный черновик платежа гаража до загрузки экрана. */
  const seedGarageDraft = async (page: Page): Promise<void> => {
    await page.addInitScript(
      ([key, value]) => {
        window.localStorage.setItem(key ?? '', value ?? '');
      },
      [
        GARAGE_DRAFT_KEY,
        JSON.stringify({ categorySlug: 'rent', title: 'Аренда гаража', updatedAt: 1756400000000 }),
      ],
    );
  };

  test('страница объекта, пустая секция: с черновиком — «У вас есть черновик», «Продолжить» resume-ит', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await seedGarageDraft(page);
    await page.goto(GARAGE_PROPERTY_URL);

    // Раньше CTA кидал straight в визард, молча возобновляя черновик (#1066).
    await paymentsSectionCta(page).click();
    await expect(page.getByRole('heading', { name: 'У вас есть черновик' })).toBeVisible();

    await page.getByRole('button', { name: 'Продолжить черновик' }).click();
    await expect(page).toHaveURL(`${GARAGE_PROPERTY_URL}/payments/new?type=payment`);
    // Категория и название в черновике — resume на шаге периодичности.
    await expect(page.getByRole('heading', { name: 'Периодичность платежа' })).toBeVisible();
  });

  test('страница объекта, пустая секция: без черновика — выбор типа, как на странице платежей', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(GARAGE_PROPERTY_URL);

    await paymentsSectionCta(page).click();
    await expect(page.getByRole('heading', { name: 'Выберите тип платежа' })).toBeVisible();
    await expect(page.getByRole('button', { name: PAYMENT_CARD })).toBeVisible();
    await expect(page.getByRole('button', { name: AUTOPAYMENT_CARD })).toBeVisible();

    await page.getByRole('button', { name: PAYMENT_CARD }).click();
    await expect(page).toHaveURL(`${GARAGE_PROPERTY_URL}/payments/new?type=payment`);
    await expect(page.getByRole('heading', { name: 'Категория платежа' })).toBeVisible();
  });

  test('каталог «Все платежи»: с черновиком «Создать новый» ведёт в чистый визард своего типа', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    const draftKey = paymentDraftStorageKey(SEEDED_APARTMENT_PROPERTY_ID, 'payment');
    await page.addInitScript(
      ([key, value]) => {
        window.localStorage.setItem(key ?? '', value ?? '');
      },
      [draftKey, JSON.stringify({ categorySlug: 'rent', title: 'Арендная плата', updatedAt: 1756400000000 })],
    );
    await page.goto(`${APARTMENT_PAYMENTS_URL}/all`);

    await page.getByRole('button', { name: 'Добавить платеж' }).click();
    await expect(page.getByRole('heading', { name: 'У вас есть черновик' })).toBeVisible();

    await page.getByRole('button', { name: 'Создать новый' }).click();
    await expect(page).toHaveURL(`${APARTMENT_PAYMENTS_URL}/new?type=payment`);
    // Фиксированный тип: фазы выбора нет — сразу чистый визард шага 1.
    await expect(page.getByRole('heading', { name: 'Категория платежа' })).toBeVisible();
    const storedAfterDiscard = await page.evaluate(
      (key) => window.localStorage.getItem(key),
      draftKey,
    );
    expect(storedAfterDiscard).toBeNull();
  });

  test('каталог «Все платежи»: без черновика — прежний прямой вход в визард', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`${APARTMENT_PAYMENTS_URL}/all`);

    await page.getByRole('button', { name: 'Добавить платеж' }).click();
    await expect(page).toHaveURL(`${APARTMENT_PAYMENTS_URL}/new?type=payment`);
    await expect(page.getByRole('heading', { name: 'У вас есть черновик' })).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Категория платежа' })).toBeVisible();
  });

  test('каталог «Автоплатежи»: черновик автоплатежа — модалка, «Продолжить» resume-ит автоплатеж', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.addInitScript(
      ([key, value]) => {
        window.localStorage.setItem(key ?? '', value ?? '');
      },
      [
        APARTMENT_AUTO_DRAFT_KEY,
        JSON.stringify({ categorySlug: 'rent', title: 'Автоплатёж аренды', updatedAt: 1756400000000 }),
      ],
    );
    await page.goto(`${APARTMENT_PAYMENTS_URL}/auto`);

    await page.getByRole('button', { name: 'Добавить автоплатеж' }).click();
    await expect(page.getByRole('heading', { name: 'У вас есть черновик' })).toBeVisible();

    await page.getByRole('button', { name: 'Продолжить черновик' }).click();
    await expect(page).toHaveURL(`${APARTMENT_PAYMENTS_URL}/new?type=autopayment`);
    await expect(page.getByRole('heading', { name: 'Периодичность платежа' })).toBeVisible();
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
