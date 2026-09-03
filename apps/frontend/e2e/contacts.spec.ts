import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';

// Экран «Контакты объекта» и его вход (#511, карта #503): секция-ссылка
// «Контакты» на странице объекта ведёт на список нового хрома; сквозной
// жизненный цикл карточки — создание → поиск → деталка → правка →
// удаление — тем же путём, которым идёт владелец при приёмке.
//
// Имя контакта уникально за попытку (суффикс — номер retry): повтор
// упавшей попытки в CI создаёт карточку заново, а остатки прошлой попытки
// с другим суффиксом не попадают в проверки по имени. Тест сам удаляет
// контакт в конце; карточка-сирота от упавшей середины сценария остаётся
// в e2e-базе, но с уникальным именем — проверки по имени не задевает.

const PROPERTY_URL = `/properties/${SEEDED_APARTMENT_PROPERTY_ID}`;
const CONTACTS_URL = `${PROPERTY_URL}/contacts`;

test.describe('вход «Контакты» на карточке объекта', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('секция-ссылка «Контакты» ведёт на список контактов объекта', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(PROPERTY_URL);
    // Секция-ссылка идёт по образцу «Платежей» (#463): заголовок со
    // стрелкой, aria-label «Перейти в раздел «Контакты»».
    await page.getByRole('link', { name: 'Перейти в раздел «Контакты»' }).click();

    await expect(page).toHaveURL(new RegExp(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}/contacts$`));
    await expect(page.getByText('Контакты объекта', { exact: true })).toBeVisible();
  });
});

test.describe('жизненный цикл контакта', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('создать → деталка → изменить → удалить', async ({
    page,
    seededUser,
  }, testInfo) => {
    const contactName = `Сантехник Тест ${testInfo.retry}`;
    const updatedRole = 'электрик';

    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(CONTACTS_URL);

    // Создание (#509): полноширинная кнопка списка → форма с одним
    // обязательным полем «Имя».
    await page.getByRole('button', { name: 'Добавить контакт' }).click();
    await expect(page.getByRole('button', { name: 'Отменить создание' })).toBeVisible();
    await page.getByRole('textbox', { name: 'Имя *' }).fill(contactName);
    await page.getByRole('textbox', { name: 'Роль' }).fill('сантехник');
    // Сабмит двумя поверхностями — ✓ в шапке и полноширинная кнопка,
    // появляющаяся по готовности формы; жмём нижнюю.
    await page.getByRole('button', { name: 'Создать контакт' }).last().click();

    // Возврат на список; поиск (приёмочный путь «список (поиск)») находит
    // карточку по имени, закрытие поиска возвращает полный список.
    await expect(page).toHaveURL(/\/contacts$/);
    await page.getByRole('button', { name: 'Поиск' }).click();
    await page.getByRole('searchbox', { name: 'Поиск контактов' }).fill(contactName);
    await expect(page.getByText(contactName, { exact: true })).toBeVisible();
    // Тосты не автозакрываются (autoClose=false) и висят поверх шапки,
    // перехватывая клик по «Закрыть поиск» — гасим крестиком, как юзер.
    await page.locator('.Toastify').getByRole('button', { name: 'Закрыть' }).click();
    await page.getByRole('button', { name: 'Закрыть поиск' }).click();
    const row = page.getByText(contactName, { exact: true });
    await expect(row).toBeVisible();
    await row.click();

    // Деталка: заголовок-имя и роль на карточке.
    await expect(page).toHaveURL(/\/contacts\/[0-9a-f-]{36}$/);
    await expect(page.getByText('Контакт', { exact: true })).toBeVisible();
    await expect(page.getByText('сантехник', { exact: true })).toBeVisible();

    // Правка через кебаб: роль меняется, сохранение возвращает на деталку.
    await page.getByRole('button', { name: 'Меню контакта' }).click();
    await page.getByRole('menuitem', { name: 'Изменить' }).click();
    await expect(page.getByText('Изменить контакт', { exact: true })).toBeVisible();
    await page.getByRole('textbox', { name: 'Роль' }).fill(updatedRole);
    // Нижняя полноширинная кнопка; exact отсекает шапочное «Сохранить изменения».
    await page.getByRole('button', { name: 'Сохранить', exact: true }).click();

    await expect(page).toHaveURL(/\/contacts\/[0-9a-f-]{36}$/);
    await expect(page.getByText(updatedRole, { exact: true })).toBeVisible();

    await captureScreen(page, testInfo, 'contact-detail-updated');

    // Удаление через кебаб с подтверждением; возврат на список без карточки.
    await page.getByRole('button', { name: 'Меню контакта' }).click();
    await page.getByRole('menuitem', { name: 'Удалить' }).click();
    await expect(page.getByText('Удалить контакт?')).toBeVisible();
    await page.getByRole('button', { name: 'Удалить', exact: true }).click();

    await expect(page).toHaveURL(/\/contacts$/);
    await expect(page.getByText(contactName)).toHaveCount(0);
  });
});
