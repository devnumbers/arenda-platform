import {
  captureScreen,
  expect,
  openCabinetWithSeededSession,
  SEEDED_PROPERTIES,
  test,
} from './fixtures';

// Screen smoke of the properties list (ticket #456): the canonical example
// every future screen spec follows — enter the cabinet on the seeded
// session, assert the seeded data on screen, capture a screenshot artifact
// for the Figma comparison (decision of 2026-08-25, spec #453 revision).
// The UI-login path into the same screen lives in auth.spec.ts.

test('список объектов: карточки сид-объектов и скриншот экрана', async ({ page, seededUser }, testInfo) => {
  await openCabinetWithSeededSession(page, seededUser);
  await page.goto('/properties');

  await expect(page.getByRole('heading', { name: 'Мои объекты' })).toBeVisible();
  for (const name of SEEDED_PROPERTIES) {
    await expect(page.getByText(name, { exact: true })).toBeVisible();
  }

  await captureScreen(page, testInfo, 'properties');
});
