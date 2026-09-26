import {
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Ширина колонки контента — гардлаил правила CODING_STANDARDS §Screen
// shell (решение владельца 26.09, аудит #870): колонка — ровно кап
// PageContent (max-w-column, 560), контентные блоки сидят внутри со своим
// px-6 и не могут быть шире колонки ни на одном брейкпоинте. Негативная
// маржин-компенсация к обёртке кабинета растягивала хабы /payments и
// /payments/objects до 600px на вьюпортах <1200 — за кап; спек меряет
// прямых детей .max-w-column на каждой ширине, включая 1100 — внутри
// десктопного яруса, где ветвятся arbitrary-брейкпоинты (на 1440 тот хак
// спал).

/** Хабы и подэкран: страницы с контентом в колонке. */
const PAGES = [
  '/properties',
  '/operations',
  '/payments',
  '/payments/objects',
  '/payments/favorites',
];

/** Ярусы и произвольные брейкпоинты: 1440 и 1100 — десктоп (кап 560
 * проверяется на обоих), 375 — мобайл (колонка = вьюпорт, тот же
 * инвариант «ребёнок не шире колонки» ловит и вылезание за экран). */
const WIDTHS = [
  { width: 1440, height: 900 },
  { width: 1100, height: 900 },
  { width: 375, height: 812 },
];

for (const path of PAGES) {
  test(`контент не шире колонки: ${path}`, async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);

    for (const viewport of WIDTHS) {
      await page.setViewportSize(viewport);
      await page.goto(path);
      await page.waitForLoadState('load');

      const columns = page.locator('.max-w-column');
      await expect(columns.first()).toBeVisible();

      // В RSC-стриме рядом живут и loading-фолбэк, и живой экран — на
      // странице бывает несколько .max-w-column, правилу подчиняется каждая
      // отрисованная (скрытая копия стрима даёт нулевой прямоугольник).
      const verdict = (
        await columns.evaluateAll((columns) =>
          columns.map((column) => {
            const columnWidth = column.getBoundingClientRect().width;
            if (columnWidth === 0) return null;
            // Позиционированные оверлеи (тосты, шиты) не претендуют на ритм
            // колонки — меряются только блоки потока.
            const flowing = [...column.children].filter((el) => {
              const position = getComputedStyle(el).position;
              return position !== 'absolute' && position !== 'fixed';
            });
            const offenders = flowing
              .filter((el) => el.getBoundingClientRect().width > Math.ceil(columnWidth) + 1)
              .map((el) => ({
                cls: (el.className || '').toString().slice(0, 80),
                width: Math.round(el.getBoundingClientRect().width),
              }));
            return {
              columnWidth: Math.round(columnWidth),
              childCount: flowing.length,
              offenders,
            };
          }),
        )
      ).filter((v) => v !== null);

      // Страница непуста: хоть в одной колонке есть блок (иначе проверка
      // проходила бы вакуумно на пустых PageContent).
      expect(
        verdict.reduce((sum, v) => sum + v.childCount, 0),
        `${path} @${viewport.width}: ни одного блока в колонках`,
      ).toBeGreaterThan(0);

      for (const v of verdict) {
        // Кап колонки: на ярусах ≥561 колонка ровно 560.
        if (viewport.width >= 561) {
          expect(v.columnWidth, `${path} @${viewport.width}: колонка не 560`).toBe(560);
        }
        expect(
          v.offenders,
          `${path} @${viewport.width}: блоки шире колонки ${v.columnWidth}`,
        ).toEqual([]);
      }
    }
  });
}
