# Research #1163: планирование уведомлений на точное настенное время по поясу — практики и сверка с каноном

Дата: 2026-10-06. Методология: исследование по первоисточникам (официальные доки River, Braze, Quartz, robfig/cron, PostgreSQL, Apple HIG, README system-design-primer) и код репозитория; каждое утверждение привязано к источнику. Тикет: [#1163](https://github.com/devnumbers/arenda-platform/issues/1163), карта: [#1162](https://github.com/devnumbers/arenda-platform/issues/1162). Репозиторий не изменялся (док в бросовой ветке `research/notifications-scheduling`).

---

## TL;DR

1. **Смена `fire_at` с полуночи на 10:00/22:00 соответствует практике.** Локальное время получателя — индустриальный стандарт (Braze: «Quiet hours apply in each user's local time zone»; OneSignal/Batch — достав-ка по поясу устройства); 10:00 попадает в рекомендуемое утреннее окно, полночь — в тишину, которую практика велит не будить.
2. **Машина репозитория — канон**: отложенная джоба на точный момент события (River `ScheduledAt`) + часовой sweeper-подстраховка + at-least-once с дедупом по бизнес-идентичности — это ровно то, что у практика-первоисточников выглядит как «delayed job per event + sweeper + idempotency» (Quartz misfire = отставание планировщика; Braze hold-and-deliver = догон после окна тишины).
3. **Найден разрыв: «машина не меняется» в формулировке #1168 неполна.** Скан-ноги платежей (`ListPaymentDueTargets`/`ListPaymentOverdueTargets`/`ListPaymentReminderTargets`) датированы (`o.date = / < сегодня`), без привязки к мгновению границы. С границей в 10:00/22:00 часовой скан **обгонит** граничную джобу: первый проход после местной полуночи (~01:00) опубликует уведомление на 9–21 часа раньше назначенного времени, и джоба уйдёт вежливым no-op по дедупу. Требуемый компаньон-изменение: instant-гейт на скан-ногах — прецедент в репо есть (скан задач сравнивает мгновение срока с `now`, `notifications.sql:527`).
4. **DST: крайний случай полуночи исчезает.** 10:00 и 22:00 не попадают в окно переходов IANA (00:00–04:00 местного) — оба настенных времени всегда существуют; несуществующая полночь — реальный класс багов (robfig/cron чинил «non-existent midnight», #157).
5. Для #1169 («автоплатёж исполнен» в 10:00) главный тонкий момент — **порядок с тиком автосписания**: если в 10:00 списание ещё не исполнено, джоба уходит no-op, публикацию догонит скан в первый часовой проход после исполнения; семантику «10:00 или первый проход после» должен зафиксировать гриллинг.

---

## 1. Что спрашивали и что проверяли

Вопрос тикета: что говорит индустриальная практика о планировании уведомлений на точное настенное время по таймзоне пользователя, и подтверждает ли она смену `fire_at` с 00:00 на 10:00/22:00 в существующей машине (booking-окно 48ч, unique-джобы River, часовой зональный скан-подстраховка, delivery-time re-check, дедуп-ключи (правило, дата операции)).

Факты канона проверены по коду (состояние `dev` на 06.10.2026):

| Факт | Где |
|---|---|
| Booking-выражения: полночь даты операции (due, `notifications.sql:311`), полночь дня после (overdue, `:332`), полночь «даты − N» (reminder, `:429`) — все через `AT TIME ZONE u.timezone` | `apps/backend/db/queries/notifications.sql` |
| Booking-окно `(now, now+48ч]`, перезапрос каждый час | `internal/notifications/application/tasks_publisher.go:21` (`scheduledHorizon = 48 * time.Hour`), `payments_publisher.go:214` |
| Unique-джобы: `UniqueOpts{ByArgs: true, ByState: [...нетерминальные]}` — максимум одна джоба «в полёте» на (нога, правило, дата) | `internal/notifications/adapters/notificationsjob/args.go:34–36` |
| Скан-ноги платежей: **date-only** предикаты (`o.date = $2::date`, `o.date < $2::date`, `(o.date − offset) = $2::date`) | `notifications.sql:271`, `:296`, `:415` |
| Скан задач: **instant**-предикат — `(due_date + due_time) AT TIME ZONE tz <= now` для timed-задач | `notifications.sql:527` |
| Delivery-time re-check: `GetScheduled*Payment` перечитывают живое состояние в момент пробуждения | `notifications.sql:343–465`, `payments_publisher.go:267–307` |
| Дедуп: `payment_due/payment_overdue/payment_reminder:<rule_id>:<YYYY-MM-DD>` | `payments_publisher.go:313–339`, `internal/notifications/application/scan.go:66–68` |

---

## 2. Практики из первоисточников

### 2.1. Локальное время получателя и гуманные часы

Практика платформ доставки единодушна: расписывать по **поясу получателя**, а не сервера, и не будить в ночные часы.

- **Braze** (официальная документация, quiet hours): «Quiet hours apply in each user's local time zone.» Сообщение, попавшее в окно тишины, не отменяется и не досылается немедленно: «Braze holds the message and delivers it at the next available time after quiet hours end» (пример: окно 22:00–06:00, сообщение на 05:30 уходит в 06:00). Типовое окно тишины в примерах Braze — 22:00–06:00, то есть **22:00 — граница, а не глубина ночи** ([Braze Docs: Quiet hours](https://www.braze.com/docs/user_guide/messaging/messaging_fundamentals/quiet_hours)).
- **OneSignal** / **Batch** / Feedify: «deliver by time zone» — доставка в один и тот же настенный час во всех поясах; консенсус статей практики: утренние слоты (8–10 утра местного) и обед/вечер — лучшие окна вовлечённости; ночь (примерно 22:00–08:00) — не слать ([OneSignal: Scheduling Push Notifications by User Time Zone](https://onesignal.com/blog/deliver-by-timezone-push-notification), [Batch: What is the best time to send push notifications](https://doc.batch.com/guides-and-best-practices/orchestration/what-is-the-best-time-to-send-push-notifications), [Feedify](https://feedify.net/blog/scheduling-push-notifications-by-user-time-zone/)).
- **Apple HIG** (платформенный первоисточник): пуш-уведомление — «timely, high-value information» (укладывается по времени, когда оно ценно); Time Sensitive-уровень, пробивающий Focus/тихие часы, — только для актуального «здесь и сейчас», всё остальное глушится системными Focus-режимами ([Apple HIG: Notifications](https://developer.apple.com/design/human-interface-guidelines/notifications), [Managing notifications](https://developer.apple.com/design/human-interface-guidelines/managing-notifications)). Для приложения это значит: полуночный пуш почти наверняка попадёт в подавленный Focus-слот и не даст звука — точный настенный час имеет смысл только в бодрственном окне.

Вывод: полночь как момент доставки — худший час из возможных (тишина + подавление Focus'ом + паника «что случилось»); 10:00 — внутри рекомендуемого утреннего окна; 22:00 — на границе окна тишины, приемлемо для транзакционного «последнего звонка», но на самом краю.

### 2.2. Точный момент: отложенная джоба на событие против крона

Индустриальный шаблон для «доставить ровно в настенный час X по поясу пользователя» — **не крон, а отложенная задача, поставленная на конкретный инстант**, с планировщиком-подстраховкой для пропусков:

- **River** (очередь репозитория, ADR 0059): «At insertion time, any job can specify a `ScheduledAt` as part of its InsertOpts to run it at a future time»; при наступлении момента «the next loop of the Scheduler will move it to available»; гарантированная задержка пробуждения: «there will always be some delay after the scheduled time (generally less than 5 seconds)» — «not suitable for running jobs only a few seconds in the future» ([River Docs: Scheduled jobs](https://riverqueue.com/docs/scheduled-jobs)). Ровно это делает граничная джоба уведомлений: секунды задержки несущественны на фоне почасового скана.
- **system-design-primer** (Asynchronism): отложенная работа через очередь — «Message queues receive, hold, and deliver messages. If an operation is too slow to perform inline, you can use a message queue»; task-очереди — «They can support scheduling and can be used to run computationally-intensive jobs in the background» (пример Celery) ([system-design-primer README](https://github.com/donnemartin/system-design-primer#asynchronism)).
- **Крон по поясу** (полуночный fan-out, N расписаний по зонам) — то, от чего ADR 0048 осознанно отказался (десяток расписаний вместо одного, догон только следующим часом). Практика соглашается: cron-библиотеки держат пояс отдельной опцией расписания (robfig/cron: «CRON_TZ is now the recommended way to specify the timezone of a single schedule» — [README](https://raw.githubusercontent.com/robfig/cron/master/README.md)), но семантика «точный инстант события» в кроне не выражается — только «каждый час проверяй».
- Сопутствующий сдвиг инстанта у отложенных джоб: очередь измеряет окно бронирования в UTC-инстантах; настенное время через `AT TIME ZONE` считается от даты в wall-clock домене, и только потом конвертируется. Это правильный порядок: PostgreSQL прямо предупреждает, что арифметика `interval '24 hours'` поверх `timestamptz` в DST-зоне отличается от «+1 день» по календарю ([PostgreSQL Docs §9.9.4](https://www.postgresql.org/docs/current/functions-datetime.html)). Канон репозитория (дата ± дни в date-домене, `AT TIME ZONE` в конце) этому соответствует.

### 2.3. Catch-up семантика после простоя

Классическая таксономия «что делать с пропущенным моментом» — **misfire-политики Quartz** (первоисточник термина):

- Определение: «A misfire occurs if a persistent trigger "misses" its firing time because of the scheduler being shutdown, or because there are no available threads in Quartz's thread pool for executing the job» ([Quartz Tutorial 04](https://www.quartz-scheduler.org/documentation/quartz-2.3.0/tutorials/tutorial-lesson-04.html)).
- Политики: `IGNORE_MISFIRE_POLICY` (догнать всё накопленное — шторм отложенных срабатываний), `DO_NOTHING` (пропустить устаревшее, ждать следующего момента), `FIRE_NOW` (сработать один раз сейчас). Дефолт — «smart policy», для CronTrigger интерпретируемый как `FIRE_NOW`: «The "smart policy" instruction is interpreted by CronTrigger as MISFIRE_INSTRUCTION_FIRE_NOW» ([Quartz Tutorial 06](https://www.quartz-scheduler.org/documentation/quartz-2.3.0/tutorials/tutorial-lesson-06.html)).
- Платформенная аналогия удержания: **Braze hold-and-deliver** — сообщение из окна тишины доставляется «at the next available time», то есть догон один раз и без накопления ([Braze Docs](https://www.braze.com/docs/user_guide/messaging/messaging_fundamentals/quiet_hours)).

Семантика репозитория после простоя воркера — по суте `FIRE_NOW` с дедупликацией: скан-подстраховка догоняет всё пропущенное в первый часовой проход, дедуп-ключи делают флад публикаций однократным (дополнение ADR 0048 п.3: «Флад публикаций при первом проходе после простоя — ожидаемая ретроспектива»). Это лучшая из трёх политик для уведомлений: `IGNORE_MISFIRE` штормил бы (уведомления о недели-старых сроках пачкой), `DO_NOTHING` молча терял бы факты.

**Backlog-политика «слать ли пропущенное за прошлые дни»** вытекает из конструкции ног и соответствует практике «не спамить устаревшим»:

- due-нога (`o.date = сегодня`) и reminder-нога (день напоминания = сегодня) — **самогаснущие**: если день прошёл, нога молчит навсегда; пропущенное напоминание не досылается задним числом. Это `DO_NOTHING` по отношению к просроченному дню.
- overdue-нога (`o.date < сегодня`) — **персистентная**: просрочка сканится ежедневно, дедуп (правило, дата) держит одну строку — пользователь получает ровно одно «Платёж просрочен» при любом раскладе, когда бы воркер ни ожил. Это `FIRE_NOW`.

Комбинация «событийные ноги гаснут, статусная нога живёт» — разумный ответ на вопрос тикета о backlog-политике: пропущенный день не досылается, но факт долга не теряется.

### 2.4. Идемпотентность и дедуп

- **River** честно ограничивает гарантию: uniqueness применяется только к вставке, исполнение остаётся «at-least-once guarantee for unique jobs» — уникальная джоба может выполниться дважды; при повторной вставке с тем же ключом «JobInsertResult.Job contains … the preexisting one with matching unique conditions if insertion was skipped» ([River Docs: Unique jobs](https://riverqueue.com/docs/unique-jobs)).
- **system-design-primer** фиксирует то же для очередей: у SQS — «the possibility of messages being delivered twice», и паттерна дедупа в primer'е нет ([README, Asynchronism](https://github.com/donnemartin/system-design-primer#asynchronism)).
- Канон практики: дедуп **на уровне бизнес-эффекта** (idempotency key = бизнес-идентичность события), а не транспорта. Репозиторий делает ровно это дважды: unique-джоба убирает дубли бронирования, дедуп-ключ строки ленты (тип, правило, дата операции) убирает дубли публикаций — при этом пара (правило, дата операции) стабильна относительно времени срабатывания и не зависит от того, кто пришёл первым (джоба или скан). Смена оффсета/перенос срока меняет ключ или уводит джобу в no-op через delivery-time re-check — семантика совпадает с каноном «ключ выводится из фактов триггера».

### 2.5. DST

- Точные переходы IANA лежат в узком утреннем окне: в действующих зонах переходы — между 00:00 и 04:00 местного (EU: 02:00/03:00; Куба 01:00; Чили 24:00→00:00; Azores 01:00; Lord Howe 02:00 ±30 мин; Nuuk 2023: 23:00→00:00). **10:00 и 22:00 в это окно не попадают ни в одной зоне** — оба настенных времени всегда существуют, неоднозначных/несуществующих инстантов не возникает. Дополнение ADR 0048 и тикет #1168 утверждают то же; подтверждаем.
- Обратная сторона: полночь — легитимный момент перехода в ряде зон. Это не теория: robfig/cron чинил баг «adjust times when rolling the clock forward to handle non-existent midnight (#157)» ([README](https://raw.githubusercontent.com/robfig/cron/master/README.md)); в зонах с переходом в 00:00 (исторически Бразилия/Чили) настенная полночь может быть несуществующей (пружинный переход) или удвоенной (осенний). PostgreSQL docs (§9.9.4) поведение для таких времен явно не документирует — разрешение отдано правилам IANA TZDB (несуществующее — офсетом до перехода, двойное — после). Смена на 10:00/22:00 **убирает весь этот класс** из платёжных ног, а не «сужает» его.

### 2.6. Digest против immediate

Платёжные события — **транзакционные**: адресованы конкретному человеку, требуют действия, цена ошибки (молчание) — просрочка и пени. Практика не сворачивает транзакционные уведомления в дайджесты; digest — для не urgent-потока (сводки активности). Репозиторий уже совмещает обе формы: всё всегда пишется в ленту (digest-экран), а каналы (email/push) доставляют транзакционные события по матрице настроек; глушение каналов не трогает ленту. Смена 00:00 → 10:00/22:00 не меняет эту модель — она лишь сдвигает момент доставки внутрь бодрственного окна.

### 2.7. system-design-primer — что в нём есть и чего нет

Тикет ссылается на system-design-primer как ориентир. Честная фиксация: **специализированной главы про notification/push-системы и таймзонное планирование в primer'е нет** — ни в README, ни в подборке design-вопросов (там чат, новостная лента, URL-shortener, краулер, KV-хранилище). Primer даёт фундамент (асинхронность, очереди, task-очереди с планированием, back pressure), но вопрос «уведомление в настенный час по поясу» он не покрывает — потому опора сделана на первоисточники платформ (River, Braze, Quartz, Apple HIG), перечисленные выше.

---

## 3. Сверка с каноном репозитория

### 3.1. Что подтверждено

| Элемент машины | Оценка практикой |
|---|---|
| Граничная River-джоба `ScheduledAt` на точный момент (полночь → 10:00/22:00) | Канон: delayed job per event — River ScheduledAt; задержка < 5 с несущественна против почасового скана |
| Booking-окно 48ч, перезапрос каждый час | Канон: окно ≥ суточного цикла пересчёта, граница не может «проскочить» между проходами; 48ч покрывает и 25-часовые DST-сутки |
| Unique-джобы по args в нетерминальных состояниях | Канон: River UniqueOpts; ровно то, для чего они сделаны (одна джоба «в полёте» на (нога, правило, дата)) |
| Delivery-time re-check (джоба перечитывает живое на пробуждении) | Канон: at-least-once джоба обязана самопроверяться; устаревшее событие — вежливый no-op |
| Дедуп (тип, правило, дата операции) | Канон: idempotency key из бизнес-идентичности; двойной слой с unique-джобой — правильно (вставка ≠ исполнение) |
| Часовой зональный скан-подстраховка | Канон: sweeper против misfire; семантика после простоя = Quartz FIRE_NOW + дедуп, отставание до часа — приемлемая цена |
| Смена полночи на 10:00/22:00 | Канон: локальный пояс получателя + гуманные часы; 10:00 — рекомендуемое утро; полночь — худший час (тишина + Focus-подавление) |

### 3.2. Найденный разрыв: скан-подстраховка обгонит 10:00/22:00

Тикет #1168 формулирует: «Скан-подстраховка будет догонять до часа после границы — это канон, ничего не менять». Проверка по коду показывает, что это верно **только для полуночных границ** и неверно для 10:00/22:00 при текущих запросах:

- Скан-ноги платежей датированы: due — `o.date = ($3 AT TIME ZONE tz)::date` (`notifications.sql:368` — это re-check джобы; скан-нога `ListPaymentDueTargets:271` та же), overdue — `o.date < сегодня` (`:296`, `:392`), reminder — `(o.date − offset) = сегодня` (`:415`, `:464`). Предикат становится истинным **в местную полночь**, за 10 (due/reminder) и 22 (overdue) часа до нового момента границы.
- Часовой скан публикует безусловно (первым же проходом, где предикат истинен), джоба-ровесник не мешает (бронирование — отдельная вставка, публикация скана о стоящих джобах не знает), дедуп молчит, пока никто не публиковал. Итог: уведомление уйдёт ~01:00 местного, назначенная джоба в 10:00/22:00 вставит пусто. Требование владельца «чётко в 10:00/22:00» сломано **страховкой**, а не носителем момента.
- После простоя/при опоздании джобы скан действительно догоняет «до часа после границы» — эта половина формулы #1168 верна; но только если скану запрещено стрелять **до** границы.

Компаньон-изменение маленькое и **уже имеет прецедент в репозитории**: скан задач сравнивает с `now` мгновение срока, а не дату с «сегодня» — `(t.due_date + t.due_time) AT TIME ZONE u.timezone <= $2::timestamptz` (`notifications.sql:527`). Тем же способом платёжным скан-ногам нужен instant-гейт: тот же `fire_at`-выражение, что у booking-запроса, с предикатом `<= now`:

- due: `(o.date::timestamp + time '10:00') AT TIME ZONE tz <= now` (если гриллинг поставит due в 10:00);
- reminder: `((o.date − offset)::timestamp + time '10:00') AT TIME ZONE tz <= now`;
- overdue: `((o.date + 1)::timestamp + time '22:00') AT TIME ZONE tz <= now`.

При этом date-половины предикатов (`= сегодня` / `< сегодня`) остаются — они несут семантику ноги (самогасание due/reminder, персистентность overdue). Re-check'и джоб (`GetScheduled*Payment`) менять не нужно: их дата-против-«сегодня» предикаты истинны и в 10:00/22:00 (обновить только комментарии) — с этим тикет #1168 прав.

Симметрично: если ноги «автоплатёж исполнен» (#1169) даётся скан-нога, её предикат обязан быть instant-гейтнутым с первого дня — копировать готовый паттерн, а не date-only.

### 3.3. Краевые случаи

1. **DST.** 10:00/22:00 всегда легальны (§2.5) — DST-крайний случай полуночи исчезает из платёжных ног. Обратное тоже важно: в зонах с переходом в 00:00 текущая полночная машина могла бы вставать на несуществующем/удвоенном инстанте — смена времени это лечит. Арифметика правильная и после смены: `+1 день` в date-домене, `AT TIME ZONE` в конце; не брать `interval '24 hours'` на `timestamptz` (предупреждение PG §9.9.4).
2. **Отставание скан-подстраховки.** При instant-гейте: пропущенная граница догоняется в первый проход после неё — до часа; приемлемо и канонично (Braze hold-and-deliver — та же форма «доставить в следующее доступное время»).
3. **Ретроспектива после простоя.** День-событийные ноги (due/reminder) самогасаются — вчерашнее не досылается; overdue персистентна — одно уведомление о долге независимо от длительности простоя. Backlog-политика не требует решений — она уже согласована (§2.3).
4. **Пуш ночью.** Overdue в 22:00 — на границе типового окна тишины (Braze: 22:00–06:00). Для транзакционного «последнего звонка» приемлемо; альтернатива для гриллинга — 20:00–21:00, если владельцу важно не задевать тишину вовсе. Email в 22:00 — вне зоны риска.
5. **Booking-окно против смены времени правила.** Оффсет/дата меняются после бронирования — старая джоба уходит no-op по re-check'у (текущий оффсет), новая граница бронируется своим проходом; окно 48ч > максимального сдвига даты одной правкой не гарантируется, но повторный запрос каждый час перевыбирает актуальные границы — разрыва нет.
6. **Секундная задержка River** (< 5 с после `scheduled_at`) — шум на фоне почасового скана; «чётко в 10:00» стоит читать как «в 10:00:0x».

---

## 4. Рекомендации для #1168 (времена)

1. Смена трёх `fire_at`-выражений booking-запросов — правильная и достаточная для **носителя момента**; машина (окно, unique, дедуп, re-check) не меняется — подтверждено.
2. **Обязательно добавить компаньон-изменение**: instant-гейт на трёх скан-ногах платежей (`ListPaymentDueTargets`, `ListPaymentOverdueTargets`, `ListPaymentReminderTargets`) по образцу скан-ноги задач — иначе уведомления уйдут в ~01:00 и смысл всей смены теряется. Комменты `GetScheduled*Payment` обновить (предикаты держатся).
3. Переписать тесты, ассертящие инстанты (`payment_scan_integration_test.go`, `payments_publisher_test.go`, `payment_boundary_test.go`) и **добавить тест на момент скан-ноги**: «до 10:00 местного скан молчит, после — публикует».
4. Время due-ноги оставить решению гриллинга; по практике оба варианта валидны: 10:00 (единый утренний каскад с напоминанием) или вечер (если «Оплатите платёж» читается как вечернее напоминание перед сроком) — индустрия даёт перевес утру.

## 5. Рекомендации для #1169 («автоплатёж исполнен»)

1. Модель, дедуп `payment_auto_paid:<rule_id>:<YYYY-MM-DD>`, очередь `notifications_payments`, unique по (правило, дата) — по канону #776, сомнений нет.
2. **Порядок с тиком автосписания — главный тонкий момент.** Списание делает материализационный тик (ежечасный); джоба в 10:00 перечитывает вхождение: исполнено — публикует, ещё planned — вежливый no-op. Если тик зоны исполняет вхождение после 10:00, публикацию догонит скан в первый проход после исполнения — фактическое время «10:00 или ближайший часовой проход после исполнения». Гриллингу стоит зафиксировать, что это допустимая семантика (либо потребовать приоритета тика до 10:00 — но это отдельное изменение драйвера тика).
3. Кейс «оплатили вручную до 10:00»: re-check джобы видит погашенное вхождение — молчок; кейс «не исполнилось»: молчок, 22:00 скажет просрочка. Оба уже совпадают с формулировкой тикета — подтверждить гриллингом.
4. Скан-ногу этой ноги делать сразу instant-гейтнутой (§3.2).

## 6. Открытые риски

- **Риск-1 (блокирующий для цели смены): pre-emption сканом** — закрывается компаньон-изменением §3.2; если его пропустить, эффект будет противоположен ожидаемому (уведомления в час ночи).
- **Риск-2: 22:00 на границе окна тишины** — продуктовый, на решение владельца; цена ошибки низкая (транзакционный контент).
- **Риск-3: семантика #1169 при позднем тике** — «10:00 или следующий час»; зафиксировать на гриллинге.
- **Риск-4: тестовая глина** — 9+ инстантов в интеграционных тестах; механическая работа, но легко забыть комменты SQL-запросов (тикет уже помечает).
- Внеполосное: пояс собственника в репозитории — ручной выбор с фолбэком Москвы, а не автодетект устройства (ADR 0048 п.1); индустрия по умолчанию берёт пояс устройства. Это осознанное решение репозитория, менять не просили — фиксирую как контекст.

## Источники

- River Docs — Scheduled jobs: <https://riverqueue.com/docs/scheduled-jobs>
- River Docs — Unique jobs: <https://riverqueue.com/docs/unique-jobs>
- Braze Docs — Quiet hours: <https://www.braze.com/docs/user_guide/messaging/messaging_fundamentals/quiet_hours>
- Quartz Scheduler — Tutorials 04 (Trigger misfire) и 06 (CronTrigger): <https://www.quartz-scheduler.org/documentation/quartz-2.3.0/tutorials/tutorial-lesson-04.html>, <https://www.quartz-scheduler.org/documentation/quartz-2.3.0/tutorials/tutorial-lesson-06.html>
- robfig/cron — README (CRON_TZ, DST midnight #157): <https://github.com/robfig/cron/blob/master/README.md>
- PostgreSQL Docs §9.9.4 (AT TIME ZONE, DST-арифметика): <https://www.postgresql.org/docs/current/functions-datetime.html>
- Apple Human Interface Guidelines — Notifications / Managing notifications: <https://developer.apple.com/design/human-interface-guidelines/notifications>, <https://developer.apple.com/design/human-interface-guidelines/managing-notifications>
- OneSignal — Scheduling Push Notifications by User Time Zone: <https://onesignal.com/blog/deliver-by-timezone-push-notification>; Batch — What is the best time to send push notifications: <https://doc.batch.com/guides-and-best-practices/orchestration/what-is-the-best-time-to-send-push-notifications>
- system-design-primer — README, Asynchronism: <https://github.com/donnemartin/system-design-primer#asynchronism>
- Код репозитория: `apps/backend/db/queries/notifications.sql`, `apps/backend/internal/notifications/application/{payments_publisher,scan,tasks_publisher}.go`, `apps/backend/internal/notifications/adapters/notificationsjob/args.go`; `docs/adr/0048-payments-tick-drivers-owner-timezone.md`; `apps/backend/internal/notifications/GLOSSARY.md`
