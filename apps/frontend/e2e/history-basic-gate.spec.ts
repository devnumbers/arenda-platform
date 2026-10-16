import {
  expect,
  execE2eSql,
  memberTaskEntry,
  openCabinetWithSeededSession,
  openCabinetWithSessionToken,
  paymentCreatedEntry,
  seededMemberSessionToken,
  SEEDED_APARTMENT_PROPERTY_ID,
  seedJournalEntry,
  screenHeader,
  test,
  type SeededUser,
} from './fixtures';

// Разводка «Истории» на базовом тарифе (карта #1459, тикет #1466,
// ревизия ADR 0064): proxy-паттерн раздела сужен до ленты — объектные
// поверхности /history/properties/{id} и
// /history/participants/{p}/properties/{id} выпускают basic до
// отрисовки, роль на объекте разводит экран (HistoryObjectGate, шов
// sharingGateRedirect): участник объекта читает на любом тарифе
// («пригласили — значит пользуется», бекенд-гейт роль-осведомлённый
// #1465), владелец на basic идёт на интерстишал шаринга #1470. Лента
// /history платная для всех basic — вход ей не нужен.

const APARTMENT_ID = SEEDED_APARTMENT_PROPERTY_ID;

// Сида роли #467: подписки владельца квартиры и участницы Марии
// (full_access); прежний тариф запоминаем — восстановление честное,
// не литерал.
const OWNER_SUBSCRIPTION_ID = '99999999-9999-4999-8999-999999999901';
const MARIA_SUBSCRIPTION_ID = '99999999-9999-4999-8999-999999999902';

/** uuid юзера сидовой участницы (participantId пары, actor_id журнала). */
const MEMBER_UUID_SQL = `SELECT id FROM users WHERE email = 'e2e-member@example.com'`;

/** Две записи квартиры — владельца и участницы Марии: участник на basic
 * читает суженную область объекта участия целиком, обеих актёров. */
async function seedApartmentJournal(user: SeededUser): Promise<void> {
  await execE2eSql('DELETE FROM action_journal;');
  await seedJournalEntry(
    paymentCreatedEntry('f0000000-0000-4000-8000-000000000001', 5, 'Аренда за сентябрь'),
    user,
  );
  await seedJournalEntry(memberTaskEntry('f0000000-0000-4000-8000-000000000002', 3), user);
}

test('участник на basic читает обе объектные поверхности истории, лента ведёт на выбор тарифа; сид восстанавливается', async ({ page, seededUser }) => {
  // Мария (full_access) на basic: proxy объектные маршруты больше не
  // держит, бекенд-гейт суженную область её участия пропускает (#1465).
  const previousTariff = await execE2eSql(
    `SELECT t.name FROM user_subscriptions s JOIN tariffs t ON t.id = s.tariff_id WHERE s.id = '${MARIA_SUBSCRIPTION_ID}'`,
  );
  await execE2eSql(
    `UPDATE user_subscriptions SET tariff_id = (SELECT id FROM tariffs WHERE name = 'basic') ` +
      `WHERE id = '${MARIA_SUBSCRIPTION_ID}'`,
  );
  try {
    await seedApartmentJournal(seededUser);
    const memberUuid = await execE2eSql(MEMBER_UUID_SQL);
    await openCabinetWithSessionToken(page, seededMemberSessionToken());

    // «История объекта» из кебаба «Участников объекта» — жива: страница
    // открывается и остаётся на месте, объектная лента читается целиком
    // (обеих актёров, сужение по объекту, не по актёру).
    await page.goto(`/history/properties/${APARTMENT_ID}`);
    await expect(page).toHaveURL(new RegExp(`/history/properties/${APARTMENT_ID}$`));
    await expect(page.getByText('История объекта').first()).toBeVisible();
    await expect(screenHeader(page).getByText('Квартира на Ленина')).toBeVisible();
    await expect(page.getByText('Платёж создан:')).toBeVisible();
    await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();

    // Пара «участник + объект» со страницы «Права участника» — жива:
    // та же участница читает свои действия в объекте; запись владельца
    // (чужой актёр) прибитую пару не покидает.
    await page.goto(`/history/participants/${memberUuid}/properties/${APARTMENT_ID}`);
    await expect(page).toHaveURL(
      new RegExp(`/history/participants/${memberUuid}/properties/${APARTMENT_ID}$`),
    );
    await expect(page.getByText('Действия участника в объекте').first()).toBeVisible();
    await expect(screenHeader(page).getByText('Квартира на Ленина')).toBeVisible();
    await expect(page.getByText('Задача выполнена: Заменить кран')).toBeVisible();
    await expect(page.getByText('Платёж создан:')).toHaveCount(0);

    // Лента /history — по-прежнему платная для всех basic, вход ей не
    // нужен: proxy ведёт на выбор тарифа (ретаргет на интерстишал —
    // тикет #1470).
    await page.goto('/history');
    await expect(page).toHaveURL(/\/profile\/tariff\/change$/);
    await expect(page.getByText('Выбрать тариф').first()).toBeVisible();
  } finally {
    await execE2eSql(
      `UPDATE user_subscriptions SET tariff_id = (SELECT id FROM tariffs WHERE name = '${previousTariff}') ` +
        `WHERE id = '${MARIA_SUBSCRIPTION_ID}'`,
    );
  }
});

test('владелец на basic — объектная история своего объекта разводит на интерстишал шаринга; сид восстанавливается', async ({ page, seededUser }) => {
  // Строгий вариант бекенд-гейта #1465: свой объект в истории не
  // значится ни в какой поверхности — 402. Экран разводит по роли из
  // property-запроса до запроса ленты (страховка после загрузки роли и
  // тарифа, как на странице участников #1463).
  const previousTariff = await execE2eSql(
    `SELECT t.name FROM user_subscriptions s JOIN tariffs t ON t.id = s.tariff_id WHERE s.id = '${OWNER_SUBSCRIPTION_ID}'`,
  );
  await execE2eSql(
    `UPDATE user_subscriptions SET tariff_id = (SELECT id FROM tariffs WHERE name = 'basic') ` +
      `WHERE id = '${OWNER_SUBSCRIPTION_ID}'`,
  );
  try {
    const memberUuid = await execE2eSql(MEMBER_UUID_SQL);
    await openCabinetWithSeededSession(page, seededUser);

    await page.goto(`/history/properties/${APARTMENT_ID}`);
    await expect(page).toHaveURL(/\/profile\/tariff\/sharing$/);

    // Пара на своём объекте — та же разводка: participantId роли
    // читателя не меняет.
    await page.goto(`/history/participants/${memberUuid}/properties/${APARTMENT_ID}`);
    await expect(page).toHaveURL(/\/profile\/tariff\/sharing$/);
  } finally {
    await execE2eSql(
      `UPDATE user_subscriptions SET tariff_id = (SELECT id FROM tariffs WHERE name = '${previousTariff}') ` +
        `WHERE id = '${OWNER_SUBSCRIPTION_ID}'`,
    );
  }
});
