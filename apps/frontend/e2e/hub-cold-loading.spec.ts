import { expect, openCabinetWithSessionToken, test } from './fixtures';
import type { Page } from '@playwright/test';

// Теневые loading.tsx хабов (карта #1437, план 045, решение владельца
// 16.10 «давай сразу все»): сегментный loading.tsx стримится первым кадром
// холодной загрузки ВСЕХ детей сегмента — префетчащий ребёнок получает
// скелетон родительского списка вместо своего архетипа. Снос файла чинит
// детей (in-page fallback) и не меняет кадр самого хаба (тот же виджет
// через in-page границу страницы).
//
// Механика та же, что в properties-cold-loading.spec.ts (план 044):
// MutationObserver-журнал из init-скрипта, троттлинг сети, сброс кеша.

type MarkerMap = Record<string, boolean>;

/** Троттлинг + отключение кеша: фазы стриминга наблюдаемы. */
async function coldNetwork(page: Page): Promise<void> {
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Network.enable');
  await cdp.send('Network.setCacheDisabled', { cacheDisabled: true });
  await cdp.send('Network.emulateNetworkConditions', {
    offline: false,
    latency: 700,
    downloadThroughput: (1.6 * 1024 * 1024) / 8,
    uploadThroughput: (750 * 1024) / 8,
  });
}

function journalScript(): void {
  // Маркеры инлайн: addInitScript сериализует только тело функции,
  // внешние константы в браузере не существуют.
  const markers = [
    'contacts-loading',
    'payments-loading',
    'notifications-loading',
    'participants-list-loading',
  ];
  const log: MarkerMap = {};
  (window as unknown as { __hubColdLog: MarkerMap }).__hubColdLog = log;
  const observer = new MutationObserver(() => {
    for (const marker of markers) {
      if (document.querySelector(`[data-testid="${marker}"]`) !== null) {
        log[marker] = true;
      }
    }
  });
  observer.observe(document, { childList: true, subtree: true });
}

async function readLog(page: Page): Promise<MarkerMap> {
  return page.evaluate(
    () => (window as unknown as { __hubColdLog: MarkerMap }).__hubColdLog,
  );
}

test.describe('холодная загрузка детей хабов (план 045)', () => {
  test('«Платежи на оплату»: скелетон хаба «Платежи» не стримится', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSessionToken(page, seededUser.sessionToken);
    await page.addInitScript(journalScript);
    await coldNetwork(page);

    await page.goto(`${process.env.E2E_BASE_URL ?? 'http://127.0.0.1:3010'}/payments/due`, {
      waitUntil: 'commit',
    });
    await page.waitForLoadState('load');
    // Страница доехала — прогон не пустой.
    await expect(page.getByText('Платежи на оплату')).toBeVisible();

    const log = await readLog(page);
    expect(log['payments-loading'], 'скелетон хаба «Платежи» на /payments/due').toBeUndefined();
  });

  test('участник: скелетон списка участников не стримится', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSessionToken(page, seededUser.sessionToken);
    await page.addInitScript(journalScript);
    await coldNetwork(page);

    // full_access-участник квартиры из сида (seed.sql, матрица ролей #467).
    await page.goto(
      `${process.env.E2E_BASE_URL ?? 'http://127.0.0.1:3010'}/participants/99999999-9999-4999-8999-999999999931`,
      { waitUntil: 'commit' },
    );
    await page.waitForLoadState('load');
    await expect(page.getByText('Участник').first()).toBeVisible();

    const log = await readLog(page);
    expect(
      log['participants-list-loading'],
      'скелетон списка участников на странице участника',
    ).toBeUndefined();
  });

  test('хаб «Контакты»: свой кадр живёт и после сноса loading.tsx', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSessionToken(page, seededUser.sessionToken);
    await page.addInitScript(journalScript);
    await coldNetwork(page);

    await page.goto(`${process.env.E2E_BASE_URL ?? 'http://127.0.0.1:3010'}/contacts`, {
      waitUntil: 'commit',
    });
    await page.waitForLoadState('load');
    await expect(page.getByText('Контакты').first()).toBeVisible();

    // Гард регрессии сноса: кадр холодной загрузки хаба существует —
    // после сноса сегментного файла его рисует in-page fallback страницы
    // (тот же виджет), пустого окна быть не должно.
    const log = await readLog(page);
    expect(log['contacts-loading'], 'кадр загрузки хаба «Контакты» стримился').toBe(true);
  });
});
