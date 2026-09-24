import {
  captureScreen,
  expect,
  execE2eSql,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Страница участника и её подэкраны (карта #692, тикет #698): блок
// «аватар — имя — почта — чип агрегата», «Доступные объекты» с бейджами
// ролей, экран «Права участника» (сегмент роли, отзыв из объекта) и
// «Пригласить в объект» (мультичек, POST /participants/{id}/properties).
// Сид: владелец «Квартиры на Ленина» (33333333-…) с двумя участниками —
// Мария Петрова (full_access, 12111111-…21) и Сергей Сидоров (viewer,
// 13111111-…31), оба active на квартире; гараж (44444444-…) свободен.
// Разрушающие тесты восстанавливают сид через execE2eSql (workers=1).

const header = 'header[aria-label="Навигация экрана"]';

const MARIA_ID = '12111111-1111-4111-8111-111111111121';
const SERGEY_ID = '13111111-1111-4111-8111-111111111131';
const APARTMENT_ID = '33333333-3333-4333-8333-333333333333';
const GARAGE_ID = '44444444-4444-4444-8444-444444444444';

test('страница участника: шапка, чип агрегата, ряды объектов с бейджами ролей, кебаб', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/participants/${MARIA_ID}`);

  // exact+first: в переходный момент в шапке встречается дубль-спан.
  await expect(
    page.locator(header).getByText('Участник', { exact: true }).first(),
  ).toBeVisible();

  // Блок участника: имя, почта, чип агрегата (partial — активна 1 из 3).
  await expect(page.getByRole('heading', { name: 'Мария Петрова' })).toBeVisible();
  await expect(page.getByText('e2e-member@example.com')).toBeVisible();
  await expect(page.getByText('Доступно 1 объект')).toBeVisible();

  await expect(page.getByText('Доступные объекты')).toBeVisible();
  const apartmentRow = page.getByRole('button', { name: /Квартира на Ленина/ });
  await expect(apartmentRow).toBeVisible();
  await expect(apartmentRow.getByText('Редактирование')).toBeVisible();

  // Кебаб шапки: приглашение, действия участника (#712) и отзыв
  // (макет 2008-48318).
  await page.locator(header).getByRole('button', { name: 'Еще — действия с участником' }).click();
  await expect(page.getByRole('menuitem', { name: 'Пригласить в объект' })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'Действия участника' })).toBeVisible();
  await expect(page.getByRole('menuitem', { name: 'Отозвать доступ к объектам' })).toBeVisible();
  await page.keyboard.press('Escape');

  await captureScreen(page, testInfo, 'participant-page');
});

test('deep-link на отозванного/чужого участника — «Участник не найден»', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  // Существующий пользователь, но вне сцопа читающего — приватный 404.
  await page.goto('/participants/00000000-0000-4000-8000-000000000000');

  await expect(page.getByText('Участник не найден')).toBeVisible();
  // Кебаб не показывается без данных (§7), назад в шапке жив.
  await expect(
    page.locator(header).getByRole('button', { name: 'Еще — действия с участником' }),
  ).toHaveCount(0);
  await expect(page.locator(header).getByRole('button', { name: 'Назад' })).toBeVisible();
});

test('права участника: смена роли сегментом, попап «Права изменены»; сид восстанавливается', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/participants/${MARIA_ID}`);

  await page.getByRole('button', { name: /Квартира на Ленина/ }).click();
  await expect(page.locator(header).getByText('Права участника')).toBeVisible();
  await expect(page.getByRole('radio', { name: 'Редактирование' })).toBeChecked();

  try {
    await page.getByRole('radio', { name: 'Просмотр' }).click();

    await expect(
      page.getByRole('dialog').locator('p', { hasText: 'Права изменены' }),
    ).toBeVisible();
    await page.keyboard.press('Escape');

    // Бейдж на странице участника перечитан (инвалидация агрегатов).
    await page.locator(header).getByRole('button', { name: 'Назад' }).click();
    await expect(
      page.getByRole('button', { name: /Квартира на Ленина/ }).getByText('Просмотр'),
    ).toBeVisible();
  } finally {
    await execE2eSql(
      `UPDATE property_members SET role = 'full_access' WHERE id = '99999999-9999-4999-8999-999999999931'`,
    );
  }
  await page.reload();
  await expect(
    page.getByRole('button', { name: /Квартира на Ленина/ }).getByText('Редактирование'),
  ).toBeVisible();
});

test('пригласить в объект: мультичек гаража, шит роли, «Доступ выдан»; сид восстанавливается', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/participants/${MARIA_ID}`);

  await page.locator(header).getByRole('button', { name: 'Еще — действия с участником' }).click();
  await page.getByRole('menuitem', { name: 'Пригласить в объект' }).click();

  await expect(page.locator(header).getByText('Пригласить в объект')).toBeVisible();

  // Квартира уже выдана — в списке гараж и студия; «Все объекты» unchecked.
  const garageRow = page.getByRole('checkbox', { name: /Гараж на Садовой/ });
  await expect(garageRow).toBeVisible();
  await expect(page.getByRole('button', { name: /Квартира на Ленина/ })).toHaveCount(0);
  const allRow = page.getByRole('checkbox', { name: /Все объекты/ });
  await expect(allRow).toHaveAttribute('aria-checked', 'false');

  // Tri-state (2010-131721): выбран один из двух — «Все объекты» mixed.
  await garageRow.click();
  await expect(allRow).toHaveAttribute('aria-checked', 'mixed');

  const inviteButton = page.getByRole('button', { name: 'Пригласить' }).filter({ hasText: 'Пригласить' });
  await expect(inviteButton).toBeEnabled();
  await inviteButton.click();

  // Шит роли (макет 2010-131724): сегмент + кнопка «Пригласить».
  const sheet = page.getByRole('dialog');
  await sheet.getByRole('radio', { name: 'Редактирование' }).click();
  try {
    await sheet.getByRole('button', { name: 'Пригласить' }).click();

    // Попап «Доступ выдан» (2010-131859) на странице участника; грант
    // пришёл suspended — у сид-получателя нет подписки, тарифный слот
    // превышен (второй объект). Статус и пояснение — на уровне участника
    // (макет 2036-84861, правка приёмки #756): чип в шапке + жёлтая
    // карточка; ноги несут бейджи ролей.
    await expect(
      page.getByRole('dialog').locator('p', { hasText: 'Доступ выдан' }),
    ).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByText('Превышен лимит объектов').first()).toBeVisible();
    await expect(page.getByTestId('participant-limit-notice')).toBeVisible();
    await expect(
      page
        .getByTestId('participant-limit-notice')
        .getByText('Пользователь пока не может пользоваться вашим объектом'),
    ).toBeVisible();
    await expect(page.getByTestId('participant-limit-notice').getByText(/Попросите его освободить слот/)).toBeVisible();
    await expect(
      page.getByRole('button', { name: /Гараж на Садовой/ }).getByText('Редактирование'),
    ).toBeVisible();
    await expect(page.getByRole('button', { name: /Квартира на Ленина/ }).getByText('Редактирование')).toBeVisible();
  } finally {
    await execE2eSql(
      `DELETE FROM property_members WHERE property_id = '${GARAGE_ID}' AND user_id = '${MARIA_ID}'`,
    );
  }
  await page.reload();
  await expect(page.getByText('Доступно 1 объект')).toBeVisible();
});

test('отзыв из объекта: подтверждение, попап успеха, ряд исчезает; сид восстанавливается', async ({ page, seededUser }) => {
  // Нога на гараже — временная, создаётся и убирается этим тестом.
  await execE2eSql(
    `INSERT INTO property_members (id, property_id, user_id, role, granted_by) ` +
      `VALUES ('99999999-9999-4999-8999-999999999933', '${GARAGE_ID}', '${SERGEY_ID}', 'viewer', '11111111-1111-4111-8111-111111111111') ` +
      `ON CONFLICT (id) DO NOTHING`,
  );

  await openCabinetWithSeededSession(page, seededUser);
  await page.goto(`/participants/${SERGEY_ID}`);

  await page.getByRole('button', { name: /Гараж на Садовой/ }).click();
  await expect(page.locator(header).getByText('Права участника')).toBeVisible();

  await page.getByRole('button', { name: 'Отозвать доступ к объекту' }).click();

  const dialog = page.getByRole('dialog');
  await expect(dialog.getByText('Уверены, что хотите отозвать доступ?')).toBeVisible();
  try {
    await dialog.getByRole('button', { name: 'Отозвать' }).click();

    // Попап (2008-83135) на странице участника; нога на гараже исчезла.
    await expect(
      page.getByRole('dialog').locator('p', {
        hasText: 'У участника больше нет доступа к объекту',
      }),
    ).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('button', { name: /Гараж на Садовой/ })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /Квартира на Ленина/ })).toBeVisible();
  } finally {
    await execE2eSql(
      `DELETE FROM property_members WHERE id = '99999999-9999-4999-8999-999999999933'`,
    );
  }
});

test('кебаб «Отозвать доступ к объектам»: подтверждение, попап «Участник удален» на списке; сид восстанавливается', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  // Путь пользователя: список → ряд Сергея (нужна история — успех
  // возврата по канону goBack ведёт именно на список).
  await page.goto('/participants/list');
  await page.getByRole('button', { name: /Сергей Сидоров/ }).click();
  await expect(page.locator(header).getByText('Участник')).toBeVisible();

  await page.locator(header).getByRole('button', { name: 'Еще — действия с участником' }).click();
  await page.getByRole('menuitem', { name: 'Отозвать доступ к объектам' }).click();

  const dialog = page.getByRole('dialog');
  await expect(
    dialog.getByText('Отозвать у пользователя доступ ко всем вашим объектам?'),
  ).toBeVisible();
  try {
    await dialog.getByRole('button', { name: 'Отозвать и удалить' }).click();

    // Возврат на список «Участники» с попапом «Участник удален»
    // (2008-83716); Сергей из списка исчез, Мария осталась.
    await expect(page.locator(header).getByText('Ваши участники')).toBeVisible();
    await expect(
      page.getByRole('dialog').locator('p', { hasText: 'Участник удален' }),
    ).toBeVisible();
    // Модалка скрывает фон от a11y-дерева — список проверяем после закрытия.
    await page.keyboard.press('Escape');
    await expect(page.getByRole('button', { name: /Сергей Сидоров/ })).toHaveCount(0);
    await expect(page.getByRole('button', { name: /Мария Петрова/ })).toBeVisible();
  } finally {
    await execE2eSql(
      `INSERT INTO property_members (id, property_id, user_id, role, granted_by) ` +
        `VALUES ('99999999-9999-4999-8999-999999999932', '${APARTMENT_ID}', '${SERGEY_ID}', 'viewer', '11111111-1111-4111-8111-111111111111') ` +
        `ON CONFLICT (id) DO NOTHING`,
    );
  }
  await page.reload();
  await expect(page.getByRole('button', { name: /Сергей Сидоров/ })).toBeVisible();
});

test('pending-участник: deep-link по почте, бейдж «Приглашён», смена роли приглашения; сид восстанавливается', async ({ page, seededUser }) => {
  // Pending-приглашение на гараж (гейт тикета: состояния pending на тех же
  // экранах; права уходят в PATCH/DELETE /access/invitations/{id}).
  await execE2eSql(
    `INSERT INTO property_member_invitations (id, property_id, email, role, invited_by, last_sent_at) ` +
      `VALUES ('88888888-8888-4888-8888-888888888881', '${GARAGE_ID}', 'e2e-pending@example.com', 'viewer', '11111111-1111-4111-8111-111111111111', now()) ` +
      `ON CONFLICT (id) DO NOTHING`,
  );
  try {
    await openCabinetWithSeededSession(page, seededUser);
    // Идентификатор pending-участника — почта (контракт #693).
    await page.goto('/participants/e2e-pending%40example.com');

    await expect(page.locator(header).getByText('Участник', { exact: true }).first()).toBeVisible();
    // Имени нет — почта титул (контракт: «the email is the label»); чип
    // агрегата в шапке — «Приглашён» (#772: считанный partial/0 как
    // «Доступно 0 объектов» читался как отказ). first(): шапка в DOM раньше
    // списка, где у pending-ноги гаража свой такой же чип (ассерт ниже).
    await expect(page.getByText('e2e-pending@example.com').first()).toBeVisible();
    await expect(page.getByText('Приглашён').first()).toBeVisible();

    const garageRow = page.getByRole('button', { name: /Гараж на Садовой/ });
    await expect(garageRow.getByText('Приглашён')).toBeVisible();

    await garageRow.click();
    await expect(page.locator(header).getByText('Права участника')).toBeVisible();
    await expect(page.getByRole('radio', { name: 'Просмотр' })).toBeChecked();

    await page.getByRole('radio', { name: 'Редактирование' }).click();
    await expect(
      page.getByRole('dialog').locator('p', { hasText: 'Права изменены' }),
    ).toBeVisible();
    await page.keyboard.press('Escape');

    // Серверная правда: роль приглашения реально сменилась.
    expect(
      await execE2eSql(
        `SELECT count(*) FROM property_member_invitations WHERE id = '88888888-8888-4888-8888-888888888881' AND role = 'full_access'`,
      ),
    ).toBe('1');
  } finally {
    await execE2eSql(
      `DELETE FROM property_member_invitations WHERE id = '88888888-8888-4888-8888-888888888881'`,
    );
  }
});
