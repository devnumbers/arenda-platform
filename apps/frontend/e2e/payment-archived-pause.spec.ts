import {
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';

// Автопауза платежей при архивации объекта (тикет #1342): архивация
// ставит системную паузу всем живым правилам объекта, возврат снимает
// только её. e2e ведёт мутацию по-настоящему — API-архивация сидовой
// квартиры под сессией владельца, затем возврат тем же маршрутом в
// finally: сид восстанавливается сам (мост закрывает свои автопаузы),
// workers=1 — соседи не читают объект в этот момент. Проверяется кнопка
// «Возобновить» страницы платежа: задизейблена с пояснением, пока объект
// в архиве, жива после возврата.

const PROPERTY = SEEDED_APARTMENT_PROPERTY_ID;
const RENT_PAYMENT_URL = `/properties/${PROPERTY}/payments/55555555-5555-4555-8555-555555555551`;

// Журналы — append-only: закрытие автопаузы мостом интервалы и строки не
// сносит, а соседние спеки читают «Историю платежа» и «Историю действий»
// сида. В finally сид возвращается дословно: автопаузы и их журнальные
// строки (обеих лент) сносятся SQL-ом — работники=1, до следующего файла
// сид снова исходный.
async function cleanupArchiveTrace(): Promise<void> {
  await execE2eSql(`
    DELETE FROM payment_change_log WHERE property_id = '${PROPERTY}' AND origin = 'auto_archive';
    DELETE FROM action_journal WHERE property_id = '${PROPERTY}' AND context->>'pause_origin' = 'auto_archive';
    DELETE FROM payment_pauses WHERE origin = 'auto_archive'
      AND payment_id IN (SELECT id FROM payments WHERE property_id = '${PROPERTY}');
  `);
}

test.describe('автопауза платежей при архивации #1342', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('архивация: «Возобновить» задизейблена с пояснением; возврат: кнопка жива', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);

    // Настоящая архивация: мост ставит системные паузы живым правилам
    // (арендная …551 в сиде активна, без открытой паузы).
    const archived = await page.request.post(`/api/properties/${PROPERTY}/archive`);
    expect(archived.ok()).toBeTruthy();

    try {
      // Страница платежа: «Возобновить» видна (пауза стоит) и задизейблена,
      // под рядом — пояснение с путём выхода.
      await page.goto(RENT_PAYMENT_URL);
      const resume = page.getByRole('button', { name: 'Возобновить' });
      await expect(resume).toBeVisible();
      await expect(resume).toBeDisabled();
      await expect(
        page.getByText('Платеж на паузе, пока объект в архиве', { exact: false }),
      ).toBeVisible();

    } finally {
      // Возврат из архива тем же маршрутом: мост закрывает только свои
      // автопаузы; журнальные строки догоняет SQL-очистка.
      const restored = await page.request.post(`/api/properties/${PROPERTY}/unarchive`);
      expect(restored.ok()).toBeTruthy();
      await cleanupArchiveTrace();
    }

    // После возврата пауза снята мостом: правило снова активно —
    // «На паузу» вместо «Возобновить», пояснения нет.
    await page.goto(RENT_PAYMENT_URL);
    await expect(page.getByRole('button', { name: 'На паузу' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Возобновить' })).toHaveCount(0);
    await expect(
      page.getByText('Платеж на паузе, пока объект в архиве', { exact: false }),
    ).toHaveCount(0);
  });
});
