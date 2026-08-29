import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';

// Визард создания платежа (#464): пять шагов на одном маршруте
// /properties/[id]/payments/new, все ветки периодичности (без «Один раз»),
// черновик в localStorage, создание через POST и экран успеха с датой
// первого вхождения. Скриншоты — материал для сверки с Figma-фреймами #449
// (823:4243, 830:16721, 823:11219 + ветки дат, 843:8345, 834:19662,
// 835:19893).

const APARTMENT_PAYMENTS_URL = `/properties/${SEEDED_APARTMENT_PROPERTY_ID}/payments`;
const wizardUrl = (type: 'payment' | 'autopayment'): string =>
  `${APARTMENT_PAYMENTS_URL}/new?type=${type}`;

/** Открыть визард под сеансом сидированного пользователя.
 * Перед визардом заходим на «Платежи объекта»: как в реальном потоке —
 * тогда закрытие успеха возвращается назад по истории на источник,
 * а не на пустую запись about:blank свежей вкладки. */
async function openWizard(
  page: Parameters<typeof openCabinetWithSeededSession>[0],
  user: Parameters<typeof openCabinetWithSeededSession>[1],
  type: 'payment' | 'autopayment' = 'payment',
): Promise<void> {
  await openCabinetWithSeededSession(page, user);
  await page.goto(APARTMENT_PAYMENTS_URL);
  await expect(page.getByRole('button', { name: 'Добавить' })).toBeVisible();
  await page.goto(wizardUrl(type));
  await expect(
    page
      .getByRole('heading', { name: 'Периодичность платежа' })
      .or(page.getByRole('heading', { name: 'Категория платежа' })),
  ).toBeVisible();
}

/**
 * Пройти шаги 1–3 с категорией «Арендная плата» (первая строка каталога)
 * до выбранной периодичности: общий вход почти всех сценариев.
 */
async function selectCategory(page: Parameters<typeof openCabinetWithSeededSession>[0], slugLabel = 'Арендная плата'): Promise<void> {
  await page.getByRole('button', { name: slugLabel }).click();
  await page.getByRole('button', { name: 'Продолжить' }).click();
  await expect(page.getByRole('heading', { name: 'Назовите платеж' })).toBeVisible();
}

async function passTitleStep(page: Parameters<typeof openCabinetWithSeededSession>[0], title?: string): Promise<void> {
  const field = page.getByRole('textbox');
  if (title !== undefined) {
    await field.fill(title);
  }
  await page.getByRole('button', { name: 'Продолжить' }).click();
  await expect(page.getByRole('heading', { name: 'Периодичность платежа' })).toBeVisible();
}

type PaymentFromApi = {
  id: string;
  title: string;
  amountKopecks: number;
  type: string;
  paymentForm: string;
  autoPay: boolean;
  since: string;
  endDate?: string | null;
  recurrence: { kind: string; weekdays?: number[]; dayOfMonth?: number; month?: number; day?: number };
};

/** Прочитать список платежей объекта из API тем же сеансом (куки контекста). */
async function fetchPayments(
  page: Parameters<typeof openCabinetWithSeededSession>[0],
): Promise<PaymentFromApi[]> {
  const response = await page.request.get(`/api/properties/${SEEDED_APARTMENT_PROPERTY_ID}/payments`);
  expect(response.ok()).toBe(true);
  const payload = (await response.json()) as { items: PaymentFromApi[] };
  return payload.items;
}

test.describe('визард создания платежа', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('поиск категории: фокус при открытии, подсказка, крестик и клик вне закрывают', async ({
    page,
    seededUser,
  }) => {
    await openWizard(page, seededUser);

    // Лупа открывает поиск: подсказка вместо списка, поле сразу активно.
    await page.getByRole('button', { name: 'Поиск по категориям' }).click();
    await expect(page.getByText('Начните искать категорию')).toBeVisible();
    await expect(page.getByRole('searchbox')).toBeFocused();

    // С запросом — отфильтрованный список, подсказка исчезает.
    await page.getByRole('searchbox').fill('страхов');
    await expect(page.getByRole('button', { name: 'Страхование' })).toBeVisible();
    await expect(page.getByText('Начните искать категорию')).toHaveCount(0);

    // Крестик очищает и закрывает поиск.
    await page.getByRole('button', { name: 'Очистить поиск' }).click();
    await expect(page.getByRole('heading', { name: 'Категория платежа' })).toBeVisible();
    await expect(page.getByRole('searchbox')).toHaveCount(0);

    // Клик вне хедера при пустом запросе тоже закрывает.
    await page.getByRole('button', { name: 'Поиск по категориям' }).click();
    await expect(page.getByText('Начните искать категорию')).toBeVisible();
    await page.mouse.click(195, 400);
    await expect(page.getByRole('heading', { name: 'Категория платежа' })).toBeVisible();
    await expect(page.getByRole('searchbox')).toHaveCount(0);
  });

  test('полный путь платежа: ежемесячное 10-го; платёж в API и списке объекта', async ({
    page,
    seededUser,
  }, testInfo) => {
    const title = 'E2E страховка квартиры';
    await openWizard(page, seededUser);

    // Шаг 1 — лупа раскрывает поиск в хедере; запрос ведёт на «Страхование».
    await page.getByRole('button', { name: 'Поиск по категориям' }).click();
    await page.getByRole('searchbox').fill('страхов');
    await page.getByRole('button', { name: 'Страхование' }).click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await expect(page.getByRole('heading', { name: 'Назовите платеж' })).toBeVisible();
    await captureScreen(page, testInfo, 'wizard-step1-category-mobile');

    // Шаг 2 — название.
    await page.getByRole('textbox').fill(title);
    await page.getByRole('button', { name: 'Продолжить' }).click();

    // Шаг 3 — меню периодичности без «Один раз»; ветка месяца.
    await expect(page.getByRole('button', { name: 'Каждый день' })).toBeVisible();
    await expect(page.getByText('Один раз')).toHaveCount(0);
    await captureScreen(page, testInfo, 'wizard-step3-periodicity-mobile');
    await page.getByRole('button', { name: 'Каждый месяц' }).click();
    await expect(page.getByRole('heading', { name: 'Выберите день', exact: true })).toBeVisible();
    await page.getByRole('button', { name: '10', exact: true }).first().click();
    await captureScreen(page, testInfo, 'wizard-step3-month-days-mobile');

    // Шаг 4 — окончание необязательно: пропускаем.
    await page.getByRole('button', { name: 'Далее' }).click();
    await expect(page.getByRole('heading', { name: 'Окончание платежа' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();

    // Шаг 5 — сумма и признаки; кнопка заблокирована до заполнения.
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeDisabled();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('2500');
    await page.getByRole('button', { name: 'Расход' }).click();
    await page.getByRole('button', { name: 'Наличные' }).click();
    await captureScreen(page, testInfo, 'wizard-step5-amount-mobile');
    await page.getByRole('button', { name: 'Создать платеж' }).click();

    // Экран успеха с первым вхождением из серверного ответа.
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(
      `«${title}»`,
    );
    await expect(page.getByText(/Первый платеж .* на 2\u00A0500 ₽, далее каждый месяц 10 числа/)).toBeVisible();
    await expect(page.getByText('Посмотреть платеж')).toHaveCount(0);
    await captureScreen(page, testInfo, 'wizard-success-mobile');

    // Контракт: платёж появился в API с серверным since и регулярностью.
    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created).toBeDefined();
    expect(created?.recurrence).toStrictEqual({ kind: 'monthly', dayOfMonth: 10 });
    expect(created?.amountKopecks).toBe(250000);
    expect(created?.type).toBe('expense');
    expect(created?.paymentForm).toBe('cash');
    expect(created?.autoPay).toBe(false);
    expect(created?.since).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(created?.endDate ?? null).toBeNull();

    // Закрытие успеха возвращает на «Платежи объекта». Секции показывают
    // максимум 3 ближайших платежа, поэтому видимость карточки проверяем
    // на странице секции — там весь список.
    await page.getByRole('button', { name: 'Хорошо, закрыть' }).click();
    await expect(page).toHaveURL(new RegExp(`${APARTMENT_PAYMENTS_URL}$`));
    await page.getByRole('button', { name: 'Открыть все платежи' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/all$`));
    await expect(page.getByText(title).first()).toBeVisible();
  });

  test('ветка недели: несколько дней недели сохраняются сортированным набором', async ({
    page,
    seededUser,
  }) => {
    const title = 'E2E уборка по вторникам и пятницам';
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, title);

    await page.getByRole('button', { name: 'Каждую неделю' }).click();
    await expect(page.getByRole('heading', { name: 'Выберите день' })).toBeVisible();
    await expect(page.getByText('Можно выбрать несколько дней')).toBeVisible();
    // Без выбора кнопка Далее недоступна (валидация ветки).
    await expect(page.getByRole('button', { name: 'Далее' })).toBeDisabled();
    await page.getByRole('button', { name: 'Вт', exact: true }).click();
    await page.getByRole('button', { name: 'Пт', exact: true }).click();
    await page.getByRole('button', { name: 'Далее' }).click();

    await expect(page.getByRole('heading', { name: 'Окончание платежа' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('800');
    await page.getByRole('button', { name: 'Расход' }).click();
    await page.getByRole('button', { name: 'Перевод' }).click();
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.recurrence).toStrictEqual({ kind: 'weekly', weekdays: [2, 5] });
  });

  test('ветка месяца: «Последний день месяца» прижимается через деньOfMonth=31', async ({
    page,
    seededUser,
  }) => {
    const title = 'E2E взнос последнего дня';
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, title);

    await page.getByRole('button', { name: 'Каждый месяц' }).click();
    await page.getByRole('button', { name: 'Последний день месяца' }).click();
    await page.getByRole('button', { name: 'Далее' }).click();

    await expect(page.getByRole('heading', { name: 'Окончание платежа' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('40000');
    await page.getByRole('button', { name: 'Расход' }).click();
    await page.getByRole('button', { name: 'Перевод' }).click();
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);
    // Прижатие читается в описании успеха канонической меткой периодичности.
    await expect(page.getByText(/далее последний день каждого месяца/)).toBeVisible();

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.recurrence).toStrictEqual({ kind: 'monthly', dayOfMonth: 31 });
  });

  test('ежедневная периодичность завершает шаг сразу, без ветки дат', async ({
    page,
    seededUser,
  }) => {
    const title = 'E2E парковка ежедневно';
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, title);

    await page.getByRole('button', { name: 'Каждый день' }).click();
    // Ветка не открылась — сразу шаг окончания.
    await expect(page.getByRole('heading', { name: 'Окончание платежа' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();

    await page.getByRole('textbox', { name: 'Сумма' }).fill('200');
    await page.getByRole('button', { name: 'Расход' }).click();
    await page.getByRole('button', { name: 'Наличные' }).click();
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);
    await expect(page.getByText(/далее ежедневно/)).toBeVisible();

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.recurrence).toStrictEqual({ kind: 'daily' });
  });

  test('ежегодная ветка: месяц по умолчанию текущий, выбирается день', async ({
    page,
    seededUser,
  }) => {
    const title = 'E2E налог раз в год';
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, title);

    await page.getByRole('button', { name: 'Каждый год' }).click();
    await expect(page.getByRole('heading', { name: 'Выберите месяц и день' })).toBeVisible();
    // Месяц остаётся текущим (дефолт от «сегодня»); выбираем 13-е.
    await page.getByRole('button', { name: '13', exact: true }).first().click();
    await page.getByRole('button', { name: 'Далее' }).click();

    await expect(page.getByRole('heading', { name: 'Окончание платежа' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('5000');
    await page.getByRole('button', { name: 'Расход' }).click();
    await page.getByRole('button', { name: 'Перевод' }).click();
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.recurrence.kind).toBe('yearly');
    expect(created?.recurrence.day).toBe(13);
    const todayMonth = Number(new Date().toISOString().slice(5, 7));
    expect(created?.recurrence.month).toBe(todayMonth);
  });

  test('окончание платежа задается датой из календаря и попадает в контракт', async ({
    page,
    seededUser,
  }) => {
    const title = 'E2E аренда с окончанием';
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, title);

    await page.getByRole('button', { name: 'Каждый месяц' }).click();
    await page.getByRole('button', { name: '15', exact: true }).first().click();
    await page.getByRole('button', { name: 'Далее' }).click();

    // Шаг 4: разворачиваем календарь и берём последний включённый день
    // последнего отрисованного месяца — он заведомо в будущем, каким бы ни
    // было сегодняшнее число (вертикальный календарь подгружает месяцы).
    await page.getByRole('button', { name: 'Выбрать дату' }).click();
    await expect(page.getByText(/\d{4}/).first()).toBeVisible();
    const enabledDay = page.locator('button:enabled').filter({ hasText: /^\d{1,2}$/ });
    await enabledDay.last().click();
    await expect(page.getByRole('button', { name: 'Убрать дату' })).toBeVisible();

    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('15000');
    await page.getByRole('button', { name: 'Доход' }).click();
    await page.getByRole('button', { name: 'Перевод' }).click();
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.endDate).toBeDefined();
    expect(typeof created?.endDate).toBe('string');
  });

  test('черновик переживает перезагрузку: поля шага суммы восстановлены', async ({
    page,
    seededUser,
  }) => {
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, 'E2E черновик после перезагрузки');

    await page.getByRole('button', { name: 'Каждую неделю' }).click();
    await page.getByRole('button', { name: 'Ср', exact: true }).click();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('button', { name: 'Далее' }).click();

    // Сумма введена, но чипы не выбраны — дальше не пускает.
    await page.getByRole('textbox', { name: 'Сумма' }).fill('1234');
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeDisabled();

    await page.reload();

    // Восстановление на первом незавершённом шаге — шаг 5 с суммой из черновика.
    await expect(page.getByRole('heading', { name: 'Сумма платежа' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Сумма' })).toHaveValue(/1\s?234/);
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeDisabled();
    await page.getByRole('button', { name: 'Доход' }).click();
    await page.getByRole('button', { name: 'Наличные' }).click();
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeEnabled();
  });

  test('автоплатёж создаётся с флагом autoPay и заголовком успеха про автоплатёж', async ({
    page,
    seededUser,
  }) => {
    const title = 'E2E автоплатеж коммуналки';
    await openWizard(page, seededUser, 'autopayment');
    await selectCategory(page);
    await passTitleStep(page, title);

    await page.getByRole('button', { name: 'Каждый месяц' }).click();
    await page.getByRole('button', { name: '5', exact: true }).first().click();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('button', { name: 'Далее' }).click();

    await page.getByRole('textbox', { name: 'Сумма' }).fill('3300');
    await page.getByRole('button', { name: 'Расход' }).click();
    await page.getByRole('button', { name: 'Перевод' }).click();
    await page.getByRole('button', { name: 'Создать автоплатеж' }).click();

    await expect(page.getByRole('heading', { name: /Вы создали автоплатеж/ })).toContainText(`«${title}»`);
    await expect(page.getByText(/Платеж пополнится сам/)).toBeVisible();

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.autoPay).toBe(true);
    expect(created?.recurrence.dayOfMonth).toBe(5);
  });
});

test.describe('визард создания платежа — десктоп', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('колонка 560 внутри глобального хрома, тот же поток шагов; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openWizard(page, seededUser);
    await expect(page.getByRole('heading', { name: 'Рентли' })).toBeHidden();
    await captureScreen(page, testInfo, 'wizard-desktop-top');

    await selectCategory(page);
    await passTitleStep(page);
    await page.getByRole('button', { name: 'Каждую неделю' }).click();
    await captureScreen(page, testInfo, 'wizard-desktop-weekdays');
    await page.getByRole('button', { name: 'Пн', exact: true }).click();
    await page.getByRole('button', { name: 'Далее' }).click();
    await expect(page.getByRole('heading', { name: 'Окончание платежа' })).toBeVisible();
  });
});
