import type { Locator, Page } from '@playwright/test';
import {
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';

/**
 * Персистентность сортировок (тикет #785): выбор чипа живёт в query строки
 * страницы (?sort=&order= — конвенция состояния в адресе) и переживает
 * перезагрузку. Покрыты четыре поверхности с сидовыми/создаваемыми API
 * данными; «История операций» аренды использует тот же парсинг и переключатель,
 * что и история платежа (unit-покрытие parseHistoryOrderParams), сидовой
 * завершённой аренды у стенда нет.
 */

/** Сидовая квартира с платежами и правилом «Интернет» (seed.sql). */
const APARTMENT = SEEDED_APARTMENT_PROPERTY_ID;
/** Сидовое правило «Интернет» с 55+ paid-вхождениями (seed.sql, #466). */
const INTERNET_PAYMENT_ID = '55555555-5555-4555-8555-555555555556';

/** Вертикальная координата строки по тексту — порядок строк внутри списка. */
async function yOf(scope: Locator | Page, text: string): Promise<number> {
  const box = await scope.getByText(text).boundingBox();
  expect(box, `строка «${text}» видима`).not.toBeNull();
  return box?.y ?? Number.NaN;
}

/** Единственный экземпляр локатора перед первым действием: при стриминге
 * Next экран на долю секунды смонтирован в DOM дважды (замер: ~100 мс
 * после прихода данных) — к клику ждём схлопывания дубля. */
async function settle(locator: Locator): Promise<Locator> {
  await expect(locator).toHaveCount(1);
  return locator;
}

/** Создание правила задачи API-вызовом от сидовой сессии (правило без срока
 * материализуется сразу — undated-задача в группе «Без даты»). */
async function createTaskRule(page: Page, title: string): Promise<string> {
  const response = await page.request.post(`/api/properties/${APARTMENT}/tasks/rules`, {
    data: { title, repeat: 'once' },
  });
  expect(response.ok(), `создание правила «${title}»`).toBe(true);
  const rule = (await response.json()) as { id: string };
  return rule.id;
}

/** Создание карточки контакта на объекте API-вызовом от сидовой сессии. */
async function createContact(page: Page, firstName: string): Promise<string> {
  const response = await page.request.post('/api/contacts', {
    data: { propertyId: APARTMENT, firstName },
  });
  expect(response.ok(), `создание контакта «${firstName}»`).toBe(true);
  const contact = (await response.json()) as { id: string };
  return contact.id;
}

test.describe('сортировки переживают перезагрузку (#785)', () => {
  test('история платежа: направление живёт в адресе и переживает перезагрузку', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`/properties/${APARTMENT}/payments/${INTERNET_PAYMENT_ID}/history`);

    // Дефолт «сначала новые»: первая группа — свежая дата без года.
    const freshChip = await settle(
      page.getByRole('button', { name: /Сортировка: сначала новые/ }),
    );
    await expect(freshChip).toBeVisible();
    const newestFirstHeading = await page.getByRole('heading').first().textContent();

    // Переключение на «сначала старые»: адрес получил ?order=asc, первая
    // группа — самая старая сидовая дата (55+ месяцев, год в подписи).
    await freshChip.click();
    await expect(page).toHaveURL(/order=asc/);
    await expect(page.getByRole('button', { name: /Сортировка: сначала старые/ })).toBeVisible();
    const oldestFirstHeading = await page.getByRole('heading').first().textContent();
    expect(oldestFirstHeading, 'первая группа в asc — дата с годом')
      .toMatch(/\d{4}/);
    expect(oldestFirstHeading).not.toEqual(newestFirstHeading);

    // Перезагрузка: направление восстановлено из адреса (#785).
    await page.reload();
    await expect(page).toHaveURL(/order=asc/);
    await expect(page.getByRole('button', { name: /Сортировка: сначала старые/ })).toBeVisible();
    await expect(page.getByRole('heading').first()).toHaveText(oldestFirstHeading ?? '');
  });

  test('задачи объекта: сортировка живёт в адресе и переживает перезагрузку', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    // «Бета» создана раньше «Альфы»: дефолт «Дата, asc» держит порядок
    // создания, «Название» его переворачивает.
    const ruleBeta = await createTaskRule(page, 'E2E-сорт Бета');
    const ruleAlpha = await createTaskRule(page, 'E2E-сорт Альфа');
    try {
      await page.goto(`/properties/${APARTMENT}/tasks`);
      const section = page.getByTestId('section-undated');
      await expect(section.getByText('E2E-сорт Бета')).toBeVisible();
      await expect(section.getByText('E2E-сорт Альфа')).toBeVisible();
      expect(await yOf(section, 'E2E-сорт Бета')).toBeLessThan(
        await yOf(section, 'E2E-сорт Альфа'),
      );

      const chip = await settle(page.getByTestId('tasks-sort-chip'));
      await chip.click();
      await page.getByRole('menuitem', { name: 'По названию' }).click();
      await expect(page).toHaveURL(/sort=title/);
      expect(await yOf(section, 'E2E-сорт Альфа')).toBeLessThan(
        await yOf(section, 'E2E-сорт Бета'),
      );

      await chip.click();
      await page.getByRole('menuitem', { name: 'Убывание' }).click();
      await expect(page).toHaveURL(/sort=title/);
      await expect(page).toHaveURL(/order=desc/);
      expect(await yOf(section, 'E2E-сорт Бета')).toBeLessThan(
        await yOf(section, 'E2E-сорт Альфа'),
      );

      // Перезагрузка: поле и направление восстановлены из адреса (#785).
      await page.reload();
      await expect(page).toHaveURL(/sort=title/);
      await expect(page).toHaveURL(/order=desc/);
      await expect(chip).toHaveText(/Название/);
      expect(await yOf(section, 'E2E-сорт Бета')).toBeLessThan(
        await yOf(section, 'E2E-сорт Альфа'),
      );
    } finally {
      await page.request.delete(`/api/properties/${APARTMENT}/tasks/rules/${ruleAlpha}`);
      await page.request.delete(`/api/properties/${APARTMENT}/tasks/rules/${ruleBeta}`);
    }
  });

  test('глобальная лента задач: сортировка живёт в адресе и переживает перезагрузку', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    const ruleBeta = await createTaskRule(page, 'E2E-лента Бета');
    const ruleAlpha = await createTaskRule(page, 'E2E-лента Альфа');
    try {
      // Вход с активным фильтром #524 (?property=): смена сортировки обязана
      // писать ?sort= ПОВЕРХ фильтра, не затирая его (находка код-ревью).
      await page.goto(`/tasks?property=${APARTMENT}`);
      const section = page.getByTestId('section-undated');
      await expect(section.getByText('E2E-лента Бета')).toBeVisible();
      await expect(section.getByText('E2E-лента Альфа')).toBeVisible();
      expect(await yOf(section, 'E2E-лента Бета')).toBeLessThan(
        await yOf(section, 'E2E-лента Альфа'),
      );

      const chip = await settle(page.getByTestId('tasks-sort-chip'));
      await chip.click();
      await page.getByRole('menuitem', { name: 'По названию' }).click();
      await expect(page).toHaveURL(/sort=title/);
      await expect(page).toHaveURL(new RegExp(`property=${APARTMENT}`));
      expect(await yOf(section, 'E2E-лента Альфа')).toBeLessThan(
        await yOf(section, 'E2E-лента Бета'),
      );

      // Перезагрузка: поле и фильтр восстановлены из адреса, направление —
      // дефолт (в адресе не писался) (#785).
      await page.reload();
      await expect(page).toHaveURL(/sort=title/);
      await expect(page).toHaveURL(new RegExp(`property=${APARTMENT}`));
      await expect(page.getByTestId('tasks-sort-chip')).toHaveText(/Название/);
      expect(await yOf(section, 'E2E-лента Альфа')).toBeLessThan(
        await yOf(section, 'E2E-лента Бета'),
      );

      // Возврат на дефолт: параметры снимаются из адреса (находка живой
      // приёмки #785) — иначе после перезагрузки сортировка «возвращается».
      await chip.click();
      await page.getByRole('menuitem', { name: 'Убывание' }).click();
      await expect(page).toHaveURL(/order=desc/);
      await chip.click();
      await page.getByRole('menuitem', { name: 'По дате создания' }).click();
      await expect(page).toHaveURL(/order=desc/);
      await chip.click();
      await page.getByRole('menuitem', { name: 'Возрастание' }).click();
      await expect(page).toHaveURL(new RegExp(`property=${APARTMENT}`));
      await expect(page).not.toHaveURL(/sort=|order=/);
      await expect(page.getByTestId('tasks-sort-chip')).toHaveText(/Дата/);
    } finally {
      await page.request.delete(`/api/properties/${APARTMENT}/tasks/rules/${ruleAlpha}`);
      await page.request.delete(`/api/properties/${APARTMENT}/tasks/rules/${ruleBeta}`);
    }
  });

  test('контакты объекта: направление живёт в адресе и переживает перезагрузку', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    const contactAlpha = await createContact(page, 'Э2Е Ася');
    const contactBeta = await createContact(page, 'Э2Е Борис');
    try {
      await page.goto(`/properties/${APARTMENT}/contacts`);
      const chip = await settle(page.getByRole('button', { name: 'Имя', exact: true }));
      await expect(chip).toBeVisible();
      // Дефолт «А→Я»: Ася выше Бориса.
      expect(await yOf(page, 'Э2Е Ася')).toBeLessThan(await yOf(page, 'Э2Е Борис'));

      await chip.click();
      await page.getByRole('menuitem', { name: 'Имя от Я до А' }).click();
      await expect(page).toHaveURL(/order=desc/);
      expect(await yOf(page, 'Э2Е Борис')).toBeLessThan(await yOf(page, 'Э2Е Ася'));

      // Перезагрузка: направление восстановлено из адреса (#785).
      await page.reload();
      await expect(page).toHaveURL(/order=desc/);
      expect(await yOf(page, 'Э2Е Борис')).toBeLessThan(await yOf(page, 'Э2Е Ася'));
    } finally {
      await page.request.delete(`/api/contacts/${contactAlpha}`);
      await page.request.delete(`/api/contacts/${contactBeta}`);
    }
  });
});
