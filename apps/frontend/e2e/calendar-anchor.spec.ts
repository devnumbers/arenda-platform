import type { Page } from '@playwright/test';
import { expect, openCabinetWithSessionToken, test } from './fixtures';

// Ручной якорь приписки (план 051): позиция прокрутки при дорисовке
// месяцев СВЕРХУ возвращается до пейнта ([overflow-anchor:none] + запись
// scrollTop в layout-эффекте). Прежняя ставка на нативный scroll anchoring
// (план 042) работала только на Chromium: в WebKit якоря нет
// (overflow-anchor — Safari 27+), и на телефоне владельца каждая приписка
// телепортировала контент на высоту порции (~2200px) — «жёстко лагает»
// в моментуме; на ПК Chromium компенсировал нативно, поэтому «на ПК нет».
//
// Инвариант журнала — тихая приписка: протокол скроллит всплесками с
// паузами, порция доезжает в паузе. Сигнатура тихой приписки в кадрах
// rAF: scrollHeight вырос на Δ и scrollTop вырос ровно на тот же Δ —
// компенсация (ручная или нативная) вернула ровно вставку, ничего больше
// не скроллило. У такой пары контент обязан стоять: top общего месяца не
// меняется (±допуск на субпиксель). Без компенсации сигнатуры не бывает
// (scrollTop при вставке не двигается вовсе) — партий не набирается:
// на webkit-проекте тест был красным до проводки якоря («Октябрь, 2024»
// уезжал на ~4700px при стоящем скролле). CDP-троттлинг не нужен и
// невозможен на webkit — красный не про производительность.

const UI_KIT = '/ui-kit';

type AnchorFrame = {
  st: number;
  sh: number;
  items: Array<[string, number]>;
};

/** Журнал кадров: st/sh ленты + top всех секций в координатах вьюпорта. */
function installFrameJournal(page: Page): void {
  void page.evaluate(() => {
    const c = document.querySelector(
      '[role="dialog"] .overflow-y-auto',
    ) as HTMLElement;
    const probe = window as unknown as { __anchorFrames: AnchorFrame[] };
    probe.__anchorFrames = [];
    const tick = () => {
      const items = Array.from(c.querySelectorAll('section')).map(
        (section) =>
          [
            section.querySelector('h3')?.textContent ?? '',
            Math.round(section.getBoundingClientRect().top * 10) / 10,
          ] as [string, number],
      );
      probe.__anchorFrames.push({
        st: Math.round(c.scrollTop),
        sh: c.scrollHeight,
        items,
      });
      if (probe.__anchorFrames.length < 4000) {
        requestAnimationFrame(tick);
      }
    };
    requestAnimationFrame(tick);
  });
}

/** Тихая приписка не двигает контент. Сигнатура тихой приписки: между
 * кадрами scrollHeight вырос на Δ и scrollTop вырос ровно на тот же Δ —
 * компенсация (ручная или нативная) вернула ровно вставку, ничего больше
 * не скроллило (паузы протокола). У такой пары контент обязан стоять:
 * top общего месяца не меняется. Без компенсации сигнатуры не бывает
 * (st не растёт вовсе) — партий не наберётся. Пары с чужим скроллом
 * (Δst ≠ Δsh) не проверяются — компенсацию там от скролла не отделить. */
function assertQuietPrependsHoldContent(frames: AnchorFrame[]): void {
  const TOLERANCE_PX = 48;
  let quietPrepends = 0;
  for (let i = 1; i < frames.length; i += 1) {
    const prev = frames[i - 1];
    const frame = frames[i];
    if (prev === undefined || frame === undefined) {
      continue;
    }
    const shDelta = frame.sh - prev.sh;
    const stDelta = frame.st - prev.st;
    if (shDelta <= 0 || stDelta !== shDelta) {
      continue;
    }
    quietPrepends += 1;
    const before = new Map(prev.items);
    let shared = 0;
    for (const [label, top] of frame.items) {
      const prevTop = before.get(label);
      if (prevTop === undefined) {
        continue;
      }
      shared += 1;
      const shift = top - prevTop;
      expect(
        Math.abs(shift),
        `кадр ${i}: приписка на ${shDelta}px вернула scrollTop ровно на вставку, но «${label}» уехал на ${Math.round(shift)}px — компенсация мимо контента`,
      ).toBeLessThanOrEqual(TOLERANCE_PX);
    }
    expect(shared, `кадр ${i}: нет общих месяцев с предыдущим кадром`).toBeGreaterThan(0);
  }
  expect(quietPrepends, 'тихих приписок за забег (компенсация возвращает scrollTop ровно на вставку)').toBeGreaterThanOrEqual(3);
}

test.beforeEach(async ({ page, seededUser }) => {
  await openCabinetWithSessionToken(page, seededUser.sessionToken);
});

test('одиночный пикер (без пола): тихая приписка не двигает контент', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(UI_KIT);
  await page
    .getByRole('button', { name: 'Открыть пикер (allowPast без minDate: задним числом без пола)' })
    .click();
  const dialog = page.getByRole('dialog', { name: 'Выбрать дату' });
  await expect(dialog).toBeVisible();
  await page.waitForTimeout(600);

  const cont = dialog.locator('.overflow-y-auto');
  const box = await cont.boundingBox();
  if (box === null) {
    throw new Error('лента пикера даты не найдена');
  }
  installFrameJournal(page);

  // Забег в прошлое: всплеск — пауза. Порция (≤ 12 месяцев) доезжает в
  // паузе — каждая приписка в журнале тихая. Стартовый запас 12 месяцев
  // держит открытие выше порога 3000; 10 всплесков по 1500px дают ~4
  // приписки.
  await page.mouse.move(box.x + box.width / 2, box.y + box.height * 0.5);
  for (let f = 0; f < 10; f += 1) {
    await page.mouse.wheel(0, -1500);
    await page.waitForTimeout(300);
  }
  await page.waitForTimeout(600);

  const frames = await page.evaluate(
    () => (window as unknown as { __anchorFrames: AnchorFrame[] }).__anchorFrames,
  );
  expect(frames.length, 'журнал кадров не пуст').toBeGreaterThan(100);
  assertQuietPrependsHoldContent(frames);
});

test('пикер периода: тихая приписка не двигает контент', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(UI_KIT);
  await expect(page.getByRole('button', { name: 'Открыть пикер периода' })).toBeVisible();
  await page.waitForTimeout(500);
  await page.getByRole('button', { name: 'Открыть пикер периода' }).click();
  const dialog = page.getByRole('dialog', { name: 'Выберите период' });
  await expect(dialog).toBeVisible();
  await page.waitForTimeout(600);

  const cont = dialog.locator('.overflow-y-auto');
  const box = await cont.boundingBox();
  if (box === null) {
    throw new Error('лента пикера периода не найдена');
  }
  installFrameJournal(page);

  await page.mouse.move(box.x + box.width / 2, box.y + box.height * 0.5);
  for (let f = 0; f < 10; f += 1) {
    await page.mouse.wheel(0, -1500);
    await page.waitForTimeout(300);
  }
  await page.waitForTimeout(600);

  const frames = await page.evaluate(
    () => (window as unknown as { __anchorFrames: AnchorFrame[] }).__anchorFrames,
  );
  expect(frames.length, 'журнал кадров не пуст').toBeGreaterThan(100);
  assertQuietPrependsHoldContent(frames);
});
