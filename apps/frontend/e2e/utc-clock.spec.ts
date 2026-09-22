import { expect, test } from './fixtures';

// Инвариант #796: весь прогон живёт в UTC. Сид считает «сегодня/вчера» через
// CURRENT_DATE в postgres-контейнере Etc/UTC; playwright.config.ts закрепляет
// ту же таймзону у браузера (use.timezoneId) и у node-воркеров спеков
// (process.env.TZ). Без этого после 00:00 МСК локальная дата браузера
// обгоняет сид на сутки до 03:00 МСК — «сегодняшняя» операция рендерится
// группой «Вчера», и ночные прогоны краснеют на getByText('Сегодня').

test('часы прогона в UTC: воркер и браузер согласованы с сидом', async ({ page }) => {
  // Node-сторона: спеки вычисляют ожидаемые дни/месяцы через new Date().
  expect(Intl.DateTimeFormat().resolvedOptions().timeZone).toBe('UTC');
  expect(new Date().getTimezoneOffset()).toBe(0);

  // Браузерная сторона: фронт группирует операции в «Сегодня»/«Вчера»
  // по локальной дате браузера.
  await page.goto('/login');
  const browserTz = await page.evaluate(
    () => Intl.DateTimeFormat().resolvedOptions().timeZone,
  );
  expect(browserTz).toBe('UTC');
});
