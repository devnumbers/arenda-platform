import {
  expect,
  openCabinetWithSeededSession,
  SEEDED_GARAGE_PROPERTY_ID,
  test,
} from './fixtures';

// Глобальный вход визарда платежа — /payments/new (решение владельца
// 23.10.2026, аналог /operations/new #570): «+» хаба «Платежей» открывает
// модалку выбора типа без черновой фазы, из неё — 6 шагов с шагом «Выбрать
// объект» сразу после категории; объектный вход (5 шагов без объекта) —
// канон payments-wizard.spec.ts. Регресс полного пути закрывает раскладку
// WIZARD_STEP_LAYOUTS.global: переход категории → объекта жил только в
// слое экрана и приёмкой был пойман проскоченным шагом.

type PaymentFromApi = {
  id: string;
  title: string;
  amountKopecks: number;
  type: string;
  autoPay: boolean;
  recurrence: { kind: string; daysOfMonth?: number[]; lastDay?: boolean };
};

const GARAGE_PAYMENTS_URL = `/properties/${SEEDED_GARAGE_PROPERTY_ID}/payments`;

/** Прочитать список платежей объекта из API тем же сеансом (куки контекста). */
async function fetchPayments(
  page: Parameters<typeof openCabinetWithSeededSession>[0],
  propertyId: string,
): Promise<PaymentFromApi[]> {
  const response = await page.request.get(`/api/properties/${propertyId}/payments`);
  expect(response.ok()).toBe(true);
  const payload = (await response.json()) as { items: PaymentFromApi[] };
  return payload.items;
}

test.describe('глобальный визард платежа (/payments/new)', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('«+» хаба → выбор типа → 6 шагов с «Выбрать объект» после категории', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/payments');
    await page.getByTestId('payments-create').click();

    // Модалка выбора типа — без черновой фазы.
    await expect(
      page.getByRole('heading', { name: 'Выберите тип платежа' }),
    ).toBeVisible();
    await page.getByRole('button', { name: /^Платеж/ }).click();
    await expect(page).toHaveURL(/\/payments\/new\?type=payment$/);
    await expect(page.getByText('шаг 1 из 6')).toBeVisible();

    // Шаг 1 — категория.
    await page.getByRole('button', { name: 'Интернет' }).click();
    await page.getByRole('button', { name: 'Продолжить' }).click();

    // Шаг 2 — «Выбрать объект» (только у глобального входа): дефолт —
    // верхняя строка списка (в сиде запиненных нет — первая строка
    // справочника, «Квартира на Ленина»), панель «Продолжить» видна
    // сразу; явный выбор пользователя сильнее дефолта.
    await expect(page.getByRole('heading', { name: 'Выберите объект' })).toBeVisible();
    await expect(page.getByText('шаг 2 из 6')).toBeVisible();
    await expect(
      page.getByRole('radio', { name: /Квартира на Ленина/ }),
    ).toHaveAttribute('aria-checked', 'true');
    await expect(page.getByRole('button', { name: 'Продолжить' })).toBeVisible();
    await page
      .getByRole('radio', { name: /Гараж на Садовой/ })
      .click();
    await expect(
      page.getByRole('radio', { name: /Гараж на Садовой/ }),
    ).toHaveAttribute('aria-checked', 'true');
    await page.getByRole('button', { name: 'Продолжить' }).click();

    // Шаги 3–5 — канон объектного визарда, номера сдвинуты на единицу.
    await expect(page.getByText('шаг 3 из 6')).toBeVisible();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await page.getByRole('button', { name: 'Каждый месяц' }).click();
    await page.getByRole('button', { name: '20', exact: true }).first().click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    await expect(page.getByRole('button', { name: 'Далее' })).toBeVisible();
    await page.getByRole('button', { name: 'Далее' }).click();

    // Шаг 6 — сумма; сабмит глобального входа создаёт правило выбранного
    // объекта (POST /properties/{гараж}/payments).
    await expect(page.getByText('шаг 6 из 6')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Создать платеж' })).toBeDisabled();
    await page.getByRole('textbox', { name: 'Сумма' }).fill('800');
    await page.getByRole('radio', { name: 'Расход' }).click();
    await page.getByRole('button', { name: 'Создать платеж' }).click();

    // Успех: название подставлено из категории (поле названия не трогали).
    await expect(page.getByRole('heading', { name: /Вы создали платеж/ })).toBeVisible();
    await expect(page.getByText(/Первый платеж .*, далее каждый месяц 20 числа/)).toBeVisible();

    // Серверная правда: правило в списке выбранного объекта (гаража), не
    // квартиры.
    const items = await fetchPayments(page, SEEDED_GARAGE_PROPERTY_ID);
    const created = items.find((payment) => payment.title === 'Интернет');
    expect(created).toBeDefined();
    expect(created?.amountKopecks).toBe(80000);
    expect(created?.type).toBe('expense');
    expect(created?.autoPay).toBe(false);
    expect(created?.recurrence).toStrictEqual({
      kind: 'monthly',
      daysOfMonth: [20],
      lastDay: false,
    });

    // Закрытие успеха возвращает на хаб «Платежей» (источник входа).
    await page.getByRole('button', { name: 'Хорошо, закрыть' }).click();
    await expect(page).toHaveURL(/\/payments$/);
  });

  test('прямой вход без типа возвращает на хаб', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/payments/new');
    await expect(page).toHaveURL(/\/payments$/);
  });

  test('объектный вход не получил шаг объекта (канон 5 шагов)', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto(`${GARAGE_PAYMENTS_URL}/new?type=payment`);
    await expect(page.getByText('шаг 1 из 5')).toBeVisible();
    await page.getByRole('button', { name: 'Домофон' }).click();
    await page.getByRole('button', { name: 'Продолжить' }).click();
    // Сразу название: объекта-шага у объектного входа нет.
    await expect(page.getByRole('heading', { name: 'Назовите платеж' })).toBeVisible();
    await expect(page.getByText('шаг 2 из 5')).toBeVisible();
  });
});
