import {
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  SEEDED_GARAGE_PROPERTY_ID,
  test,
} from './fixtures';

// Секция «На оплату» — экраны хаба и страницы (#1347; бекенд #1346):
// правила с включённым напоминанием и без автоплатежа стоят в окне
// D−N ≤ today ≤ D (today — владельца, сид-владелец в UTC, канон #796),
// автоплатёжные в секцию не попадают (решение владельца 12.10).
// Сид — четыре правила на Гараже на Садовой (сидово без платежей, чужие
// спеки не задеваем): сегодня, завтра и два в бакете «Через 3 дня»
// (+2 и +3 — порядок «Сначала новые» внутри бакета проверяется датами),
// плюс автоплатёжное с напоминанием — контрпример. Правила с since в
// будущем: загрузочный тик ничего не материализует поверх сидовых
// операций (канон seed.sql). Уборка удаляет и правила, и их операции —
// afterAll-гейт перед пустыми состояниями тех же экранов.

const GARAGE = SEEDED_GARAGE_PROPERTY_ID;

const RULE_TODAY = 'd0e00001-0000-4000-8000-000000000001';
const RULE_TOMORROW = 'd0e00001-0000-4000-8000-000000000002';
const RULE_FAR_LATE = 'd0e00001-0000-4000-8000-000000000003';
const RULE_FAR_EARLY = 'd0e00001-0000-4000-8000-000000000004';
const RULE_AUTOPAY = 'd0e00001-0000-4000-8000-000000000005';
const ALL_RULES = [
  RULE_TODAY,
  RULE_TOMORROW,
  RULE_FAR_LATE,
  RULE_FAR_EARLY,
  RULE_AUTOPAY,
].join("','");

const OP_TODAY = 'd0e00002-0000-4000-8000-000000000001';
const OP_TOMORROW = 'd0e00002-0000-4000-8000-000000000002';
const OP_FAR_LATE = 'd0e00002-0000-4000-8000-000000000003';
const OP_FAR_EARLY = 'd0e00002-0000-4000-8000-000000000004';
const OP_AUTOPAY = 'd0e00002-0000-4000-8000-000000000005';

// Даты окна в ISO (UTC-календарь сида — канон #796, сид-владелец в UTC):
// операция правила живёт через k дней от сегодня.
function isoDaysAhead(days: number): string {
  const now = new Date();
  const date = new Date(
    Date.UTC(
      now.getUTCFullYear(),
      now.getUTCMonth(),
      now.getUTCDate() + days,
    ),
  );
  return date.toISOString().slice(0, 10);
}

const TODAY_ISO = isoDaysAhead(0);
const TOMORROW_ISO = isoDaysAhead(1);
const FAR_EARLY_ISO = isoDaysAhead(2);
const FAR_LATE_ISO = isoDaysAhead(3);
const AUTOPAY_ISO = isoDaysAhead(7);

async function seedDueRules(): Promise<void> {
  // Пять правил: окна напоминаний N=3/N=7 держат операции сегодня, завтра,
  // +2, +3 и автоплатёжные +7. since = сегодня, день месяца = день операции:
  // для правила это законное ближайшее вхождение, поэтому тик (его гонит
  // мутация оплаты) сидовую операцию сохраняет, а задним числом ничего не
  // материализует — просрочки и чужие вхождения другим спекам не достаются.
  // Повторный сид идемпотентен (ON CONFLICT).
  const ownerSql = `(SELECT id FROM users WHERE email = '${process.env.E2E_USER_EMAIL}')`;
  const day = (iso: string): string => iso.slice(8, 10);
  await execE2eSql(`
    INSERT INTO payments (id, owner_id, property_id, type, title, amount_kopecks,
                          recurrence, since, end_date, auto_pay, category_slug,
                          reminder_offset_days)
    VALUES
        ('${RULE_TODAY}', ${ownerSql}, '${GARAGE}', 'expense', 'Вывоз мусора', 90000, '{"kind": "monthly", "dayOfMonth": ${day(TODAY_ISO)}}', '${TODAY_ISO}', NULL, FALSE, 'garbage-removal', 3),
        ('${RULE_TOMORROW}', ${ownerSql}, '${GARAGE}', 'expense', 'Обслуживание котельной', 250000, '{"kind": "monthly", "dayOfMonth": ${day(TOMORROW_ISO)}}', '${TODAY_ISO}', NULL, FALSE, 'heating', 7),
        ('${RULE_FAR_LATE}', ${ownerSql}, '${GARAGE}', 'expense', 'Проверка счётчиков', 50000, '{"kind": "monthly", "dayOfMonth": ${day(FAR_LATE_ISO)}}', '${TODAY_ISO}', NULL, FALSE, 'utilities', 3),
        ('${RULE_FAR_EARLY}', ${ownerSql}, '${GARAGE}', 'expense', 'Мойка фасада', 400000, '{"kind": "monthly", "dayOfMonth": ${day(FAR_EARLY_ISO)}}', '${TODAY_ISO}', NULL, FALSE, 'cleaning', 3),
        ('${RULE_AUTOPAY}', ${ownerSql}, '${GARAGE}', 'expense', 'Охрана', 300000, '{"kind": "monthly", "dayOfMonth": ${day(AUTOPAY_ISO)}}', '${TODAY_ISO}', NULL, TRUE, 'security', 7)
    ON CONFLICT (id) DO UPDATE
    SET title = EXCLUDED.title,
        amount_kopecks = EXCLUDED.amount_kopecks,
        recurrence = EXCLUDED.recurrence,
        since = EXCLUDED.since,
        end_date = EXCLUDED.end_date,
        auto_pay = EXCLUDED.auto_pay,
        category_slug = EXCLUDED.category_slug,
        reminder_offset_days = EXCLUDED.reminder_offset_days
  `);
  await execE2eSql(`
    INSERT INTO operations (id, owner_id, property_id, payment_id, origin, date,
                            paid_date, status, type, title, amount_kopecks,
                            category_label, category_slug)
    VALUES
        ('${OP_TODAY}', ${ownerSql}, '${GARAGE}', '${RULE_TODAY}', 'payment', '${TODAY_ISO}', NULL, 'planned', 'expense', 'Вывоз мусора', 90000, 'Вывоз мусора', 'garbage-removal'),
        ('${OP_TOMORROW}', ${ownerSql}, '${GARAGE}', '${RULE_TOMORROW}', 'payment', '${TOMORROW_ISO}', NULL, 'planned', 'expense', 'Обслуживание котельной', 250000, 'Обслуживание котельной', 'heating'),
        ('${OP_FAR_LATE}', ${ownerSql}, '${GARAGE}', '${RULE_FAR_LATE}', 'payment', '${FAR_LATE_ISO}', NULL, 'planned', 'expense', 'Проверка счётчиков', 50000, 'Проверка счётчиков', 'utilities'),
        ('${OP_FAR_EARLY}', ${ownerSql}, '${GARAGE}', '${RULE_FAR_EARLY}', 'payment', '${FAR_EARLY_ISO}', NULL, 'planned', 'expense', 'Мойка фасада', 400000, 'Мойка фасада', 'cleaning'),
        ('${OP_AUTOPAY}', ${ownerSql}, '${GARAGE}', '${RULE_AUTOPAY}', 'payment', '${AUTOPAY_ISO}', NULL, 'planned', 'expense', 'Охрана', 300000, 'Охрана', 'security')
    ON CONFLICT (id) DO UPDATE
    SET date = EXCLUDED.date,
        paid_date = EXCLUDED.paid_date,
        status = EXCLUDED.status,
        amount_kopecks = EXCLUDED.amount_kopecks
  `);
}

async function cleanupDueRules(): Promise<void> {
  // Сначала операции (FK правила), затем правила. По payment_id, а не по
  // списку id: тик оплаты наматывает правилам следующие вхождения с новыми
  // id — снесены будут они тоже.
  await execE2eSql(
    `DELETE FROM operations WHERE payment_id IN ('${ALL_RULES}');`,
  );
  await execE2eSql(
    `DELETE FROM payments WHERE id IN ('${ALL_RULES}');`,
  );
}

test.describe('секция «На оплату» — экраны хаба и страницы (#1347)', () => {
  test.use({ viewport: { width: 390, height: 844 } });
  test.setTimeout(240_000);
  test.describe.configure({ mode: 'serial' });

  test.beforeAll(async () => {
    await seedDueRules();
  });

  test.afterAll(async () => {
    await cleanupDueRules();
  });

  test('хаб: секция между «Просроченными» и «Платежами объектов», метки дней, автоплатёж мимо, замыкание от трёх', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/payments');

    const due = page.getByTestId('payments-section-due');
    await expect(due).toBeVisible();
    await expect(due.getByText('Вывоз мусора')).toBeVisible();

    // Порядок секций макета 1036:37525: «На оплату» — третья.
    const positions = await page.evaluate(() => {
      const top = (testid: string) => {
        const node = document.querySelector(`[data-testid="${testid}"]`);
        return node === null ? Number.POSITIVE_INFINITY : node.getBoundingClientRect().top;
      };
      return {
        overdue: top('payments-section-overdue'),
        due: top('payments-section-due'),
        objects: top('payments-section-objects'),
      };
    });
    expect(positions.overdue).toBeLessThan(positions.due);
    expect(positions.due).toBeLessThan(positions.objects);

    // Карточки окна: метка дня синим — «Сегодня», «Завтра», «Через 2 дня»,
    // «Через 3 дня»; автоплатёжное правило в секции не живёт.
    await expect(due.getByText('Сегодня')).toBeVisible();
    await expect(due.getByText('Завтра')).toBeVisible();
    await expect(due.getByText('Через 2 дня')).toBeVisible();
    await expect(due.getByText('Через 3 дня')).toBeVisible();
    await expect(due.getByText('Охрана')).toBeHidden();

    // Замыкающая «Все платежи на оплату» — правило ≥3 (#1339): в секции
    // четыре карточки, замыкание со счётчиком целого скоупа на месте.
    const allCard = due.getByText('Все платежи на оплату');
    await expect(allCard).toBeVisible();
    await expect(due.getByText('4 платежа')).toBeVisible();

    await allCard.click();
    await expect(page).toHaveURL(new RegExp('/payments/due$'));
  });

  test('страница: группы по реальным дням, чип переворачивает группы, состояние в адресе', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/payments/due');

    // Дневные группы (решение владельца 13.10): «Сегодня», «Завтра»,
    // «Через 2 дня», «Через 3 дня» — каждая дата под своим заголовком;
    // дальше окна автоплатёж не тянет (группы day4..day7 пустые).
    await expect(page.getByTestId('payments-due-group-day0')).toBeVisible();
    await expect(page.getByTestId('payments-due-group-day1')).toBeVisible();
    await expect(page.getByTestId('payments-due-group-day2')).toBeVisible();
    await expect(page.getByTestId('payments-due-group-day3')).toBeVisible();
    await expect(page.getByTestId('payments-due-group-day7')).toHaveCount(0);

    // «Сначала новые»: дни по возрастанию — «Через 2 дня» (15-е) над
    // «Через 3 дня» (16-е), разные даты больше не склеены.
    const day2 = page.getByTestId('payments-due-group-day2');
    const day3 = page.getByTestId('payments-due-group-day3');
    const near = day2.getByText('Мойка фасада');
    const far = day3.getByText('Проверка счётчиков');
    await expect(near).toBeVisible();
    await expect(far).toBeVisible();
    expect(await near.boundingBox().then((box) => box?.y ?? 0)).toBeLessThan(
      await far.boundingBox().then((box) => box?.y ?? 0),
    );

    // Строка ведёт на страницу правила (сегмент read-only).
    await far.click();
    await expect(page).toHaveURL(
      new RegExp(`/properties/${GARAGE}/payments/${RULE_FAR_LATE}$`),
    );

    // Чип разворачивает порядок групп ЦЕЛИКОМ («Через 7 дней» сверху —
    // здесь самой дальней группы день 3) и живёт в адресе — перезагрузка
    // состояние держит; внутри дня порядок не трогается.
    await page.goto('/payments/due');
    await page.getByTestId('payments-due-sort-chip').click();
    await expect(page.getByTestId('payments-due-sort-chip')).toHaveText(
      'Сначала старые',
    );
    await expect(page).toHaveURL(new RegExp('/payments/due\\?sort=old$'));
    expect(
      await day3.boundingBox().then((box) => box?.y ?? 0),
    ).toBeLessThan(await day2.boundingBox().then((box) => box?.y ?? 0));
    await page.reload();
    await expect(page.getByTestId('payments-due-sort-chip')).toHaveText(
      'Сначала старые',
    );
  });

  test('оплата гасит строку в секции и на странице; пустые состояния после уборки', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);

    // Оплата — на объектных экранах (сегмент read-only): страница
    // сегодняшней операции, «Отметить оплаченной».
    await page.goto(`/properties/${GARAGE}/operations/${OP_TODAY}`);
    await page.getByRole('button', { name: 'Отметить оплаченной' }).click();
    await expect(page.getByText('Платеж оплачен')).toBeVisible();

    // Строка уходит со страницы: инвалидация глобальной ветки после
    // оплаты — карточки не доживают до конца staleTime.
    await page.goto('/payments/due');
    await expect(page.getByText('Вывоз мусора')).toBeHidden();
    await expect(page.getByText('Обслуживание котельной')).toBeVisible();

    // Хаб: карточек осталось три — замыкание живо, счётчик скоупа сдвинулся.
    await page.goto('/payments');
    const due = page.getByTestId('payments-section-due');
    await expect(due.getByText('Вывоз мусора')).toBeHidden();
    await expect(due.getByText('3 платежа')).toBeVisible();

    // Уборка (afterAll) освобождает окно: пустые состояния обоих экранов —
    // кошелёк 1041:46762 и плейсхолдер секции.
    await cleanupDueRules();
    await page.goto('/payments/due');
    await expect(page.getByTestId('payments-due-empty')).toBeVisible();
    await expect(
      page.getByText('Нет платежей на оплату'),
    ).toBeVisible();
    await page.goto('/payments');
    await expect(
      page
        .getByTestId('payments-section-due')
        .getByText('Здесь будут платежи на оплату'),
    ).toBeVisible();
  });
});
