import { expect, BASE_URL, openCabinetWithSessionToken, SEEDED_GARAGE_PROPERTY_ID, test } from './fixtures';
import type { Page } from '@playwright/test';

// Тикет #1602 (карта #1587): data-слой скелетона при возврате назад.
// Диагноз #1593: возврат на экран через >30 c — экран снова играет
// «скелетон → контент», видимое «всё перезагрузилось», включая иконки
// пустых состояний. Слой — data, не сеть: экран хаба «Платежи» читает
// четыре ключа первого кадра (лента, стопки объектов, «На оплату»,
// справочник объектов), а слои префетча — серверный #887 и прогрев #626 —
// покрывали только два; на входе без данных в кэше (холодный вход,
// возврат после gcTime-выселения) «На оплату» отсутствует и виджет
// рисует pending-скелетон. Канон «первый кадр читает → префетчит»
// (DESIGN.md §15): pending-скелетон виджета не появляется ни на одном
// входе — ни холодном, ни возвратом. Гард по механике
// hub-cold-loading.spec.ts: журнал MutationObserver из init-скрипта.

type MarkerMap = Record<string, boolean>;

/** Троттлинг + отключение кеша: фазы загрузки наблюдаемы. */
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

/** Журнал pending-скелетонов data-слоя: маркеры инлайн — addInitScript
 * сериализует только тело функции. Скелетон детали объекта маркера не
 * имеет — ловится по его aria-label (PropertyDetailLoading). */
function journalScript(): void {
  const markers = [
    { key: 'payments-hub-pending', selector: '[data-testid="payments-hub-pending"]' },
    { key: 'property-payments-pending', selector: '[data-testid="property-payments-pending"]' },
    { key: 'property-detail-loading', selector: '[aria-label="Загрузка объекта"]' },
  ];
  const log: MarkerMap = {};
  (window as unknown as { __returnSkeletonLog: MarkerMap }).__returnSkeletonLog = log;
  const observer = new MutationObserver(() => {
    for (const { key, selector } of markers) {
      if (document.querySelector(selector) !== null) {
        log[key] = true;
      }
    }
  });
  observer.observe(document, { childList: true, subtree: true });
}

async function readLog(page: Page): Promise<MarkerMap> {
  return page.evaluate(
    () => (window as unknown as { __returnSkeletonLog: MarkerMap }).__returnSkeletonLog,
  );
}

/** Дождаться гидрационного свапа RSC после клиентской навигации (тот же
 * предикат, что в обёртке goto фикстуры — goBack она не оборачивает). */
async function settleHydrationSwap(page: Page): Promise<void> {
  await page
    .waitForFunction(() => !document.querySelector('body > div[hidden] header'), undefined, {
      timeout: 15_000,
    })
    .catch(() => {});
}

test.describe('скелетон data-слоя при входе и возврате (тикет #1602)', () => {
  test('хаб «Платежи»: холодный вход — первый кадр с данными, pending-скелетон не появляется', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSessionToken(page, seededUser.sessionToken);
    await page.addInitScript(journalScript);
    await coldNetwork(page);

    await page.goto(`${BASE_URL}/payments`, { waitUntil: 'commit' });
    await page.waitForLoadState('load');
    // Страница доехала — прогон не пустой.
    await expect(page.getByTestId('payments-section-favorites')).toBeVisible();

    const log = await readLog(page);
    expect(log['payments-hub-pending'], 'pending-скелетон хаба «Платежи» на холодном входе').toBeUndefined();
  });

  test('«Платежи объекта»: холодный вход — pending-скелетон не появляется', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSessionToken(page, seededUser.sessionToken);
    await page.addInitScript(journalScript);
    await coldNetwork(page);

    await page.goto(
      `${BASE_URL}/properties/${SEEDED_GARAGE_PROPERTY_ID}/payments`,
      { waitUntil: 'commit' },
    );
    await page.waitForLoadState('load');
    await expect(page.getByTestId('section-overdue')).toBeVisible();

    const log = await readLog(page);
    expect(
      log['property-payments-pending'],
      'pending-скелетон экрана «Платежи объекта» на холодном входе',
    ).toBeUndefined();
  });

  test('возврат назад на объект через >30 c — без скелетон-свопа', async ({
    page,
    seededUser,
  }) => {
    test.setTimeout(120_000);
    await openCabinetWithSessionToken(page, seededUser.sessionToken);
    await page.addInitScript(journalScript);
    await page.goto(
      `${BASE_URL}/properties/${SEEDED_GARAGE_PROPERTY_ID}`,
      { waitUntil: 'commit' },
    );
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();

    // Уходим в хаб и живём там дольше staleTime (30 с).
    await page.locator('nav a[href="/payments"]').first().click();
    await expect(page).toHaveURL(/\/payments$/);
    await page.waitForTimeout(31_000);

    await page.evaluate(() => {
      delete (window as unknown as { __returnSkeletonLog: MarkerMap }).__returnSkeletonLog[
        'property-detail-loading'
      ];
    });
    await page.goBack();
    await settleHydrationSwap(page);
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();

    const log = await readLog(page);
    expect(log['property-detail-loading'], 'скелетон-своп при возврате на объект').toBeUndefined();
  });

  test('возврат назад на хаб «Платежи» через >30 c — без скелетон-свопа', async ({
    page,
    seededUser,
  }) => {
    test.setTimeout(120_000);
    await openCabinetWithSessionToken(page, seededUser.sessionToken);
    await page.addInitScript(journalScript);
    // Возврат — тёплый документ без троттлинга, как на телефоне.
    await page.goto(`${BASE_URL}/payments`, { waitUntil: 'commit' });
    await expect(page.getByTestId('payments-section-favorites')).toBeVisible();

    // Уходим на подэкран и живём там дольше staleTime (30 с) — сценарий
    // владельца из #1593.
    await page.getByRole('button', { name: 'Избранные' }).click();
    await expect(page).toHaveURL(/\/payments\/favorites$/);
    await page.waitForTimeout(31_000);

    // Фаза возврата — с чистым журналом: маркер холодного входа (его
    // ловит первый тест) сюда не перетекает.
    await page.evaluate(() => {
      delete (window as unknown as { __returnSkeletonLog: MarkerMap }).__returnSkeletonLog[
        'payments-hub-pending'
      ];
    });
    await page.goBack();
    await settleHydrationSwap(page);
    await expect(page.getByTestId('payments-section-favorites')).toBeVisible();

    const log = await readLog(page);
    expect(log['payments-hub-pending'], 'скелетон-своп при возврате назад').toBeUndefined();
  });
});
