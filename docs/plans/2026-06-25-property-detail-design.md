# Дизайн: страница «Просмотр объекта»

## Цель

Сверстать и реализовать страницу объекта `app/(cabinet)/properties/[id]/page.tsx` в точном соответствии с макетами Figma (`apps/frontend/app/(cabinet)/properties/current.md`). Страница должна отражать все состояния аренды, платежей и самого объекта, в том числе статусы, которые сейчас не обработаны в UI.

## Текущее состояние

- Модели: `PropertyStatus = active | maintenance | archived`; `Occupancy = free | occupied`; `LeaseStatus = awaiting_start | active | requires_action | completed | archived`.
- В списке объектов (`widgets/properties`) уже есть `getDisplayStatus`, который умеет: `rented`, `free`, `maintenance`, `overdue`, `finished`.
- Не обработаны: `awaiting_start`, `requires_action` (как отдельные состояния страницы), экран «Аренда завершена», «В архиве / тариф закончился», пустые состояния, загрузка, подтверждения и баннеры.

## Отображаемые состояния страницы

| Состояние | Условия | Плашка |
|---|---|---|
| Арендована | `status=active`, открытая аренда `active` | «Арендована» |
| Требует действия | `status=active`, аренда `requires_action` | «Требует действия» + под-плашка просрочки |
| Аренда не началась | `status=active`, аренда `awaiting_start` | «Аренда через X дней/недель» |
| Аренда завершена | `status=active`, последняя аренда `completed` | «Аренда завершена» |
| Не арендована | `status=active`, `occupancy=free`, нет открытой аренды | «Не арендована» |
| На ремонте | `status=maintenance` | «На ремонте» |
| В архиве | `status=archived` | «В архиве» + баннер предупреждения |

Под-плашки просрочки («1 просроченный платёж», «2 просроченных платежа») показываются внутри карточек **Аренда** и **Платежи** независимо от верхнего состояния, если есть overdue-операции.

## Архитектура (FSD)

### Route
- `app/(cabinet)/properties/[id]/page.tsx` — Server Component, экспортирует `Metadata`, оборачивает виджет в `Suspense`.

### Widget-слой
Создаём выделенный FSD-слой `widgets/property-detail`, чтобы не раздувать `widgets/properties` и чтобы секции было легко тестировать и переиспользовать.

- `widgets/property-detail/ui/PropertyDetailPage.tsx` — клиентский корневой виджет, держит состояние модалок/меню, собирает данные через хуки.
- `widgets/property-detail/ui/PropertyDetailHeader.tsx` — шапка: назад, «Мой объект», кнопка `more-vertical`, выпадающее меню действий.
- `widgets/property-detail/ui/PropertyGallery.tsx` — галерея фото объекта.
- `widgets/property-detail/ui/PropertyStatusSection.tsx` — плашка статуса + название/адрес.
- `widgets/property-detail/ui/PropertyLeaseCard.tsx` — секция «Аренда» с суммой, сроком, прогресс-баром, арендатором, месяцем, просрочкой, кнопками.
- `widgets/property-detail/ui/PropertyTenantCard.tsx` — секция «Арендатор».
- `widgets/property-detail/ui/PropertyPaymentsCard.tsx` — секция «Платежи» со списком операций.
- `widgets/property-detail/ui/PropertyOperationsCard.tsx` — карточка P&L: название объекта, прибыль за текущий месяц / всё время.
- `widgets/property-detail/ui/PropertyInfoCard.tsx` — секция «Информация об объекте» с описанием.
- `widgets/property-detail/ui/PropertyActionMenu.tsx` — меню «Редактировать / На ремонт / Завершить аренду / Перевести в архив».
- `widgets/property-detail/ui/PropertyBlockedModal.tsx` — модал «нельзя сменить статус, пока есть незавершённая аренда».
- `widgets/property-detail/ui/PropertyEndLeaseModal.tsx` — подтверждение завершения аренды.
- `widgets/property-detail/ui/PropertyDepositReturnModal.tsx` — возврат залога.
- `widgets/property-detail/ui/PropertyLoading.tsx` / `PropertyError.tsx` — скелетон и ошибка.

### Feature / entity слои
- Переиспользовать существующие хуки: `useProperty`, `useUpdateProperty`, `useArchiveProperty`, `useUnarchiveProperty`, `useCompleteLease`, `useOperationsByProperty`, `useRecurringOperationsByProperty`.
- Добавить новые хуки/методы (см. «Изменения в бэкенде»).

## Изменения в бэкенде

> Согласовано с пользователем: для точного соответствия макетам добавляем недостающие endpoint’ы параллельно с фронтендом.

1. `GET /properties/{id}/leases` — список аренд объекта (открытая + история). Минимум: отдать `LeaseResponse[]` для property_id.
2. `GET /properties/{id}/operations/summary` — агрегаты:
   - `monthly_profit_kopecks` (текущий месяц),
   - `all_time_profit_kopecks`,
   - `overdue_rent_count`,
   - `overdue_total_count`,
   - `next_payment_date?`.
3. `POST /leases/{id}/deposit-return` — возврат залога. Создаёт расходную операцию на сумму залога с категорией `other_expense` и комментарием «Возврат залога», либо вводит отдельный статус/событие (уточнится при реализации, но API контракт остаётся таким).
4. OpenAPI-спецификация (`apps/backend/api/openapi/openapi.yaml`) обновляется до генерации фронтового клиента.

## Данные и их источники

| Что показываем | Endpoint / хук |
|---|---|
| Инфо об объекте | `GET /properties/{id}` → `useProperty` |
| Фото | `PropertyResponse.photos` |
| Статус / действия | `PropertyResponse.status`, `occupancy` + lease |
| Аренда | `GET /properties/{id}/leases` → новый `usePropertyLeases` |
| Арендатор | встроен в `LeaseResponse.tenant_contact` |
| Платежи/операции | `GET /properties/{id}/operations` → `useOperationsByProperty` |
| P&L | `GET /properties/{id}/operations/summary` → новый `usePropertyOperationsSummary` |
| Регулярные операции | `GET /properties/{id}/recurring-operations` → `useRecurringOperationsByProperty` |

## Действия и переходы

### Меню «⋮» в шапке
- **Редактировать объект** — переход на форму редактирования (использовать `useUpdateProperty`).
- **На ремонт** — `PATCH /properties/{id} {status: maintenance}`. Если есть открытая аренда, показать `PropertyBlockedModal`.
- **Завершить аренду** — если есть открытая аренда, показать `PropertyEndLeaseModal`; после подтверждения `POST /leases/{id}/complete`.
- **Перевести в архив** — `POST /properties/{id}/archive`. Если есть открытая аренда, показать `PropertyBlockedModal`.

### Карточка «Аренда»
| Состояние | Кнопки |
|---|---|
| active / requires_action | «Оплатить аренду», «Все операции» |
| completed | «Продлить», «Завершить» |
| awaiting_start | «Начать аренду» (переводит `awaiting_start → active`?) |
| free | «Создать аренду» |
| archived | кнопки отключены / «Оплатить тариф» в баннере |

### Карточка «Платежи»
- «Внести платёж» — создать/отметить операцию выполненной.
- «Запланировать» — создать будущую операцию.
- В пустом состоянии: «Добавить платежи».

## Модальные окна и экраны

1. **Блокировка смены статуса** — центральный диалог, когда пользователь пытается перевести объект на ремонт / в архив / завершить аренду при наличии открытой аренды. Кнопки: «Отменить», «Продолжить» (ведёт к завершению аренды).
2. **Подтверждение завершения аренды** — диалог «Завершить аренду?» + пояснение, что информация останется в разделе «Аренда».
3. **Возврат залога** — диалог с полем суммы (предзаполнено `deposit_amount_kopecks`) и кнопками «Отменить», «Продолжить».
4. **Плашка успешного завершения** — inline-баннер на странице после `complete lease` с кнопкой «Открыть аренду».
5. **Экран завершения аренды** (итоги) — отдельный маршрут или модал: срок аренды, прибыль, доходы, расходы, период, арендатор, кнопки «Назад», «Завершить аренду».

## Скелетон, ошибка и пустые состояния

- **Loading**: `PropertyLoading` со скелетонами всех секций, заголовки скрыты.
- **Error**: `PropertyError` с сообщением и кнопкой «Повторить».
- **Empty / free**: все карточки показывают плейсхолдеры и CTA «Создать аренду», «Добавить арендатора», «Добавить платежи», «Добавить описание».
- **Empty operations**: карточка P&L показывает текст «Операций ещё не было» / «Здесь будет отображаться прибыль».

## Рассмотренные подходы

1. **Монолитный виджет + вычисления на фронте** — один большой клиентский компонент, все агрегаты считает фронт из `/operations` и `/leases`. Быстро, но тяжело тестировать и поддерживать, много запросов.
2. **Модульный FSD-слой + локальные агрегаты** — секции разбиты по виджетам, агрегаты всё ещё на фронте. Лучше структура, но остаётся много клиентской логики.
3. **Модульный FSD-слой + новые backend endpoints** *(рекомендуется)* — секции разбиты, бэкенд отдаёт агрегаты и список аренд объекта. Чистая архитектура, проще тестировать, меньше запросов, соответствует контракт-first подходу.

## Рекомендуемый подход

**Подход 3**: выделенный `widgets/property-detail` + новые backend endpoint’ы + переиспользование существующих entity/feature-хуков.

## Критерии готовности

- [ ] Страница открывается по `/properties/{id}` и корректно отображает все 7 состояний объекта.
- [ ] Все под-плашки, баннеры, пустые и загрузочные состояния соответствуют макетам.
- [ ] Реализованы действия: редактирование, на ремонт, завершение аренды, архивирование, возврат залога.
- [ ] Модальные окна блокировки и подтверждения работают.
- [ ] Backend endpoints добавлены и описаны в OpenAPI.
- [ ] `npm run lint` и `npm run build` в `apps/frontend` проходят без ошибок.
- [ ] Дизайн-документ сохранён в `docs/plans/2026-06-25-property-detail-design.md`.
