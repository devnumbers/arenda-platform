import { expect, openCabinetWithSessionToken, test } from './fixtures';

// Дорисовка ленты месяцев (пикер периода, #1428 → план 054): лента по
// макету Т-Банка — снизу будущее (жёсткий край: «сегодня» + 2 приглушённых),
// сверху прошлое (бесконечное, prepend — позицию держит библиотека
// anchorTo: 'end'). Антистенная механика прошлого края: порция
// дорисовывается задолго до верхнего упора, инерция не зажимается о край
// в паузу рендера. Протоколы фликов перевёрнуты: прошлое теперь ВВЕРХ.

const UI_KIT = '/ui-kit';

test.beforeEach(async ({ page, seededUser }) => {
  await openCabinetWithSessionToken(page, seededUser.sessionToken);
});

test('дорисовка месяцев срабатывает с запасом от верхнего упора, флик не зажимается', async ({ page }) => {
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
  // Траектория внутри ленты: низ визуально уходит под фиксированную
  // нижнюю панель — тач на 95% высоты садится на её кнопки.
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

  // Десять жёстких свайпов вниз экрана = прокрутка вверх в прошлое
  // (план 054: прошлое сверху), непрерывно.
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

  // За весь забег прокрутка ни разу не коснулась верхнего упора
  // (scrollTop 0): триггер дорисовки стоит далеко от края — касание
  // упора в паузу рендера было тем самым дёрганьем, теперь его нет.
  const firstMoved = frames.findIndex((f) => f.st < firstFrame.st);
  const inFlight = firstMoved === -1 ? [] : frames.slice(firstMoved);
  const wallFrames = inFlight.filter((f) => f.st <= 0).length;
  expect(wallFrames, `кадров на верхнем упоре в полёте (из ${inFlight.length})`).toBeLessThanOrEqual(4);
});

  // Жёсткий флик реальной физики моментума (карта #1437; протокол на
  // верхней границе реального флика ~4800px при душащем CPU, план 051).
  // Инвариант тот же, направление перевёрнуто: прошлый край — верхний.
  test('жёсткий флик под душащим CPU: упор не достигается, лента успевает', async ({ page }) => {
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

  // Восемь всплесков по 600px с паузой 30мс — суммарный пробег ~4800px;
  // колесо с курсором над лентой (Input.synthesizeScrollGesture под
  // троттлингом скроллит композитором, не диспатчив ни одного
  // scroll-события — замер 16.10). deltaY < 0 = прокрутка вверх = в
  // прошлое (план 054: прошлое сверху).
  await page.mouse.move(box.x + box.width / 2, box.y + box.height * 0.5);
  for (let f = 0; f < 8; f += 1) {
    await page.mouse.wheel(0, -600);
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
  const firstMoved = frames.findIndex((f) => f.st < firstFrame.st);
  const inFlight = firstMoved === -1 ? [] : frames.slice(firstMoved);
  const wallFrames = inFlight.filter((f) => f.st <= 0).length;
  const diag = `frames=${frames.length} last=${JSON.stringify(frames.at(-1))} growths=${frames.filter((frame, i) => i > 0 && frame.sh !== frames[i - 1]?.sh).length} shGrowth=${shGrowth} wallFrames=${wallFrames}`;

  // Забег (~4800px) съел запас (~4000px) и потребовал дорисовку.
  expect(shGrowth, diag).toBeGreaterThanOrEqual(2000);
  // Упор — не более нескольких кадров в полёте; устойчивый цикл
  // «упор → дорисовка» — регресс.
  expect(wallFrames, `кадров на верхнем упоре в полёте; ${diag}`).toBeLessThanOrEqual(4);
});

// Репро владельца 19.10 («стена апрель 2025»): флик инерции телефона
// обгонял дорисовку, лента билась в scrollTop 0 — у упора scroll-событий
// не бывает вовсе, а флаг гистерезиса 054, сбрасываемый только выходом из
// зоны, запирал все следующие жесты (ПК чисто: колесо не перескакивает
// буфер одним жестом). Контракт 055: после смерти инерции об упор лента
// доращивается от самого упора — жест (touchstart/wheel) у стены вызывает
// prepend без «джиги» вниз за порог.
test('стена не держит: после упора инерции жест вверх доращивает ленту', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(UI_KIT);
  await expect(page.getByRole('button', { name: 'Открыть пикер периода' })).toBeVisible();
  await page.waitForTimeout(500);
  await page.getByRole('button', { name: 'Открыть пикер периода' }).click();
  const dialog = page.getByRole('dialog', { name: 'Выберите период' });
  await expect(dialog).toBeVisible();
  await page.waitForTimeout(800);

  const cont = dialog.locator('.overflow-y-auto');
  const box = await cont.boundingBox();
  if (box === null) {
    throw new Error('лента пикера периода не найдена');
  }

  // Слэм: инерция обогнала дорисовку и умерла о верхний упор —
  // programmatically ставим ленту в то состояние, в котором телефон
  // застревал навсегда.
  await cont.evaluate((el) => {
    el.scrollTop = 0;
  });
  await page.waitForTimeout(300);

  const shBefore = await cont.evaluate((el) => el.scrollHeight);

  // Пользователь продолжает фликать вверх от упора — колесо над лентой.
  await page.mouse.move(box.x + box.width / 2, box.y + box.height * 0.5);
  for (let f = 0; f < 6; f += 1) {
    await page.mouse.wheel(0, -800);
    await page.waitForTimeout(200);
  }
  await page.waitForTimeout(600);

  const after = await cont.evaluate((el) => ({
    st: Math.round(el.scrollTop),
    sh: el.scrollHeight,
  }));
  const shGrowth = after.sh - shBefore;
  const diag = `before sh=${shBefore} after=${JSON.stringify(after)}`;
  // Лента доращена от самого упора: жест у стены вызвал prepend без
  // отскролла вниз за порог (старый контракт требовал джигу >3000px).
  expect(shGrowth, `рост ленты от упора; ${diag}`).toBeGreaterThanOrEqual(1000);
  // Позиция отошла от упора: контент появился над вьюпортом.
  expect(after.st, `позиция отошла от упора; ${diag}`).toBeGreaterThan(0);
});
