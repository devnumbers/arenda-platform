# 030 — Ошибка перечитывания не свопит удержанные данные (константные ключи)

- **Status**: TODO
- **Commit**: 2a37f5bc5
- **Severity**: MEDIUM
- **Category**: Error policy (DESIGN.md §7 ErrorCard, §14 «держат прежнюю выдачу»)
- **Estimated scope**: 3 экрана + абзац политики в `apps/frontend/DESIGN.md` §7

## Problem

react-query v5 ставит `status: "error"` **безусловно** при любом провале
фетча — даже когда данные уже в кэше. Исходники (проветрено на
`@tanstack/query-core` 5.x из node_modules):

- `query.js:319-325` — catch любого rejection диспатчит `{type: "error"}`,
  включая неудачную `fetchNextPage` бесконечной ленты;
- `query.js:375-385` — редьюсер: `status: "error"` без гарда на `data`;
- `queryObserver.js:256` — `let { error, errorUpdatedAt, status } = newState;`
  — наблюдатель отдаёт `isError: true` при живых данных.

(полные пути: `node_modules/@tanstack/query-core/build/modern/query.js`,
`node_modules/@tanstack/query-core/build/modern/queryObserver.js`.)

Канон retry-политики (`shared/providers/query-retry.ts`): 4xx не ретраются,
5xx/сеть — до 3 попыток. Значит путь реален: лента с данными + сетевой
сбой на перечитывании (SSE-инвалидация, повторный вход, `fetchNextPage`) →
после трёх попыток `isError: true` при удержанных данных → все error-ветки
без гарда данных **свопят загруженный контент на ErrorCard**.

Сегодня этот гард есть только косвенно — там, где экран держит
`pending`-гейт на `data === undefined` и ошибку рисует отдельной веткой.
На константных ключах (нет серверных фильтров — ключ не меняется после
монтирования) единственный источник такого провала — перечитывание
существующего запроса, и своп удержанных данных — чистый вред: пользователь
теряет загруженный список без сохранения контекста.

Проветренные места (константные ключи, error-ветка без гарда данных):

1. `widgets/payments/ui/payment-overdue-screen.tsx:72-87`:

```tsx
{overdueQuery.isError && (
  <PaymentsStateCard
    title="Не удалось загрузить просроченные"
    hint="Проверьте подключение и попробуйте снова"
    ...
```

и гейт списка `:88`: `{!overdueQuery.isPending && !overdueQuery.isError && (` —
при ошибке с данными список скрывается целиком.

2. `widgets/payments/ui/payment-history-screen.tsx:118-121`:

```tsx
const failed = changesMode
  ? operationsQuery.isError || changesQuery.isError
  : operationsQuery.isError;
```

(оба запроса константного ключа экрана; `changesQuery` включается
`enabled`-гейтом режима «Изменения» — его первичный провал обязан остаться
карточкой, гард делается по-запросный; ветка рендера по `failed` свопит
ленту на карточку.)

3. `widgets/participants/ui/participants-list-screen.tsx:134`:

```tsx
const listError = isPaid && participantsQuery.isError;
```

ветка `:268` рендерит `ErrorCard` вместо списка (ключ `/participants`
константный — серверных фильтров у списка нет).

## Политика (записывается в DESIGN.md §7 исполнителем этого плана)

- **Данных нет** (первичный провал) → ErrorCard, как сегодня — канон §7.
- **Данные есть, провал перечитывания** (константный ключ) → экран остаётся
  на удержанных данных; повтор доступен теми же путями, что и перечитывание
  (хвост ленты — план 032; действия экрана).
- **Ключ переменный** (поиск, фильтры — KPD): своп на ErrorCard при провале
  нового ключа **сохраняется** — это честно (удержанные данные отвечают на
  чужой запрос); тихое удержание летящего запроса — план 029.
  Обоснование не трогать: отличить «провал нового ключа» от «провала
  перечитывания» флагами v5 нельзя.

## Target

Гард «данных нет» на error-условии трёх экранов:

`payment-overdue-screen.tsx:72`:

```tsx
{(overdueQuery.isError && overdueQuery.data === undefined) && (
```

`payment-overdue-screen.tsx:88` (список живёт при ошибке с данными):

```tsx
{!overdueQuery.isPending && (!overdueQuery.isError || overdueQuery.data !== undefined) && (
```

`payment-history-screen.tsx:118-121`:

```tsx
const failed = changesMode
  ? (operationsQuery.isError && operationsQuery.data === undefined)
    || (changesQuery.isError && changesQuery.data === undefined)
  : operationsQuery.isError && operationsQuery.data === undefined;
```

(гард по-запросный: `changesQuery` живёт только в режиме «Изменения» — его
первичный провал без данных остаётся карточкой; провал перечитывания
загруженного источника — удержанные данные.)

`participants-list-screen.tsx:134`:

```tsx
const listError = isPaid && participantsQuery.isError && participantsQuery.data === undefined;
```

## Repo conventions to follow

- Политика pending — `widgets/payments/lib/operations-feed-gate.ts:11-13`
  («pending — только пока данных нет вовсе»); правка дополняет её
  зеркальным правилом для error.
- Прецедент «ошибка без данных» — комментарий `payments-global-screen.tsx:94-95`
  («ошибка без данных показывает карточку повтора, не скелетон»).
- ErrorCard остаётся каноном первичного провала (§7, #697).

## Steps

1. `payment-overdue-screen.tsx` — два условия по Target.
2. `payment-history-screen.tsx` — `failed` по Target (гард по-запросный).
3. `participants-list-screen.tsx` — `listError` по Target.
4. Зафиксировать политику абзацем в `apps/frontend/DESIGN.md` §7
   (после пункта про ErrorCard): «Ошибка перечитывания при удержанных
   данных (константный ключ) контент не подменяет — экран остаётся на
   удержанной выдаче, повтор доступен в хвосте ленты (#1422, план 032);
   ErrorCard — только провал без данных».
5. Юнит-тест на `operations-feed-gate`-уровне не нужен (гейты в экранах) —
   регресс: `make frontend-test`; e2e лент платежей.

## Boundaries

- Do NOT трогать error-ветки экранов с переменными ключами (поиск, хабы
  операций/задач/уведомлений/истории — периоды, фильтры, скоупы) — там
  своп осознан (см. Политику).
- Do NOT трогать `ErrorCard` (план 031) и хвосты (план 032).
- Do NOT добавлять индикацию на удержанных данных.
- Если код не совпал с цитатами — STOP и доложить.

## Verification

- **Mechanical**: `make frontend-test` — зелёный; `make frontend-e2e
  TESTS='-g Просроченные|История платежа|Участники'` — зелёные.
- **Feel check**: на `/payments/overdue` (или history) с данными —
  оборвать сеть (DevTools Offline), спровоцировать перечитывание (вкладка
  возрастом > staleTime, действие экрана) — после трёх попыток список
  остаётся на экране, карточки ошибки нет; вернуть сеть, повторить —
  данные обновляются. Первичный провал (сеть от старта) — ErrorCard.
- **Done when**: ни один из трёх экранов не свопит удержанные данные на
  ErrorCard; первичные провалы ведут себя как сегодня.
