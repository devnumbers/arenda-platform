import { expect, openCabinetWithSessionToken, test } from './fixtures';

// Дорисовка ленты месяцев назад (пикер периода, #1428): порция
// дорисовывается задолго до стены скролла, инерция не зажимается о край
// в паузу рендера. Прежний порог 32px срабатывал вплотную к краю: пока
// React рисовал 6 месячных сеток (40–110мс под нагрузкой), флик
// доезжал до scrollTop 0 — прокрутка останавливалась и «дёргалась».
// Замер живой приёмки: триггер брал высоту ленты на scrollTop 0–9px
// из 5 дорисовок.

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
  const prependCount = frames.filter(
    (frame, i) => i > 0 && frames[i - 1] !== undefined && frame.sh !== frames[i - 1]?.sh,
  ).length;
  const minSt = Math.min(...frames.map((f) => f.st));

  // Дорисовки состоялись (лента реально удлинялась).
  expect(prependCount).toBeGreaterThanOrEqual(3);
  // И за весь забег прокрутка ни разу не коснулась стены: триггер
  // дорисовки стоит далеко от края (кадр-зонд между дорисовками читает
  // уже компенсированную позицию — касание стены в паузу рендера и есть
  // то самое дёрганье, теперь оно не возникает вовсе).
  expect(minSt).toBeGreaterThan(0);
});

  // Жёсткий непрерывный флик (карта #1437, план 042, feel-check владельца
  // 16.10: «лагает и годы буквально пролетают»): в отличие от приёмки
  // #1428 выше — без пауз между фликами и вдвое длиннее, скорость под
  // 100px/мс. На прежнем пороге 32px лента жила циклом «стена →
  // компенсация прыжком на порцию» (замер 16.10: пауза на scrollTop 0
  // между каждой дорисовкой) — это и есть «пролетающие годы».
  test('жёсткий непрерывный флик: стена не достигается, лента успевает', async ({ page }) => {
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

  // Разгон вниз (в будущее, до упора демо-ленты): пикер открывается
  // прямо на верхнем крае (initialSt 0, замер 16.10) — флик вверх с
  // потолка не может ничего скроллить. Забег: вниз набираем ход,
  // затем непрерывный флик вверх (в прошлое): 20 wheel-всплесков по
  // 3000px с паузой 25мс — устойчивые ~100px/мс против разовых фликов
  // приёмки #1428. Драйвер — колесо с курсором над лентой (wheel
  // скроллит то, что под указателем): Input.synthesizeScrollGesture под
  // троттлингом скроллит композитором, не диспатчив на главный поток ни
  // одного scroll-события (замер 16.10: evs=0 при прокрученной ленте).
  await page.mouse.move(box.x + box.width / 2, box.y + box.height * 0.5);
  for (let f = 0; f < 10; f += 1) {
    await page.mouse.wheel(0, 3000);
    await page.waitForTimeout(25);
  }
  for (let f = 0; f < 20; f += 1) {
    await page.mouse.wheel(0, -3000);
    await page.waitForTimeout(25);
  }
  await page.waitForTimeout(2000);
  await cdp.send('Emulation.setCPUThrottlingRate', { rate: 1 });

  const frames = await page.evaluate(
    () =>
      (window as unknown as { __hardFrames: Array<{ st: number; sh: number }>; __evs: number[] })
        .__hardFrames,
  );
  const evs = await page.evaluate(() => (window as unknown as { __evs: number[] }).__evs);
  const prependCount = frames.filter(
    (frame, i) => i > 0 && frame.sh !== frames[i - 1]?.sh,
  ).length;
  const lastFrame = frames.at(-1);
  const firstFrame = frames[0];
  if (lastFrame === undefined || firstFrame === undefined) {
    throw new Error('зонд кадров пуст — rAF не тикал');
  }
  const shGrowth = lastFrame.sh - firstFrame.sh;
  const wallFrames = frames.filter((f) => f.st === 0).length;
  // Стена считается после первого движения: пикер открывается прямо на
  // верхнем крае (initialSt 0) — ведущие нули это стартовая позиция, а
  // не касание стены в полёте.
  const firstMoved = frames.findIndex((f) => f.st > 0);
  const wallFramesInFlight = (firstMoved === -1 ? [] : frames.slice(firstMoved)).filter((f) => f.st === 0).length;
  const diag = `frames=${frames.length} last=${JSON.stringify(frames.at(-1))} evs=${evs.length} prependCount=${prependCount} shGrowth=${shGrowth} wallFrames=${wallFrames}`;

  // Лента догнала флик: дорисовки шли, суммарный прирост высоты покрывает
  // забег (60 000px вверх минус стартовый буфер ~2800px — скоростные
  // порции по несколько порций на событие).
  expect(prependCount, diag).toBeGreaterThanOrEqual(3);
  expect(shGrowth, diag).toBeGreaterThanOrEqual(12000);
  // Стена — не более нескольких кадров в полёте: под душащим CPU событие
  // может доехать, когда композитор уже у края, и скоростная порция
  // снимается с них в тот же коммит; устойчивый цикл «стена → прыжок»
  // (24 кадра на стене на прежней механике) — регресс.
  expect(wallFramesInFlight, `кадров на стене (scrollTop 0) в полёте; ${diag}`).toBeLessThanOrEqual(4);
});
