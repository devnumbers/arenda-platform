import { expect, openCabinetWithSessionToken, test } from './fixtures';

// Дорисовка ленты месяцев назад (пикер периода, #1428 → план 053):
// история растёт append'ом вниз, порция добавляется ниже вьюпорта
// задолго до нижней стены. Инерция не зажимается о край: записей в
// скролл не существует (главный инвариант — calendar-anchor.spec.ts),
// здесь проверяется антистенная механика под душащим CPU — жёсткость
// контента и протоколы фликов прежние, направление протокола перевёрнуто
// (прошлое теперь вниз).

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
  // Траектория внутри ленты: низ ленты визуально уходит под фиксированную
  // нижнюю панель — тач на 95% высоты садится на её кнопки и не скроллит
  // (старый протокол начинал с безопасных 30%, зеркалим с запасом).
  const y1 = box.y + box.height * 0.25;
  const y2 = box.y + box.height * 0.8;

  // Зонд: кадры прокрутки ленты
  await page.evaluate(() => {
    const c = document.querySelector('[role="dialog"] .overflow-y-auto') as HTMLElement;
    (window as unknown as { __stFrames: Array<{ st: number; ch: number; sh: number }> }).__stFrames = [];
    const probe = window as unknown as {
      __stFrames: Array<{ st: number; ch: number; sh: number }>;
    };
    const tick = () => {
      probe.__stFrames.push({
        st: Math.round(c.scrollTop),
        ch: Math.round(c.clientHeight),
        sh: Math.round(c.scrollHeight),
      });
      if (probe.__stFrames.length < 2000) requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  });

  // Десять жёстких свайпов вверх экрана = прокрутка вниз в прошлое
  // (план 053: лента растёт вниз), непрерывно.
  for (let f = 0; f < 10; f += 1) {
    await cdp.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [{ x, y: y2 }] });
    for (let s = 1; s <= 6; s += 1) {
      await cdp.send('Input.dispatchTouchEvent', {
        type: 'touchMove',
        touchPoints: [{ x, y: y2 + (y1 - y2) * (s / 6) }],
      });
    }
    await cdp.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
    await page.waitForTimeout(60);
  }
  await page.waitForTimeout(2000);
  await cdp.send('Emulation.setCPUThrottlingRate', { rate: 1 });

  const frames = await page.evaluate(
    () =>
      (window as unknown as { __stFrames: Array<{ st: number; ch: number; sh: number }> }).__stFrames,
  );
  const firstFrame = frames[0];
  const lastFrame = frames.at(-1);
  if (firstFrame === undefined || lastFrame === undefined) {
    throw new Error('зонд кадров пуст — rAF не тикал');
  }
  const shGrowth = lastFrame.sh - firstFrame.sh;
  // Дорисовки состоялись: забег (~5500px) съел стартовый запас (~4000px)
  // и потребовал минимум одну порцию.
  expect(shGrowth, 'лента доросла под забег').toBeGreaterThanOrEqual(1000);

  // За весь забег прокрутка ни разу не коснулась нижней стены: триггер
  // дорисовки стоит далеко от края (кадр после первого движения, касание
  // стены в паузу рендера — то самое дёрганье, теперь оно не возникает).
  const firstMoved = frames.findIndex((f) => f.st > 0);
  const inFlight = firstMoved === -1 ? [] : frames.slice(firstMoved);
  const wallFrames = inFlight.filter((f) => f.st + f.ch >= f.sh - 1).length;
  expect(wallFrames, `кадров на нижней стене в полёте (из ${inFlight.length})`).toBeLessThanOrEqual(4);
});

  // Жёсткий флик реальной физики моментума (карта #1437; план 051:
  // протокол на верхней границе реального флика ~4800px суммарного
  // пробега при душащем CPU). Инварианты те же, стена — нижняя.
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
    (window as unknown as { __hardFrames: Array<{ st: number; ch: number; sh: number }>; __evs: number[] }).__hardFrames = [];
    (window as unknown as { __evs: number[] }).__evs = [];
    const probe = window as unknown as { __hardFrames: Array<{ st: number; ch: number; sh: number }>; __evs: number[] };
    c.addEventListener('scroll', () => probe.__evs.push(Math.round(c.scrollTop)));
    const tick = () => {
      probe.__hardFrames.push({
        st: Math.round(c.scrollTop),
        ch: Math.round(c.clientHeight),
        sh: Math.round(c.scrollHeight),
      });
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
    await page.mouse.wheel(0, 600);
    await page.waitForTimeout(30);
  }
  await page.waitForTimeout(2000);
  await cdp.send('Emulation.setCPUThrottlingRate', { rate: 1 });

  const frames = await page.evaluate(
    () =>
      (window as unknown as { __hardFrames: Array<{ st: number; ch: number; sh: number }> })
        .__hardFrames,
  );
  const lastFrame = frames.at(-1);
  const firstFrame = frames[0];
  if (lastFrame === undefined || firstFrame === undefined) {
    throw new Error('зонд кадров пуст — rAF не тикал');
  }
  const shGrowth = lastFrame.sh - firstFrame.sh;
  const firstMoved = frames.findIndex((f) => f.st > 0);
  const inFlight = firstMoved === -1 ? [] : frames.slice(firstMoved);
  const wallFrames = inFlight.filter((f) => f.st + f.ch >= f.sh - 1).length;
  const diag = `frames=${frames.length} last=${JSON.stringify(frames.at(-1))} growths=${frames.filter((frame, i) => i > 0 && frame.sh !== frames[i - 1]?.sh).length} shGrowth=${shGrowth} wallFrames=${wallFrames}`;

  // Забег (~4800px) съел запас (~4000px) и потребовал дорисовку.
  expect(shGrowth, diag).toBeGreaterThanOrEqual(2000);
  // Стена — не более нескольких кадров в полёте; устойчивый цикл
  // «стена → дорисовка» — регресс.
  expect(wallFrames, `кадров на нижней стене в полёте; ${diag}`).toBeLessThanOrEqual(4);
});
