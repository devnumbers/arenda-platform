import {
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  paymentCreatedEntry,
  seedJournalEntry,
  test,
} from './fixtures';
import type { Page } from '@playwright/test';

// Изоляция скролла (карта #1416, тикет #1417, конвенция «одно правило на
// слой»): фуллскрин-поверхность обязана замораживать фон на всё открытие
// (useBodyScrollLock — канон календарных пикеров, решение владельца
// 2026-10-13). Жест у края внутренней ленты не уезжает в цепочку скролла
// страницы, после закрытия поверхности позиция чтения та же.
//
// Замер — скроллеры документа: scrollingElement.scrollTop (корневой) и
// body.scrollTop (под локом body — скролл-контейнер; цепочка туда тоже
// гасится). mouse.wheel шлёт реальные wheel-события и цепочку
// воспроизводит честно (прецеденты: scroll-shift.spec.ts,
// calendar-feed.spec.ts).

type DocScroll = { readonly root: number; readonly body: number };

async function docScroll(page: Page): Promise<DocScroll> {
  return page.evaluate(() => ({
    root: document.scrollingElement?.scrollTop ?? -1,
    body: document.body.scrollTop,
  }));
}

/** Колёсные всплески в точке: вниз к краю и вверх за край — попытка
 * отдать жест цепочкой в обоих направлениях. */
async function wheelBursts(page: Page, x: number, y: number): Promise<void> {
  await page.mouse.move(x, y);
  for (let i = 0; i < 5; i += 1) {
    await page.mouse.wheel(0, 600);
  }
  for (let i = 0; i < 8; i += 1) {
    await page.mouse.wheel(0, -600);
  }
  await page.waitForTimeout(300);
}

/** Гарантированно прокручиваемый фон: распорка над контентом + прокрутка
 * документа вниз. */
async function scrollBackground(page: Page): Promise<void> {
  await page.evaluate(() => {
    const spacer = document.createElement('div');
    spacer.style.height = '2500px';
    document.body.prepend(spacer);
  });
  await page.mouse.move(200, 400);
  for (let i = 0; i < 6; i += 1) {
    await page.mouse.wheel(0, 700);
  }
  await page.waitForTimeout(300);
}

test.describe('Изоляция скролла', () => {
  test('фон заморожен под поверхностью: шит фильтров истории', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.setViewportSize({ width: 390, height: 844 });
    await execE2eSql('DELETE FROM action_journal;');
    for (let i = 0; i < 12; i += 1) {
      await seedJournalEntry(
        paymentCreatedEntry(
          `b1000000-0000-4000-8000-0000000000${String(i).padStart(2, '0')}`,
          i,
          `Стресс ${i}`,
        ),
        seededUser,
      );
    }
    await page.goto('/history');
    await expect(page.getByText('Платёж создан: Стресс 0')).toBeVisible();
    await scrollBackground(page);

    await page.getByRole('button', { name: 'Настройки' }).click();
    const dialog = page.getByRole('dialog', { name: 'Фильтры истории' });
    await expect(dialog).toBeVisible();
    // База снимается ПОСЛЕ открытия: клик мог автоскроллить страницу.
    const before = await docScroll(page);

    // Всплески: центр ленты шита, верхний край ленты (симптом тикета),
    // шапка поверхности — вне ленты.
    const list = dialog.locator('.overflow-y-auto');
    const listBox = await list.boundingBox();
    const dialogBox = await dialog.boundingBox();
    if (listBox === null || dialogBox === null) {
      throw new Error('лента или каркас шита фильтров не найдены');
    }
    await wheelBursts(page, listBox.x + listBox.width / 2, listBox.y + listBox.height / 2);
    await wheelBursts(page, listBox.x + listBox.width / 2, listBox.y + 8);
    await wheelBursts(page, dialogBox.x + dialogBox.width / 2, dialogBox.y + 30);

    const during = await docScroll(page);
    expect(during.root).toBe(before.root);
    expect(during.body).toBe(0);

    await page.getByRole('button', { name: 'Закрыть фильтры' }).click();
    await expect(dialog).toBeHidden();
    expect(await docScroll(page)).toEqual(before);
  });

  test('фон заморожен под поверхностью: пикер объектов приглашения', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/participants/invite');
    await expect(page.getByText('Все 3 объекта')).toBeVisible();
    await scrollBackground(page);

    await page.getByText('Все 3 объекта').click();
    const picker = page.getByRole('dialog', { name: 'Выбрать объект' });
    await expect(picker).toBeVisible();
    const before = await docScroll(page);

    const list = picker.locator('.overflow-y-auto');
    const listBox = await list.boundingBox();
    if (listBox === null) {
      throw new Error('лента пикера объектов не найдена');
    }
    await wheelBursts(page, listBox.x + listBox.width / 2, listBox.y + 8);

    const during = await docScroll(page);
    expect(during.root).toBe(before.root);
    expect(during.body).toBe(0);
  });

  test('фон заморожен под поверхностью: пикер периода (регресс канона)', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/ui-kit');
    await scrollBackground(page);

    await page.getByRole('button', { name: 'Открыть пикер периода' }).click();
    const dialog = page.getByRole('dialog', { name: 'Выберите период' });
    await expect(dialog).toBeVisible();
    const before = await docScroll(page);

    const list = dialog.locator('.overflow-y-auto');
    const listBox = await list.boundingBox();
    if (listBox === null) {
      throw new Error('лента пикера периода не найдена');
    }
    await wheelBursts(page, listBox.x + listBox.width / 2, listBox.y + 8);

    expect((await docScroll(page)).root).toBe(before.root);
  });
});
