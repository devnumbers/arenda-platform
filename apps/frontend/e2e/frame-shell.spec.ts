import type { Page } from '@playwright/test';
import {
  openCabinetWithSeededSession,
  expect,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  test,
} from './fixtures';

// Каркас-шелл страницы (карта #862, тикет #865; канон восстановлен планом
// 047 после регресса тотального сноса 046): колонка контента центрируется
// во всём вьюпорте на любом ярусе; полноэкранные поверхности начинаются на
// границе сайдбара (меню видно и кликабельно, 25.09/#865), их колонки
// легают с колонками страниц; открытая МОДАЛКА опускает хром под скрим
// (точечное правило плана 047 — лечение бага 16.10: сайдбар пробивал
// скрим колесного шита над поверхностью); нижняя панель на ПК — колонка
// 560 (углы свободны, пилюли не глушатся никогда — «хром ПК постоянен»,
// решение 25.09, правка #561); планшетные шапки подэкранов — слоты у
// краёв вьюпорта; TabBar глушится нижними панелями на мобайле и планшете;
// мобильное меню «Ещё» — без изменений (указание владельца 16.10);
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
    // на ПК он начинается на границе сайдбара (решение 25.09/#865,
    // возвращено планом 047 после регресса тотального сноса 046).
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
  // 25.09, доработка #865, возвращено планом 047): шапку с крыльями
  // рисует сама поверхность, пилюли остаются смонтированными, шит панели
  // пикера — колонка 560 (углы свободны), клик над сайдбаром ловит
  // пункт меню.
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
  // Правое крыло — ссылка профиля в шапке поверхности, у правого края
  // вьюпорта (паддинг даёт сама ссылка; решение владельца 02.10 —
  // UserButton настоящий Link, имя на всех ярусах).
  const profile = surface.locator('header').getByRole('link', { name: /Иван/ });
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

test('ПК: модалка над поверхностью опускает хром под скрим (план 047)', async ({
  page,
  seededUser,
}) => {
  // Баг владельца 16.10 (скриншот пикера даты на /operations/new): колесный
  // шит месяца/года открыт над полноэкранной поверхностью, а сайдбар и
  // пилюли пробивали скрим — хром, поднятый над поверхностью до z-60
  // (#865), бил оверлей модалки (z-50). Точечное лечение плана 047:
  // пока открыт оверлей модалки, хром спускается к z-30 — точка над
  // сайдбаром ловит скрим, не пункт меню. Поверхность без модалки меню
  // не прячет (тест выше).
  await openCabinetWithSeededSession(page, seededUser);
  await page.setViewportSize(PC);
  await page.goto('/operations/new');
  // Визард открывается шагом суммы; дата — второй шаг («Выберите дату»).
  await page.getByRole('textbox', { name: 'Сумма' }).fill('1000');
  await page.getByRole('button', { name: 'Продолжить' }).click();
  const monthChip = page.getByRole('button', {
    name: /^(Январь|Февраль|Март|Апрель|Май|Июнь|Июль|Август|Сентябрь|Октябрь|Ноябрь|Декабрь) \d{4}$/,
  });
  await expect(monthChip).toBeVisible();

  await monthChip.click();
  const wheelSheet = page.getByRole('dialog', { name: 'Месяц и год' });
  await expect(wheelSheet).toBeVisible();

  const hit = await page.evaluate(() => {
    const target = document.elementFromPoint(110, 300);
    return {
      inSidebar:
        target instanceof HTMLElement
          ? target.closest('nav[aria-label="Основная навигация"]') !== null
          : false,
      inOverlay:
        target instanceof HTMLElement ? target.closest('.modal-overlay') !== null : false,
    };
  });
  expect(hit.inSidebar, 'сайдбар пробивает скрим модалки').toBe(false);
  expect(hit.inOverlay, 'точка над сайдбаром ловит оверлей модалки').toBe(true);
});

test('ПК: нижняя панель страницы — колонка 560, пилюли не глушатся никогда', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.setViewportSize(PC);

  // Страница с нижней кнопкой действия: панель — колонка 560 по центру,
  // углы свободны, пилюли на месте («хром ПК постоянен», решение 25.09 —
  // правка канона #561, тикет #865).
  await page.goto(`/properties/${SEEDED_GARAGE_PROPERTY_ID}/edit`);
  await expect(bottomBarButton(page, /Сохранить/)).toBeVisible();
  await expect(page.locator('nav[aria-label="Дополнительная навигация"] .fixed')).toHaveCount(2);

  // Шит панели — колонка 560, по центру вьюпорта (как контент страницы).
  const bar = await page.evaluate(() => {
    const sheet = [...document.querySelectorAll('.fixed.inset-x-0.bottom-0')].find(
      (el) => getComputedStyle(el).display !== 'none',
    );
    if (!sheet) throw new Error('нижняя панель не найдена');
    const column = sheet.firstElementChild;
    if (!column) throw new Error('колонка панели не найдена');
    const c = column.getBoundingClientRect();
    const s = sheet.getBoundingClientRect();
    return { columnWidth: c.width, columnLeft: c.left, sheetLeft: s.left, sheetWidth: s.width };
  });
  expect(bar.columnWidth).toBe(560);
  expect(bar.sheetWidth).toBe(560);
  expect(Math.abs(bar.columnLeft - (PC.width - 560) / 2)).toBeLessThanOrEqual(2);
  expect(Math.abs(bar.sheetLeft - (PC.width - 560) / 2)).toBeLessThanOrEqual(2);

  // Хаб без нижней панели: пилюли тоже на месте.
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
    ['/participants', 'Участники'],
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

  // Канон планшета (DESIGN.md, правило 561–1023): шит панели не сужается —
  // во всю ширину вьюпорта, кап колонки 560 включается только на ПК.
  const barSheetWidth = await page.evaluate(() => {
    const sheet = [...document.querySelectorAll('.fixed.inset-x-0.bottom-0')].find(
      (el) => getComputedStyle(el).display !== 'none',
    );
    if (!sheet) throw new Error('нижняя панель не найдена');
    return sheet.getBoundingClientRect().width;
  });
  expect(barSheetWidth).toBe(TABLET.width);

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

// Плавающая кнопка ленты над TabBar (решение владельца 06.10.2026, #1166):
// футер истории НЕ глушится, «Настройки» плавает над ним; внутри канона
// (операции, редактирование объекта) подавление работает как раньше —
// проверено тестами выше и ниже. Панель CTA раздела «Участники», чей лег
// проверяли здесь раньше, снесена с хабом (D1 #1313).
test('кнопка «Настройки» истории — над видимым TabBar (мобайл и планшет)', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);

  for (const viewport of [MOBILE, TABLET]) {
    await page.setViewportSize(viewport);

    await page.goto('/history');
    const tabbar = page.locator('nav[aria-label="Нижняя навигация"]');
    await expect(tabbar).toBeVisible();
    const settings = page.getByRole('button', { name: 'Настройки' });
    await expect(settings).toBeVisible();
    const settingsBox = await settings.boundingBox();
    const tabBox = await tabbar.boundingBox();
    expect((settingsBox?.y ?? 0) + (settingsBox?.height ?? 0)).toBeLessThanOrEqual((tabBox?.y ?? 0) + 2);
  }
});

// Полноэкранный пикер дня оплаты аренды (#530/#532): на планшете и ПК TopNav
// внутри поверхности фиксируется (tablet:fixed) и выпадает из её потока —
// без компенсации tablet:mt-[72px] (канон CalendarDatePicker) контент
// поверхности уезжает под бар. Мобайл держит хедер в потоке — зазор там
// структурный.
test('пикер дня оплаты: календарь не заезжает под закреплённый хедер (мобайл, планшет, ПК)', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);

  for (const viewport of [MOBILE, TABLET, PC]) {
    await page.setViewportSize(viewport);
    await page.goto(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}/rentals/new`);
    await expect(page.getByRole('heading', { name: 'Цена и число оплаты' })).toBeVisible();
    await page.getByRole('button', { name: 'День оплаты: Выбрать день' }).click();
    const picker = page.getByRole('dialog', { name: 'Выбор дня оплаты' });
    await expect(picker).toBeVisible();

    const gap = await picker.evaluate((dialog) => {
      const header = dialog.querySelector('header');
      const grid = dialog.querySelector('.grid.grid-cols-7');
      if (!header || !grid) throw new Error('хедер или грид дней пикера не найден');
      return grid.getBoundingClientRect().top - header.getBoundingClientRect().bottom;
    });
    expect(gap).toBeGreaterThanOrEqual(-1);
  }
});

test('легаси /dashboard редиректит на /properties — слово владельца #975', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  const response = await page.goto('/dashboard');
  // 308 перманентный на дом кабинета: старые входы (закладка, иконка PWA
  // со старым start_url) заканчиваются на живом экране, не на 404.
  expect(page.url()).toContain('/properties');
  expect(response?.status()).toBe(200);
});
