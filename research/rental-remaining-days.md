# Ресерч #1383: семантика остатка аренды — дни вместо месяцев, когда осталось меньше месяца

Слово владельца 13.10: «если осталось меньше месяца — писать сколько осталось дней; сейчас пишется в месяцах». Карта — #1379 (не редактируется этим тикетом).

Источники: ADR 0053 §2 (`docs/adr/0053-rentals-context-schema-lifecycle-contracts.md`), бэкенд `apps/backend/internal/rentals/`, фронт `apps/frontend/`. Все пути абсолютны от корня репо; факты процитированы из кода.

---

## 1. Сервер: как считается monthsRemaining

`apps/backend/internal/rentals/application/rental_service.go:713-718` — для срочной аренды:

```go
remaining := domain.FullMonthsBetween(today, *rental.PlannedEndDate)
progress.MonthsRemaining = &remaining
```

`apps/backend/internal/rentals/domain/rental.go:252-268` — `FullMonthsBetween` считает **полные календарные месяцы** (month-index diff минус 1, если день `to` не дошёл до дня `from`); «a `to` before or on `from` counts nothing», `< 0` прижимается к 0.

**Ответ на вопрос тикета:** при остатке < месяца `monthsRemaining = 0`, **не 1** — floor, а не округление вверх. При плановом конце в прошлом — тоже 0 (функция никогда не отрицательна). Значение 0, таким образом, означает «меньше месяца ИЛИ план уже прошёл» — различить их можно только по статусу (§2 ADR: `needs_attention` при `planned_end_date < today`; в сам день планового окончания аренда ещё `active`, `apps/backend/internal/rentals/domain/rental.go:131-146`).

ADR 0053 §2 (строка 96): «`monthsRemaining` — полных календарных месяцев от `today` до `planned_end_date` (только срочная)»; §1 обоснование: «Сервер считает всё сам — клиент пояса не знает (прецедент tasks/payments); `today` — по TZ собственника (ADR 0048)»; §2: «`today` — в каждом ответе аренды». Контракт: `apps/backend/api/openapi/openapi.yaml:9163-9176` — `monthsRemaining` обязателен в `RentalProgress`. Фронт-тип: `apps/frontend/entities/rental/model/types.ts:57-65` (`RentalProgress`), `:69-87` (`Rental.plannedEndDate: IsoDate | null`, `Rental.today: IsoDate`).

## 2. Что пользователь видит сейчас при monthsRemaining = 0 — три поверхности, три разных текста

### 2.1. Детализация аренды — строки остатка нет вовсе

`apps/frontend/widgets/rentals/ui/rental-detail-body.tsx:88-96`:

```ts
const progressLine = indefinite
  ? rentalElapsedLine(rental.startDate, rental.today)
  : rentalRemainingLine(
      rental.progress.monthsRemaining !== null && rental.progress.monthsRemaining > 0
        ? rental.progress.monthsRemaining
        : null,
    );
```

Гвард `> 0` превращает 0 в `null` → `rentalRemainingLine(null)` → `undefined` (`apps/frontend/features/rentals/lib/rental-view.ts:76-82`) → строка **не рендерится**. Карточка прогресса при этом жива: `hasProgressCard` (`rental-view.ts:91-101`) истинна, пока есть будущее вхождение платежа (`nextPayment !== null` — оно есть всё время, пока плановый конец в будущем), есть просрочка, аренда бессрочная или `monthsRemaining > 0`. Итог: **в последние недели аренды пользователь видит только синюю строку «N дней до следующего платежа» и бар — подпись «Осталось …» исчезает.**

В сам день планового окончания: `nextPayment = null` (будущих вхождений нет — ADR 0053 §2 «null, когда будущего вхождения нет»), `monthsRemaining = 0`, просрочки нет → `hasProgressCard = false` — **карточка прячется целиком** (решение F1 от 23.09: «пустого контейнера макеты не рисуют»; тест `rental-view.test.ts:299-304` «F1: день планового окончания и „Ожидает действия“ — карточка скрыта целиком»). После планового конца — `needs_attention`, карточка скрыта, при задолженности — только красная строка `rentalOverdueLine`.

### 2.2. Блок «Аренда» на детали объекта — «Осталось 0 месяцев аренды»

`apps/frontend/widgets/property-detail/lib/rental-block.ts:72-79` — гварда нет:

```ts
footerLine = rentalRemainingLine(progress.monthsRemaining) ?? rentalElapsedLine(...);
```

При активной аренде с `monthsRemaining = 0` вызывается `rentalRemainingLine(0)`: `pluralize(0, 'Остался', 'Осталось', 'Осталось')` → many (`apps/frontend/shared/lib/pluralize.ts:13`) → **«Осталось 0 месяцев аренды»** — заведомо кривой текст. При `needs_attention` футер подменяется: «Срок аренды подошел к концу» (`rental-block.ts:57,73-74`).

### 2.3. Бейдж карточки объекта в списке — «Осталось меньше месяца»

`apps/frontend/features/properties/lib/property-badges.ts:30-45` (`rentalMonthsLeftLabel`, резолюция #584): остаток считается **клиентом** — `fullMonthsBetween(today, planned_end_date)` из списочного ответа (`apps/frontend/shared/lib/calendar.ts:92-100`, зеркальная копия серверной функции); при `months <= 0` — «Осталось меньше месяца», при 1 — «Остался 1 месяц аренды». **Это готовый прецедент клиентской надстройки над серверным каноном** — бейдж уже считает остаток сам из `today` + `planned_end_date` (резолюция #584).

**Сводка (a):** детализация — пусто (только строка дней до платежа), в день плана карточка целиком скрыта; блок объекта — «Осталось 0 месяцев аренды»; бейдж списка — «Осталось меньше месяца». Слово владельца справедливо для всех трёх.

## 3. Предлагаемое правило дней

**Источник данных:** `days = rental.plannedEndDate − rental.today` — оба поля уже в ответе аренды (ADR 0053 §2/§4), календарные дни. Готовых хелперов два: `daysOverdue(due, today) = today − due` (`calendar.ts:50-52`, обратный порядок) и серверный канон именования `daysUntil = date − today` (ADR 0053 §2, `RentalNextPayment.daysUntil`, `types.ts:35`). Чище добавить симметричный `daysUntil(from, to)` в `shared/lib/calendar.ts` (или считать `−daysOverdue(plannedEndDate, today)`), чем нагружать просрочный хелпер.

**Правило переключения:** серверный `monthsRemaining` остаётся триггером —
- `monthsRemaining ≥ 1` → месячная строка как сегодня («Осталось 23 месяца аренды»);
- `monthsRemaining = 0` → строка дней по `days = plannedEndDate − today`:
  - `days > 0` → «Остался 1 день аренды» / «Осталось 2 дня аренды» / «Осталось 5 дней аренды»;
  - `days = 0` (план сегодня, статус ещё `active`) → «Заканчивается сегодня»;
  - `days < 0` (план прошёл) → строку дней **никогда не рисовать**: поверхности уже переключены статусом — блок объекта «Срок аренды подошел к концу», детализация прячет карточку (F1) или показывает только просрочку, бейдж «Аренда завершена» (`property-badges.ts:56-58`).

**Плюрализация — уже решена в коде:**
- существительное: `formatOverdueDays(days)` = `${days} ${pluralize(|days|, 'день', 'дня', 'дней')}` (`apps/frontend/shared/lib/date-format.ts:92-94`) — тот же хелпер, что рендерит «150 дней до следующего платежа» (`rental-view.ts:62`);
- глагол: `pluralize(days, 'Остался', 'Осталось', 'Осталось')` — схема `rentalRemainingLine` (`rental-view.ts:80`);
- «Заканчивается сегодня» — прецедент «Платёж сегодня» при `daysUntil <= 0` (`rental-view.ts:58-63`).

**Незадеты:** бессрочные и срочные без `plannedEnd` — `monthsRemaining = null` → строки остатка нет (как сейчас), бейдж `null`; завершённые — отдельный экран (детализация completed), блока бейджа не касаются.

Открытое слово владельца (одна строка в резолюции): оставить ли карточку прогресса скрытой в сам день планового окончания (F1 от 23.09) при появлении текста «Заканчивается сегодня» — предложение: F1 сохранить (на детализации в этот день карточка по-прежнему скрыта, строка «Заканчивается сегодня» живёт в футере блока объекта и бейдже), т.к. F1 формулировался про пустоту контейнера до появления дней.

## 4. Что менять (список правок)

1. **`apps/frontend/features/rentals/lib/rental-view.ts`** — `rentalRemainingLine(monthsRemaining: number | null)` → метка от дат: `rentalRemainingLine(today: IsoDate, plannedEndDate: IsoDate)` (внутри: месяцы при `monthsRemaining ≥ 1` — серверный счётчик придётся передать третьим аргументом, либо правило целиком от дат через клиентский `fullMonthsBetween`, как у бейджа #584). **Сигнатура «принимать rental» не годится**: бейдж списка не имеет `Rental` — он работает с `occupancy` списочного ответа (`property-badges.ts:51-76`), поэтому минимум — `(today, plannedEndDate)`.
2. **`hasProgressCard(nextPayment, progress)`** (`rental-view.ts:91-101`) — сигнатуру расширить датами (или принимать `Rental`): в окне «последний платёж уже прошёл, до конца несколько дней» (`nextPayment = null`, `monthsRemaining = 0`, `days > 0`, просрочки нет) нынешнее условие спрячет карточку, хотя строка дней есть. Условие `(progress.monthsRemaining ?? 0) > 0` → «есть строка остатка (месяцы, или дни > 0)».
3. **`apps/frontend/widgets/rentals/ui/rental-detail-body.tsx:90-96`** — снять гвард `> 0`, передавать даты в новую метку.
4. **`apps/frontend/widgets/property-detail/lib/rental-block.ts:72-79`** — та же метка от `rental.today`/`rental.plannedEndDate`; `days = 0` → «Заканчивается сегодня» в футере; ветка `needs_attention` не трогается.
5. **`apps/frontend/features/properties/lib/property-badges.ts:33-45`** — внутренности `rentalMonthsLeftLabel` заменить общей меткой (текст «Осталось меньше месяца» уходит; ветка `needs_attention` → «Аренда завершена» остаётся). Слово владельца 13.10 бьёт и по бейджу — там сейчас ровно «меньше месяца» без дней.
6. **`apps/frontend/shared/lib/calendar.ts`** — симметричный хелпер `daysUntil(from, to)` (по образцу серверного `daysUntil`), либо переиспользование `−daysOverdue`.

Альтернатива (отклоняемая): оставить `rentalRemainingLine` для месяцев и добавить отдельную `rentalRemainingDaysLine(days)` — тогда правило переключения месяцев/дней дублируется во всех трёх местах; по словарю codebase-design одна функция-метка от `(today, plannedEndDate)` глубже.

## 5. Соотношение с серверным monthsRemaining (ADR 0053)

Канон §1/§2 «Сервер считает всё сам — клиент пояса не знает» **не ломается**:
- `monthsRemaining` остаётся серверным счётчиком полных месяцев и триггером переключения;
- дни — рендер-правило над двумя уже серверными полями (`plannedEndDate`, `today` в TZ собственника): клиент не получает никакой поясной логики — это то же форматирование, что уже делает `nextPayment.daysUntil` → «N дней до следующего платежа»;
- прецедент клиентской надстройки в проде: бейдж #584 считает остаток клиентом из `today` + `planned_end_date` списочного ответа.

**Бэкенд-эскалация** (`daysRemaining` в `RentalProgress`: правка ADR 0053 §2/§4 + `apps/backend/api/openapi/openapi.yaml` + регенерация клиента) нужна только если (1) правило дней станет входом серверных решений — push-уведомление «осталось 3 дня», тексты в админке — или (2) владелец хочет строгий канон. Сейчас потребителей `MonthsRemaining` вне rentals нет (grep по `apps/backend` и `apps/admin` — только rentals-сервис, хендлер `rental_handlers.go:595`, openapi-ген). Рекомендация: клиентская надстройка.

## 6. Затронутые тесты

Unit:
- `apps/frontend/features/rentals/lib/rental-view.test.ts:154-168` — `rentalRemainingLine`: месяцы со склонением и `null → undefined`; переписывается под новую сигнатуру + новые кейсы дней («Остался 1 день», «Осталось 2 дня», «Осталось 5 дней», «Заканчивается сегодня»).
- `apps/frontend/features/rentals/lib/rental-view.test.ts:293-326` — `hasProgressCard`: кейс F1 (`monthsRemaining: 0`, `nextPayment: null` → скрыта) остаётся при `days = 0`; добавить кейс «`nextPayment: null`, `monthsRemaining: 0`, `days > 0` → карточка жива ради строки дней».
- `apps/frontend/widgets/property-detail/lib/rental-block.test.ts` — футер: добавить активную с `monthsRemaining: 0` (`days > 0` → «Осталось N дней аренды»; `days = 0` → «Заканчивается сегодня»); существующие кейсы `:105-110` (needs_attention → «Срок аренды подошел к концу») и `:120-126` (бессрочная → «Прошло 5 месяцев») не меняются.
- `apps/frontend/features/properties/lib/property-badges.test.ts:35-37` — кейс «N=0 — „Осталось меньше месяца“» меняется на дни (fixture `2026-09-10` → `2026-09-25` даст «Осталось 15 дней аренды»).

E2e (`apps/frontend/e2e/`): прямых assert'ов на строку остатка нет — grep «Осталось|Остался» по спекам пуст. `property-rental-actions.spec.ts:223` (`getByText('8 месяцев')`) — экран «Прошлых аренд», строка «Срок аренды» условий, не задет. По желанию владельца — новый e2e-сценарий рендера блока при остатке < месяца.

Бэкенд: не задет — серверная семантика не меняется (`rental_service_test.go`, домен-тесты `FullMonthsBetween` остаются).

## 7. Резолюция

**Правило дней:** при `monthsRemaining ≥ 1` — месячная строка как сегодня; при `monthsRemaining = 0` — `days = plannedEndDate − today` (календарные): «Остался 1 день / Осталось 2 дня / Осталось 5 дней аренды», `days = 0` — «Заканчивается сегодня», `days < 0` — строку не рисовать (статусные тексты уже владеют сценой).

**Список правок:** метка от `(today, plannedEndDate)` в `rental-view.ts` (замена сигнатуры `rentalRemainingLine`), расширение `hasProgressCard` датами, оба рендера (`rental-detail-body.tsx`, `rental-block.ts`), бейдж `rentalMonthsLeftLabel` (#584) на общую метку, хелпер `daysUntil` в `calendar.ts`. Бессрочные/без `plannedEnd` не задеты. Канон ADR 0053 не ломается — клиентская надстройка над серверными полями; серверный `daysRemaining` — только при эскалации (push/админка).
