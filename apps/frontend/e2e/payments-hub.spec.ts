import {
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  openCabinetWithSessionToken,
  seededViewerSessionToken,
  test,
} from './fixtures';

// Хаб «Платежи» — первый блок по макету (3226-75059, хедер 3229-94522,
// тикет #1235): «+» создания платежа в ряду заголовка и в компакт-баре;
// цель «+» — paymentHubAddTarget (единственный редактируемый объект — шит
// единого входа, несколько — страница «Объекты»); у зрителя «+» нет (#703).
// Состав секций и пилюля — наследие #578 (ширины — column-width.spec.ts).

test.describe('хаб «Платежи» — «+» создания', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('владелец с несколькими редактируемыми объектами: «+» ведёт на выбор объекта', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/payments');

    await expect(page.getByTestId('payments-create')).toBeVisible();
    await page.getByTestId('payments-create').click();
    await expect(page).toHaveURL(/\/payments\/objects$/);
  });

  test('чистый зритель: «+» нет ни в ряду заголовка, ни в компакт-баре (#703)', async ({
    page,
  }) => {
    await openCabinetWithSessionToken(page, seededViewerSessionToken());
    await page.goto('/payments');

    await expect(
      page.getByRole('heading', { name: 'Платежи', exact: true }),
    ).toBeVisible();
    await expect(page.getByTestId('payments-create')).toHaveCount(0);
    await expect(page.getByTestId('payments-create-compact')).toHaveCount(0);
  });
});

// Замыкающие карточки секций — «Все избранные», «Все просроченные»,
// «Показать все» (решение владельца #1339): видны только от трёх карточек
// в секции; страница категории и так открывается с шеврона заголовка.
// Правила избранного — личные строки владельца сида (правка по образцу
// favorites-edit.spec.ts): трогаем только сидовые правила, в конце
// возвращаем сидовое «в избранном одна аренда».
const HUB_FAVORITE_IDS = [
  '55555555-5555-4555-8555-555555555551', // аренда — в сидовом избранном
  '55555555-5555-4555-8555-555555555553', // электричество
  '55555555-5555-4555-8555-555555555554', // домофон
] as const;

test.describe('хаб «Платежи» — замыкающие карточки секций (#1339)', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('сидовой хаб: одно избранное — без замыкания, просрочка и объекты — с ним', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/payments');

    // В сидовом избранном одно правило — секция без замыкающей карточки.
    const favorites = page.getByTestId('payments-section-favorites');
    await expect(favorites.getByText('Арендная плата')).toBeVisible();
    await expect(favorites.getByText('Все избранные')).toHaveCount(0);

    // Просроченных правил в сиде пять, объектов три — обе с замыканием.
    await expect(
      page
        .getByTestId('payments-section-overdue')
        .getByText('Все просроченные'),
    ).toBeVisible();
    await expect(
      page.getByTestId('payments-section-objects').getByText('Показать все'),
    ).toBeVisible();
  });

  test('граница трёх избранных: две карточки — без замыкания, три — с ним', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    const ownerSql = `(SELECT id FROM users WHERE email = '${seededUser.email}')`;
    const seedFavorites = async (ids: ReadonlyArray<string>): Promise<void> => {
      await execE2eSql(
        `DELETE FROM user_payment_favorites
         WHERE user_id = ${ownerSql}
           AND payment_id IN ('${HUB_FAVORITE_IDS.join("','")}');`,
      );
      if (ids.length > 0) {
        await execE2eSql(
          `INSERT INTO user_payment_favorites (user_id, payment_id)
           SELECT ${ownerSql}, id FROM payments
           WHERE id IN ('${ids.join("','")}');`,
        );
      }
    };

    await seedFavorites(HUB_FAVORITE_IDS.slice(0, 2));
    try {
      await page.goto('/payments');
      const favorites = page.getByTestId('payments-section-favorites');
      await expect(favorites.getByText('Электроэнергия')).toBeVisible();
      await expect(favorites.getByText('Все избранные')).toHaveCount(0);

      await seedFavorites(HUB_FAVORITE_IDS);
      await page.goto('/payments');
      const allFavorites = favorites.getByText('Все избранные');
      await expect(allFavorites).toBeVisible();
      await allFavorites.click();
      await expect(page).toHaveURL(/\/payments\/favorites$/);
    } finally {
      // Возврат к сиду в любом исходе: соседние спеки ждут «в избранном
      // одна аренда» (в избранном владельца одна аренда, без позиции).
      await seedFavorites(HUB_FAVORITE_IDS.slice(0, 1));
    }
  });
});
