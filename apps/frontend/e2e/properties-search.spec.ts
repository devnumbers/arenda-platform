import { expect, openCabinetWithSeededSession, test } from './fixtures';

// Открытие поиска с пустым полем — подсказка «Введите название или адрес»
// (канон соседей, #601), не скелетон: хук при пустом запросе выключен
// (ad9b5b4d6 — 400-контракт), а выключенный запрос TanStack v5 держит
// isPending вечно — гейт скелетона обязан сидеть за searching.
test('пустое поле при открытии — подсказка, не скелетон', async ({
  page,
  seededUser,
}) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties/search');

  // Сайдбар дозагрузился (UserButton рисует имя) — пульсы шапки не
  // подмешиваются в ассерты скелетона.
  await expect(page.getByText('Иван', { exact: true })).toBeVisible();

  await expect(
    page.getByText('Введите название или адрес объекта'),
  ).toBeVisible();
  await expect(page.locator('div[aria-hidden] .animate-pulse')).toHaveCount(0);
});

// Поиск объектов (тикет #601): stale empty state — план 029 конвейера
// скелетонов (#1437/#1442). После осевшего пустого ответа правка запроса
// показывает скелетон, а не «Такого объекта нет» за летящим запросом:
// держать пустые прежние результаты нельзя (расширение политики тихого
// удержания operations-feed-gate), осевший пустой ответ — «Такого объекта
// нет», как раньше.
test('stale empty state: после пустого ответа правка запроса — скелетон, не «Такого объекта нет»', async ({
  page,
  seededUser,
}) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties/search');

  // Сайдбар дозагрузился (UserButton рисует имя) — пульсы шапки не
  // подмешиваются в дальнейшие ассерты скелетона.
  await expect(page.getByText('Иван', { exact: true })).toBeVisible();

  // Осевший пустой ответ: бессмысленная строка даёт «Такого объекта нет».
  const searchField = page.getByPlaceholder('Найти объект');
  await searchField.fill('ззз-none');
  await expect(page.getByText('Такого объекта нет')).toBeVisible({ timeout: 5_000 });

  // Правка запроса: новый ответ задержан route-заморозкой — до его прихода
  // виден скелетон поиска (пульсы строк), определительное пустое состояние
  // за летящий запрос отвечать не имеет права. Дебаунс 300 мс покрывается
  // таймаутом ассерта.
  await page.route('**/api/properties/search**', async (route) => {
    await page.waitForTimeout(2_000);
    await route.continue();
  });
  await searchField.fill('ззз-none2');

  const skeletonRows = page.locator('div[aria-hidden] .animate-pulse');
  await expect(skeletonRows.first()).toBeVisible();
  await expect(page.getByText('Такого объекта нет')).toBeHidden();
  await expect(page.getByTestId('properties-search-results')).toHaveCount(0);

  // Осевший пустой ответ нового запроса возвращает «Такого объекта нет».
  await expect(page.getByText('Такого объекта нет')).toBeVisible({ timeout: 10_000 });
  await page.unrouteAll({ behavior: 'ignoreErrors' });
});
