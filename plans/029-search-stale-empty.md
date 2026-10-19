# 029 — Поиск: пустой результат не отвечает за летящий запрос

- **Status**: TODO
- **Commit**: 2a37f5bc5
- **Severity**: MEDIUM
- **Category**: Loading policy (DESIGN.md §14 keepPreviousData; состояния списков §7)
- **Estimated scope**: 8 экранов, по одному условию на каждый

## Problem

Канон §14: «in-place-обновления (поиск, фильтры, период) держат прежнюю
выдачу (keepPreviousData)». Выполнено: миганий скелетоном на смене запроса
нет ни на одной из 20 поверхностей (гейты — `isPending`/`data === undefined`,
при KPD истинные только до первого успеха).

Пробел — **stale empty state**: если предыдущий ответ был пуст, то при
правке запроса прежняя выдача — пустой массив, и экран показывает
определительное «Такого X нет» как ответ на **новый** запрос, пока тот
летит (дебаунс 300мс + время фетча). Состояние ложно: результата нового
запроса ещё нет.

Механика на примере `widgets/properties/ui/properties-search-screen.tsx:82-97`
(текущий код):

```tsx
{searchQuery.isPending && searching ? (
  <PropertiesSearchSkeleton />
) : searchQuery.isError ? (
  <PropertiesSearchErrorCard onRetry={() => void searchQuery.refetch()} />
) : !searching ? (
  <EmptyState
    imageSrc="/images/payments/payments-objects-search.png"
    className="pt-16 pb-16"
    description="Введите название или адрес объекта"
  />
) : properties.length === 0 ? (
  <EmptyState
    imageSrc="/images/payments/payments-objects-search.png"
    className="pt-16 pb-16"
    description="Такого объекта нет"
  />
) : (
```

После первого успеха `isPending` больше не истин (`features/properties/api/hooks.ts:142`
— KPD), `properties = searchQuery.data ?? []` (`:56`) держит старый (пустой)
ответ, и ветка `properties.length === 0` рисует «Такого объекта нет» во
время полёта нового запроса.

Восемь поверхностей с этой структурой (все проветрены по месту):

| # | Экран | Скелетон-гейт (тек.) | Пустая ветка (тек.) |
| --- | --- | --- | --- |
| 1 | `widgets/properties/ui/properties-search-screen.tsx` | `:82` | `:92` («Такого объекта нет» `:96`) |
| 2 | `widgets/contacts/ui/contact-book-search-screen.tsx` | `:80` | `:87` (`ContactsNoResults` `:88`) |
| 3 | `widgets/contacts/ui/contacts-of-property-screen.tsx` | `:177` | `:182` (`ContactsNoResults` `:183`) |
| 4 | `widgets/rentals/ui/rental-contact-picker-screen.tsx` | `:94` | `:98` (`ContactsNoResults` `:99`) |
| 5 | `widgets/payments/ui/operations-search-screen.tsx` | `:86-92` (`pending`) | `:153` (`SearchStateView` «Такой операции нет» `:154`) |
| 6 | `widgets/payments/ui/operations-global-search-screen.tsx` | `:111-116` (`pending`) | `:171` (`:175`) |
| 7 | `widgets/payments/ui/payments-objects-search-screen.tsx` | `:55-57` (`pending`) | `:123` (`:126`) |
| 8 | `widgets/payments/ui/payments-global-search-screen.tsx` | `:88-97` (`pending`) | `:159` (`:162`) |

Декларации данных (для условий): `properties` — `properties-search-screen.tsx:56`;
`contacts` — `contact-book-search-screen.tsx:54`, `contacts-of-property-screen.tsx:111`,
`rental-contact-picker-screen.tsx:69`; `items` — `payments-objects-search-screen.tsx:49`,
`payments-global-search-screen.tsx:86`; `operations` — `operations-search-screen.tsx:81`,
`operations-global-search-screen.tsx:89`.

## Target

Единое правило, согласованное с осознанной политикой тихого удержания
(`operations-feed-gate.ts:11-13`: «недогрузка одного из соседей при
keepPreviousData — прежние данные, не скелетон»):

- держать **непустые** прежние результаты без индикации (осознано, не
  трогаем);
- держать **пустые** прежние результаты нельзя (они лгут) — пока запрос
  текущего критерия летит и показывать нечего, рисуется архетип скелетона
  поиска (тот же, что на первом лоаде);
- осевший пустой ответ — «Такого X нет», как сегодня.

Идиома правки: скелетон-гейт дополняется `(query.isFetching && <данные>.length === 0)`;
**пустая ветка не правится** — при осевшем пустом ответе `isFetching` уже
false.

### 1–4. Экраны с `isPending && searching` / `isPending` гейтом

`properties-search-screen.tsx:82`:

```tsx
{(searchQuery.isPending || (searching && searchQuery.isFetching && properties.length === 0)) ? (
  <PropertiesSearchSkeleton />
) : searchQuery.isError ? (
```

`contact-book-search-screen.tsx:80`:

```tsx
{(contactsQuery.isPending || (searching && contactsQuery.isFetching && contacts.length === 0)) ? (
  <ContactsSkeleton />
) : contactsQuery.isError ? (
```

`contacts-of-property-screen.tsx:177` (гейт без `searching` — книга и поиск
на одном запросе; расширять только поиск-ветку нельзя, скелетон общий —
ставим уточнение на `isFetching`-слагаемое):

```tsx
{(contactsQuery.isPending || (searching && contactsQuery.isFetching && contacts.length === 0)) ? (
```

`rental-contact-picker-screen.tsx:94` (поле поиска активно всегда, экран
целиком поисковый):

```tsx
{(contactsQuery.isPending || (contactsQuery.isFetching && contacts.length === 0)) ? (
```

### 5–8. Экраны с `pending`-флагом (гейт на `data === undefined`)

`operations-search-screen.tsx:86-92` — текущий код:

```tsx
const pending =
  !emptyQuery
  && (listQuery.data === undefined || summaryQuery.data === undefined)
  && !listQuery.isError
  && !summaryQuery.isError;
```

target:

```tsx
const pending =
  !emptyQuery
  && (listQuery.data === undefined
    || summaryQuery.data === undefined
    || (listQuery.isFetching && operations.length === 0))
  && !listQuery.isError
  && !summaryQuery.isError;
```

`operations-global-search-screen.tsx:111-116` — та же форма
(`operations`, `:89`; сводка через `summaryQuery`). Текущий код:

```tsx
const pending =
  !emptyQuery
  && (listQuery.data === undefined || summaryQuery.data === undefined)
  && !listQuery.isError
  && !summaryQuery.isError;
```

target — как у №5 (имена `listQuery`/`summaryQuery`/`operations` те же).

`payments-objects-search-screen.tsx:55-57` — текущий код:

```tsx
const pending =
  !emptyQuery && objectsQuery.data === undefined && !objectsQuery.isError;
```

target:

```tsx
const pending =
  !emptyQuery
  && (objectsQuery.data === undefined
    || (objectsQuery.isFetching && items.length === 0))
  && !objectsQuery.isError;
```

`payments-global-search-screen.tsx:88-97` — текущий код (два запроса,
элементы `items`, `:86`):

```tsx
const pending =
  !emptyQuery
  && (searchQuery.data === undefined || categoriesQuery.data === undefined)
  && !searchQuery.isError
  && !categoriesQuery.isError;
```

target:

```tsx
const pending =
  !emptyQuery
  && (searchQuery.data === undefined
    || categoriesQuery.data === undefined
    || (searchQuery.isFetching && items.length === 0))
  && !searchQuery.isError
  && !categoriesQuery.isError;
```

Порядок деклараций правки не требует: `operations` объявлен раньше
`pending` (`:81` → `:86`, `:89` → `:111`), `items` — тоже (`:49` → `:55`,
`:86` → `:88`).

## Repo conventions to follow

- Политика тихого удержания — `widgets/payments/lib/operations-feed-gate.ts:11-13`
  и комментарий `operations-global-search-screen.tsx:108-110` («Скелетон —
  только пока данных нет вовсе… не дёргает экран»): правка расширяет
  правило единственным случаем «держать нечего».
- Дебаунс единый — 300мс (`shared/lib/hooks/useSearchQueryState.ts:22`);
  правка его не трогает.
- e2e-прецедент проверки удержания — `e2e/history-search.spec.ts:223`
  («до ответа держится прежняя выдача (keepPreviousData)»).

## Steps

1. Экраны №1–4 — заменить гейты скелетона по Target-блокам (по одному
   условию).
2. Экраны №5–8 — расширить `pending` по Target-блокам.
3. Юнит/e2e: в стиле `e2e/history-search.spec.ts` добавить проверку «после
   пустого ответа правка запроса показывает скелетон, а не „Такого X нет“,
   до прихода нового ответа» на одной поверхности (properties-search) —
   остальными покрывает общий механизм.
4. `make frontend-test`; e2e поиска (`make frontend-e2e TESTS='-g Поиск'` —
   Docker).

## Boundaries

- Do NOT добавлять индикацию (dim/спиннер) на удержанных **непустых**
  результатах — политика тихого удержания осознана (gate-комментарии выше).
- Do NOT трогать error-ветки — ими ведает план 030.
- Do NOT трогать hint-кадры (`!searching` → «Введите название…») и
  дебаунс.
- Do NOT трогать `ContactsNoResults`/`SearchStateView`/`EmptyState` —
  меняются только условия ветвления.
- Если код не совпал с цитатами — STOP и доложить.

## Verification

- **Mechanical**: `make frontend-test` — зелёный; новый e2e — зелёный.
- **Feel check**: на поиске объектов: найти бессмысленную строку («Такого
  объекта нет») → дополнить строку символом → после дебаунса до ответа
  виден скелетон поиска (не пустое состояние) → осевший пустой ответ
  возвращает «Такого объекта нет»; найти существующую строку → дополнить
  символ → прежние результаты стоят без мигания (политика №1 не задета).
- **Done when**: ни одна из 8 поверхностей не показывает определительное
  «Такого X нет» для летящего запроса; удержанные непустые результаты
  стоят как раньше.
