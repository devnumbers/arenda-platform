import type { Page } from '@playwright/test';
import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  screenHeader,
  SEEDED_APARTMENT_PROPERTY_ID,
  SEEDED_GARAGE_PROPERTY_ID,
  test,
} from './fixtures';

// Пустота глобальной ленты «Задачи» с фильтром «Объект» (решение владельца
// 15.10.2026): фильтр без результатов не прячет ряд чипов и «+» — вместо
// канона «Задач нет» минимальное «Задач не найдено. Попробуйте изменить
// фильтр» (канон пустого периода операций #474); совсем без задач и без
// фильтра — прежний канон «Задач нет» с CTA (анатомия 3235-74057).
// Сид: квартира с задачей, гараж намеренно пуст (seed.sql).

/** Сидовая квартира — на ней создаём задачу; гараж — пустой фильтр. */
const APARTMENT = SEEDED_APARTMENT_PROPERTY_ID;
const GARAGE = SEEDED_GARAGE_PROPERTY_ID;

/** Создание правила задачи API-вызовом от сидовой сессии — канон
 * sorting-persistence.spec (правило без срока материализуется сразу). */
async function createTaskRule(page: Page, title: string): Promise<string> {
  const response = await page.request.post(`/api/properties/${APARTMENT}/tasks/rules`, {
    data: { title, repeat: 'once' },
  });
  expect(response.ok(), `создание правила «${title}»`).toBe(true);
  const rule = (await response.json()) as { id: string };
  return rule.id;
}

test.describe('пустая лента задач с фильтром', () => {
  test('фильтр без результатов: чипы и «+» на месте, текст про фильтр, фильтр снимается чипом', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    const ruleId = await createTaskRule(page, 'E2E фильтр без результатов');
    try {
      await page.goto(`/tasks?property=${GARAGE}`);

      // Минимальное состояние (канон пустого периода #474): одна строка
      // про фильтр, без заголовка «Задач нет» и без CTA «Добавить задачу».
      await expect(page.getByText('Задач не найдено. Попробуйте изменить фильтр')).toBeVisible();
      await expect(page.getByRole('heading', { name: 'Задач нет' })).toHaveCount(0);
      await expect(page.getByRole('button', { name: 'Добавить задачу' })).toHaveCount(0);

      // Чипы не спрятаны: фильтр «Объект» активен, «+» шапки на месте —
      // фильтр меняют тем же чипом (решение владельца 15.10.2026).
      const objectChip = page.getByRole('button', { name: 'Объект' });
      await expect(objectChip).toBeVisible();
      await expect(screenHeader(page).getByRole('button', { name: 'Создать задачу' })).toBeVisible();
      await captureScreen(page, testInfo, 'tasks-feed-filtered-empty');

      // Снятие фильтра тем же чипом: «Все задачи» → лента с задачей.
      await objectChip.click();
      await page.getByRole('checkbox', { name: 'Все задачи' }).click();
      await page.getByRole('button', { name: 'Выбрать' }).click();
      await expect(page).toHaveURL(/\/tasks$/);
      await expect(page.getByText('E2E фильтр без результатов')).toBeVisible();
    } finally {
      await page.request.delete(`/api/properties/${APARTMENT}/tasks/rules/${ruleId}`);
    }
  });

  test('совсем пустая лента без фильтра — прежний канон «Задач нет» с CTA', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/tasks');

    await expect(page.getByRole('heading', { name: 'Задач нет' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Добавить задачу' })).toBeVisible();
    // Чипы с фильтром спрятаны вместе с «+» шапки (анатомия 3235-74057).
    await expect(page.getByRole('button', { name: 'Объект' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Создать задачу' })).toHaveCount(0);

    // Верх пустоты — канон §7 (замечание владельца 23.10): от нижней кромки
    // строки заголовка хаба иллюстрацию ведёт только pt-14 (56) самого
    // EmptyState — без mt-6 обёртки ленты, как на «Платежах» и bareEmpty
    // «Объектов» (решение владельца 14.10.2026).
    const titleBox = await page
      .getByRole('heading', { name: 'Задачи', exact: true })
      .boundingBox();
    const imageBox = await page.locator('img[src*="empty-tasks"]').boundingBox();
    if (titleBox === null || imageBox === null) {
      throw new Error('не нашлись заголовок хаба «Задачи» или иллюстрация пустоты');
    }
    expect(Math.round(imageBox.y - (titleBox.y + titleBox.height))).toBe(56);
  });
});
