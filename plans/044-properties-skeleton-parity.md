# 044 — Скелетоны хаба объектов: хромированный кадр и снос чужого архетипа

- **Status**: DONE
- **Baseline**: ветка multi-scroll, догонка feel-check 16.10 (039–043 DONE)
- **Severity**: HIGH (решение владельца, feel-check 16.10)
- **Origin**: карта #1437, догонка feel-check владельца
- **Estimated scope**: PropertiesLoading.tsx, properties/page.tsx, properties/loading.tsx (снос), e2e, ~120 строк

## Problem

Холодная перезагрузка (сброс кеша) с медленной сетью (телефон через
туннель) показывает сломанные скелетоны на «Объектах» и на объекте.
Диагноз (воспроизведён на стенде, CDP-троттлинг 700ms/1.6Mbps):

1. **Список — голый кадр.** `PropertiesLoading` — единственный
   route-loading виджет без хрома (у «Задач», «Платежей», детали объекта,
   contacts/notifications/participants/operations — TopNav + PageContent,
   «кнопки в покое», §7): карточки во всю ширину (нет px-6), нет шапки
   «рентли», заголовка с «+», пилюли поиска, ряда сортировки. Резолв
   данных рисует всё это разом — страница вздрагивает (паритет #604
   нарушен).
2. **Объект — скелетон чужой страницы.** Segment-level
   `app/(screens)/properties/loading.tsx` накрывает всех детей
   `/properties`: при холодной загрузке `/properties/[id]` первый кадр —
   скелетон списка (async-компонент страницы suspends до inner Suspense,
   внешний boundary стримит свой fallback), затем правильный
   `PropertyDetailRouteLoading`, затем контент. На телефоне чужой кадр
   висит секунды. У всех префетчащих детей (`[id]`,
   `[id]/payments/[paymentId]`, `[id]/rentals/[rentalId]`,
   `[id]/operations/[operationId]`) есть свои in-page Suspense fallback'и
   — сегментный файл только вредит.

Решение владельца 16.10: чинить и то и другое; кадр списка — полный
паритет с живым экраном (прецедент TasksLoading); системная зачистка
остальных хабов — отдельным планом 045.

## Target

**Шаг 1 — `widgets/properties/ui/PropertiesLoading.tsx`**: разделить
роли. Нынешний `PropertiesLoading` (карточки-скелетоны, `gap-4`,
`data-testid="properties-loading"`) остаётся внутренним скелетоном фазы
react-query (живёт внутри настоящего хрома страницы, ветка `isLoading`).
Новый экспорт `PropertiesHubLoading` — полный кадр по прецеденту
TasksLoading: TopNav (mobileWings, collapse title «Объекты», trailing
IconButton «+» в покое) → PageContent → HubCollapseAnchor → ряд заголовка
(HubTitle «Объекты» + IconButton «+» в покое) → обёртка `px-6 pt-4` с
ритмом живого экрана: SearchPill в покое (тот же testid
properties-search-pill), ряд сортировки `mt-6` (кнопка-чип в покое:
secondary/small, SortingSmallBig + SmallArrowDown + «Название» — без
PickerMenu, интерактивным экран подменяет), список карточек `mt-6`
(существующий PropertyCardSkeleton, testid properties-loading).

**Шаг 2 — `app/(screens)/properties/page.tsx`**: fallback Suspense-
границы меняется с `<PropertiesLoading />` на `<PropertiesHubLoading />`
(кадр холодной загрузки = кадр клиентской навигации, неотличим от
pending-состояния).

**Шаг 3 — снос `app/(screens)/properties/loading.tsx`**: сегментный
файл сносится — список холодной загрузки получает свой полный кадр через
in-page границу (визуально тот же), дети `/properties` перестают
получать чужой архетип: `[id]` и платёж/аренда/операция стримят свои
in-page fallback'и первым кадром.

**Шаг 4 — grep-проход**: `PropertiesLoading` употребляется только в
page.tsx (обе роли до шага 2) и index.ts экспорте; после — проверить, что
внешние потребители не ссылаются на снесённый loading.tsx.

## TDD seams

- Новый `e2e/properties-loading.spec.ts` (проект chromium, CDP
  emulateNetworkConditions 700ms/1.6Mbps + setCacheDisabled, как в
  диагностике):
  - «холодная загрузка хаба: кадр с хромом, карточки не во всю ширину» —
    goto /properties с waitUntil commit, ожидание
    [data-testid="properties-loading"], в его присутствии: пилюля поиска
    видна, bounding box первой карточки x ≥ 20 (сейчас: пилюли нет,
    x = 0 — красный);
  - «холодная загрузка объекта: скелетон списка не стримится» —
    addInitScript с MutationObserver-журналом появления
    [data-testid="properties-loading"], goto /properties/[id], после
    load: журнал пуст, в фазе загрузки виден aria-label «Загрузка
    объекта» (сейчас: журнал непуст — красный).
- Существующие e2e хаба — прогон: паритет кадра не ломает переходы
  (тесты ждут контент, не скелетон).

## Boundaries

- Do NOT трогать внутренний скелетон карточек (PropertyCardSkeleton) —
  паритет карточки #604 уже верен, чинится только обёртка-кадр.
- Do NOT трогать PropertyDetailRouteLoading и in-page fallback'и детей —
  они верны, ломает их только сегментный loading.tsx.
- Do NOT трогать другие хабы — их тень и их кадры проверяет план 045.
- Do NOT трогать ритм живого экрана (pt-4/mt-6, решение владельца
  14.10.2026) — кадр его зеркалит, не меняет.

## Verification

- `npx tsc --noEmit`, `npm run lint`, `make frontend-test` — зелёные.
- `make frontend-e2e TESTS="e2e/properties-loading.spec.ts e2e/scroll-isolation.spec.ts"`
  и хабовые спеки (properties) — зелёные.
- Feel на стенде: перезагрузка /properties и /properties/[id] с троттлингом
  — хром на месте с первого кадра, чужого скелетона нет.

## Done when

Холодная загрузка списка показывает хромированный кадр (шапка, заголовок,
поиск, сортировка, карточки в паддингах) без вздрагивания; холодная
загрузка объекта и детей /properties никогда не показывает скелетон
списка; properties/loading.tsx снесён; e2e зелёные.
