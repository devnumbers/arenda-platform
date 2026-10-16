# Research: endDate раньше первого вхождения — лучшие практики и дизайн валидации

Тикет #1150 (часть #1147). Баг: периодический Платёж (правило) с `endDate` между `since` и
первым вхождением графика молча не создаёт ни одной Операции. Пример: weekly «каждую пятницу»,
since = среда, endDate = четверг. То же у Платежа аренды: monthly по дню оплаты, since = старт
аренды, `end_date` = плановый конец — старт 10-го, день оплаты 25-е, конец 20-го → 0 платежей.

Решение владельца: нельзя выбирать дату окончания раньше первого вхождения (создание и правка
платежа; создание и правка аренды). Отдельное решение владельца (2026-10-06): при правке платежа,
если смена периодичности сделала стоящую endDate невалидной — endDate сбрасывается **молча, без
подсказки**.

Структура: 1 — внешние практики, 2 — бекенд payments, 3 — бекенд rentals (вердикт по каждой
точке из уточнения владельца), 4 — фронт, 5 — тест-кейсы для TDD. В конце — разбивка на 3 тикета.

---

## 1. Внешние практики

### iCal RFC 5545 (RRULE/UNTIL)

По первоисточнику (RFC 5545 §3.3.10 «Recurrence Rule», datatracker.ietf.org/doc/html/rfc5545):

- UNTIL ограничивает правило **включительно**: при совпадении с очередным вхождением «becomes
  the last instance of the recurrence» — endDate == первое вхождение даёт ровно одно вхождение.
- Нормативные требования UNTIL: тот же тип значения, что DTSTART (DATE ↔ DATE, DATETIME ↔
  UTC-DATETIME), и взаимоисключимость с COUNT.
- **Нормативного требования «UNTIL ≥ DTSTART / первое вхождение» НЕТ.** Если UNTIL стоит раньше
  первого вхождения, множество вхождений просто пусто — ни ошибки, ни определённого поведения.
  Библиотеки (rrule.js, python-dateutil) молча выдают пустое множество — в точности наш баг.

Вывод: стандарт сам не спасает — «окно» графика является продуктовым инвариантом, и валидировать
его должна сама система, а не библиотека повторений.

### Stripe subscriptions

По первоисточнику (docs.stripe.com/api/subscriptions/create):

- Первый инвойс финализируется **в самом запросе создания**: «with `collection_method=
  charge_automatically`, the first invoice is finalized as part of the request». То есть
  «ноль инвойсов» невозможен **конструкцией** — Stripe не валидирует «хотя бы один период»,
  потому что гарантирует его сам: подписка всегда биллит старт своего цикла.
- `cancel_at` — будущий timestamp; доки описывают только семантику прорейтов («If set to a date
  before the current period ends, this will cause a proration…»), отдельной валидации
  «не раньше первого периода» нет — она не нужна.

Вывод: Stripe решает проблему **моделью** (первое вхождение = старт цикла), а не валидацией.
У нас график якорный (первое вхождение ≠ since), поэтому модельную гарантию повторить нельзя —
нужен явный guard. Косвенный урок: сервер обязан отклонять невозможное, а не молча принимать.

### Google Calendar

- Валидация повторений — **на сервере (слое API)**: некорректное RRULE при создании/правке
  события отклоняется с 400 («Invalid recurrence rule»); клиентский UI дополнительно не даёт
  построить невалидное правило. Описан и обратный класс проблем — «молчаливые» успешные ответы
  без применения правила (discuss.google.dev, тред «Google Calendar API sometimes silently fails
  to update recurrence rules») — аргумент за громкое отклонение вместо тихого игнорирования.

Вывод (сводный по трём системам):

| Система | Слой | Механика |
|---|---|---|
| RFC 5545 | нет | пустое множество молча — инвариант остаётся за продуктом |
| Stripe | модель | первое вхождение = старт, «ноль инвойсов» невозможен |
| Google | сервер API + UI | 400 на невалидное правило, UI-профилактика |

Наш дизайн: **UI-профилактика (minDate пикера + автосброс) + серверный отклоняющий 400 как
бэкстоп** — канонический двухслойный ответ.

---

## 2. Бекенд payments

### Текущее состояние

- `apps/backend/internal/payments/domain/occurrences.go` — чистое ядро:
  `OccurrencesBetween(p Payment, start, end)` (нижняя граница `Since`, верхняя — `EndDate`
  включительно, паузы вырезаются, потолок `maxOccurrences = 1000`) и `NextOccurrenceAfter(p, date)`
  (горизонт `addYearsClamped(date, 5)`).
- `apps/backend/internal/payments/application/payment_service.go:466` — `validateRule`, единый
  валидатор контракта create/update; о дате окончания только:

  ```go
  if rule.EndDate != nil && rule.EndDate.Before(rule.Since) {
      return ErrInvalidInput
  }
  ```

  Вызовы: `CreatePayment` (строка 173; `Since: today` — «сегодня» владельца, ADR 0048, сервер
  ставит сам, не редактируется) и `UpdatePayment` (строка 284 — **после** `applyUpdate`, то есть
  валидируется слитое правило: смена периодичности при стоящей endDate видна валидатору).
- Ошибки: `ErrInvalidInput` (ports.go:28) → 400 с фиксированным детейлом «Некорректные данные
  платежа» из таблицы `userFacingDetails` (`internal/platform/httpsupport/problem.go`). Отдельные
  коды ошибок мапит таблица `staticPaymentProblems` (`internal/payments/adapters/http/
  payment_handlers.go:67`): `{Err, Status, Title, Detail}`, консультируется **раньше** общей
  таблицы — готовый механизм для специфичного сообщения.
- DB: `payments_since_end_date_check` (`db/migrations/000115_payments_context.up.sql:44`,
  `end_date IS NULL OR end_date >= since`) — «первое вхождение» в SQL невыразимо (зависит от
  recurrence), CHECK остаётся как есть, валидация — только в application-слое.
- Управляемый платёж аренды строится в `cmd/api/wire/rentals.go:118` (`Since = seed.StartDate`,
  `EndDate = seed.PlannedEndDate`) и **не проходит** через `CreatePayment` — его валидирует
  rentals (раздел 3).

### Дизайн

**Домен.** В `internal/payments/domain/occurrences.go` — чистая функция первого вхождения
**без учёта EndDate** (валидатор спрашивает «когда было бы первое вхождение, если бы окончания
не стояло»; наивный вызов `OccurrencesBetween` с текущим правилом вернул бы пустое множество
именно в багованном случае — та же ловушка уже отражена в доке фронтового аналога):

```go
// FirstOccurrence returns the rule's earliest schedule occurrence at or after
// Since; EndDate is ignored (the window validator asks where the schedule
// would start, whatever end stands), pauses cut. False — the schedule never
// fires (an open pause over everything).
func FirstOccurrence(p Payment) (time.Time, bool) {
    probe := p
    probe.EndDate = nil
    ds := OccurrencesBetween(probe, p.Since, addYearsClamped(p.Since, 5))
    if len(ds) == 0 {
        return time.Time{}, false
    }
    return ds[0], true
}
```

**Переиспользование `NextOccurrenceAfter` с горизонтом — да, ок и для yearly.** Эквивалент:
`NextOccurrenceAfter(probe, p.Since.AddDate(0, 0, -1))` — горизонт `addYearsClamped(since-1, 5)`
покрывает yearly с запасом (якорное первое вхождение ≤ 1 года от since). Перечисление дёшево для
всех видов: daily — 1-й шаг, weekly — ≤ 7 дневных шагов, monthly/yearly — якорь от собственного
месяца/года правила, потолок `maxOccurrences` не достигается. Обе формы корректны; вариант выше
зеркалит фронтовый `firstOccurrence` (раздел 4) и не зависит от семантики «строго после».

**Application.** Новый сентинел в `internal/payments/application/ports.go` рядом с
`ErrInvalidInput`:

```go
ErrEndDateBeforeFirstOccurrence = errors.New("payments: end date before first occurrence")
```

Проверка — отдельной чистой функцией рядом с `validateRule` (не смешивать с общим
`ErrInvalidInput`-валидатором), и **триггер узкий**:

```go
// validateRuleWindow enforces the schedule window: the end date never stands
// before the schedule's first occurrence (ticket #1150).
func validateRuleWindow(rule domain.Payment) error {
    if rule.EndDate == nil {
        return nil
    }
    first, ok := domain.FirstOccurrence(rule)
    if ok && rule.EndDate.Before(first) {
        return ErrEndDateBeforeFirstOccurrence
    }
    return nil
}
```

- `CreatePayment`: `validateRule(draft)` → затем `validateRuleWindow(draft)` (всегда).
- `UpdatePayment`: `validateRule(rule)` → `validateRuleWindow(rule)` **только когда команда
  трогает расписание или окончание** (`cmd.Recurrence != nil || cmd.EndDate != nil`).
  Обоснование: «сломанные» правила (созданные до фикса) с дыркой не должны блокировать
  несвязанные правки (title/amount/категория) — на экране появилось бы не относящееся к делу
  сообщение. Прецедент узкой валидации: фронтовый `buildRentalUpdateCommand`
  (`apps/frontend/features/rentals/lib/edit-model.ts`) — «валидация только того, что уходит в
  PATCH», и бекендовый `paymentSyncFromUpdate` в rentals валидирует только изменённые поля.

**HTTP.** В `staticPaymentProblems` (payment_handlers.go) — запись (таблица консультируется
раньше ветки `ErrInvalidInput`, конфликтов нет):

```go
{
    Err: application.ErrEndDateBeforeFirstOccurrence, Status: http.StatusBadRequest,
    Title: "Bad request", Detail: "Дата окончания не может быть раньше первого платежа",
},
```

Контракт для фронта: 400, `application/problem+json`, деталь готова к показу пользователю
(тот же конвейер, что у 409-таблицы: `writePaymentsError` → `httpsupport.WriteErrorProblem`).
Сентинел **не** оборачивает `ErrInvalidInput`, чтобы общий детейл «Некорректные данные платежа»
не перебивал специфичный.

### Пограничные случаи

| Случай | Поведение |
|---|---|
| daily | первое вхождение = since: новый guard не добавляет ничего к существующему `EndDate ≥ Since` — проверка не срабатывает, оба остаются |
| endDate == первое вхождение | **валидно** (UNTIL-семантика RFC 5545 и `OccurrencesBetween` включают hardEnd — вхождение дня окончания материализуется) |
| endDate == since | daily — валидно; weekly/monthly/yearly — отклоняется, если первое вхождение позже (это и есть баг) |
| правка recurrence при стоящей endDate | сервер валидирует **слитое** правило: явная невалидная пара → 400 с новым сообщением. Фронт по решению владельца 2026-10-06 шлёт `endDate: null` (молчаливый сброс) → слитое правило без EndDate валидно → принятие. Сервер не «знает» о молчаливом сбросе — он просто принимает очистку; расхождение фронтового расчёта с Go ловится 400-бэкстопом |
| title-only PATCH на легаси-правиле с дыркой | принимается (узкий триггер — см. выше) |
| паузы | отдельного случая нет: `FirstOccurrence` идёт через `OccurrencesBetween` и паузы естественно вырезает; на create пауз не существует, на update открытая пауза стартует от «сегодня» владельца и не накрывает прошедшее первое вхождение. Семантика совпадает с фронтовым `firstOccurrence` |
| часовые пояса владельца | `since` — серверная календарная дата владельца (OwnerCalendar, `users.timezone`, ADR 0048); валидация — чистая дата-арифметика в UTC-полуночах, как весь occurrences.go, off-by-one на бекенде нет. Риск только на фронте — раздел 4 |
| performance | первый кандидат близко: daily 1 итерация, weekly ≤ 7, monthly ≤ 2 месяца-кандидата, yearly ≤ 2 якоря; горизонт 5 лет покрывает yearly; `maxOccurrences` не достигается. O(1) в каждом виде |

---

## 3. Бекенд rentals

### Текущее состояние

- `apps/backend/internal/rentals/application/rental_service.go:654` — `validateCreate`:
  `PlannedEndDate > StartDate` и `StartDate ≥ today` — про первое вхождение ничего.
- `paymentSyncFromUpdate` (строка 686) + `validatePlannedEnd` (строка 721): правка условий —
  `!plannedEnd.After(start) || plannedEnd.Before(today) → ErrInvalidInput`. День оплаты меняется
  той же командой (`cmd.PaymentDay`), старт **не редактируется вовсе** (`UpdateRentalCommand`
  без StartDate, комментарий ADR 0053 §3). `domain.Rental` дня оплаты не хранит (решение №5 —
  день живёт в recurrence платежа); текущий день читается гейтвеем `RentPaymentState`.
- Завершение: `CompleteRental` (строка 436) → `validateCompletedDate` (`start ≤ дата ≤ today`) →
  `stores.pay.Stop` = `cmd/api/wire/rentals.go:189`: `payment.EndDate = &completedDate` +
  удаление planned строго после завершения.
- Продление — **отдельного бекенд-пути нет**: экран `apps/frontend/widgets/rentals/ui/
  rental-extend-screen.tsx:158` шлёт `useUpdateRental({ plannedEndDate })` — тот же PATCH, что и
  правка условий, тот же `validatePlannedEnd`.
- Управляемый платёж: `cmd/api/wire/rentals.go:118` — monthly по дню оплаты (`recurrenceForDay`:
  1..30 → свой день, 31 и «последний день» → маркер последнего), `Since = StartDate`,
  `EndDate = PlannedEndDate`. Через `CreatePayment` не идёт — валидировать надо в rentals.
- Минимальная функция первого вхождения дня оплаты — чистая дата-арифметика:

```go
// FirstPaymentDate returns the first payment-day date on or after start:
// 1..30 keep their day (clamped to the month's length), 31 and «последний
// день» give the month's actual last day.
func (d PaymentDay) FirstPaymentDate(start time.Time) time.Time
```

  Место: `internal/rentals/domain/rental.go` рядом с `PaymentDay` (та же дата-арифметика, что в
  `payments/domain` `dateInMonth`, но без зависимости rentals → payments).

### Вердикты по каждой точке (уточнение владельца 2026-10-06)

| Точка | Вердикт | Guard |
|---|---|---|
| **Создание** (визард, шаг условий) | **НУЖЕН, оба слоя** | Бекенд: в `validateCreate` — `cmd.PlannedEndDate != nil && cmd.PlannedEndDate.Before(cmd.PaymentDay.FirstPaymentDate(cmd.StartDate)) → ErrInvalidInput` (в дополнение к `> start`). Фронт: minDate пикера + автосброс (раздел 4) |
| **Правка условий** (rental-terms-edit-screen) | **НУЖЕН, оба слоя** | Бекенд: в `paymentSyncFromUpdate`/`updatedRentalOutcome` — валидировать **слитую пару**: эффективный день = `cmd.PaymentDay`, иначе текущий день платежа (один `RentPaymentState` в транзакции, только когда `cmd.PlannedEndDate != nil && cmd.PaymentDay == nil`); слитое окончание = `cmd.PlannedEndDate.Value`, иначе `rental.PlannedEndDate`. `plannedEnd < FirstPaymentDate(start, day) → ErrInvalidInput`. Фронт: minDate от effectiveDay |
| **Продление** (rental-extend-screen) | **НУЖЕН, но достаётся бесплатно** | Это тот же PATCH `plannedEndDate` → бекенд-guard из строки выше покрывает. Отдельная дыра реальна только у открытой аренды (минимум «сегодня»/`start+1` может быть раньше первого вхождения: старт 10-е, день 25-е, сегодня 12-е, продлили до 20-е → 0 платежей). Фронт: добавить первое вхождение нижней границей в `rentalExtendMinDate` |
| **Завершение** (complete ставит end_date) | **НЕ НУЖЕН** | Завершение — **факт, не план**: `validateCompletedDate` уже гарантирует `start ≤ дата ≤ today`; `Stop` ставит `EndDate = completedDate ≥ start` — CHECK `payments_since_end_date_check` держится. «0 операций» легально: пожил меньше одного периода, платёж вне платформы — запись фактов, не движение денег (ADR 0036). Блокировать факт нельзя; edge case «завершение раньше первого вхождения» фиксируется тестом как легальное поведение |

Открытый вопрос правки: смена дня оплаты и окончания одним PATCH. Слитая пара валидируется
целиком — поменяли день так, что стоящее окончание стало невалидным → 400 с человеческим
сообщением; фронт правки условий (раздел 4) не даёт выбрать такое значение, а при смене дня
подчищает окончание автосбросом, как в визарде.

---

## 4. Фронт

### Источник minDate: фронтовое зеркало уже существует

`apps/frontend/entities/payment/lib/occurrences.ts` — полный клиентский порт перечисления
вхождений (порт прототипа, спека #453) со своим тестом `occurrences.test.ts`, и **нужная функция
уже там** (строка 170):

```ts
/** Самое раннее вхождение правила — превью первого вхождения в визарде …
 * null — вхождений нет (например, окончание раньше даты заведения). */
export function firstOccurrence(schedule: PaymentSchedule): IsoDate | null
```

Она уже используется для фразы экрана успеха (`features/payments/lib/success-copy.ts`). Для
валидатора minDate вызывать с выключенным окончанием: `firstOccurrence({ ...schedule, endDate:
undefined })` (та же ловушка «EndDate режет всё» — задокументирована в её доке).

**Вердикт: локальное зеркало + серверный 400-бэкстоп. Серверное поле `firstOccurrence` в
`PaymentResponse` не вводим**: (а) главный экран — СОЗДАНИЕ, правила ещё нет, полю неоткуда
взяться; (б) в правке `since` иммутабелен и уже приходит в `Payment` — расчёт локален и дёшев;
(в) риск расхождения с Go — уже принятый, покрытый тестами риск этого порта (день ошибки в minDate
не фатален: пикер поднимет черновик до minDate, а дивергенция ловится 400 с понятным сообщением).

### Единый подход

minDate каждого пикера endDate = «первое вхождение», автосброс невалидного значения по
прецеденту аренды (`draftAfterStartChange`, `features/rentals/lib/wizard-model.ts:146`).

**Платёж, создание** (`widgets/payments/ui/payment-create-wizard/payment-settings-step.tsx`):
периодичность выбирается на шаге 3, окончание — на шаге 4, значит к моменту выбора `draft.recurrence`
известен. `since` при создании = «сегодня» владельца (сервер ставит сам) →

```ts
const minDate = firstOccurrence({
  recurrence: draft.recurrence, since: today, endDate: undefined, pauses: [],
});
```

`today` здесь — существующий паттерн визардов `dateToIsoLocal(new Date())`
(`payment-create-wizard-flow.tsx:110`). Часовой пояс: сервер ставит `since` по календарю
владельца (ADR 0048), фронтовый `today` — локальный по устройству; расхождение возможно только
в окне полуночи при TZ устройства ≠ TZ владельца. Ловится бэкстопом (400 «Дата окончания не
может быть раньше первого платежа»), та же граница уже принята во всех остальных валидациях
визардов (например `rentalStartDateError`). Отдельный механизм «узнать сегодняшнее владельца до
создания» не строим — END-точки с серверным today (раздел 3/4 ниже) покрывают правки, а
create-визард остаётся в общем паттерне.

**Платёж, правка** (`widgets/payments/ui/payment-edit-screen.tsx`): `payment.since` — серверная
истина; minDate = `firstOccurrence({ ...payment, recurrence: form.recurrence, endDate: undefined })`
(при открытой странице периодичности — от `periodicityDraft`, чтобы после «Выбрать» minDate и
валидность обновились вместе). Молчаливый сброс по решению владельца: в `applyPeriodicity`
(строка 440) после `update('recurrence', …)` — если `form.endDate !== undefined` и
`firstOccurrence(...слитое расписание...) > form.endDate`, сбросить endDate в `undefined`
(в команде PATCH это tri-state `endDate: null` — «открыть срок», `features/payments/lib/
update-model.ts:129`). Существующая валидация `editFormReady` не расширяется — невалидное
значение в форму не попадает.

**Аренда, создание** (`widgets/rentals/ui/conditions-step.tsx`): платежа ещё нет, минимум считается
локально из старта и дня оплаты. День оплаты известен — шаг 1 («сумма + день оплаты») раньше
шага 2 («условия»), уточнение владельца. Новая чистая функция рядом с
`rentalPlannedEndDateError` в `features/rentals/lib/wizard-model.ts`:

```ts
/** Первая дата дня оплаты на или после старта: 1..30 — свой день (прижатый
 * к длине месяца), 31 и «последний» — фактический последний день месяца.
 * Зеркало backend PaymentDay.FirstPaymentDate. */
export function firstPaymentDate(startDate: IsoDate, paymentDay: RentalPaymentDay): IsoDate
```

minDate пикера окончания = `max(addDays(startDate, 1), firstPaymentDate(startDate, paymentDay))`
вместо сегодняшнего `minDate={addDays(startDate, 1)}` (conditions-step.tsx:153). Автосброс:
`draftAfterStartChange` расширить (или добавить `draftAfterPaymentDayChange`) — окончание,
переставшее быть ≥ первого вхождения (при смене старта **или** дня оплаты), очищается.

**Аренда, правка условий** (`widgets/rentals/ui/rental-terms-edit-screen.tsx`): today здесь уже
серверный (`rental.today: IsoDate` — объект аренды несёт «сегодня» владельца, ADR 0048; тип —
`entities/rental/model/types.ts:76`); effectiveDay = форма ?? `rental.rentPayment.paymentDay`.
minDate = `max(addDays(startDate, 1), firstPaymentDate(startDate, effectiveDay))`; при смене дня
оплаты — автосброс окончания, как выше.

**Аренда, продление** (`widgets/rentals/ui/rental-extend-screen.tsx`): в
`features/rentals/lib/extend-model.ts` `rentalExtendMinDate` добавить третью нижнюю границу —
`firstPaymentDate(startDate, paymentDay)` (аргумент в функцию). Закрывает дыру открытой/незапущенной
аренды (сегодня или `start+1` раньше первого вхождения). Бекенд-guard тот же PATCH — см. раздел 3.

**Завершение** (`rental-complete-screen.tsx`): ничего не меняем (вердикт раздела 3).

---

## 5. Тест-кейсы для TDD

### Бекенд payments — `internal/payments/application/payment_rule_test.go` + доменные тесты occurrences

| # | Кейс | Ожидание |
|---|---|---|
| 1 | weekly «пт», since = ср 2026-10-07, endDate = чт 2026-10-08 (create) | `ErrEndDateBeforeFirstOccurrence` → 400 |
| 2 | weekly «пт», since = ср, endDate = пт 2026-10-09 (== первое вхождение) | ок, ровно одно вхождение |
| 3 | daily, since = X, endDate = X | ок (первое вхождение = since) |
| 4 | daily, since = X, endDate = X−1 | ошибка (существующий `EndDate < Since`) |
| 5 | monthly 31-е, since = 2026-01-31, endDate = 2026-02-28 | ок (первое = 31.01, клампинг февраля не ломает) |
| 6 | yearly 29.02, since = 2026-03-01, endDate = 2026-12-31 | ошибка (первое = 29.02.2028) |
| 7 | yearly 29.02, since = 2026-03-01, endDate = 2028-02-29 | ок |
| 8 | update: weekly «пн», since = пн, endDate = пн (валидно); PATCH recurrence → «пт» | 400 (слитая пара невалидна) |
| 9 | тот же PATCH + `endDate: null` | ок, окончание снято (молчаливый сброс фронта) |
| 10 | update: PATCH title-only на правиле с валидным окном | ок (триггер не сработает) |
| 11 | update: PATCH recurrence-only на легаси-правиле с дыркой | 400 (триггер = расписание) |
| 12 | домен: `FirstOccurrence` на правиле, где EndDate режет всё | не зависит от EndDate, возвращает дату |
| 13 | домен: пауза, накрывающая якорь | `FirstOccurrence` перескакивает паузу |
| 14 | handler: 400 problem+json с detail «Дата окончания не может быть раньше первого платежа» | `writePaymentsError` отдаёт специфичный детейл раньше общего «Некорректные данные платежа» |

### Бекенд rentals — тесты `rental_service.go` / домена PaymentDay

| # | Кейс | Ожидание |
|---|---|---|
| 15 | create: start = 2026-10-10, day = 25, plannedEnd = 2026-10-20 | `ErrInvalidInput` → 400 |
| 16 | create: plannedEnd = 2026-10-25 (== первое вхождение) | ок |
| 17 | create: day = last, start = 2026-10-10, plannedEnd = 2026-10-31 | ок; plannedEnd = 2026-10-30 → ошибка |
| 18 | create: day = 31, start = 2026-01-10, plannedEnd = 2026-01-31 | ок (первое = 31.01, клампинг) |
| 19 | edit: смена day 25 → 5 при start = 2026-10-10, plannedEnd = 2026-11-20 | ок (слитое первое = 05.11 ≤ 20.11) |
| 20 | edit: смена day 25 → 5 при plannedEnd = 2026-10-20 | 400 (первое 05.11 > конца) |
| 21 | edit: plannedEnd раньше первого вхождения без смены day | 400 |
| 22 | extend (тот же PATCH): открытая незапущенная аренда, plannedEnd < первого вхождения | 400 |
| 23 | complete: completedDate раньше первого вхождения | **ок** — легально, 0 операций (вердикт «не блокировать»), EndDate = completedDate, CHECK держится |

### Фронт — vitest (`*.test.ts` рядом с моделями) + e2e

| # | Кейс | Ожидание |
|---|---|---|
| 24 | `firstPaymentDate`: day = 25, start = 2026-10-10 | `2026-10-25` |
| 25 | day = 5, start = 2026-10-10 | `2026-11-05` |
| 26 | day = 'last', start = 2026-10-10 | `2026-10-31` |
| 27 | day = 31, start = 2026-11-10 | `2026-11-30` (клампинг); start = 2026-02-10 → `2026-02-28` |
| 28 | day = 10, start = 2026-10-10 | `2026-10-10` (== старт) |
| 29 | визард платежа, шаг 4: weekly «пт», today = ср | minDate пикера = ближайшая пт |
| 30 | правка платежа: смена периодичности «пн» → «пт» при endDate = пн | endDate сброшен молча; в команде `endDate: null` |
| 31 | визард аренды: start = 10-е, day = 25, конец 20-е | `rentalPlannedEndDateError`/сброс — команда не собирается |
| 32 | `rentalExtendMinDate`: открытая аренда start = 10.10, day = 25, today = 12.10 | min = `2026-10-25`, не today |
| 33 | e2e: создание платежа с endDate раньше первого вхождения | дата в пикере недоступна, шаг непрошёл |
| 34 | e2e: создание аренды (старт 10-е, день 25-е, окончание 20-е) | пикер не даёт 20-е, минимальная дата — 25-е |

---

## Предложение архитектуры и разбивка на 3 TDD-тикета

Модель: «окно графика» — инвариант правила: `endDate` (если стоит) ≥ первое вхождение расписания
(без учёта endDate); валидируется на бекенде (авторитет, 400 с человеческим сообщением),
профилактируется на фронте (minDate пикеров + молчаливый автосброс по решению владельца).
Аренда валидирует ту же пару через первое вхождение дня оплаты; завершение — факт, без guard.

### Тикет 1 — бекенд payments

- `internal/payments/domain/occurrences.go`: `FirstOccurrence(p Payment) (time.Time, bool)`
  (EndDate игнорирует, паузы вырезает).
- `internal/payments/application/ports.go`: сентинел `ErrEndDateBeforeFirstOccurrence`.
- `internal/payments/application/payment_service.go`: `validateRuleWindow`; вызов из
  `CreatePayment` (всегда) и из `UpdatePayment` (триггер `cmd.Recurrence != nil ||
  cmd.EndDate != nil`).
- `internal/payments/adapters/http/payment_handlers.go`: запись в `staticPaymentProblems`
  (400, «Дата окончания не может быть раньше первого платежа»).
- Тесты: кейсы 1–14.

### Тикет 2 — бекенд rentals

- `internal/rentals/domain/rental.go`: `(PaymentDay) FirstPaymentDate(start)`.
- `internal/rentals/application/rental_service.go`: `validateCreate` — guard создания;
  `paymentSyncFromUpdate`/`updatedRentalOutcome` — guard слитой пары правки/продления
  (текущий день платежа через gateway `RentPaymentState`, когда день не в команде).
- Завершение — тест-фиксация легальности «0 операций» (кейс 23).
- Тесты: кейсы 15–23.

### Тикет 3 — фронт-экраны

- `entities/payment/lib/occurrences.ts` — переиспользование `firstOccurrence` (без новых сим,
  возможно док-комментарий про `endDate: undefined`).
- `features/rentals/lib/wizard-model.ts`: `firstPaymentDate`; расширение `draftAfterStartChange`
  / новый `draftAfterPaymentDayChange`; `features/rentals/lib/extend-model.ts` — третья граница в
  `rentalExtendMinDate`.
- Экраны: `payment-settings-step.tsx` (minDate), `payment-edit-screen.tsx` (minDate + молчаливый
  сброс в `applyPeriodicity`), `conditions-step.tsx` (minDate = max(start+1, firstPaymentDate)),
  `rental-terms-edit-screen.tsx` (minDate от effectiveDay, сброс), `rental-extend-screen.tsx`
  (minDate через `rentalExtendMinDate`).
- Тесты: кейсы 24–34.

Источники внешних практик: RFC 5545 §3.3.10 (datatracker.ietf.org/doc/html/rfc5545);
Stripe Subscriptions API — Create a subscription (docs.stripe.com/api/subscriptions/create);
Google Calendar API — Recurring events (developers.google.com/workspace/calendar/api/guides/
recurringevents) и тред «Google Calendar API sometimes silently fails to update recurrence rules»
(discuss.google.dev).
