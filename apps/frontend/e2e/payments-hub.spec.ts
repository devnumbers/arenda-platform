import {
  expect,
  openCabinetWithSeededSession,
  openCabinetWithSessionToken,
  seededViewerSessionToken,
  test,
} from './fixtures';

// Хаб «Платежи» — первый блок по макету (3226-75059, хедер 3229-94522,
// тикет #1235): «+» создания платежа в ряду заголовка и в компакт-баре;
// цель «+» — paymentHubAddTarget (единственный редактируемый объект — шит
// единого входа, несколько — страница «Объекты»); у зрителя «+» нет (#703).
// Состав секций и пилюля — наследие #578 (ширины — column-width.spec.ts).

test.describe('хаб «Платежи» — «+» создания', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('владелец с несколькими редактируемыми объектами: «+» ведёт на выбор объекта', async ({
    page,
    seededUser,
  }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/payments');

    await expect(page.getByTestId('payments-create')).toBeVisible();
    await page.getByTestId('payments-create').click();
    await expect(page).toHaveURL(/\/payments\/objects$/);
  });

  test('чистый зритель: «+» нет ни в ряду заголовка, ни в компакт-баре (#703)', async ({
    page,
  }) => {
    await openCabinetWithSessionToken(page, seededViewerSessionToken());
    await page.goto('/payments');

    await expect(
      page.getByRole('heading', { name: 'Платежи', exact: true }),
    ).toBeVisible();
    await expect(page.getByTestId('payments-create')).toHaveCount(0);
    await expect(page.getByTestId('payments-create-compact')).toHaveCount(0);
  });
});
