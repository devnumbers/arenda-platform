import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  passOperationDateStep,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';

// Визард создания операции (#570), шаг 1 — сумма и направление (тикет
// #1007; макет 2913:69551, решение владельца 2026-10-22 — единый ярус
// вместо двухъярусной карты #1005): бокс «Сумма» 56px Title In на всех
// ширинах (мобилка/планшет/ПК), на ПК ≥1024 поле активно при маунте;
// сегмент «Расход/Доход» во всю колонку с обрезанной треком тенью
// (2913:69741). До
// сабмита визард ничего не пишет на сервер, состояние шагов —
// клиентский useState (черновика у операции нет, карта #1052 Q2=В) —
// спека данных не создаёт. Шаг 3 «Категория» — H1 «Выберите категорию
// операции» без подзаголовка, в режиме поиска заголовка нет: паритет
// с шагом категории платежа #1152 (решение владельца 13.10). С 13.10
// между суммой и названием — шаг даты: экран целиком CalendarDatePicker
// (#1394), проход через него — «Продолжить» пикера с предвыбранным
// «сегодня владельца» (дата задним числом, карта #1390).

/** Глобальный вход в визард (шаг «Выбрать объект» — пятый, шаг 1
 * суммы доступен сразу). */
const WIZARD_URL = '/operations/new';

test.describe('визард операции — шаг суммы', () => {
  test('≥1024 (ПК): бокс «Сумма», дисплей скрыт, сегмент во всю колонку; гейт кнопки и круговой маршрут', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(WIZARD_URL);
    await expect(page.getByText('Добавить операцию')).toBeVisible();

    // Один input «Сумма» — единый ярус (бокс Title In на всех ширинах).
    const amount = page.getByRole('textbox', { name: 'Сумма' });
    await expect(amount).toHaveCount(1);
    // Бокс Title In — строка значения 16px, лейбл «Сумма» плавает внутри.
    await expect(amount).toHaveCSS('font-size', '16px');
    await expect(page.getByText('Сумма', { exact: true })).toBeVisible();
    // ПК: поле активно сразу при заходе (решение владельца 2026-10-22,
    // разворот п.4 #1151).
    await expect(amount).toBeFocused();

    const segment = page.getByRole('radiogroup', { name: 'Направление операции' });
    await expect(segment).toBeVisible();

    // Кнопка неактивна без суммы (Figma 1858:104557), сумма с группировкой
    // разрядов её включает.
    const next = page.getByRole('button', { name: 'Продолжить' });
    await expect(next).toBeDisabled();
    await amount.fill('2500');
    await expect(amount).toHaveValue(/2\s?500/);
    await expect(next).toBeEnabled();

    // Широкий макет 2913:69553: сегмент — вся ширина колонки минус
    // px-6 шага (560 − 48 = 512).
    const widths = await page.evaluate(() => {
      const column = document.querySelector('.max-w-column');
      const group = document.querySelector(
        '[role="radiogroup"][aria-label="Направление операции"]',
      );
      return {
        column: column?.getBoundingClientRect().width ?? 0,
        segment: group?.getBoundingClientRect().width ?? 0,
      };
    });
    expect(widths.column).toBeGreaterThan(500);
    expect(Math.abs(widths.column - 48 - widths.segment)).toBeLessThan(1);

    // Сумма и направление едут в шаг 2 (экран даты) и возвращаются
    // «Назад» пикера (состояние шагов живёт в useState, пока смонтирован
    // поток); «Продолжить» пикера коммитит предвыбранное «сегодня».
    await page.getByRole('radio', { name: 'Доход' }).click();
    await next.click();
    const calendar = page.getByRole('dialog', { name: 'Выберите дату' });
    await expect(calendar).toBeVisible();
    await calendar.getByRole('button', { name: 'Назад' }).click();
    await expect(amount).toHaveValue(/2\s?500/);
    await expect(page.getByRole('radio', { name: 'Доход' })).toBeChecked();

    await next.click();
    await passOperationDateStep(page);
    await expect(page.getByText('Операция', { exact: true })).toBeVisible();

    await captureScreen(page, testInfo, 'operation-wizard-step1-amount-desktop');
  });

  test('<1024 (мобилка и планшет): тот же бокс «Сумма», сегмент во всю ширину — единый ярус', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.setViewportSize({ width: 393, height: 852 });
    await page.goto(WIZARD_URL);
    await expect(page.getByText('Добавить операцию')).toBeVisible();

    // Один input «Сумма» — единый ярус: тот же бокс Title In, что на ПК
    // (макет 2913:69551 «Mobile»; на мобилке вне жеста фокуса нет —
    // iOS клавиатуру не поднимает, research #1148 §A).
    const amount = page.getByRole('textbox', { name: 'Сумма' });
    await expect(amount).toHaveCount(1);
    await expect(amount).toHaveCSS('font-size', '16px');
    // Негатив контракта фокуса: маунт вне жеста не фокусирует поле.
    await expect(amount).not.toBeFocused();

    // Сегмент во всю ширину колонки (макет 2913:69741; 393 − px-6 = 345).
    const segment = page.getByRole('radiogroup', { name: 'Направление операции' });
    const segmentBox = await segment.boundingBox();
    expect(segmentBox?.width).toBeCloseTo(345);
    await page.getByRole('radio', { name: 'Доход' }).click();
    await expect(page.getByRole('radio', { name: 'Доход' })).toBeChecked();

    // Ввод в бокс: группировка разрядов и гейт кнопки.
    await amount.fill('6000');
    await expect(amount).toHaveValue(/6\s?000/);
    await expect(page.getByRole('button', { name: 'Продолжить' })).toBeEnabled();

    await captureScreen(page, testInfo, 'operation-wizard-step1-amount-mobile');

    // Планшет 768–1023 — тот же единый ярус; колонка капится 560,
    // сегмент = 560 − px-6 = 512 (проверено живой приёмкой).
    await page.setViewportSize({ width: 800, height: 1024 });
    await expect(amount).toHaveCSS('font-size', '16px');
    const tabletBox = await segment.boundingBox();
    expect(tabletBox?.width).toBeCloseTo(512);
    await captureScreen(page, testInfo, 'operation-wizard-step1-amount-tablet');
  });
});

test.describe('визард операции — шаг категории', () => {
  test('заголовок H1 «Выберите категорию операции», поиск без заголовка, кнопка после выбора', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(WIZARD_URL);
    await expect(page.getByText('Добавить операцию')).toBeVisible();

    // Шаги 1–3: сумма (гейт кнопки), дата («Продолжить» пикера) и
    // название (необязательно — «Продолжить» доступно сразу).
    await page.getByRole('textbox', { name: 'Сумма' }).fill('2500');
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await passOperationDateStep(page);
    await expect(page.getByRole('heading', { name: 'Что хотите добавить?' })).toBeVisible();
    await page.getByRole('button', { name: 'Продолжить' }).click();

    // Заголовок шага — H1 «Выберите категорию операции» без подзаголовка
    // (паритет с шагом категории платежа #1152, решение владельца 13.10);
    // в шапке — название шага «Категория».
    await expect(
      page.getByRole('heading', { name: 'Выберите категорию операции' }),
    ).toBeVisible();
    await expect(page.getByText('Выберите категорию', { exact: true })).toHaveCount(0);
    await expect(page.getByText('Категория', { exact: true })).toBeVisible();

    // Кнопки шага до выбора категории нет (канон шага платежа) — панель
    // скрыта целиком.
    await expect(page.getByRole('button', { name: 'Продолжить' })).toHaveCount(0);

    // Лупа меняет название шага на поле поиска; пустой запрос — подсказка
    // вместо списка (Figma 1049:46256). В режиме поиска заголовка нет —
    // паритет с шагом категории платежа (Figma 1049:46418, решение
    // владельца 13.10).
    await page.getByRole('button', { name: 'Поиск по категориям' }).click();
    const search = page.getByRole('searchbox', { name: 'Поиск по названиям категорий' });
    await expect(search).toBeVisible();
    await expect(page.getByText('Начните искать категорию')).toBeVisible();
    await expect(page.getByRole('heading')).toHaveCount(0);

    // С запросом — отфильтрованный список, тоже без заголовка; крестик
    // возвращает шаг с заголовком.
    await search.fill('интер');
    await expect(page.getByRole('button', { name: 'Интернет', exact: true })).toBeVisible();
    await expect(page.getByRole('heading')).toHaveCount(0);
    await page.getByRole('button', { name: 'Очистить поиск' }).click();
    await expect(page.getByRole('searchbox')).toHaveCount(0);
    await expect(
      page.getByRole('heading', { name: 'Выберите категорию операции' }),
    ).toBeVisible();

    // Выбор категории открывает кнопку; у глобального входа она ведёт
    // на шаг выбора объекта. Дальше сабмита нет — сервер ничего не пишет.
    await page.getByRole('button', { name: 'Интернет', exact: true }).click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await expect(page.getByText('Выбрать объект', { exact: true })).toBeVisible();

    await captureScreen(page, testInfo, 'operation-wizard-step3-category');
  });

  test('вход с объекта — тот же заголовок шага категории, сабмит-кнопка после выбора', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}/operations/new`);
    await expect(page.getByText('Добавить операцию')).toBeVisible();

    // Шаги 1–3 до категории: сумма, дата (проход пикером), название.
    await page.getByRole('textbox', { name: 'Сумма' }).fill('1200');
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await passOperationDateStep(page);
    await page.getByRole('button', { name: 'Продолжить' }).click();

    // Компонент шага общий — заголовок одинаков у обоих входов.
    await expect(
      page.getByRole('heading', { name: 'Выберите категорию операции' }),
    ).toBeVisible();
    await expect(page.getByRole('button', { name: 'Добавить операцию' })).toHaveCount(0);

    // Категория — последний шаг входа с объекта: сабмит-кнопка появляется,
    // но не нажимается — сервер ничего не пишет.
    await page.getByRole('button', { name: 'Интернет', exact: true }).click();
    await expect(page.getByRole('button', { name: 'Добавить операцию' })).toBeVisible();
  });
});
