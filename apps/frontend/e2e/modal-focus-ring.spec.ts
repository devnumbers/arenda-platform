import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';
import type { Locator } from '@playwright/test';
/**
 * Контракт «никогда» на кольце фокуса триггера (решение владельца
 * 19.10): после закрытия модальной поверхности фокус возвращается на
 * триггер (Tab-навигация не деградирует), но БЕЗ :focus-visible-кольца —
 * даже при клавиатурном закрытии по Esc, на котором эвристика браузера и
 * рисовала залипшее кольцо: программный focus() дефолта Radix наследует
 * клавиатурную модальность ввода. Живая эвристика проверяется в реальном
 * Chromium (подавление — focus({focusVisible:false}), Chromium 145+);
 * механика хендлеров запинена юнит-каноном focus-restore.test.ts.
 *
 * Поверхности: витрина /ui-kit — канонический Modal с настоящим
 * ModalTrigger (на нём дефолт Radix и лип: triggerRef.focus() после
 * Esc-закрытия), и модалка «Связаться с нами» — controlled без
 * DialogTrigger (там старый код фокус не возвращает вовсе — фокус падал
 * в body; контракт чинит и это, возвратом на захваченного хозяина).
 * Сидовая сессия нужна только чтобы middleware пустил в /ui-kit
 * (прецедент wheel-picker.spec.ts).
 */

const UI_KIT = '/ui-kit';

const ringOn = (locator: Locator): Promise<boolean> =>
  locator.evaluate((el) => el.matches(':focus-visible'));

test('Esc-закрытие модалки с триггером: фокус вернулся, кольца нет', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(UI_KIT);

  const trigger = page.getByRole('button', { name: 'Открыть модалку' });
  // Открытие с клавиатуры (Enter по сфокусированной кнопке) —
  // клавиатурная модальность, худший для контракта случай: на нём
  // дефолтный возврат фокуса Radix и лип.
  await trigger.focus();
  await page.keyboard.press('Enter');

  const dialog = page.getByRole('dialog', { name: 'Что создать?' });
  await expect(dialog).toBeVisible();

  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();

  await expect(trigger).toBeFocused();
  expect(await ringOn(trigger)).toBe(false);

  await captureScreen(page, testInfo, 'modal-focus-ring-esc');
});

test('закрытие кликом по «Отмена»: фокус вернулся, кольца нет — гард клик-флоу', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(UI_KIT);

  const trigger = page.getByRole('button', { name: 'Открыть модалку' });
  await trigger.click();

  const dialog = page.getByRole('dialog', { name: 'Что создать?' });
  await expect(dialog).toBeVisible();

  await dialog.getByRole('button', { name: 'Отмена' }).click();
  await expect(dialog).toBeHidden();

  await expect(trigger).toBeFocused();
  expect(await ringOn(trigger)).toBe(false);
});

test('Esc-закрытие controlled-модалки без триггера: фокус вернулся на хозяина, кольца нет', async ({ page }, testInfo) => {
  await page.goto('/login');
  const trigger = page.getByRole('button', { name: 'Написать в поддержку' });
  await trigger.focus();
  await page.keyboard.press('Enter');

  const dialog = page.getByRole('dialog', { name: 'Связаться с нами' });
  await expect(dialog).toBeVisible();

  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();

  // У SupportModal нет DialogTrigger — старый код Radix вообще не
  // возвращал фокус (triggerRef пуст, фокус падал в body).
  await expect(trigger).toBeFocused();
  expect(await ringOn(trigger)).toBe(false);

  await captureScreen(page, testInfo, 'modal-focus-ring-esc-controlled');
});

test('меню объекта: клавиатурное открытие, Esc — триггер в фокусе без кольца', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}`);

  const trigger = page.getByRole('button', { name: 'Действия с объектом' });
  // Клавиатурное открытие меню (Enter) — модальность клавиатурная: на ней
  // дефолтный возврат фокуса Radix и лип. Хозяин пинится на самом
  // триггере (event.currentTarget его события) — у DropdownMenu своего
  // onOpenAutoFocus нет.
  await trigger.focus();
  await page.keyboard.press('Enter');

  const menu = page.getByRole('menu');
  await expect(menu).toBeVisible();

  await page.keyboard.press('Escape');
  await expect(menu).toBeHidden();

  await expect(trigger).toBeFocused();
  expect(await ringOn(trigger)).toBe(false);
});

test.describe('мобайл 390: vaul-канва Modal', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('на шите фокус вернулся в хозяина без кольца', async ({ page }) => {
    // Хозяин — пилюля на самой странице логина, не ячейка чужого шита:
    // открытие SupportModal над шитом «Еще» закрывает их одной жестом
    // (pointerdown гасит модалку, click доезжает до её бэкдропа), якорь
    // уходит из DOM и возврат легитимно вырождается в body.
    await page.goto('/login');
    const trigger = page.getByRole('button', { name: 'Написать в поддержку' });
    await trigger.click();

    const dialog = page.getByRole('dialog', { name: 'Связаться с нами' });
    await expect(dialog).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();

    await expect(trigger).toBeFocused();
    expect(await ringOn(trigger)).toBe(false);
  });
});
