import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  screenHeader,
  test,
} from './fixtures';

// Экран «Создать задачу» (#500, Figma 1539-77823/1539-78288/1539-82273) —
// двухшаговая форма: шаг 1 название, шаг 2 комментарий/дата-время/повтор.
// «Назад» между шагами (#1065, карта #1052 D5) — канон операции-визарда:
// ведущая кнопка шапки Cancel↔ArrowLeft, «Закрыть» остаётся с шага 1.
// Данные живут в useState — перезагрузка даёт чистый лист (канон
// сохранения #1052). До сабмита сервер ничего не получает — спека данных
// не создаёт.

const CREATE_URL = '/tasks/new';

test.describe('создание задачи — навигация между шагами', () => {
  test('шаг 2 → «Назад» → шаг 1 с сохранённым заголовком', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(CREATE_URL);
    const title = page.getByRole('textbox', { name: 'Задача' });
    await expect(title).toBeVisible();

    // «Далее» гейтится заполненным названием (шаг 1 #500).
    const next = page.getByRole('button', { name: 'Далее' });
    await expect(next).toBeDisabled();
    await title.fill('Поменять смеситель');
    await next.click();

    await expect(page.getByRole('textbox', { name: 'Комментарий' })).toBeVisible();
    await screenHeader(page).getByRole('button', { name: 'Назад' }).click();
    await expect(title).toHaveValue('Поменять смеситель');
    await expect(next).toBeVisible();
    // Обратная половина канона: шапка вернулась в «Закрыть» (✕).
    await expect(screenHeader(page).getByRole('button', { name: 'Закрыть' })).toBeVisible();
    await captureScreen(page, testInfo, 'task-create-back-to-step1');
  });

  test('перезагрузка — чистый лист: шаг 1, пустая форма', async ({
    page,
    seededUser,
  }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(CREATE_URL);
    await page.getByRole('textbox', { name: 'Задача' }).fill('Поменять смеситель');
    await page.getByRole('button', { name: 'Далее' }).click();
    await expect(page.getByRole('textbox', { name: 'Комментарий' })).toBeVisible();
    await captureScreen(page, testInfo, 'task-create-step2-back-button');

    await page.reload();
    await expect(page.getByRole('textbox', { name: 'Задача' })).toHaveValue('');
    await expect(page.getByRole('button', { name: 'Далее' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Комментарий' })).toHaveCount(0);
  });

  test('«Закрыть» с шага 1 закрывает', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/tasks');
    // На хабе кнопка живёт дважды (trailing компакт-шапки + ряд HubTitle) —
    // обе ведут в /tasks/new, спеке всё равно какая.
    await page.getByRole('button', { name: 'Создать задачу' }).last().click();
    await page.waitForURL('**/tasks/new');
    await expect(page.getByRole('textbox', { name: 'Задача' })).toBeVisible();

    await screenHeader(page).getByRole('button', { name: 'Закрыть' }).click();
    await expect(page).toHaveURL(/\/tasks$/);
    await expect(page.getByRole('button', { name: 'Создать задачу' }).last()).toBeVisible();
  });
});
