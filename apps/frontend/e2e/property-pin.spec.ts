import type { Page } from '@playwright/test';
import {
  captureScreen,
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  openCabinetWithSessionToken,
  seededViewerSessionToken,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  test,
} from './fixtures';

// Личный пин объекта — живой сценарий (спека #1302, cutover #1307, D11):
// зритель закрепляет чужой объект по праву просмотра (кнопка доступна по
// видимости, не «полный доступ+»), порядок списка зрителя меняется, а
// порядок владельца чужой пин не трогает — личное состояние читателя, не
// свойство объекта. Порядок среди закреплённых — «первый отмеченный выше»
// (повторный/свежий пин время не меняет и выше раннего не подпрыгивает),
// поэтому сценарий читает естественный порядок хаба до пина и закрепляет
// вторую карточку — она поднимается на первое место. Сид восстанавливается
// в finally (workers=1, соседние спеки читают те же объекты).

const VIEWER_ID = '13111111-1111-4111-8111-111111111131';
const OWNER_ID = '11111111-1111-4111-8111-111111111111';
const GARAGE_VIEWER_MEMBERSHIP_ID = '99999999-9999-4999-8999-999999999941';

const PROPERTY_ID_BY_NAME: Record<string, string> = {
  'Квартира на Ленина': SEEDED_APARTMENT_PROPERTY_ID,
  'Гараж на Садовой': SEEDED_GARAGE_PROPERTY_ID,
};

async function seedPinScenario(): Promise<void> {
  await execE2eSql(`
    DELETE FROM user_property_pins
     WHERE property_id IN ('${SEEDED_APARTMENT_PROPERTY_ID}', '${SEEDED_GARAGE_PROPERTY_ID}');
    INSERT INTO property_members (id, property_id, user_id, role, granted_by)
     VALUES ('${GARAGE_VIEWER_MEMBERSHIP_ID}', '${SEEDED_GARAGE_PROPERTY_ID}', '${VIEWER_ID}', 'viewer', '${OWNER_ID}')
     ON CONFLICT (property_id, user_id) WHERE status = 'active'
     DO UPDATE SET role = EXCLUDED.role;
  `);
}

async function cleanPinScenario(): Promise<void> {
  await execE2eSql(`
    DELETE FROM user_property_pins
     WHERE property_id IN ('${SEEDED_APARTMENT_PROPERTY_ID}', '${SEEDED_GARAGE_PROPERTY_ID}');
    DELETE FROM property_members WHERE id = '${GARAGE_VIEWER_MEMBERSHIP_ID}';
  `);
}

/** Имена карточек главного списка в порядке экрана. */
async function hubCardNames(page: Page): Promise<string[]> {
  await expect(page.getByTestId('properties-list')).toBeVisible();
  return page
    .getByTestId('properties-list')
    .getByRole('heading')
    .allTextContents();
}

test.describe('личный пин объекта #1307', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('зритель закрепляет по видимости: его порядок меняется, владельца — нет (D11)', async ({
    page,
    seededUser,
  }, testInfo) => {
    await seedPinScenario();
    try {
      // Базовый порядок владельца — до чужого пина (3 своих объекта).
      await openCabinetWithSeededSession(page, seededUser);
      await page.goto('/properties');
      const ownerBaseline = await hubCardNames(page);
      expect(ownerBaseline.length).toBeGreaterThanOrEqual(2);

      // Зритель: две видимые карточки, естественный порядок без пинов.
      await openCabinetWithSessionToken(page, seededViewerSessionToken());
      await page.goto('/properties');
      const viewerBefore = await hubCardNames(page);
      expect(viewerBefore.length).toBe(2);

      // Вторая карточка поднимается пином: деталь → «Сделать основным»
      // доступна зрителю (по праву просмотра) → шит #630 исполняет пин,
      // строка переключается на «Убрать из основных» (isPinned читателя).
      const target = viewerBefore[1];
      if (target === undefined) {
        throw new Error(`viewer hub order ${JSON.stringify(viewerBefore)} has no second card`);
      }
      await page.goto(`/properties/${PROPERTY_ID_BY_NAME[target]}`);
      const manage = page.getByTestId('property-manage-list');
      await expect(manage.getByText('Сделать основным')).toBeVisible();
      await manage.getByText('Сделать основным').click();
      await page.getByRole('button', { name: 'Сделать объект основным' }).click();
      await expect(manage.getByText('Убрать из основных')).toBeVisible();
      await captureScreen(page, testInfo, '1307-viewer-pinned');

      // Порядок зрителя: закреплённая карточка теперь первая.
      await page.goto('/properties');
      const viewerAfter = await hubCardNames(page);
      expect(viewerAfter[0]).toBe(target);

      // Владелец: порядок не сдвинулся — чужой (зрительский) пин мой экран
      // не меняет.
      await openCabinetWithSeededSession(page, seededUser);
      await page.goto('/properties');
      expect(await hubCardNames(page)).toEqual(ownerBaseline);

      // Деталь закреплённого зрителем объекта у владельца: его личный пин
      // не поставлен — «Сделать основным».
      await page.goto(`/properties/${PROPERTY_ID_BY_NAME[target]}`);
      await expect(
        page.getByTestId('property-manage-list').getByText('Сделать основным'),
      ).toBeVisible();
    } finally {
      await cleanPinScenario();
    }
  });

  test('двойной клик по подтверждению — ровно один PUT, время первого пина (#1113, D11)', async ({
    page,
  }) => {
    test.setTimeout(120_000);
    await seedPinScenario();
    try {
      await openCabinetWithSessionToken(page, seededViewerSessionToken());
      await page.goto(`/properties/${SEEDED_GARAGE_PROPERTY_ID}`);

      // Счётчик мутаций — page.on('request'), не route (грабля #1113:
      // route-мок валит вкладку).
      const puts: string[] = [];
      page.on('request', (request) => {
        if (request.method() === 'PUT' && request.url().includes('/pin')) {
          puts.push(request.url());
        }
      });

      const manage = page.getByTestId('property-manage-list');
      await expect(manage.getByText('Сделать основным')).toBeVisible();
      await manage.getByText('Сделать основным').click();
      const confirm = page.getByRole('button', { name: 'Сделать объект основным' });
      await expect(confirm).toBeVisible();

      // Двойной клик одним evaluate-таском: гард мутации сводит его к
      // одному PUT (канон #1113).
      await confirm.evaluate((element) => {
        (element as HTMLElement).click();
        (element as HTMLElement).click();
      });

      // Успех: шит закрылся, строка переключилась — к этому моменту PUT
      // завершён и счётчик финален.
      await expect(manage.getByText('Убрать из основных')).toBeVisible();
      expect(puts).toHaveLength(1);

      // SQL-правда: ровно одна строка с одним временем (повторный пин
      // время не меняет).
      const rows = await execE2eSql(
        `SELECT count(*) || '|' || count(DISTINCT pinned_at)
           FROM user_property_pins
          WHERE user_id = '${VIEWER_ID}'
            AND property_id = '${SEEDED_GARAGE_PROPERTY_ID}'`,
      );
      expect(rows).toBe('1|1');
    } finally {
      await cleanPinScenario();
    }
  });
});
