# 032 — Хвост ленты: повтор при провале порции

- **Status**: TODO
- **Commit**: 2a37f5bc5
- **Severity**: LOW
- **Category**: Error policy (DESIGN.md §6 «Хвост бесконечной ленты» #633)
- **Estimated scope**: 1 канон-компонент + тест; вызовы не трогаются

## Problem

Канон хвоста — `InfiniteQueryTail` (`shared/ui/design/infinite-query-tail.tsx`).
Его срез запроса не знает об ошибке порции:

```tsx
export type InfiniteQueryTailQuery = {
  readonly hasNextPage: boolean | undefined;
  readonly isFetchingNextPage: boolean;
  readonly fetchNextPage: () => Promise<unknown>;
};
```

Рендер (`:50-58`): `isFetchingNextPage` → `LoadingMoreIndicator`, иначе —
только sentinel. Провал `fetchNextPage` (после трёх попыток `queryRetry`,
`shared/providers/query-retry.ts`) ничего не показывает: пользователь
скроллит в пустоту без объяснения и без повтора.

Хуже: провал запускает **немой автоповтор**. `useInfiniteScroll`
(`shared/lib/hooks/useInfiniteScroll.ts`) пересоздает обсервер при смене
`resetKey` (= `isFetchingNextPage`), и первичный отчет пересекающего
sentinel стреляет снова — докблок прямо это обещает («Пересоздание даёт
свежий первичный отчёт: пока sentinel видим, дозагрузка продолжается»).
При устойчивой ошибке лента уходит в бесконечный цикл «индикатор мигнул —
ничего не пришло», невидимый пользователю.

Механика ошибки проветрена по исходникам v5: неудачная `fetchNextPage`
диспатчит `{type: "error"}` (`query-core/query.js:319-325`), редьюсер ставит
`status: "error"` безусловно (`:375-385`) — `query.isError === true` при
живых страницах. Значит срезу достаточно опционального `isError`.

## Target

```tsx
export type InfiniteQueryTailQuery = {
  readonly hasNextPage: boolean | undefined;
  readonly isFetchingNextPage: boolean;
  readonly fetchNextPage: () => Promise<unknown>;
  /** Провал порции (v5: error при живых страницах). Опционален — срез
   * структурный, проекция графика платежей его не передаёт. */
  readonly isError?: boolean;
};
```

Рендер — после `if (!hasNext) return null;`:

```tsx
if (!hasNext) {
  return null;
}
if (query.isError) {
  // Провал порции: sentinel снят (рвёт немой автоповтор обсервера),
  // повтор — канон-карточка с фазой (план 031); fetchNextPage доигрывает
  // следующую страницу, прежние страницы не трогаются.
  return (
    <ErrorCard
      title="Не удалось загрузить еще"
      onRetry={() => void query.fetchNextPage()}
      loading={query.isFetchingNextPage}
      className={className}
    />
  );
}
return (
  <>
    <div ref={sentinelRef} aria-hidden />
    {query.isFetchingNextPage && <LoadingMoreIndicator tone={tone} className={className} />}
  </>
);
```

В `onRetry` — именно `fetchNextPage`, не `refetch()`: v5 `refetch()`
перечитывает **все** загруженные страницы, `fetchNextPage` доигрывает
только следующую.

## Repo conventions to follow

- Карточка повтора — канон `ErrorCard` (§7, #697) с фазой из плана 031
  (`loading={isFetchingNextPage}` — глушение машинное, #1119).
- Индикатор догрузки остаётся `LoadingMoreIndicator` (#633); докблок
  канона (`infinite-query-tail.tsx:26-28`) про график платежей не трогается.
- Тип среза структурный — все 19 канон-вызовов передают объект
  `UseInfiniteQueryResult` целиком (`query={searchQuery}`), `isError`
  протечёт без правок вызовов; ui-kit-заглушка
  (`app/ui-kit/design-layer/sections/feedback.tsx:184-210`) поле не
  передаёт — поведение не меняется.

## Steps

1. TDD: `shared/ui/design/infinite-query-tail.test.tsx` — по образцу
   `button.test.ts`: (a) `isError: true` + `hasNextPage: true` → карточка
   «Не удалось загрузить еще», sentinel-дива нет; (b) клик «Повторить» →
   `fetchNextPage` вызван; (c) `loading: true` → кнопка глушится; (d)
   `isError` не передан → прежнее поведение (sentinel + индикатор).
   Тесты хука не дублировать (обсервер покрыт логикой рендера).
2. `infinite-query-tail.tsx` — тип и рендер по Target.
3. Вызовы не править (протечёт структурно). Проверить, что ни один вызов
   не передаёт собственный `isError`-стаб.
4. `make frontend-test`; регресс e2e лент (прокрутка до порций).

## Boundaries

- Do NOT трогать `useInfiniteScroll` — механика пересоздания канонична
  (#631), правится только потребитель хвоста.
- Do NOT трогать `payment-schedule-screen` — проекция без react-query,
  документированное исключение (докблок канона `:26-28`).
- Do NOT трогать `InfiniteQueryHead`-вызовы отдельно (тот же компонент —
  история получает фикс автоматически).
- Порядок: строго после плана 031 (нужен `loading` у ErrorCard).
- Если код не совпал с цитатами — STOP и доложить.

## Verification

- **Mechanical**: новые тесты хвоста — зелёные; `make frontend-test` —
  зелёный; e2e лент с порциями (history, operations, contacts) — зелёные.
- **Feel check**: на длинной ленте (например, история объекта) оборвать
  сеть перед порцией — после трёх попыток в хвосте карточка «Не удалось
  загрузить еще», автоповтор замолчал (Network: без цикла запросов);
  вернуть сеть, «Повторить» — кнопка глохнет, порция приезжает, карточка
  сменяется sentinel'ом; скролл продолжает дозагрузку.
- **Done when**: провал порции видим и повторяем; немой цикл автоповтора
  невозможен; обычная дозагрузка без ошибок — как раньше.
