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
  recurrence: { kind: string; weekdays?: number[]; daysOfMonth?: number[]; lastDay?: boolean; month?: number; day?: number };
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
    await page.getByRole('button', { name: '15', exact: true }).first().click();
    await captureScreen(page, testInfo, 'wizard-step3-month-days-mobile');

    // Шаг 4 — окончание необязательно: пропускаем.
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await expect(page.getByRole('heading', { name: 'Окончание платежа' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();

    // Шаг 5 — сумма и признаки; кнопка заблокирована, пока сумма пуста.
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeDisabled();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('2500');
    // Дефолт «Доход/Перевод» активирует кнопку сразу; клик по чипу меняет
    // значение на альтернативное: доход → расход, перевод → наличные.
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeEnabled();
    await page.getByRole('button', { name: 'Доход' }).click();
    await page.getByRole('button', { name: 'Перевод' }).click();
    await captureScreen(page, testInfo, 'wizard-step5-amount-mobile');
    await page.getByRole('button', { name: 'Создать платеж' }).click();

    // Экран успеха с первым вхождением из серверного ответа.
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(
      `«${title}»`,
    );
    await expect(page.getByText(/Первый платеж .* на 2\u00A0500 ₽, далее каждый месяц 10 и 15 числа/)).toBeVisible();
    await expect(page.getByText('Посмотреть платеж')).toHaveCount(0);
    await captureScreen(page, testInfo, 'wizard-success-mobile');

    // Контракт: платёж появился в API с серверным since и регулярностью.
    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created).toBeDefined();
    expect(created?.recurrence).toStrictEqual({ kind: 'monthly', daysOfMonth: [10, 15], lastDay: false });
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
    // В хедере — подпись типа периода вместо чипа шага.
    await expect(page.getByText('Каждую неделю')).toBeVisible();
    // Без выбора панель с кнопкой не показана (Figma 1056:52895).
    await expect(page.getByRole('button', { name: 'Продолжить' })).toHaveCount(0);
    await page.getByRole('button', { name: 'Вторник', exact: true }).click();
    await page.getByRole('button', { name: 'Пятница', exact: true }).click();
    await page.getByRole('button', { name: 'Продолжить' }).click();

    await expect(page.getByRole('heading', { name: 'Окончание платежа' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('800');
    await page.getByRole('button', { name: 'Доход' }).click(); // → «Расход», форма уже «Перевод»
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.recurrence).toStrictEqual({ kind: 'weekly', weekdays: [2, 5] });
  });

  test('ветка месяца: «Последний день месяца» — отдельный маркер lastDay', async ({
    page,
    seededUser,
  }) => {
    const title = 'E2E взнос последнего дня';
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, title);

    await page.getByRole('button', { name: 'Каждый месяц' }).click();
    await page.getByRole('button', { name: 'Последний день месяца' }).click();
    await page.getByRole('button', { name: 'Продолжить' }).click();

    await expect(page.getByRole('heading', { name: 'Окончание платежа' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('40000');
    await page.getByRole('button', { name: 'Доход' }).click(); // → «Расход», форма уже «Перевод»
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);
    // Прижатие читается в описании успеха канонической меткой периодичности.
    await expect(page.getByText(/далее последний день каждого месяца/)).toBeVisible();

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.recurrence).toStrictEqual({ kind: 'monthly', daysOfMonth: [], lastDay: true });
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
    await page.getByRole('button', { name: 'Доход' }).click(); // → «Расход»
    await page.getByRole('button', { name: 'Перевод' }).click(); // → «Наличные»
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
    // Месяц остаётся текущим (дефолт от «сегодня»), ничего не предвыбрано;
    // выбираем 13-е на календаре одного месяца.
    const monthName = new Date().toLocaleDateString('ru-RU', { month: 'long' });
    await expect(page.getByRole('button', { name: new RegExp(monthName, 'i') })).toBeVisible();
    await page.getByRole('button', { name: '13', exact: true }).first().click();
    await page.getByRole('button', { name: 'Продолжить' }).click();

    await expect(page.getByRole('heading', { name: 'Окончание платежа' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('5000');
    await page.getByRole('button', { name: 'Доход' }).click(); // → «Расход», форма уже «Перевод»
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.recurrence.kind).toBe('yearly');
    expect(created?.recurrence.day).toBe(13);
    const todayMonth = Number(new Date().toISOString().slice(5, 7));
    expect(created?.recurrence.month).toBe(todayMonth);
  });

  test('ежегодная ветка: пикер месяца и года — месяц меняется, год не раньше текущего', async ({
    page,
    seededUser,
  }) => {
    const title = 'E2E взнос по году';
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, title);

    await page.getByRole('button', { name: 'Каждый год' }).click();
    // Чип открывает шит-пикер: колесо месяцев (все 12) и годы от текущего.
    await page.getByRole('button', { name: /месяц год|\d{4}/i }).first().click();
    await expect(page.getByText('Отменить')).toBeVisible();
    await expect(page.getByText(String(new Date().getFullYear())).first()).toBeVisible();

    // Регресс «декабрь → январь даёт ноябрь»: выбранной остаётся прокрученная
    // строка, значение не уводит колесо к чужому ряду.
    await page.getByText('Сентябрь').first().click();
    await expect(
      page
        .getByRole('listbox', { name: 'Месяц' })
        .getByRole('option', { selected: true }),
    ).toHaveText('Сентябрь');

    // Годы без верхней границы: упор в конец списка удлиняет его вперёд.
    const yearWheel = page.getByRole('listbox', { name: 'Год' });
    await yearWheel.press('End');
    await yearWheel.press('End');
    await expect(
      page.getByText(String(new Date().getFullYear() + 20)).first(),
    ).toBeVisible();

    await page.getByRole('button', { name: 'Выбрать' }).click();

    // Календарь переключился на сентябрь; выбираем 10-е.
    await page.getByRole('button', { name: '10', exact: true }).first().click();
    await page.getByRole('button', { name: 'Продолжить' }).click();

    await expect(page.getByRole('heading', { name: 'Окончание платежа' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('7000');
    await page.getByRole('button', { name: 'Доход' }).click(); // → «Расход», форма уже «Перевод»
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.recurrence.kind).toBe('yearly');
    expect(created?.recurrence.month).toBe(9);
    expect(created?.recurrence.day).toBe(10);
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
    await page.getByRole('button', { name: 'Продолжить' }).click();

    // Шаг 4: «Выбрать дату» открывает модалку годового календаря;
    // прыжок на последний год — все дни заведомо в будущем.
    await page.getByRole('button', { name: 'Выбрать дату' }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    // Чип «Месяц Год» открывает колёса; год — не раньше текущего.
    await dialog.getByRole('button', { name: /\d{4}/ }).click();
    const yearWheel = page.getByRole('listbox', { name: 'Год' });
    await expect(yearWheel).toBeVisible();
    await yearWheel.press('End');
    await page.getByRole('button', { name: 'Выбрать' }).click();
    // Календарь переключился на далёкий год — все дни доступны.
    const enabledDay = dialog.locator('button:enabled').filter({ hasText: /^\d{1,2}$/ });
    await enabledDay.first().click();
    // Модалка закрылась, дата вернулась на экран окончания.
    await expect(dialog).toHaveCount(0);
    await expect(page.getByText(/\d{1,2} [а-я]+, \d{4}/)).toBeVisible();

    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('15000');
    // Дефолт «Доход/Перевод» совпадает с нужными признаками — без кликов.
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.endDate).toBeDefined();
    expect(typeof created?.endDate).toBe('string');
  });

  test('ввод суммы: набранное не подменяется, «,00» не дописывается само', async ({
    page,
    seededUser,
  }) => {
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, 'E2E ввод суммы без автокопеек');
    await page.getByRole('button', { name: 'Каждый день' }).click();
    await page.getByRole('button', { name: 'Далее' }).click();

    const field = page.getByRole('textbox', { name: 'Сумма' });
    await field.click();
    // Регресс «2» → «2,00»: первая цифра не дописывает копейки, ввод продолжается.
    await field.pressSequentially('2');
    await expect(field).toHaveValue(/^2$/);
    await field.pressSequentially('500');
    await expect(field).toHaveValue(/2\s?500$/);
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeEnabled();

    // Дробь появляется только от пользователя; после ухода из поля —
    // канонические два знака (правка владельца 2026-08-31).
    await field.fill('');
    await field.pressSequentially('1234,5');
    await expect(field).toHaveValue(/^1\s?234,5$/);
    await field.blur();
    await expect(field).toHaveValue(/^1\s?234,50$/);

    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toBeVisible();
    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === 'E2E ввод суммы без автокопеек');
    expect(created?.amountKopecks).toBe(123450);
  });

  test('черновик переживает перезагрузку: поля шага суммы восстановлены', async ({
    page,
    seededUser,
  }) => {
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, 'E2E черновик после перезагрузки');

    await page.getByRole('button', { name: 'Каждую неделю' }).click();
    await page.getByRole('button', { name: 'Среда', exact: true }).click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Далее' }).click();

    // Сумма введена — кнопка активна сразу: у признаков дефолт «Доход/Перевод».
    await page.getByRole('textbox', { name: 'Сумма' }).fill('1234');
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeEnabled();

    await page.reload();

    // Восстановление на шаге 5 с суммой из черновика; кнопка снова активна,
    // переключатель типа работает с восстановленным значением.
    await expect(page.getByRole('heading', { name: 'Сумма платежа' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Сумма' })).toHaveValue(/1\s?234/);
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeEnabled();
    await page.getByRole('button', { name: 'Доход' }).click();
    await expect(page.getByRole('button', { name: 'Расход' })).toBeVisible();
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
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Далее' }).click();

    await page.getByRole('textbox', { name: 'Сумма' }).fill('3300');
    await page.getByRole('button', { name: 'Доход' }).click(); // → «Расход», форма уже «Перевод»
    await page.getByRole('button', { name: 'Создать автоплатеж' }).click();

    await expect(page.getByRole('heading', { name: /Вы создали автоплатеж/ })).toContainText(`«${title}»`);
    await expect(page.getByText(/Платеж пополнится сам/)).toBeVisible();

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.autoPay).toBe(true);
    expect(created?.recurrence.daysOfMonth).toStrictEqual([5]);
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
    await page.getByRole('button', { name: 'Понедельник', exact: true }).click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await expect(page.getByRole('heading', { name: 'Окончание платежа' })).toBeVisible();
  });
});
