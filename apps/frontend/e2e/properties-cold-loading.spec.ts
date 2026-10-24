import {
  expect,
  openCabinetWithSessionToken,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';
import type { Page } from '@playwright/test';

// Холодная загрузка хаба объектов (карта #1437, план 044; аменда #1605
// — однофлушный вход, react/react#37014): страницы с серверным
// префетчем не оборачиваются в Suspense, скелетон на холодном входе не
// стримится — первый кадр несёт данные и хром из реестра маршрутов.
// Страница объекта не должна получать чужой архетип списка (сегментный
// loading.tsx снесён, дети стримят свои in-page fallback'и).
//
// Механика честного замера: фаза загрузки коротка и ретраить её нельзя —
// MutationObserver с init-скрипта ведёт журнал всего времени, пока
// скелетон-маркер присутствует в DOM (одна мутация = один снимок
// пост-коммитного DOM, гонки фаз нет). Скрытый свап-дубль (#698,
// body > div[hidden]) не считается: прямоугольники скрытого дерева
// нулевые, гард «маркер прикреплён» отсекает записи после резолва.

type ColdLog = {
  seen: boolean;
  pillSeen: boolean;
  minCardX: number | null;
  listSkeletonSeen: boolean;
};

/** Троттлинг сети + отключение кеша: фазы стриминга растягиваются до
 * наблюдаемых, кадр честно холодный. */
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
  const log: ColdLog = {
    seen: false,
    pillSeen: false,
    minCardX: null,
    listSkeletonSeen: false,
  };
  (window as unknown as { __coldLog: ColdLog }).__coldLog = log;
  const observer = new MutationObserver(() => {
    const loading = document.querySelector('[data-testid="properties-loading"]');
    if (loading !== null) {
      log.seen = true;
      const pill = document.querySelector('[data-testid="properties-search-pill"]');
      if (pill !== null && pill.getBoundingClientRect().width > 0) {
        log.pillSeen = true;
      }
      const card = loading.querySelector(':scope > div');
      if (card !== null) {
        // Только отстиленный DOM: до загрузки CSS карточка лежит в потоке
        // с дефолтным body margin 8px — это состояние браузер не красит
        // (CSS render-blocking), в замер геометрии оно не годится.
        const styled = getComputedStyle(card).borderRadius !== '0px';
        if (styled) {
          const x = card.getBoundingClientRect().x;
          if (log.minCardX === null || x < log.minCardX) {
            log.minCardX = x;
          }
        }
      }
    }
    if (document.querySelector('[data-testid="properties-loading"]') !== null) {
      log.listSkeletonSeen = true;
    }
  });
  observer.observe(document, { childList: true, subtree: true });
}

async function readLog(page: Page): Promise<ColdLog> {
  return page.evaluate(() => (window as unknown as { __coldLog: ColdLog }).__coldLog);
}

test.describe('холодная загрузка хаба объектов (план 044, аменда #1605)', () => {
  test('хаб: холодный вход однофлушный — данные, пилюля и хром первого кадра, скелетона нет', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSessionToken(page, seededUser.sessionToken);
    await page.addInitScript(journalScript);
    await coldNetwork(page);

    await page.goto(`${process.env.E2E_BASE_URL ?? 'http://127.0.0.1:3010'}/properties`, {
      waitUntil: 'commit',
    });
    await page.waitForLoadState('load');
    const log = await readLog(page);

    // Канон однофлушного входа (аменда #1605, react/react#37014):
    // скелетон на холодном входе не стримится вовсе — первый кадр и есть
    // кадр с данными.
    expect(log.seen, 'скелетон списка на холодном входе не стримится').toBe(false);
    // Данные в первом кадре: сид-объекты рендерятся сразу (граница ждала
    // данные до рендера).
    await expect(page.getByText('Квартира на Ленина').first()).toBeVisible();
    // Хром первого кадра: пилюля поиска хаба — из реестра маршрутов.
    await expect(page.getByRole('link', { name: 'Найти объект' })).toBeVisible();
  });

  test('объект: чужой скелетон списка не стримится', async ({ page, seededUser }) => {
    await openCabinetWithSessionToken(page, seededUser.sessionToken);
    await page.addInitScript(journalScript);
    await coldNetwork(page);

    await page.goto(
      `${process.env.E2E_BASE_URL ?? 'http://127.0.0.1:3010'}/properties/${SEEDED_APARTMENT_PROPERTY_ID}`,
      { waitUntil: 'commit' },
    );
    await page.waitForLoadState('load');
    // Независимый гард «страница доехала»: секция детали видна — прогон не
    // пустой (скелетон детали отдельным кадром может не стримиться: серверный
    // префетч резолвится раньше params, React шлёт fallback и контент одним
    // флешем — проверяем не его, а отсутствие ЧУЖОГО архетипа).
    await expect(page.getByText('Регулярные платежи')).toBeVisible();
    const log = await readLog(page);

    // Чужой архетип: скелетон списка не появлялся ни разу.
    expect(log.listSkeletonSeen, 'скелетон списка на странице объекта').toBe(false);
  });
});
