# Реестр: дата вне текущего года без года (ресерч #1382, часть #1379)

Вопрос тикета: где во `apps/frontend` дата вне текущего года рендерится без года и что с каждым местом делать по правилу владельца 13.10: **текущий год — год не пишется, любой другой — пишется**.

Канон: `formatDayMonthWithYear(iso, today)` — `apps/frontend/shared/lib/date-format.ts:70-76` (резолюция #452, ADR 0048 `docs/adr/0048-payments-tick-drivers-owner-timezone.md`: «сегодня» интерфейса считает сервер по TZ собственника; прецедент клиентского «сегодня» — ADR 0061 §7, проекции платежей #453).

Метод: полный `rg 'formatDayMonth\('` по `*.ts`/`*.tsx` вне тестов + поиск кастомных форматов (`Intl.DateTimeFormat`, `toLocaleDateString`, ручные `slice`-шаблоны, `MONTH_SHORT`/`MONTH_PREPOSITIONAL`, `formatDayMonthDotted`, `getFullYear()` в UI) по всему `apps/frontend`. Все утверждения о «чьём сегодня» проверены чтением скоупа каждого файла (строки приведены).

---

## A. Чинить (12 мест в 10 файлах)

| # | Место | Экран | Чьё «сегодня» в скоупе | Правка |
|---|-------|-------|------------------------|--------|
| 1 | `apps/frontend/widgets/tasks/ui/task-create-screen.tsx:228` | «Создать задачу», поле «Дата» (лейбл пикера дедлайна) | Серверное: `today` из срезов `useActiveTasks`/`usePropertylessTasks` (ADR 0048), строки 90-97; `IsoDate \| undefined` — кнопка ждёт today (комментарий 224-225) | `formatDayMonthWithYear(draft.dueDate, today)` с guard на `undefined` (лейбл может отрисоваться с предзаполненной датой до прихода today) |
| 2 | `apps/frontend/widgets/tasks/ui/task-edit-screen.tsx:284` | «Редактировать задачу», поле «Дата» | Серверное, prop `today: IsoDate \| undefined` (строки 229, 285) | Та же, с guard (дедлайн правила бывает и прошлого года) |
| 3 | `apps/frontend/widgets/payments/ui/operation-detail-screen.tsx:178` | «Детали операции», `TopNavTitle` (дата факта или плановая; история уходит в прошлые годы) | Клиентский уже в скоупе: `dateToIsoLocal(new Date())` (строка 306), у `OperationView` prop `today` (334, 347) | Считать today один раз в шапке тела и передать в `formatDayMonthWithYear` |
| 4 | `apps/frontend/widgets/payments/ui/payment-detail-screen.tsx:545` | Страница платежа, секция «Ближайшая операция», subtitle | Prop `today: IsoDate` уже приходит в `NextPaymentSection` (491-497; клиентский, экран считает на строке 212) | `formatDayMonthWithYear(..., today)` |
| 5 | `apps/frontend/widgets/payments/ui/payment-detail-screen.tsx:680` | Страница платежа, секция «История платежа», превью-строка (дата факта — прошлые годы) | Тот же `today` экрана (строка 212); в `HistoryPreviewRow` прокинуть пропом | `formatDayMonthWithYear(operation.paidDate ?? operation.date, today)` |
| 6 | `apps/frontend/widgets/payments/ui/payments-sections.tsx:426` | Строки правил секций «Платежи»/«Автоплатежи» объекта (paymentRowSubtitle, callers: `payments-of-property-screen.tsx:199,219`) | Prop `today: IsoDate` (407-413), клиентский у callers (73) | `formatDayMonthWithYear(subtitle.iso, today)` |
| 7 | `apps/frontend/widgets/payments/lib/payments-global-model.ts:76` | «Ближайший» на карточках глобального хаба, избранного и глобального поиска (`payments-global-screen.tsx:390`, `payments-favorites-screen.tsx:324,698`, `payments-global-search-screen.tsx:204`) | **Серверное прямо в контракте**: `GlobalPayment.today` (`entities/payment/model/types.ts:221`, ADR 0048) — самое каноничное место | `formatDayMonthWithYear(item.nearestDate, item.today)`; тест-фикстура уже обязана нести today (поле обязательное) |
| 8 | `apps/frontend/widgets/payments/ui/projected-operation-screen.tsx:60` | «Просмотр проекции вхождения», `TopNavTitle` (проекция по построению будущая — часто следующий год) | Клиентский, строка 45 | `formatDayMonthWithYear(date, today)` |
| 9 | `apps/frontend/widgets/profile/lib/tariff-overview.ts:22-24` (`dayMonth`) | Экран «Тариф»: «Спишем …» (paid, 65), «Оплатите тариф до …» (grace, 52), «Действует до» (stopped, 41) | Серверного today в контракте подписки нет (`entities/billing`), в скоупе только `useSubscription` (`tariff-screen.tsx:32-33`) | Клиентский `dateToIsoLocal(new Date())` параметром в `dayMonth`/`tariffHero` (прецедент клиентского today: ADR 0061 §7). Высокозначимо: годовой тариф — `validUntil` почти всегда в следующем году. Опц. фоллоу-ап на бэкенд: отдавать today в `GET /subscription` |
| 10 | `apps/frontend/widgets/rentals/ui/rental-detail-body.tsx:398` | «Детализация аренды», строка «Арендная плата» (дата следующего вхождения) | **Серверное `rental.today`** в скоупе (`entities/rental/model/types.ts:85`; уже используется строкой 91) | `formatDayMonthWithYear(rental.rentPayment.nextPayment.date, rental.today)` |
| 11 | `apps/frontend/features/payments/lib/operations-history.ts:137,140` (`operationsGroupLabel`) | Ленты «Операций объекта» (#474) и подэкранов: «Сегодня, 10 ноября» / «Вчера, 9 ноября» | Клиентский, приходит параметром `today` (82-86) | Заменить `formatDayMonth` → `formatDayMonthWithYear(date, today)` в обеих строках — автоматически закрывает край «Вчера, 31 декабря» (см. §C) |
| 12 | `apps/frontend/features/payments/lib/success-copy.ts:54` | Экран успеха визарда платежа: «Первый платеж 13 августа, далее …» / «Платеж пополнится сам …» | Серверного нет (платежа ещё не существует; firstOccurrence — клиентская проекция, докстрок 9-19) | Клиентский today (`dateToIsoLocal`) параметром в `successScreenCopy` (функция чистая); создав в декабре платёж на январь, пользователь видит дату следующего года |

### Найдено вне списка тикета (dobор через кастомный поиск)

| # | Место | Экран | Чьё «сегодня» | Вердикт |
|---|-------|-------|---------------|---------|
| 13 | `apps/frontend/features/properties/lib/property-badges.ts:70` | Бейдж «Аренда с 01.09» карточки объекта (upcoming, #586) | Серверное из списочного ответа: `propertyBadges(property, today)` (79; докстрок 10-13 — today ADR 0048), в `rentalBadge` на месте | Чинить: upcoming-аренда может начинаться в январе при декабрьском «сегодня». Вне текущего года — `formatDottedDate(start_date)` («01.01.2027»), приём уже канонный — `formatRangeBound` (`shared/lib/date-format.ts:108-110`); в текущем году форма «DD.MM» из макета #586 не меняется |

## B. Осознанно без года / не чинить

| Место | Экран | Основание |
|-------|-------|-----------|
| `apps/frontend/entities/payment/lib/recurrence-label.ts:47` (`formatYearlyDayMonth`) | «Каждое 13 мая» — метка годового правила периодичности (визард, тексты успеха) | Это не дата события, а повторяющееся правило («каждый год 13 мая») — год не имеет смысла; канон #452. Не чинить |
| `apps/frontend/widgets/property-detail/lib/operations-section.ts:11` | «Операции в сентябре» — заголовок секции детали объекта (#589) | Всегда текущий месяц (`operationsMonthOf(today)`, `features/payments/lib/operations-month.ts:17-19`), навигации по месяцам нет — год тривиально текущий. Не чинить |
| `shared/lib/format-date.ts` (`formatDate`) | «Изменения вступят в силу …» (`widgets/profile/ui/TariffChangeSuccess.tsx:90`) | Легаси полноширинный `ru-RU` формат — год есть всегда. Не чинить |
| `features/rentals/lib/success-copy.ts:52-54` | «Аренда продлена до 01.01.2027» | `formatDottedDate` — год всегда. Не чинить |
| Канонные потребители (проверены): `formatRangeBound`, `formatIsoRangeChipLabel`, `feed-groups` истории (`features/history/lib/feed-groups.ts`) и уведомлений (`features/notifications/lib/feed-groups.ts:43`), `entities/task/lib/date-format.ts` («Выполнена …», год уже «чужой»), `tasks-list`/`task-row`, `payment-history-timeline.ts:121-128`, `payment-history-model.ts:151`, `past-model.ts` (серверный `rental.today`), `formatSessionLastSeen` (устройства — год вне текущего уже пишет), `payment-change-chips` (`formatDayMonthYear` — год всегда), поисковики операций, график/правка платежа, шаги визардов аренд и платежей — все на `formatDayMonthWithYear`/`formatDayMonthYear` с переданным today | — | Правило соблюдено, правок нет |

### Серые зоны — макет без года, решить с владельцем (рекомендация: чинить)

| Место | Экран | Детали |
|-------|-------|--------|
| `apps/frontend/features/rentals/lib/rental-view.ts:132` (`rentalTeaserRows`) | Карточка «Условия аренды» (3 строки) на «Детализации аренды» (`rental-detail-body.tsx:260`) и на «Завершённой аренде» (`rental-completed-screen.tsx:215`) | Комментарий «без года (макет 1232:61524)» пришёл с #531 (коммит ae1df324f, 2026-09-05) — **до** правила владельца 13.10, т.е. это макетная, а не осознанная против правила, оговорка. Тот же «Начало аренды» на полном экране «Условия аренды» рисуется с годом (`rentalTermsRows`, `rental-view.ts:152`, макет 1302:53783) — противоречие внутри одного слайса. У завершённой аренды начало почти всегда не в текущем году. `rental.today` в обоих экранах есть. Рекомендация: чинить (`formatDayMonthWithYear(rental.startDate, rental.today)`, параметр today в `rentalTeaserRows`), макет 1232:61524 переиграть — подтвердить у владельца |
| `shared/lib/date-format.ts:137-141` (`formatDayMonthTime`) | Страница уведомления (#745, макет 2333:184048) — «15 сентября, 14:40» без года всегда | Решение владельца 17.09.2026 в докстроке — про время на странице (оно остаётся всегда), про год решений нет; макет нарисован без года. Старое уведомление по диплинку покажет дату прошлого года без года. Соседний канон `formatSessionLastSeen` (149-157) вне текущего года год пишет. Рекомендация: чинить по шаблону `formatSessionLastSeen` (параметр `now` с дефолтом), подтвердить у владельца. Низкий приоритет: из ленты старые уведомления почти недостижимы |

## C. Крайний случай «Вчера, 31 декабря»

`operationsGroupLabel` (`features/payments/lib/operations-history.ts:135-143`): 1 января лента операций объекта показывает «Вчера, 31 декабря» — дата прошлого года без года, нарушение правила ровно один день в году.

**Решение (минимальное, без нового API): заменить `formatDayMonth(date)` на `formatDayMonthWithYear(date, today)` в обеих ветках (строки 137, 140).**

- «Сегодня» всегда в текущем году — текст не меняется ни одним днём года.
- «Вчера» через Новый год автоматически становится «Вчера, 31 декабря, 2025»; остальные 364 дня — без изменений.
- Соседняя ветка non-day-лейблов (строка 142) уже `formatDayMonthWithYear(date, today)` — стиль сходится.
- Второй лейбл-механизм файла, `historyGroupLabel` (123-133), не задет: там «Сегодня»/«Вчера» без даты вовсе, а датовые группы всегда с годом (`formatDayMonthYear`, решение #802).
- Plain-«Сегодня»/«Вчера» без даты в истории (`payment-history-timeline.ts:122-128`) и уведомлениях (`features/notifications/lib/feed-groups.ts:41`) даты не несут — края нет.
- Вариант «добавлять год только у Вчера через год» отдельной веткой отвергнут: `formatDayMonthWithYear` уже выражает правило, отдельная логика — дубль канона.

## D. Тесты, пинящие тексты дат (к переучиванию в тикете-реализации)

Unit (vitest):

- `apps/frontend/features/payments/lib/operations-history.test.ts:79-86,122-123,141` — пины «Сегодня, 27 августа»/«Вчера, 26 августа» и т.п.; после правки 11 тексты в текущем году не меняются, но нужно добавить кейс через Новый год («Вчера, 31 декабря, 2025») и right-to-fail старые пины при смене фикстур на чужой год.
- `apps/frontend/features/payments/lib/success-copy.test.ts:19,31,41,49` — пины «Платеж пополнится сам 10 сентября, далее …»/«Первый платеж 31 августа, далее …»; после правки 12 сигнатура получает today — обновить вызовы, добавить кейс первого вхождения в следующем году.
- `apps/frontend/widgets/payments/lib/payments-global-model.test.ts:73-79` — `nearestDateLine` пин «17 августа»; фикстура `GlobalPayment` уже обязана нести `today` — добавить кейс чужого года.
- `apps/frontend/features/rentals/lib/rental-view.test.ts:176-250` — `rentalTeaserRows` пины «10 октября» (181, 213, 236) и `rentalTermsRows` годовые кейсы (241-250); при правке серой зоны teaser'а — обновить и добавить чужой год.
- `apps/frontend/features/properties/lib/property-badges.test.ts:69-75` — пин «Аренда с 05.10»; добавить кейс upcoming в следующем году («Аренда с 05.01.2027»).
- `apps/frontend/shared/lib/date-format.test.ts` — канон уже покрыт (16-17); дополнить при появлении новых хелперов (по реестру новых хелперов не нужно — правки переиспользуют `formatDayMonthWithYear`/`formatDottedDate`).

E2E (Playwright, `apps/frontend/e2e`):

- `e2e/operations-paid-date.spec.ts:136-144` — зеркалит ленты «Сегодня, {formatDayMonth}» и историю — переписать на `formatDayMonthWithYear`.
- `e2e/payment-next-date.spec.ts:8,106,133` — пины `formatDayMonth` на датах строк-ближайших (секции «Платежи» объекта, «Ближайшая операция») — на `formatDayMonthWithYear` (даты сидов текущего года — тексты не изменятся).
- `e2e/payment-detail.spec.ts:9,110-111,197` — превью «Истории платежа» (formatDayMonth сегодня/вчера) и строка ближайшей — на `formatDayMonthWithYear`.
- `e2e/payment-subscreens.spec.ts:118,170` — regex `\d{1,2} <месяц>` — допускает хвост «, 2027» (regex-поиск подстроки), должен пройти; проверить прогоном.
- Без изменений (уже канон/не задеты): `e2e/history-feed.spec.ts:29` (чип через `formatDayMonthWithYear`), `e2e/payments-wizard.spec.ts:597-599`, `e2e/payment-edit-delete.spec.ts:452`, `e2e/rentals-wizard.spec.ts:427`, `e2e/profile-devices.spec.ts:124,128` (сессии всегда свежие), `e2e/history-search.spec.ts`.
- Вне скоупа фронта: `e2e/notifications-feed.spec.ts:21` — «Срок оплаты: 5 октября» composing на бэкенде (`apps/backend/internal/notifications/application/payments_publisher.go`); к правилу владельца тексты пушей/ленты — отдельный тикет по бэкенду.

## Итог

- Чинить: 13 мест (12 из списка тикета + 1 найденное — бейдж «Аренда с»), из них 6 получают серверное today без изменений контрактов (№1, 2, 7, 10, 13 и teaser при подтверждении), остальные — клиентский today с задокументированной TZ-оговоркой ADR 0061 §7.
- Осознанно без года: метка периодичности «Каждое 13 мая», заголовок «Операции в <текущий месяц>», легаси-полные форматы с годом.
- Серые зоны (макет старше правила, рекомендация — чинить и переиграть макет): тизер «Условия аренды» (#531), страница уведомления (#745).
- Край «Вчера, 31 декабря» закрывается автоматически заменой на `formatDayMonthWithYear` в `operationsGroupLabel`.
- Новых канонных функций не требуется: все правки — переход на существующие `formatDayMonthWithYear`/`formatDottedDate`.
