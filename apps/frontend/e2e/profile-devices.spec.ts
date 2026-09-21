import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

import type { Page } from '@playwright/test';

// Экран «Устройства» (карта #724, тикет #730; моки 1804-105061,
// 1903-39135, 1903-38679): секция «Это устройство» (синий «В сети»),
// красное действие «Завершить все другие сеансы», «Активные сеансы» и два
// confirm-шита. Ревокации гоняются ТОЛЬКО на page.route-моках — живые
// DELETE /me/sessions* угасили бы сид-сессии параллельных воркеров (тот же
// запрет, что у подтверждённого логаута в profile-tree).

test.describe('экран «Устройства» — список и ревокации #730', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  type MockSession = {
    readonly id: string;
    readonly deviceType: 'computer' | 'phone';
    readonly browser: string;
    readonly browserMajor: number;
    readonly os: string;
    readonly city: string | null;
    readonly lastSeenAt: string;
    readonly createdAt: string;
    readonly current: boolean;
  };

  const CURRENT: MockSession = {
    id: '11111111-1111-4111-8111-111111111111',
    deviceType: 'phone',
    browser: 'Chrome',
    browserMajor: 121,
    os: 'Android',
    city: 'Москва',
    lastSeenAt: '2026-09-18T09:00:00Z',
    createdAt: '2026-09-01T08:00:00Z',
    current: true,
  };
  const OTHER_PHONE: MockSession = {
    id: '22222222-2222-4222-8222-222222222222',
    deviceType: 'phone',
    browser: 'Safari',
    browserMajor: 26,
    os: 'iOS',
    city: 'Екатеринбург',
    lastSeenAt: '2026-08-14T14:41:00Z',
    createdAt: '2026-08-01T08:00:00Z',
    current: false,
  };
  const OTHER_COMPUTER: MockSession = {
    id: '33333333-3333-4333-8333-333333333333',
    deviceType: 'computer',
    browser: 'Chrome',
    browserMajor: 120,
    os: 'macOS',
    city: 'Московская область',
    lastSeenAt: '2026-08-13T11:53:00Z',
    createdAt: '2026-08-02T08:00:00Z',
    current: false,
  };

  /** Мок GET /me/sessions с изменяемым составом: мутации экрана
   * инвалидируют запрос, перечитывание получает следующий ответ. */
  function mockSessions(page: Page): {
    setSessions: (sessions: ReadonlyArray<MockSession>) => void;
  } {
    let sessions: ReadonlyArray<MockSession> = [CURRENT, OTHER_PHONE, OTHER_COMPUTER];
    void page.route('**/api/me/sessions**', (route) =>
      route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ sessions }),
      }),
    );
    return { setSessions: (next) => (sessions = next) };
  }

  /** Строка чужой сессии по городу. Фильтр visible — Next держит в body
   * скрытый клон дерева (прецедент profile-tree), дублирующий локаторы. */
  function otherRow(page: Page, city: string) {
    return page
      .locator('div[role="button"]')
      .filter({ visible: true })
      .filter({ hasText: city });
  }

  test('секции по моку: текущая «В сети» синим, чужие серым, красное действие', async ({
    page,
    seededUser,
  }, testInfo) => {
    mockSessions(page);
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/devices');

    await expect(
      page
        .getByRole('heading', { level: 2, name: 'Это устройство' })
        .filter({ visible: true }),
    ).toBeVisible();

    // Текущая: «Chrome 121» + синий «В сети • Москва»; строка
    // неинтерактивна — завершается выходом на хабе, не ревокацией.
    const currentText = page.getByText('В сети • Москва').filter({ visible: true });
    await expect(currentText).toBeVisible();
    await expect(currentText).toHaveCSS('color', 'rgb(43, 127, 255)');

    await expect(
      page
        .getByRole('button', { name: 'Завершить все другие сеансы' })
        .filter({ visible: true }),
    ).toBeVisible();
    await expect(
      page
        .getByRole('heading', { level: 2, name: 'Активные сеансы' })
        .filter({ visible: true }),
    ).toBeVisible();

    // Чужие: момент активности • город, порядок бэка (по свежей активности).
    const ekb = otherRow(page, 'Екатеринбург');
    await expect(ekb.getByText('14 августа')).toBeVisible();
    // Серый тон чужой строки: #6F787C мока 1804-105061 (текущая — синий).
    await expect(ekb.getByText('• Екатеринбург')).toHaveCSS('color', 'rgb(111, 120, 124)');
    const region = otherRow(page, 'Московская область');
    await expect(region.getByText('13 августа')).toBeVisible();
    await expect(ekb.getByRole('button')).toHaveCount(0); // шеврон декоративен

    await captureScreen(page, testInfo, 'profile-devices-screen-mobile');
  });

  test('строка чужой сессии — шит завершения (мок 1903-38679); отмена и подтверждение', async ({
    page,
    seededUser,
  }, testInfo) => {
    const mock = mockSessions(page);
    void page.route('**/api/me/sessions/22222222-2222-4222-8222-222222222222', (route) =>
      route.fulfill({ status: 204 }),
    );
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/devices');

    const ekb = otherRow(page, 'Екатеринбург');
    await ekb.click();

    // Шит: иконка устройства, «Safari 26», детали, «Отменить»+danger.
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText('Safari 26').filter({ visible: true })).toBeVisible();
    await expect(dialog.getByText('Екатеринбург')).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Отменить' })).toBeVisible();
    await expect(dialog.getByRole('button', { name: 'Завершить' })).toBeVisible();
    await captureScreen(page, testInfo, 'profile-devices-revoke-sheet-mobile');

    // Отмена закрывает, строка остаётся.
    await dialog.getByRole('button', { name: 'Отменить' }).click();
    await expect(page.getByRole('dialog')).toHaveCount(0);
    await expect(ekb).toBeVisible();

    // Подтверждение: DELETE-мок 204 → перечитывание без завершённой сессии.
    mock.setSessions([CURRENT, OTHER_COMPUTER]);
    await ekb.click();
    await dialog.getByRole('button', { name: 'Завершить' }).click();
    await expect(page.getByRole('dialog')).toHaveCount(0);
    await expect(
      page
        .getByRole('heading', { level: 2, name: 'Активные сеансы' })
        .filter({ visible: true }),
    ).toBeVisible();
    await expect(otherRow(page, 'Екатеринбург')).toHaveCount(0);
    await expect(otherRow(page, 'Московская область')).toBeVisible();
  });

  test('красное действие — шит «Завершить сеансы» (мок 1903-39135); подтверждение гасит секцию', async ({
    page,
    seededUser,
  }, testInfo) => {
    const mock = mockSessions(page);
    void page.route('**/api/me/sessions/logout-others', (route) =>
      route.fulfill({ status: 204 }),
    );
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/devices');

    await page.getByRole('button', { name: 'Завершить все другие сеансы' }).click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByText('Завершить сеансы')).toBeVisible();
    await expect(
      dialog.getByText('Уверены, что хотите завершить все другие сеансы, кроме текущего?'),
    ).toBeVisible();
    await captureScreen(page, testInfo, 'profile-devices-logout-others-sheet-mobile');

    // Отмена: секции на месте.
    await dialog.getByRole('button', { name: 'Отменить' }).click();
    await expect(page.getByRole('dialog')).toHaveCount(0);
    await expect(
      page
        .getByRole('heading', { level: 2, name: 'Активные сеансы' })
        .filter({ visible: true }),
    ).toBeVisible();

    // Подтверждение: POST-мок 204 → перечитывание вернёт только текущую —
    // красное действие и «Активные сеансы» скрываются (действия нет).
    mock.setSessions([CURRENT]);
    await page.getByRole('button', { name: 'Завершить все другие сеансы' }).click();
    await dialog.getByRole('button', { name: 'Завершить' }).click();
    await expect(page.getByRole('dialog')).toHaveCount(0);
    await expect(
      page
        .getByRole('heading', { level: 2, name: 'Это устройство' })
        .filter({ visible: true }),
    ).toBeVisible();
    await expect(
      page
        .getByRole('heading', { level: 2, name: 'Активные сеансы' })
        .filter({ visible: true }),
    ).toHaveCount(0);
    await expect(
      page
        .getByRole('button', { name: 'Завершить все другие сеансы' })
        .filter({ visible: true }),
    ).toHaveCount(0);
  });

  test('одна текущая сессия: без красного действия и секции «Активные сеансы»', async ({
    page,
    seededUser,
  }) => {
    mockSessions(page).setSessions([CURRENT]);
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/devices');

    await expect(
      page
        .getByRole('heading', { level: 2, name: 'Это устройство' })
        .filter({ visible: true }),
    ).toBeVisible();
    await expect(
      page
        .getByRole('button', { name: 'Завершить все другие сеансы' })
        .filter({ visible: true }),
    ).toHaveCount(0);
    await expect(
      page
        .getByRole('heading', { level: 2, name: 'Активные сеансы' })
        .filter({ visible: true }),
    ).toHaveCount(0);
  });
});
