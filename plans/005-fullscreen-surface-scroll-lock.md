# 005 — Лок фона фуллскрин-поверхностей (шит фильтров, пикер объектов, пикер дня, поиск адреса)

- **Status**: TODO
- **Commit**: 7a963de1a
- **Severity**: HIGH
- **Category**: Изоляция скроллеров — слой «фуллскрин-поверхность» (конвенция «Мульти-скролл», карта #1416 / тикет #1417; mobile-native §2 — страница под поверхностью не живёт своей жизнью; прецедент канона — `useBodyScrollLock`, решение владельца 2026-10-13)
- **Estimated scope**: 6 файлов (4 поверхности, `useBodyScrollLock.ts`, новый e2e-спека), ~150 строк с гейтом

## Problem

Полноэкранная поверхность — `fixed inset-0` поверх страницы; страница под ней
остаётся скроллируемой, и жест у края внутренней ленты поверхности уезжает
в цепочку скролла фоновой страницы. Под непрозрачной поверхностью движение
невидимо, но позицию чтения уводит: самый частый случай — шит фильтров
истории, при закрытии лента оказывается «уехавшей» (тикет #1417).

Для календарных пикеров это уже починено каноном `useBodyScrollLock`
(решение владельца 2026-10-13), а четыре поверхности того же класса —
все на `fullscreenSurfaceClass` — канон пропустили:

1. `apps/frontend/widgets/history/ui/history-filters-sheet.tsx` — шит
   «Фильтры истории»: компонент `HistoryFiltersSheet` (`:253`), корень
   `fullscreenSurfaceClass` (`:373`), внутренняя лента `:382` —
   `overflow-y-auto` без лока фона; хук в файле не импортируется. Тот же шит
   открывает канон `CalendarRangePicker` (`:529`) поверх себя — первая
   вложенность «поверхность на поверхности».
2. `apps/frontend/widgets/participants/ui/participants-invite-screen.tsx` —
   пикер «Выбрать объект» приглашения: компонент `InviteObjectsPicker`
   (`:240`), корень `cn(fullscreenSurfaceClass, 'font-sans')` (`:273`), лента
   `:287` — `overflow-y-auto` без лока.
3. `apps/frontend/widgets/rentals/ui/wizard-chrome.tsx` — шаг «Выберите день»
   визарда аренды: компонент `PaymentDayPicker` (`:207`), корень
   `fullscreenSurfaceClass` (`:245`), лента `:253` — без лока.
4. `apps/frontend/widgets/properties/ui/property-edit/property-address-search.tsx` —
   поиск адреса объекта: компонент `PropertyAddressSearch` (`:39`), корень
   `cn(fullscreenSurfaceClass, 'overflow-y-auto')` (`:68`) — поверхность сама
   скроллер, лока нет.

Текущий код эталона класса (делаем так же):

```tsx
// apps/frontend/shared/ui/design/calendar-date-picker.tsx:47,185,601 — reference
import { useBodyScrollLock } from '@/shared/lib/hooks/useBodyScrollLock';
// …в CalendarDatePicker:
  useBodyScrollLock();
// …и в CalendarRangePicker:
  useBodyScrollLock();
```

Побочная находка: JSDoc хука (`apps/frontend/shared/lib/hooks/useBodyScrollLock.ts:10-14`)
утверждает «Вложенные локи не поддерживаются: полноэкранные поверхности не
вкладываются друг в друга». Пункт 1 делает это утверждение ложью: шит
фильтров вкладывает пикер периода. Хук при вложенности корректен и без
изменений: restore по захваченному `previous` — LIFO, а React очищает
эффекты детей раньше родительских, поэтому порядок восстановления всегда
правильный. Стек «поверхность с этим хуком → Radix-шит (свой лок)» уже
живёт: календарный пикер открывает шит месяца. Чинится текстом JSDoc.

## Target

1. Четыре поверхности получают `useBodyScrollLock()` первой инструкцией
   после деструктуризации пропов (до остальных хуков — порядок не важен,
   эффект монтирования). Импорт — к существующему блоку импортов файла.

```tsx
// apps/frontend/widgets/history/ui/history-filters-sheet.tsx — target
import { useBodyScrollLock } from '@/shared/lib/hooks/useBodyScrollLock';

export function HistoryFiltersSheet({ /* …пропы без изменений… */ }): JSX.Element {
  // Лок фона: поверхность непрозрачно накрывает ленту — жест у края
  // внутренней ленты не должен уезжать в цепочку скролла страницы
  // (канон useBodyScrollLock, решение владельца 2026-10-13; конвенция
  // слоёв — тикет #1417).
  useBodyScrollLock();
```

Аналогично в `InviteObjectsPicker`
(participants-invite-screen.tsx, функция `:240`),
`PaymentDayPicker` (rentals/ui/wizard-chrome.tsx, функция `:207`,
комментарий тот же) и `PropertyAddressSearch`
(property-edit/property-address-search.tsx, функция `:39`).

2. JSDoc `useBodyScrollLock.ts` — заменить абзац про вложенность
   (абзац про Windows остаётся):

```ts
// apps/frontend/shared/lib/hooks/useBodyScrollLock.ts — target
/** Лок прокрутки страницы на всё время монтирования поверхностного
 * диалога. Полноэкранные поверхности (календарные пикеры, шит фильтров
 * истории, пикеры визардов, поиск адреса) — fixed-оверлеи без модальной
 * обёртки: страница под ними остаётся скроллируемой, и жест у края
 * внутренней ленты уезжает в цепочку скролла страницы; под непрозрачной
 * поверхностью движение невидимо и «уводит» ленту к закрытию (решение
 * владельца 2026-10-13; прецедент — ручной overflow:hidden в more-sheet;
 * конвенция слоёв — карта #1416, тикет #1417). Восстановление — по
 * очистке эффекта. Локи вкладываются LIFO (шит фильтров открывает пикер
 * периода поверх себя): React очищает эффекты детей раньше родительских,
 * restore по захваченному previous корректен. С локом Radix-шитов
 * (react-remove-scroll) хук не конфликтует — стек «календарный пикер
 * (этот хук) → шит месяца (Radix)» уже живёт. На Windows
 * появление/исчезновение скроллбара сдвигает фоновую страницу на ширину
 * гюттера (html держит scrollbar-gutter: stable — сдвига колонки нет). */
```

3. Новый e2e-гейт класса — `apps/frontend/e2e/scroll-isolation.spec.ts`
   (целиком, три теста; расширяется планом 006):

```ts
import {
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  paymentCreatedEntry,
  seedJournalEntry,
  test,
} from './fixtures';
import type { Page } from '@playwright/test';

// Изоляция скролла (карта #1416, тикет #1417, конвенция «одно правило на
// слой»): фуллскрин-поверхность обязана замораживать фон на всё открытие
// (useBodyScrollLock — канон календарных пикеров, решение владельца
// 2026-10-13). Жест у края внутренней ленты не уезжает в цепочку скролла
// страницы, после закрытия поверхности позиция чтения та же.
//
// Замер — скроллеры документа: scrollingElement.scrollTop (корневой) и
// body.scrollTop (под локом body — скролл-контейнер; цепочка туда тоже
// гасится). mouse.wheel шлёт реальные wheel-события и цепочку
// воспроизводит честно (прецеденты: scroll-shift.spec.ts,
// calendar-feed.spec.ts).

type DocScroll = { readonly root: number; readonly body: number };

async function docScroll(page: Page): Promise<DocScroll> {
  return page.evaluate(() => ({
    root: document.scrollingElement?.scrollTop ?? -1,
    body: document.body.scrollTop,
  }));
}

/** Колёсные всплески в точке: вниз к краю и вверх за край — попытка
 * отдать жест цепочкой в обоих направлениях. */
async function wheelBursts(page: Page, x: number, y: number): Promise<void> {
  await page.mouse.move(x, y);
  for (let i = 0; i < 5; i += 1) {
    await page.mouse.wheel(0, 600);
  }
  for (let i = 0; i < 8; i += 1) {
    await page.mouse.wheel(0, -600);
  }
  await page.waitForTimeout(300);
}

/** Гарантированно прокручиваемый фон: распорка над контентом + прокрутка
 * документа вниз. */
async function scrollBackground(page: Page): Promise<void> {
  await page.evaluate(() => {
    const spacer = document.createElement('div');
    spacer.style.height = '2500px';
    document.body.prepend(spacer);
  });
  await page.mouse.move(200, 400);
  for (let i = 0; i < 6; i += 1) {
    await page.mouse.wheel(0, 700);
  }
  await page.waitForTimeout(300);
}

test.describe('Изоляция скролла', () => {
  test('фон заморожен под поверхностью: шит фильтров истории', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.setViewportSize({ width: 390, height: 844 });
    await execE2eSql('DELETE FROM action_journal;');
    for (let i = 0; i < 12; i += 1) {
      await seedJournalEntry(
        paymentCreatedEntry(
          `b1000000-0000-4000-8000-0000000000${String(i).padStart(2, '0')}`,
          i,
          `Стресс ${i}`,
        ),
        seededUser,
      );
    }
    await page.goto('/history');
    await expect(page.getByText('Платёж создан: Стресс 0')).toBeVisible();
    await scrollBackground(page);

    await page.getByRole('button', { name: 'Настройки' }).click();
    const dialog = page.getByRole('dialog', { name: 'Фильтры истории' });
    await expect(dialog).toBeVisible();
    // База снимается ПОСЛЕ открытия: клик мог автоскроллить страницу.
    const before = await docScroll(page);

    // Всплески: центр ленты шита, верхний край ленты (симптом тикета),
    // шапка поверхности — вне ленты.
    const list = dialog.locator('.overflow-y-auto');
    const listBox = await list.boundingBox();
    const dialogBox = await dialog.boundingBox();
    if (listBox === null || dialogBox === null) {
      throw new Error('лента или каркас шита фильтров не найдены');
    }
    await wheelBursts(page, listBox.x + listBox.width / 2, listBox.y + listBox.height / 2);
    await wheelBursts(page, listBox.x + listBox.width / 2, listBox.y + 8);
    await wheelBursts(page, dialogBox.x + dialogBox.width / 2, dialogBox.y + 30);

    const during = await docScroll(page);
    expect(during.root).toBe(before.root);
    expect(during.body).toBe(0);

    await page.getByRole('button', { name: 'Закрыть фильтры' }).click();
    await expect(dialog).toBeHidden();
    expect(await docScroll(page)).toEqual(before);
  });

  test('фон заморожен под поверхностью: пикер объектов приглашения', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/participants/invite');
    await expect(page.getByText('Все 3 объекта')).toBeVisible();
    await scrollBackground(page);

    await page.getByText('Все 3 объекта').click();
    const picker = page.getByRole('dialog', { name: 'Выбрать объект' });
    await expect(picker).toBeVisible();
    const before = await docScroll(page);

    const list = picker.locator('.overflow-y-auto');
    const listBox = await list.boundingBox();
    if (listBox === null) {
      throw new Error('лента пикера объектов не найдена');
    }
    await wheelBursts(page, listBox.x + listBox.width / 2, listBox.y + 8);

    const during = await docScroll(page);
    expect(during.root).toBe(before.root);
    expect(during.body).toBe(0);
  });

  test('фон заморожен под поверхностью: пикер периода (регресс канона)', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/ui-kit');
    await scrollBackground(page);

    await page.getByRole('button', { name: 'Открыть пикер периода' }).click();
    const dialog = page.getByRole('dialog', { name: 'Выберите период' });
    await expect(dialog).toBeVisible();
    const before = await docScroll(page);

    const list = dialog.locator('.overflow-y-auto');
    const listBox = await list.boundingBox();
    if (listBox === null) {
      throw new Error('лента пикера периода не найдена');
    }
    await wheelBursts(page, listBox.x + listBox.width / 2, listBox.y + 8);

    expect((await docScroll(page)).root).toBe(before.root);
  });
});
```

## Repo conventions to follow

- Прецедент и эталон — `calendar-date-picker.tsx:47,185,601` (см. Problem):
  ровно этот хук на ровно этом классе поверхностей, слова владельца 13.10.
- Лок Radix-шитов не дублируем: `Modal`/vaul локает сам Radix Dialog
  (react-remove-scroll), прыжок колонки на Windows уже погашен нейтрализатором
  `html body[data-scroll-locked]` (`apps/frontend/app/globals.css:993-996`,
  #925) — поверхностям на `fullscreenSurfaceClass` Radix-лока нет, им нужен
  ручной хук.
- Комментарии в коде — по-русски, «почему», со ссылкой на решение/канон
  (образец — прежний JSDoc этого хука и комментарий к `overscroll-contain`
  в `shared/ui/design/modal.tsx:134-135`).
- e2e: сид через `fixtures` (`execE2eSql`/`seedJournalEntry`/`paymentCreatedEntry`),
  локаторы — по a11y-ролям и именам, кубики через `boundingBox()` с явным
  `throw` (образец — `e2e/calendar-feed.spec.ts:37-42`).

## Steps

1. `apps/frontend/shared/lib/hooks/useBodyScrollLock.ts`: заменить JSDoc
   блока на текст из Target (п.2) — код хука не менять.
2. `apps/frontend/widgets/history/ui/history-filters-sheet.tsx`: добавить
   импорт и вызов `useBodyScrollLock()` в `HistoryFiltersSheet` (Target п.1).
3. `apps/frontend/widgets/participants/ui/participants-invite-screen.tsx`:
   то же в `InviteObjectsPicker`. Комментарий — тот же, что в п.2.
4. `apps/frontend/widgets/rentals/ui/wizard-chrome.tsx`: то же в
   `PaymentDayPicker`.
5. `apps/frontend/widgets/properties/ui/property-edit/property-address-search.tsx`:
   то же в `PropertyAddressSearch`.
6. Создать `apps/frontend/e2e/scroll-isolation.spec.ts` — код из Target п.3.

## Boundaries

- Do NOT трогать календарные пикеры (`calendar-date-picker.tsx`), `modal.tsx`,
  `wheel-picker.tsx`, `more-sheet.tsx` — там канон уже соблюдён.
- Do NOT добавлять overscroll-классы и скрытия скроллбаров на ленты — это
  слой «внутренняя лента», план 006.
- Do NOT менять разметку, геометрию и классы поверхностей — только вызов
  хука и JSDoc.
- Do NOT переписывать хук (refcount и пр.) — LIFO-restore корректен,
  чинится текстом JSDoc.
- Do NOT вешать лок на `ParticipantsInviteScreen` (форма приглашения —
  страница-маршрут, документный слой) — только на `InviteObjectsPicker`.
- Если код в указанных местах не совпадает с цитатами (дрейф от коммита
  7a963de1a) — СТОП и отчёт, не импровизировать.

## Verification

- **Mechanical**:
  - `cd apps/frontend && npx tsc --noEmit` — без ошибок; `npm run lint` —
    зелёный; `make frontend-test` — зелёный.
  - `make frontend-e2e TESTS='-g фон заморожен'` (Docker) — 3 теста зелёные.
    Red-проверка перед фиксом: спека на чистом дереве красная (первый тест —
    на цепочке у края ленты шита, документ «уезжает»); после шагов 2–5 —
    зелёная.
- **Feel check** (телефон, протокол карты #1416; эмуляция feel не
  воспроизводит):
  - шит фильтров истории: доскроллить ленту страницы вниз, открыть шит,
    свайпнуть список шита вверх до упора и за край — лента ПОД шитом стоит
    на месте; закрыть шит — страница на той же позиции чтения;
  - то же на пикере объектов приглашения, шаге «Выберите день» визарда
    аренды и поиске адреса объекта;
  - iOS: свайп у верхнего края ленты шита не тянет страницу за собой;
    Android: аналогично.
- **Done when**: четыре поверхности держат лок; JSDoc описывает реальную
  семантику вложенности; `make frontend-e2e TESTS='-g фон заморожен'`
  зелёный; `make frontend-test` зелёный.
