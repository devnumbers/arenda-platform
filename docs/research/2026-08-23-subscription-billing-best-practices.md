# Лучшие практики организации SaaS-подписок: внешний бенчмарк для ревью модуля billing

- Research-тикет: «Лучшие практики организации подписок (внешнее исследование)».
- Дата: 2026-08-23. Все факты — из первоисточников (официальная документация Stripe Billing, Chargebee, Recurly, Kill Bill, Lago), ссылка у каждого утверждения.
- Назначение: чек-лист для ревью кода модуля billing. Контекст потребителя чек-листа: провайдер — Т-Касса; деньги — BIGINT-копейки; одна подписка на пользователя (базовый/про/бизнес); апгрейд немедленный по полной цене с новым периодом; даунгрейд отложенный на конец оплаченного периода; статусы `active`/`grace`/`cancelled`; автопродление по сохранённой карте (ADR 0008, ADR 0037, ADR 0039).
- Метод: web-research по официальным докам пяти биллинговых систем 2026-08-23; сопоставление с моделью из `apps/backend/internal/billing/CONTEXT.md` и ADR — только на уровне терминологии, код не аудировался.

## Оглавление

1. [Резюме](#резюме)
2. [Модель подписки и конечный автомат](#1-модель-подписки-и-конечный-автомат)
3. [Апгрейд и даунгрейд](#2-апгрейд-и-даунгрейд-proration)
4. [Отмена и восстановление](#3-отмена-и-восстановление-cancel--reactivate)
5. [Неудачные списания: dunning и grace](#4-неудачные-списания-dunning-и-grace)
6. [Сохранённые платёжные средства и рекуррентные списания](#5-сохранённые-платёжные-средства-и-рекуррентные-списания-citmit)
7. [Идемпотентность и exactly-once](#6-идемпотентность-и-exactly-once)
8. [Вебхуки](#7-вебхуки)
9. [Сверка с провайдером (reconciliation)](#8-сверка-с-провайдером-reconciliation)
10. [Работа с деньгами](#9-работа-с-деньгами)
11. [Сводный чек-лист для ревью кода](#сводный-чек-лист-для-ревью-кода)

---

## Резюме

Главные выводы из пяти первоисточников, в терминах нашей модели:

1. **Минимальный зрелый автомат**: `active` → `past_due/grace` (неудачное списание) → терминальное `canceled/expired`; отдельное промежуточное состояние «отмена запланирована, доступ до конца периода» (Stripe `cancel_at_period_end`, Chargebee `non_renewing`, Recurly `canceled`-pre-expiry). Наш `cancelled`-с-доступом-до-конца-периода — это индустриальный стандарт; терминальная «истинная» отмена у нас выражается переходом на базовый тариф.
2. **`canceled`-по-периоду и «отменено навсегда» — разные состояния** у всех провайдеров; смешение их в одно — источник багов (реактивация возможна только до конца оплаченного периода).
3. **Апгрейд немедленный, даунгрейд отложенный** — рекомендованный по умолчанию паттерн (Stripe: end-of-term schedules для дешёвых планов; Kill Bill: `IMMEDIATE` для upgrade, `END_OF_TERM` для downgrade в дефолтных правилах). Наша схема совпадает.
4. **Proration — не обязательная, а настраиваемая политика**: Stripe даёт три режима (`create_prorations`/`always_invoice`/`none`) и отдельный режим «сброс периода + полная цена» (`billing_cycle_anchor=now`). Наш «апгрейд по полной цене с новым периодом» — легитимная альтернатива прорации, главное — не смешивать режимы незаметно для пользователя.
5. **Кредит за неиспользованное время не возвращается деньгами автоматически** (Stripe: «Negative prorations aren't automatically refunded»; Kill Bill банкирует на баланс через `CBA_ADJ`). Даунгрейд в конце периода вообще обходит проблему кредита.
6. **Первый неудачный список не блокирует доступ**: все провайдеры дают окно повторов (Stripe Smart Retries — дефолт 8 попыток за 2 недели; Chargebee Smart — до 12; Recurly — кампания ретраев с dunning-циклом; Kill Bill — 8,8,8 дней). Наш grace-период — это past_due+retries-окно.
7. **Hard vs soft declines**: жёсткие отказы (украденная карта, неверный номер) не ретраятся тем же методом — только после смены способа оплаты (Stripe, Chargebee). Ретраи без учёта типа отказа — жечь авторизации и время.
8. **CIT/MIT — явная семантика**: первый платёж в сессии клиента (CIT) сохраняет метод и создаёт основу для последующих безбальных списаний (MIT, off-session); при MIT-отказе клиента возвращают в сессию. Наш порт с явным CIT/MIT-признаком — правильно.
9. **Вебхуки**: порядок доставки не гарантирован и дубликаты — норма; идемпотентность по event/object-id обязательна; при расхождении — тянуть статус у провайдера как источника истины. Индустриальный дефолт — быстрый 2xx + фоновая очередь; синхронная обработка в HTTP-бюджете с не-200 при сбое (наш выбор, ADR 0039) — допустимая альтернатива при идемпотентности, потому что ретраи выполняет провайдер.
10. **Сверка (reconciliation) — отдельный воркер**: подметание недоставленных событий (Stripe: list events `delivery_success=false`, окно 30 дней), периодическая перепроверка состояния без событий (Kill Bill `autoReEvaluationInterval`), сторож зависших промежуточных статусов (наш `refunding`-сага, ADR 0037).
11. **Деньги — целые минорные единицы во всех слоях** (Stripe: «amount values in the currency's minor unit», RUB — двухдесятичная, т.е. копейки); округления — детерминированные, остаток относить на баланс, а не терять (Stripe UGX-практики, Kill Bill `CBA_ADJ`).
12. **Идемпотентные ключи на мутациях к провайдеру**: сохранять результат первой попытки (включая ошибку), повтор с тем же ключом возвращает тот же исход, ключ живёт ограниченное время (Stripe: 24 часа); конфликт параметров при том же ключе — ошибка.
13. **Неизменяемость**: события и журнал переходов не переписываются — корректировки добавляются новыми записями (Stripe: «You can't change `Event` objects after creation»; Kill Bill: корректировки новыми `ITEM_ADJ`/`REPAIR_ADJ` вместо правки исходных, `auditLogs` на объектах). Наша «история переходов подписки» — тот же паттерн.
14. **Уведомления — часть dunning, а не довесок**: письмо на каждый неудачный список, напоминание до конца grace, уведомление об истечении карты за месяц (Stripe revenue recovery emails).

---

## 1. Модель подписки и конечный автомат

### Must

- **Различать «запланированную отмену» и «терминальную отмену».** Stripe: `cancel_at_period_end=true` оставляет подписку активной до конца оплаченного периода (доступ сохраняется, автопродление выключено), `canceled` — терминальное состояние, обновить нельзя ([docs.stripe.com/billing/subscriptions/cancel](https://docs.stripe.com/billing/subscriptions/cancel), [overview](https://docs.stripe.com/billing/subscriptions/overview)). Chargebee разделяет `non_renewing` («will be canceled at the end of the current term») и `cancelled` («no longer in service») ([apidocs.chargebee.com/docs/api/subscriptions](https://apidocs.chargebee.com/docs/api/subscriptions)). Recurly: `canceled` — pre-expiry состояние с сохранением доступа, реактивация возможна до expiration; `expired` — чёрн, реактивировать нельзя ([docs.recurly.com/recurly-subscriptions/docs/expire-subscription](https://docs.recurly.com/recurly-subscriptions/docs/expire-subscription), [subscription dashboard](https://docs.recurly.com/recurly-subscriptions/docs/subscription-dashboard)).
- **`past_due`/grace — отдельный статус с продолжением доступа.** Stripe: `past_due` — оплата последнего инвойса не прошла, подписка продолжает жить, возвращается в `active` при оплате ([overview#subscription-statuses](https://docs.stripe.com/billing/subscriptions/overview)). Chargebee: подписка остаётся active, пока идёт dunning ([KB: Subscription Status as Active when Payment Failed on Renewal](https://www.chargebee.com/docs/billing/2.0/kb/billing/subscription-status-as-active-when-payment-failed-on-renewal)). Recurly: состояние подписки и состояние инвойса — отдельные объекты; подписка может быть `active` при past-due инвойсе ([Why Is a Subscription Active but the Invoice Is Past Due?](https://support.recurly.com/hc/en-us/articles/46649114797204-Why-Is-a-Subscription-Active-but-the-Invoice-is-Past-Due)).
- **Материализованное текущее состояние + иммутабельный журнал переходов.** Все провайдеры хранят текущий статус полем объекта (Stripe `status`, Chargebee `status`, Recurly `state`) — не вычисляют его на лету из событий; при этом переходы фиксируются неизменяемыми записями: Stripe события нельзя изменить после создания («You can't change `Event` objects after creation» — [docs.stripe.com/webhooks#api-versioning](https://docs.stripe.com/webhooks#api-versioning)), Kill Bill ведёт `auditLogs` на объектах и исправляет документы новыми корректирующими айтемами (`ITEM_ADJ`, `REPAIR_ADJ`), а не правкой исходных ([docs.killbill.io/latest/userguide_subscription](https://docs.killbill.io/latest/userguide_subscription)).
- **Явные недопустимые переходы.** Терминальные состояния не покидаются: Stripe `canceled` — «terminal state that can't be updated», `incomplete_expired` не биллится ([overview](https://docs.stripe.com/billing/subscriptions/overview)); Recurly `expired` не реактивируется — нужна новая подписка ([expire-subscription](https://docs.recurly.com/recurly-subscriptions/docs/expire-subscription)). Автомат переходов должен запрещать такие рёбра, а не полагаться на дисциплину вызовов.
- **Хранить причину и инициатора каждого перехода.** Chargebee: `cancel_reason` проставляется автоматически (`not_paid`, `no_card`, `fraud_review_failed`, …), `cancel_schedule_created_at` фиксирует расписание отмены ([apidocs.chargebee.com/docs/api/subscriptions](https://apidocs.chargebee.com/docs/api/subscriptions)); Stripe — `cancellation_details` с причиной и комментарием ([cancel](https://docs.stripe.com/billing/subscriptions/cancel)).

### Recommended

- **Разделять entitlement (доступ) и billing (деньги).** Kill Bill: entitlement и billing «connected, but not necessarily aligned» — можно отменить доступ немедленно, а биллинг завершить по charged-through date; overdue-блокировка выключает и то и другое отдельными флагами ([userguide_subscription](https://docs.killbill.io/latest/userguide_subscription), [entitlement_subsystem](https://docs.killbill.io/latest/entitlement_subsystem)). Практика: статус «что пользователь может делать» не обязан совпадать 1:1 со статусом «что происходит с деньгами» (у нас это readonly-режим и лимиты поверх статусов).
- **Не заводить состояния, которые не производятся кодом.** Stripe поддерживает `unpaid`, `paused`, `incomplete` и т.д., но это опции поведения, а не обязательный набор; ADR 0008 нашего репо уже удалил мёртвый `blocked` — тот же принцип: набор состояний = ровно то, что реально производит автомат.
- **Pause — отдельная отмена операция.** Stripe: приостановка сборов (`pause_collection`) не меняет статус подписки и рекомендуется как триггер остановки сервиса отдельно от отмены ([cancel](https://docs.stripe.com/billing/subscriptions/cancel)); Chargebee `paused` — не продлевается, пока не возобновлён ([apidocs](https://apidocs.chargebee.com/docs/api/subscriptions)).

*Соотнесение с нашей моделью: `active`/`grace`/`cancelled` + «история переходов подписки» с причиной и инициатором (billing CONTEXT.md) соответствуют must-практикам; «cancelled» в нашей терминологии — это Stripe `cancel_at_period_end` + Chargebee `non_renewing` (доступ до конца периода), терминальная фаза выражается переходом на базовый тариф.*

---

## 2. Апгрейд и даунгрейд (proration)

### Must

- **Асимметричная политика: дорогое — сразу, дешёвое — в конце периода.** Stripe: для смены тарифа в конце периода — subscription schedules; при downgrade выдаются кредитные proration'ы, которые не возвращаются деньгами автоматически («Negative prorations aren't automatically refunded») ([upgrade-downgrade](https://docs.stripe.com/billing/subscriptions/upgrade-downgrade), [prorations#credit-prorations](https://docs.stripe.com/billing/subscriptions/prorations)). Kill Bill в примере правил каталога: апгрейды — `IMMEDIATE`, остальное — `END_OF_TERM` ([userguide_subscription](https://docs.killbill.io/latest/userguide_subscription)).
- **Смена тарифа не должна незаметно удваивать биллинг.** Stripe предостерегает: при обновлении item'а без указания id заменяемого item'а новый тариф добавляется рядом со старым («Failing to do so results in adding the new price so both prices are active») ([upgrade-downgrade](https://docs.stripe.com/billing/subscriptions/upgrade-downgrade)). Для модели «одна подписка на пользователя» инвариант: активный тариф в каждый момент ровно один (у нас — сменой записи, не добавлением).
- **Сброс периода — явная политика с немедленной оплатой.** Stripe: смена тарифа с разным периодом или с `billing_cycle_anchor=now` переносит дату списаний на день смены; `always_invoice` считает proration и сразу выставляет инвойс; при немедленном списании и отказе платежа смена вступает, подписка уходит в `past_due` ([upgrade-downgrade#immediate-payment](https://docs.stripe.com/billing/subscriptions/upgrade-downgrade), [billing-cycle#changing](https://docs.stripe.com/billing/subscriptions/billing-cycle)). Наш «апгрейд по полной цене с новым периодом» — вариант «reset anchor», легитимен; чек-лист: апгрейд применяется только после успешной оплаты.
- **Не выдавать кредит за время, которое не оплачено.** Stripe: при смене тарифа поверх неоплаченного инвойса классическая прорация может дать кредит за неиспользованное время тарифа, который клиент не оплачивал («might receive a credit for unused time... even if they haven't paid for that time yet») — Stripe предлагает отключить proration или аннулировать старый инвойс ([prorations#prorations-and-unpaid-invoices](https://docs.stripe.com/billing/subscriptions/prorations)). Модель «полная цена + новый период» обходит этот класс ошибок по построению.

### Recommended

- **Показывать сумму до списания.** Stripe: preview proration через `create_preview` до применения; из-за посекундной прорации суммы «плывут» между preview и применением — фиксируй `proration_date` ([prorations#preview-proration](https://docs.stripe.com/billing/subscriptions/prorations)). Для нас: UI апгрейда должен показывать полную цену нового тарифа до подтверждения (у нас без прораций сумма детерминирована — проще).
- **Класть кредиты на баланс, а не рефандить.** Kill Bill: если кредит за старый план превышает спис за новый, разница обнуляется айтемом `CBA_ADJ` — кредит на счёт аккаунта ([userguide_subscription](https://docs.killbill.io/latest/userguide_subscription)). Lago: досрочное прекращение pay-in-advance подписки по умолчанию рождает credit note за неиспользованное время ([subscription-object](https://getlago.com/docs/api-reference/subscriptions/subscription-object)).
- **Даунгрейд с проверкой лимитов в момент вступления, не в момент заказа.** Chargebee: заказ отмены/смены на конец терма — расписание; применение — по наступлении срока ([apidocs](https://apidocs.chargebee.com/docs/api/subscriptions)). Наша архивация избыточных объектов при даунгрейде (ADR 0008) должна выполняться в фазе применения, с актуальным на тот момент списком объектов.

*Соотнесение: ADR 0008 — апгрейд немедленный после успешной оплаты с новым периодом, даунгрейд отложенный через `pending_*`-поля и авто-включение автопродления; это закрывает все must-пункты без прораций.*

---

## 3. Отмена и восстановление (cancel / reactivate)

### Must

- **Отмена по умолчанию — с сохранением доступа до конца оплаченного периода.** Stripe: `cancel_at_period_end=true` «allows the subscription to complete the duration of time the customer has already paid for» ([cancel#cancel-at-the-end-of-the-current-billing-period](https://docs.stripe.com/billing/subscriptions/cancel)). Recurly: отменённая подписка «remains active until the selected end date. The customer retains access to the service during this period» ([expire-subscription](https://docs.recurly.com/recurly-subscriptions/docs/expire-subscription)).
- **Отмена мгновенно выключает автопродление.** Chargebee `non_renewing` = «will be canceled at the end of the current term» — продления прекращаются сразу, хотя доступ остаётся ([apidocs](https://apidocs.chargebee.com/docs/api/subscriptions)). Stripe: после `cancel_at_period_end` подписка не продлевается, а по концу периода генерирует `customer.subscription.deleted` ([cancel](https://docs.stripe.com/billing/subscriptions/cancel)).
- **Восстановление — только до конца периода.** Stripe: реактивировать запланированную отмену можно в любой момент до конца периода (`cancel_at_period_end=false`); «You can't reactivate a canceled subscription» ([cancel#stop-a-pending-cancellation](https://docs.stripe.com/billing/subscriptions/cancel)). Recurly: реактивация `canceled` возможна до expiration; `expired` — нельзя ([subscription dashboard](https://docs.recurly.com/recurly-subscriptions/docs/subscription-dashboard)).
- **Отделять отмену от немедленного прекращения доступа и от приостановки.** Stripe: отмена по умолчанию немедленна для биллинга, но рекомендуемый поток услуги — отложенный; pause payment collection — отдельная операция, не меняющая статус ([cancel](https://docs.stripe.com/billing/subscriptions/cancel)). Смешение трёх семантик в одном ручье — антипаттерн.

### Recommended

- **События об отмене: отдельно «запланирована», отдельно «состоялась».** Stripe: `customer.subscription.updated` при установке `cancel_at_period_end` и `customer.subscription.deleted` при фактической отмене (в т.ч. по концу периода) ([cancel#identify-cancellation-events](https://docs.stripe.com/billing/subscriptions/cancel)). Аналог для внутренних событий: переход «cancelled» (план отмены) и переход на базовый тариф (исполнение) — разные записи журнала.
- **Фиксировать причину отмены.** Stripe `cancellation_details` (reason/comment), Chargebee `cancel_reason` + `cancel_reason_code` ([cancel](https://docs.stripe.com/billing/subscriptions/cancel), [apidocs](https://apidocs.chargebee.com/docs/api/subscriptions)).
- **Немедленная отмена — отдельная операция с возвратом/невозвратом средств, выбираемая явно.** Stripe Dashboard: при отмене выбирают момент (сразу/конец периода/дата) и возврат (prorated/full/none); при `prorate`-отмене счёт за usage и outstanding-айтемы — явные решения ([cancel#prorate-for-usage-based-billing](https://docs.stripe.com/billing/subscriptions/cancel)).

*Соотнесение: наш `cancelled` = автопродление выключено, изменения разрешены до конца периода, восстановление оплатой тарифа — точное соответствие `cancel_at_period_end`/`non_renewing`/Recurly pre-expiry; терминальная фаза — переход на базовый тариф.*

---

## 4. Неудачные списания: dunning и grace

### Must

- **Первый отказ ≠ блокировка: окно повторов обязательно.** Stripe: Smart Retries, рекомендуемый дефолт «8 tries within 2 weeks»; ретраи продолжаются до 3 дней доставки событий и до 2 месяцев по настройке ([smart-retries](https://docs.stripe.com/billing/revenue-recovery/smart-retries)). Chargebee: Smart — до 12 ретраев; Custom — до 5 с расписанием в днях; dunning period — «time period during which an invoice will stay in dunning before the final action» ([dunning-v2](https://www.chargebee.com/docs/payments/2.0/dunning/dunning-v2)). Recurly: dunning-кампания ретраев после past-due; dunning начинается через 24 часа после due date ([dunning-management](https://docs.recurly.com/recurly-subscriptions/docs/dunning-management), [invoice-management](https://docs.recurly.com/recurly-subscriptions/docs/invoice-management)).
- **Различать hard и soft declines.** Stripe: при hard decline (список кодов: `lost_card`, `stolen_card`, `incorrect_number`, `authentication_required`, …) ретраи не выполняются, пока не появится новый способ оплаты ([smart-retries#hard-decline-codes](https://docs.stripe.com/billing/revenue-recovery/smart-retries)). Chargebee: hard declines «not retried until the payment method is updated», soft — ретраятся ([dunning-v2](https://www.chargebee.com/docs/payments/2.0/dunning/dunning-v2)).
- **Явно настроенное окончание dunning: что происходит с подпиской и с инвойсом.** Stripe: после финальной попытки подписка уходит в `canceled`, `unpaid` или остаётся `past_due` — по настройке ([smart-retries](https://docs.stripe.com/billing/revenue-recovery/smart-retries)). Chargebee: «let the subscription remain active or cancel the subscription at once» + инвойс в `not paid`/void/write-off/reverse ([dunning-v2](https://www.chargebee.com/docs/payments/2.0/dunning/dunning-v2)). Recurly: конец цикла — fail инвойса (подписка истекает) или остаётся past due с бесконечными ретраями ([dunning-management](https://docs.recurly.com/recurly-subscriptions/docs/dunning-management)).
- **Уведомлять клиента на каждом шаге.** Stripe: письмо после каждого неудачного списания со ссылкой обновить способ оплаты; напоминания, пока инвойс не оплачен или dunning не истечёт ([customer-emails](https://docs.stripe.com/billing/revenue-recovery/customer-emails), [dunning-v2](https://www.chargebee.com/docs/payments/2.0/dunning/dunning-v2)).
- **Возврат в active при оплате в любой момент grace.** Stripe: «The subscription status becomes `active` regardless of whether the payment is done before or after the latest invoice due date» ([overview#subscription-statuses](https://docs.stripe.com/billing/subscriptions/overview)).

### Recommended

- **Умный выбор момента ретрая лучше равномерного расписания.** Stripe Smart Retries выбирает время по ML-сигналам (время суток, паттерны устройства); Chargebee Smart — «based on transaction patterns and the type of gateway transaction errors» ([smart-retries](https://docs.stripe.com/billing/revenue-recovery/smart-retries), [dunning-v2](https://www.chargebee.com/docs/payments/2.0/dunning/dunning-v2)). Для нас: как минимум не ретраить в один и тот же час при одинаковой ошибке и уходить от «n-й день подряд» в пользу пары точек за окно.
- **Планировать следующий ретрай явно и показывать его.** Stripe: `next_payment_attempt` на инвойсе, `attempt_count` в вебхуке ([smart-retries#webhook-events](https://docs.stripe.com/billing/revenue-recovery/smart-retries)). Внутренний аналог — колонка времени следующей попытки у платежа в grace.
- **Предупреждать заранее об истечении карты.** Stripe: письмо за месяц до истечения карты по умолчанию ([customer-emails#expiring-card-notifications](https://docs.stripe.com/billing/revenue-recovery/customer-emails)). **Наше расхождение (тикет #418, wontfix):** не делаем — «истечение привязанной карты» не существует как состояние продукта (срок не отдаётся в API/UI), отказ списания по истёкшей карте покрывается реактивным контуром (grace + dunning-повторы + GraceEntered/GraceExpiring); проактивный опрос GetCardList по всем пользователям — постоянная инфраструктура ради редкого случая.

*Соотнесение: наш grace-период = past_due+dunning-окно; истечение grace → переход на базовый тариф + архивация избыточных объектов — это заряд «final action» Stripe/Chargebee в варианте «cancel» (у нас — даунгрейд, т.к. есть бесплатный базовый тариф); GraceEntered/GraceExpiring-события соответствуют практике уведомлений.*

---

## 5. Сохранённые платёжные средства и рекуррентные списания (CIT/MIT)

### Must

- **Явная CIT/MIT-семантика: первый платёж в сессии, рекуррентные — off-session по сохранённому методу.** Stripe: сохранить метод при первом платеже (`setup_future_usage`), затем списывать `off_session=true, confirm=true` по сохранённому PaymentMethod; off-session = «without the direct involvement of the customer, using previously-collected payment information» ([save-during-payment](https://docs.stripe.com/payments/save-during-payment)). Наш порт с явным «инициатор операции (CIT/MIT)» — соответствует.
- **Явное согласие клиента на сохранение и будущие списания.** Stripe compliance: термины (сумма/периодичность/политика отмены), явный чекбокс «Save my payment method», запись согласия; использовать сохранённый метод только для заявленной цели ([save-during-payment#compliance](https://docs.stripe.com/payments/save-during-payment)). Для РФ с Т-Кассой — требование связки/первого CIT-платежа того же рода.
- **Ретраи идут на актуальный активный метод.** Stripe: приоритет методов при ретраях — subscription default → customer default; «update the field where the previous payment failed», иначе ретраи продолжатся старой картой ([smart-retries#payment-method-ordering](https://docs.stripe.com/billing/revenue-recovery/smart-retries)). У нас один активный способ оплаты — смена активного метода в grace должна подхватываться следующей попыткой.
- **Плательщик опознаётся по токену метода, а не по реквизитам.** Stripe: сохранённый метод — идентификатор `PaymentMethod`, привязанный к клиенту; карты обновляются на стороне провайдера без передачи реквизитов мерчанту ([save-during-payment#charge-the-saved-payment-method-later](https://docs.stripe.com/payments/save-during-payment)).

### Recommended

- **Обновление карт провайдером (Account Updater) — где доступно.** Stripe автоматически обновляет сохранённые карты при перевыпуске/истечении через сети; у Т-Кассы прямого аналога нет — практический минимум: следить за сроком действия и предупреждать заранее ([cards overview / automatic card updates](https://docs.stripe.com/payments/cards/overview), [support-заметка Card Updater](https://support.stripe.com/questions/how-can-i-see-cards-updated-automatically-via-card-updater)).
- **При off-session отказе из-за аутентификации — вернуть клиента в сессию.** Stripe: off-session платёж запрашивает exemption на основе предыдущего on-session платежа; если банк требует 3DS — PaymentIntent уходит в ошибку/`requires_action`, клиента приводят в сессию ([save-during-payment#charge-the-saved-payment-method-later](https://docs.stripe.com/payments/save-during-payment), [overview#requires-action](https://docs.stripe.com/billing/subscriptions/overview)). Аналог в РФ — подтверждение по СМС/3DS при привязке: списания по токену после подтверждённого CIT.
- **Хранить у привязки срок жизни сессии привязки и «плейсхолдер» до подтверждения.** Наш паттерн «сессия привязки карты» с RequestKey и сроком жизни до подтверждения — соответствует модели SetupIntent (создание метода отложено до подтверждения клиентом) ([save-during-payment](https://docs.stripe.com/payments/save-during-payment)).

---

## 6. Идемпотентность и exactly-once

### Must

- **Идемпотентные ключи на всех платёжных мутациях к провайдеру.** Stripe API: ключ сохраняет статус и тело первого выполнения («regardless of whether it succeeds or fails» — включая 500); повтор с тем же ключом возвращает тот же исход; ключи до 255 символов, рекомендованы UUID v4; живут ≥24 часов; несовпадение параметров при том же ключе — ошибка («to prevent accidental misuse») ([api/idempotent_requests](https://docs.stripe.com/api/idempotent_requests)). Практика для ревью: каждый исходящий вызов «создать платёж/возврат» несёт стабильный ключ, повтор сетевой ошибки не создаёт вторую операцию.
- **Дедупликация вебхуков по идентификатору события с персистентной отметкой.** Stripe: «Webhook endpoints might occasionally receive the same event more than once. You can guard against duplicated event receipts by logging the event IDs you've processed, and then not processing already-logged events»; для раздельных дублей — пара `event.type` + id объекта ([webhooks#handle-duplicate-events](https://docs.stripe.com/webhooks#handle-duplicate-events)). Плюс защита от гонки при повторном прогоне бэкфилла: состояния processing/processed в БД ([process-undelivered-events#process-the-events](https://docs.stripe.com/webhooks/process-undelivered-events)).
- **Двойное применение события — no-op, а не ошибка.** Stripe-паттерн: получив уже обработанное событие, «ignore the event and return a successful response to stop future retries» ([process-undelivered-events](https://docs.stripe.com/webhooks/process-undelivered-events)). Повторная доставка вебхука должна быть идемпотентным no-op'ом (наш billing CONTEXT) — иначе ретраи провайдера будут бесконечными.
- **Уникальные ограничения на уровне хранилища как последний рубеж.** Следствие практик выше: уникальный индекс на внешнем идентификаторе платежа/события; «списание ровно одно на период» обеспечивается схемой, а не только логикой (у Kill Bill то же на уровне движка: повторные ретраи создают транзакции на том же Payment, двойная оплата блокируется — [userguide_subscription](https://docs.killbill.io/latest/userguide_subscription)).

### Recommended

- **Идемпотентность и для не-платёжных операций.** Stripe: все POST принимают ключи; GET/DELETE идемпотентны по определению ([api/idempotent_requests](https://docs.stripe.com/api/idempotent_requests)).
- **Тайм-аут ≠ неудача: исход «в полёте» требует сверки, а не слепого повтора.** Stripe сохраняет даже 500-е ответы первого выполнения; повтор после тайм-аута с тем же ключом вернёт исходный результат, а не выполнит вторую операцию ([api/idempotent_requests](https://docs.stripe.com/api/idempotent_requests)). Для нас: платёж в промежуточном статусе после тайм-аута адаптера — кандидат сторожа reconciliation, не повторного запуска.

---

## 7. Вебхуки

### Must

- **Проверка подлинности каждого вебхука.** Stripe: подпись HMAC-SHA256 в заголовке с timestamp; сравнение в постоянном времени; отклонение схем кроме текущей; timestamp-тOLERANCE (дефолт библиотек 5 минут) против replay-атак («Don't use a tolerance value of 0» — это отключает проверку свежести); плюс IP-allowlist как второй слой; секреты периодически ротировать ([webhooks#best-practices](https://docs.stripe.com/webhooks#best-practices), [webhooks#preventing-replay-attacks](https://docs.stripe.com/webhooks#preventing-replay-attacks)).
- **Не полагаться на порядок доставки.** Stripe: «Stripe doesn't guarantee the delivery of events in the order that they're generated»; событие «succeeded после failed» может прийти в любом порядке — недостающие объекты дотягивать из API ([webhooks#event-ordering](https://docs.stripe.com/webhooks#event-ordering)). Наш ADR 0039 (сверка со статусом провайдера при out-of-order) — та же практика.
- **Идемпотентная обработка дублей** (см. секцию 6) — повторные доставки — норма, а не исключение.
- **Ответ 2xx только после успешной обработки; сбой — не-2xx, чтобы провайдер ретраил.** Stripe: ретраи до 3 дней с экспоненциальным backoff в live-режиме; «always-200» глушит доставку и теряет события ([webhooks#automatic-retries](https://docs.stripe.com/webhooks#automatic-retries), [process-undelivered-events](https://docs.stripe.com/webhooks/process-undelivered-events)).
- **Слушать только нужные типы событий.** Stripe: подписка на лишние типы — лишняя нагрузка ([webhooks#only-listen-to-event-types-your-integration-requires](https://docs.stripe.com/webhooks#only-listen-to-event-types-your-integration-requires)).

### Recommended (и одно осознанное расхождение)

- **Индустриальный дефолт — быстрый 2xx + асинхронная очередь.** Stripe: «Your endpoint must quickly return a successful status code (2xx) prior to any complex logic that could cause a timeout» и «Configure your handler to process incoming events with an asynchronous queue» — пики доставок (начало месяца) валят синхронную обработку ([webhooks#handle-events-asynchronously](https://docs.stripe.com/webhooks#handle-events-asynchronously)). **Наше расхождение (ADR 0010/0039)**: синхронная обработка в HTTP-бюджете ~8 с без фоновой очереди — допустимо, потому что (а) повторная доставка — идемпотентный no-op, (б) при сбое отвечаем не-200 и ретраит провайдер. Ревью-чек: не добавлять в вебхук-путь тяжёлых синхронных вызовов, не влезающих в бюджет, и не вводить always-200 с «обработаем потом в памяти» (это худшее из двух миров — ни очереди, ни ретраев).
- **Источник истины — API провайдера, не тело события.** Stripe: объект в событии — снапшот на момент события; при сомнении «retrieve the API resource from the Stripe API to access the latest and up-to-date object definition» ([webhooks#example-endpoint](https://docs.stripe.com/webhooks#example-endpoint)). Практика: при расхождении и для критичных переходов перечитывать статус платежа у провайдера.
- **Освобождать вебхук-роут от CSRF/сессионных middleware.** Stripe: exempt из CSRF-защиты фреймворка ([webhooks#exempt-webhook-route-from-csrf-protection](https://docs.stripe.com/webhooks#exempt-webhook-route-from-csrf-protection)); у нас аналог — не прогонять вебхук через auth-посредников, аутентичность обеспечивает подпись.
- **Неизменяемость событий**: объекты событий не меняются после создания, включая ретраи с новой подписью ([webhooks#api-versioning](https://docs.stripe.com/webhooks#api-versioning)) — внутренний журнал уведомлений тоже append-only.

---

## 8. Сверка с провайдером (reconciliation)

### Must

- **Периодическое подметание пропущенных событий.** Stripe: если эндпоинт лежал, события можно добрать списком `GET /v1/events` с `delivery_success=false` за окно 30 дней и обработать в хронологическом порядке с той же идемпотентной защитой ([process-undelivered-events](https://docs.stripe.com/webhooks/process-undelivered-events)). Вебхуки — «best effort», сверка — гарантия полноты.
- **Сторож зависших промежуточных состояний.** Паттерн Stripe (processing/processed + ретраи) в сочетании с нашим ADR 0037 (`refunding`-резерв вне транзакции, неопределённый исход остаётся под сторожем reconciliation-воркера) — общий принцип: операция с внешним вызовом вне транзакции обязана иметь воркер перепроверки по таймауту.
- **Провайдер — источник истины о статусе денег.** Stripe: при out-of-order и дублях объект дотягивается из API ([webhooks#event-ordering](https://docs.stripe.com/webhooks#event-ordering)); Kill Bill: overdue-состояние перепроверяется таймером `autoReEvaluationInterval` даже без новых платёжных событий ([userguide_subscription](https://docs.killbill.io/latest/userguide_subscription)). Внутренний статус платежа приводится к провайдерному, не наоборот.

### Recommended

- **Регулярная сверка payouts/платежей с отчётами провайдера.** Stripe: payout reconciliation report — матчинг банковских поступлений с батчами операций ([reports/payout-reconciliation](https://docs.stripe.com/reports/payout-reconciliation)); автоматическая сверка платежей и инвойсов ([invoicing/automatic-reconciliation](https://docs.stripe.com/invoicing/automatic-reconciliation)). Для нас — периодический отчёт Т-Кассы против internal payments.
- **Метрики/алерты на аномалии**: события старше N часов без обработки, платежи в промежуточном статусе дольше SLA, расхождения сумм. Основание — те же 3 дня ретраев Stripe и 30-дневное окно листинга событий: у сверки есть дедлайн, после которого данные у провайдера уже недоступны ([webhooks#automatic-retries](https://docs.stripe.com/webhooks#automatic-retries), [process-undelivered-events](https://docs.stripe.com/webhooks/process-undelivered-events)).

---

## 9. Работа с деньгами

### Must

- **Целые минорные единицы во всех слоях.** Stripe: «All API requests expect `amount` values in the currency's minor unit» — 1000 для 10 USD; RUB — обычная двухдесятичная валюта (копейки); ноль-десятичные валюты — отдельный список ([currencies#specify-amounts-in-api-requests](https://docs.stripe.com/currencies)). Наш BIGINT-копейки через все слои (ADR 0008) — прямое соответствие; float для денег не появляется нигде.
- **Детерминированное правило округления при дробных суммах.** Stripe: если после proration/купон/налогов сумма получается дробной в минорных единицах, Stripe округляет до кратного допустимого и относит разницу на баланс клиента («We credit or debit any difference from rounding to the customer balance») ([currencies#special-cases](https://docs.stripe.com/currencies)). Практика: любое округление (сейчас нам почти не нужно, но появится при скидках/промо) — правило зафиксировано, остаток — на баланс/в честную сторону клиента, не «в никуда».
- **Минимальная сумма списания.** Stripe: минимум 0.50 RUB и т.п., «subscription charges support zero-amount charges... for coupons and free trials» ([currencies#minimum-and-maximum-charge-amounts](https://docs.stripe.com/currencies)). У нас суммы фиксированы тарифами — риск минимален, но валидация суммы на порте адаптера уместна.

### Recommended

- **Корректировки — новыми записями, не правкой исходных.** Kill Bill: инвойсы «for the most part, static»; правки — айтемами `ITEM_ADJ`/`REPAIR_ADJ`/`CBA_ADJ` ([userguide_subscription](https://docs.killbill.io/latest/userguide_subscription)). Аналог: сторно/коррекция платежа — отдельная запись, ссылающаяся на исходную.
- **Валюта одна, если продукт не мультивалютный.** Stripe поддерживает 135+ валют и конверсии, но это осознанная функциональность, а не дефолт ([currencies](https://docs.stripe.com/currencies)). Наша позиция «только RUB» (ADR 0036) — нормальная практика для локального продукта; чек-лист: не протекали ли currency-поля в API там, где всегда RUB.

---

## Сводный чек-лист для ревью кода

Уровень: **M** = must (нарушение — дефект ревью), **R** = recommended (нарушение — обсудить).

| # | Тема | Практика | Уровень |
|---|---|---|---|
| 1 | Статусы | «Запланированная отмена» (доступ до конца периода) отделена от терминального конца подписки | M |
| 2 | Статусы | `grace`/past_due — отдельный статус, доступ сохраняется, возврат в `active` при оплате | M |
| 3 | Статусы | Текущий статус — материализованное поле; переходы — иммутабельный журнал (append-only) | M |
| 4 | Статусы | Терминальные переходы запрещены автоматом (из «конца» нет рёбер назад) | M |
| 5 | Статусы | Каждый переход: причина + инициатор (пользователь/админ/система) | M |
| 6 | Статусы | Entitlement (доступ/лимиты) и billing (деньги) связаны, но не слиплись в одну логику | R |
| 7 | Статусы | Мёртвых состояний нет: каждое производимо и наблюдаемо | R |
| 8 | Апгрейд/даунгрейд | Дорогое — сразу (после успешной оплаты), дешёвое — в конце периода | M |
| 9 | Апгрейд/даунгрейд | Активный тариф в каждый момент ровно один; смена не создаёт «двойного биллинга» | M |
| 10 | Апгрейд/даунгрейд | Апгрейд применяется только после подтверждённой оплаты; период сбрасывается явно | M |
| 11 | Апгрейд/даунгрейд | Нет кредита/зачёта за неоплаченное время | M |
| 12 | Апгрейд/даунгрейд | Сумма к списанию показывается до подтверждения | R |
| 13 | Апгрейд/даунгрейд | Кредиты (если появятся) — на баланс, не авторефанд | R |
| 14 | Апгрейд/даунгрейд | Лимиты нового тарифа проверяются в момент вступления, не заказа | R |
| 15 | Отмена | Отмена сохраняет доступ до конца оплаченного периода | M |
| 16 | Отмена | Отмена мгновенно выключает автопродление | M |
| 17 | Отмена | Восстановление — только до конца периода (после — новая оплата/новая подписка) | M |
| 18 | Отмена | События: «отмена запланирована» и «подписка закончилась» — разные записи/события | R |
| 19 | Dunning | Первый отказ не блокирует: окно повторов с расписанием | M |
| 20 | Dunning | Hard declines не ретраятся тем же методом — только после смены способа оплаты | M |
| 21 | Dunning | Окончание grace — явная политика (у нас: переход на базовый тариф + архивация) | M |
| 22 | Dunning | Уведомления: на входе в grace и перед истечением, минимум дважды за окно | M |
| 23 | Dunning | Следующая попытка запланирована явно (время/счётчик попыток в данных) | R |
| 24 | Dunning | Предупреждение об истечении карты заранее — отклонено (#418) | R |
| 25 | CIT/MIT | Первый платёж — CIT в сессии клиента; рекуррентные — MIT по сохранённому методу | M |
| 26 | CIT/MIT | Явное согласие на сохранение метода и автосписания | M |
| 27 | CIT/MIT | Смена активного способа оплаты в grace подхватывается следующей попыткой | M |
| 28 | CIT/MIT | Реквизиты не хранятся у нас — только токен метода от провайдера | M |
| 29 | CIT/MIT | Привязка живёт в сессии с TTL; неактивная сессия не создаёт метод | R |
| 30 | Идемпотентность | Исходящие мутации к провайдеру — с идемпотентным ключом (стабильным при повторе) | M |
| 31 | Идемпотентность | Вебхук-дубликаты дедуплицированы по идентификатору события с персистентной отметкой | M |
| 32 | Идемпотентность | Повторная доставка уже обработанного события — no-op с ответом успеха | M |
| 33 | Идемпотентность | Уникальные ограничения в БД на внешние id платежей/событий — последний рубеж | M |
| 34 | Идемпотентность | Тайм-аут адаптера оставляет операцию «в полёте» под сверку, не запускает дубль | R |
| 35 | Вебхуки | Подпись/подлинность проверяется на каждом запросе (HMAC/токен провайдера) | M |
| 36 | Вебхуки | Порядок событий не предполагается; расхождение решается запросом к провайдеру | M |
| 37 | Вебхуки | 2xx — только после полной обработки; сбой — не-2xx (ретраи делает провайдер) | M |
| 38 | Вебхуки | Обработка вписывается в HTTP-бюджет; тяжёлая работа не в вебхук-пути | M |
| 39 | Вебхуки | Подписаны только нужные типы уведомлений | R |
| 40 | Вебхуки | Вебхук-роут без auth/CSRF-посредников (безопасность — подписью) | R |
| 41 | Сверка | Воркер периодической сверки внутренних состояний с провайдером | M |
| 42 | Сверка | Сторож зависших промежуточных статусов (refunding и т.п.) с перепроверкой по таймауту | M |
| 43 | Сверка | При расхождении внутреннее приводится к провайдерному (провайдер — источник истины) | M |
| 44 | Сверка | Метрики/алерти на зависшие платежи и необработанные события | R |
| 45 | Деньги | Суммы — целые копейки (BIGINT) во всех слоях; float запрещён | M |
| 46 | Деньги | Округления (если есть) детерминированы; остаток — на баланс, не теряется | M |
| 47 | Деньги | Валидация суммы на границе с провайдером (минимум/максимум) | R |
| 48 | Деньги | Корректировки — новыми записями со ссылкой на исходную, исходные не переписываются | R |
| 49 | Деньги | Валюта не протекает в контракт (всегда RUB) | R |

## Источники

- Stripe Billing: [subscriptions overview](https://docs.stripe.com/billing/subscriptions/overview) · [upgrade/downgrade](https://docs.stripe.com/billing/subscriptions/upgrade-downgrade) · [prorations](https://docs.stripe.com/billing/subscriptions/prorations) · [cancel](https://docs.stripe.com/billing/subscriptions/cancel) · [smart retries](https://docs.stripe.com/billing/revenue-recovery/smart-retries) · [customer emails](https://docs.stripe.com/billing/revenue-recovery/customer-emails)
- Stripe инфраструктура: [webhooks](https://docs.stripe.com/webhooks) · [process undelivered events](https://docs.stripe.com/webhooks/process-undelivered-events) · [idempotent requests](https://docs.stripe.com/api/idempotent_requests) · [currencies](https://docs.stripe.com/currencies) · [save during payment](https://docs.stripe.com/payments/save-during-payment) · [cards overview](https://docs.stripe.com/payments/cards/overview) · [payout reconciliation](https://docs.stripe.com/reports/payout-reconciliation) · [automatic reconciliation](https://docs.stripe.com/invoicing/automatic-reconciliation)
- Chargebee: [API subscriptions](https://apidocs.chargebee.com/docs/api/subscriptions) · [dunning v2](https://www.chargebee.com/docs/payments/2.0/dunning/dunning-v2) · [KB: active при неудачном продлении](https://www.chargebee.com/docs/billing/2.0/kb/billing/subscription-status-as-active-when-payment-failed-on-renewal)
- Recurly: [expire/cancel subscription](https://docs.recurly.com/recurly-subscriptions/docs/expire-subscription) · [subscription dashboard](https://docs.recurly.com/recurly-subscriptions/docs/subscription-dashboard) · [dunning campaigns](https://docs.recurly.com/recurly-subscriptions/docs/dunning-management) · [invoice management](https://docs.recurly.com/recurly-subscriptions/docs/invoice-management) · [list subscriptions API](https://docs.recurly.com/recurly-subscriptions/reference/listsubscriptions) · [support: active + past due](https://support.recurly.com/hc/en-us/articles/46649114797204-Why-Is-a-Subscription-Active-but-the-Invoice-is-Past-Due)
- Kill Bill: [subscription guide](https://docs.killbill.io/latest/userguide_subscription) · [entitlement subsystem](https://docs.killbill.io/latest/entitlement_subsystem)
- Lago: [grace period](https://getlago.com/docs/guide/invoicing/invoicing-settings/grace-period) · [subscription object](https://getlago.com/docs/api-reference/subscriptions/subscription-object) · [payment request](https://getlago.com/docs/api-reference/payment-requests/payment-request-object)
