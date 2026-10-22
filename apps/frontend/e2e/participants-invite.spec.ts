import type { Page } from '@playwright/test';
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
// tri-state «Все объекты», POST /participants/invite и тост «Участник
// приглашен» на цели возврата. Сид: владелец «Квартиры на Ленина»
// (33333333-…), «Гаража на Садовой» (44444444-…) и «Студии на Полевой»
// (46464646-…) — три объекта в скоупе; Мария Петрова (12111111-…21)
// active на квартире. Разрушающие тесты восстанавливают сид через
// execE2eSql (workers=1).

const GARAGE_ID = '44444444-4444-4444-8444-444444444444';
const STUDIO_ID = '46464646-4646-4646-8646-464646464646';
const INVITEE_EMAIL = 'e2e-invite@example.com';
// Свободный сид-юзер (без подписки, лимит исчерпан) для suspended/slot-
// сценариев приглашений — у Марии и Сергея подписки pro для гейтов
// платных разделов (#997).
const LIMITED_EMAIL = 'e2e-limited@example.com';
const LIMITED_ID = '14111111-1111-4111-8111-111111111141';

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
  await expect(page.getByRole('radio', { name: 'Управление' })).not.toBeChecked();

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

test('приглашение незарегистрированной почты: все объекты, тост «Участник приглашен», pending-ряд на списке; сид восстанавливается', async ({ page, seededUser }) => {
  await openCabinetWithSeededSession(page, seededUser);

  // Путь пользователя: раздел → кебаб → «Пригласить участника» (CTA
  // нижней панели снесена с хабом, D1 #1313; нужна история — возврат
  // goBack ведёт на раздел с тостом).
  await page.goto('/participants');
  await openSectionInvite(page);
  await expect(page.getByRole('heading', { name: 'Пригласите участника' })).toBeVisible();

  await page.getByRole('textbox', { name: 'Электронная почта' }).fill(INVITEE_EMAIL);
  // Выбор по умолчанию — все объекты; роль «Просмотр».
  try {
    await page.getByRole('button', { name: 'Пригласить' })
      .filter({ hasText: 'Пригласить' })
      .click();

    // Тост «Участник приглашен» (2010-134458) — в момент успеха, виден на
    // разделе после возврата (тост вместо успех-попапа, решение владельца
    // 19.10.2026); закрываем кнопкой, чтобы не перехватывал клики (канон
    // contacts-спек). URL с якорем конца строки отличает раздел от
    // вложенных маршрутов.
    await expect(page).toHaveURL(/\/participants$/);
    const toast = page.locator('.Toastify__toast', { hasText: 'Участник приглашен' });
    await expect(toast).toBeVisible();
    await toast.getByRole('button', { name: 'Закрыть' }).click();

    // Pending-ряд появился: титул «Пользователь», почта подзаголовком
    // (аменда владельца 22.10.2026, макет 3313-77128), бейдж «Аккаунт не
    // создан» (D3 #1313: у pending-агрегата ровно этот статус).
    const inviteeRow = page.getByRole('button', { name: new RegExp(INVITEE_EMAIL) });
    await expect(inviteeRow).toBeVisible();
    await expect(inviteeRow.getByText('Пользователь', { exact: true })).toBeVisible();
    await expect(inviteeRow.getByText('Аккаунт не создан')).toBeVisible();

    // Серверная правда: pending-приглашения на все 3 объекта снапшота.
    expect(
      await execE2eSql(
        `SELECT count(*) FROM property_member_invitations WHERE email = '${INVITEE_EMAIL}'`,
      ),
    ).toBe('3');
  } finally {
    await execE2eSql(`DELETE FROM property_member_invitations WHERE email = '${INVITEE_EMAIL}'`);
  }
  await page.reload();
  await expect(page.getByRole('button', { name: new RegExp(INVITEE_EMAIL) })).toHaveCount(0);
});

test('приёмник-раздел: приглашение из раздела → возврат на /participants → тост «Участник приглашен» (цикл ×3); сид восстанавливается', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  // Мобильная канва — тост сверху по центру, флоу приглашения проверяем
  // на телефоне.
  await page.setViewportSize({ width: 375, height: 667 });

  // P2-хвост #759 / #771: вход во флоу — из раздела «Участники» (кебаб;
  // CTA нижней панели снесена с хабом, D1 #1313), возврат goBack ведёт на
  // раздел. Цикл повторов — регрессионная сетка нестабильности; у каждого
  // цикла своя почта, иначе повторное приглашение уходит в granted=0
  // без тоста и без возврата.
  await page.goto('/participants');

  // Самозаживление: провал предыдущего прогона до очистки оставил бы
  // pending-приглашения — повторное приглашение ушло бы в granted=0 и
  // отравило бы весь цикл.
  await execE2eSql(`DELETE FROM property_member_invitations WHERE email LIKE 'e2e-invite-hub-%'`);

  for (let cycle = 1; cycle <= 3; cycle++) {
    const email = `e2e-invite-hub-${cycle}@example.com`;

    await openSectionInvite(page);
    await expect(page.getByRole('heading', { name: 'Пригласите участника' })).toBeVisible();

    await page.getByRole('textbox', { name: 'Электронная почта' }).fill(email);
    // Выбор по умолчанию — все объекты; роль «Просмотр».
    await page.getByRole('button', { name: 'Пригласить' })
      .filter({ hasText: 'Пригласить' })
      .click();

    // Приёмник goBack — именно хаб, с тостом «Участник приглашен»
    // (2010-134458; тост вместо успех-попапа, решение владельца
    // 19.10.2026). URL с якорем конца строки отличает хаб от списка.
    await expect(page).toHaveURL(/\/participants$/);
    const toast = page.locator('.Toastify__toast', { hasText: 'Участник приглашен' });
    await expect(toast).toBeVisible();
    if (cycle === 1) {
      await captureScreen(page, testInfo, 'participants-invite-hub-toast');
    }
    await toast.getByRole('button', { name: 'Закрыть' }).click();
    // Ждём полного ухода тоста: на локальном стеке следующий цикл
    // успевает показать новый до конца 350ms exit-анимации предыдущего —
    // два узла под общим локатором дают strict violation.
    await expect(page.locator('.Toastify__toast')).toHaveCount(0);

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

  await page.goto('/participants');
  await openSectionInvite(page);

  await page.getByRole('textbox', { name: 'Электронная почта' }).fill(LIMITED_EMAIL);

  // Снимаем квартиру — приглашаем на гараж и студию (черновик пикера).
  await page.getByText('Все 3 объекта').click();
  const picker = page.getByRole('dialog', { name: 'Выбрать объект' });
  await picker.getByRole('checkbox', { name: /Квартира на Ленина/ }).click();
  await picker.getByRole('button', { name: 'Выбрать' }).click();
  await expect(page.getByText('Все 3 объекта')).toHaveCount(0);
  await expect(page.getByRole('button', { name: /Гараж на Садовой/ })).toBeVisible();

  const limitedRow = page.getByRole('button', { name: /Анна Лимитова/ });
  try {
    await page.getByRole('button', { name: 'Пригласить' })
      .filter({ hasText: 'Пригласить' })
      .click();

    // granted=2 (suspended — у Анны нет подписки, тарифный слот превышен;
    // грабля #698) — тост успеха, возврат на список; закрываем кнопкой,
    // чтобы не перехватывал клики.
    const toast = page.locator('.Toastify__toast', { hasText: 'Участник приглашен' });
    await expect(toast).toBeVisible();
    await toast.getByRole('button', { name: 'Закрыть' }).click();

    // Чип агрегата Анны сменился: активна квартира + suspended гараж и
    // студия — «Превышен лимит объектов» (макет 2036-84861).
    await expect(limitedRow.getByText('Превышен лимит объектов')).toBeVisible();

    // Серверная правда: suspended-членство на гараже.
    expect(
      await execE2eSql(
        `SELECT count(*) FROM property_members ` +
          `WHERE property_id = '${GARAGE_ID}' AND user_id = '${LIMITED_ID}' AND role = 'viewer' AND status = 'suspended'`,
      ),
    ).toBe('1');
  } finally {
    await execE2eSql(
      `DELETE FROM property_members WHERE user_id = '${LIMITED_ID}' AND property_id IN ('${GARAGE_ID}', '${STUDIO_ID}')`,
    );
  }
  await page.reload();
  await expect(limitedRow.getByText('Превышен лимит объектов')).toHaveCount(0);
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

/** Вход в форму приглашения из раздела «Участники»: кебаб → «Пригласить
 * участника» (D1 #1313 — CTA нижней панели снесена с хабом). Radix-меню
 * под нагрузкой может не открыться с первого клика — ретраим до пункта. */
async function openSectionInvite(page: Page): Promise<void> {
  await expect(async () => {
    await page
      .getByTestId('participants-section-kebab-row')
      .getByRole('button', { name: 'Еще — действия со списком' })
      .click();
    await expect(page.getByRole('menuitem', { name: 'Пригласить участника' })).toBeVisible();
  }).toPass({ timeout: 10_000 });
  await page.getByRole('menuitem', { name: 'Пригласить участника' }).click();
}
