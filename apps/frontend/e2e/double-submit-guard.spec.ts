import type { Locator, Page } from '@playwright/test';
import {
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';

// Гард двойного сабмита (Т2 #1120, карта #1112). Окно до перерисовки —
// два клика одним JS-таском — закрыт синхронным слотом useGuardedMutation;
// кнопка в полёте глушится корневым фиксом #1119 (disabled === true ||
// loading). Представители первой волны ресерча #1113: визард платежа
// (худшая комбинация: кнопка жива весь полёт + бекенд без дедупа),
// операция глобального входа, создание задачи. Счётчик фаеров —
// page.on('request'), не route (грабля #1113: route('**/*') валит
// вкладку); полёт удлиняется CDP-latency (грабля #1113: route-задержка
// ненадёжна), чтобы disable-состояние кнопки было ловимо. SQL-правда:
// ровно одна строка на каждый прогон.

const APARTMENT_PAYMENTS_URL = `/properties/${SEEDED_APARTMENT_PROPERTY_ID}/payments`;
const PROPERTY_BUTTON = 'Квартира на Ленина';

function trackPosts(page: Page, urlPart: string): string[] {
  const urls: string[] = [];
  page.on('request', (request) => {
    if (request.method() === 'POST' && request.url().includes(urlPart)) {
      urls.push(request.url());
    }
  });
  return urls;
}

/** CDP-задержка сети: полёт мутации удлиняется — disable-кнопка в полёте
 * наблюдаема (rAF-поллинг Playwright ловит состояние). Возвращает
 * функцию снятия эмуляции. */
async function holdLatency(page: Page, latencyMs: number): Promise<() => Promise<void>> {
  const cdp = await page.context().newCDPSession(page);
  await cdp.send('Network.enable');
  await cdp.send('Network.emulateNetworkConditions', {
    offline: false,
    latency: latencyMs,
    downloadThroughput: -1,
    uploadThroughput: -1,
  });
  return async () => {
    await cdp.send('Network.emulateNetworkConditions', {
      offline: false,
      latency: 0,
      downloadThroughput: -1,
      uploadThroughput: -1,
    });
  };
}

async function sqlCount(table: string, title: string): Promise<number> {
  const raw = await execE2eSql(`SELECT count(*) FROM ${table} WHERE title = '${title}'`);
  return Number(raw);
}

/** Два клика одной кнопкой в одном JS-таске — воспроизведение окна до
 * перерисовки из проб #1113 (джанк главного потока). */
async function doubleClickInOneTask(button: Locator): Promise<void> {
  await button.evaluate((element) => {
    const el = element as HTMLElement;
    el.click();
    el.click();
  });
}

test.describe('гард двойного сабмита (#1120)', () => {
  test('визард платежа: одиночный клик гасит кнопку, двойной одним таском — ровно один POST', async ({
    page,
    seededUser,
  }) => {
    test.setTimeout(120_000);
    await openCabinetWithSeededSession(page, seededUser);
    const posts = trackPosts(page, '/payments');

    // --- Прогон 1: одиночный клик — кнопка гаснет в полёте (#1119).
    const title1 = `Гард-платёж А ${Date.now()}`;
    await page.goto(`${APARTMENT_PAYMENTS_URL}/new?type=payment`);
    const rows = page.locator('div.flex.flex-col.pt-6').getByRole('button');
    await expect(rows.last()).toHaveAccessibleName('Другое');
    await rows.last().click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('textbox').fill(title1);
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Каждый месяц' }).click();
    await page.getByRole('button', { name: '10', exact: true }).first().click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('1500');
    await page.getByRole('radio', { name: 'Расход' }).click();

    const release1 = await holdLatency(page, 600);
    const submit = page.getByRole('button', { name: 'Создать платеж' });
    await submit.click();
    await expect(submit).toBeDisabled();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(
      `«${title1}»`,
    );
    await release1();
    expect(await sqlCount('payments', title1)).toBe(1);

    // --- Прогон 2: два клика одним JS-таском — окно до перерисовки;
    // слот гарда отбрасывает второй фаер до запроса.
    const title2 = `Гард-платёж Б ${Date.now()}`;
    await page.goto(`${APARTMENT_PAYMENTS_URL}/new?type=payment`);
    await expect(rows.last()).toHaveAccessibleName('Другое');
    await rows.last().click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('textbox').fill(title2);
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Каждый месяц' }).click();
    await page.getByRole('button', { name: '10', exact: true }).first().click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Далее' }).click();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('1500');
    await page.getByRole('radio', { name: 'Расход' }).click();

    const release2 = await holdLatency(page, 600);
    await doubleClickInOneTask(page.getByRole('button', { name: 'Создать платеж' }));
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeDisabled();
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(
      `«${title2}»`,
    );
    await release2();
    expect(posts).toHaveLength(2); // 1 из прогона 1 + 1 из пары (второй дропнут)
    expect(await sqlCount('payments', title2)).toBe(1);
  });

  test('операция, глобальный вход: двойной клик одним таском — ровно один POST', async ({
    page,
    seededUser,
  }) => {
    test.setTimeout(120_000);
    await openCabinetWithSeededSession(page, seededUser);
    const posts = trackPosts(page, '/operations');
    const title = `Гард-операция ${Date.now()}`;

    await page.goto('/operations/new');
    await page.getByRole('textbox', { name: 'Сумма' }).fill('600');
    await page.getByRole('radio', { name: 'Расход' }).click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('textbox').fill(title);
    await page.getByRole('button', { name: 'Продолжить' }).click();
    const rows = page.locator('div.flex.flex-col.pt-6').getByRole('button');
    await expect(rows.last()).toHaveAccessibleName('Другое');
    await rows.last().click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    // Строки объектов шага 4 — радио-карточки «Row Button» (#525).
    await page.getByRole('radio', { name: PROPERTY_BUTTON }).click();

    const release = await holdLatency(page, 600);
    const submit = page.getByRole('button', { name: 'Добавить операцию' });
    await doubleClickInOneTask(submit);
    await expect(submit).toBeDisabled();
    await expect(page.getByRole('heading', { name: title })).toBeVisible();
    await release();
    expect(posts).toHaveLength(1);
    expect(await sqlCount('operations', title)).toBe(1);
  });

  test('создание задачи: двойной клик одним таском — ровно один POST', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    const posts = trackPosts(page, '/tasks/rules');
    const title = `Гард-задача ${Date.now()}`;

    // Вход со списка задач (клик «Создать задачу»), не прямой goto:
    // success закрывается goBack-ом — без истории он уезжает в
    // about:blank (грабля карты #1068, MCP-таб/прямой goto).
    await page.goto('/tasks');
    await page.getByRole('button', { name: 'Создать задачу' }).last().click();
    await page.waitForURL('**/tasks/new');
    await page.getByRole('textbox', { name: 'Задача' }).fill(title);
    await page.getByRole('button', { name: 'Далее' }).click();

    const release = await holdLatency(page, 600);
    const submit = page.getByRole('button', { name: 'Создать', exact: true });
    await doubleClickInOneTask(submit);
    await expect(submit).toBeDisabled();
    await expect(page).toHaveURL(/\/tasks$/);
    await release();
    expect(posts).toHaveLength(1);
    expect(await sqlCount('task_rules', title)).toBe(1);
  });
});
