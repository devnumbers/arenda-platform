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
