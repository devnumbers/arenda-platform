import type { Page } from '@playwright/test';
import { execE2eSql, expect, openCabinetWithSeededSession, test } from './fixtures';

// Hero-карточка «Тарифа» кликается целиком (решение владельца 19.10.2026;
// прежний канон «карточка целиком ссылкой не является» отменён в том же
// дереве): stretched-link на ссылке-заголовке растягивает зону клика на
// всю карточку — поле паддинга вокруг строк ведёт туда же, куда заголовок
// («О тарифе»); нижняя строка-ссылка остаётся самостоятельной зоной
// поверх оверлея (для grace это «Оплатить тариф» с другим назначением).
// Спека кликает по геометрии карточки, а не по текстовым локаторам —
// именно «вся карточка» и есть контракт.
test.describe('герой «Тарифа» — кликабельная карточка', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  async function openTariffCard(page: Page, seededUser: Parameters<typeof openCabinetWithSeededSession>[1]) {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/tariff');
    // Ссылка с h2 внутри — заголовок героя; карточка — дед (родитель
    // обёртки контента): rounded-32 + p-8 + фото-фон, у паддинга которой
    // и проверяем мёртвые зоны. Текст не фиксируем: сид владельца —
    // тариф pro, имя решает бекенд.
    const heroLink = page.locator('a').filter({ has: page.locator('h2') });
    await expect(heroLink).toBeVisible();
    return heroLink.locator('xpath=../..');
  }

  test('клик в поле паддинга карточки ведёт на «О тарифе»', async ({ page, seededUser }) => {
    const card = await openTariffCard(page, seededUser);
    const box = await card.boundingBox();
    if (!box) throw new Error('карточка героя без геометрии');

    // Нижнее поле паддинга (32) под последней строкой — раньше мёртвая зона.
    await page.mouse.click(box.x + box.width / 2, box.y + box.height - 12);
    await expect(page).toHaveURL(/\/profile\/tariff\/about$/);
  });

  test('клик в верхний правый угол карточки ведёт на «О тарифе»', async ({ page, seededUser }) => {
    const card = await openTariffCard(page, seededUser);
    const box = await card.boundingBox();
    if (!box) throw new Error('карточка героя без геометрии');

    // Угол возле шеврона, но за пределами строки заголовка.
    await page.mouse.click(box.x + box.width - 12, box.y + 12);
    await expect(page).toHaveURL(/\/profile\/tariff\/about$/);
  });

  test('нижняя строка «О тарифе» остаётся самостоятельной зоной', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/profile/tariff');

    await page.getByRole('link', { name: 'О тарифе' }).click();
    await expect(page).toHaveURL(/\/profile\/tariff\/about$/);
  });

  // Grace: у нижней строки другое назначение — «Оплатить тариф» на флоу
  // оплаты; оверлей карточки не должен его глушить. Сид статуса — как в
  // tariff-change.spec (подписка сида общая, восстанавливаем тут же:
  // workers 1, но лимиты объектов других спек зависят от статуса).
  test('grace: «Оплатить тариф» ведёт на оплату поверх оверлея', async ({ page, seededUser }) => {
    const SEEDED_SUBSCRIPTION_ID = '99999999-9999-4999-8999-999999999901';
    await execE2eSql(
      `UPDATE user_subscriptions SET status = 'grace', valid_until = now() + interval '5 days' WHERE id = '${SEEDED_SUBSCRIPTION_ID}'`,
    );
    try {
      await openCabinetWithSeededSession(page, seededUser);
      await page.goto('/profile/tariff');
      const cta = page.getByRole('link', { name: 'Оплатить тариф' });
      await expect(cta).toBeVisible();

      await cta.click();
      await expect(page).toHaveURL(/\/profile\/tariff\/change$/);
    } finally {
      await execE2eSql(
        `UPDATE user_subscriptions SET status = 'active', valid_until = NULL WHERE id = '${SEEDED_SUBSCRIPTION_ID}'`,
      );
    }
  });
});
