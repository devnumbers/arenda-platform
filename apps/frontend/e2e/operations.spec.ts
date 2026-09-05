import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  test,
} from './fixtures';

// Экраны «Операции объекта» и направлений (#474–#478): пустые состояния
// 1:1 с макетами — совсем пустой объект (1518-92899), пустой период
// главного списка (1510-77308), «Нет доходов»/«Нет трат» (1510-76177 и
// 1510-75650), пустой выбор категорий (1518-92530) — и сквозная
// навигация по флоу (главный → направления → поиск → фильтры → назад).
// Скриншоты — материал для сверки с Figma.

const APARTMENT_OPERATIONS_URL = `/properties/${SEEDED_APARTMENT_PROPERTY_ID}/operations`;
const GARAGE_OPERATIONS_URL = `/properties/${SEEDED_GARAGE_PROPERTY_ID}/operations`;

// Период без операций: сидовые оплаченные операции квартиры («Интернет»)
// идут с сегодняшнего дня назад, а другие сценарии создают расходы только
// в текущем месяце — февраль 2020-го пуст при любом порядке прогонов.
const EMPTY_PERIOD_QUERY = '?from=2020-02-01&to=2020-02-28';

const EMPTY_PERIOD_CAPTION = 'Операции не найдены. Попробуйте выбрать другой период';
const CATEGORIES_EMPTY_CAPTION = 'Категорий, по которым были операции в этот период не было. '
  + 'Попробуйте выбрать другой период';

// Названия месяцев в заголовках секций пикера периода («Сентябрь, 2026»).
const PICKER_MONTH_NAMES = [
  'Январь', 'Февраль', 'Март', 'Апрель', 'Май', 'Июнь',
  'Июль', 'Август', 'Сентябрь', 'Октябрь', 'Ноябрь', 'Декабрь',
] as const;

test.describe('экраны операций — пустые состояния', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('объект без операций — «Операций еще не было», без чипов, сводки и поиска; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(GARAGE_OPERATIONS_URL);

    await expect(page.getByText('Операций еще не было')).toBeVisible();
    await expect(
      page.getByText('Добавьте аренду, другие платежи и начните отмечать оплату'),
    ).toBeVisible();
    await expect(page.locator('img[src*="operations-empty"]')).toBeVisible();

    // Макет 1518-92899: вместо чипов, карточек и поиска — только заглушка.
    await expect(page.getByRole('button', { name: 'Поиск операций' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Открыть расходы объекта' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Открыть доходы объекта' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /Все категории/ })).toHaveCount(0);

    await captureScreen(page, testInfo, 'operations-never-had-mobile');
  });

  test('направления пустого объекта — «Нет доходов»/«Нет трат» с чипами; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);

    await page.goto(`${GARAGE_OPERATIONS_URL}/income`);
    await expect(page.getByText('Нет доходов', { exact: true })).toBeVisible();
    await expect(page.getByText(EMPTY_PERIOD_CAPTION)).toBeVisible();
    await expect(page.locator('img[src*="operations-empty"]')).toBeVisible();
    // Чипы и поиск остаются (макет 1510-76177).
    await expect(page.getByRole('button', { name: 'Поиск операций' })).toBeVisible();
    await expect(page.getByRole('button', { name: /Все категории/ })).toBeVisible();

    await page.goto(`${GARAGE_OPERATIONS_URL}/expense`);
    await expect(page.getByText('Нет трат', { exact: true })).toBeVisible();
    await expect(page.getByText(EMPTY_PERIOD_CAPTION)).toBeVisible();

    await captureScreen(page, testInfo, 'operations-type-empty-mobile');
  });

  test('выбор категорий на пустом объекте — папка и подпись, без «Выбрать»; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`${GARAGE_OPERATIONS_URL}/categories`);

    await expect(page.locator('img[src*="operations-categories"]')).toBeVisible();
    await expect(page.getByText(CATEGORIES_EMPTY_CAPTION)).toBeVisible();
    // Черновик пуст и фильтр не применён — панель «Выбрать» не показывается.
    await expect(page.getByRole('button', { name: 'Выбрать категории' })).toHaveCount(0);

    await captureScreen(page, testInfo, 'operations-categories-empty-mobile');
  });

  test('операции квартиры: сводка (доходы 0 ₽ серым), список по датам', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(APARTMENT_OPERATIONS_URL);

    // Обычный экран, не «Операций еще не было»: есть чипы, сводка и поиск.
    await expect(page.getByRole('button', { name: 'Поиск операций' })).toBeVisible();
    await expect(page.getByRole('button', { name: /Все категории/ })).toBeVisible();

    // Сидовые оплаченные операции — только расходы («Интернет»); доходов
    // нет ни в сиде, ни в других сценариях: карточка «Доходы» — 0 ₽.
    const incomeCard = page.getByRole('button', { name: 'Открыть доходы объекта' });
    await expect(incomeCard.getByText('0 ₽')).toBeVisible();
    const expenseCard = page.getByRole('button', { name: 'Открыть расходы объекта' });
    await expect(expenseCard.getByText(/₽/)).toBeVisible();

    await expect(page.getByText('Сегодня').first()).toBeVisible();
    await expect(page.getByText('Интернет').first()).toBeVisible();
    await expect(page.getByText('Операций еще не было')).toHaveCount(0);
  });

  test('направления квартиры: доходы пусты («Нет доходов»), расходы — со списком', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);

    await page.goto(`${APARTMENT_OPERATIONS_URL}/income`);
    await expect(page.getByText('Нет доходов', { exact: true })).toBeVisible();
    await expect(page.getByText(EMPTY_PERIOD_CAPTION)).toBeVisible();

    await page.goto(`${APARTMENT_OPERATIONS_URL}/expense`);
    await expect(page.getByText('Расходы объекта')).toBeVisible();
    await expect(page.getByText('Сегодня').first()).toBeVisible();
    await expect(page.getByText('Интернет').first()).toBeVisible();
  });

  test('пустой период главного списка — сводка по нулям и подпись; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`${APARTMENT_OPERATIONS_URL}${EMPTY_PERIOD_QUERY}`);

    // Пустой период, но не «еще не было» — операции в другие периоды есть.
    await expect(page.getByText(EMPTY_PERIOD_CAPTION)).toBeVisible();
    await expect(page.getByText('Операций еще не было')).toHaveCount(0);
    const expenseCard = page.getByRole('button', { name: 'Открыть расходы объекта' });
    const incomeCard = page.getByRole('button', { name: 'Открыть доходы объекта' });
    await expect(expenseCard.getByText('0 ₽')).toBeVisible();
    await expect(incomeCard.getByText('0 ₽')).toBeVisible();

    await captureScreen(page, testInfo, 'operations-empty-period-mobile');
  });

  test('пустой период направлений и категорий квартиры — по макетам', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);

    await page.goto(`${APARTMENT_OPERATIONS_URL}/expense${EMPTY_PERIOD_QUERY}`);
    await expect(page.getByText('Нет трат', { exact: true })).toBeVisible();
    await expect(page.getByText(EMPTY_PERIOD_CAPTION)).toBeVisible();

    await page.goto(`${APARTMENT_OPERATIONS_URL}/categories${EMPTY_PERIOD_QUERY}`);
    await expect(page.locator('img[src*="operations-categories"]')).toBeVisible();
    await expect(page.getByText(CATEGORIES_EMPTY_CAPTION)).toBeVisible();
  });
});

test.describe('экраны операций — сквозной флоу', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('главный → направления → поиск → период → категории → возврат с фильтром', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(APARTMENT_OPERATIONS_URL);

    // Карточка «Доходы» ведёт на экран направления, «Назад» возвращает.
    await page.getByRole('button', { name: 'Открыть доходы объекта' }).click();
    await expect(page).toHaveURL(new RegExp('/operations/income$'));
    await expect(page.getByText('Доходы объекта')).toBeVisible();
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(new RegExp('/operations$'));

    // Карточка «Расходы» — то же для второго направления.
    await page.getByRole('button', { name: 'Открыть расходы объекта' }).click();
    await expect(page).toHaveURL(new RegExp('/operations/expense$'));
    await expect(page.getByText('Расходы объекта')).toBeVisible();
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(new RegExp('/operations$'));

    // Иконка поиска ведёт на экран поиска, «Назад» возвращает.
    await page.getByRole('button', { name: 'Поиск операций' }).click();
    await expect(page).toHaveURL(new RegExp('/operations/search$'));
    await expect(page.getByRole('searchbox', { name: 'Найти операцию' })).toBeVisible();
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(new RegExp('/operations$'));

    // Чип периода открывает канонический пикер поверх списка; «Назад»
    // пикера закрывает его, адрес списка не меняется.
    await page.getByRole('button', { name: /\d{4}/ }).click();
    await expect(page.getByText('Выберите период')).toBeVisible();
    await page.getByRole('dialog').getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(new RegExp('/operations$'));

    // Чип категорий → выбор строки → «Выбрать» возвращает список с фильтром.
    await page.getByRole('button', { name: 'Все категории' }).click();
    await expect(page).toHaveURL(new RegExp('/operations/categories'));
    await page.getByRole('button', { name: /Интернет/ }).click();
    await page.getByRole('button', { name: 'Выбрать категории' }).click();
    await expect(page).toHaveURL(/\?category=internet$/);
    await expect(page.getByRole('button', { name: 'Интернет' }).first()).toBeVisible();
  });

  test('«Назад» ведёт на родительский экран, а не листает историю браузера', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(APARTMENT_OPERATIONS_URL);

    // Лейбл чипа периода зависит от применённого выбора («Сентябрь 2026»,
    // «5 — 10 сент.»…), поэтому берём его как соседа стабильного «Все
    // категории». Диапазон выбираем в прошлом месяце — все его дни
    // доступны при любом дне прогона.
    const periodChip = page
      .getByRole('button', { name: 'Все категории' })
      .locator('xpath=preceding-sibling::button[1]');

    const applyRange = async (days: readonly [string, string]): Promise<void> => {
      await periodChip.click();
      await expect(page.getByText('Выберите период')).toBeVisible();
      const previousMonth = new Date();
      previousMonth.setDate(1);
      previousMonth.setMonth(previousMonth.getMonth() - 1);
      const monthBlock = page.locator('section').filter({
        has: page.getByRole('heading', {
          name: `${PICKER_MONTH_NAMES[previousMonth.getMonth()]}, ${previousMonth.getFullYear()}`,
        }),
      });
      for (const day of days) {
        await monthBlock.getByRole('button', { name: day, exact: true }).click();
      }
      await page.getByRole('dialog').getByRole('button', { name: 'Выбрать', exact: true }).click();
      await expect(page).toHaveURL(/\?from=/);
    };

    // Период применён дважды: каждое применение replace-ит запись списка,
    // поэтому в истории браузера лежат прежние периоды списка.
    await applyRange(['5', '10']);
    await applyRange(['5', '15']);

    // «Назад» — структурный переход на объект, а не на прошлый период.
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}$`));

    // С направления «Назад» — на главный список операций, без query.
    await page.goto(`${APARTMENT_OPERATIONS_URL}/expense`);
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(new RegExp('/operations$'));
  });

  test('смена периода не подменяет страницу скелетоном — сумма и список на месте', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    // Замедляем сводку: детерминированное окно «загрузка идёт», в котором
    // раньше страница целиком подменялась скелетонами.
    await page.route('**/operations/summary*', async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 600));
      await route.continue();
    });
    await page.goto(`${APARTMENT_OPERATIONS_URL}/expense`);
    await expect(page.getByRole('button', { name: 'Предыдущий месяц' })).toBeVisible();
    await expect(page.getByText(/2\s000\s₽/)).toBeVisible();

    await page.getByRole('button', { name: 'Предыдущий месяц' }).click();

    // Пока сводка августа в полёте: скелетона нет, H1-сумма (прежнего
    // периода) не исчезает — страница не дёргается.
    await expect(page.locator('section[aria-hidden] .animate-pulse')).toHaveCount(0);
    await expect(page.getByText(/2\s000\s₽/)).toBeVisible();
    await expect(page.getByText(/3\s000\s₽/)).toBeVisible({ timeout: 5_000 });
  });

  test('фильтры периода и категории не сбрасываются при переходе между списками', async ({
    page,
    seededUser,
  }) => {
    const previousMonth = new Date();
    previousMonth.setDate(1);
    previousMonth.setMonth(previousMonth.getMonth() - 1);
    const ym = `${previousMonth.getFullYear()}-${String(previousMonth.getMonth() + 1).padStart(2, '0')}`;
    const from = `${ym}-03`;
    const to = `${ym}-14`;

    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`${APARTMENT_OPERATIONS_URL}?from=${from}&to=${to}&category=internet`);
    await expect(page.getByRole('button', { name: 'Открыть доходы объекта' })).toBeVisible();

    // Карточка направления открывает его с теми же фильтрами.
    await page.getByRole('button', { name: 'Открыть доходы объекта' }).click();
    await expect(page).toHaveURL(
      new RegExp(`/operations/income\\?from=${from}&to=${to}&category=internet$`),
    );

    // «Назад» — на главный список с теми же фильтрами.
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(
      new RegExp(`/operations\\?from=${from}&to=${to}&category=internet$`),
    );
  });

  test('стрелки сдвигают произвольный период на его же длину — 3–14 становится соседними 12 днями', async ({
    page,
    seededUser,
  }) => {
    const first = new Date();
    first.setDate(1);
    first.setMonth(first.getMonth() - 1);
    const second = new Date(first);
    second.setMonth(second.getMonth() - 1);
    const ym = (date: Date): string =>
      `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;

    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`${APARTMENT_OPERATIONS_URL}/expense?from=${ym(first)}-03&to=${ym(first)}-14`);
    await expect(page.getByRole('button', { name: 'Предыдущий месяц' })).toBeVisible();

    // 3–14 <прошлый месяц> при листании назад — предыдущие 12 дней:
    // 22 <позапрошлый> — 2 <прошлый>, окна стыкуются без нахлёста.
    await page.getByRole('button', { name: 'Предыдущий месяц' }).click();
    await expect(page).toHaveURL(new RegExp(`from=${ym(second)}-22&to=${ym(first)}-02$`));

    // Вперёд — исходное окно 3–14 (листание обратимо).
    await expect(page.getByRole('button', { name: 'Следующий месяц' })).toBeEnabled();
    await page.getByRole('button', { name: 'Следующий месяц' }).click();
    await expect(page).toHaveURL(new RegExp(`from=${ym(first)}-03&to=${ym(first)}-14$`));

    // Дефолтный период — текущий месяц: следующее окно ушло бы в будущее.
    await page.goto(`${APARTMENT_OPERATIONS_URL}/expense`);
    await expect(page.getByRole('button', { name: 'Следующий месяц' })).toBeDisabled();
  });
});
