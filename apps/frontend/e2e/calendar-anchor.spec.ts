import type { Page } from '@playwright/test';
import { expect, openCabinetWithSessionToken, test } from './fixtures';
import { MONTH_LABELS } from '../shared/ui/design/month-grid';

// Лента по убыванию (план 053): индекс 0 — верхний месяц у будущего края,
// история хранится вниз и дорисовывается append'ом ниже вьюпорта. Класс
// prepend-багов 042-052 снят архитектурно: на приписке сверху позицию
// держала запись scrollTop (компенсация), WebKit в моментуме терял её
// read-back — окно замирало и прыгало на чанк (фел-чек 16.10: «завис на
// июле 2026, дёрнулся и остановился на декабре 2025» = чанки 6±1 месяца).
// Теперь записей в скролл не существует ни в одном направлении.
//
// Главный инвариант — ноль программных записей scrollTop за весь забег
// в прошлое (счётчик оборачивает сеттер Element.prototype.scrollTop до
// бандла приложения; на коде 042-052 инвариант красный — компенсация
// писала на каждый prepend). Рост истории видит scrollHeight; стена
// снизу не достигается в полёте; в паузах протокола общие месяцы стоят.

const UI_KIT = '/ui-kit';

type AnchorFrame = {
  st: number;
  ch: number;
  sh: number;
  items: Array<[string, number]>;
};

/** Счётчик программных записей scrollTop — addInitScript, до бандла. */
async function installScrollWriteCounter(page: Page): Promise<void> {
  await page.addInitScript(() => {
    const descriptor = Object.getOwnPropertyDescriptor(Element.prototype, 'scrollTop');
    if (
      descriptor === undefined
      || descriptor.get === undefined
      || descriptor.set === undefined
    ) {
      return;
    }
    const nativeGet = descriptor.get;
    const nativeSet = descriptor.set;
    const probe = window as unknown as { __scrollWrites: number[] };
    probe.__scrollWrites = [];
    Object.defineProperty(Element.prototype, 'scrollTop', {
      get: nativeGet,
      set(value: number) {
        probe.__scrollWrites.push(value);
        nativeSet.call(this, value);
      },
      configurable: true,
    });
  });
}

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

/** Append-единственность (план 053): позиция существующего контента в
 * координатах ленты (rect.top + scrollTop) неизменна — офсеты строк не
 * двигаются, видимое смещение объясняется только скроллом. Проверяется
 * по всем парам кадров с общими месяцами: topΔ + stΔ ≈ 0 (допуск —
 * субпиксель measureElement). На коде с компенсацией инвариант красный:
 * потерянный read-back сдвигал контент без скролла. */
function assertAppendOnlyContent(frames: AnchorFrame[]): void {
  const TOLERANCE_PX = 4;
  let pairs = 0;
  for (let i = 1; i < frames.length; i += 1) {
    const prev = frames[i - 1];
    const frame = frames[i];
    if (prev === undefined || frame === undefined) {
      continue;
    }
    const before = new Map(prev.items);
    for (const [label, top] of frame.items) {
      const prevTop = before.get(label);
      if (prevTop === undefined) {
        continue;
      }
      pairs += 1;
      const contentShift = top + frame.st - (prevTop + prev.st);
      expect(
        Math.abs(contentShift),
        `кадр ${i}: «${label}» уехал в координатах ленты на ${Math.round(contentShift)}px — контент сдвигается без скролла`,
      ).toBeLessThanOrEqual(TOLERANCE_PX);
    }
  }
  expect(pairs, 'пар общих месяцев за забег').toBeGreaterThan(100);
}

test.beforeEach(async ({ page, seededUser }) => {
  await openCabinetWithSessionToken(page, seededUser.sessionToken);
});

/** Месяц якоря (текущий) ровно под верхней кромкой — контракт открытия
 * (план 052: позиции по точным высотам; в 053 якорь всегда в верхней
 * части окна — clamp дна невозможен). */
async function assertAnchorOpensAtTop(page: Page): Promise<void> {
  const top = await page.evaluate(() => {
    const el = document.querySelector('[role="dialog"] .overflow-y-auto');
    if (el === null) {
      return null;
    }
    const cRect = el.getBoundingClientRect();
    for (const section of Array.from(el.querySelectorAll('section'))) {
      const r = section.getBoundingClientRect();
      if (r.bottom > cRect.top + 4) {
        return {
          label: section.querySelector('h3')?.textContent ?? '?',
          offset: Math.round(r.top - cRect.top),
        };
      }
    }
    return null;
  });
  expect(top).not.toBeNull();
  const now = new Date();
  const monthLabel = `${MONTH_LABELS[now.getUTCMonth()]}, ${now.getUTCFullYear()}`;
  expect(top?.label, 'месяц якоря под верхней кромкой').toBe(monthLabel);
  expect(
    Math.abs(top?.offset ?? 999),
    `offset верхней кромки у «${top?.label}»`,
  ).toBeLessThanOrEqual(4);
}

for (const picker of [
  {
    name: 'одиночный пикер (без пола)',
    open: 'Открыть пикер (allowPast без minDate: задним числом без пола)',
    dialogName: 'Выбрать дату',
  },
  {
    name: 'пикер периода',
    open: 'Открыть пикер периода',
    dialogName: 'Выберите период',
  },
] as const) {
  test(`${picker.name}: забег в прошлое — ноль записей в скролл, история растёт вниз`, async ({
    page,
  }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await installScrollWriteCounter(page);
    await page.goto(UI_KIT);
    // Клик до гидрации теряется (кнопка уже в DOM, обработчик ещё нет —
    // на холодном стеке next-JIT гидрация webkit дольше секунды): ретраим
    // клик до появления диалога.
    const openButton = page.getByRole('button', { name: picker.open });
    const dialog = page.getByRole('dialog', { name: picker.dialogName });
    await expect(async () => {
      await openButton.click();
      await expect(dialog).toBeVisible({ timeout: 2000 });
    }).toPass({ timeout: 30000 });
    await expect(dialog).toBeVisible();
    await page.waitForTimeout(600);
    await assertAnchorOpensAtTop(page);

    const cont = dialog.locator('.overflow-y-auto');
    const box = await cont.boundingBox();
    if (box === null) {
      throw new Error('лента пикера не найдена');
    }
    // Записи открытия (якорный scrollToOffset) — легитимные; забег
    // меряем от чистого счётчика.
    await page.evaluate(() => {
      (window as unknown as { __scrollWrites: number[] }).__scrollWrites = [];
    });
    installFrameJournal(page);

    // Забег в прошлое (теперь это прокрутка ВНИЗ): всплеск — пауза.
    // 10 всплесков по 1500px съедают стартовый запас (~4300px) и требуют
    // дорисовок.
    await page.mouse.move(box.x + box.width / 2, box.y + box.height * 0.5);
    for (let f = 0; f < 10; f += 1) {
      await page.mouse.wheel(0, 1500);
      await page.waitForTimeout(300);
    }
    await page.waitForTimeout(600);

    const frames = await page.evaluate(
      () => (window as unknown as { __anchorFrames: AnchorFrame[] }).__anchorFrames,
    );
    const writes = await page.evaluate(
      () => (window as unknown as { __scrollWrites: number[] }).__scrollWrites,
    );
    expect(frames.length, 'журнал кадров не пуст').toBeGreaterThan(100);
    expect(
      writes,
      `программных записей scrollTop за забег: ${JSON.stringify(writes.slice(0, 8))} — позицию пишет кто-то, класс prepend-багов вернулся`,
    ).toHaveLength(0);

    // Стена снизу не достигается в полёте (кадры после первого движения).
    const firstMoved = frames.findIndex((f) => f.st > 0);
    const inFlight = firstMoved === -1 ? [] : frames.slice(firstMoved);
    const wallFrames = inFlight.filter((f) => f.st + f.ch >= f.sh - 1).length;
    expect(
      wallFrames,
      `кадров на нижней стене в полёте — дорисовка не успевает`,
    ).toBeLessThanOrEqual(4);

    // История выросла: забег (~15000px) съел стартовый запас (~4300px)
    // и потребовал несколько порций.
    const shGrowth =
      (frames.at(-1)?.sh ?? 0) - (frames[0]?.sh ?? 0);
    expect(shGrowth, 'рост ленты за забег').toBeGreaterThanOrEqual(2000);

    assertAppendOnlyContent(frames);
  });
}
