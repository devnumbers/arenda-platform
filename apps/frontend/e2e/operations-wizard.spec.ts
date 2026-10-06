import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';

// Визард создания операции (#570), шаг 1 — сумма и направление (тикет
// #1007, карта #1005; макеты 1858:104557/105397 мобилка, 2913:69551
// широкий; дополнение 01.10 — дисплейный ярус расширен на планшет):
// денежное поле двумя ярусами — <1024 (мобилка и планшет) дисплей «0 ₽»
// 44/48 (AmountField), ≥1024 (ПК) бокс «Сумма» 56px Title In; сегмент
// «Расход/Доход» 232px на дисплейном ярусе и во всю колонку на ПК. До
// сабмита визард ничего не пишет на сервер, состояние шагов —
// клиентский useState (черновика у операции нет, карта #1052 Q2=В) —
// спека данных не создаёт. Шаг 3 «Категория» — хедер H1/600 по спеке
// research #1149 (Figma 1049:34768, тикет #1152).

/** Глобальный вход в визард (шаг «Выбрать объект» — четвёртый, шаг 1
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

    // Один input «Сумма» в дереве доступности: бокс яруса ПК виден,
    // дисплейный скрыт display:none (канон WizardAmountField).
    const amount = page.getByRole('textbox', { name: 'Сумма' });
    await expect(amount).toHaveCount(1);
    // Ярус опознаётся по строке значения: бокс Title In — 16px (дисплей —
    // 44px), лейбл «Сумма» плавает внутри бокса.
    await expect(amount).toHaveCSS('font-size', '16px');
    await expect(page.getByText('Сумма', { exact: true })).toBeVisible();

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
    // px-6 шага (560 − 48 = 512), а не дисплейные 232px.
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

    // Сумма и направление едут в шаг 2 и возвращаются «Назад» (состояние
    // шагов живёт в useState, пока смонтирован поток).
    await page.getByRole('radio', { name: 'Доход' }).click();
    await next.click();
    await expect(page.getByText('Операция', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(amount).toHaveValue(/2\s?500/);
    await expect(page.getByRole('radio', { name: 'Доход' })).toBeChecked();

    await captureScreen(page, testInfo, 'operation-wizard-step1-amount-desktop');
  });

  test('<1024 (мобилка и планшет): дисплей 44/48 по центру, сегмент 232px', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.setViewportSize({ width: 393, height: 852 });
    await page.goto(WIZARD_URL);
    await expect(page.getByText('Добавить операцию')).toBeVisible();

    // Один input «Сумма» — дисплейный ярус; бокс скрыт display:none.
    const amount = page.getByRole('textbox', { name: 'Сумма' });
    await expect(amount).toHaveCount(1);
    await expect(amount).toHaveCSS('font-size', '44px');

    // Сегмент 232px (Figma 1858:105404) и переключение направления.
    const segment = page.getByRole('radiogroup', { name: 'Направление операции' });
    const segmentBox = await segment.boundingBox();
    expect(segmentBox?.width).toBeCloseTo(232);
    await page.getByRole('radio', { name: 'Доход' }).click();
    await expect(page.getByRole('radio', { name: 'Доход' })).toBeChecked();

    // Ввод через скрытый focusable input дисплея: группировка разрядов
    // на дисплее (макет 1858:105403 «6 000 ₽») и гейт кнопки.
    await amount.fill('6000');
    await expect(amount).toHaveValue(/6\s?000/);
    await expect(page.getByRole('button', { name: 'Продолжить' })).toBeEnabled();

    await captureScreen(page, testInfo, 'operation-wizard-step1-amount-mobile');

    // Планшет 768–1023 — тот же дисплейный ярус (дополнение карты 01.10:
    // макеты 1858:104557/105397.mobile), блок один в один с мобилкой.
    await page.setViewportSize({ width: 800, height: 1024 });
    await expect(amount).toHaveCSS('font-size', '44px');
    const tabletBox = await segment.boundingBox();
    expect(tabletBox?.width).toBeCloseTo(232);
    await captureScreen(page, testInfo, 'operation-wizard-step1-amount-tablet');
  });
});

test.describe('визард операции — шаг категории (#1152)', () => {
  test('хедер H1 «Выберите категорию операции», поиск в шапке, кнопка после выбора', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(WIZARD_URL);
    await expect(page.getByText('Добавить операцию')).toBeVisible();

    // Шаги 1–2: сумма (гейт кнопки) и название (необязательно — «Продолжить»
    // доступно сразу).
    await page.getByRole('textbox', { name: 'Сумма' }).fill('2500');
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await expect(page.getByRole('heading', { name: 'Что хотите добавить?' })).toBeVisible();
    await page.getByRole('button', { name: 'Продолжить' }).click();

    // Хедер шага категории — H1/600 без подзаголовка («Без описания»,
    // спека research #1149); в шапке — название шага «Категория».
    await expect(
      page.getByRole('heading', { name: 'Выберите категорию операции' }),
    ).toBeVisible();
    await expect(page.getByText('Категория', { exact: true })).toBeVisible();

    // Кнопки шага до выбора категории нет (канон шага платежа) — панель
    // скрыта целиком.
    await expect(page.getByRole('button', { name: 'Продолжить' })).toHaveCount(0);

    // Лупа меняет название шага на поле поиска; пустой запрос — подсказка
    // вместо списка (Figma 1049:46256).
    await page.getByRole('button', { name: 'Поиск по категориям' }).click();
    const search = page.getByRole('searchbox', { name: 'Поиск по названиям категорий' });
    await expect(search).toBeVisible();
    await expect(page.getByText('Начните искать категорию')).toBeVisible();

    // С запросом — отфильтрованный список; крестик возвращает шаг.
    await search.fill('интер');
    await expect(page.getByRole('button', { name: 'Интернет', exact: true })).toBeVisible();
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

  test('вход с объекта — тот же хедер H1 шага категории, сабмит-кнопка после выбора', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}/operations/new`);
    await expect(page.getByText('Добавить операцию')).toBeVisible();

    // Шаги 1–2 до категории.
    await page.getByRole('textbox', { name: 'Сумма' }).fill('1200');
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Продолжить' }).click();

    // Компонент шага общий — заголовок одинаков у обоих входов (#1152).
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
