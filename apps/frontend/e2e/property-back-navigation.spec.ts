import {
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_PROPERTIES,
  test,
} from './fixtures';

// Стрелка «Назад» верхней цепочки ходит по истории браузера (решение
// владельца 21.10 — замена структурных push'ей #472/#1374): «Назад»
// возвращает туда, откуда пришли, и конечная цепочка
// операция → операции → хаб → список проходима назад целиком —
// дубликатов хаба в истории нет, рост стека на каждый клик «Назад»
// вверх исчез.

const APARTMENT = SEEDED_APARTMENT_PROPERTY_ID;
const APARTMENT_NAME = SEEDED_PROPERTIES[0];

test.describe('стрелка «Назад» верхней цепочки', () => {
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

    // «Назад» ленты — по истории на хаб.
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${APARTMENT}$`));

    // «Назад» хаба — по истории на список объектов.
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(/\/properties$/);
    await expect(
      page.getByRole('heading', { name: 'Объекты', exact: true }),
    ).toBeVisible();
  });

  test('операция → строка «Объект» → хаб → «Назад» — страница операции (источник)', async ({
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

    // «Назад» хаба — по истории на источник: страницу операции, откуда
    // пришли (решение владельца 21.10: честная история, список объектов
    // остаётся достижимым следующими «Назад»).
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(
      new RegExp(`/properties/${APARTMENT}/operations/[0-9a-f-]+$`),
    );

    // Дальше по истории — лента операций.
    await page.getByRole('button', { name: 'Назад' }).click();
    await expect(page).toHaveURL(
      new RegExp(`/properties/${APARTMENT}/operations$`),
    );
  });
});
