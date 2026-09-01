import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Визард создания объекта: каркас флоу и шаг «Выбор категории» (#480,
// Figma 1213-52111), шаг «Адрес» с подсказками (#481, Figma 1213-52017/
// 52391, 1519-94336). Один маршрут /properties/new, шаги — клиентское
// состояние, черновик переживает перезагрузку (sessionStorage).
//
// Живые подсказки проверяются только при настроенном стабе Dadata
// (DADATA_BASE_URL локального прогона): в CI ключа нет, endpoint тихо
// деградирует — каркасные тесты от этого не зависят.

const dadataStubConfigured = process.env.DADATA_BASE_URL !== undefined;

async function openWizard(page: Parameters<typeof openCabinetWithSeededSession>[0], user: Parameters<typeof openCabinetWithSeededSession>[1]): Promise<void> {
  await openCabinetWithSeededSession(page, user);
  await page.goto('/properties');
  await page.getByRole('link', { name: 'Добавить объект' }).click();
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
  await expect(page.getByRole('heading', { name: 'Мои объекты' })).toBeVisible();
});

test('шаг 2: подсказки адреса — список, выбор, возврат после правки', async ({ page, seededUser }, testInfo) => {
  test.skip(!dadataStubConfigured, 'живые подсказки — только с стабом Dadata (DADATA_BASE_URL)');

  await openWizard(page, seededUser);
  await page.getByRole('group', { name: 'Категория объекта' }).getByRole('button', { name: 'Гараж' }).click();
  const address = page.getByRole('textbox', { name: 'Введите адрес' });
  await expect(address).toBeFocused();

  // До трёх символов подсказок нет; со строки «Ленина» приходит список:
  // заголовок — улица и дом (город отрезан), подпись — город.
  await address.fill('Ле');
  await expect(page.getByRole('button', { name: /Ленина/ })).toHaveCount(0);
  await address.fill('Ленина');
  const firstRow = page.getByRole('button', { name: 'Ленина, д. 1 Москва' });
  await expect(firstRow).toBeVisible();
  await expect(page.getByRole('button', { name: 'Ленина, 5 Санкт-Петербург' })).toBeVisible();
  await captureScreen(page, testInfo, '02-address-suggestions');

  // Escape прячет список, правка значения возвращает его.
  await address.press('Escape');
  await expect(firstRow).toBeHidden();
  await address.press('a');
  await expect(firstRow).toBeVisible();

  // Выбор подсказки: полный адрес в поле, список скрыт, «Продолжить» есть.
  await firstRow.click();
  await expect(address).toHaveValue('г. Москва, Ленина, д. 1');
  await expect(firstRow).toBeHidden();
  await expect(page.getByRole('button', { name: 'Продолжить' })).toBeVisible();
  await captureScreen(page, testInfo, '03-address-picked');
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
