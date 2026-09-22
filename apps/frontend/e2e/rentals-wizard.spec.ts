import {
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  test,
} from './fixtures';

// Визард создания аренды (#530) и шаг «Контакт арендатора» по серии 1855
// (#807): заголовок + два действия («Выбрать контакт» — отдельный экран
// выбора, «Создать контакт» — ветвь #509), экран выбора с серверным поиском
// и сортировкой «по недавним», меню из четырёх пунктов, свап кнопок экрана
// успеха («Хорошо» primary сверху) и server-truth `rentals.contact_id`.
// Черновик визарда живёт в localStorage — между маршрутами едет сам.
//
// Имена контактов уникальны за попытку (суффикс — номер retry), остатки
// сценария тест убирает в finally: контакты — DELETE по имени; аренда
// сносит rentals → operations → payments по уникальной сумме (инвариант
// «одна незавершённая аренда на объект» не должен ломать повтор прогона).

const APARTMENT_URL = `/properties/${SEEDED_APARTMENT_PROPERTY_ID}`;
const GARAGE_URL = `/properties/${SEEDED_GARAGE_PROPERTY_ID}`;
const APARTMENT_WIZARD_URL = `${APARTMENT_URL}/rentals/new`;
const GARAGE_WIZARD_URL = `${GARAGE_URL}/rentals/new`;

test.describe('визард создания аренды — шаг «Контакт арендатора»', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  /** Войти в визард под сид-пользователем: как в реальном потоке — через
   * страницу объекта, чтобы закрытие успеха шло назад по истории. */
  async function openWizard(
    page: Parameters<typeof openCabinetWithSeededSession>[0],
    user: Parameters<typeof openCabinetWithSeededSession>[1],
    wizardUrl: string,
    propertyUrl: string,
  ): Promise<void> {
    await openCabinetWithSeededSession(page, user);
    await page.goto(propertyUrl);
    await page.goto(wizardUrl);
    await expect(page.getByRole('heading', { name: 'Цена и число оплаты' })).toBeVisible();
  }

  /** Шаг 1: сумма и день оплаты (пикер дней, коммит «Выбрать»). */
  async function passAmountDay(
    page: Parameters<typeof openCabinetWithSeededSession>[0],
    rubles: string,
  ): Promise<void> {
    await page.getByRole('textbox', { name: 'Арендная плата, рублей' }).fill(rubles);
    await page.getByRole('button', { name: 'День оплаты: Выбрать день' }).click();
    const dayPicker = page.getByRole('dialog', { name: 'Выбор дня оплаты' });
    await dayPicker.getByRole('button', { name: '10', exact: true }).click();
    await dayPicker.getByRole('button', { name: 'Выбрать', exact: true }).click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await expect(page.getByRole('heading', { name: 'Условия аренды' })).toBeVisible();
  }

  /** Шаг 2: начало — сегодня (черновик канона при открытии), коммит
   * «Выбрать»; шаг 3: автоплатёж по желанию → «Продолжить». */
  async function passConditions(
    page: Parameters<typeof openCabinetWithSeededSession>[0],
    autoPay = false,
  ): Promise<void> {
    await page.getByRole('button', { name: 'Начало аренды: Выбрать дату' }).click();
    const calendar = page.getByRole('dialog', { name: 'Начало аренды' });
    await expect(calendar).toBeVisible();
    await calendar.getByRole('button', { name: 'Выбрать', exact: true }).click();
    // Шаг 2 закрывает «Далее», шаг 3 — «Продолжить».
    await page.getByRole('button', { name: 'Далее' }).click();
    if (autoPay) {
      await page.getByRole('switch', { name: 'Сделать платеж автоматическим' }).click();
    }
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await expect(
      page.getByRole('heading', { name: 'Добавьте контакт арендатора' }),
    ).toBeVisible();
  }

  /** Просеять шаги 1–3 до шага «Контакт арендатора». */
  async function passToContactStep(
    page: Parameters<typeof openCabinetWithSeededSession>[0],
    user: Parameters<typeof openCabinetWithSeededSession>[1],
    wizardUrl: string,
    propertyUrl: string,
    options: { rubles: string; autoPay?: boolean } = { rubles: '56 000' },
  ): Promise<void> {
    await openWizard(page, user, wizardUrl, propertyUrl);
    await passAmountDay(page, options.rubles);
    await passConditions(page, options.autoPay);
  }

  /** Засидировать контакт объекта с контролем created_at (недавние —
   * сверху на экране выбора). */
  async function seedContact(params: {
    readonly propertyId: string;
    readonly name: string;
    readonly role: string | null;
    readonly ageInterval: string;
  }): Promise<void> {
    const role = params.role === null ? 'NULL' : `'${params.role}'`;
    await execE2eSql(
      `INSERT INTO contacts (id, owner_id, property_id, first_name, role, created_at, updated_at)
       SELECT gen_random_uuid(), p.owner_id, p.id, '${params.name}', ${role},
              now() - interval '${params.ageInterval}', now() - interval '${params.ageInterval}'
       FROM properties p WHERE p.id = '${params.propertyId}'`,
    );
  }

  /** Убрать аренду сценария: rentals → operations → payments по сумме —
   * инвариант «одна незавершённая аренда» не должен ломать повтор. */
  async function cleanupRental(propertyId: string, rubles: string): Promise<void> {
    const kopecks = rubles.replace(/\s/g, '') + '00';
    await execE2eSql(`DELETE FROM rentals WHERE property_id = '${propertyId}'`);
    await execE2eSql(
      `DELETE FROM operations WHERE payment_id IN (
         SELECT id FROM payments WHERE property_id = '${propertyId}' AND amount_kopecks = '${kopecks}'
       )`,
    );
    await execE2eSql(
      `DELETE FROM payments WHERE property_id = '${propertyId}' AND amount_kopecks = '${kopecks}'`,
    );
  }

  test('пустая книга: только «Создать контакт», ветвь #509 с ролью, футер глушится, «Закрыть» возвращает на источник', async ({
    page,
    seededUser,
  }) => {
    await passToContactStep(page, seededUser, GARAGE_WIZARD_URL, GARAGE_URL);

    // Заголовок серии 1855 + пустая книга (решение владельца): единственная
    // строка «Создать контакт».
    await expect(
      page.getByText('Выберите существующий или создайте новый контакт'),
    ).toBeVisible();
    await expect(page.getByRole('button', { name: 'Выбрать контакт' })).toBeHidden();
    await expect(page.getByRole('button', { name: 'Создать контакт' })).toBeVisible();

    // Ветка #509: роль «Арендатор» подставлена; футер глушится, как на
    // шагах визарда (#807, P3).
    await page.getByRole('button', { name: 'Создать контакт' }).click();
    await expect(page.getByRole('button', { name: 'Отменить создание' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Роль' })).toHaveValue('Арендатор');
    await expect(
      page.getByRole('navigation', { name: 'Нижняя навигация' }),
    ).toBeHidden();

    // «Закрыть» визарда: goBack возвращает на источник по истории
    // (вход был со страницы объекта, как в реальном потоке).
    await page.getByRole('button', { name: 'Отменить создание' }).click();
    await expect(page.getByRole('button', { name: 'Создать контакт' })).toBeVisible();
    await page.getByRole('button', { name: 'Закрыть' }).click();
    await expect(page).toHaveURL(new RegExp(`/properties/${SEEDED_GARAGE_PROPERTY_ID}$`));
  });

  test('экран выбора: недавние сверху, роли, серверный поиск; выбор и «Выбрать другой»; тап по выбранному — карточка', async ({
    page,
    seededUser,
  }, testInfo) => {
    const aleksei = `Алексей Тест ${testInfo.retry}`;
    const boris = `Борис Тест ${testInfo.retry}`;
    const propertyId = SEEDED_APARTMENT_PROPERTY_ID;
    try {
      await seedContact({ propertyId, name: aleksei, role: 'Арендатор', ageInterval: '2 days' });
      await seedContact({ propertyId, name: boris, role: null, ageInterval: '0 hours' });
      await passToContactStep(page, seededUser, APARTMENT_WIZARD_URL, APARTMENT_URL);

      // Непустая книга — два действия (1855:63385); строки создания больше
      // на шаге нет, книга живёт на отдельном экране.
      await expect(page.getByRole('button', { name: 'Выбрать контакт' })).toBeVisible();
      await expect(page.getByRole('button', { name: 'Создать контакт' })).toBeVisible();
      await page.getByRole('button', { name: 'Выбрать контакт' }).click();

      // Экран выбора (1855:64129): поисковая шапка, свежий Борис сверху,
      // у Алексея подпись роли; у Бориса роли нет — подзаголовка нет.
      await expect(page.getByRole('searchbox', { name: 'Поиск контактов' })).toBeVisible();
      const alekseiRow = page.getByText(aleksei, { exact: true });
      const borisRow = page.getByText(boris, { exact: true });
      await expect(alekseiRow).toBeVisible();
      await expect(borisRow).toBeVisible();
      await expect(page.getByText('Арендатор', { exact: true })).toBeVisible();
      const borisBox = (await borisRow.boundingBox()) as { y: number };
      const alekseiBox = (await alekseiRow.boundingBox()) as { y: number };
      expect(borisBox.y).toBeLessThan(alekseiBox.y);

      // Серверный ?search=: фильтр по подстроке имени, прежний срез держится
      // (keepPreviousData), сброс возвращает книгу.
      await page.getByRole('searchbox', { name: 'Поиск контактов' }).fill('Бор');
      await expect(alekseiRow).toBeHidden();
      await expect(borisRow).toBeVisible();
      await page.getByRole('searchbox', { name: 'Поиск контактов' }).fill('');
      await expect(alekseiRow).toBeVisible();

      // Тап по строке выбирает арендатора в черновик и возвращает на шаг;
      // строка выбранного с кебабом (1855:64385).
      await borisRow.click();
      await expect(page.getByRole('heading', { name: 'Добавьте контакт арендатора' })).toBeVisible();
      await expect(page.getByText(boris, { exact: true })).toBeVisible();

      // «Выбрать другой» — единственная смена арендатора (решение
      // владельца 22.09).
      await page.getByRole('button', { name: 'Меню контакта' }).click();
      await page.getByRole('menuitem', { name: 'Выбрать другой' }).click();
      await expect(page.getByRole('searchbox', { name: 'Поиск контактов' })).toBeVisible();
      await page.getByText(aleksei, { exact: true }).click();
      await expect(page.getByRole('heading', { name: 'Добавьте контакт арендатора' })).toBeVisible();
      await expect(page.getByText(aleksei, { exact: true })).toBeVisible();

      // Тап по выбранной строке — карточка контакта (решение владельца),
      // а не повторный выбор.
      await page.getByText(aleksei, { exact: true }).click();
      await expect(page).toHaveURL(
        new RegExp(`/properties/${propertyId}/contacts/[0-9a-f-]{36}$`),
      );
    } finally {
      await execE2eSql(`DELETE FROM contacts WHERE property_id = '${propertyId}'
         AND first_name IN ('${aleksei}', '${boris}')`);
    }
  });

  test('меню контакта: «Изменить» открывает форму, «Удалить» отвязывает и деградирует шаг к двум действиям', async ({
    page,
    seededUser,
  }, testInfo) => {
    const contactName = `Марина Тест ${testInfo.retry}`;
    // Второй контакт остаётся в книге: после удаления выбранного шаг
    // деградирует к состоянию непустой книги — два действия.
    const leftoverName = `Сергей Тест ${testInfo.retry}`;
    const propertyId = SEEDED_APARTMENT_PROPERTY_ID;
    try {
      await seedContact({ propertyId, name: contactName, role: 'Арендатор', ageInterval: '1 hours' });
      await seedContact({ propertyId, name: leftoverName, role: null, ageInterval: '2 hours' });
      await passToContactStep(page, seededUser, APARTMENT_WIZARD_URL, APARTMENT_URL);
      await page.getByRole('button', { name: 'Выбрать контакт' }).click();
      await page.getByText(contactName, { exact: true }).click();
      await expect(page.getByRole('heading', { name: 'Добавьте контакт арендатора' })).toBeVisible();

      await page.getByRole('button', { name: 'Меню контакта' }).click();
      await page.getByRole('menuitem', { name: 'Изменить' }).click();
      await expect(page).toHaveURL(/\/contacts\/[0-9a-f-]{36}\/edit$/);
      await page.goBack();
      await expect(
        page.getByRole('heading', { name: 'Добавьте контакт арендатора' }),
      ).toBeVisible();

      // Удаление: шит подтверждения (шит 1419:27158), после — контакт
      // стёрт из книги, шаг вернулся к двум действиям.
      await page.getByRole('button', { name: 'Меню контакта' }).click();
      await page.getByRole('menuitem', { name: 'Удалить' }).click();
      await expect(page.getByText('Удалить контакт?')).toBeVisible();
      await page.getByRole('button', { name: 'Удалить', exact: true }).click();
      await expect(page.getByRole('button', { name: 'Выбрать контакт' })).toBeVisible();
      await expect(page.getByText(contactName, { exact: true })).toBeHidden();
      expect(await execE2eSql(
        `SELECT count(*) FROM contacts WHERE first_name = '${contactName}'`,
      )).toBe('0');
    } finally {
      await execE2eSql(`DELETE FROM contacts WHERE property_id = '${propertyId}'
         AND first_name IN ('${contactName}', '${leftoverName}')`);
    }
  });

  test('создание с арендатором: свап кнопок успеха (ручная отметка), «Открыть аренду» — детализация, contact_id в БД', async ({
    page,
    seededUser,
  }, testInfo) => {
    const contactName = `Ольга Тест ${testInfo.retry}`;
    const propertyId = SEEDED_APARTMENT_PROPERTY_ID;
    const rubles = '47 000';
    try {
      await seedContact({ propertyId, name: contactName, role: 'Арендатор', ageInterval: '3 hours' });
      await passToContactStep(page, seededUser, APARTMENT_WIZARD_URL, APARTMENT_URL, { rubles });
      await page.getByRole('button', { name: 'Выбрать контакт' }).click();
      await page.getByText(contactName, { exact: true }).click();
      await expect(page.getByText(contactName, { exact: true })).toBeVisible();
      await page.getByRole('button', { name: 'Создать аренду' }).click();

      // Экран успеха: «Хорошо» — primary сверху, «Открыть аренду» —
      // secondary снизу (свап #807); копия ручной отметки.
      await expect(page.getByRole('heading', { name: 'Вы создали аренду' })).toBeVisible();
      await expect(page.getByText('отмечается вручную')).toBeVisible();
      const good = page.getByRole('button', { name: 'Хорошо' });
      const open = page.getByRole('button', { name: 'Открыть аренду' });
      await expect(good).toBeVisible();
      await expect(open).toBeVisible();
      const goodBox = (await good.boundingBox()) as { y: number };
      const openBox = (await open.boundingBox()) as { y: number };
      expect(goodBox.y).toBeLessThan(openBox.y);

      await open.click();
      await expect(page).toHaveURL(
        new RegExp(`/properties/${propertyId}/rentals$`),
      );
      const linkedTenant = await execE2eSql(
        `SELECT c.first_name FROM rentals r JOIN contacts c ON c.id = r.contact_id
         WHERE r.property_id = '${propertyId}' ORDER BY r.created_at DESC LIMIT 1`,
      );
      expect(linkedTenant).toBe(contactName);
    } finally {
      await cleanupRental(propertyId, rubles);
      await execE2eSql(`DELETE FROM contacts WHERE property_id = '${propertyId}'
         AND first_name = '${contactName}'`);
    }
  });

  test('создание без арендатора (автоплатёж): свап кнопок на автоматической копии, contact_id пуст', async ({
    page,
    seededUser,
  }) => {
    const propertyId = SEEDED_GARAGE_PROPERTY_ID;
    const rubles = '51 000';
    try {
      await passToContactStep(page, seededUser, GARAGE_WIZARD_URL, GARAGE_URL, {
        rubles,
        autoPay: true,
      });
      await expect(page.getByRole('button', { name: 'Выбрать контакт' })).toBeHidden();
      await page.getByRole('button', { name: 'Создать аренду' }).click();

      await expect(page.getByRole('heading', { name: 'Вы создали аренду' })).toBeVisible();
      await expect(page.getByText('фиксируется автоматически')).toBeVisible();
      const good = page.getByRole('button', { name: 'Хорошо' });
      const open = page.getByRole('button', { name: 'Открыть аренду' });
      await expect(good).toBeVisible();
      await expect(open).toBeVisible();
      const goodBox = (await good.boundingBox()) as { y: number };
      const openBox = (await open.boundingBox()) as { y: number };
      expect(goodBox.y).toBeLessThan(openBox.y);

      const linkedCount = await execE2eSql(
        `SELECT count(*) FROM rentals WHERE property_id = '${propertyId}' AND contact_id IS NOT NULL`,
      );
      expect(linkedCount).toBe('0');
    } finally {
      await cleanupRental(propertyId, rubles);
    }
  });
});
