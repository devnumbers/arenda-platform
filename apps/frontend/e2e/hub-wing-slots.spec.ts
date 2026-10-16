import { expect, openCabinetWithSeededSession, screenHeader, test } from './fixtures';

// Постоянные правые слоты хабов ниже ПК (решение владельца 02.10, макет
// 2329-148674): barTrailing хаба и trailing компакта живут в самом ряду
// крыла — клиренс от ссылки профиля структурный при любой ширине имени.
// Раньше слоты стояли на константном клиренсе 72 (barTrailing, аудит
// #876) или на крае колонки 560 (компакт) и наезжали на крыло: кебаб
// «Уведомлений» — при pending-скелетоне, «+» компакта «Задач» — на всём
// диапазоне 561–800 (замер живой приёмки 02.10: на 600px точка клика
// ловила ссылку профиля).

test.describe('крылья хабов ниже ПК — слоты в ряду крыла', () => {
  test('планшет 600: «+» компакта «Операций» кликабелен рядом с крылом', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.setViewportSize({ width: 600, height: 900 });
    await page.goto('/operations');
    const header = screenHeader(page);

    // Прокрутка хаб-шапки поднимает компакт — ждём класс гейта кликов
    // (hub-collapse-on). «+» из ряда крыла ведёт в визард операции.
    await page.evaluate(() => window.scrollTo(0, 600));
    await page.waitForFunction(() => document.documentElement.classList.contains('hub-collapse-on'));
    const plus = header
      .locator('div.hub-compact.flex')
      .getByRole('button', { name: 'Добавить операцию' });
    await plus.click();
    await expect(page).toHaveURL(/\/operations\/new$/);
  });

  test('мобайл 390: крыло профиля на «Задачах» — ссылка на /profile', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/tasks');
    const header = screenHeader(page);

    await header.getByRole('link', { name: /Иван|Профиль/ }).click();
    await expect(page).toHaveURL(/\/profile$/);
  });
});
