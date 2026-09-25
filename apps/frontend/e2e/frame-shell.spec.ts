import type { Page } from '@playwright/test';
import { openCabinetWithSeededSession, expect, SEEDED_GARAGE_PROPERTY_ID, test } from './fixtures';

// Каркас-шелл страницы (карта #862, тикет #865): ПК-колонка правее
// сайдбара, полноэкранные поверхности не накрывают сайдбар, глушение
// нижнего хрома на всех брейкпоинтах, кап нижней кнопки 560/24, ноль
// дёрганья шапок хабов. Геометрия меряется bounding-box'ами — вердикт
// живьём остаётся за /ui-walkthrough (три брейкпоинта), спека держит
// регресс-гейт на трёх ярусах (мобайл 375 / планшет 768 / ПК 1440).

const PC = { width: 1440, height: 900 };
const TABLET = { width: 768, height: 1024 };
const MOBILE = { width: 375, height: 812 };

/** Полная ширина ПК-сайдбара (токен --sidebar-total, globals.css). */
const SIDEBAR_TOTAL = 224;

async function contentColumnRect(page: Page): Promise<{ left: number; right: number; width: number }> {
  return page.evaluate(() => {
    const column = document.querySelector('.max-w-column');
    if (!column) throw new Error('max-w-column не найден на странице');
    const r = column.getBoundingClientRect();
    return { left: r.left, right: r.right, width: r.width };
  });
}

test('ПК: колонка контента центрируется правее сайдбара', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.setViewportSize(PC);
  await page.goto('/operations');
  await expect(page.getByRole('heading', { level: 1, name: 'Операции' })).toBeVisible();

  const column = await contentColumnRect(page);
  expect(column.width).toBe(560);
  // Колонка целиком правее сайдбара и центрирована в оставшемся пространстве
  // (зазоры слева/справа от неё совпадают в пределах допуска на субпиксели).
  expect(column.left).toBeGreaterThanOrEqual(SIDEBAR_TOTAL);
  const leftGap = column.left - SIDEBAR_TOTAL;
  const rightGap = PC.width - column.right;
  expect(Math.abs(leftGap - rightGap)).toBeLessThanOrEqual(2);

  // Центральная колонка закреплённого хедера — на той же оси, что контент.
  const headerColumn = await page.evaluate(() => {
    const header = document.querySelector('header[aria-label="Навигация экрана"]');
    const inner = header?.querySelector('[class*="max-w-column"]');
    if (!inner) throw new Error('центральная колонка хедера не найдена');
    const r = inner.getBoundingClientRect();
    return { left: r.left, right: r.right };
  });
  expect(Math.abs(headerColumn.left - column.left)).toBeLessThanOrEqual(2);
});

/** Кнопка действия нижней панели: сам бар ищется классом канвы, кнопка —
 * внутри него (на странице бывают другие «Сохранить…» — строгий режим
 * Playwright требует скоуп). */
function bottomBarButton(page: Page, name: RegExp): ReturnType<Page['getByRole']> {
  return page
    .locator('.fixed.inset-x-0.bottom-0')
    .getByRole('button', { name });
}

test('ПК: пикер периода не накрывает сайдбар', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.setViewportSize(PC);
  await page.goto('/operations');
  await expect(page.getByRole('heading', { level: 1, name: 'Операции' })).toBeVisible();

  await page.getByRole('button', { name: /Период/ }).click();
  const surface = page.locator('[role="dialog"][aria-label="Выберите период"]');
  await expect(surface).toBeVisible();

  const geometry = await page.evaluate(() => {
    const dialog = document.querySelector('[role="dialog"][aria-label="Выберите период"]');
    const sidebar = document.querySelector('nav[aria-label="Основная навигация"]');
    if (!dialog || !sidebar) throw new Error('поверхность или сайдбар не найдены');
    const d = dialog.getBoundingClientRect();
    const probe = document.elementFromPoint(100, 200);
    return {
      surfaceLeft: d.left,
      sidebarUnderSurface: probe ? sidebar.contains(probe) : false,
    };
  });
  // Поверхность начинается на границе сайдбара — сайдбар видим и кликабелен.
  expect(geometry.surfaceLeft).toBe(SIDEBAR_TOTAL);
  expect(geometry.sidebarUnderSurface).toBe(true);

  await page.getByRole('button', { name: 'Назад' }).click();
  await expect(surface).not.toBeVisible();
});

test('ПК: пилюли глушатся, пока смонтирована нижняя панель, и возвращаются после', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.setViewportSize(PC);

  // Страница с нижней кнопкой действия: пилюль нет.
  await page.goto(`/properties/${SEEDED_GARAGE_PROPERTY_ID}/edit`);
  await expect(bottomBarButton(page, /Сохранить/)).toBeVisible();
  await expect(page.locator('nav[aria-label="Дополнительная навигация"] .fixed')).toHaveCount(0);

  // Кап колонки нижней панели — 560 (max-w-column), кнопка 512 (24 по бокам);
  // сам шит начинается на границе сайдбара, колонка центрирована правее него.
  const bar = await page.evaluate(() => {
    const sheet = [...document.querySelectorAll('.fixed.inset-x-0.bottom-0')].find(
      (el) => getComputedStyle(el).display !== 'none',
    );
    if (!sheet) throw new Error('нижняя панель не найдена');
    const column = sheet.firstElementChild;
    if (!column) throw new Error('колонка панели не найдена');
    const c = column.getBoundingClientRect();
    const s = sheet.getBoundingClientRect();
    return { columnWidth: c.width, columnLeft: c.left, sheetLeft: s.left };
  });
  expect(bar.columnWidth).toBe(560);
  expect(bar.sheetLeft).toBe(SIDEBAR_TOTAL);
  // Колонка центрирована в пространстве правее сайдбара: зазоры слева и
  // справа от неё (внутри этого пространства) совпадают.
  const leftGap = bar.columnLeft - SIDEBAR_TOTAL;
  const rightGap = PC.width - (bar.columnLeft + bar.columnWidth);
  expect(Math.abs(leftGap - rightGap)).toBeLessThanOrEqual(2);

  // Хаб без нижней панели: пилюли на месте.
  await page.goto('/operations');
  await expect(page.getByRole('heading', { level: 1, name: 'Операции' })).toBeVisible();
  await expect(page.locator('nav[aria-label="Дополнительная навигация"] .fixed')).toHaveCount(2);
});

test('ПК: топ заголовка хаба — 24 от хедера на всех хабах (ноль дёрганья)', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.setViewportSize(PC);

  for (const [path, title] of [
    ['/operations', 'Операции'],
    ['/tasks', 'Задачи'],
    ['/contacts', 'Контакты'],
    ['/participants', 'Совместный доступ'],
  ] as const) {
    await page.goto(path);
    const heading = page.getByRole('heading', { level: 1, name: title });
    await expect(heading).toBeVisible();
    // Первый кадр после goto может застать пре-CSS раскладку — меряем
    // устойчивый топ, а не мгновение.
    await expect
      .poll(
        async () => heading.evaluate((el) => el.getBoundingClientRect().top),
        { timeout: 3_000 },
      )
      .toBe(96);
  }
});

test('планшет: TabBar глушится нижней панелью и возвращается; колонка — по центру вьюпорта', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.setViewportSize(TABLET);

  await page.goto('/operations');
  await expect(page.getByRole('heading', { level: 1, name: 'Операции' })).toBeVisible();
  const hubColumn = await contentColumnRect(page);
  // Планшет живёт без сайдбара: колонка центрирована во всём вьюпорте.
  expect(Math.abs(hubColumn.left - (TABLET.width - 560) / 2)).toBeLessThanOrEqual(2);

  await page.goto(`/properties/${SEEDED_GARAGE_PROPERTY_ID}/edit`);
  await expect(bottomBarButton(page, /Сохранить/)).toBeVisible();
  await expect(page.locator('nav[aria-label="Нижняя навигация"]')).toHaveCount(0);

  await page.goto('/operations');
  await expect(page.locator('nav[aria-label="Нижняя навигация"]')).toBeVisible();
});

test('мобайл: хедер в потоке, TabBar на хабе и глушится нижней панелью', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.setViewportSize(MOBILE);

  await page.goto('/operations');
  await expect(page.getByRole('heading', { level: 1, name: 'Операции' })).toBeVisible();
  const headerPosition = await page.evaluate(() => {
    const header = document.querySelector('header[aria-label="Навигация экрана"]');
    return header ? getComputedStyle(header).position : null;
  });
  // В потоке (fixed включается только с tablet:).
  expect(headerPosition).toBe('relative');
  await expect(page.locator('nav[aria-label="Нижняя навигация"]')).toBeVisible();

  await page.goto(`/properties/${SEEDED_GARAGE_PROPERTY_ID}/edit`);
  await expect(bottomBarButton(page, /Сохранить/)).toBeVisible();
  await expect(page.locator('nav[aria-label="Нижняя навигация"]')).toHaveCount(0);
});

test('легаси /dashboard снесён — ответ 404, а не редирект', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  const response = await page.goto('/dashboard');
  expect(response?.status()).toBe(404);
});
