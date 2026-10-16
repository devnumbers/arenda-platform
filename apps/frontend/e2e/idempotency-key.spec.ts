import {
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';

// Идемпотентные ключи creations (Т3 #1121, карта #1112): клиент шлёт
// Idempotency-Key на создание правила платежа; повтор того же POST с тем
// же ключом возвращает сохранённый результат первой попытки — вторая
// строка в БД не появляется (паттерн Stripe, последняя линия защиты).
// SQL-правда против живого бекенда.

const APARTMENT_PAYMENTS_URL = `/properties/${SEEDED_APARTMENT_PROPERTY_ID}/payments`;

test('повтор создания платежа с тем же Idempotency-Key — тот же ответ, одна строка', async ({
  page,
  seededUser,
}) => {
  test.setTimeout(120_000);
  await openCabinetWithSeededSession(page, seededUser);

  const captured: { key: string; path: string; body: string }[] = [];
  page.on('request', (request) => {
    if (request.method() === 'POST' && request.url().includes('/payments')) {
      captured.push({
        key: request.headers()['idempotency-key'] ?? '',
        path: new URL(request.url()).pathname,
        body: request.postData() ?? '',
      });
    }
  });

  const title = `Идемпотентность ${Date.now()}`;
  await page.goto(`${APARTMENT_PAYMENTS_URL}/new?type=payment`);
  const rows = page.locator('div.flex.flex-col.pt-6').getByRole('button');
  await expect(rows.last()).toHaveAccessibleName('Другое');
  await rows.last().click();
  await page.getByRole('button', { name: 'Продолжить' }).click();
  await page.getByRole('textbox').fill(title);
  await page.getByRole('button', { name: 'Продолжить' }).click();
  await page.getByRole('button', { name: 'Каждый месяц' }).click();
  await page.getByRole('button', { name: '10', exact: true }).first().click();
  await page.getByRole('button', { name: 'Продолжить' }).click();
  await page.getByRole('button', { name: 'Далее' }).click();
  await page.getByRole('textbox', { name: 'Сумма' }).fill('1500');
  await page.getByRole('radio', { name: 'Расход' }).click();
  await page.getByRole('button', { name: 'Создать платеж' }).click();
  await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toContainText(
    `«${title}»`,
  );

  // Контракт: клиент сгенерировал ключ на попытку.
  expect(captured).toHaveLength(1);
  const original = captured[0];
  if (!original) {
    throw new Error('ключ создания не перехвачен');
  }
  expect(original.key).not.toBe('');

  const countBefore = Number(
    await execE2eSql(`SELECT count(*) FROM payments WHERE title = '${title}'`),
  );
  expect(countBefore).toBe(1);

  // Реплей: точный повтор POST (тот же путь, тело, ключ) — как сетевой
  // ретрай или перезагрузка с тем же ключом.
  const replay = await page.evaluate(async ({ path, key, body }) => {
    const response = await fetch(path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'Idempotency-Key': key },
      body,
    });
    return { status: response.status, text: await response.text() };
  }, original);

  expect(replay.status).toBe(201);
  expect(replay.text).toContain('Идемпотентность');

  const countAfter = Number(
    await execE2eSql(`SELECT count(*) FROM payments WHERE title = '${title}'`),
  );
  expect(countAfter).toBe(1);
});
