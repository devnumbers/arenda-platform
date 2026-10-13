# Ресерч #1384: завершение аренды без залога (`depositKopecks === null`)

Вопрос владельца 13.10: на шаге 2 «Возвращение залога» мастера завершения убрать поле суммы и переименовать шаг в «Добавьте комментарий» — остаётся только комментарий. Что при этом отправлять в `POST …/complete`, что покажут итоги и прошлые аренды. Часть карты #1379; код не менялся.

## 1. Контракт: бэкенд принимает complete без `depositReturn`

Все три источника контракта согласны — возврат залога опционален:

- **OpenAPI** (`apps/frontend/shared/api/generated.ts:4073-4086`): `RentalCompleteRequest` = `{ completedDate: string; depositReturn?: { amountKopecks: number; comment?: string } }`. `depositReturn` не обязателен; внутри него обязателен только `amountKopecks`, «0 валиден — „не вернул“», комментарий — «Только при сумме».
- **HTTP handler** (`apps/backend/internal/rentals/adapters/http/rental_handlers.go:488-497`, `completeRentalCommand`): `body.DepositReturn == nil` → `cmd.DepositReturn` остаётся `nil`. Никакого требования нет.
- **Application service** (`apps/backend/internal/rentals/application/rental_service.go:481, 895-906`): `validateDepositReturn(nil)` возвращает `nil`; валидируется только дата (`validateCompletedDate`, окно [начало, сегодня]) и, при наличии записи, сумма 0…10⁹ копеек и длина комментария.
- **DB CHECK** (миграция `apps/backend/db/migrations/000120_rentals_context.up.sql:34-38`): запрет только на возврат у незавершённой и на комментарий без суммы; пара `NULL/NULL` валидна.
- **ADR 0053** (`docs/adr/0053-rentals-context-schema-lifecycle-contracts.md`): §3 завершение №8 — «Возврат залога — опциональные запись-сумма и комментарий»; §4 — тело `depositReturn?: {amountKopecks 0…10⁹, comment?}`; §1 решение №6 — «сумма 0 валидна („не вернул“), **связь с размером залога не проверяется**».

### Что попадает в БД/аудит/историю по вариантам

| Вариант тела | `deposit_return_kopecks` | `deposit_return_comment` | Аудит | История |
|---|---|---|---|---|
| без `depositReturn` | `NULL` | `NULL` | `rental.completed` без пометки поля | `rental.completed` + `payment.deleted` — как всегда |
| `{amountKopecks: 0}` | `0` | `NULL` | `rental.completed`, `fields: ["deposit_return"]` | тот же |
| `{amountKopecks: 0, comment}` | `0` | текст | тот же | тот же |

- Запись идёт через `RentalStore.Complete` (`.../adapters/postgres/rental_store.go:141-165`): `nil`-запись → обе колонки `NULL`.
- Аудит-пометка `fields: ["deposit_return"]` ставится только при наличии записи (`rental_service.go:526-528`).
- **История операций от залога не зависит вообще**: ряд `RentalCompleted` (`apps/backend/internal/history/domain/rows.go:301-319`) — это «Аренда завершена: {арендатор}» + контекст с именем; суммы туда не пишутся. `payment.deleted` (`keep_overdue: true`) пишется в обоих вариантах (`rental_service.go:504-518`).
- **GET после complete** вернёт `depositReturnKopecks: null` / `0` и `depositReturnComment: null` / текст соответственно (`rental_handlers.go:588-589`) — т.е. варианты «без записи» и «с нулём» различимы на чтении.

Ключевое следствие: **«не отправить `depositReturn`» и «отправить 0» — разные записи**. `NULL` = «записи нет» (канон `validateOptionalAmount`: «nil is „нет записи“», `rental_service.go:908-909`), `0` = «не вернул». Для аренды, у которой залога не было, «не вернул» — ложная семантика: возвращать было нечего.

## 2. Комментарий без суммы контрактом не выражаем

- В команде комментарий живёт **только внутри** `depositReturn` (`generated.ts:4077-4085`) — поля «комментарий завершения» вне возврата в схеме нет.
- Дюральный CHECK `rentals_deposit_return_comment_needs_amount_check` (`000120:37-38`) и ADR 0053 §1 («комментарий — только при сумме») запрещают комментарий без суммы.
- Отдельный путь — общий `comment` аренды через PATCH — для завершённой закрыт: `UpdateRental` на завершённой отвечает `ErrRentalCompleted` (409, `rental_service.go:391-392`). Технически можно PATCH-нуть комментарий аренды **до** POST complete (аренда ещё незавершённая, PATCH три-стейт), но это другой термин: комментарий условий, который покажется в «Условиях аренды» (`rentalCommentText`), а не в «Итогах», и это второй сетевой запрос. Семантически это не то, что просит владелец.

**Итого по форме команды** (ответ (b) тикета): контракт позволяет реализовать слово владельца без изменений бэкенда, единственным способом:

- шаг 2 при `depositKopecks === null` — только комментарий; команда:
  - комментарий пустой (после trim) → `{ completedDate }` — **без** `depositReturn` (записи нет, канон `NULL`);
  - комментарий непустой → `{ completedDate, depositReturn: { amountKopecks: 0, comment } }` — единственная легальная форма сохранить текст (0 = «залог не возвращался», что при отсутствии залога фактически «нет возврата»).
- Никакой эскалации на бэкенд не требуется, **если** владелец согласен, что комментарий завершения хранится как комментарий возврата залога с нулевой суммой. Если он хочет «просто комментарий завершения», не привязанный к залогу, — тогда да, эскалация на карту: нужна ревизия контракта (новое опциональное поле). Это решение владельца, отметить на приёмке.
- Альтернатива «не отправлять ничего, комментарий выбрасывать» противоречит слову владельца (шаг-то остаётся) — не рекомендуется.

## 3. Что показывать (ответ (c))

Текущее состояние фронтенда:

- `RentalCompleteCommand` (`apps/frontend/entities/rental/model/types.ts:142-148`) — `depositReturn?` уже опционален в TS-типе; док-комментарий: «комментарий только при сумме».
- Сборка команды (`apps/frontend/features/rentals/lib/complete-model.ts:61-78`): пустая/нечитаемая сумма → команды нет (шаг не готов); комментарий попадает в тело только вместе с суммой.
- Шаг 2 (`apps/frontend/widgets/rentals/ui/rental-complete-flow.tsx:263-315`): сумма обязательна (кнопка шага скрыта до готовности — канон 2026-09-05), чипы с суммой залога и «0 ₽».
- Шаг 3 + «Итоги аренды» (`rental-summary-content.tsx:78-89`): секция «Возвращение залога» всегда; в мастер приходит `depositAmount ?? 0` (`rental-complete-flow.tsx:388`), на экране итогов — `rental.depositReturnKopecks ?? 0` (`rental-summary-screen.tsx:129-130`), т.е. сегодня «0 ₽» рисуется и при отсутствии записи.
- Прошлые аренды: залог возврата **не показывается нигде**, кроме «Итогов». Карточки и детализация не содержат ни залога, ни возврата (`apps/frontend/features/rentals/lib/past-model.ts` — строки карточки: плата/начало/окончание; `rental-completed-screen.tsx` — `rentalTeaserRows`: плата/день/начало). Архив «Условий аренды» показывает залог (`rentalTermsRows` → `optionalMoney`: null → «0 ₽», макет 1550:94419), но не возврат.

Канон для пустых денег в репозитории — не один:

1. **«0 ₽»** для nullable-денег в строках условий: `optionalMoney` (`rental-view.ts:120-124`, макет 1550:94419: «залог и комиссия без значения показываются нулём, „Не указано“ — только строки»).
2. **Секция без контента не выводится вовсе**: секция «Арендатор» без арендатора (`rental-view.ts:168-173`), карточка прогресса «прячется целиком: пустого контейнера макеты не рисуют» (F1, решение владельца 23.09, `rental-view.ts:84-101`).

Рекомендация:

- **В мастере (шаг 2 → шаг 3)** при `depositKopecks === null`: убрать `MoneyField` и чипы, шаг — «Добавьте комментарий», готовность определяется непустотой суммы не должна — шаг всегда готов (комментарий опционален); на шаге 3 секцию залога рисовать только с комментарием (заголовок «Комментарий», без строки «Сумма на возврат»), при пустом комментарии секцию не выводить (канон 2).
- **На «Итогах аренды» завершённой** (`rental-summary-screen.tsx`): `depositReturnKopecks === null && depositReturnComment === null` (CHECK делает «комментарий без суммы» невозможным) → секцию «Возвращение залога» не выводить — записи нет; `depositReturnKopecks === 0` → «0 ₽» (канон 1: это честная запись «не вернул»); `> 0` → сумма. Формулировку «0 ₽» для записи-нуля оставить как есть.
- В «Условиях аренды» (в т.ч. архиве) ничего менять не нужно: залог уже показывает «0 ₽» по канону, возврата там нет.

## 4. Затронутые тесты (ответ (d))

Бэкенд — **не меняется**, все формы уже покрыты:

- `apps/backend/internal/rentals/application/rental_service_test.go:1147-1174` (`TestCompleteRental_Validation`): 0 валиден; завершение без `DepositReturn` уже тестируется (`:1080, :1232, :1247` — `CompleteRentalCommand{CompletedDate: today}`).
- `apps/backend/internal/rentals/adapters/http/rental_handlers_test.go:483-509` — декод тела с `depositReturn`; тело без него покрывается дефолтной веткой (`body.DepositReturn != nil` не сработает).
- `apps/backend/internal/rentals/application/integration_test.go:413-447` (`TestRentalsIntegration_CompleteLifecycle`) — полный жизненный цикл с записью возврата.

Фронтенд unit:

- `apps/frontend/features/rentals/lib/complete-model.test.ts:10-53` — переписывается ветка `buildRentalCompleteCommand`: новые кейсы «без залога: пустой комментарий → команда без `depositReturn`» и «без залога: комментарий → `{amountKopecks: 0, comment}`»; существующий «с нечитаемой суммой команды нет» остаётся для ветки с залогом; черновик (`RentalCompleteDraft`) потребует признака «залога нет» (например, вычислять из `rental.depositKopecks === null` на уровне вызова или добавить поле черновика) — решение тикета-реализации.

Widget-тестов на `rental-complete-flow.tsx` / `rental-summary-content.tsx` / `rental-summary-screen.tsx` сейчас нет (vitest их не покрывает) — либо не появляется, либо добавляются в тикете-реализации.

E2E: шаг возврата залога мастером завершения не покрыт (`grep` по `apps/frontend/e2e/` на «залог/Возвращение/deposit» — пусто; `rentals-wizard.spec.ts` — только создание, `rental-page-actions.spec.ts` — только видимость кнопки «Завершить аренду»). Регрессии e2e не ожидаются; сквозной кейс «завершение аренды без залога» — кандидат в новый e2e в тикете-реализации.

## 5. Источники

- `apps/frontend/entities/rental/model/types.ts:136-148` — `RentalCompleteCommand`
- `apps/frontend/shared/api/generated.ts:2269-2286, 4073-4086, 8688-8730` — OpenAPI complete
- `apps/backend/internal/rentals/adapters/http/rental_handlers.go:260-280, 488-497, 584-589`
- `apps/backend/internal/rentals/application/rental_service.go:451-539, 895-906, 908-918`
- `apps/backend/internal/rentals/adapters/postgres/rental_store.go:141-165`
- `apps/backend/db/migrations/000120_rentals_context.up.sql:13-39`
- `apps/backend/internal/history/domain/rows.go:301-319`
- `docs/adr/0053-rentals-context-schema-lifecycle-contracts.md` §1 (№6), §3 (№8), §4
- `apps/backend/internal/rentals/GLOSSARY.md` — «Залог», «Возврат залога»
- Фронтенд: `complete-model.ts` / `complete-model.test.ts`; `rental-complete-flow.tsx:263-315, 379-391`; `rental-summary-content.tsx:78-89`; `rental-summary-screen.tsx:121-131`; `rental-view.ts:120-124`; `past-model.ts`; `rental-terms-screen.tsx`
