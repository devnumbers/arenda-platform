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
 * материализуется сразу — undated-задача в группе «Без даты»). Канал
 * осознанный (в спеках прецедента нет): API дешевле UI-обхода и точнее
 * прямой SQL-вставки; очистка — в finally каждого теста. */
async function createTaskRule(page: Page, title: string): Promise<string> {
  const response = await page.request.post(`/api/properties/${APARTMENT}/tasks/rules`, {
    data: { title, repeat: 'once' },
  });
  expect(response.ok(), `создание правила «${title}»`).toBe(true);
  const rule = (await response.json()) as { id: string };
  return rule.id;
}

/** Создание карточки контакта на объекте API-вызовом от сидовой сессии
 * (канал — как у createTaskRule выше). */
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

    // Дефолт «сначала новые»: первая группа — свежая дата (датовые группы
    // несут год, канон 1302:52209; «Сегодня/Вчера» — без).
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
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    // «Бета» создана раньше «Альфы»: дефолт «Дата, asc» держит порядок
    // создания, «Название» его переворачивает. Суффикс retry: повтор
    // упавшей попытки не встречает остатки прошлой (testing-strategy,
    // прецедент contacts-book.spec).
    const titleBeta = `E2E-сорт Бета ${testInfo.retry}`;
    const titleAlpha = `E2E-сорт Альфа ${testInfo.retry}`;
    const ruleBeta = await createTaskRule(page, titleBeta);
    const ruleAlpha = await createTaskRule(page, titleAlpha);
    try {
      await page.goto(`/properties/${APARTMENT}/tasks`);
      const section = page.getByTestId('section-undated');
      await expect(section.getByText(titleBeta)).toBeVisible();
      await expect(section.getByText(titleAlpha)).toBeVisible();
      expect(await yOf(section, titleBeta)).toBeLessThan(
        await yOf(section, titleAlpha),
      );

      const chip = await settle(page.getByTestId('tasks-sort-chip'));
      await chip.click();
      await page.getByRole('menuitem', { name: 'По названию' }).click();
      await expect(page).toHaveURL(/sort=title/);
      expect(await yOf(section, titleAlpha)).toBeLessThan(
        await yOf(section, titleBeta),
      );

      await chip.click();
      await page.getByRole('menuitem', { name: 'Убывание' }).click();
      await expect(page).toHaveURL(/sort=title/);
      await expect(page).toHaveURL(/order=desc/);
      expect(await yOf(section, titleBeta)).toBeLessThan(
        await yOf(section, titleAlpha),
      );

      // Перезагрузка: поле и направление восстановлены из адреса (#785).
      await page.reload();
      await expect(page).toHaveURL(/sort=title/);
      await expect(page).toHaveURL(/order=desc/);
      await expect(chip).toHaveText(/Название/);
      expect(await yOf(section, titleBeta)).toBeLessThan(
        await yOf(section, titleAlpha),
      );

      // «Назад/вперёд» (приёмка #785): смены сортировки пишут replace —
      // записей истории не создают: «назад» уводит со страницы, «вперёд»
      // возвращает её вместе с адресом и сортировкой.
      await page.goBack();
      await expect(page).not.toHaveURL(new RegExp(`properties/${APARTMENT}/tasks`));
      await page.goForward();
      await expect(page).toHaveURL(/sort=title/);
      await expect(page).toHaveURL(/order=desc/);
    } finally {
      await page.request.delete(`/api/properties/${APARTMENT}/tasks/rules/${ruleAlpha}`);
      await page.request.delete(`/api/properties/${APARTMENT}/tasks/rules/${ruleBeta}`);
    }
  });

  test('глобальная лента задач: сортировка живёт в адресе и переживает перезагрузку', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    // Суффикс retry — см. тест «задачи объекта» выше.
    const titleBeta = `E2E-лента Бета ${testInfo.retry}`;
    const titleAlpha = `E2E-лента Альфа ${testInfo.retry}`;
    const ruleBeta = await createTaskRule(page, titleBeta);
    const ruleAlpha = await createTaskRule(page, titleAlpha);
    try {
      // Вход с активным фильтром #524 (?property=): смена сортировки обязана
      // писать ?sort= ПОВЕРХ фильтра, не затирая его (находка код-ревью).
      await page.goto(`/tasks?property=${APARTMENT}`);
      const section = page.getByTestId('section-undated');
      await expect(section.getByText(titleBeta)).toBeVisible();
      await expect(section.getByText(titleAlpha)).toBeVisible();
      expect(await yOf(section, titleBeta)).toBeLessThan(
        await yOf(section, titleAlpha),
      );

      const chip = await settle(page.getByTestId('tasks-sort-chip'));
      await chip.click();
      await page.getByRole('menuitem', { name: 'По названию' }).click();
      await expect(page).toHaveURL(/sort=title/);
      await expect(page).toHaveURL(new RegExp(`property=${APARTMENT}`));
      expect(await yOf(section, titleAlpha)).toBeLessThan(
        await yOf(section, titleBeta),
      );

      // Перезагрузка: поле и фильтр восстановлены из адреса, направление —
      // дефолт (в адресе не писался) (#785).
      await page.reload();
      await expect(page).toHaveURL(/sort=title/);
      await expect(page).toHaveURL(new RegExp(`property=${APARTMENT}`));
      await expect(page.getByTestId('tasks-sort-chip')).toHaveText(/Название/);
      expect(await yOf(section, titleAlpha)).toBeLessThan(
        await yOf(section, titleBeta),
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
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    // Суффикс retry — см. тест «задачи объекта» выше.
    const nameAlpha = `Э2Е Ася ${testInfo.retry}`;
    const nameBeta = `Э2Е Борис ${testInfo.retry}`;
    const contactAlpha = await createContact(page, nameAlpha);
    const contactBeta = await createContact(page, nameBeta);
    try {
      await page.goto(`/properties/${APARTMENT}/contacts`);
      const chip = await settle(page.getByRole('button', { name: 'Имя', exact: true }));
      await expect(chip).toBeVisible();
      // Дефолт «А→Я»: Ася выше Бориса.
      expect(await yOf(page, nameAlpha)).toBeLessThan(await yOf(page, nameBeta));

      await chip.click();
      await page.getByRole('menuitem', { name: 'Имя от Я до А' }).click();
      await expect(page).toHaveURL(/order=desc/);
      expect(await yOf(page, nameBeta)).toBeLessThan(await yOf(page, nameAlpha));

      // Перезагрузка: направление восстановлено из адреса (#785).
      await page.reload();
      await expect(page).toHaveURL(/order=desc/);
      expect(await yOf(page, nameBeta)).toBeLessThan(await yOf(page, nameAlpha));
    } finally {
      await page.request.delete(`/api/contacts/${contactAlpha}`);
      await page.request.delete(`/api/contacts/${contactBeta}`);
    }
  });
});
