import {
  captureScreen,
  expect,
  monthlyDayOnOrAfter,
  openCabinetWithSessionToken,
  openCabinetWithSeededSession,
  seededMemberSessionToken,
  seededViewerSessionToken,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_STUDIO_PROPERTY_ID,
  test,
  todayIso,
} from './fixtures';
import { addDays } from '@/shared/lib/calendar';
import { formatDayMonthWithYear } from '@/shared/lib/date-format';

// Экран правки и модалка удаления (#467): форма, не визард — все поля
// предзаполнены правилом, сохранение — частичный PATCH (меняет только
// будущее, `since` недоступен), danger-кнопка «Удалить платеж» с модалкой
// выбора судьбы просрочек (тексты — Figma 1127:33148). Форма — по макету
// 705:10034: тип («Доход или расход») — строка, значение меняется простым
// нажатием; «Формы оплаты» на экране нет — поле снесено (карта #1005,
// тикеты #1006–#1009). Скриншоты — материал для сверки с Figma
// (705:10034 правка + модалка, 1096:36680 десктоп).
//
// Сид: «Страхование» …552 на квартире — цель правки (сумма 320 000 кап,
// день 15; одна просрочка остаётся прошлым); студия — «Консьерж-сервис»
// …559 (две просрочки, удаление без чекбокса — долг остаётся) и
// «Телевидение» …55a (одна просрочка, удаление с чекбоксом — сносится).
// Правки в этом файле не ломают последующие: название «Страхование» и
// просрочка остаются, сидовые платежи для удаления живут на студии.

const PROPERTY = SEEDED_APARTMENT_PROPERTY_ID;
const STUDIO = SEEDED_STUDIO_PROPERTY_ID;
const URLS = {
  insuranceEdit: `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555552/edit`,
  insurance: `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555552`,
  electricityEdit: `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555553/edit`,
  conciergeEdit: `/properties/${STUDIO}/payments/55555555-5555-4555-8555-555555555559/edit`,
  tvEdit: `/properties/${STUDIO}/payments/55555555-5555-4555-8555-55555555555a/edit`,
  studioPayments: `/properties/${STUDIO}/payments`,
};

test.describe('экран правки платежа', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('форма предзаполнена правилом; сохранение закрыто до правки; скриншот', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(URLS.insuranceEdit);

    // Шапка: только «Редактирование», отмена и сохранение.
    await expect(page.getByText('Редактирование')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Отменить правку' })).toBeVisible();
    // exact: «Сохранить» — клавиша Check в шапке, не sticky «Сохранить изменения».
    const check = page.getByRole('button', { name: 'Сохранить', exact: true });
    await expect(check).toBeDisabled();

    // Поля предзаполнены: сумма, название, категория, тип, регулярность,
    // бессрочное окончание.
    await expect(page.getByRole('textbox', { name: 'Сумма' })).toHaveValue(/3\s?200/);
    await expect(page.getByRole('textbox', { name: 'Название платежа' })).toHaveValue('Страхование');
    await expect(page.getByRole('button', { name: 'Категория' })).toHaveText(/Страхование/);
    await expect(page.getByRole('button', { name: 'Доход или расход' })).toHaveText(/Расход/);
    await expect(page.getByRole('button', { name: 'Регулярность платежа' })).toHaveText(
      /Каждый месяц 15 числа/,
    );
    await expect(page.getByRole('button', { name: 'Окончание платежа' })).toHaveText(/Бессрочно/);

    // `since` на форме нет, удаление доступно владельцу.
    await expect(page.getByText('Действует с')).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Удалить платеж' })).toBeVisible();

    await captureScreen(page, testInfo, 'payment-edit-filled-mobile');
  });

  test('правка суммы и регулярности сохраняется PATCH и меняет только будущее', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    // Вход через «Изменить»: сохранение возвращает назад по истории
    // (goBack), тост живёт в корневом портале того же приложения.
    await page.goto(URLS.insurance);
    await page.getByRole('button', { name: 'Изменить' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/edit$`));

    await page.getByRole('textbox', { name: 'Сумма' }).fill('4000');

    // Регулярность: страница «Выбор периодичности» → «Каждый месяц» → день 20.
    await page.getByRole('button', { name: 'Регулярность платежа' }).click();
    await expect(page.getByText('Выбор периодичности')).toBeVisible();
    // Контентных заголовков на странице правки нет — хедер ведёт тайтлом
    // (решение владельца 07.10, #1192).
    await expect(page.getByRole('heading', { name: 'Периодичность платежа' })).toHaveCount(0);
    await page.getByRole('button', { name: 'Каждый месяц' }).click();
    await expect(page.getByRole('heading', { name: 'Выберите день', exact: true })).toHaveCount(0);
    // Мультивыбор: у сид-платежа день 15 — снимаем его и ставим 20.
    await page.getByRole('button', { name: '15', exact: true }).click();
    await page.getByRole('button', { name: '20', exact: true }).click();
    // Правка живёт в черновике страницы и применяется кнопкой «Выбрать».
    await page.getByRole('button', { name: 'Выбрать' }).click();
    await expect(
      page.getByRole('button', { name: 'Регулярность платежа' }),
    ).toHaveText(/Каждый месяц 20 числа/);

    await expect(page.getByRole('button', { name: 'Сохранить изменения' })).toBeEnabled();
    await page.getByRole('button', { name: 'Сохранить изменения' }).click();

    // Тост и возврат на страницу платежа с новыми значениями.
    await expect(page.getByText('Изменения сохранены')).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+$`));
    await expect(page.getByText('4 000 ₽').first()).toBeVisible();
    await expect(page.getByText('Каждый месяц 20 числа')).toBeVisible();

    // Прошлое не тронуто: просрочка осталась на странице платежа.
    await expect(page.getByText('Просроченные платежи')).toBeVisible();
    await expect(page.getByText(/\d+ (день|дня|дней)/).first()).toBeVisible();

    await captureScreen(page, testInfo, 'payment-edited-detail-mobile');
  });

  test('черновик периодичности: ветка без дней не меняет форму до «Выбрать»', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(URLS.insuranceEdit);

    // Ветка недели без выбранных дней — незавершённый период остаётся
    // черновиком страницы, «Назад» его отбрасывает (баг «Каждую неделю в —»).
    await page.getByRole('button', { name: 'Регулярность платежа' }).click();
    await page.getByRole('button', { name: 'Каждую неделю' }).click();
    await expect(page.getByText('Понедельник')).toBeVisible();
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page.getByRole('button', { name: 'Каждый месяц' })).toBeVisible();
    await page.getByRole('button', { name: 'Назад' }).click();

    // Форма с прежним значением, сохранение закрыто — правки не было.
    await expect(page.getByRole('button', { name: 'Регулярность платежа' })).toHaveText(
      /Каждый месяц 20 числа/,
    );
    await expect(page.getByRole('button', { name: 'Сохранить изменения' })).toBeDisabled();
  });

  test('периодичность как в создании: день применяется сразу, год — «Продолжить» календаря', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(URLS.insuranceEdit);

    // «Каждый день»: ветки нет — выбор пункта меню сразу применяет правило
    // и закрывает страницу, как в создании (решение #995, правка #1153).
    await page.getByRole('button', { name: 'Регулярность платежа' }).click();
    await expect(page.getByText('Выбор периодичности')).toBeVisible();
    await page.getByRole('button', { name: 'Каждый день' }).click();
    await expect(page.getByText('Выбор периодичности')).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Регулярность платежа' })).toHaveText(
      /Ежедневно/,
    );

    // Правка не сохранена: после перезагрузки прежнее правило на месте.
    await page.reload();
    await expect(page.getByRole('button', { name: 'Регулярность платежа' })).not.toHaveText(
      /Ежедневно/,
    );

    // «Каждый год»: ветка — календарь с кнопкой «Продолжить» (не «Выбрать»);
    // подтверждение применяет правило и закрывает страницу сразу.
    await page.getByRole('button', { name: 'Регулярность платежа' }).click();
    await page.getByRole('button', { name: 'Каждый год' }).click();
    const calendar = page.getByRole('dialog', { name: 'Выбрать дату' });
    await expect(calendar).toBeVisible();
    // Черновик предвыбран сегодняшним днём (маркер aria-current), «Продолжить»
    // коммитит без действий; день читается из маркера, месяц — из UTC-часов
    // раннера: экран и браузер e2e (timezoneId UTC) живут в одних сутках.
    const todayCell = calendar.locator('button[aria-current="date"]');
    const todayLabel = ((await todayCell.textContent()) ?? '').trim();
    await expect(calendar.getByRole('button', { name: 'Продолжить' })).toBeVisible();
    await calendar.getByRole('button', { name: 'Продолжить' }).click();
    await expect(calendar).toHaveCount(0);
    await expect(page.getByText('Выбор периодичности')).toHaveCount(0);
    const monthGen = [
      'января', 'февраля', 'марта', 'апреля', 'мая', 'июня',
      'июля', 'августа', 'сентября', 'октября', 'ноября', 'декабря',
    ][new Date().getUTCMonth()];
    await expect(page.getByRole('button', { name: 'Регулярность платежа' })).toHaveText(
      new RegExp(`Каждое ${todayLabel} ${monthGen}`),
    );

    // Правка не сохранена: после перезагрузки прежнее правило на месте.
    await page.reload();
    await expect(page.getByRole('button', { name: 'Регулярность платежа' })).not.toHaveText(
      /Каждое /,
    );
  });

  test('тип меняется простым нажатием, без пикера', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(URLS.insuranceEdit);

    // Нажатие по строке сразу переключает значение — ни пикера, ни шита.
    await page.getByRole('button', { name: 'Доход или расход' }).click();
    await expect(page.getByRole('button', { name: 'Доход или расход' })).toHaveText(/Доход/);
    await expect(page.getByRole('dialog')).toHaveCount(0);

    // Правка не сохранена: после перезагрузки сидовые значения на месте.
    await page.reload();
    await expect(page.getByRole('button', { name: 'Доход или расход' })).toHaveText(/Расход/);
  });

  test('выбор категории — отдельная страница, как в визарде', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(URLS.insuranceEdit);

    await page.getByRole('button', { name: 'Категория' }).click();
    // Хедер страницы категории — тот же H1, что на шаге 1 визарда (#1152).
    await expect(
      page.getByRole('heading', { name: 'Выберите категорию платежа' }),
    ).toBeVisible();
    // «Готово» появляется только после смены категории.
    await expect(page.getByRole('button', { name: 'Готово' })).toHaveCount(0);
    await page.getByRole('button', { name: 'Интернет', exact: true }).click();
    await page.getByRole('button', { name: 'Готово' }).click();

    // Форма показывает выбранное и готова к сохранению; правка не сохранена.
    await expect(page.getByRole('button', { name: 'Категория' })).toHaveText(/Интернет/);
    await expect(page.getByRole('button', { name: 'Сохранить изменения' })).toBeEnabled();
    await page.reload();
    await expect(page.getByRole('button', { name: 'Категория' })).toHaveText(/Страхование/);

    // Поиск со страницы категории (лупа в хедере) фильтрует список.
    await page.getByRole('button', { name: 'Категория' }).click();
    await page.getByRole('button', { name: 'Поиск по категориям' }).click();
    await page.getByRole('searchbox', { name: 'Поиск по названиям категорий' }).fill('страх');
    await expect(page.getByRole('button', { name: 'Страхование', exact: true })).toBeVisible();
    // В режиме поиска заголовка нет — как на шаге 1 визарда (макет
    // 1049:46418, решение владельца 06.10).
    await expect(
      page.getByRole('heading', { name: 'Выберите категорию платежа' }),
    ).toHaveCount(0);
    // Пустой результат — тоже без заголовка.
    await page.getByRole('searchbox', { name: 'Поиск по названиям категорий' }).fill('йцукен');
    await expect(page.getByText('Ничего не нашлось')).toBeVisible();
    await expect(
      page.getByRole('heading', { name: 'Выберите категорию платежа' }),
    ).toHaveCount(0);
    // Крестик возвращает шаг — заголовок на месте.
    await page.getByRole('button', { name: 'Очистить поиск' }).click();
    await expect(
      page.getByRole('heading', { name: 'Выберите категорию платежа' }),
    ).toBeVisible();
    // Вернуть поиск для замера шапки ниже.
    await page.getByRole('button', { name: 'Поиск по категориям' }).click();
    await page.getByRole('searchbox', { name: 'Поиск по названиям категорий' }).fill('страх');
    await expect(page.getByRole('button', { name: 'Страхование', exact: true })).toBeVisible();

    // Поисковая шапка — канон TopNav variant="search": поле тянется ОТ
    // стрелки «Назад», а не под ней (баг приёмки #1069: без варианта
    // дефолтная шапка клала поле в центральный слой на всю ширину, стрелка
    // рисовалась поверх плейсхолдера). Замер по boundingBox: левый край
    // поля не левее правого края стрелки.
    const arrowBox = await page.getByRole('button', { name: 'Назад' }).boundingBox();
    const fieldBox = await page
      .getByRole('searchbox', { name: 'Поиск по названиям категорий' })
      .boundingBox();
    if (arrowBox === null || fieldBox === null) {
      throw new Error('шапка поиска не нашлась для замера');
    }
    expect(fieldBox.x).toBeGreaterThanOrEqual(arrowBox.x + arrowBox.width);

    // Клик по «Назад» при открытом поиске закрывает пикер категории.
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page.getByRole('button', { name: 'Категория' })).toBeVisible();
  });

  test('окончание платежа: пикер не даёт день раньше первого вхождения', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(URLS.insuranceEdit);

    await page.getByRole('button', { name: 'Окончание платежа' }).click();
    // Канон (04.09): бесконечный календарь поверх формы, заголовок —
    // название поля (#1155). Минимум пикера — первое вхождение действующего
    // monthly-20 правила (сид: since = сегодня+5), оно всегда в будущем:
    // сегодня погашено, черновик прижат к первому доступному дню, «Выбрать»
    // коммитит его без действий (прецедент продления #802).
    const firstOccurrence = monthlyDayOnOrAfter(20, addDays(todayIso(), 5));
    const calendar = page.getByRole('dialog', { name: 'Окончание платежа' });
    await expect(calendar).toBeVisible();
    await expect(calendar.locator('button[aria-current="date"]')).toBeDisabled();
    await calendar.getByRole('button', { name: 'Выбрать', exact: true }).click();
    await expect(calendar).toHaveCount(0);
    // Первое вхождение применено к форме (правка не сохранена — после
    // перезагрузки снова «Бессрочно»).
    await expect(page.getByRole('button', { name: 'Окончание платежа' })).toHaveText(
      formatDayMonthWithYear(firstOccurrence, todayIso()),
    );
    await expect(page.getByRole('button', { name: 'Сохранить изменения' })).toBeEnabled();
    await page.reload();
    await expect(page.getByRole('button', { name: 'Окончание платежа' })).toHaveText(/Бессрочно/);
  });

  test('смена периодичности сбрасывает невалидное окончание молча, без подсказки (#1155)', async ({
    page,
    seededUser,
  }) => {
    // К этому тесту сид-платёж уже переведён на monthly-20 (тест выше
    // сохраняет PATCH); правки здесь не сохраняются — порядок файла важен.
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(URLS.insuranceEdit);

    // Стоящее окончание = первое вхождение monthly-20 (минимум пикера,
    // коммит без действий).
    await page.getByRole('button', { name: 'Окончание платежа' }).click();
    const calendar = page.getByRole('dialog', { name: 'Окончание платежа' });
    await expect(calendar).toBeVisible();
    await calendar.getByRole('button', { name: 'Выбрать', exact: true }).click();
    await expect(calendar).toHaveCount(0);

    // Смена периодичности на годовую: первое вхождение уезжает на год
    // вперёд (год подтверждается «Продолжить» календаря, #1153) — стоящее
    // окончание становится невалидным.
    await page.getByRole('button', { name: 'Регулярность платежа' }).click();
    await page.getByRole('button', { name: 'Каждый год' }).click();
    const yearly = page.getByRole('dialog', { name: 'Выбрать дату' });
    await expect(yearly).toBeVisible();
    await yearly.getByRole('button', { name: 'Продолжить' }).click();
    await expect(yearly).toHaveCount(0);
    await expect(page.getByText('Выбор периодичности')).toHaveCount(0);

    // Решение владельца 2026-10-06: сброс молча, без подсказки —
    // «Бессрочно», сохранение доступно (уходит tri-state endDate: null).
    await expect(page.getByRole('button', { name: 'Окончание платежа' })).toHaveText(
      /Бессрочно/,
    );
    await expect(page.getByRole('button', { name: 'Сохранить изменения' })).toBeEnabled();

    // Правка не сохранена: после перезагрузки сид-правило на месте,
    // окончание бессрочное.
    await page.reload();
    await expect(page.getByRole('button', { name: 'Окончание платежа' })).toHaveText(/Бессрочно/);
    await expect(page.getByRole('button', { name: 'Регулярность платежа' })).toHaveText(
      /Каждый месяц 20 числа/,
    );
  });

  test('отмена крестом возвращает на страницу платежа без изменений', async ({
    page,
    seededUser,
  }) => {
    // Вход честным путём — «Изменить» со страницы платежа: отмена возвращает
    // назад по истории (при прямом goto её нет).
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(URLS.insurance);
    await page.getByRole('button', { name: 'Изменить' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/edit$`));

    await page.getByRole('textbox', { name: 'Сумма' }).fill('999');
    await page.getByRole('button', { name: 'Отменить правку' }).click();

    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+$`));
    await expect(page.getByText('3 200 ₽').first()).toBeVisible();
  });

  test('модалка удаления без просрочек: чекбокса нет, отмена ничего не меняет', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(URLS.electricityEdit);

    await expect(page.getByRole('button', { name: 'Удалить платеж' })).toBeVisible();
    await page.getByRole('button', { name: 'Удалить платеж' }).click();

    // Автоплатёж зовётся иначе; просрочек нет — чекбокса нет (история 35).
    await expect(page.getByText('Удалить автоплатёж?')).toBeVisible();
    await expect(
      page.getByText('Его нельзя будет восстановить. Вместо удаления платежа, его можно поставить на паузу'),
    ).toBeVisible();
    await expect(page.getByText('Удалить просроченные операции')).toHaveCount(0);

    await page.waitForTimeout(700);
    await captureScreen(page, testInfo, 'payment-delete-modal-no-overdue-mobile');

    await page.getByRole('button', { name: 'Отменить' }).click();
    await expect(page.getByText('Удалить автоплатёж?')).not.toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/edit$`));
  });
});

test.describe('удаление платежа', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('без чекбокса: правило сносится, просрочки остаются долгом; скриншоты', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);

    // До удаления: правило в списке + две просрочки карточками.
    await page.goto(URLS.studioPayments);
    // Правило в списке + две просрочки карточками (даты старше сидовых 55
    // месяцев аренды студии — карточки в первой порции asc-порции по 50).
    await expect(page.getByText('Консьерж-сервис')).toHaveCount(3);

    await page.goto(URLS.conciergeEdit);
    await page.getByRole('button', { name: 'Удалить платеж' }).click();

    await expect(page.getByText('Удалить платеж?')).toBeVisible();
    // Просрочки есть — чекбокс виден; безопасный дефолт — не отмечен.
    const checkbox = page.getByRole('checkbox', { name: 'Удалить просроченные операции' });
    await expect(checkbox).toBeVisible();
    await expect(checkbox).not.toBeChecked();

    await page.waitForTimeout(700);
    await captureScreen(page, testInfo, 'payment-delete-modal-overdue-mobile');

    await page.getByRole('button', { name: 'Удалить', exact: true }).click();

    // Тост «Платеж удален» — уже на списке объекта (история 34/52).
    await expect(page.getByText('Платеж удален')).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/properties/${STUDIO}/payments$`));

    // Долг остался: обе просрочки карточками; правила в списке больше нет.
    await expect(page.getByText('Просроченные операции')).toBeVisible();
    await expect(page.getByText('Консьерж-сервис')).toHaveCount(2);
  });

  test('с чекбоксом: просрочки сносятся вместе с правилом', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);

    await page.goto(URLS.studioPayments);
    await expect(page.getByText('Телевидение')).toHaveCount(2);

    await page.goto(URLS.tvEdit);
    await page.getByRole('button', { name: 'Удалить платеж' }).click();

    await expect(page.getByText('Удалить платеж?')).toBeVisible();
    await page.getByText('Удалить просроченные операции').click();
    await expect(
      page.getByRole('checkbox', { name: 'Удалить просроченные операции' }),
    ).toBeChecked();

    await page.getByRole('button', { name: 'Удалить', exact: true }).click();

    await expect(page.getByText('Платеж удален')).toBeVisible();
    await expect(page).toHaveURL(new RegExp(`/properties/${STUDIO}/payments$`));

    // Снесено всё: ни правила, ни просрочек.
    await expect(page.getByText('Телевидение')).toHaveCount(0);
    await expect(page.getByText('Консьерж-сервис')).toHaveCount(2);

    // Оплаченный факт переживает правило с пометкой «платёж удалён»:
    // операция на месте, payment_id обнулён (AC #467; поверхность показа
    // пометки — экраны истории операций объекта, следующий срез).
    const paid = await page.request.get(
      `/api/properties/${STUDIO}/operations?status=paid&limit=50`,
    );
    expect(paid.ok()).toBe(true);
    const { items } = (await paid.json()) as {
      items: ReadonlyArray<{ title: string; paymentId: string | null }>;
    };
    const tvPaid = items.filter((op) => op.title === 'Телевидение');
    expect(tvPaid).toHaveLength(1);
    expect(tvPaid[0]?.paymentId).toBeNull();
  });
});

test.describe('экран правки — роли', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('смотрящий: экрана правки нет, на странице платежа нет «Изменить»', async ({
    page,
  }) => {
    await openCabinetWithSessionToken(page, seededViewerSessionToken());
    await page.goto(URLS.insuranceEdit);

    await expect(page.getByText('Правка недоступна')).toBeVisible();
    await expect(page.getByText('У вас доступ только для просмотра этого объекта')).toBeVisible();
    await expect(page.getByText('Сумма', { exact: true })).toHaveCount(0);

    await page.goto(URLS.insurance);
    await expect(page.getByRole('button', { name: 'Изменить' })).toHaveCount(0);
  });

  test('полный доступ: правит, но удаления нет', async ({ page }) => {
    await openCabinetWithSessionToken(page, seededMemberSessionToken());
    await page.goto(URLS.insurance);
    await expect(page.getByRole('button', { name: 'Изменить' })).toBeVisible();
    await page.getByRole('button', { name: 'Изменить' }).click();
    await expect(page).toHaveURL(new RegExp(`/payments/[0-9a-f-]+/edit$`));

    // Сумма уже правлена мобильным тестом выше (файл идёт по порядку).
    await expect(page.getByRole('textbox', { name: 'Сумма' })).toHaveValue(/4\s?000/);
    await expect(page.getByRole('button', { name: 'Удалить платеж' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Сохранить изменения' })).toBeDisabled();

    // Сохранение доступно после правки и работает под полным доступом.
    await page.getByRole('textbox', { name: 'Название платежа' }).fill('Страхование квартиры');
    await expect(page.getByRole('button', { name: 'Сохранить изменения' })).toBeEnabled();
    await page.getByRole('button', { name: 'Сохранить изменения' }).click();
    await expect(page.getByText('Изменения сохранены')).toBeVisible();
    await expect(page.getByText('Страхование квартиры').first()).toBeVisible();
  });
});

test.describe('экран правки — десктоп', () => {
  test.use({ viewport: { width: 1440, height: 900 } });

  test('форма и модалка удаления в колонке 560; скриншоты', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(URLS.insuranceEdit);

    await expect(page.getByText('Редактирование')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Регулярность платежа' })).toHaveText(
      /Каждый месяц 20 числа/,
    );
    await captureScreen(page, testInfo, 'payment-edit-desktop');

    // Модалка удаления — карточка по центру (адаптивный шелл), чекбокс на месте.
    await page.getByRole('button', { name: 'Удалить платеж' }).click();
    await expect(page.getByText('Удалить платеж?')).toBeVisible();
    await expect(
      page.getByRole('checkbox', { name: 'Удалить просроченные операции' }),
    ).toBeVisible();
    await page.waitForTimeout(600);
    await captureScreen(page, testInfo, 'payment-delete-modal-desktop');

    await page.getByRole('button', { name: 'Отменить' }).click();
    await expect(page.getByText('Удалить платеж?')).not.toBeVisible();
  });
});
