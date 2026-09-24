import {
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Макет 2333-180696 (сверка #827): мастер выключен глушит только доставку —
// тумблеры категорий остаются кликабельными и хранят значения. На макете
// включённый пуш-тумблер стоит в полную непрозрачность под выключенным
// мастером (аннотация «Пуш включился после разрешения уведомлений»);
// прежнее затемнение/блокировку из неверного чтения макета сняли.
// Дальше клика спека не идёт: в headless chromium разрешение на уведомления
// автогрантовано, и включение категории уходит в тихую подписку, а не в шит.

test.describe('настройки уведомлений — пуш-колонка при выключенном мастере #827', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('мастер выключен — тумблер категории активен, строка не затемнена', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/notifications');

    const master = page.getByRole('switch', {
      name: 'Получать пуш-уведомления на этом устройстве',
    });
    await expect(master).toBeVisible();
    await master.click();

    const rentalPush = page.getByRole('switch', { name: 'Пуш-уведомления — Аренда' });
    await expect(rentalPush).toBeEnabled();
    // строка в полную непрозрачность — затемнения нет
    await expect(rentalPush.locator('xpath=..')).toHaveCSS('opacity', '1');
  });
});
