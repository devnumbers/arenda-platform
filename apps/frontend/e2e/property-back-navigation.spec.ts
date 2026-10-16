import {
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_PROPERTIES,
  test,
} from './fixtures';

// Стрелка «Назад» хаба объекта — регресс цикла навигации: вверх на хаб
// ведут push'и (стрелка «Операций» объекта, #472, и строка «Объект» на
// странице операции), и history.back() со страницы объекта разворачивал
// обратно в ветку операций/платежей — список объектов оставался
// недостижим, сколько ни жми. Хабовая стрелка детерминированна: всегда
// ведёт на /properties, с какого места стека её ни нажми.

const APARTMENT = SEEDED_APARTMENT_PROPERTY_ID;
const APARTMENT_NAME = SEEDED_PROPERTIES[0];

test.describe('стрелка «Назад» хаба объекта', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('объект → операции → «Назад» → объект → «Назад» — список объектов', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/properties');
    // Карточку перекрывает overlay-ссылка всей площади (zARUQW_cardLink) —
    // клик по заголовку ловит «intercepts pointer events».
    await page.getByRole('link', { name: `Открыть объект ${APARTMENT_NAME}` }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${APARTMENT}$`));

    // Секция «Операции в …» хаба — обычный push вниз по иерархии; у секции
    // два якоря с одним href (overlay + видимая ссылка) — берём ролевой.
    await page.getByRole('link', { name: /Операции в/ }).click();
    await expect(page).toHaveURL(
      new RegExp(`/properties/${APARTMENT}/operations$`),
    );

    // Стрелка «Операций» (#472) кладёт хаб в историю как новую запись.
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${APARTMENT}$`));

    // Регресс: здесь была history.back() — возврат в «Операции».
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(/\/properties$/);
    await expect(
      page.getByRole('heading', { name: 'Объекты', exact: true }),
    ).toBeVisible();
  });

  test('операция → строка «Объект» → хаб → «Назад» — список объектов', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`/properties/${APARTMENT}/operations`);

    // Сидовые оплаченные операции квартиры — «Интернет» (operations.spec);
    // постраничный текст неуникален (по месяцу на строку) — ролевой
    // локатор с .first() (канон локаторов #860/#925).
    await page.getByRole('button', { name: 'Интернет -1 000 ₽' }).first().click();
    await expect(page).toHaveURL(
      new RegExp(`/properties/${APARTMENT}/operations/[0-9a-f-]+$`),
    );

    // Строка «Объект» страницы операции — push хаба поверх ветки; имя
    // строки — «<объект> Объект» (title + subtitle), берём целиком.
    await page
      .getByRole('button', { name: `${APARTMENT_NAME} Объект` })
      .click();
    await expect(page).toHaveURL(new RegExp(`/properties/${APARTMENT}$`));

    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(/\/properties$/);
    await expect(
      page.getByRole('heading', { name: 'Объекты', exact: true }),
    ).toBeVisible();
  });
});
