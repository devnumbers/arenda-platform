import {
  captureScreen,
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  SEEDED_STUDIO_PROPERTY_ID,
  test,
} from './fixtures';

// Вход в архив с пустого хаба (карта #1047, тикет #1051): когда все
// объекты в архиве, хаб раньше прятал ряд сортировки вместе с кнопкой
// «Архив» — на архив попадали только прямым URL. Решение владельца
// 02.10.2026 (вариант 1А «на обычном месте»): ряд сортировки на пустоте
// остаётся, «Архив» справа на своём месте; чип сортировки прячется
// (канон §7), поиск остаётся спрятанным (#1004).
// Сид архивных не содержит: все три сид-объекта (#1003 добавил студию)
// архивируем SQL-ом посреди теста и восстанавливаем в finally
// (workers=1, соседние спеки читают те же объекты) — по образцу
// property-archived-deeplink.spec.ts.

const SEEDED_PROPERTY_IDS = [
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  SEEDED_STUDIO_PROPERTY_ID,
] as const;
const SEEDED_PROPERTY_NAMES = [
  'Квартира на Ленина',
  'Гараж на Садовой',
  'Студия на Полевой',
] as const;

async function setAllPropertiesStatus(status: 'archived' | 'active'): Promise<void> {
  for (const id of SEEDED_PROPERTY_IDS) {
    await execE2eSql(
      status === 'archived'
        ? `UPDATE properties SET status = 'archived', pinned_at = NULL WHERE id = '${id}'`
        : `UPDATE properties SET status = 'active' WHERE id = '${id}'`,
    );
  }
}

test.describe('вход в архив с пустого хаба #1051', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('все объекты в архиве: «Архив» на обычном месте, архив открывается', async ({
    page,
    seededUser,
  }, testInfo) => {
    await setAllPropertiesStatus('archived');
    try {
      await openCabinetWithSeededSession(page, seededUser);
      await page.goto('/properties');

      // Пустой хаб: пустое состояние на месте (канон #1004 не менялся),
      // поиск спрятан, а ряд сортировки остался — «Архив» справа.
      await expect(page.getByText('Объектов нет', { exact: true })).toBeVisible();
      await expect(page.getByTestId('properties-search-pill')).toHaveCount(0);

      const sortRow = page.getByTestId('properties-sort-row');
      await expect(sortRow).toBeVisible();
      const archiveButton = sortRow.getByRole('button', { name: 'Архив' });
      await expect(archiveButton).toBeVisible();

      // Чип сортировки на пустоте спрятан — сортировать нечего (§7).
      await expect(sortRow.getByText('Название', { exact: true })).toHaveCount(0);

      await captureScreen(page, testInfo, '1051-empty-hub-archive-visible');

      // Тап по «Архив» ведёт на экран архива со всеми объектами.
      await archiveButton.click();
      await expect(page).toHaveURL(new RegExp('/properties/archive$'));
      await expect(page.getByText('Архивные объекты')).toBeVisible();
      for (const name of SEEDED_PROPERTY_NAMES) {
        await expect(page.getByText(name, { exact: true })).toBeVisible();
      }
    } finally {
      await setAllPropertiesStatus('active');
    }
  });
});

