import { expect, openCabinetWithSessionToken, test } from './fixtures';
import type { Page } from '@playwright/test';

// Холодная загрузка хабов: история канона — план 044/045 (теневые
// loading.tsx, карта #1437) сменила аменда #1605 (react/react#37014):
// страницы с серверным префетчем больше не оборачиваются в Suspense, и
// на маршрутах без loading.tsx HTML однофлушный — холодный вход рисует
// кадр с данными и хромом из реестра, скелетон не стримится вовсе.
// Гонка гидратации React 19.2 (batched Suspense reveal) закрыта по
// построению; спеки ниже — гарды нового канона.
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

/** Журнал холодного входа хаба «Участники»: следит, чтобы скелетон хаба
 * не появился ни разу. Канон однофлушного входа (аменда #1605) —
 * скелетон на холодном входе не стримится вовсе, шапка приходит из
 * реестра маршрутов в том же HTML; голая страница без шапки =
 * регрессия (класс C9, «шапка прыгает»). */
function firstFrameJournal(): void {
  const log: MarkerMap = {};
  (window as unknown as { __hubColdLog: MarkerMap }).__hubColdLog = log;
  const observer = new MutationObserver(() => {
    if (document.querySelector('[data-testid="participants-list-loading"]') !== null) {
      log['participants-list-loading'] = true;
    }
  });
  observer.observe(document, { childList: true, subtree: true });
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

  test('хаб «Контакты»: холодный вход однофлушный — скелетона нет, данные первым кадром', async ({
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

    // Канон однофлушного холодного входа (аменда #1605, react/react#37014):
    // ServerPrefetchBoundary ждёт данные до рендера, скелетон на холодном
    // входе не стримится вовсе — первый кадр и есть кадр с данными.
    const log = await readLog(page);
    expect(
      log['contacts-loading'],
      'скелетон хаба «Контакты» на холодном входе не стримится',
    ).toBeUndefined();
  });
});

test.describe('холодная загрузка хаба «Участники» (гард хрома §7)', () => {
  test('кадр холодной загрузки несёт хром экрана — шапка из реестра в первом кадре, скелетона нет', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSessionToken(page, seededUser.sessionToken);
    await page.addInitScript(firstFrameJournal);
    await coldNetwork(page);

    await page.goto(`${process.env.E2E_BASE_URL ?? 'http://127.0.0.1:3010'}/participants`, {
      waitUntil: 'commit',
    });
    await page.waitForLoadState('load');
    await expect(page.getByText('Участники').first()).toBeVisible();

    const log = await readLog(page);
    // Однофлушный вход (аменда #1605): скелетон хаба не стримится вовсе.
    expect(
      log['participants-list-loading'],
      'скелетон хаба «Участники» на холодном входе не стримится',
    ).toBeUndefined();
    // Хром первого кадра: шапка персистентной шапки приходит из реестра
    // маршрутов в том же однофлушном HTML (голая страница без шапки =
    // регрессия «шапка прыгает»).
    await expect(
      page.locator('header[aria-label="Навигация экрана"]'),
    ).toBeVisible();
  });
});
