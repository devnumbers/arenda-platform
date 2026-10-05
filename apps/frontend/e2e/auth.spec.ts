import {
  BASE_URL,
  execE2eSql,
  expect,
  extractLoginCode,
  loginViaUi,
  openCabinetWithSessionToken,
  seededSessionTokenHash,
  test,
} from './fixtures';

// Маркер живого документа (#1098): SPA-переход (router.push/replace) его
// сохранил бы, полная перезагрузка обязана смыть.
const MARKER_KEY = 't1098Marker';

async function plantDocumentMarker(page: Parameters<typeof loginViaUi>[0]): Promise<void> {
  await page.evaluate((key) => {
    (window as unknown as Record<string, unknown>)[key] = true;
  }, MARKER_KEY);
}

async function readDocumentMarker(page: Parameters<typeof loginViaUi>[0]): Promise<unknown> {
  return page.evaluate(
    (key) => (window as unknown as Record<string, unknown>)[key] ?? null,
    MARKER_KEY,
  );
}

// Smoke of the real login flow (ticket #456): the seeded owner enters the
// phone, the code arrives in the backend log (fake email sender), and the
// cabinet opens on the properties list. Validates the whole chain — UI
// form, /api proxy, auth endpoints, session cookie, middleware redirect —
// before payment screens start relying on this infrastructure.
//
// Жёсткий вход (#1098): verify обязан завершаться полной перезагрузкой на
// цель, а не router.push — клиентская навигация оставляла бы гостевые
// RSC-остатки и react-query предыдущего документа в JS-heap той же вкладки.
// Маркер живого документа ставится до кода: SPA-переход его сохранил бы,
// новый документ обязан смыть.
test('вход по коду из письма ведёт в кабинет', async ({ page, seededUser }) => {
  await loginViaUi(page, seededUser, () => plantDocumentMarker(page));

  const marker = await readDocumentMarker(page);
  expect(marker).toBeNull();
});

// The redesigned code step (#765): a wrong code shows the mockup's inline
// «Неверный код» in the field (no toast), the field's × resets value and
// error, and the correct code then logs in through the same autosubmit.
test('неверный код — инлайн в поле, крестик сбрасывает', async ({ page, seededUser }) => {
  await page.goto('/login');
  await page.getByRole('textbox', { name: 'Телефон' }).fill(seededUser.phoneDigits);
  await page.getByRole('button', { name: 'Войти' }).click();
  await expect(page.getByRole('heading', { name: 'Введите код' })).toBeVisible();

  await page.getByRole('textbox', { name: 'Код' }).fill('000000');
  await expect(page.getByText('Неверный код')).toBeVisible();

  await page.getByRole('button', { name: 'Очистить поле' }).click();
  await expect(page.getByText('Неверный код')).toBeHidden();
  await expect(page.getByRole('textbox', { name: 'Код' })).toHaveValue('');

  const code = await extractLoginCode(seededUser);
  await page.getByRole('textbox', { name: 'Код' }).fill(code);
  await page.waitForURL('**/properties');
});

// The ← in the top bar returns to the previous step: the seeded user logged
// in by phone only (no email in the draft), so back means the phone step.
test('стрелка назад ведёт на предыдущий шаг', async ({ page, seededUser }) => {
  await page.goto('/login');
  await page.getByRole('textbox', { name: 'Телефон' }).fill(seededUser.phoneDigits);
  await page.getByRole('button', { name: 'Войти' }).click();
  await expect(page.getByRole('heading', { name: 'Введите код' })).toBeVisible();

  // Resend-канон #733: the tile is disabled with the countdown caption while
  // the 60 s cooldown from retryAfter runs. The mockup has no length counter
  // under the field (walking back a native-maxLength regression, #765).
  await expect(page.getByRole('button', { name: 'Отправить новый код' })).toBeDisabled();
  await expect(page.getByText(/Запросить новый код можно через/)).toBeVisible();
  await expect(page.getByText(/\d\/6/)).toHaveCount(0);

  await page.getByRole('button', { name: 'Назад' }).click();
  await expect(page.getByRole('heading', { name: 'Введите номер телефона' })).toBeVisible();
});

// Планшетная карточка входа — 600 по макету (#1100, Figma 2343:67107):
// при 768 карточка встаёт с полями по 84 (проверено по кадру пиксельным
// замером), бокс поля — 84 + padding 32 = 116, инпут внутри бокса —
// +18 (pl) = 134, ширина инпута — 536 (600 − паддинги 32×2) − pr 8 =
// 510. До фикса карточка была 720 (только 24px внешнего паддинга) —
// инпут стоял на 74 шириной 630.
test('планшетная карточка входа — 600 по макету (#1100)', async ({page}) => {
  await page.setViewportSize({width: 768, height: 1024});
  await page.goto('/login');
  const field = page.getByRole('textbox', {name: 'Телефон'});
  await expect(field).toBeVisible();
  const box = await field.boundingBox();
  expect(box).not.toBeNull();
  expect(box?.x ?? 0).toBeGreaterThanOrEqual(133);
  expect(box?.x ?? 0).toBeLessThanOrEqual(135);
  expect(box?.width ?? 0).toBeGreaterThanOrEqual(509);
  expect(box?.width ?? 0).toBeLessThanOrEqual(511);
});

// Двойной клик «Войти» (#1099): до фикса второй клик, пришедший раньше
// перерисовки (реальный мир — джанк главного потока: input-событие приоритетнее
// рендер-таска react-query), уходил вторым POST /auth/send и ловил 429
// троттлинга бэка (minSendInterval 60с) — тост «Не удалось войти» рисовался
// поверх уже сменившегося шага. Воспроизведение детерминирует худший случай:
// два клика одним JS-таском, перерисовка между ними невозможна в принципе.
// Гард отправки обязан держать ровно один запрос и вести на следующий шаг:
// код для существующего номера, почта — для нового. /auth/send перехвачен
// route-моком (канон phone-change.spec): ответы детерминированы, бюджет
// burst 3 бэка не тратится.
test('двойной клик «Войти» шлёт один код и ведёт на шаг кода (#1099)', async ({page}) => {
  let sendCalls = 0;
  await page.route('**/api/auth/send', async (route) => {
    sendCalls += 1;
    if (sendCalls === 1) {
      // Задержанный ответ: окно, в котором кнопка обязана быть disabled
      // на время отправки (решение владельца #1099) — без этого второй
      // клик реального пользователя уходит вторым POST-ом.
      await new Promise((resolve) => setTimeout(resolve, 1500));
      return route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({sent: true, retryAfter: 60}),
      });
    }
    // Второй запрос — 429 троттлинга, как у бэка: до фикса он рисовал
    // тост ошибки поверх шага кода и вешал лишний кулдаун.
    return route.fulfill({
      status: 429,
      contentType: 'application/problem+json',
      headers: {'Retry-After': '60'},
      body: JSON.stringify({code: 'code_sent_too_recently', detail: 'Код уже отправлен'}),
    });
  });

  await page.goto('/login');
  await page.getByRole('textbox', {name: 'Телефон'}).fill('9261112233');
  const loginButton = page.getByRole('button', {name: 'Войти'});
  await loginButton
    .evaluate((button: HTMLElement) => {
      button.click();
      button.click();
    });
  // Ответ ещё в полёте — кнопка погашена на время отправки.
  await expect(loginButton).toBeDisabled();

  await expect(page.getByRole('heading', {name: 'Введите код'})).toBeVisible();
  expect(sendCalls).toBe(1);
  // Тост ошибки поверх ушедшего шага — регрессия двойной отправки (#1099).
  await expect(page.getByText('Не удалось войти')).toHaveCount(0);
});

// Та же гонка на ветке нового номера (sent:false): двойной клик ведёт на
// шаг почты ровно одним запросом.
test('двойной клик «Войти» с новым номером ведёт на шаг почты (#1099)', async ({page}) => {
  let sendCalls = 0;
  await page.route('**/api/auth/send', (route) => {
    sendCalls += 1;
    return route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({sent: false}),
    });
  });

  await page.goto('/login');
  await page.getByRole('textbox', {name: 'Телефон'}).fill('9261112233');
  await page
    .getByRole('button', {name: 'Войти'})
    .evaluate((button: HTMLElement) => {
      button.click();
      button.click();
    });

  await expect(page.getByRole('heading', {name: 'Введите почту'})).toBeVisible();
  expect(sendCalls).toBe(1);
});

// Жёсткий выход (#1098): кабинет уходит полной перезагрузкой на /login (не
// router.push), и «Назад»/«Вперёд» не возвращают ЛК — без сессии proxy.ts
// отвечает редиректом на /login?from=…, а возврат из bfcache перехватывает
// pageshow-гард той же перезагрузкой.
//
// Лог-out угасает сессии, посеянные прямо в БД (seededSessionTokenHash):
// сид-сессия E2E_SESSION_TOKEN нужна параллельным воркерам (канон
// profile-tree), а бюджет /auth/send (burst 3 на телефон) не тратится вовсе.
const LOGOUT_SESSION_TOKEN = 'e2e-t1098-logout-session-token';
const LOGOUT_SESSION_ID = '22222222-2222-4222-8222-222222222298';
const HEADER_PROBE_SESSION_TOKEN = 'e2e-t1098-header-probe-session-token';
const HEADER_PROBE_SESSION_ID = '22222222-2222-4222-8222-222222222299';
const SEEDED_OWNER_USER_ID = '11111111-1111-4111-8111-111111111111';

// Заголовок Clear-Site-Data проверяется двумя независимыми пробами:
//  1. node-сторона (request-фикстура, мимо браузерного стека): POST /api/auth/logout
//     через прокси несёт "cache" — контракт бекенда и транзита BFF;
//  2. браузер: Chrome самоотчитывается в консоль («Cleared data types:
//     "cache"») — сам заголовок он из ответа снимает после обработки, поэтому
//     браузерный headers() его не показывает (находка отладки #1098).
test('выход: Clear-Site-Data, перезагрузка на /login, «Назад» не возвращает ЛК (#1098)', async ({
  page,
  request,
}) => {
  await execE2eSql(`
    INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at, last_used_at)
    VALUES
      ('${LOGOUT_SESSION_ID}', '${SEEDED_OWNER_USER_ID}',
       '${seededSessionTokenHash(LOGOUT_SESSION_TOKEN)}',
       now() + interval '7 days', now(), now()),
      ('${HEADER_PROBE_SESSION_ID}', '${SEEDED_OWNER_USER_ID}',
       '${seededSessionTokenHash(HEADER_PROBE_SESSION_TOKEN)}',
       now() + interval '7 days', now(), now())
    ON CONFLICT (id) DO UPDATE
    SET token_hash = EXCLUDED.token_hash, expires_at = EXCLUDED.expires_at
  `);

  // Проба 1: заголовок на уровне контракта — бекенд и прокси (без браузера).
  const probe = await request.post(`${BASE_URL}/api/auth/logout`, {
    headers: { cookie: `session_id=${HEADER_PROBE_SESSION_TOKEN}` },
  });
  expect(probe.status()).toBe(204);
  expect(probe.headers()['clear-site-data']).toContain('"cache"');

  // Проба 2: браузерная — жёсткая навигация + подтверждение Chrome в консоли.
  const clearSiteDataConfirmations: string[] = [];
  page.on('console', (message) => {
    if (message.type() === 'info' && /Clear-Site-Data/.test(message.text())) {
      clearSiteDataConfirmations.push(message.text());
    }
  });

  await openCabinetWithSessionToken(page, LOGOUT_SESSION_TOKEN);
  // Первым экраном /properties: после замены /profile → /login у истории
  // должен остаться кабинет позади — goBack внизу возвращается именно в него.
  await page.goto('/properties');
  await page.goto('/profile');
  await plantDocumentMarker(page);

  const logoutResponsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === 'POST' && response.url().includes('/api/auth/logout'),
  );
  await page.getByRole('button', { name: 'Выйти' }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('button', { name: 'Выйти' }).click();

  await logoutResponsePromise;
  // Консольное подтверждение Chrome может прийти на тик позже ответа —
  // ожидание с ретраем.
  await expect
    .poll(
      () => clearSiteDataConfirmations.some((text) => text.includes('"cache"')),
      {
        message: `Chrome did not confirm Clear-Site-Data "cache"; got: ${JSON.stringify(clearSiteDataConfirmations)}`,
      },
    )
    .toBe(true);

  await page.waitForURL('**/login');
  await expect(page.getByRole('heading', { name: 'Введите номер телефона' })).toBeVisible();
  // Полная перезагрузка: метка живого документа смылась.
  expect(await readDocumentMarker(page)).toBeNull();

  // SQL-правда: сессия удалена на беке.
  const remaining = await execE2eSql(
    `SELECT count(*) FROM sessions WHERE id = '${LOGOUT_SESSION_ID}'`,
  );
  expect(remaining).toBe('0');

  // «Назад» из /login не показывает кабинет: свежий документ /properties без
  // сессии редиректится на /login?from=…; восстановление из bfcache гасится
  // гардом той же перезагрузкой.
  await page.goBack();
  await page.waitForURL(/\/login/);
  await expect(page.getByRole('heading', { name: 'Введите номер телефона' })).toBeVisible();

  // «Вперёд» — тоже не кабинет: история ведёт снова на /login (замена
  // /profile → /login), никакого содержимого ЛК.
  await page.goForward();
  await page.waitForURL(/\/login/);
  await expect(page.getByRole('heading', { name: 'Введите номер телефона' })).toBeVisible();
});
