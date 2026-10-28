import { captureScreen, expect, openCabinetWithSeededSession, test } from './fixtures';

// Офлайн-экран по макету «Рентли» Figma 40000051:81613 (редизайн 27.10.2026):
// public/offline.html отдаётся из кэша SW при выключенной сети, поэтому
// страница самодостаточна (лого/иллюстрация инлайном, стили/JS инлайном,
// Onest — прекэш шрифтов в sw.js, guard offline-fonts.test.ts).
// «Поддержка» — прямая ссылка на Telegram из shared/config/support.ts
// (упрощение владельца 21.09: «на тг ссылку просто», без блока копирования).

test('офлайн-страница: заголовок, кнопки, поддержка', async ({ page }, testInfo) => {
  await page.goto('/offline.html');

  await expect(page.getByRole('heading', { name: 'Нет соединения' })).toBeVisible();
  await expect(
    page.getByText('Проверьте подключение к сети. Если интернет работает, попробуйте отключить VPN и повторить попытку'),
  ).toBeVisible();
  await expect(
    page.getByText('Из-за VPN сервис может не загружаться или работать некорректно'),
  ).toBeVisible();

  // «Поддержка» — внешняя ссылка на Telegram; адрес — из
  // shared/config/support.ts.
  const telegram = page.getByRole('link', { name: 'Поддержка' });
  await expect(telegram).toHaveAttribute('href', 'https://t.me/smirnowwwivan');
  await expect(telegram).toHaveAttribute('target', '_blank');

  // «Обновить» перезагружает документ: маркер текущего окна после
  // перезагрузки исчезает (новый документ его не ставил).
  await page.evaluate(() => {
    (window as { offlineE2eMarker?: boolean }).offlineE2eMarker = true;
  });
  await page.getByRole('button', { name: 'Обновить' }).click();
  await expect.poll(() => page.evaluate(() => (window as { offlineE2eMarker?: boolean }).offlineE2eMarker)).toBeFalsy();

  // Onest (решение владельца 27.10: не системный стек): @font-face
  // реально загрузил кириллический сабсет заголовка.
  const onestLoaded = await page.evaluate(() =>
    document.fonts.ready.then(() => document.fonts.check('600 32px Onest', 'Нет соединения')),
  );
  expect(onestLoaded).toBe(true);

  await captureScreen(page, testInfo, 'offline-page');
});

test('офлайн-страница: колонка 560 отцентрована на десктопе', async ({ page }, testInfo) => {
  await page.goto('/offline.html');

  // Ширина вьюпорта — из контекста, не литералом: дефолт живёт в
  // playwright.config.ts и вправе поменяться независимо от этого теста.
  const viewportWidth = page.viewportSize()?.width ?? 0;
  const column = await page.evaluate(() => {
    const rect = (document.querySelector('.page') as HTMLElement).getBoundingClientRect();
    return { left: rect.left, width: rect.width };
  });
  // max-width 560 (--container-column), по центру вьюпорта.
  expect(column.width).toBe(560);
  expect(column.left).toBe((viewportWidth - 560) / 2);

  await captureScreen(page, testInfo, 'offline-page-desktop');
});

test.describe('мобайл 390', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('офлайн-страница на мобайле', async ({ page }, testInfo) => {
    await page.goto('/offline.html');

    await expect(page.getByRole('heading', { name: 'Нет соединения' })).toBeVisible();
    await expect(page.getByRole('link', { name: 'Поддержка' })).toBeVisible();

    // Геометрия макета 40000051:81613 (паддинги 24 — решение владельца
    // 27.10): root-колонка держит трёх детей — лого / иллюстрация+текст /
    // кнопки — и space-between разводит их: лого вверху, кнопки у низа,
    // середина висит в центре освободившегося пространства.
    const layout = await page.evaluate(() => {
      const rect = (selector: string) =>
        (document.querySelector(selector) as HTMLElement).getBoundingClientRect();
      const logo = rect('.logo');
      const content = rect('.content');
      const buttons = rect('.buttons');
      const secondary = rect('.btn-secondary');
      const primary = rect('.btn-primary');
      return {
        logoTop: logo.top,
        logoLeft: logo.left,
        bottomGap: window.innerHeight - buttons.bottom,
        leftEdge: buttons.left,
        gapAboveMiddle: content.top - logo.bottom,
        gapBelowMiddle: buttons.top - content.bottom,
        buttonsOnOneLine: secondary.top === primary.top,
        secondaryLeftOfPrimary: secondary.right < primary.left,
      };
    });
    expect(layout.logoTop).toBe(24);
    expect(layout.logoLeft).toBe(24);
    expect(layout.bottomGap).toBe(24);
    expect(layout.leftEdge).toBe(24);
    // Три ребёнка space-between: свободное место делится пополам над и под
    // средним блоком (в макете 393×852 — зазоры 210/210).
    expect(Math.abs(layout.gapAboveMiddle - layout.gapBelowMiddle)).toBeLessThanOrEqual(1);
    expect(layout.gapBelowMiddle).toBeGreaterThan(0);
    // Кнопки в ряд: «Поддержка» слева, «Обновить» справа (макет
    // 40000053:81673) — не столбиком, как было до редизайна.
    expect(layout.buttonsOnOneLine).toBe(true);
    expect(layout.secondaryLeftOfPrimary).toBe(true);

    await captureScreen(page, testInfo, 'offline-page-mobile');
  });
});

test('SW отдаёт офлайн-страницу при потере сети', async ({ page, context, seededUser }) => {
  test.setTimeout(60_000);
  // SW регистрируется внутри ScreenLayout (кабинетные экраны), поэтому
  // нужен сеанс: /login вне кабинетного layout регистрации не делает.
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties');
  // SW сам делает clients.claim в activate (public/sw.js) — контроллер
  // появляется на уже открытой странице без перезагрузки;
  // ServiceWorkerRegister только register('/sw.js').
  await page.waitForFunction(() => navigator.serviceWorker.controller !== null, undefined, {
    timeout: 30_000,
  });

  await context.setOffline(true);
  await page.goto('/properties');
  await expect(page.getByRole('heading', { name: 'Нет соединения' })).toBeVisible();

  // Шрифты офлайн-страницы достаются из прекэша SW (OFFLINE_FONT_PATHS,
  // cache-first в fetch-хендлере): без него @font-face офлайн умирает
  // (/fonts отдаётся must-revalidate) и страница рисуется системным стеком.
  const onestLoaded = await page.evaluate(() =>
    document.fonts.ready.then(() => document.fonts.check('600 32px Onest', 'Нет соединения')),
  );
  expect(onestLoaded).toBe(true);

  await context.setOffline(false);
});
