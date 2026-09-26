import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  screenHeader,
  test,
} from './fixtures';

// Историческая страница-заглушка единой навигации хрома (карта #556,
// тикет #559) заменена живым разделом: хаб «Совместный доступ» (карта
// #692, тикет #696) с карточками-счётчиками, CTA и входом из профиля.
// Заглушка «Платежи» была заменена картой #573 — её покрытие переехало
// в payments.spec.ts.

test('хаб «Совместный доступ»: карточки со счётчиками сида и CTA', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants');

  const header = screenHeader(page);
  await expect(header.getByRole('link', { name: 'Объекты' })).toBeVisible();
  await expect(
    page.getByRole('heading', { level: 1, name: 'Совместный доступ' }),
  ).toBeVisible();

  // Сид: владелец «Квартиры на Ленина» с двумя участниками (member +
  // viewer, seed.sql); чужих объектов у него нет.
  const participantsCard = page.getByRole('link', { name: /Ваши участники/ });
  await expect(participantsCard).toBeVisible();
  await expect(participantsCard.getByText('2 участника')).toBeVisible();

  const propertiesCard = page.getByRole('link', { name: /Объекты пользователей/ });
  await expect(propertiesCard).toBeVisible();
  await expect(propertiesCard.getByText('Нет объектов')).toBeVisible();

  // Иконка-приглашение в строке заголовка (слоты мобайла и десктопа
  // рендерятся оба — виден актуальный ярусу).
  await expect(
    header.getByRole('button', { name: 'Пригласить участника' }).first(),
  ).toBeVisible();
  // CTA в нижней панели — текстовая кнопка (вне шапки).
  await expect(
    page
      .getByRole('button', { name: 'Пригласить участника' })
      .filter({ hasText: 'Пригласить участника' }),
  ).toBeVisible();

  // 3D-иллюстрации карточек: к моменту скриншота (design-comparison в CI)
  // данные должны быть декодированы — naturalWidth 48 появляется только
  // после загрузки пикселей (нативные <img> next/image; svg-иконки строк
  // локатором не матчатся).
  await expect(participantsCard.locator('img')).toHaveJSProperty('naturalWidth', 48);
  await expect(propertiesCard.locator('img')).toHaveJSProperty('naturalWidth', 48);

  await captureScreen(page, testInfo, 'participants-hub');
});

test('цели карточек хаба — живые маршруты-каркасы, не 404', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);

  const header = screenHeader(page);

  await page.goto('/participants/list');
  await expect(header.getByText('Ваши участники')).toBeVisible();

  await page.goto('/participants/properties');
  // Живой экран #701: по макету (2010-132145) заголовок «Доступные объекты»,
  // не имя карточки хаба.
  await expect(header.getByText('Доступные объекты')).toBeVisible();

  await page.goto('/participants/invite');
  // Живой экран #699: по макету (2008-46375) заголовок в контенте, шапка —
  // только «Назад».
  await expect(
    page.getByRole('heading', { name: 'Пригласите участника' }),
  ).toBeVisible();
});

test('вход из профиля: строка «Участники» ведёт на хаб', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/profile');

  await page.getByRole('button', { name: 'Участники' }).click();
  await page.waitForURL('**/participants');
  await expect(
    page.getByRole('heading', { level: 1, name: 'Совместный доступ' }),
  ).toBeVisible();
});

