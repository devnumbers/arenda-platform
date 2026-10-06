import {
  captureScreen,
  expect,
  mockEmailCategoryShortcut,
  openCabinetWithSeededSession,
  pickCalendarDay,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';

// Визард создания платежа (#464): пять шагов на одном маршруте
// /properties/[id]/payments/new, все ветки периодичности (без «Один раз»),
// черновик в localStorage, создание через POST и экран успеха с датой
// первого вхождения. Шаг 5 — по макетам суммы карты #1005 (тикет #1008:
// 1858:104557/105397 мобилка, 2913:69551 широкий, чип формы оплаты
// снесён). Скриншоты — материал для сверки с Figma-фреймами #449
// (823:4243, 830:16721, 823:11219 + ветки дат, 843:8345, шаг 4:
// 1084-24863, 1056-54338).

const APARTMENT_PAYMENTS_URL = `/properties/${SEEDED_APARTMENT_PROPERTY_ID}/payments`;
const wizardUrl = (type: 'payment' | 'autopayment'): string =>
  `${APARTMENT_PAYMENTS_URL}/new?type=${type}`;

/** Родительные падежи месяцев — ожидаемые подписи formatDayMonth. */
const MONTH_GENITIVE = [
  'января', 'февраля', 'марта', 'апреля', 'мая', 'июня',
  'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря',
];

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
      .or(page.getByRole('heading', { name: 'Выберите категорию платежа' })),
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

/** Подтвердить черновик канонного календаря кнопкой — диалог закрывается
 * (точка отличия от до-канонных поверхностей). В ветке года визарда
 * кнопка «Продолжить» (решение владельца 30.09, #995 — подтверждение
 * сразу ведёт на следующий шаг), в остальных пикерах — канонное
 * «Выбрать». */
async function confirmCalendar(
  page: Parameters<typeof openCabinetWithSeededSession>[0],
  button = 'Выбрать',
): Promise<void> {
  await page
    .getByRole('dialog', { name: 'Выбрать дату' })
    .getByRole('button', { name: button, exact: true })
    .click();
}

type PaymentFromApi = {
  id: string;
  title: string;
  amountKopecks: number;
  type: string;
  autoPay: boolean;
  since: string;
  endDate?: string | null;
  reminderOffsetDays?: number | null;
  recurrence: { kind: string; weekdays?: number[]; daysOfMonth?: number[]; lastDay?: boolean; month?: number; day?: number };
  category?: { source: string; slug?: string | null; label: string };
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
    await expect(page.getByRole('heading', { name: 'Выберите категорию платежа' })).toBeVisible();
    await expect(page.getByRole('searchbox')).toHaveCount(0);

    // Клик вне хедера при пустом запросе тоже закрывает.
    await page.getByRole('button', { name: 'Поиск по категориям' }).click();
    await expect(page.getByText('Начните искать категорию')).toBeVisible();
    await page.mouse.click(195, 400);
    await expect(page.getByRole('heading', { name: 'Выберите категорию платежа' })).toBeVisible();
    await expect(page.getByRole('searchbox')).toHaveCount(0);
  });

  test('категория «Другое» замыкает пикер и проходит сквозь визард (#1069)', async ({
    page,
    seededUser,
  }) => {
    const title = 'E2E прочие расходы по дому';
    await openWizard(page, seededUser);

    // Шаг 1 — «Другое» — последняя строка списка каталога (1049:34832),
    // выбирается как любая категория.
    const rows = page.locator('div.flex.flex-col.pt-6').getByRole('button');
    await expect(rows.last()).toHaveAccessibleName('Другое');
    await rows.last().click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await expect(page.getByRole('heading', { name: 'Назовите платеж' })).toBeVisible();

    // Шаг 2–4 — имя, месяц 10-го, настройки по умолчанию.
    await page.getByRole('textbox').fill(title);
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Каждый месяц' }).click();
    await expect(page.getByRole('heading', { name: 'Выберите день', exact: true })).toBeVisible();
    await page.getByRole('button', { name: '10', exact: true }).first().click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();

    // Шаг 5 — сумма; категория нейтральна к направлению, берём расход.
    await page.getByRole('textbox', { name: 'Сумма' }).fill('1500');
    await page.getByRole('radio', { name: 'Расход' }).click();
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);

    // Контракт: слаг other доходит до API и возвращается в CategoryView —
    // метку рисует снапшот, иконку+цвет — каталог фронта.
    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created).toBeDefined();
    expect(created?.category?.source).toBe('default');
    expect(created?.category?.slug).toBe('other');
    expect(created?.category?.label).toBe('Другое');
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
    // Канонный грид MonthDaysGrid (#809): выбранная клетка несёт
    // aria-pressed, у каждой из 30 клеток имя — само число.
    await expect(page.getByRole('button', { name: '10', exact: true })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    await expect(page.getByRole('button', { name: '30', exact: true })).toHaveAttribute(
      'aria-pressed',
      'false',
    );
    await captureScreen(page, testInfo, 'wizard-step3-month-days-mobile');

    // Шаг 4 — напоминание и настройки необязательны: пропускаем.
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();

    // Шаг 5 — сумма и тип; кнопка заблокирована, пока сумма пуста.
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeDisabled();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('2500');
    // Шаг один в один с операционным (дополнение 01.10): сегмент
    // «Расход/Доход», «Форма оплаты» снесена; дефолт «Доход» подсвечен и
    // активирует кнопку сразу, клик по пилюле — явный выбор.
    await expect(page.getByRole('button', { name: 'Перевод' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeEnabled();
    await page.getByRole('radio', { name: 'Расход' }).click();
    await captureScreen(page, testInfo, 'wizard-step5-amount-mobile');
    await page.getByRole('button', { name: 'Создать платеж' }).click();

    // Экран успеха с первым вхождением из серверного ответа. Канон успеха
    // (1858:105544/105549): описание и сумма — отдельные узлы, сумма со
    // знаком (правка 6, a41617f9).
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(
      `«${title}»`,
    );
    await expect(page.getByText(/Первый платеж .*, далее каждый месяц 10 и 15 числа/)).toBeVisible();
    // «Страхование» — расход: канон знака направления «-2 500 ₽».
    await expect(page.getByText(`-2\u00A0500 ₽`)).toBeVisible();
    // Кнопка из фрейма 835:19893 — ведёт на страницу платежа.
    await expect(page.getByRole('button', { name: 'Посмотреть платеж' })).toBeVisible();
    await captureScreen(page, testInfo, 'wizard-success-mobile');

    // Контракт: платёж появился в API с серверным since и регулярностью.
    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created).toBeDefined();
    expect(created?.recurrence).toStrictEqual({ kind: 'monthly', daysOfMonth: [10, 15], lastDay: false });
    expect(created?.amountKopecks).toBe(250000);
    expect(created?.type).toBe('expense');
    expect(created?.autoPay).toBe(false);
    expect(created?.since).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(created?.endDate ?? null).toBeNull();
    // Напоминание опционально: без выбора в контракте напоминаний нет (#822).
    expect(created?.reminderOffsetDays ?? null).toBeNull();

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

    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('800');
    await page.getByRole('radio', { name: 'Расход' }).click();
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

    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('40000');
    await page.getByRole('radio', { name: 'Расход' }).click();
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
    // Ветка не открылась — сразу шаг «Настройте платеж».
    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();

    await page.getByRole('textbox', { name: 'Сумма' }).fill('200');
    await page.getByRole('radio', { name: 'Расход' }).click();
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
    // Канон (04.09): ветка года — бесконечный календарь; чип показывает
    // текущий месяц, прошлые дни недоступны.
    const monthName = new Date().toLocaleDateString('ru-RU', { month: 'long' });
    await expect(page.getByRole('button', { name: new RegExp(monthName, 'i') })).toBeVisible();
    const target = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000);
    await pickCalendarDay(page, target);
    await confirmCalendar(page, 'Продолжить');

    // Подтверждение сразу ведёт на шаг настроек — меню периодичности
    // пропускается (решение владельца 30.09, #995).
    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('5000');
    await page.getByRole('radio', { name: 'Расход' }).click();
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.recurrence.kind).toBe('yearly');
    expect(created?.recurrence.day).toBe(target.getDate());
    const targetMonth = target.getMonth() + 1;
    expect(created?.recurrence.month).toBe(targetMonth);
  });

  test('годовая ветка: готовый месяц сбрасывается без подтверждения календаря, подтверждение сразу ведёт на шаг 4 (#948)', async ({
    page,
    seededUser,
  }) => {
    const title = 'E2E годовой взнос вместо месяца';
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, title);

    // Готовый месяц: 15-е число; назад в меню — строки чистые, без
    // подписей значений (решение владельца 30.09, #995).
    await page.getByRole('button', { name: 'Каждый месяц' }).click();
    await page.getByRole('button', { name: '15', exact: true }).first().click();
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page.getByRole('heading', { name: 'Периодичность платежа' })).toBeVisible();
    // Строки меню — чистые имена видов, без подписей значений (решение
    // владельца 30.09, #995): exact-имя не совпадает, если в строке
    // появился суффикс «· …».
    for (const row of ['Каждый день', 'Каждую неделю', 'Каждый месяц', 'Каждый год']) {
      await expect(page.getByRole('button', { name: row, exact: true })).toBeVisible();
    }

    // «Каждый год» открывает календарь; выход без подтверждения сбрасывает
    // готовый месяц (дефект А): «Продолжить» со старым видом не показан.
    await page.getByRole('button', { name: 'Каждый год' }).click();
    const dialog = page.getByRole('dialog', { name: 'Выбрать дату' });
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', { name: 'Назад' }).click();
    await expect(dialog).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Продолжить' })).toHaveCount(0);

    // Повторный выбор годовой: «Продолжить» канона ветки пишет правило
    // и сразу ведёт на шаг настроек, минуя меню.
    await page.getByRole('button', { name: 'Каждый год' }).click();
    const target = new Date(Date.now() + 14 * 24 * 60 * 60 * 1000);
    await pickCalendarDay(page, target);
    await confirmCalendar(page, 'Продолжить');
    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();

    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('1200');
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.recurrence).toStrictEqual({
      kind: 'yearly',
      month: target.getMonth() + 1,
      day: target.getDate(),
    });
  });

  test('ежегодная ветка: дата в следующем году через пикер года (дыра «только текущий месяц»)', async ({
    page,
    seededUser,
  }) => {
    const title = 'E2E годовой взнос следующего года';
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, title);

    await page.getByRole('button', { name: 'Каждый год' }).click();
    const dialog = page.getByRole('dialog', { name: 'Выбрать дату' });
    await dialog.getByRole('button', { name: /\d{4}/ }).click();
    const year = new Date().getFullYear();
    const yearWheel = page.getByRole('listbox', { name: 'Год' });
    await yearWheel.getByText(String(year + 1)).click();
    await expect(yearWheel.getByRole('option', { selected: true })).toHaveText(String(year + 1));
    await page.getByRole('dialog', { name: 'Месяц и год' }).getByRole('button', { name: 'Выбрать' }).click();

    // Календарь прыгнул на текущий месяц следующего года; 10-е существует
    // в любом месяце, дни будущего года все доступны.
    const target = new Date(year + 1, new Date().getMonth(), 10);
    await pickCalendarDay(page, target);
    await confirmCalendar(page, 'Продолжить');
    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('2400');
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.recurrence).toStrictEqual({ kind: 'yearly', month: target.getMonth() + 1, day: 10 });
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
    // Чип открывает шит-пикер: колесо месяцев и годы от текущего. В
    // граничном году кольцо (#810) несёт только разрешённые месяцы от
    // текущего — «Сентябрь»-хардкод падал с октября, месяц в тесте сезонно-
    // независимый: «Декабрь» (последний разрешённый текущего года).
    await page.getByRole('dialog', { name: 'Выбрать дату' }).getByRole('button', { name: /\d{4}/ }).click();
    await expect(page.getByText('Отменить')).toBeVisible();
    await expect(page.getByText(String(new Date().getFullYear())).first()).toBeVisible();

    // Регресс «декабрь → январь даёт ноябрь»: выбранной остаётся прокрученная
    // строка, значение не уводит колесо к чужому ряду.
    await page.getByRole('listbox', { name: 'Месяц' }).getByText('Декабрь').first().click();
    await expect(
      page
        .getByRole('listbox', { name: 'Месяц' })
        .getByRole('option', { selected: true }),
    ).toHaveText('Декабрь');

    // Годы без верхней границы: упор в конец списка удлиняет его вперёд.
    // End ставит selected на текущий конец ленты; следующий End жмём только
    // после монтирования дорисованных строк: 2029 (текущий+3) → 2039 (+13),
    // лента после второго упора достаёт до +23 — год+20 виден.
    const yearWheel = page.getByRole('listbox', { name: 'Год' });
    const year = new Date().getFullYear();
    await yearWheel.press('End');
    await expect(yearWheel.getByRole('option', { selected: true })).toHaveText(String(year + 3));
    await expect(yearWheel.getByText(String(year + 13)).first()).toBeVisible();
    await yearWheel.press('End');
    await expect(yearWheel.getByRole('option', { selected: true })).toHaveText(String(year + 13));
    await expect(yearWheel.getByText(String(year + 23)).first()).toBeVisible();
    await expect(page.getByText(String(year + 20)).first()).toBeVisible();

    // Подтверждаем колёса: календарь прыгает на декабрь выбранного года.
    await page.getByRole('dialog', { name: 'Месяц и год' }).getByRole('button', { name: 'Выбрать' }).click();
    const jumpedYear = year + 13;

    // Календарь переключился на декабрь прыжка; выбираем 10-е — секция
    // именно этого месяца: в ленте остаются и прошлые месяцы с тем же днём.
    await pickCalendarDay(page, new Date(jumpedYear, 11, 10));
    await confirmCalendar(page, 'Продолжить');
    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('7000');
    await page.getByRole('radio', { name: 'Расход' }).click();
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(`«${title}»`);

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.recurrence.kind).toBe('yearly');
    expect(created?.recurrence.month).toBe(12);
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

    // Шаг 4: «Выбрать дату» открывает канонический бесконечный календарь;
    // черновик при открытии — сегодня (первый доступный день), «Выбрать»
    // его коммитит. Тап по выбранному дню снял бы выбор (канон снятия).
    await page.getByRole('button', { name: 'Выбрать дату' }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await confirmCalendar(page);
    // Модалка закрылась, дата вернулась на экран окончания; дата текущего
    // года рендерится без года (formatDayMonthWithYear), месяц — родительный.
    await expect(dialog).toHaveCount(0);
    const monthGenitive = MONTH_GENITIVE[new Date().getMonth()];
    const dayExpected = String(new Date().getDate());
    await expect(page.getByText(new RegExp(`^${dayExpected} ${monthGenitive}$`))).toBeVisible();

    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('15000');
    // Дефолт «Доход» совпадает с нужным типом — без кликов.
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
    // Мобильный ярус — дисплей 44/48 (канон WizardAmountField, карта #1005).
    await expect(field).toHaveCSS('font-size', '44px');
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

  test('успех без названия: заголовок без «названия», «Посмотреть платеж» ведёт на страницу', async ({
    page,
    seededUser,
  }) => {
    await openWizard(page, seededUser);
    // Название не вводим — заголовок успеха остаётся без «названия».
    await selectCategory(page);
    await passTitleStep(page);

    await page.getByRole('button', { name: 'Каждую неделю' }).click();
    await page.getByRole('button', { name: 'Понедельник', exact: true }).click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('900');
    await page.getByRole('button', { name: 'Создать платеж' }).click();

    await expect(page.getByRole('heading', { name: 'Вы создали платеж' })).toBeVisible();
    await expect(page.getByText(/Первый платеж .*, далее каждый понедельник/)).toBeVisible();
    await expect(page.getByText('+900 ₽')).toBeVisible();

    await page.getByRole('button', { name: 'Посмотреть платеж' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[^/]+$`));
  });

  test('правило одним днём: после оплаты — «Платеж завершен» сразу, не после endDate', async ({
    page,
    seededUser,
  }) => {
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, 'E2E правило одним днём');

    await page.getByRole('button', { name: 'Каждый день' }).click();
    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();
    // Окончание = сегодня: правило стартует сегодня, единственное вхождение
    // — сегодняшнее. Черновик канона при открытии уже сегодня, «Выбрать»
    // коммитит его; тап по выбранному дню снял бы выбор (канон снятия).
    await page.getByRole('button', { name: 'Выбрать дату' }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await confirmCalendar(page);
    await expect(dialog).toHaveCount(0);
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('600');
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toBeVisible();

    // Оплачиваем единственное вхождение со страницы операции — правило
    // завершено сразу же (серверный isCompleted), без ожидания
    // календарного endDate+1.
    await page.getByRole('button', { name: 'Посмотреть платеж' }).click();
    const pay = page.getByRole('button', { name: 'Оплатить' });
    await expect(pay).toBeEnabled();
    await pay.click();
    await expect(page).toHaveURL(new RegExp(`/operations/[0-9a-f-]+(\\?.*)?$`));
    await page.getByRole('button', { name: 'Отметить оплаченной' }).click();
    await expect(page.getByText('Платеж оплачен')).toBeVisible();
    await page.getByRole('button', { name: 'Хорошо', exact: true }).click();
    // Закрытие success возвращает на страницу платежа (#1072).
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+$`));
    // Данные страницы могут быть stale (staleTime 30с, прогрев #626) —
    // перезагрузка читает с сервера завершённое состояние.
    await page.reload();
    await expect(page.getByText('Платеж завершен')).toBeVisible();

    // И в графике — то же завершённое состояние вместо «ближайших» дат.
    // Плиток больше нет (#1073), у завершённого нет и строки ближайшего —
    // вход по прямому URL подэкрана.
    await page.goto(`${page.url()}/schedule`);
    await expect(page.getByText('Платеж завершен')).toBeVisible();
    await expect(page.getByText('Следующие')).toHaveCount(0);
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

    // Сумма введена — кнопка активна сразу: у типа дефолт «Доход».
    await page.getByRole('textbox', { name: 'Сумма' }).fill('1234');
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeEnabled();

    await page.reload();

    // Восстановление на шаге 5 с суммой из черновика; кнопка снова активна,
    // сегмент направления работает с восстановленным значением.
    await expect(page.getByRole('textbox', { name: 'Сумма' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Сумма' })).toHaveValue(/1\s?234/);
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeEnabled();
    await page.getByRole('radio', { name: 'Расход' }).click();
    await expect(page.getByRole('radio', { name: 'Расход' })).toBeChecked();
  });

  test('назад со шага 3 на шаг 2: перезагрузка возвращает на шаг 2, без перескока (#1055)', async ({
    page,
    seededUser,
  }) => {
    const title = 'E2E назад на шаг названия';
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, title);
    // Шаги 1 и 3 заполнены: без штампа пересчёт перепрыгнул бы оба
    // опциональных шага (2 и 4) сразу на шаг 5.
    await page.getByRole('button', { name: 'Каждую неделю' }).click();
    await page.getByRole('button', { name: 'Понедельник', exact: true }).click();

    // «Назад» на шаг названия — тоже переход: штамп шага 2 едет в черновик
    // наравне с переходами вперёд. Первый клик закрывает ветку дней
    // (осознанный квирк шага 3), второй уводит на шаг 2.
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page.getByRole('heading', { name: 'Периодичность платежа' })).toBeVisible();
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page.getByRole('heading', { name: 'Назовите платеж' })).toBeVisible();

    // Перезагрузка: точный сохранённый шаг 2 с названием, а не пересчёт
    // из заполненности (даёт шаг 5) и не шаг 3.
    await page.reload();
    await expect(page.getByRole('heading', { name: 'Назовите платеж' })).toBeVisible();
    await expect(page.getByRole('textbox')).toHaveValue(title);
    await expect(page.getByRole('heading', { name: 'Периодичность платежа' })).toHaveCount(0);
    await expect(page.getByRole('textbox', { name: 'Сумма' })).toHaveCount(0);
  });

  test('напоминание: «За 3 дня» переживает перезагрузку и попадает в контракт', async ({
    page,
    seededUser,
  }, testInfo) => {
    const title = 'E2E платеж с напоминанием';
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, title);

    await page.getByRole('button', { name: 'Каждый месяц' }).click();
    await page.getByRole('button', { name: '15', exact: true }).first().click();
    await page.getByRole('button', { name: 'Продолжить' }).click();

    // Шаг 4, ручная ветка: радио «за N дней», ниже — «Настройки платежа»
    // с окончанием и почтой (Figma 1084-24863).
    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'За 1 день' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'За 3 дня' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'За 7 дней' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Настройки платежа' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Выбрать дату' })).toBeVisible();
    await captureScreen(page, testInfo, 'wizard-step4-reminder-manual-mobile');
    await page.getByRole('button', { name: 'За 3 дня' }).click();

    // Черновик с напоминанием переживает перезагрузку: resume на точном
    // шаге 4 (штамп перехода, #1055; раньше пересчёт перепрыгивал опциональный
    // шаг на шаг 5), значение доезжает до контракта.
    await page.reload();
    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();
    await expect(page.getByRole('textbox', { name: 'Сумма' })).toBeVisible();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('1500');
    await page.getByRole('button', { name: 'Создать платеж' }).click();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toBeVisible();

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.reminderOffsetDays).toBe(3);
  });

  test('тумблер «Уведомления на почту» — шоткат глобальной настройки категории «Платежи и операции»', async ({
    page,
    seededUser,
  }) => {
    // Email-матрица аккаунта (#743): тумблер шага отражает её значение,
    // клик пишет PUT с флипом категории — пер-платёжных override нет (#822).
    // Мок stateful: refetch после PUT возвращает сохранённое состояние.
    const { savedCategories } = await mockEmailCategoryShortcut(page);

    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page, 'E2E шоткат почты');
    // Ежедневная ветка завершает шаг 3 сама — клик ведёт сразу на шаг 4.
    await page.getByRole('button', { name: 'Каждый день' }).click();
    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();

    const toggle = page.getByRole('switch', { name: 'Уведомления на почту' });
    await expect(toggle).toHaveAttribute('aria-checked', 'true');
    await toggle.click();
    // Оптимистичный флип и PUT — трекер запросов только через expect.poll.
    await expect(toggle).toHaveAttribute('aria-checked', 'false');
    await expect
      .poll(() => savedCategories()?.payments_operations, { timeout: 5_000 })
      .toBe(false);
  });

  test('автоплатёж создаётся с флагом autoPay и заголовком успеха про автоплатёж', async ({
    page,
    seededUser,
  }, testInfo) => {
    const title = 'E2E автоплатеж коммуналки';
    await openWizard(page, seededUser, 'autopayment');
    await selectCategory(page);
    await passTitleStep(page, title);

    await page.getByRole('button', { name: 'Каждый месяц' }).click();
    await page.getByRole('button', { name: '5', exact: true }).first().click();
    await page.getByRole('button', { name: 'Продолжить' }).click();

    // Шаг 4, ветка автоплатежа: карточка «Уведомления об оплате» вместо
    // радио, «Окончание платежа» и тумблер почты (Figma 1056-54338).
    await expect(page.getByRole('heading', { name: 'Уведомления об оплате' })).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Окончание платежа' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'За 1 день' })).toHaveCount(0);
    await captureScreen(page, testInfo, 'wizard-step4-autopayment-mobile');
    await page.getByRole('button', { name: 'Далее' }).click();

    await page.getByRole('textbox', { name: 'Сумма' }).fill('3300');
    await page.getByRole('radio', { name: 'Расход' }).click();
    await page.getByRole('button', { name: 'Создать автоплатеж' }).click();

    await expect(page.getByRole('heading', { name: /Вы создали автоплатеж/ })).toContainText(`«${title}»`);
    await expect(page.getByText(/Платеж пополнится сам/)).toBeVisible();

    const items = await fetchPayments(page);
    const created = items.find((payment) => payment.title === title);
    expect(created?.autoPay).toBe(true);
    expect(created?.recurrence.daysOfMonth).toStrictEqual([5]);
    // Радио в ветке автоплатежа нет — напоминание не выбирается (#822).
    expect(created?.reminderOffsetDays ?? null).toBeNull();
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
    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();
  });

  // Шаг 5 на ПК ≥1024 (тикет #1008, макет 2913:69551): бокс «Сумма» 56px
  // Title In вместо дисплейного яруса; сегмент направления один в один с
  // операцией, «Форма оплаты» снесена.
  test('шаг суммы ≥1024 (ПК): бокс «Сумма», сегмент направления; гейт кнопки', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page);
    await page.getByRole('button', { name: 'Каждую неделю' }).click();
    await page.getByRole('button', { name: 'Понедельник', exact: true }).click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await expect(page.getByRole('heading', { name: 'Настройте платеж' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();

    // Один input «Сумма» в дереве доступности: бокс яруса ПК виден,
    // дисплейный скрыт display:none (канон WizardAmountField); ярус
    // опознаётся по 16px бокса (дисплей — 44px).
    const amount = page.getByRole('textbox', { name: 'Сумма' });
    await expect(amount).toHaveCount(1);
    await expect(amount).toHaveCSS('font-size', '16px');

    // Сегмент направления без формы оплаты: пилюли «Расход/Доход»,
    // дефолт «Доход» подсвечен.
    await expect(page.getByRole('button', { name: 'Перевод' })).toHaveCount(0);
    await expect(page.getByRole('radio', { name: 'Доход' })).toBeVisible();
    await expect(page.getByRole('radio', { name: 'Доход' })).toBeChecked();

    // Гейт кнопки: без суммы заблокирована, группированная сумма включает.
    const submit = page.getByRole('button', { name: 'Создать платеж' });
    await expect(submit).toBeDisabled();
    await amount.fill('2500');
    await expect(amount).toHaveValue(/2\s?500/);
    await expect(submit).toBeEnabled();

    await captureScreen(page, testInfo, 'wizard-step5-amount-desktop');
  });

  // Дополнение карты 01.10: дисплейный ярус — вся мобилка и планшет
  // (<1024), бокс остался только на ПК.
  test('шаг суммы на планшете 768–1023: дисплейный ярус, сегмент 232px', async ({
    page,
    seededUser,
  }) => {
    await page.setViewportSize({ width: 800, height: 1024 });
    await openWizard(page, seededUser);
    await selectCategory(page);
    await passTitleStep(page);
    await page.getByRole('button', { name: 'Каждую неделю' }).click();
    await page.getByRole('button', { name: 'Понедельник', exact: true }).click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Далее' }).click();

    // Дисплейный ярус на планшете — тот же блок, что на мобилке: 44px
    // дисплей, сегмент 232px, бокс скрыт.
    const amount = page.getByRole('textbox', { name: 'Сумма' });
    await expect(amount).toHaveCount(1);
    await expect(amount).toHaveCSS('font-size', '44px');
    const segment = page.getByRole('radiogroup', { name: 'Направление платежа' });
    const segmentBox = await segment.boundingBox();
    expect(segmentBox?.width).toBeCloseTo(232);
    await expect(page.getByRole('radio', { name: 'Доход' })).toBeChecked();
  });
});
