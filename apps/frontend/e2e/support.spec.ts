import type { Page } from '@playwright/test';
import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Страница «Поддержка» /support (макеты 40000074:80636/40000075:80950,
// 28.10): восстановлена вместо модалки #766 — пилюля десктопа и шестая
// ячейка шита «Еще» ведут на страницу, на /support обе подсвечены
// активными. Модалка «Связаться с нами» осталась только на шагах входа
// (Telegram t.me/smirnowwwivan, почта hello@rentlee.ru с копированием
// в буфер).

/** Контент страницы: H1-хаб, блок «Свяжитесь с нами», Telegram-ссылка
 * и почта с копированием — состав макета 40000074:80636. */
async function expectSupportPageContent(page: Page): Promise<void> {
  await expect(page.getByRole('heading', { name: 'Поддержка', level: 1 })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Свяжитесь с нами' })).toBeVisible();
  await expect(page.getByText('Напишите нам в Телеграм или на почту')).toBeVisible();
  await expect(page.locator('img[src*="support-hero"]')).toBeVisible();

  // Telegram — внешняя ссылка в новой вкладке; адрес — из
  // shared/config/support.ts.
  const telegram = page.getByRole('link', { name: 'Написать в Телеграм' });
  await expect(telegram).toHaveAttribute('href', 'https://t.me/smirnowwwivan');
  await expect(telegram).toHaveAttribute('target', '_blank');

  // Почта — копирование в буфер: иконка/подпись сменяются на
  // «Скопировано» на 2 с (канон карточки контакта).
  const copy = page.getByRole('button', { name: /Скопировать/ });
  await expect(copy).toContainText('hello@rentlee.ru');
  await copy.click();
  await expect(page.getByRole('button', { name: 'Скопировано' })).toBeVisible();
}

test('модалка шагов входа — единственное место SupportModal', async ({ page }, testInfo) => {
  await page.goto('/login');
  await page.getByRole('button', { name: 'Написать в поддержку' }).click();

  const dialog = page.getByRole('dialog', { name: 'Связаться с нами' });
  await expect(dialog).toBeVisible();
  await expect(page.getByText('Напишите нам в Телеграм или на почту')).toBeVisible();

  const telegram = dialog.getByRole('link', { name: 'Написать в Телеграм' });
  await expect(telegram).toHaveAttribute('href', 'https://t.me/smirnowwwivan');
  await expect(telegram).toHaveAttribute('target', '_blank');

  const copy = dialog.getByRole('button', { name: /Скопировать/ });
  await expect(copy).toContainText('hello@rentlee.ru');
  await copy.click();
  await expect(dialog.getByRole('button', { name: 'Скопировано' })).toBeVisible();

  await page.keyboard.press('Escape');
  await expect(dialog).toBeHidden();

  await captureScreen(page, testInfo, 'support-modal-login');
});

test('пилюля «Поддержка» кабинета ведёт на страницу /support', async ({ page, seededUser }, testInfo) => {
  // Пилюля — часть ПК-хрома (≥1024); в мобильном проекте её нет.
  test.skip(
    testInfo.project.name === 'mobile-chromium',
    'пилюля «Поддержка» — только проект chromium (ПК-хром ≥1024)',
  );

  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties');

  await page.getByRole('link', { name: 'Поддержка' }).click();
  await expect(page).toHaveURL(/\/support$/);

  // Пилюля подсвечена активной на своём разделе (getActiveNavItem).
  await expect(page.getByRole('link', { name: 'Поддержка' })).toHaveAttribute(
    'aria-current',
    'page',
  );

  await expectSupportPageContent(page);

  await captureScreen(page, testInfo, 'support-page-pill');
});

test.describe('мобайл 390', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('шестая ячейка шита «Еще» ведёт на страницу /support', async ({ page, seededUser }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/properties');

    await page.getByRole('button', { name: 'Еще' }).click();
    // Ячейка шита — ссылка раздела, не кнопка-действие.
    await page.getByRole('link', { name: 'Поддержка' }).click();

    // Шит закрылся, страница открылась.
    await expect(page).toHaveURL(/\/support$/);
    await expectSupportPageContent(page);

    await captureScreen(page, testInfo, 'support-page-more-sheet');
  });

  test('глубокая ссылка: /support открывается напрямую, «Еще» активен', async ({ page, seededUser }, testInfo) => {
    await openCabinetWithSeededSession(page, seededUser);

    const response = await page.goto('/support');
    expect(response?.status()).toBe(200);
    await expectSupportPageContent(page);

    // Таб «Еще» подсвечен на всей странице раздела (getActiveMobileTab).
    const more = page.getByRole('button', { name: 'Еще' });
    await expect(more).toHaveAttribute('aria-expanded', 'false');
    await more.click();
    await expect(page.getByRole('link', { name: 'Поддержка' })).toHaveAttribute(
      'aria-current',
      'page',
    );

    await captureScreen(page, testInfo, 'support-page-deeplink');
  });
});
