import {
  captureScreen,
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Экран «Пригласите участника» (карта #692, тикет #699): полноэкранный
// флоу приглашения из хаба — почта, сегмент роли, свёрнутый выбор
// объектов («Все N объектов» / выбранные), пикер «Выбрать объект» с
// tri-state «Все объекты», POST /participants/invite и попап «Участник
// приглашен» на цели возврата. Сид: владелец «Квартиры на Ленина»
// (33333333-…), «Гаража на Садовой» (44444444-…) и «Студии на Полевой»
// (46464646-…) — три объекта в скоупе; Мария Петрова (12111111-…21)
// active на квартире. Разрушающие тесты восстанавливают сид через
// execE2eSql (workers=1).

const header = 'header[aria-label="Навигация экрана"]';

const MARIA_ID = '12111111-1111-4111-8111-111111111121';
const GARAGE_ID = '44444444-4444-4444-8444-444444444444';
const STUDIO_ID = '46464646-4646-4646-8646-464646464646';
const INVITEE_EMAIL = 'e2e-invite@example.com';

test('экран по макету: заголовок, почта, сегмент роли, свёрнутые «Все 3 объекта», CTA погашен', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants/invite');

  await expect(page.getByRole('heading', { name: 'Пригласите участника' })).toBeVisible();
  await expect(
    page.getByText('Укажите электронную почту и выберите роль'),
  ).toBeVisible();

  // Плавающая подпись поля (макет 2010-134350) — a11y-имя инпута.
  const emailInput = page.getByRole('textbox', { name: 'Электронная почта' });
  await expect(emailInput).toBeVisible();

  // Сегмент роли: по умолчанию «Просмотр» (макеты 2008-46375).
  await expect(page.getByRole('radio', { name: 'Просмотр' })).toBeChecked();
  await expect(page.getByRole('radio', { name: 'Редактирование' })).not.toBeChecked();

  // Свёрнутый выбор: дефолт «Все N объектов» (снапшот трёх объектов).
  await expect(page.getByText('Все 3 объекта')).toBeVisible();
  await expect(page.getByText('Поделиться всеми объектами')).toBeVisible();

  // Почта пуста — CTA погашен (макет 2008-46375).
  const inviteButton = page.getByRole('button', { name: 'Пригласить' }).filter({ hasText: 'Пригласить' });
  await expect(inviteButton).toBeDisabled();

  await captureScreen(page, testInfo, 'participants-invite');
});

test('пикер «Выбрать объект»: tri-state «Все объекты», черновик коммитится «Выбрать», Esc откатывает', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants/invite');

  await page.getByText('Все 3 объекта').click();
  const picker = page.getByRole('dialog', { name: 'Выбрать объект' });
  await expect(picker).toBeVisible();

  // Дефолт «Все объекты» — чекбокс on (макет 2008-46627 — mixed после
  // частичного снятия).
  const allRow = picker.getByRole('checkbox', { name: /Все объекты/ });
  await expect(allRow).toHaveAttribute('aria-checked', 'true');

  const apartmentRow = picker.getByRole('checkbox', { name: /Квартира на Ленина/ });
  const garageRow = picker.getByRole('checkbox', { name: /Гараж на Садовой/ });
  const studioRow = picker.getByRole('checkbox', { name: /Студия на Полевой/ });
  await expect(apartmentRow).toHaveAttribute('aria-checked', 'true');
  await expect(garageRow).toHaveAttribute('aria-checked', 'true');

  // Снятие одного — «Все объекты» уходит в mixed.
  await apartmentRow.click();
  await expect(allRow).toHaveAttribute('aria-checked', 'mixed');

  // Esc закрывает без коммита — сводка остаётся «Все 3 объекта».
  await page.keyboard.press('Escape');
  await expect(picker).toHaveCount(0);
  await expect(page.getByText('Все 3 объекта')).toBeVisible();

  // Повторное открытие: черновик стартует от коммиченного выбора;
  // «Выбрать» коммитит — свёрнутыми остаются гараж и студия (квартира
  // снята). Пустой черновик «Выбрать» не подтверждает (гард тупика:
  // сводка — единственная точка входа в пикер).
  await page.getByText('Все 3 объекта').click();
  await expect(picker).toBeVisible();
  await apartmentRow.click();
  await garageRow.click();
  await studioRow.click();
  await expect(allRow).toHaveAttribute('aria-checked', 'false');
  await expect(picker.getByRole('button', { name: 'Выбрать' })).toBeDisabled();
  await apartmentRow.click();
  await expect(allRow).toHaveAttribute('aria-checked', 'mixed');
  await picker.getByRole('button', { name: 'Выбрать' }).click();
  await expect(picker).toHaveCount(0);
  // Коммичен один объект — свёрнутая строка «Квартира на Ленина».
  await expect(page.getByRole('button', { name: /Квартира на Ленина/ })).toBeVisible();
  await expect(page.getByRole('button', { name: /Гараж на Садовой/ })).toHaveCount(0);
  await expect(page.getByText('Все 3 объекта')).toHaveCount(0);

  // Некорректная почта при непустом поле: клик показывает ошибку, экран
  // остаётся на месте (макет ошибки не рисует — канон TextField error).
  await page.getByRole('textbox', { name: 'Электронная почта' }).fill('maksim');
  const inviteButton = page.getByRole('button', { name: 'Пригласить' }).filter({ hasText: 'Пригласить' });
  await expect(inviteButton).toBeEnabled();
  await inviteButton.click();
  await expect(page.getByText('Укажите корректную электронную почту')).toBeVisible();
});

test('приглашение незарегистрированной почты: все объекты, попап «Участник приглашен», pending-ряд на списке; сид восстанавливается', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);

  // Путь пользователя: список → CTA (нужна история — возврат goBack ведёт
  // на список с попапом).
  await page.goto('/participants/list');
  await page.getByRole('button', { name: 'Пригласить участника' })
    .filter({ hasText: 'Пригласить участника' })
    .click();
  await expect(page.getByRole('heading', { name: 'Пригласите участника' })).toBeVisible();

  await page.getByRole('textbox', { name: 'Электронная почта' }).fill(INVITEE_EMAIL);
  // Выбор по умолчанию — все объекты; роль «Просмотр».
  await page.getByRole('button', { name: 'Пригласить' })
    .filter({ hasText: 'Пригласить' })
    .click();

  // Попап «Участник приглашен» (2010-134458) на списке «Ваши участники».
  await expect(page.locator(header).getByText('Ваши участники')).toBeVisible();
  await expect(
    page.getByRole('dialog').locator('p', { hasText: 'Участник приглашен' }),
  ).toBeVisible();
  await page.keyboard.press('Escape');

  // Pending-ряд появился: почта — титул, чип «Доступно 0 объектов».
  const inviteeRow = page.getByRole('button', { name: new RegExp(INVITEE_EMAIL) });
  await expect(inviteeRow).toBeVisible();
  await expect(inviteeRow.getByText('Доступно 0 объектов')).toBeVisible();

  // Серверная правда: pending-приглашения на оба объекта снапшота.
  expect(
    await execE2eSql(
      `SELECT count(*) FROM property_member_invitations WHERE email = '${INVITEE_EMAIL}'`,
    ),
  ).toBe('3');

  await execE2eSql(`DELETE FROM property_member_invitations WHERE email = '${INVITEE_EMAIL}'`);
  await page.reload();
  await expect(page.getByRole('button', { name: new RegExp(INVITEE_EMAIL) })).toHaveCount(0);
});

test('хаб-приёмник: приглашение из хаба → возврат на /participants → попап «Участник приглашен» (цикл ×3); сид восстанавливается', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  // Мобильная канва — условия обхода #759: попап здесь рисует vaul-шит,
  // и именно на ней #771 поймал «невидимый» попап (шит за сгибом).
  await page.setViewportSize({ width: 375, height: 667 });

  // P2-хвост #759 / #771: вход во флоу — с хаба «Совместный доступ»
  // (CTA нижней панели), возврат goBack ведёт на хаб — попап рисует И
  // список, И хаб (#699). Раньше спека покрывала только список; на хабе
  // была поймана редкая потеря попапа (1 из 8 прогонов). Цикл повторов —
  // регрессионная сетка нестабильности; у каждого цикла своя почта,
  // иначе повторное приглашение уходит в granted=0 без попапа.
  await page.goto('/participants');

  // Самозаживление: провал предыдущего прогона до очистки оставил бы
  // pending-приглашения — повторное приглашение ушло бы в granted=0 без
  // попапа и отравило бы весь цикл.
  await execE2eSql(`DELETE FROM property_member_invitations WHERE email LIKE 'e2e-invite-hub-%'`);

  for (let cycle = 1; cycle <= 3; cycle++) {
    const email = `e2e-invite-hub-${cycle}@example.com`;

    await page.getByRole('button', { name: 'Пригласить участника' })
      .filter({ hasText: 'Пригласить участника' })
      .click();
    await expect(page.getByRole('heading', { name: 'Пригласите участника' })).toBeVisible();

    await page.getByRole('textbox', { name: 'Электронная почта' }).fill(email);
    // Выбор по умолчанию — все объекты; роль «Просмотр».
    await page.getByRole('button', { name: 'Пригласить' })
      .filter({ hasText: 'Пригласить' })
      .click();

    // Приёмник goBack — именно хаб, с попапом «Участник приглашен»
    // (2010-134458). URL с якорем конца строки отличает хаб от списка
    // (заголовок хаба не проверяем: открытая модалка прячет фон из
    // a11y-дерева — aria-modal).
    await expect(page).toHaveURL(/\/participants$/);
    await expect(
      page.getByRole('dialog').locator('p', { hasText: 'Участник приглашен' }),
    ).toBeVisible();
    // Попап целиком в вьюпорте: в DOM он был и когда шит терял
    // position:fixed (#771: tailwind-merge снимал fixed под relative
    // потребителя) и рисовался за сгибом — toBeVisible такой дефект не
    // ловит, прямоугольник ловит. Поллинг — шит въезжает 0.5s, ждём
    // завершение слайда, а не первый кадр.
    const viewport = page.viewportSize();
    await expect
      .poll(
        async () => {
          const box = await page.getByRole('dialog').boundingBox();
          return box === null ? Number.POSITIVE_INFINITY : box.y + box.height;
        },
        { timeout: 3_000 },
      )
      .toBeLessThanOrEqual(viewport ? viewport.height : Number.POSITIVE_INFINITY);
    expect(
      await page.getByRole('dialog').boundingBox().then((box) => (box === null ? null : box.y)),
    ).toBeGreaterThanOrEqual(0);
    if (cycle === 1) {
      await captureScreen(page, testInfo, 'participants-invite-hub-popup');
    }
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog')).toHaveCount(0);

    // Серверная правда: pending-приглашения на все 3 объекта снапшота;
    // чистим сразу, чтобы циклы были независимы.
    expect(
      await execE2eSql(
        `SELECT count(*) FROM property_member_invitations WHERE email = '${email}'`,
      ),
    ).toBe('3');
    await execE2eSql(`DELETE FROM property_member_invitations WHERE email = '${email}'`);
  }

  await page.reload();
  expect(
    await execE2eSql(`SELECT count(*) FROM property_member_invitations WHERE email LIKE 'e2e-invite-hub-%'`),
  ).toBe('0');
});

test('приглашение зарегистрированной почты на один объект: suspended-слот, чип «Превышен лимит объектов»; сид восстанавливается', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);

  await page.goto('/participants/list');
  await page.getByRole('button', { name: 'Пригласить участника' })
    .filter({ hasText: 'Пригласить участника' })
    .click();

  await page.getByRole('textbox', { name: 'Электронная почта' }).fill('e2e-member@example.com');

  // Снимаем квартиру — приглашаем на гараж и студию (черновик пикера).
  await page.getByText('Все 3 объекта').click();
  const picker = page.getByRole('dialog', { name: 'Выбрать объект' });
  await picker.getByRole('checkbox', { name: /Квартира на Ленина/ }).click();
  await picker.getByRole('button', { name: 'Выбрать' }).click();
  await expect(page.getByText('Все 3 объекта')).toHaveCount(0);
  await expect(page.getByRole('button', { name: /Гараж на Садовой/ })).toBeVisible();

  await page.getByRole('button', { name: 'Пригласить' })
    .filter({ hasText: 'Пригласить' })
    .click();

  // granted=2 (suspended — у Марии нет подписки, тарифный слот превышен;
  // грабля #698) — попап успеха, возврат на список.
  await expect(
    page.getByRole('dialog').locator('p', { hasText: 'Участник приглашен' }),
  ).toBeVisible();
  await page.keyboard.press('Escape');

  // Чип агрегата Марии сменился: активна квартира + suspended гараж и
  // студия — «Превышен лимит объектов» (макет 2036-84861).
  const mariaRow = page.getByRole('button', { name: /Мария Петрова/ });
  await expect(mariaRow.getByText('Превышен лимит объектов')).toBeVisible();

  // Серверная правда: suspended-членство на гараже.
  expect(
    await execE2eSql(
      `SELECT count(*) FROM property_members ` +
        `WHERE property_id = '${GARAGE_ID}' AND user_id = '${MARIA_ID}' AND role = 'viewer' AND status = 'suspended'`,
    ),
  ).toBe('1');

  await execE2eSql(
    `DELETE FROM property_members WHERE user_id = '${MARIA_ID}' AND property_id IN ('${GARAGE_ID}', '${STUDIO_ID}')`,
  );
  await page.reload();
  await expect(mariaRow.getByText('Превышен лимит объектов')).toHaveCount(0);
});

test('своя почта — ошибка бэка «Нельзя добавить себя участником», экран остаётся на месте', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/participants/invite');

  await page.getByRole('textbox', { name: 'Электронная почта' }).fill(seededUser.email);
  await page.getByRole('button', { name: 'Пригласить' })
    .filter({ hasText: 'Пригласить' })
    .click();

  // Семантический 400 (#694): текст проблемы бэка показывается под полем.
  await expect(page.getByText('Нельзя добавить себя участником')).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Пригласите участника' })).toBeVisible();
});
