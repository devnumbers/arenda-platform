import type { Page } from '@playwright/test';
import {
  captureScreen,
  expect,
  execE2eSql,
  openCabinetWithSessionToken,
  seededViewerSessionToken,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';

// Режим просмотра (карта #692, тикет #703): центральные права из
// property.access и чеклист поверхностей по макетам 2235-*. Сид: Сергей
// Сидоров — зритель «Квартиры на Ленина» (роль viewer, #467) и не имеет
// своих объектов, то есть чистый зритель. Проверки: деталь объекта
// («Владелец объекта», «Управление» = об объекте/совместный доступ/
// покинуть), скрытие «+» в разделах и глобальной ленте операций, экран
// участников без manage-контролов, флоу «Покинуть объект» с SQL-правдой.
// Выход из объекта сносит сидовое членство — восстанавливаем его в конце
// теста (workers=1, файл идёт последним по алфавиту среди новых прогонов).

const PROPERTY = SEEDED_APARTMENT_PROPERTY_ID;
const VIEWER_MEMBERSHIP_ID = '99999999-9999-4999-8999-999999999932';
const VIEWER_USER_ID = '13111111-1111-4111-8111-111111111131';
const OWNER_ID = '11111111-1111-4111-8111-111111111111';

async function openViewerCabinet(
  page: Page,
): Promise<void> {
  await openCabinetWithSessionToken(page, seededViewerSessionToken());
}

async function openDetail(page: Page): Promise<void> {
  await openViewerCabinet(page);
  // Путь пользователя (канон #698): список → ряд. Прямой goto ломает
  // goBack-навигацию после выхода — история уводит мимо приложения.
  await page.goto('/properties');
  await page
    .getByRole('link', { name: /Квартира на Ленина/ })
    .first()
    .click();
  await expect(page.getByText('Владелец объекта')).toBeVisible();
}

test.describe('режим просмотра', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('деталь объекта: секция владельца, управление зрителя, кебаб без мутаций', async (
    { page },
    testInfo,
  ) => {
    await openDetail(page);

    // Секция «Владелец объекта» (2200-97365): имя и почта владельца из
    // контракта; пилюля «Просмотр» под заголовком вместо баннера (#757).
    await expect(page.getByText('Иван Иванов', { exact: true })).toBeVisible();
    await expect(page.getByText('e2e@example.com', { exact: true })).toBeVisible();
    await expect(page.getByText('Просмотр', { exact: true })).toBeVisible();

    // «Управление» — ровно три ряда зрителя; мутирующих строк нет.
    const manage = page.getByTestId('property-manage-list');
    await expect(manage.getByText('Об объекте')).toBeVisible();
    await expect(manage.getByText('Совместный доступ')).toBeVisible();
    await expect(manage.getByText('Покинуть объект')).toBeVisible();
    await expect(manage.getByText('Редактировать объект')).toHaveCount(0);
    await expect(manage.getByText('Удалить объект')).toHaveCount(0);
    await expect(manage.getByText('Начать аренду')).toHaveCount(0);

    // Кебаб (2235-100370): об объекте и совместный доступ, без статусов.
    await page.getByRole('button', { name: 'Действия с объектом' }).click();
    await expect(page.getByRole('menuitem', { name: 'Об объекте' })).toBeVisible();
    await expect(
      page.getByRole('menuitem', { name: 'Совместный доступ' }),
    ).toBeVisible();
    await expect(page.getByRole('menuitem', { name: 'Изменить статус' })).toHaveCount(0);
    await page.keyboard.press('Escape');

    await captureScreen(page, testInfo, '703-viewer-detail');
  });

  test('создание скрыто в разделах объекта и в глобальной ленте операций', async ({
    page,
  }) => {
    await openViewerCabinet(page);

    await page.goto(`/properties/${PROPERTY}/tasks`);
    await expect(page.getByRole('button', { name: 'Создать задачу' })).toHaveCount(0);

    await page.goto(`/properties/${PROPERTY}/operations`);
    await expect(page.getByRole('button', { name: 'Добавить операцию' })).toHaveCount(0);

    // Чистый зритель: редактируемых объектов нет — «+» глобальной ленты
    // и CTA пустой книги скрыты (макет 2235-100976).
    await page.goto('/operations');
    await expect(page.getByRole('button', { name: 'Добавить операцию' })).toHaveCount(0);
  });

  test('прямые ссылки форм гасятся карточками недоступности (обход #758)', async ({
    page,
  }) => {
    await openViewerCabinet(page);

    // Правка объекта (deep-link /edit): гард #607 — карточка вместо формы;
    // без него сервер отвечал 403, а тост винил соединение.
    await page.goto(`/properties/${PROPERTY}/edit`);
    await expect(page.getByText('Правка недоступна')).toBeVisible();
    await expect(
      page.getByText('У вас доступ только для просмотра этого объекта'),
    ).toBeVisible();
    await expect(page.getByText('Название объекта')).toHaveCount(0);

    // Создание контакта в объекте: карточка вместо формы с предвыбранным
    // объектом; без гарда сервер отвечал 403 на отправку.
    await page.goto(`/properties/${PROPERTY}/contacts/new`);
    await expect(page.getByText('Создание недоступно')).toBeVisible();
    await expect(
      page.getByText('У вас доступ только для просмотра этого объекта'),
    ).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Имя' })).toHaveCount(0);
  });

  test('участники объекта: список читается, manage-контролы скрыты', async ({ page }) => {
    await openViewerCabinet(page);
    await page.goto(`/properties/${PROPERTY}/participants`);

    // Чтение сохранено: владелец и сам зритель в списке (2235-103698);
    // собственный ряд подписан «(Вы)».
    await expect(page.getByText('Иван Иванов', { exact: true })).toBeVisible();
    await expect(page.getByText('Сергей Сидоров (Вы)', { exact: true })).toBeVisible();

    // Manage-контролы зрителю не рисуются: кебаб и CTA приглашения.
    await expect(
      page.getByRole('button', { name: 'Еще — действия со списком' }),
    ).toHaveCount(0);
    await expect(
      page.getByRole('button', { name: 'Пригласить участника' }),
    ).toHaveCount(0);
  });

  test('покинуть объект: отмена ничего не делает, выход сносит ногу (SQL)', async ({
    page,
  }) => {
    await openDetail(page);

    // Отмена: диалог закрывается, деталь остаётся.
    await page.getByTestId('property-manage-list').getByText('Покинуть объект').click();
    const dialog = page.getByRole('dialog');
    await expect(
      dialog.getByText('Уверены, что хотите покинуть объект?'),
    ).toBeVisible();
    await dialog.getByRole('button', { name: 'Отмена' }).click();
    await expect(dialog).toHaveCount(0);
    await expect(page.getByText('Владелец объекта')).toBeVisible();

    // Выход: канон-подтверждение #701 (2010-132970), тост, возврат к
    // списку объектов по истории (путь пользователя, канон #698).
    await page.getByTestId('property-manage-list').getByText('Покинуть объект').click();
    await dialog.getByRole('button', { name: 'Покинуть', exact: true }).click();
    await expect(page.getByText('Вы покинули объект')).toBeVisible();
    await expect(page).toHaveURL(/\/properties$/);

    // Серверная правда: членство зрителя удалено.
    const count = await execE2eSql(
      `SELECT count(*) FROM property_members WHERE id = '${VIEWER_MEMBERSHIP_ID}'`,
    );
    expect(count).toBe('0');

    // Восстановление сида: зритель квартиры нужен другим спекам.
    await execE2eSql(
      'INSERT INTO property_members (id, property_id, user_id, role, granted_by, status) VALUES ' +
        `('${VIEWER_MEMBERSHIP_ID}', '${PROPERTY}', '${VIEWER_USER_ID}', 'viewer', '${OWNER_ID}', 'active') ` +
        'ON CONFLICT (id) DO NOTHING',
    );
  });
});
