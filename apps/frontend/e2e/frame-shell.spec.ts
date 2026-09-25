import type { Page } from '@playwright/test';
import { openCabinetWithSeededSession, expect, SEEDED_GARAGE_PROPERTY_ID, test } from './fixtures';

// Каркас-шелл страницы (карта #862, тикет #865 + доработка 25.09 по
// макетам 1185-40815/40811/40814 + хром-фикс 25.09): колонка контента
// центрируется во всём вьюпорте на любом ярусе; полноэкранные поверхности
// пикеров начинаются на границе сайдбара (меню видно), их колонки легают
// с колонками страниц; хром ПК не исчезает под открытой поверхностью —
// крылья рисует поверхность, пилюли остаются, сайдбар кликабелен; шит
// панели поверхности на ПК — колонка 560; планшетные шапки подэкранов —
// слоты у краёв вьюпорта; глушение нижнего хрома на всех брейкпоинтах
// (панели страниц — TabBar+пилюли, панели поверхностей — только TabBar);
// ноль дёрганья шапок хабов. Геометрия меряется bounding-box'ами —
// вердикт живьём остаётся за /ui-walkthrough, спека держит регресс-гейт
// на трёх ярусах (мобайл 375 / планшет 768 / ПК 1440).

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

test('ПК: колонка контента центрируется во всём вьюпорте', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.setViewportSize(PC);
  await page.goto('/operations');
  await expect(page.getByRole('heading', { level: 1, name: 'Операции' })).toBeVisible();

  const column = await contentColumnRect(page);
  expect(column.width).toBe(560);
  // Колонка — ровно по центру вьюпорта (макет 1185-40815: слот x=488 при
  // фрейме 1536), зазоры к краям совпадают в пределах допуска на субпиксели.
  const leftGap = column.left;
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

test('ПК: пикер периода не накрывает сайдбар, колонка пикера легает с колонкой страницы', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.setViewportSize(PC);
  await page.goto('/operations');
  await expect(page.getByRole('heading', { level: 1, name: 'Операции' })).toBeVisible();
  const pageColumn = await contentColumnRect(page);

  await page.getByRole('button', { name: /Период/ }).click();
  const surface = page.locator('[role="dialog"][aria-label="Выберите период"]');
  await expect(surface).toBeVisible();

  const geometry = await page.evaluate(() => {
    const dialog = document.querySelector('[role="dialog"][aria-label="Выберите период"]');
    const sidebar = document.querySelector('nav[aria-label="Основная навигация"]');
    if (!dialog || !sidebar) throw new Error('поверхность или сайдбар не найдены');
    const column = dialog.querySelector('.max-w-column');
    const c = column?.getBoundingClientRect();
    // Белый лист поверхности — псевдоэлемент .fullscreen-surface::before:
    // на ПК он начинается на границе сайдбара, меню остаётся видимым.
    const sheetLeft = getComputedStyle(dialog, '::before').left;
    return {
      sheetLeft,
      pickerColumnLeft: c ? c.left : null,
    };
  });
  // Лист — правее сайдбара; колонка пикера — по центру вьюпорта, на оси
  // колонки страницы (открытие пикера не сдвигает контент).
  expect(geometry.sheetLeft).toBe(`${SIDEBAR_TOTAL}px`);
  expect(geometry.pickerColumnLeft).not.toBeNull();
  expect(Math.abs((geometry.pickerColumnLeft ?? 0) - pageColumn.left)).toBeLessThanOrEqual(2);

  // Хром ПК не исчезает под открытой поверхностью (решение владельца
  // 25.09, доработка #865): шапку с крыльями рисует сама поверхность,
  // пилюли остаются смонтированными, шит панели пикера — колонка 560
  // (углы свободны), клик над сайдбаром ловит пункт меню.
  const chrome = await page.evaluate(() => {
    const dialog = document.querySelector('[role="dialog"][aria-label="Выберите период"]');
    const sheet = dialog?.querySelector('.fixed.inset-x-0.bottom-0');
    const s = sheet?.getBoundingClientRect();
    const logo = dialog?.querySelector('a[aria-label="Объекты"]');
    const l = logo?.getBoundingClientRect();
    const pills = document.querySelectorAll('nav[aria-label="Дополнительная навигация"] .fixed');
    const hit = document.elementFromPoint(110, 300);
    return {
      logoX: l?.x ?? null,
      pills: pills.length,
      sheetWidth: s?.width ?? null,
      sheetLeft: s?.left ?? null,
      hitInSidebar: hit instanceof HTMLElement ? hit.closest('nav[aria-label="Основная навигация"]') !== null : false,
    };
  });
  expect(chrome.logoX).toBe(0);
  expect(chrome.pills).toBe(2);
  // Правое крыло — кнопка профиля в шапке поверхности, у правого края
  // вьюпорта (паддинг даёт сама кнопка).
  const profile = surface.locator('header').getByRole('button', { name: /Иван/ });
  await expect(profile).toBeVisible();
  const profileBox = await profile.boundingBox();
  expect(profileBox).not.toBeNull();
  // Если бокса нет — зазор вырождается во всю ширину и падает по допуску.
  expect(PC.width - ((profileBox?.x ?? 0) + (profileBox?.width ?? 0))).toBeLessThanOrEqual(2);
  expect(chrome.sheetWidth).toBe(560);
  expect(Math.abs((chrome.sheetLeft ?? 0) - (PC.width - 560) / 2)).toBeLessThanOrEqual(2);
  expect(chrome.hitInSidebar).toBe(true);

  // Меню кликабельно при открытом пикере (§1): клик уводит из пикера.
  await page.getByRole('link', { name: 'Платежи' }).click();
  await expect(surface).not.toBeVisible();
  await expect(page).toHaveURL(/\/payments$/);
});

test('ПК: пилюли глушатся, пока смонтирована нижняя панель, и возвращаются после', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.setViewportSize(PC);

  // Страница с нижней кнопкой действия: пилюль нет.
  await page.goto(`/properties/${SEEDED_GARAGE_PROPERTY_ID}/edit`);
  await expect(bottomBarButton(page, /Сохранить/)).toBeVisible();
  await expect(page.locator('nav[aria-label="Дополнительная навигация"] .fixed')).toHaveCount(0);

  // Кап колонки нижней панели — 560 (max-w-column), шит во всю ширину
  // вьюпорта, колонка — по центру вьюпорта (как контент страницы).
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
  expect(bar.sheetLeft).toBe(0);
  expect(Math.abs(bar.columnLeft - (PC.width - 560) / 2)).toBeLessThanOrEqual(2);

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

test('планшет: слоты шапки подэкрана у краёв вьюпорта, TabBar глушится панелью', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.setViewportSize(TABLET);

  await page.goto('/operations');
  await expect(page.getByRole('heading', { level: 1, name: 'Операции' })).toBeVisible();
  const hubColumn = await contentColumnRect(page);
  // Планшет живёт без сайдбара: колонка центрирована во всём вьюпорте.
  expect(Math.abs(hubColumn.left - (TABLET.width - 560) / 2)).toBeLessThanOrEqual(2);

  // Шапка подэкрана во всю ширину (макет 1185-40814): ведущая кнопка —
  // у левого края вьюпорта (pl-3.5 ≈ 14px), не у края колонки.
  await page.goto(`/properties/${SEEDED_GARAGE_PROPERTY_ID}/edit`);
  const leading = page.locator('header[aria-label="Навигация экрана"]').getByRole('button', { name: 'Закрыть' });
  await expect(leading).toBeVisible();
  const leadingBox = await leading.boundingBox();
  expect(leadingBox).not.toBeNull();
  expect(leadingBox?.x ?? 999).toBeLessThan(30);
  await expect(bottomBarButton(page, /Сохранить/)).toBeVisible();
  await expect(page.locator('nav[aria-label="Нижняя навигация"]')).toHaveCount(0);

  await page.goto('/operations');
  await expect(page.locator('nav[aria-label="Нижняя навигация"]')).toBeVisible();

  // Пикер на планшете: TabBar под поверхностью заглушён, шит панели — во
  // всю ширину (сужение surface-шита до колонки — только ПК).
  await page.getByRole('button', { name: /Период/ }).click();
  const picker = page.locator('[role="dialog"][aria-label="Выберите период"]');
  await expect(picker).toBeVisible();
  await expect(page.locator('nav[aria-label="Нижняя навигация"]')).toHaveCount(0);
  const pickerSheetWidth = await picker
    .locator('.fixed.inset-x-0.bottom-0')
    .evaluate((el) => el.getBoundingClientRect().width);
  expect(pickerSheetWidth).toBe(TABLET.width);
  // Крыльев у поверхности на планшете нет (анатомия подэкрана, §2) —
  // лого рисует только ПК.
  await expect(picker.locator('a[aria-label="Объекты"]')).toBeHidden();
  await page.getByRole('button', { name: 'Назад' }).click();
  await expect(picker).not.toBeVisible();
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
