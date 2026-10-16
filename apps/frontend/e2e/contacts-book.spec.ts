import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  screenHeader,
  test,
} from './fixtures';

// Плоская книга контактов /contacts на едином хроме (карта #556, тикет
// #565): хаб-шапка («крылья» и на мобайле, заголовок раздела 28), подсветка
// «Контакты» в сайдбаре ПК и в шите «Еще», сквозной жизненный цикл
// карточки книги — создание (CTA пустоты, «+» ряда заголовка) → поиск →
// деталка → удаление.
//
// Сид контактов пуст, карточку каждая попытка создаёт и удаляет сама; имя
// уникально за попытку (суффикс — номер retry), поэтому параллельные
// попытки и остатки прошлых не задевают проверки по имени.

const CONTACT_NAME = (retry: number) => `Сантехник Книги ${retry}`;

test.describe('книга контактов — хаб нового хрома', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('хаб-шапка с крыльями на мобайле, «Контакты» подсвечен в шите «Еще»', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/contacts');

    // Хаб-шапка: крылья и на мобайле (лого — ссылка на объекты).
    await expect(screenHeader(page).getByRole('link', { name: 'Объекты' })).toBeVisible();
    await expect(
      page.getByRole('heading', { level: 1, name: 'Контакты', exact: true }),
    ).toBeVisible();
    // Пустая книга (#1004): пилюли поиска нет — искать нечего; создание —
    // CTA пустого состояния.
    await expect(page.getByRole('button', { name: 'Найти контакт' })).toHaveCount(0);
    await expect(
      page.getByRole('button', { name: 'Добавить контакт', exact: true }),
    ).toBeVisible();

    // Таб активен «Еще», в шите пункт «Контакты» подсвечен; тап по пункту
    // закрывает шит и остаётся в книге.
    await page.getByRole('button', { name: 'Еще' }).click();
    const moreSheet = page.getByRole('dialog', { name: 'Еще' });
    await expect(
      moreSheet.getByRole('link', { name: 'Контакты' }),
    ).toHaveAttribute('aria-current', 'page');
    await moreSheet.getByRole('link', { name: 'Контакты' }).click();
    await expect(page).toHaveURL(/\/contacts$/);

    await captureScreen(page, testInfo, 'contacts-book-hub-mobile');
  });

  // Первый блок по макетам карты #1232 (3226-74333/3229-94602, тикет
  // #1238): «+» создания — в ряду заголовка и в компакт-баре, пилюля —
  // чистый канон без «+»; на подтверждённой пустоте «+» нет — создание
  // остаётся CTA пустого состояния (#1004).
  test('«+» создания — в ряду заголовка на непустой книге, пилюля без «+»', async ({
    page,
    seededUser,
  }, testInfo) => {
    const contactName = CONTACT_NAME(testInfo.retry);

    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/contacts');

    // Пустая книга (#1004): «+» нет ни в ряду заголовка, ни в компакт-баре.
    await expect(page.getByTestId('contacts-create')).toHaveCount(0);
    await expect(page.getByTestId('contacts-create-compact')).toHaveCount(0);

    // Делаем книгу непустой тем же путём, каким идёт владелец: CTA пустоты
    // → форма → возврат в книгу.
    await page.getByRole('button', { name: 'Добавить контакт', exact: true }).click();
    await page.getByRole('textbox', { name: 'Имя', exact: true }).fill(contactName);
    await page.getByRole('button', { name: 'Создать контакт' }).last().click();
    await expect(page).toHaveURL(/\/contacts$/);
    // Тост висит поверх шапки и перехватывает клик по «+» — гасим крестиком,
    // как юзер (паттерн теста «создание из книги»).
    await page.locator('.Toastify').getByRole('button', { name: 'Закрыть' }).click();

    // «+» в ряду заголовка; пилюля — чистый канон, «+» внутри неё больше нет.
    await expect(page.getByTestId('contacts-create')).toBeVisible();
    await expect(
      page
        .getByRole('button', { name: 'Найти контакт' })
        .getByRole('button', { name: 'Добавить контакт' }),
    ).toHaveCount(0);
    await page.getByTestId('contacts-create').click();
    await expect(page).toHaveURL(/\/contacts\/new$/);

    // Уборка попытки: назад в книгу, карточка удаляется поиском (паттерн
    // теста «создание из книги»).
    await page.goBack();
    await expect(page).toHaveURL(/\/contacts$/);
    await page.getByRole('button', { name: 'Найти контакт' }).click();
    await page.getByRole('searchbox', { name: 'Поиск контактов' }).fill(contactName);
    await page.getByText(contactName, { exact: true }).click();
    await page.getByRole('button', { name: 'Меню контакта' }).click();
    await page.getByRole('menuitem', { name: 'Удалить' }).click();
    await page.getByRole('button', { name: 'Удалить', exact: true }).click();

    await expect(page).toHaveURL(/\/contacts\/search$/);
    await page.locator('.Toastify').getByRole('button', { name: 'Закрыть' }).click();
    await page.getByRole('button', { name: 'Закрыть поиск' }).click();
    await expect(page).toHaveURL(/\/contacts$/);
    await expect(page.getByText(contactName)).toHaveCount(0);
  });

  test('создание из книги: CTA пустого состояния → форма → возврат в книгу', async ({
    page,
    seededUser,
  }, testInfo) => {
    const contactName = CONTACT_NAME(testInfo.retry);

    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/contacts');

    // Пустая книга (#1004): создание — CTA пустого состояния; на заполненной
    // книге создание — «+» ряда заголовка (тикет #1238).
    await page.getByRole('button', { name: 'Добавить контакт', exact: true }).click();
    await expect(page.getByRole('button', { name: 'Отменить создание' })).toBeVisible();
    // Обязательное поле «Имя»: доступное имя без звёздочки — маркер
    // обязательности aria-hidden и из accname исключён (design TextField).
    await page.getByRole('textbox', { name: 'Имя', exact: true }).fill(contactName);
    await page.getByRole('button', { name: 'Создать контакт' }).last().click();

    // Возврат в корень книги; карточка находится поиском книги.
    await expect(page).toHaveURL(/\/contacts$/);
    // Тосты не автозакрываются и висят поверх шапки, перехватывая клик по
    // пилюле — гасим крестиком, как юзер (паттерн contacts.spec).
    await page.locator('.Toastify').getByRole('button', { name: 'Закрыть' }).click();
    await page.getByRole('button', { name: 'Найти контакт' }).click();
    await page.getByRole('searchbox', { name: 'Поиск контактов' }).fill(contactName);
    await expect(page.getByText(contactName, { exact: true })).toBeVisible();

    await captureScreen(page, testInfo, 'contacts-book-search');

    // Тап по находке открывает карточку книги; удаление — уборка попытки.
    // goBack после удаления возвращается по истории — в поиск, откуда
    // карточка открыта; закрытие поиска возвращает в книгу без карточки.
    await page.getByText(contactName, { exact: true }).click();
    await expect(page.getByText('Контакт', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Меню контакта' }).click();
    await page.getByRole('menuitem', { name: 'Удалить' }).click();
    await page.getByRole('button', { name: 'Удалить', exact: true }).click();

    await expect(page).toHaveURL(/\/contacts\/search$/);
    await page.locator('.Toastify').getByRole('button', { name: 'Закрыть' }).click();
    await page.getByRole('button', { name: 'Закрыть поиск' }).click();
    await expect(page).toHaveURL(/\/contacts$/);
    await expect(page.getByText(contactName)).toHaveCount(0);
  });
});

test.describe('книга контактов — десктоп', () => {
  test('сайдбар подсвечивает «Контакты», хаб-шапка с крыльями', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/contacts');

    const sidebar = page.getByRole('navigation', { name: 'Основная навигация' });
    await expect(
      sidebar.getByRole('link', { name: 'Контакты' }),
    ).toHaveAttribute('aria-current', 'page');
    await expect(
      page.getByRole('heading', { level: 1, name: 'Контакты', exact: true }),
    ).toBeVisible();

    await captureScreen(page, testInfo, 'contacts-book-hub-desktop');
  });
});
