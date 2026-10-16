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

  test('направления пустого объекта — «Операций еще не было» без чипов и поиска; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);

    // Канон направлений #679: на совсем пустом объекте направление, как и
    // главный список (#478), показывает «Операций еще не было» с CTA —
    // прежние «Нет доходов»/«Нет трат» (1510-76177/1510-75650) остались в
    // макетах, но зона живёт конвенцией #478/#571.
    await page.goto(`${GARAGE_OPERATIONS_URL}/income`);
    await expect(page.getByText('Операций еще не было')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Поиск операций' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /Все категории/ })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Добавить операцию' })).toBeVisible();

    await page.goto(`${GARAGE_OPERATIONS_URL}/expense`);
    await expect(page.getByText('Операций еще не было')).toBeVisible();

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

  test('операции квартиры: сводка направлений за всё время, список по датам', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(APARTMENT_OPERATIONS_URL);

    // Обычный экран, не «Операций еще не было»: есть чипы, сводка и поиск.
    await expect(page.getByRole('button', { name: 'Поиск операций' })).toBeVisible();
    await expect(page.getByRole('button', { name: /Все категории/ })).toBeVisible();

    // Дефолт «весь период» (#674): карточки направлений показывают итоги
    // за всё время — доход один (оверлей живой приёмки, «Аренда
    // машиноместа»), расходы — 57 сидовых «Интернет» по 1000 ₽.
    const incomeCard = page.getByRole('button', { name: 'Открыть доходы объекта' });
    await expect(incomeCard.getByText(/2\s000\s₽/)).toBeVisible();
    const expenseCard = page.getByRole('button', { name: 'Открыть расходы объекта' });
    await expect(expenseCard.getByText(/57\s000\s₽/)).toBeVisible();

    await expect(page.getByText('Сегодня').first()).toBeVisible();
    await expect(page.getByText('Интернет').first()).toBeVisible();
    await expect(page.getByText('Операций еще не было')).toHaveCount(0);
  });

  test('направления квартиры: доходы — карточка и строка оверлея, расходы — со списком', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);

    // Дефолт «весь период» (#675): карточка направления показывает итог за
    // всё время. Доход у квартиры один — «Аренда машиноместа» из оверлея
    // живой приёмки (2 000 ₽), объект не пуст — без neverHad-гейта.
    await page.goto(`${APARTMENT_OPERATIONS_URL}/income`);
    await expect(page.getByText('2 000 ₽', { exact: true })).toBeVisible();
    await expect(page.getByText('Аренда машиноместа').first()).toBeVisible();
    await expect(page.getByText('Операций еще не было')).toHaveCount(0);

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
    // Пустое окно на направлении (#679): карточка с нулём и общая подпись
    // пустой ленты — H1 «Нет трат» ушёл вместе с листанием периода.
    await expect(page.getByText(EMPTY_PERIOD_CAPTION)).toBeVisible();
    await expect(page.getByText('0 ₽')).toBeVisible();

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

    // Чип «Период» — нейтральный в дефолте «весь период» (#670) — открывает
    // канонический пикер поверх списка; «Назад» пикера закрывает его,
    // адрес списка не меняется.
    await page.getByRole('button', { name: 'Период', exact: true }).click();
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

    // Чип периода в дефолте «весь период» нейтральный («Период», #670),
    // с применённым выбором — диапазон; берём его как соседа стабильного
    // «Все категории». Диапазон выбираем в прошлом месяце — все его дни
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
    // Дефолт «весь период» (#675): all-time сводка расходов квартиры —
    // 57 оплаченных «Интернет» по 1000 ₽ (55 базового сида + 2 оверлея),
    // значение стабильно при любом дне прогона.
    await page.goto(`${APARTMENT_OPERATIONS_URL}/expense`);
    await expect(page.getByText(/57\s000\s₽/)).toBeVisible();

    // Замедляем сводку: детерминированное окно «загрузка идёт», в котором
    // раньше страница целиком подменялась скелетонами.
    await page.route('**/operations/summary*', async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 600));
      await route.continue();
    });

    // Период применяем чипом и пикером (#675): окно «1 — конец месяца» два
    // месяца назад — там ровно одна сидовая операция при любом дне прогона
    // (дневные «сегодня/вчера» туда не попадают, помесячные — по одной).
    await page.getByRole('button', { name: 'Период', exact: true }).click();
    await expect(page.getByText('Выберите период')).toBeVisible();
    const twoMonthsAgo = new Date();
    twoMonthsAgo.setDate(1);
    twoMonthsAgo.setMonth(twoMonthsAgo.getMonth() - 2);
    const monthBlock = page.locator('section').filter({
      has: page.getByRole('heading', {
        name: `${PICKER_MONTH_NAMES[twoMonthsAgo.getMonth()]}, ${twoMonthsAgo.getFullYear()}`,
      }),
    });
    const lastDay = new Date(
      twoMonthsAgo.getFullYear(),
      twoMonthsAgo.getMonth() + 1,
      0,
    ).getDate();
    await monthBlock.getByRole('button', { name: '1', exact: true }).click();
    await monthBlock.getByRole('button', { name: String(lastDay), exact: true }).click();
    await page.getByRole('dialog').getByRole('button', { name: 'Выбрать', exact: true }).click();
    await expect(page).toHaveURL(/\?from=/);

    // Пока сводка окна в полёте: скелетона нет, прежняя all-time сумма не
    // исчезает — страница не дёргается.
    await expect(page.locator('section[aria-hidden] .animate-pulse')).toHaveCount(0);
    await expect(page.getByText(/57\s000\s₽/)).toBeVisible();

    // Сводка окна доехала: одна операция месяца = 1 000 ₽, all-time сумма
    // ушла вместе со сменой данных — без скелетона и мигания.
    await expect(page.getByText(/57\s000\s₽/)).toHaveCount(0, { timeout: 5_000 });
    await expect(page.getByText(/1\s000\s₽/).first()).toBeVisible();
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
});
