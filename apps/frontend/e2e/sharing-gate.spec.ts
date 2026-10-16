import {
  captureScreen,
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  screenHeader,
  test,
} from './fixtures';

// Интерстишал «Совместный доступ недоступен» (#1470, макет 3518-80047):
// промежуточный экран тарифного гейта шаринга — шапка «Повысить тариф»
// (по макету), статичный текст (в тикете: гейты ведут сюда только
// владельца на basic), «Выбрать тариф» → /profile/tariff/change,
// «Отменить» → назад (history-first, фолбэк — хаб участников).
//
// Развязка входов — в соседних спеках: proxy-ретаргет ленты истории —
// history-basic-gate.spec, диплинки страницы участников и разводка
// клика «Совместного доступа» — property-participants.spec, апселл-CTA
// хаба — participants-list.spec.

// Перевод владельца на basic — UPDATE подписки, не удаление («подписки
// нет» фронт читает как optimistic-платный); прежний тариф
// восстанавливаем честно, как в participants-list.spec.
const OWNER_SUBSCRIPTION_ID = '99999999-9999-4999-8999-999999999901';

function flipOwnerTariffSql(to: string): string {
  return (
    `UPDATE user_subscriptions SET tariff_id = (SELECT id FROM tariffs WHERE name = '${to}') ` +
    `WHERE id = '${OWNER_SUBSCRIPTION_ID}'`
  );
}

test('диплинк: каркас целиком — шапка «Повысить тариф», заголовок, статичный текст, иллюстрация', async ({ page, seededUser }, testInfo) => {
  const previousTariff = await execE2eSql(
    `SELECT t.name FROM user_subscriptions s JOIN tariffs t ON t.id = s.tariff_id WHERE s.id = '${OWNER_SUBSCRIPTION_ID}'`,
  );
  await execE2eSql(flipOwnerTariffSql('basic'));
  try {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/tariff/sharing');

    await expect(screenHeader(page).getByText('Повысить тариф')).toBeVisible();
    await expect(page.getByRole('heading', { name: 'Совместный доступ недоступен' })).toBeVisible();
    await expect(
      page.getByText('Повысьте тариф до «Про» или «Бизнес», чтобы пользоваться совместным доступом'),
    ).toBeVisible();
    // Иллюстрация — экспорт композиции 3518:80115 (#1469-форма).
    await expect(page.locator('img[src*="tariff-sharing"]')).toBeVisible();

    await captureScreen(page, testInfo, 'sharing-interstitial');
  } finally {
    await execE2eSql(flipOwnerTariffSql(previousTariff));
  }
});

test('интерстишал: «Выбрать тариф» ведёт на смену тарифа', async ({ page, seededUser }) => {
  const previousTariff = await execE2eSql(
    `SELECT t.name FROM user_subscriptions s JOIN tariffs t ON t.id = s.tariff_id WHERE s.id = '${OWNER_SUBSCRIPTION_ID}'`,
  );
  await execE2eSql(flipOwnerTariffSql('basic'));
  try {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/tariff/sharing');

    await page.getByRole('button', { name: 'Выбрать тариф' }).click();
    await expect(page).toHaveURL(/\/profile\/tariff\/change$/);
  } finally {
    await execE2eSql(flipOwnerTariffSql(previousTariff));
  }
});

test('интерстишал: «Отменить» возвращает назад на хаб участников', async ({ page, seededUser }) => {
  const previousTariff = await execE2eSql(
    `SELECT t.name FROM user_subscriptions s JOIN tariffs t ON t.id = s.tariff_id WHERE s.id = '${OWNER_SUBSCRIPTION_ID}'`,
  );
  await execE2eSql(flipOwnerTariffSql('basic'));
  try {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/participants');
    await page.goto('/profile/tariff/sharing');

    await page.getByRole('button', { name: 'Отменить' }).click();
    await expect(page).toHaveURL(/\/participants$/);
  } finally {
    await execE2eSql(flipOwnerTariffSql(previousTariff));
  }
});
