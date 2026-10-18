import type { Page } from '@playwright/test';
import { expect, openCabinetWithSessionToken, test } from './fixtures';
import { MONTH_LABELS } from '../shared/ui/design/month-grid';

// Лента по макету Т-Банка (план 054): снизу будущее, сверху прошлое;
// вьюпорт якорится к нижнему краю (`anchorTo: 'end'` TanStack Virtual —
// официальный чатовый контракт). Открытие — «сегодня» у нижней кромки;
// история досыпается prepend'ом сверху — позицию держит библиотека по
// стабильному ключу месяца; будущее доскролливается append'ом снизу за
// пределами вьюпорта.
//
// Главный инвариант — координаты ленты: rect.top + scrollTop любого
// видимого месяца неизменны в каждой паре кадров забега (допуск —
// субпиксель measureElement). На коде с ручной компенсацией (051/052)
// инвариант красный: потерянный read-back записи сдвигал контент без
// скролла. Рост истории виден по scrollHeight; счётчик программных
// записей 053 снят — библиотека пишет в скролл по дизайну (она теперь
// отвечает за компенсацию).

const UI_KIT = '/ui-kit';

type AnchorFrame = {
  st: number;
  ch: number;
  sh: number;
  items: Array<[string, number]>;
};

/** Журнал кадров: st/ch/sh ленты + top всех секций в координатах вьюпорта. */
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
        ch: Math.round(c.clientHeight),
        sh: Math.round(c.scrollHeight),
        items,
      });
      if (probe.__anchorFrames.length < 4000) {
        requestAnimationFrame(tick);
      }
    };
    requestAnimationFrame(tick);
  });
}

/** Координатная жёсткость ленты (prepend-модель 054): компенсированная
 * вставка не двигает контент в кадре — topΔ общих месяцев ≈ 0, а
 * запись позиции возвращает ровно вставку (topΔ + stΔ − shΔ ≈ 0; на
 * коде 051/052 некомпенсированная вставка давала стойкий topΔ = высоте
 * порции — «жёстко лагает», «завис на июле 2026»). Подстройка позиции
 * библиотекой может доезжать 1-2 кадра после вставки — окно
 * сходимости SETTLE_FRAMES: суммарно от пред-вставки до него контент
 * обязан стоять. Пары кадров вне окон вставок — обычный скролл:
 * topΔ ≈ −stΔ. */
function assertContentStableInFeed(frames: AnchorFrame[]): void {
  const TOLERANCE_PX = 4;
  const SETTLE_FRAMES = 3;
  const topDeltaOver = (a: AnchorFrame, b: AnchorFrame): number | null => {
    const before = new Map(a.items);
    for (const [label, top] of b.items) {
      const prevTop = before.get(label);
      if (prevTop !== undefined) {
        return top - prevTop;
      }
    }
    return null;
  };
  let pairs = 0;
  let settled = 0;
  let blockUntil = -1;
  for (let i = 1; i < frames.length; i += 1) {
    const prev = frames[i - 1];
    const frame = frames[i];
    if (prev === undefined || frame === undefined) {
      continue;
    }
    const shDelta = frame.sh - prev.sh;
    if (shDelta > 0) {
      // Вставка: суммарно от пред-вставки до кадра сходимости контент
      // стоит. Для prepend'а (прошлое, вставка над вьюпортом) подстройка
      // возвращает ровно вставку (topΔ + stΔ ≈ shΔ); для append'а
      // будущего строки уходят за фолд — достаточно topΔ ≈ 0.
      const s = Math.min(i + SETTLE_FRAMES, frames.length - 1);
      const settledFrame = frames[s];
      if (settledFrame !== undefined) {
        const topDelta = topDeltaOver(prev, settledFrame);
        if (topDelta !== null) {
          const stDelta = settledFrame.st - prev.st;
          const shSum = settledFrame.sh - prev.sh;
          // Компенсированный prepend: подстройка перекинула вставку в
          // scrollTop (topΔ + stΔ ≈ shΔ); append будущего: строки за
          // фолдом, видимое объясняется скроллом (topΔ + stΔ ≈ 0).
          // Прыжок некомпенсированной вставки в кадре скролла этой
          // формулой не отличить (палец держит фел-чек) — стойкое
          // «замороженное окно» ловят строгие пары ниже.
          const compensated = Math.abs(topDelta + stDelta - shSum) <= TOLERANCE_PX;
          const plain = Math.abs(topDelta + stDelta) <= TOLERANCE_PX;
          expect(
            compensated || plain,
            `сходимость вставки +${shDelta}px: ни компенсации, ни чистого скролла (topΔ=${Math.round(topDelta)}, stΔ=${Math.round(stDelta)}, shΣ=${Math.round(shSum)})`,
          ).toBe(true);
          settled += 1;
        }
      }
      blockUntil = i + SETTLE_FRAMES;
    }
    if (i <= blockUntil) {
      continue;
    }
    // Обычная пара: видимое смещение объясняется только скроллом.
    const before = new Map(prev.items);
    for (const [label, top] of frame.items) {
      const prevTop = before.get(label);
      if (prevTop === undefined) {
        continue;
      }
      pairs += 1;
      const shift = top - prevTop + (frame.st - prev.st);
      expect(
        Math.abs(shift),
        `кадр ${i}: «${label}» уехал на ${Math.round(shift)}px мимо скролла`,
      ).toBeLessThanOrEqual(TOLERANCE_PX);
    }
  }
  expect(pairs, 'пар общих месяцев за забег').toBeGreaterThan(100);
  expect(settled, 'вставок, сошедшихся за окно сходимости').toBeGreaterThanOrEqual(1);
}

test.beforeEach(async ({ page, seededUser }) => {
  await openCabinetWithSessionToken(page, seededUser.sessionToken);
});

/** Открытие — месяц якоря («сегодня», при правке — значения) ровно под
 * верхней кромкой ленты, ≤ 4px (контракт бага-2 052; в раскладке 054
 * приглушённый контекст краёв виден ниже). */
async function assertOpensAtAnchor(page: Page): Promise<void> {
  const now = new Date();
  const todayLabel = `${MONTH_LABELS[now.getUTCMonth()]}, ${now.getUTCFullYear()}`;
  const diag = await page.evaluate((label) => {
    const el = document.querySelector('[role="dialog"] .overflow-y-auto');
    if (el === null) {
      return null;
    }
    const cRect = el.getBoundingClientRect();
    let topGap: number | null = null;
    for (const section of Array.from(el.querySelectorAll('section'))) {
      const sectionLabel = section.querySelector('h3')?.textContent ?? '';
      if (sectionLabel === label) {
        topGap = section.getBoundingClientRect().top - cRect.top;
        break;
      }
    }
    return {
      topGap,
      st: el.scrollTop,
      ch: el.clientHeight,
      sh: el.scrollHeight,
      labels: Array.from(el.querySelectorAll('section h3')).map((h) => h.textContent),
    };
  }, todayLabel);
  const dump = `st=${diag?.st} ch=${diag?.ch} sh=${diag?.sh} месяцы: ${(diag?.labels ?? []).slice(0, 10).join('; ')}`;
  expect(diag?.topGap, `месяц якоря не найден в DOM (${dump})`).not.toBeNull();
  expect(
    Math.abs(diag?.topGap ?? 999),
    `месяц якоря у верхней кромки (${dump})`,
  ).toBeLessThanOrEqual(4);
}

/** Клик по кнопке открытия с ретраем до появления диалога (холодный стек:
 * клик до гидрации теряется — кнопка уже в DOM, обработчик ещё нет). */
async function openPicker(
  page: Page,
  openName: string,
  dialogName: string,
): Promise<void> {
  await page.goto(UI_KIT);
  const openButton = page.getByRole('button', { name: openName });
  const dialog = page.getByRole('dialog', { name: dialogName });
  await expect(async () => {
    await openButton.click();
    await expect(dialog).toBeVisible({ timeout: 2000 });
  }).toPass({ timeout: 30000 });
  await expect(dialog).toBeVisible();
  await page.waitForTimeout(600);
}

test('одиночный пикер без пола: открытие снизу, история сверху — prepend держит координаты', async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await openPicker(
    page,
    'Открыть пикер (allowPast без minDate: задним числом без пола)',
    'Выбрать дату',
  );
  await assertOpensAtAnchor(page);

  const dialog = page.getByRole('dialog', { name: 'Выбрать дату' });
  const cont = dialog.locator('.overflow-y-auto');
  const box = await cont.boundingBox();
  if (box === null) {
    throw new Error('лента пикера не найдена');
  }
  installFrameJournal(page);

  // Забег в прошлое (вверх): 10 всплесков по 1500px съедают стартовый
  // запас (~4300px) и требуют prepend'ов.
  await page.mouse.move(box.x + box.width / 2, box.y + box.height * 0.5);
  for (let f = 0; f < 10; f += 1) {
    await page.mouse.wheel(0, -1500);
    await page.waitForTimeout(300);
  }
  await page.waitForTimeout(600);

  // И в будущее (вниз): месяцы досыпаются append'ом за краем вьюпорта.
  for (let f = 0; f < 10; f += 1) {
    await page.mouse.wheel(0, 1500);
    await page.waitForTimeout(300);
  }
  await page.waitForTimeout(600);

  const frames = await page.evaluate(
    () => (window as unknown as { __anchorFrames: AnchorFrame[] }).__anchorFrames,
  );
  expect(frames.length, 'журнал кадров не пуст').toBeGreaterThan(100);
  const shGrowth = (frames.at(-1)?.sh ?? 0) - (frames[0]?.sh ?? 0);
  expect(shGrowth, 'лента выросла в обе стороны').toBeGreaterThanOrEqual(3000);
  assertContentStableInFeed(frames);
});

test('пикер периода: открытие снизу, prepend истории держит координаты', async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await openPicker(page, 'Открыть пикер периода', 'Выберите период');
  await assertOpensAtAnchor(page);

  const dialog = page.getByRole('dialog', { name: 'Выберите период' });
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
  const shGrowth = (frames.at(-1)?.sh ?? 0) - (frames[0]?.sh ?? 0);
  expect(shGrowth, 'история доросла под забег').toBeGreaterThanOrEqual(2000);
  assertContentStableInFeed(frames);
});
