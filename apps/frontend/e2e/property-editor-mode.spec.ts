import type { Page } from '@playwright/test';
import {
  captureScreen,
  expect,
  openCabinetWithSessionToken,
  seededMemberSessionToken,
  test,
} from './fixtures';

// Деталь чужого объекта у участника-редактора (правки приёмки #757,
// макеты 2200-97255/97365): пилюля доступа «Редактирование» под
// заголовком вместо снесённого баннера «С вами делится…», контактный
// ряд владельца с почтой (owner_email в access-контракте) и
// «Управление» без мёртвых кнопок — «Перевести в архив» и «Удалить
// объект» владельческие (сервер отвечает 403), в шите статуса их тоже
// нет. Сид: Мария Петрова — full_access «Квартиры на Ленина» (#467).
const OWNER_EMAIL = 'e2e@example.com';

async function openEditorDetail(page: Page): Promise<void> {
  await openCabinetWithSessionToken(page, seededMemberSessionToken());
  // Путь пользователя: список → ряд (канон #698).
  await page.goto('/properties');
  await page
    .getByRole('link', { name: /Квартира на Ленина/ })
    .first()
    .click();

  await expect(page.getByText('Владелец объекта')).toBeVisible();
}

test.describe('деталь объекта у участника-редактора', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('пилюля доступа, владелец с почтой, управление без мёртвых кнопок', async (
    { page },
    testInfo,
  ) => {
    await openEditorDetail(page);

    // Пилюля «Редактирование» под заголовком (2200-97365); баннера нет.
    await expect(page.getByText('Редактирование', { exact: true })).toBeVisible();
    await expect(page.getByText(/С вами делится/)).toHaveCount(0);

    // Контактный ряд владельца: имя и почта (owner_email в контракте).
    await expect(page.getByText('Иван Иванов', { exact: true })).toBeVisible();
    await expect(page.getByText(OWNER_EMAIL, { exact: true })).toBeVisible();

    // «Управление»: правки и статус доступны, архив/удаление — нет.
    const manage = page.getByTestId('property-manage-list');
    await expect(manage.getByText('Редактировать объект')).toBeVisible();
    await expect(manage.getByText('Начать аренду')).toBeVisible();
    await expect(manage.getByText('Объект на ремонте')).toBeVisible();
    await expect(manage.getByText('Совместный доступ')).toBeVisible();
    await expect(manage.getByText('Покинуть объект')).toBeVisible();
    await expect(manage.getByText('Перевести в архив')).toHaveCount(0);
    await expect(manage.getByText('Удалить объект')).toHaveCount(0);

    // Шит статуса без архивного пункта.
    await page.getByRole('button', { name: 'Действия с объектом' }).click();
    await expect(page.getByRole('menuitem', { name: 'Изменить статус' })).toBeVisible();
    await page.getByRole('menuitem', { name: 'Изменить статус' }).click();
    await expect(page.getByText('Начать аренду').first()).toBeVisible();
    await expect(page.getByText('Перевести в архив')).toHaveCount(0);
    await page.keyboard.press('Escape');
    await page.waitForTimeout(300);

    await captureScreen(page, testInfo, '757-editor-detail');
  });
});
