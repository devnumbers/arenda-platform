# Research: payload уведомлений для иконок категорий (тикет #1344)

Дата: 2026-10-12. Research для тикета реализации #1348 «Иконки категорий в уведомлениях». Все факты — из первоисточников (код, SQL, миграции, глоссарии) ветки `dev`.

## Вопрос

В уведомлениях платёжных событий (`payment_due`, `payment_reminder`, `payment_overdue`, `payment_auto_paid`) фронт рисует одну стандартную иконку: категория уведомлений `payments_operations` → BoldKey на синем фоне (`apps/frontend/entities/notification/ui/notification-category-icon.tsx`, `CATEGORY_VISUALS`). Нужно по каждому платежу показывать иконку его категории (41 категория каталога + пользовательские). Выяснить: несёт ли payload категорию сегодня, как донести её с обратной совместимостью, какие ассеты есть на фронте и что выбрать для кругляша.

## Факты (с путями)

**Домен notifications** — `apps/backend/internal/notifications/domain/notification.go`:
- `Payload` (строки 176–192): `Property`/`Actor` (*EntityRef*), `RentalID`, `PaymentID`, `PaymentDate`, `TaskID`, `TaskRuleID`, `MembershipID`, `Tariff`. Все поля опциональны; payload — «навигация и рендер карточек», текст-снимок title/body самодостаточен.
- `EntityRef` (153–160) — снимок момента публикации: `id`, `name`, `address`, `email`, `type` (ключ глифа-плейсхолдера, #1217/#1244), `photo` (same-origin путь, #1275). Прецедент «словарь-снимок в payload» уже отработан.
- Категория уведомления выводится из типа события (`feedCatalog`): все четыре платёжных события → `CategoryPaymentsOperations`. Категории платежа в payload **нет**.

**Издатель** — `apps/backend/internal/notifications/application/payments_publisher.go`:
- `PaymentScanTarget` (34–55): `PaymentID`, `DueDate`, `Title`, `AmountKopecks`, `PropertyID/Name/Address/Type/Photo`, `OwnerID`. Категории нет.
- `publish()` (388–416) кладёт в payload только `Property` (EntityRef с адресом/типом/фото) + `PaymentID` + `PaymentDate`.
- Прецедент «живое правило, не снимок операции»: SQL-комментарий `db/queries/notifications.sql:263-265` — «The rule's title travels (not the operation's snapshot): the notification's link lands on the payment's page».

**SQL ног** — `apps/backend/db/queries/notifications.sql`: 12 запросов с `JOIN payments pay ON pay.id = o.payment_id` (8 списков: Due/Overdue/Reminder/AutoPaid × sweep + scheduled, строки 257–593; 4 перезагрузки GetScheduled*Payment, строки 373–625). Сейчас выбирают `pay.title` и свойства объекта, но **не** `pay.category_slug` / `pay.user_category_id` — а JOIN на payments уже есть, колонки лежат в той же строке.

**Схема БД** — `apps/backend/db/migrations/000115_payments_context.up.sql`: `payments.category_slug TEXT` XOR `payments.user_category_id UUID` под `CHECK (num_nonnulls(...) = 1)` (строки 40–47); индекс `idx_payments_user_category`. Пользовательские категории — таблица `payment_categories` (id + name).

**Хранение ленты** — `apps/backend/internal/notifications/adapters/postgres/notification_repository.go` (строки 80–92, 190–204): payload маршируется `json.Marshal(domain.Payload)` в JSONB-колонку и обратно. JSONB — «schema-on-read»: новые опциональные поля миграций не требуют.

**Чтение API** — `apps/backend/internal/notifications/adapters/http/feed_handlers.go` (`payloadSnapshot`, 202–215): payload уезжает на фронт free-form объектом (`NotificationItem.payload` — опциональный object в openapi-контракте). Новые поля JSON появляются в ответе без изменений контракта.

**SSE** — `apps/backend/internal/notifications/adapters/stream/publisher.go`: кадр `notification.created` (`createdPayload`, 82–92) несёт только `{id, category, contextLabel, title, body, url}` — payload не едет. Комментарий в коде: добавление полей аддитивно к envelope v1, старые клиенты игнорируют.

**Web Push** — `apps/backend/internal/notifications/application/push_payload.go`: `PushPayload{Title, Body, Tag, URL, EventType}`, компактный JSON с потолком 3993 байта (RFC 8030). Воркер `adapters/notificationsjob/worker_push.go` (74–80) собирает его из перечитанной строки ленты. Сервис-воркер фронта `apps/frontend/public/sw.js` (`parsePushPayload`, 218–236) читает только `title/body/tag/url` и рисует **статичную иконку приложения** (`icon: PUSH_ICON_PATH`) — ОС-уведомление векторную иконку категории показать не может без отдельной работы со SW и PNG-ассетами.

**Email** — `adapters/notificationsjob/worker_email.go` (131–139): текстовый шаблон title/body/кнопка — иконок нет, лега не касается.

**Ключевой факт совместимости** — `adapters/notificationsjob/args.go` (шапка пакета): «Jobs carry only the notification id — the worker reloads the committed feed row». Delivery-джобы несут `NotificationID`, boundary-джобы — `(PaymentID, DueDate)`. Контента (payload) в джобах **нет вообще**: он рождается кодом издателя в момент публикации и читается из строки ленты в момент доставки.

**Фронт, иконки категорий платежей**:
- `apps/frontend/features/payment-categories/lib/generated/categories.ts` — генерация из `tools/payment-categories/catalog.json` (v1, **41** категория, включая замыкающую «other»; глоссарий payments «Категория»), на каждую `slug`/`label`/`icon` (имя `bold-*`)/`color`. `userCategoryDefault = { icon: 'bold-other', color: '#ABAB9C' }` — одна иконка на серой подложке для пользовательских.
- `lib/category-style.ts` — `categoryStyle(source, slug?)`: дефолтный слаг из каталога → иконка+цвет; `custom` или неизвестный слаг → `userCategoryDefault`. Готовый резолвер с фолбэком.
- `lib/icon-registry.ts` — `categoryIconComponents`: имена `bold-*` → React-компоненты из бареля `@/shared/assets/icons`; покрытие каталога охраняет тест `icon-registry.test.ts`.
- `ui/category-icon.tsx` — `CategoryIcon`: круг 44, подложка цветом каталога, белый кант `circleIconRing`, белая bold-иконка 24 — ровно стиль карточек платежей (`PaymentRowButton`, DESIGN.md §6, строка 391).
- Договор категории в контракте API — `PaymentCategoryView { source: 'default'|'custom', slug?, id?, label }` (`apps/frontend/entities/payment/model/types.ts:38-44`; бэк `internal/payments/adapters/http/payment_handlers.go:801-819` — «Icon and color are frontend catalog metadata keyed by slug and never travel in the response»).

**Фронт, иконки уведомлений**:
- `entities/notification/ui/notification-category-icon.tsx`: 3D-PNG есть только для 5 категорий уведомлений (`/public/images/notifications/category-{rental,tasks,shared-access,tariff,system}.png`); для `payments_operations` 3D нет — BoldKey на `bg-primary` (макет 2329:149014). Варианты row (круг 44) и page (круг 96). **Приоритет кругляша сегодня: `photoUrl` из `payload.property.photo` перекрывает категорию** — платёжные уведомления с фото объекта уже рисуют фото, не BoldKey.
- Кадр SSE → тост: `features/notifications/ui/notification-toast.tsx:43` рисует иконку только по `frame.category`, payload в кадре не ездит.
- Таблица компонентов DESIGN.md §754: `Category Icon` → `CircleIcon`/`CategoryIcon`; «категория уведомлений — NotificationCategoryIcon (3D-PNG эталоны + запечённая точка)».
- Канон иконок — `apps/frontend/DESIGN.md` §10: «иконки не генерируются и не рисуются сами» — только экспорт из Figma; 3D-набора на 41 категорию платежей не существует и самим рисовать нельзя.
- `docs/research/pwa-icons-canon.md` — про **браузерные/PWA-иконки** (favicon, apple-touch, launcher, maskable). К кругляшу ленты отношения не имеет; релевантен только если когда-нибудь захотим менять иконку ОС-push (там maskable-ограничения).

**Глоссарий payments** (`apps/backend/internal/payments/GLOSSARY.md`, «Категория»): единый нередактируемый каталог 41 категории (БД ссылается слагом) + пользовательские категории уровня аккаунта — «одна дефолтная иконка на серой подложке». Операция наследует категорию снапшотом при материализации; в `operations` лежат `category_label` (NOT NULL) и `category_slug`, но **`user_category_id` в operations нет** — снимок пользовательской категории живёт только лейблом.

## Как устроен payload сейчас

1. Издатель (sweep или boundary-джоба) собирает `Publication` из `PaymentScanTarget` → `domain.Payload{Property, PaymentID, PaymentDate}`.
2. `Publisher` пишет строку ленты каждому получателю: payload сериализуется в JSONB.
3. Три ноги доставки перечитывают строку: email (текст), Web Push (`PushPayload` из title/body/eventType, SW рисует иконку приложения), SSE-кадр «wake-up» (id/category/title/body/url без payload).
4. Фронт: лента и страница читают payload из API (фото объекта перекрывает категорию-иконку); тост — только категорию из SSE-кадра; ОС-push — иконка приложения.

Категории платежа нет ни в одном звене. Ближайшие аналоги-прецеденты: `EntityRef.type` (ключ глифа) и `EntityRef.photo` (снапшот-путь) — тоже опциональные словари-снимки, добавленные после v1 (#1217, #1275) без миграций.

## Схема добавления категории (бекенд)

**Миграции не нужны.** Колонки уже есть (`payments.category_slug` / `payments.user_category_id`), JOIN уже есть, payload — JSONB, категория уведомлений остаётся `payments_operations`, postgres-enum не меняется.

Изменения — целиком внутри контекста notifications (издатель уже читает таблицы payments напрямую, consumer-declared port):

1. SQL: в 12 запросов `notifications.sql` добавить в SELECT `pay.category_slug`, `pay.user_category_id` (выбор — с живого правила `pay.*`, по прецеденту заголовка «the rule's title travels»: ссылка уведомления ведёт на страницу правила; альтернатива — снимок операции `o.category_slug`, но у операции нет user_category_id, см. открытые вопросы).
2. `PaymentScanTarget` (payments_publisher.go): добавить `CategorySlug string` + `UserCategoryID *uuid.UUID` (XOR сохраняется источником — CHECK в БД).
3. `paymentScanTarget()` (`adapters/postgres/payment_scan.go:348`) — прокинуть два параметра во все call sites (12 ног + сгенерённые типы sqlc перегенерировать).
4. `domain.Payload`: аддитивное опциональное поле-снимок, например

   ```go
   PaymentCategory *PaymentCategoryRef `json:"payment_category,omitempty"`
   // type PaymentCategoryRef struct {
   //     Slug           string     `json:"slug,omitempty"`
   //     UserCategoryID *uuid.UUID `json:"user_category_id,omitempty"`
   // }
   ```

   Оба поля omitempty — пустой снимок в JSON не попадает (контракт обещает отсутствие поля, не пустой объект — комментарий `payloadSnapshot`).
5. `payments_publisher.publish()` — заполнить снимок из таргета.
6. Фронт: `NotificationPayload` (`entities/notification/model/types.ts`) дополнить опциональным `paymentCategory`; спека openapi правки не требует (payload — свободный object), типы сгенерировать заново.
7. Тесты: publisher (снимок в payload), repository round-trip (старые строки без поля десериализуются), фронтовый маппинг categoryStyle.

## Совместимость (старые джобы / старая лента)

- **Стоящие в River boundary-джобы** (`notifications:payment_due|overdue|reminder|auto_paid`) несут только `(PaymentID, DueDate)` и в момент пробуждения публикуют **текущим** кодом издателя — после деплоя они сами начнут класть категорию. Никакой миграции джобов.
- **Стоящие delivery-джобы** (`deliver_push`/`deliver_email`) перечитывают строку ленты и не знают о payload сверх title/body — ничего не ломается.
- **Старые строки ленты** в JSONB не содержат `payment_category` — при чтении поле просто отсутствует (omitempty/optional). Фронт обязан деградировать: нет снимка → текущая стандартная иконка (BoldKey на синем). Это тот же механизм, что уже работает для `photo` (#1275) и `type` (#1244).
- **SSE-кадр**: если решим показывать категорию в тосте — добавление поля в `createdPayload` аддитивно к envelope v1 (старые клиенты игнорируют; комментарий уже в коде publisher.go).
- **Web Push**: поле в `PushPayload` тоже аддитивно (SW читает белый список и лишнее отбрасывает), но см. открытый вопрос про ОС-иконку.

## Выбор ассетов (рекомендация)

**Для кругляша уведомления — векторные `categoryIconComponents` на цветной подложке каталога (стиль `CategoryIcon`), не 3D.**

- 3D-ассеты на 41 категорию не существуют, канон DESIGN.md §10 запрещает рисовать/генерировать иконки самим; 3D-набор уведомлений канонизирован именно для категорий уведомлений, а не платежей.
- Векторный вариант переиспользует готовый договор: `categoryStyle(source, slug)` + `categoryIconComponents` (с тестом покрытия) + подложка цветом каталога + белый кант `circleIconRing` — визуально совпадает с карточками платежей, ноль новых ассетов, размеры выводятся из варианта row/page.
- Пользовательские категории: `userCategoryDefault` (bold-other на сером #ABAB9C) — ровно глоссарий платежей; резолв уже реализован в `categoryStyle` (custom и неизвестный слаг → дефолт).
- ОС-уведомления Web Push остаются со статичной иконкой приложения (вектор в ОС не отрисовать; PNG-набор на 41 категорию против канона) — смена только внутриприложенческих поверхностей: лента, страница уведомления, тост.

## Открытые вопросы для #1348

1. **Приоритет кругляша**: сегодня `payload.property.photo` перекрывает иконку категории; для платёжных событий — что главнее, фото объекта или иконка категории платежа? Нужен макет/решение владельца (иначе фронт не знает порядок деградаций).
2. **Источник категории**: живое правило (`pay.category_slug`/`pay.user_category_id` — прецедент заголовка, ссылка ведёт на правило) или снимок операции (`o.category_slug`; пользовательская категория в operations живёт только лейблом — для неё всё равно придётся читать `pay.user_category_id`). Рекомендация — `pay.*`.
3. **SSE-тост**: добавлять ли снимок категории в кадр `notification.created` (аддитивно, просто в реализации), или тост остаётся на стандартной иконке?
4. **Page-вариант**: пропорции иконки на круге 96 (у 3D — 61px, у BoldKey — 52px; bold-иконке категории нужен свой размер/подложка) — согласовать с макетом.
5. **ОС-push**: подтвердить, что иконка ОС-уведомления не меняется в этом тикете (отдельная тема: PNG-ассеты + sw.js + maskable-канон).
6. **Витрина /ui-kit** и тесты маппинга — включить в тикет реализации (упомянуты в #1348).
