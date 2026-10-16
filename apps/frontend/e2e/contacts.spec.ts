import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  openCabinetWithSessionToken,
  seededMemberSessionToken,
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
    // Карточка объекта #583/#588: секция-ссылка несёт имя заголовка секции.
    await page.getByRole('link', { name: 'Контакты' }).click();

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
    // Обязательное поле «Имя»: доступное имя без звёздочки — маркер
    // обязательности aria-hidden и из accname исключён (design TextField).
    await page.getByRole('textbox', { name: 'Имя', exact: true }).fill(contactName);
    await page.getByRole('textbox', { name: 'Роль' }).fill('сантехник');
    // Телефон — для пина копирующей плашки на деталке (слово владельца
    // 12.10.2026: вся строка значения копирует).
    await page.getByRole('textbox', { name: 'Телефон' }).fill('+7 (993) 123-12-12');
    // Сабмит двумя поверхностями — ✓ в шапке и полноширинная кнопка,
    // появляющаяся по готовности формы; жмём нижнюю.
    await page.getByRole('button', { name: 'Создать контакт' }).last().click();

    // Возврат на список; поиск (приёмочный путь «список (поиск)») находит
    // карточку по имени, закрытие поиска возвращает полный список.
    await expect(page).toHaveURL(/\/contacts$/);
    await page.getByRole('button', { name: 'Поиск' }).click();
    await page.getByRole('searchbox', { name: 'Поиск контактов' }).fill(contactName);
    await expect(page.getByText(contactName, { exact: true })).toBeVisible();
    // Тост уходит сам за 5с (TOAST_AUTO_CLOSE_MS) и висит поверх шапки,
    // перехватывая клик по «Закрыть поиск» — гасим крестиком, как юзер;
    // холодный рефetch списка после создания может занять больше 5с —
    // тогда тост уже аутклоузился, и гасить нечего (флейк 12.10.2026).
    const toastClose = page.locator('.Toastify').getByRole('button', { name: 'Закрыть' });
    await toastClose.click({ timeout: 2000 }).catch(() => {});
    await page.getByRole('button', { name: 'Закрыть поиск' }).click();
    const row = page.getByText(contactName, { exact: true });
    await expect(row).toBeVisible();
    await row.click();

    // Деталка: заголовок-имя и роль на фоне страницы (компоновка #1333).
    await expect(page).toHaveURL(/\/contacts\/[0-9a-f-]{36}$/);
    await expect(page.getByText('Контакт', { exact: true })).toBeVisible();
    await expect(page.getByText('сантехник', { exact: true })).toBeVisible();

    // Правка тапом по карточке «Информация» (#1333, карта #1332: правка
    // переехала из кебаба, за кебабом — только удаление): роль меняется,
    // сохранение возвращает на деталку.
    await page.getByRole('link', { name: 'Информация' }).click();
    await expect(page.getByText('Изменить контакт', { exact: true })).toBeVisible();
    await page.getByRole('textbox', { name: 'Роль' }).fill(updatedRole);
    // Нижняя полноширинная кнопка; exact отсекает шапочное «Сохранить изменения».
    await page.getByRole('button', { name: 'Сохранить', exact: true }).click();

    await expect(page).toHaveURL(/\/contacts\/[0-9a-f-]{36}$/);
    await expect(page.getByText(updatedRole, { exact: true })).toBeVisible();

    await captureScreen(page, testInfo, 'contact-detail-updated');

    // Тап по плашке значения копирует и не уводит на правку (#1333, слово
    // владельца 12.10.2026): URL на месте, строка переходит в состояние
    // «Скопировано» (aria-label всей строки-кнопки; буферу не нужен
    // permission — подсказка канона useCopiedHint горит независимо).
    await page.getByText('+7 (993) 123-12-12', { exact: true }).click();
    await expect(page).toHaveURL(/\/contacts\/[0-9a-f-]{36}$/);
    await expect(page.getByRole('button', { name: 'Скопировано', exact: true })).toBeVisible();
    // Правка остаётся на тапе по заголовку карточки.
    await page.getByRole('link', { name: 'Информация' }).click();
    await expect(page.getByText('Изменить контакт', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(/\/contacts\/[0-9a-f-]{36}$/);

    // Удаление через кебаб с подтверждением; кебаб — только удаление
    // (#1333): пункта «Изменить» у него больше нет. Возврат на список
    // без карточки.
    await page.getByRole('button', { name: 'Меню контакта' }).click();
    await expect(page.getByRole('menuitem', { name: 'Изменить' })).toHaveCount(0);
    await page.getByRole('menuitem', { name: 'Удалить' }).click();
    await expect(page.getByText('Удалить контакт?')).toBeVisible();
    await page.getByRole('button', { name: 'Удалить', exact: true }).click();

    await expect(page).toHaveURL(/\/contacts$/);
    await expect(page.getByText(contactName)).toHaveCount(0);
  });
});

test.describe('граница прав на перепривязке (D10 #1309)', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('полный доступ: инпут объекта в правке карточки глухой', async ({ page }) => {
    await openCabinetWithSessionToken(page, seededMemberSessionToken());

    // Привязанная карточка API-вызовом от сессии участника: привязка при
    // создании — правомерна, в отличие от перепривязки (D10 #1309).
    const created = await page.request.post('/api/contacts', {
      data: {
        firstName: 'Сантехник от участника',
        role: 'сантехник',
        propertyId: SEEDED_APARTMENT_PROPERTY_ID,
      },
    });
    expect(created.ok(), 'создание привязанной карточки полным доступом').toBe(true);
    const contact = (await created.json()) as { id: string };

    await page.goto(`/contacts/${contact.id}/edit`);
    await expect(page.getByText('Изменить контакт', { exact: true })).toBeVisible();
    // Граница прав видна до попытки сохранить: инпут глухой, страницу
    // выбора объект-пикера участнику форма не открывает.
    const objectSelect = page.getByRole('button', { name: /Привязанный объект/ });
    await expect(objectSelect).toBeDisabled();
    await expect(page.getByText('Выбрать объект', { exact: true })).toHaveCount(0);

    // Уборка: карточка уходит из книги владельца квартиры (удаление —
    // правом редактирования, участнику доступно).
    const deleted = await page.request.delete(`/api/contacts/${contact.id}`);
    expect(deleted.ok(), 'уборка карточки').toBe(true);
  });
});
