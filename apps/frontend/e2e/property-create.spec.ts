import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Визард создания объекта: каркас флоу и шаг «Выбор категории» (#480,
// Figma 1213-52111), шаг «Адрес» с подсказками (#481, Figma 1213-52017/
// 52391, 1519-94336), шаг «Характеристики» (#482, Figma 1218-54295) и
// экран успеха (#483, Figma 1425-55788). Один маршрут /properties/new,
// шаги — клиентское состояние, черновик переживает перезагрузку
// (sessionStorage). Успех шагом не считается; с ?returnTo= он пропускается.
//
// Живые подсказки проверяются только при настроенном стабе Dadata
// (DADATA_BASE_URL локального прогона): в CI ключа нет, endpoint тихо
// деградирует — каркасные тесты от этого не зависят.
//
// Полный флоу создаёт объекты поверх трёх сидовых: подписка pro сида
// (лимит 5) вмещает ровно два создания на прогон — новые сценарии
// создания согласовывают с этим бюджетом.

const dadataStubConfigured = process.env.DADATA_BASE_URL !== undefined;

async function openWizard(page: Parameters<typeof openCabinetWithSeededSession>[0], user: Parameters<typeof openCabinetWithSeededSession>[1]): Promise<void> {
  await openCabinetWithSeededSession(page, user);
  await page.goto('/properties');
  // Поверхностей создания в хабе три (#586): на десктопном ярусе кликабельна
  // «+» поисковой пилюли — компакт-бар шапки скрыт (opacity-0, канон
  // сворачивания #556), пилюля живёт в контенте на всех ширинах.
  await page.getByTestId('properties-search-pill').getByRole('button', { name: 'Создать объект' }).click();
  await expect(page.getByRole('heading', { name: 'Выберите, какая у вас недвижимость' })).toBeVisible();
}

test('шаг 1 → шаг 2: категория ведёт на адрес, ручной ввод продолжает', async ({ page, seededUser }, testInfo) => {
  await openWizard(page, seededUser);

  // Шаг 1: девять чипов категорий, чип «шаг 1 из 3».
  const group = page.getByRole('group', { name: 'Категория объекта' });
  await expect(group.getByRole('button')).toHaveCount(9);
  await expect(page.getByText('шаг 1 из 3')).toBeVisible();
  await captureScreen(page, testInfo, '01-category-step');

  // Выбор категории сразу переводит на шаг адреса.
  await group.getByRole('button', { name: 'Квартира' }).click();
  await expect(page.getByText('шаг 2 из 3')).toBeVisible();

  // Поле захватило фокус на входе в шаг; ручной ввод открывает «Продолжить».
  const address = page.getByRole('textbox', { name: 'Введите адрес' });
  await expect(address).toBeFocused();
  await address.fill('Гаражный кооператив «Восход», бокс 12');
  const next = page.getByRole('button', { name: 'Продолжить' });
  await expect(next).toBeVisible();

  await next.click();
  await expect(page.getByText('шаг 3 из 3')).toBeVisible();

  // Назад: адрес сохранён; ещё назад — категория отмечена выбранным чипом.
  await page.getByRole('button', { name: 'Назад' }).click();
  await expect(address).toHaveValue('Гаражный кооператив «Восход», бокс 12');
  await page.getByRole('button', { name: 'Назад' }).click();
  await expect(group.getByRole('button', { name: 'Квартира' })).toHaveAttribute('aria-pressed', 'true');

  // Крестик закрывает визард на список объектов.
  await page.getByRole('button', { name: 'Закрыть' }).click();
  await expect(page.getByRole('heading', { name: 'Объекты', exact: true })).toBeVisible();
});

test('шаг 2: подсказки адреса — список, выбор, возврат после правки', async ({ page, seededUser }, testInfo) => {
  test.skip(!dadataStubConfigured, 'живые подсказки — только с стабом Dadata (DADATA_BASE_URL)');

  await openWizard(page, seededUser);
  await page.getByRole('group', { name: 'Категория объекта' }).getByRole('button', { name: 'Гараж' }).click();
  const address = page.getByRole('textbox', { name: 'Введите адрес' });
  await expect(address).toBeFocused();

  // До трёх символов подсказок нет; со строки «Ленина» приходит список:
  // заголовок — улица и дом (город отрезан), подпись — город. Список —
  // ARIA-listbox, строки — опции (#482: полный listbox-паттерн).
  const listbox = page.getByRole('listbox', { name: 'Подсказки адреса' });
  await address.fill('Ле');
  await expect(listbox.getByRole('option', { name: /Ленина/ })).toHaveCount(0);
  await address.fill('Ленина');
  const firstRow = listbox.getByRole('option', { name: 'Ленина, д. 1 Москва' });
  await expect(firstRow).toBeVisible();
  await expect(listbox.getByRole('option', { name: 'Ленина, 5 Санкт-Петербург' })).toBeVisible();
  await captureScreen(page, testInfo, '02-address-suggestions');

  // Стрелки входят в список (вниз — к первой строке), Escape со строки
  // возвращает фокус в поле и прячет список.
  await address.press('ArrowDown');
  await expect(firstRow).toBeFocused();
  await firstRow.press('Escape');
  await expect(address).toBeFocused();
  await expect(firstRow).toBeHidden();

  // Правка значения возвращает список.
  await address.press('a');
  await expect(firstRow).toBeVisible();

  // Выбор подсказки: полный адрес в поле, список скрыт, «Продолжить» есть.
  await firstRow.click();
  await expect(address).toHaveValue('г. Москва, Ленина, д. 1');
  await expect(firstRow).toBeHidden();
  await expect(page.getByRole('button', { name: 'Продолжить' })).toBeVisible();
  await captureScreen(page, testInfo, '03-address-picked');
});

test('шаг 3: характеристики — поля каталога, тип жилья, гейт названия', async ({ page, seededUser }, testInfo) => {
  await openWizard(page, seededUser);
  await page.getByRole('group', { name: 'Категория объекта' }).getByRole('button', { name: 'Квартира' }).click();
  await page.getByRole('textbox', { name: 'Введите адрес' }).fill('Ленина, 1');
  await page.getByRole('button', { name: 'Продолжить' }).click();

  // Заголовок и подсказка шага 3; «Тип жилья» только у категории
  // «Квартира» (закрывает тип apartments).
  await expect(page.getByRole('heading', { name: 'Характеристики' })).toBeVisible();
  await expect(page.getByText('Вы можете создать объект, а характеристики заполнить позже')).toBeVisible();
  const housing = page.getByRole('group', { name: 'Тип жилья' });
  await expect(housing.getByRole('button', { name: 'Квартира' })).toHaveAttribute('aria-pressed', 'true');
  await expect(housing.getByRole('button', { name: 'Апартаменты' })).toHaveAttribute('aria-pressed', 'false');

  // Поля каталога квартиры: enum-чипы (комнат в наборе больше нет) и
  // единицы слева внутри бокса.
  await expect(page.getByRole('group', { name: 'Комнаты' })).toHaveCount(0);
  await expect(page.getByRole('group', { name: 'Санузел' }).getByRole('button', { name: 'Раздельный' })).toBeVisible();
  await expect(page.getByText('м²').first()).toBeVisible();

  // Первое поле шага захватило фокус на входе (паттерн шага «Адрес»);
  // проверяем до кликов по чипам — они перенесут фокус.
  const name = page.getByRole('textbox', { name: 'Название объекта' });
  await expect(name).toBeFocused();

  // Смена типа жилья — чипы перезаключаются, набор полей тот же (один
  // каталог у квартиры и апартаментов), смена тихая — без нотиса.
  await housing.getByRole('button', { name: 'Апартаменты' }).click();
  await expect(housing.getByRole('button', { name: 'Апартаменты' })).toHaveAttribute('aria-pressed', 'true');
  await expect(page.getByRole('group', { name: 'Санузел' }).getByRole('button', { name: 'Раздельный' })).toBeVisible();
  await expect(page.getByRole('status')).toHaveCount(0);

  // Нечислимый ввод не попадает в поле: фильтр по типу поля.
  const floor = page.getByRole('textbox', { name: 'Этаж', exact: true });
  await floor.pressSequentially('5а,');
  await expect(floor).toHaveValue('5');
  const area = page.getByRole('textbox', { name: 'Общая площадь' });
  await area.pressSequentially('47,5м');
  await expect(area).toHaveValue('47,5');

  // Заполнение характеристики, «Создать объект» появляется с названием
  // (обязательное поле), счётчик лимита 64 работает.
  const submit = page.getByRole('button', { name: 'Создать объект' });
  await expect(submit).toBeDisabled();
  await name.fill('Квартира на Ленина');
  await expect(page.getByText('18/64')).toBeVisible();
  await expect(submit).toBeEnabled();
  await captureScreen(page, testInfo, '04-characteristics');
});

test('шаг 2: черновик переживает перезагрузку, очистка убирает «Продолжить»', async ({ page, seededUser }) => {
  await openWizard(page, seededUser);
  await page.getByRole('group', { name: 'Категория объекта' }).getByRole('button', { name: 'Гараж' }).click();
  const address = page.getByRole('textbox', { name: 'Введите адрес' });
  await expect(address).toBeFocused();

  await address.fill('Ленина, 1');
  await page.getByRole('button', { name: 'Продолжить' }).click();
  await expect(page.getByText('шаг 3 из 3')).toBeVisible();

  // Перезагрузка восстанавливает первый незавершённый шаг (3 — без
  // названия); назад — адрес на месте; очистка убирает «Продолжить».
  await page.reload();
  await expect(page.getByText('шаг 3 из 3')).toBeVisible();
  await page.getByRole('button', { name: 'Назад' }).click();
  await expect(address).toHaveValue('Ленина, 1');
  await page.getByRole('button', { name: 'Очистить поле' }).click();
  await expect(address).toHaveValue('');
  await expect(page.getByRole('button', { name: 'Продолжить' })).toBeHidden();
});

test('полный флоу: «Создать объект» ведёт на успех, «Открыть объект» — на карточку', async ({ page, seededUser }, testInfo) => {
  await openWizard(page, seededUser);
  await page.getByRole('group', { name: 'Категория объекта' }).getByRole('button', { name: 'Дом' }).click();
  await page.getByRole('textbox', { name: 'Введите адрес' }).fill('Ленина, 2');
  await page.getByRole('button', { name: 'Продолжить' }).click();
  const name = page.getByRole('textbox', { name: 'Название объекта' });
  await name.fill('Дом на Ленина');
  await page.getByRole('button', { name: 'Создать объект' }).click();

  // Успех (Figma 1425-55788): заголовок с названием, подзаголовок макета,
  // пара кнопок (override владельца: «Добавить аренду» — заглушка); хедер
  // без чипа шага — только крестик.
  await expect(page.getByRole('heading', { name: 'Объект «Дом на Ленина» создан' })).toBeVisible();
  await expect(page.getByText('Вы создали объект, теперь можете добавить аренду')).toBeVisible();
  await expect(page.getByText(/шаг \d из/)).toHaveCount(0);
  const openProperty = page.getByRole('button', { name: 'Открыть объект' });
  await expect(page.getByRole('button', { name: 'Добавить аренду' })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Закрыть' })).toBeVisible();

  // Заглушка аренды: тост-объяснение, экран остаётся на месте.
  await page.getByRole('button', { name: 'Добавить аренду' }).click();
  await expect(page.getByText('Раздел «Аренда» скоро появится')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Объект «Дом на Ленина» создан' })).toBeVisible();
  await captureScreen(page, testInfo, '05-success');

  // «Открыть объект» ведёт на карточку созданного объекта (крестик —
  // тот же переход, проверяется живой приёмкой).
  await openProperty.click();
  await expect(page).toHaveURL(/\/properties\/[0-9a-f-]{36}$/);
  await expect(page.getByRole('heading', { name: 'Дом на Ленина' })).toBeVisible();
});

test('returnTo: успех пропускается, redirect на returnTo с propertyId', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties/new?returnTo=%2Fproperties');
  await expect(page.getByRole('heading', { name: 'Выберите, какая у вас недвижимость' })).toBeVisible();

  await page.getByRole('group', { name: 'Категория объекта' }).getByRole('button', { name: 'Гараж' }).click();
  await page.getByRole('textbox', { name: 'Введите адрес' }).fill('Садовая, 3');
  await page.getByRole('button', { name: 'Продолжить' }).click();
  await page.getByRole('textbox', { name: 'Название объекта' }).fill('Гараж на Садовой 3');
  await page.getByRole('button', { name: 'Создать объект' }).click();

  // Экран успеха пропущен: визард заменяет запись истории на returnTo,
  // дополненный параметром propertyId созданного объекта.
  await expect(page).toHaveURL(/\/properties\?propertyId=[0-9a-f-]{36}$/);
  await expect(page.getByRole('heading', { name: 'Объекты', exact: true })).toBeVisible();
  await expect(page.getByText('Гараж на Садовой 3', { exact: true })).toBeVisible();
});
