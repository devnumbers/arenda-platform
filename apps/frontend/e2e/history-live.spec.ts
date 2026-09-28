import type { Page, TestInfo } from '@playwright/test';
import {
  captureScreen,
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  paymentCreatedEntry,
  seedJournalEntry,
  SEEDED_APARTMENT_PROPERTY_ID,
  type SeededUser,
  test,
} from './fixtures';
import { emitRealtimeFrame, installRealtimeBridge } from './realtime-bridge';

// Живая лента «История действий» (карта #714, тикет #718): кадр history
// realtime-стрима догоняет ленту снизу — влитие свежих строк в первую
// страницу кэша (двусторонний keyset #708). Читатель на дне видит новые
// строки сам («как в мессенджере»); листающий старое остаётся на месте
// без рывков — над «Настройками» появляется
// индикатор «Есть новые», клик ведёт к свежим, ручной доскролл гасит его.
// Строки сеются прямым INSERT'ом — сид мимо рекордера, кадров бэк не шлёт,
// поэтому кадр диспатчит тестовый мост realtime-bridge; чтение ленты при
// этом идёт через живой бэк. Двухоконная приёмка с настоящей мутацией —
// /ui-walkthrough по тикету.

/** Дневная серия из N строк владельца (сегодня, 8:xx) — заполняет вьюпорт,
 * чтобы append свежих был ниже сгиба, а прокрутка имела смысл. */
async function seedDaySeries(user: SeededUser, count: number): Promise<void> {
  for (let index = 0; index < count; index += 1) {
    await seedJournalEntry(
      paymentCreatedEntry(
        `b0000000-0000-4000-8000-0000000000${String(index + 10).padStart(2, '0')}`,
        60 - index,
        `Заготовка ${index + 1}`,
      ),
      user,
    );
  }
}

/** Свежая строка поверх серии — момент после 9:00 (новее всех заготовок). */
function freshEntry(id: string, title: string) {
  return paymentCreatedEntry(id, -5, title);
}

async function scrollMetrics(page: Page): Promise<{
  y: number;
  distanceToBottom: number;
}> {
  return page.evaluate(() => ({
    y: window.scrollY,
    distanceToBottom:
      document.documentElement.scrollHeight - (window.scrollY + window.innerHeight),
  }));
}

test('кадр history у читателя на дне — свежая строка появляется сама, без индикатора', async ({ page, seededUser }, testInfo: TestInfo) => {
  test.setTimeout(60_000);
  await execE2eSql('DELETE FROM action_journal;');
  await installRealtimeBridge(page);
  await openCabinetWithSeededSession(page, seededUser);
  await seedDaySeries(seededUser, 20);
  await page.goto('/history');

  // Лента загрузилась и встала на дно (якорь первой загрузки).
  await expect(page.getByText('Платёж создан: Заготовка 20', { exact: true })).toBeVisible();
  const anchored = await scrollMetrics(page);
  expect(anchored.distanceToBottom).toBeLessThan(120);

  // Мутация «произошлась»: строка в журнале (сид), кадр — мост.
  await seedJournalEntry(
    freshEntry('b0000000-0000-4000-8000-000000000099', 'Живая строка'),
    seededUser,
  );
  await emitRealtimeFrame(page, { entity: 'history', propertyId: SEEDED_APARTMENT_PROPERTY_ID });

  // Свежая строка раскрылась сама, лента по-прежнему на дне, индикатора нет.
  await expect(page.getByText('Платёж создан: Живая строка', { exact: true })).toBeVisible();
  const after = await scrollMetrics(page);
  expect(after.distanceToBottom).toBeLessThan(120);
  await expect(page.getByRole('button', { name: 'Есть новые' })).toHaveCount(0);

  await captureScreen(page, testInfo, 'history-live-follow');
});

test('кадр history листающему старое — индикатор «Есть новые» без рывка, клик ведёт к свежим', async ({ page, seededUser }, testInfo: TestInfo) => {
  test.setTimeout(60_000);
  await execE2eSql('DELETE FROM action_journal;');
  await installRealtimeBridge(page);
  await openCabinetWithSeededSession(page, seededUser);
  await seedDaySeries(seededUser, 20);
  await page.goto('/history');

  await expect(page.getByText('Платёж создан: Заготовка 20', { exact: true })).toBeVisible();
  // Ушёл читать старое — верх ленты.
  await page.evaluate(() => window.scrollTo(0, 0));
  await expect.poll(async () => (await scrollMetrics(page)).y).toBe(0);

  await seedJournalEntry(
    freshEntry('b0000000-0000-4000-8000-000000000098', 'Живая строка два'),
    seededUser,
  );
  await emitRealtimeFrame(page, { entity: 'history', propertyId: SEEDED_APARTMENT_PROPERTY_ID });

  // Индикатор появился, позиция чтения не дёрнулась.
  const indicator = page.getByRole('button', { name: 'Есть новые' });
  await expect(indicator).toBeVisible();
  const reading = await scrollMetrics(page);
  expect(reading.y).toBe(0);

  // Клик — плавный ход к свежим: строка видима, лента у дна, индикатор погас.
  await indicator.click();
  await expect(page.getByText('Платёж создан: Живая строка два', { exact: true })).toBeVisible();
  await expect.poll(async () => (await scrollMetrics(page)).distanceToBottom).toBeLessThan(160);
  await expect(indicator).toHaveCount(0);

  await captureScreen(page, testInfo, 'history-live-indicator');
});
