import {
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Кликабельность заголовков-секций аренды всей линией (#1242, правило
// владельца «область нажатия на блоках должна занимать весь блок»):
// у секции со стрелкой при тексте (RentalGroup, arrowPosition="text" —
// канон #531) вся линия заголовка — цель тапа, не только «текст + стрелка».
// Точка правее стрелки, но внутри линии заголовка, ведёт по назначению
// секции; клавиатура (Enter на заголовке) ведёт туда же.
//
// Объект сценария — отдельная строка в properties (сид-квартира занята
// ассертами других спек, UUID не пересекаются с rental-page-actions):
// арендная пара сеется SQL-контрактом бэка, уборка — в afterEach.

const PROPERTY_ID = '42420000-4242-4000-8000-000000012420';
const PAYMENT_ID = '42420000-4242-4000-8000-000000012421';
const RENTAL_ID = '42420000-4242-4000-8000-000000012422';
const OWNER_SQL = `(SELECT id FROM users WHERE email = '${process.env.E2E_USER_EMAIL}')`;

test.describe('кликабельность линий заголовков секций аренды', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  /** Чистый объект сценария: снос прошлых прогонов и создание строки. */
  async function seedProperty(): Promise<void> {
    await execE2eSql(`DELETE FROM rentals WHERE property_id = '${PROPERTY_ID}'`);
    await execE2eSql(`DELETE FROM properties WHERE id = '${PROPERTY_ID}'`);
    await execE2eSql(
      `INSERT INTO properties (id, owner_id, name, type, address, status)
       VALUES ('${PROPERTY_ID}', ${OWNER_SQL}, 'Дом кликабельных линий', 'house', 'ул. Линий, 12', 'active')`,
    );
  }

  /** Активная аренда SQL-контрактом бэка: управляемый Платёж «Арендная
   * плата» + аренда (страница аренды с секциями «Платеж»/«Условия»). */
  async function seedRental(): Promise<void> {
    await execE2eSql(
      `INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
                             recurrence, since, end_date, auto_pay, category_slug)
       VALUES ('${PAYMENT_ID}', ${OWNER_SQL}, '${PROPERTY_ID}',
               'income', 'Арендная плата', 4873000,
               '{"kind":"monthly","daysOfMonth":[15]}'::jsonb,
               current_date - 60, current_date + 305,
               false, 'rent')`,
    );
    await execE2eSql(
      `INSERT INTO rentals (id, owner_id, property_id, payment_id, start_date,
                            planned_end_date, completed_date, utilities)
       VALUES ('${RENTAL_ID}', ${OWNER_SQL}, '${PROPERTY_ID}',
               '${PAYMENT_ID}',
               current_date - 60, current_date + 305,
               NULL, 'included')`,
    );
  }

  /** Путь пользователя (канон #698): список → ряд объекта → секция
   * «Аренда» (прецедент rental-page-actions — deep-link ловит дубль
   * testid в окне гидрации). */
  async function openRentalPage(
    page: Parameters<typeof openCabinetWithSeededSession>[0],
    user: Parameters<typeof openCabinetWithSeededSession>[1],
  ): Promise<void> {
    await openCabinetWithSeededSession(page, user);
    await page.goto('/properties');
    await page.getByRole('link', { name: 'Дом кликабельных линий' }).click();
    await page.getByRole('link', { name: 'Аренда' }).click();
    await expect(page.getByRole('button', { name: 'Открыть условия аренды' })).toBeVisible();
  }

  test.afterEach(async () => {
    await execE2eSql(`DELETE FROM rentals WHERE property_id = '${PROPERTY_ID}'`);
    await execE2eSql(`DELETE FROM properties WHERE id = '${PROPERTY_ID}'`);
  });

  test('клик правее стрелки «Условий аренды» (внутри линии) открывает условия', async ({
    page,
    seededUser,
  }) => {
    await seedProperty();
    await seedRental();
    await openRentalPage(page, seededUser);

    const header = page.getByRole('button', { name: 'Открыть условия аренды' });
    // Обёртка линии заголовка (px-6 карточки): её правый край минус 35 —
    // правее контентной кнопки «текст + стрелка», но внутри кликабельной
    // линии (кнопка доходит до внутреннего края паддинга — минус 24).
    const headerLine = header.locator('..');
    const wrapBox = await headerLine.boundingBox();
    const lineBox = await header.boundingBox();
    if (wrapBox === null || lineBox === null) {
      throw new Error('Линия заголовка секции не отрисовалась');
    }
    await headerLine.click({
      position: {
        x: wrapBox.width - 35,
        y: lineBox.y - wrapBox.y + lineBox.height / 2,
      },
    });

    await expect(page).toHaveURL(
      new RegExp(`/properties/${PROPERTY_ID}/rentals/terms`),
    );
  });

  test('клавиатура: Enter на заголовке секции ведёт по назначению', async ({
    page,
    seededUser,
  }) => {
    await seedProperty();
    await seedRental();
    await openRentalPage(page, seededUser);

    const header = page.getByRole('button', { name: 'Открыть условия аренды' });
    await header.focus();
    await page.keyboard.press('Enter');

    await expect(page).toHaveURL(
      new RegExp(`/properties/${PROPERTY_ID}/rentals/terms`),
    );
  });
});
