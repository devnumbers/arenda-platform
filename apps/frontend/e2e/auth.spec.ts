import { loginViaUi, test } from './fixtures';

// Smoke of the real login flow (ticket #456): the seeded owner enters the
// phone, the code arrives in the backend log (fake email sender), and the
// cabinet opens on the properties list. Validates the whole chain — UI
// form, /api proxy, auth endpoints, session cookie, middleware redirect —
// before payment screens start relying on this infrastructure.
test('вход по коду из письма ведёт в кабинет', async ({ page, seededUser }) => {
  await loginViaUi(page, seededUser);
});
