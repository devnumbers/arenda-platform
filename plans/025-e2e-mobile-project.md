# 025 — e2e: мобильный проект Playwright (тач-эмуляция), опт-ин

- **Status**: TODO
- **Commit**: 6ea8bc4ed
- **Severity**: LOW
- **Category**: mobile-native §11 (тестовая инфраструктура; честная граница — feel не воспроизводится эмуляцией); карта #1416 / тикет #1421
- **Estimated scope**: 3 файла (playwright.config.ts, раннер, новый спека-смоук)

## Problem

Playwright-набор живёт в одном проекте `chromium` = `devices['Desktop Chrome']`
(`playwright.config.ts:39-44`) — ни мобильного вьюпорта по умолчанию, ни
тач-событий (`hasTouch`), хотя продукт — мобайл-первый PWA. Раннер зовёт
`npx playwright test "$@"` без пина проекта
(`tools/e2e/frontend/run-frontend-e2e.sh:293`), поэтому «просто добавить
проект» удвоило бы время каждого прогона: набор — `workers: 1`,
`fullyParallel: false` на общем засеянном бэкенде (`playwright.config.ts:27-28`).

Граница канона §11 честна: эмуляция не воспроизводит sticky hover, safe-area,
клавиатуру и feel — реальное железо остаётся за владельцем (протокол карты
#1416). Проект нужен для того, что эмуляция честно покрывает: тач-события
(тап вместо мыши), мобильный вьюпорт/UA-класс устройства, регресс тач-навигации
хрома.

## Target

1. Конфиг — второй проект рядом с первым:

```ts
// apps/frontend/playwright.config.ts — target
projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      // Мобильный проект (карта #1416, тикет #1421): тач-события и
      // мобильный вьюпорт для смоука хрома. В дефолтный прогон не входит
      // (раннер пинит проект через E2E_PROJECT) — workers:1 и общий
      // засеянный бэкенд удвоили бы каждый прогон; полный мобильный
      // прогон — решение владельца после приёмки смоука.
      name: 'mobile-chromium',
      use: { ...devices['Pixel 7'] },
    },
],
```

2. Раннер — пин проекта с Opt-out через env:

```bash
# tools/e2e/frontend/run-frontend-e2e.sh:293 — target (внутри существующего
# env-префикса; сама строка вызова одна)
  npx playwright test --project="${E2E_PROJECT:-chromium}" "$@"
```

Мобильный прогон: `E2E_PROJECT=mobile-chromium make frontend-e2e TESTS='-g Мобильный хром'`
(переменная проходит через `SOURCE_ROOT_ENV`-механику таргета — обычный
shell-env; если она не долетает, экспортировать перед `make`).

3. Тач-смоук — `apps/frontend/e2e/mobile-chrome.spec.ts`:

```ts
import {
  expect,
  openCabinetWithSeededSession,
  test,
} from './fixtures';

// Мобильный смоук хрома (карта #1416, тикет #1421, проект mobile-chromium):
// нижняя навигация работает от настоящих touch-событий (hasTouch проекта
// Pixel 7), не от мыши. Локаторы — по a11y-ролям; тап — координатами
// элемента через page.touchscreen (click шлёт mouse-события и тач не
// проверяет).

/** Тап по центру элемента настоящим тачем. */
async function tapCenter(
  page: import('@playwright/test').Page,
  locator: import('@playwright/test').Locator,
): Promise<void> {
  const box = await locator.boundingBox();
  if (box === null) {
    throw new Error('цель тача не найдена');
  }
  await page.touchscreen.tap(box.x + box.width / 2, box.y + box.height / 2);
}

test.describe('Мобильный хром', () => {
  test('нижняя навигация от тача: таб «Уведомления» и шит «Еще»', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);

    const nav = page.locator('nav[aria-label="Нижняя навигация"]');
    await expect(nav).toBeVisible();

    await tapCenter(page, nav.getByRole('link', { name: 'Уведомления' }));
    await expect(page).toHaveURL(/\/notifications/);

    await tapCenter(page, nav.getByRole('button', { name: 'Еще' }));
    await expect(page.getByRole('dialog', { name: 'Навигация' })).toBeVisible();

    await tapCenter(page, nav.getByRole('button', { name: 'Еще' }));
    await expect(page.getByRole('dialog', { name: 'Навигация' })).toBeHidden();
  });
});
```

(Точное a11y-имя диалога шита сверить по `more-sheet.tsx` на момент
исполнения; если роль/имя другие — поправить локатор, не менять подход.)

## Repo conventions to follow

- Сид и вход — через `fixtures` (`openCabinetWithSeededSession`), локаторы
  по a11y-ролям, кубики через `boundingBox()` с явным `throw` (образцы —
  `e2e/scroll-isolation.spec.ts` из плана 005, `e2e/frame-shell.spec.ts:231`).
- Таймзона/прочие глобальные настройки конфига не трогаются — проект
  наследует use-блок.
- Дефолтный прогон десктопным остаётся: время `make frontend-e2e` не меняется.

## Steps

1. `playwright.config.ts`: добавить проект `mobile-chromium` (Target п.1).
2. `tools/e2e/frontend/run-frontend-e2e.sh:293`: пин
   `--project="${E2E_PROJECT:-chromium}"` (Target п.2).
3. Создать `apps/frontend/e2e/mobile-chrome.spec.ts` (Target п.3).

## Boundaries

- Do NOT переводить существующие спеки на тач и мобильный вьюпорт.
- Do NOT включать мобильный проект в дефолтный прогон и в CI.
- Do NOT передавать `--project` вручную через `TESTS` — раннер уже пинит
  проект, второй `--project` в аргументах ор-ится с пином и прогонит оба;
  выбор проекта — только через `E2E_PROJECT`.
- Do NOT выбирать другое устройство без нужды — Pixel 7 уже в
  `devices`; смысл — тач-класс, не конкретная модель.
- Do NOT добавлять проект в `grep`-фильтры Makefile — запуск только через
  `E2E_PROJECT`.
- Если строка раннера `:293` не совпадает с цитатой (дрейф) — СТОП и отчёт.

## Verification

- **Mechanical**: `npx tsc --noEmit` (конфиг типизирован) — зелёный;
  `make frontend-e2e` (Docker, дефолт) — полный набор зелёный, по времени
  как до плана (второй проект не стартует);
  `E2E_PROJECT=mobile-chromium make frontend-e2e TESTS='-g Мобильный хром'` —
  смоук зелёный (один тест, тач-события в trace — `pointerType: touch`).
- **Feel check**: за владельцем на реальном iPhone/Android по протоколу
  карты #1416 — эмуляция feel не воспроизводит (канон §11).
- **Done when**: дефолтный прогон зелёный и не подорожал; смоук под
  `E2E_PROJECT=mobile-chromium` зелёный.
