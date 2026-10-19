# 006 — Внутренние ленты: гашение жеста у края и скрытые скроллбары

- **Status**: TODO
- **Commit**: 7a963de1a
- **Severity**: MEDIUM
- **Category**: Изоляция скроллеров — слой «внутренняя лента» (конвенция «Мульти-скролл», карта #1416 / тикет #1417; эталон — WheelPicker; DESIGN.md §14 — корневой скроллбар единственный видимый, решение владельца 2026-10-01)
- **Estimated scope**: 7 файлов (7 класс-строк в 6 файлах + DESIGN.md + e2e), ~40 строк с гейтом

## Problem

Внутренние ленты — полновысотные списки внутри поверхностей и на
полновысотных экранах (паттерн §3 DESIGN.md). Две хвори класса:

1. **Жест у края уезжает в цепочку**: у лент без `overscroll-behavior` доскролл
   до края отдаёт жест скроллеру-родителю (у поверхностей — в замороженный
   планом 005 фон, на полновысотных экранах-маршрутах — в документ).
2. **«Второй скролл»**: на классических скроллбарах (Windows Chromium) лента
   рисует собственную полосу прокрутки рядом с корневой — под закреплённой
   шапкой календаря она читается как полоса мусора (тикет #1417).

Канон класса уже существует — WheelPicker
(`shared/ui/design/wheel-picker.tsx:441`, `<ul>` ленты): `overscroll-y-contain` +
`[scrollbar-width:none]`; у календарных лент `overscroll-contain` уже стоит,
скрытия скроллбара нет. Перепись вертикальных скроллеров фронта
(`overflow-y-auto|scroll`, коммит 7a963de1a) — семь мест класса и три
осознанных исключения:

| Место | Строка | contain | скрытие |
| --- | --- | --- | --- |
| Календарь, лента одиночного пикера `calendar-date-picker.tsx` | :386 | есть | **нет** |
| Календарь, лента пикера периода `calendar-date-picker.tsx` | :765 | есть | **нет** |
| Шит фильтров истории `history-filters-sheet.tsx` | :382 | **нет** | **нет** |
| Пикер объектов приглашения `participants-invite-screen.tsx` | :287 | **нет** | **нет** |
| Пикер дня визарда аренды `rentals/ui/wizard-chrome.tsx` | :253 | **нет** | **нет** |
| Поиск адреса (корень поверхности-скроллер) `property-address-search.tsx` | :68 | **нет** | **нет** |
| Категории объекта (полновысотный экран-маршрут) `operations-categories-screen.tsx` | :140 | **нет** | **нет** |

Вне класса (не трогать): `shared/ui/design/text-field.tsx` (autogrow-textarea —
редактор ввода, не лента), `app/ui-kit/design-layer/sections/navigation.tsx`
(витрина, не продукт), `shared/ui/design/modal.tsx:136` (тело шита уже
`overscroll-contain`, короткое — скроллбара не видно), `wheel-picker.tsx`
(эталон), документные страницы (лента истории и хабы скроллятся окном —
индикатор и есть корневой скроллбар, канон решения 01.10).

Текущий код всех семи мест:

```tsx
// apps/frontend/shared/ui/design/calendar-date-picker.tsx:386 — current
        className="relative min-h-0 flex-1 overflow-y-auto overscroll-contain"

// apps/frontend/shared/ui/design/calendar-date-picker.tsx:765 — current
        className="relative min-h-0 flex-1 overflow-y-auto overscroll-contain [overflow-anchor:none]"

// apps/frontend/widgets/history/ui/history-filters-sheet.tsx:382 — current
      <div className="relative min-h-0 flex-1 overflow-y-auto">

// apps/frontend/widgets/participants/ui/participants-invite-screen.tsx:287 — current
      <div className="relative min-h-0 flex-1 overflow-y-auto tablet:mt-[72px]">

// apps/frontend/widgets/rentals/ui/wizard-chrome.tsx:253 — current
        <div className="flex-1 min-h-0 overflow-y-auto pb-6">

// apps/frontend/widgets/properties/ui/property-edit/property-address-search.tsx:68 — current
      className={cn(fullscreenSurfaceClass, 'overflow-y-auto')}

// apps/frontend/widgets/payments/ui/operations-categories-screen.tsx:140 — current
        <div className="min-h-0 flex-1 overflow-y-auto pt-2 pb-[calc(6.5rem+env(safe-area-inset-bottom))]">
```

## Target

К каждому класс-строке добавить `overscroll-y-contain [scrollbar-width:none]`
(календарным лентам — только `[scrollbar-width:none]`, contain там уже есть):

```tsx
// apps/frontend/shared/ui/design/calendar-date-picker.tsx:386 — target
        className="relative min-h-0 flex-1 overflow-y-auto overscroll-contain [scrollbar-width:none]"

// apps/frontend/shared/ui/design/calendar-date-picker.tsx:765 — target
        className="relative min-h-0 flex-1 overflow-y-auto overscroll-contain [overflow-anchor:none] [scrollbar-width:none]"

// apps/frontend/widgets/history/ui/history-filters-sheet.tsx:382 — target
      <div className="relative min-h-0 flex-1 overflow-y-auto overscroll-y-contain [scrollbar-width:none]">

// apps/frontend/widgets/participants/ui/participants-invite-screen.tsx:287 — target
      <div className="relative min-h-0 flex-1 overflow-y-auto tablet:mt-[72px] overscroll-y-contain [scrollbar-width:none]">

// apps/frontend/widgets/rentals/ui/wizard-chrome.tsx:253 — target
        <div className="flex-1 min-h-0 overflow-y-auto overscroll-y-contain [scrollbar-width:none] pb-6">

// apps/frontend/widgets/properties/ui/property-edit/property-address-search.tsx:68 — target
      className={cn(fullscreenSurfaceClass, 'overflow-y-auto overscroll-y-contain [scrollbar-width:none]')}

// apps/frontend/widgets/payments/ui/operations-categories-screen.tsx:140 — target
        <div className="min-h-0 flex-1 overflow-y-auto overscroll-y-contain [scrollbar-width:none] pt-2 pb-[calc(6.5rem+env(safe-area-inset-bottom))]">
```

`scrollbar-width: none`, а не webkit-псевдоэлементы: скрытия внутренних
скроллбаров держатся на стандартном свойстве — оно приоритетнее
webkit-псевдоэлементов при наследуемом `scrollbar-color` и не переводит
браузер на кастомный рендер (DESIGN.md §14, решение владельца 01.10).
`overscroll-y-contain`, а не `none`: гасим только цепочку наверх, локальные
эффекты края (rubber-band iOS внутри ленты) не трогаем — тот же выбор, что у
WheelPicker.

E2E-дополнение `apps/frontend/e2e/scroll-isolation.spec.ts` — три теста
внутрь существующего `test.describe('Изоляция скролла')` (файл и хелперы
`docScroll`/`wheelBursts` вводит план 005; в импорты добавляется
`SEEDED_APARTMENT_PROPERTY_ID`):

```ts
import {
  execE2eSql,
  expect,
  openCabinetWithSeededSession,
  paymentCreatedEntry,
  seedJournalEntry,
  SEEDED_APARTMENT_PROPERTY_ID,
  test,
} from './fixtures';
// …хелперы docScroll / wheelBursts — из плана 005…

  test('край ленты гасит жест: категории объекта', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`/properties/${SEEDED_APARTMENT_PROPERTY_ID}/operations/categories`);
    const feed = page.locator('.min-h-0.flex-1.overflow-y-auto');
    await expect(feed).toBeVisible();
    const style = await feed.evaluate((el) => {
      const cs = getComputedStyle(el);
      return { overscrollY: cs.overscrollBehaviorY, scrollbar: cs.scrollbarWidth };
    });
    expect(style.overscrollY).toBe('contain');
    expect(style.scrollbar).toBe('none');

    // Поведенчески: прокрутка вниз по ленте (сама лента и/или цепочка)
    // и всплески за оба края — документ не двигается (contain гасит
    // цепочку на ленте; страница-маршрут лока не имеет и не должна).
    const feedBox = await feed.boundingBox();
    if (feedBox === null) {
      throw new Error('лента категорий не найдена');
    }
    await page.mouse.move(feedBox.x + feedBox.width / 2, feedBox.y + feedBox.height / 2);
    for (let i = 0; i < 12; i += 1) {
      await page.mouse.wheel(0, 700);
    }
    await page.waitForTimeout(300);
    const before = await docScroll(page);
    await wheelBursts(page, feedBox.x + feedBox.width / 2, feedBox.y + 8);
    await wheelBursts(page, feedBox.x + feedBox.width / 2, feedBox.y + feedBox.height - 8);
    expect(await docScroll(page)).toEqual(before);
  });

  test('край ленты гасит жест: шит фильтров (contain и скрытый скроллбар)', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.setViewportSize({ width: 390, height: 844 });
    await execE2eSql('DELETE FROM action_journal;');
    await seedJournalEntry(
      paymentCreatedEntry('b2000000-0000-4000-8000-000000000001', 0, 'Аренда'),
      seededUser,
    );
    await page.goto('/history');
    await expect(page.getByText('Платёж создан: Аренда')).toBeVisible();

    await page.getByRole('button', { name: 'Настройки' }).click();
    const dialog = page.getByRole('dialog', { name: 'Фильтры истории' });
    await expect(dialog).toBeVisible();
    const list = dialog.locator('.overflow-y-auto');
    const style = await list.evaluate((el) => {
      const cs = getComputedStyle(el);
      return { overscrollY: cs.overscrollBehaviorY, scrollbar: cs.scrollbarWidth };
    });
    expect(style.overscrollY).toBe('contain');
    expect(style.scrollbar).toBe('none');
  });

  test('скроллбар ленты календаря скрыт: пикер периода', async ({ page, seededUser }) => {
    await openCabinetWithSeededSession(page, seededUser);
    await page.goto('/ui-kit');
    await page.getByRole('button', { name: 'Открыть пикер периода' }).click();
    const dialog = page.getByRole('dialog', { name: 'Выберите период' });
    await expect(dialog).toBeVisible();
    const list = dialog.locator('.overflow-y-auto');
    const scrollbar = await list.evaluate((el) => getComputedStyle(el).scrollbarWidth);
    expect(scrollbar).toBe('none');
  });
```

Запись конвенции в канон — `apps/frontend/DESIGN.md` (этот план последний
усилия: канон фиксируется, когда конвенция целиком в коде):

1. В §13 «Фиксация решений», сразу после записи «Корневой скроллбар —
   светлый с белым треком» (оканчивается словами «полная запись в §14
   («Стабильность»).»), добавить:

```markdown
- **Конвенция изоляции скроллеров — одно правило на слой** (карта #1416,
  тикет #1417): документ — единственный видимый скроллбар продукта,
  глобальных overscroll-запретов на html/body нет (системный pull-to-refresh
  жив, ADR 0031), `overflow-x: clip` не hidden (sticky жив, #709); фон
  замораживается на время любой модальной поверхности — фуллскрин-поверхности
  хуком `useBodyScrollLock` (канон 13.10: календарные пикеры, шит фильтров
  истории, пикеры визардов, поиск адреса), шиты — локом Radix из коробки;
  внутренние ленты гасят жест у края (`overscroll-y-contain`) и прячут свой
  скроллбар (`[scrollbar-width:none]`, как WheelPicker) — «второй скролл» под
  поверхностями лечится скрытием внутренних скроллбаров, не корневого:
  видимый трек корневого скроллбара под фиксированной поверхностью — норма
  (гюттер стабилен, колонка не прыгает, решение 01.10 выше). Редакторы ввода
  (textarea) и витрина /ui-kit — вне правила.
```

2. В §14, раздел «Мобильный UX», сразу после пункта «Скролл и sticky не
   ломаются…», добавить:

```markdown
- Изоляция скролла по конвенции слоёв (тикет #1417): под фуллскрин-поверхностью
  фон заморожен (жест у края ленты не уезжает в страницу, позиция чтения на
  месте после закрытия); внутренние ленты гасят жест у края и не рисуют
  второй скроллбар. Гард: e2e `scroll-isolation.spec.ts`.
```

## Repo conventions to follow

- Эталон класса — WheelPicker (`shared/ui/design/wheel-picker.tsx`, `<ul>`
  ленты): `overscroll-y-contain [scrollbar-width:none]` — ровно эта пара.
- Стандартное `scrollbar-width: none` для скрытий — канон DESIGN.md §14
  (решение владельца 01.10, абзац «Скрытия внутренних скроллбаров…»);
  webkit-псевдоэлементы не добавлять.
- Глобальные overscroll-запреты на html/body запрещены: системный
  pull-to-refresh жив (ADR 0031, ротация 12.10; DESIGN.md §14 «Pull-to-refresh
  нативен там, где он у платформы есть»); точечный гард на время жеста —
  прецедент `ptr-suppress-overscroll` (`shared/ui/pull-to-refresh/PullToRefresh.tsx:79`).
- Комментарии в класс-строках не пишутся — «почему» уходит в запись DESIGN.md
  из Target (прецедент: классы WheelPicker без комментов, канон описан в §5).

## Steps

1. `apps/frontend/shared/ui/design/calendar-date-picker.tsx`: классы `:386`
   и `:765` — по Target.
2. `apps/frontend/widgets/history/ui/history-filters-sheet.tsx`: класс `:382`
   — по Target.
3. `apps/frontend/widgets/participants/ui/participants-invite-screen.tsx`:
   класс `:287` — по Target.
4. `apps/frontend/widgets/rentals/ui/wizard-chrome.tsx`: класс `:253` —
   по Target.
5. `apps/frontend/widgets/properties/ui/property-edit/property-address-search.tsx`:
   класс `:68` — по Target.
6. `apps/frontend/widgets/payments/ui/operations-categories-screen.tsx`:
   класс `:140` — по Target.
7. `apps/frontend/e2e/scroll-isolation.spec.ts`: добавить импорт
   `SEEDED_APARTMENT_PROPERTY_ID` и три теста из Target.
8. `apps/frontend/DESIGN.md`: две вставки из Target (§13 и §14).

## Boundaries

- Do NOT трогать корневые стили (`globals.css:954-1006` — канон
  #865/#925/#709) и `html`/`body` — никаких overscroll-классов на документе.
- Do NOT трогать `wheel-picker.tsx`, `modal.tsx`, `more-sheet.tsx`,
  `text-field.tsx`, витрину `/ui-kit` — канон/вне класса.
- Do NOT вешать `useBodyScrollLock` на «Категории объекта» — это
  страница-маршрут документного слоя, лока не положено (конвенция: лок —
  только у модальных поверхностей).
- Do NOT добавлять webkit-псевдоэлементы для скрытия скроллбаров.
- Если код в указанных местах не совпадает с цитатами (дрейф от коммита
  7a963de1a или уже исполнен план 005) — СТОП и отчёт, не импровизировать.

## Verification

- **Mechanical**:
  - `cd apps/frontend && npx tsc --noEmit` — без ошибок; `npm run lint` —
    зелёный; `make frontend-test` — зелёный.
  - `make frontend-e2e TESTS='-g Изоляция скролла'` (Docker) — все 6 тестов
    файла зелёные (3 из плана 005 + 3 отсюда). Red-проверка: тесты
    «край ленты гасит жест…» и «скроллбар ленты календаря скрыт» красные до
    шагов 1–6, зелёные после.
  - Регресс по месту: `make frontend-e2e TESTS='-g WheelPicker'` и кейс
    дорисовки `calendar-feed.spec.ts` (`TESTS='-g дорисовка месяцев'`) —
    зелёные (лента периода не изменила механику докрутки).
- **Feel check** (телефон + десктоп, протокол карты #1416):
  - лента календаря и лента категорий на десктопе: скроллбара ленты больше
    нет — единственная полоса на странице корневая; колесо мыши у верхнего
    края ленты страницу не крутит;
  - шит фильтров на телефоне: инерционный свайп списка у края гаснет В
    ленте (лента доезжает до своего края и останавливается), фон не
    трогается; под лентой нет серой полосы скроллбара;
  - pull-to-refresh не сломан: на iOS standalone тяга с верха ленты истории
    поднимает спиннер (глобальных запретов не появилось).
- **Done when**: все семь мест класса классифицированы и правлены; канон
  записан в DESIGN.md (§13 + §14); e2e «Изоляция скролла» целиком зелёный;
  регресс WheelPicker/calendar-feed зелёный.
