import type { Page } from '@playwright/test';
import {
  captureScreen,
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  openCabinetWithSessionToken,
  seededViewerSessionToken,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  test,
} from './fixtures';

// Гард архивного объекта по deep-link (карта #692, хвост Х5, тикет #773):
// архивный объект не живёт ни в одной ленте (SQL фильтрует active/
// maintenance), поэтому единственный путь к нему — прямая ссылка — обязан
// нести признак архива. Сид архивных не содержит: статус ставим SQL-ом
// посреди теста и восстанавливаем в finally (workers=1, соседние спеки
// читают те же объекты). Серверная правда не ломается: GET читается,
// мутации контента сервер режет сам — фронт не рисует мёртвых кнопок.

const APARTMENT = SEEDED_APARTMENT_PROPERTY_ID;
const GARAGE = SEEDED_GARAGE_PROPERTY_ID;

async function archiveProperty(id: string): Promise<void> {
  await execE2eSql(`UPDATE properties SET status = 'archived', pinned_at = NULL WHERE id = '${id}'`);
}

async function restoreProperty(id: string): Promise<void> {
  await execE2eSql(`UPDATE properties SET status = 'active' WHERE id = '${id}'`);
}

/** Все CTA «Добавить» пустых секций глухие (канон #589: архив read-only). */
async function expectAddButtonsDisabled(page: Page): Promise<void> {
  const addButtons = page.getByRole('button', { name: 'Добавить' });
  const count = await addButtons.count();
  expect(count).toBeGreaterThan(0);
  for (let i = 0; i < count; i += 1) {
    await expect(addButtons.nth(i)).toBeDisabled();
  }
}

test.describe('архивный объект по deep-link #773', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('чужой архивный: деталь читается, пилюля «В архиве», мутационных кнопок нет', async ({
    page,
  }, testInfo) => {
    await archiveProperty(APARTMENT);
    try {
      await openCabinetWithSessionToken(page, seededViewerSessionToken());
      await page.goto(`/properties/${APARTMENT}`);

      // Признак архива: пилюля под заголовком (канон 1603-92103) рядом
      // с пилюлей роли; деталь читается как раньше.
      const pill = page.getByTestId('property-archived-pill');
      await expect(pill).toBeVisible();
      await expect(pill).toHaveText('В архиве');
      await expect(page.getByText('Просмотр', { exact: true })).toBeVisible();
      await expect(page.getByText('Владелец объекта')).toBeVisible();

      // «Управление» зрителя — чтение и выход, мутаций и возврата из
      // архива (владельческое) нет.
      const manage = page.getByTestId('property-manage-list');
      await expect(manage.getByText('Об объекте')).toBeVisible();
      await expect(manage.getByText('Совместный доступ')).toBeVisible();
      await expect(manage.getByText('Покинуть объект')).toBeVisible();
      await expect(manage.getByText('Вернуть из архива')).toHaveCount(0);
      await expect(manage.getByText('Редактировать объект')).toHaveCount(0);
      await expect(manage.getByText('Удалить объект')).toHaveCount(0);

      // Create-CTA пустых секций зрителю не рисуются вовсе.
      await expect(page.getByRole('button', { name: 'Добавить' })).toHaveCount(0);

      await captureScreen(page, testInfo, '773-viewer-archived-deeplink');
    } finally {
      await restoreProperty(APARTMENT);
    }
  });

  test('свой архивный: пилюля, возврат из архива владельцу, CTA глухие', async ({
    page,
    seededUser,
  }, testInfo) => {
    await archiveProperty(GARAGE);
    try {
      await openCabinetWithSeededSession(page, seededUser);
      await page.goto(`/properties/${GARAGE}`);

      await expect(page.getByTestId('property-archived-pill')).toBeVisible();

      // Владелец сохраняет жизненный цикл: вернуть из архива и удалить.
      const manage = page.getByTestId('property-manage-list');
      await expect(manage.getByText('Вернуть из архива')).toBeVisible();
      await expect(manage.getByText('Совместный доступ')).toBeVisible();
      await expect(manage.getByText('Удалить объект')).toBeVisible();
      await expect(manage.getByText('Начать аренду')).toHaveCount(0);
      await expect(manage.getByText('Редактировать объект')).toHaveCount(0);

      // Архив read-only (ADR 0028): CTA есть (канон #589), но глухие.
      await expectAddButtonsDisabled(page);

      await captureScreen(page, testInfo, '773-owner-archived-deeplink');
    } finally {
      await restoreProperty(GARAGE);
    }
  });
});
