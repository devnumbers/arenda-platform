import { expect, openCabinetWithSessionToken, test } from './fixtures';

// Дорисовка ленты месяцев назад (пикер периода, #1428): порция
// дорисовывается задолго до стены скролла, инерция не зажимается о край
// в паузу рендера. Прежний порог 32px срабатывал вплотную к краю: пока
// React рисовал 6 месячных сеток (40–110мс под нагрузкой), флик
// доезжал до scrollTop 0 — прокрутка останавливалась и «дёргалась».
// План 051: коммит приписки ограничен (гейт + кэп 2 = ≤ 12 месяцев),
// антистенную защиту несёт запас seed 11 (открытие ≈ 4000px выше
// порога 3000) — метрики инвариантов пересчитаны под ограниченный
// коммит: объём доращивки вместо количества приписок. Жёсткость
// контента при приписке — calendar-anchor.spec.ts.

const UI_KIT = '/ui-kit';

test.beforeEach(async ({ page, seededUser }) => {
  await openCabinetWithSessionToken(page, seededUser.sessionToken);
});

test('дорисовка месяцев срабатывает с запасом от стены, флик не зажимается', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(UI_KIT);
  await page.getByRole('button', { name: 'Открыть пикер периода' }).click();
  const dialog = page.getByRole('dialog', { name: 'Выберите период' });
  await expect(dialog).toBeVisible();
  await page.waitForTimeout(800);

  const cdp = await page.context().newCDPSession(page);
  // Троттлинг CPU растягивает рендер порции — пауза дорисовки видна
  // детерминированно; флики крутит композитор и не троттлит.
  await cdp.send('Emulation.setCPUThrottlingRate', { rate: 8 });

  const cont = dialog.locator('.overflow-y-auto');
  const box = await cont.boundingBox();
  if (box === null) {
    throw new Error('лента пикера периода не найдена');
  }
  const x = box.x + box.width / 2;
  const y1 = box.y + box.height * 0.3;
  const y2 = box.y + box.height * 0.95;

  // Зонд: кадры прокрутки ленты
  await page.evaluate(() => {
    const c = document.querySelector('[role="dialog"] .overflow-y-auto') as HTMLElement;
    (window as unknown as { __stFrames: Array<{ st: number; sh: number }> }).__stFrames = [];
    const probe = window as unknown as {
      __stFrames: Array<{ st: number; sh: number }>;
    };
    const tick = () => {
      probe.__stFrames.push({ st: Math.round(c.scrollTop), sh: c.scrollHeight });
      if (probe.__stFrames.length < 2000) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });

  // Десять жёстких фликов вниз экрана = прокрутка в прошлое, непрерывно
  for (let f = 0; f < 10; f += 1) {
    await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x, y: y1 }] });
    for (let s = 1; s <= 6; s += 1) {
      await cdp.send('Input.dispatchTouchEvent', {
        type: 'touchMove',
        touchPoints: [{ x, y: y1 + (y2 - y1) * (s / 6) }],
      });
    }
    await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
    await page.waitForTimeout(60);
  }
  await page.waitForTimeout(2000);
  await cdp.send('Emulation.setCPUThrottlingRate', { rate: 1 });

  const frames = await page.evaluate(
    () =>
      (window as unknown as { __stFrames: Array<{ st: number; sh: number }> }).__stFrames,
  );
  const firstFrame = frames[0];
  const lastFrame = frames.at(-1);
  if (firstFrame === undefined || lastFrame === undefined) {
    throw new Error('зонд кадров пуст — rAF не тикал');
  }
  const shGrowth = lastFrame.sh - firstFrame.sh;
  const minSt = Math.min(...frames.map((f) => f.st));

  // Дорисовки состоялись: забег (~5500px) съел стартовый запас (~4000px)
  // и потребовал минимум одну ограниченную порцию (~4300px).
  expect(shGrowth, 'лента доросла под забег').toBeGreaterThanOrEqual(1000);
  // И за весь забег прокрутка ни разу не коснулась стены: триггер
  // дорисовки стоит далеко от края (кадр-зонд между дорисовками читает
  // уже компенсированную позицию — касание стены в паузу рендера и есть
  // то самое дёрганье, теперь оно не возникает вовсе).
  expect(minSt).toBeGreaterThan(0);
});

  // Жёсткий флик реальной физики моментума (карта #1437; план 051:
  // протокол переведён с синтетических 100px/ms, которые мог догнать
  // только монструозный коммит — источник телефонных пауз, — на верхнюю
  // границу реального флика: ~4800px суммарного пробега при душащем CPU).
  // Инвариант тот же: стена не достигается, лента успевает.
  test('жёсткий флик под душащим CPU: стена не достигается, лента успевает', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(UI_KIT);
    // Под нагрузкой полного прогона клик до гидрации теряется (кнопка
    // уже в DOM, обработчик ещё нет) — даём гидратации устояться.
    await expect(page.getByRole('button', { name: 'Открыть пикер периода' })).toBeVisible();
    await page.waitForTimeout(500);
    await page.getByRole('button', { name: 'Открыть пикер периода' }).click();
  const dialog = page.getByRole('dialog', { name: 'Выберите период' });
  await expect(dialog).toBeVisible();
  await page.waitForTimeout(800);

  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Emulation.setCPUThrottlingRate', { rate: 8 });

  const cont = dialog.locator('.overflow-y-auto');
  const box = await cont.boundingBox();
  if (box === null) {
    throw new Error('лента пикера периода не найдена');
  }
  await page.evaluate(() => {
    const c = document.querySelector('[role="dialog"] .overflow-y-auto') as HTMLElement;
    (window as unknown as { __hardFrames: Array<{ st: number; sh: number }>; __evs: number[] }).__hardFrames = [];
    (window as unknown as { __evs: number[] }).__evs = [];
    const probe = window as unknown as { __hardFrames: Array<{ st: number; sh: number }>; __evs: number[] };
    c.addEventListener('scroll', () => probe.__evs.push(Math.round(c.scrollTop)));
    const tick = () => {
      probe.__hardFrames.push({ st: Math.round(c.scrollTop), sh: c.scrollHeight });
      if (probe.__hardFrames.length < 3000) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });

  // Восемь всплесков по 600px с паузой 30мс — суммарный пробег ~4800px,
  // верх реального жёсткого флика (~3–10px/ms с затуханием); драйвер —
  // колесо с курсором над лентой (Input.synthesizeScrollGesture под
  // троттлингом скроллит композитором, не диспатчив ни одного
  // scroll-события — замер 16.10).
  await page.mouse.move(box.x + box.width / 2, box.y + box.height * 0.5);
  for (let f = 0; f < 8; f += 1) {
    await page.mouse.wheel(0, -600);
    await page.waitForTimeout(30);
  }
  await page.waitForTimeout(2000);
  await cdp.send('Emulation.setCPUThrottlingRate', { rate: 1 });

  const frames = await page.evaluate(
    () =>
      (window as unknown as { __hardFrames: Array<{ st: number; sh: number }>; __evs: number[] })
        .__hardFrames,
  );
  const lastFrame = frames.at(-1);
  const firstFrame = frames[0];
  if (lastFrame === undefined || firstFrame === undefined) {
    throw new Error('зонд кадров пуст — rAF не тикал');
  }
  const shGrowth = lastFrame.sh - firstFrame.sh;
  const wallFrames = frames.filter((f) => f.st === 0).length;
  // Стена считается после первого движения: открытие стоит на стартовом
  // запасе (~4000px, seed 11) — ведущие нули это стартовая позиция, а
  // не касание стены в полёте.
  const firstMoved = frames.findIndex((f) => f.st > 0);
  const wallFramesInFlight = (firstMoved === -1 ? [] : frames.slice(firstMoved)).filter((f) => f.st === 0).length;
  const diag = `frames=${frames.length} last=${JSON.stringify(frames.at(-1))} prependCount=${frames.filter((frame, i) => i > 0 && frame.sh !== frames[i - 1]?.sh).length} shGrowth=${shGrowth} wallFrames=${wallFrames}`;

  // Забег (~4800px) съел запас (~4000px) и потребовал дорисовку.
  expect(shGrowth, diag).toBeGreaterThanOrEqual(2000);
  // Стена — не более нескольких кадров в полёте: под душащим CPU событие
  // может доехать, когда композитор уже у края, и порция
  // снимается с них в тот же коммит; устойчивый цикл «стена → прыжок» —
  // регресс.
  expect(wallFramesInFlight, `кадров на стене (scrollTop 0) в полёте; ${diag}`).toBeLessThanOrEqual(4);
});

