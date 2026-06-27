# Дизайн: раздел «Финансы / Операции»

## Решения, принятые после исследования

| Вопрос | Решение |
|--------|---------|
| Масштаб раздела | Глобальный `/finance` **и** объектные страницы (`/properties/:id/operations`). |
| Термин в UI | **Операция** — любая денежная запись (доход/расход). «Платёж» оставляем для арендного платежа / напоминания. |
| Статусы операций | `pending` / `overdue` / `paid` / `received`. Отметить выполненной можно **только на детальной странице/шторке**. |
| Регулярные операции | Отдельная вкладка в `/finance/operations`. Редактирование с выбором «только эта» / «вся серия вперёд». |
| Депозит | Возврат депозита остаётся только в карточке аренды (`POST /leases/:id/deposit-return`). |
| P&L / Прибыль | Новый бэкенд-эндпоинт отчёта. |

## Что уже реализовано

- **Бэкенд**: домен `Operation`/`RecurringOperation`, CRUD + `complete`, summary, напоминания, арендные операции, статусы (ADR 0012).
- **Фронтенд**: `/finance` (заглушка), `/finance/create-operation` (3-шаговый визард), React Query hooks, entity/types, dashboard-агрегатор.
- **Документы**: `apps/frontend/app/(cabinet)/finance/file.md` с ~10 экранами.

## Страницы и маршруты

| Экран из `file.md` | Маршрут | Назначение |
|--------------------|---------|------------|
| Финансы | `/finance` | Дашборд: сводка, период, быстрые действия, последние операции. |
| Финансы / Платежи | `/finance/payments` | Запланированные/просроченные операции (`pending`/`overdue`). |
| Финансы / Платежи объекта | `/properties/:id/operations?status=pending,overdue` | Фильтр «платежи» внутри объекта. |
| Финансы / Операции | `/finance/operations` | Список всех операций. |
| Финансы / Операции / Доходы | `/finance/operations?type=income` | Фильтр по доходам. |
| Финансы / Операции / Расходы | `/finance/operations?type=expense` | Фильтр по расходам. |
| Финансы / Операции / Прибыль | `/finance/operations?tab=profit` | Отчёт P&L. |
| Финансы / Добавление платежа | `/finance/create-operation` | Существующий визард; название в UI заменяем на «Добавить операцию». |
| — | `/finance/operations/:id` | Детальная страница операции. |
| — | `/finance/operations/:id/edit` | Редактирование операции. |
| Финансы / Объекты / Платежи объекта | `/properties/:id/operations` | Финансы объекта: summary + список. |

## Фронтенд-компоненты

### Виджеты (`widgets/`)

- `widgets/finance/ui/FinanceDashboard.tsx` — страница-дашборд.
- `widgets/finance/ui/FinancePeriodSelect.tsx` — переключатель периода.
- `widgets/finance/ui/FinanceSummaryCards.tsx` — карточки доход/расход/прибыль/ожидается/просрочено.
- `widgets/operations/ui/OperationsList.tsx` — список операций с группировкой по месяцам.
- `widgets/operations/ui/OperationListItem.tsx` — строка операции.
- `widgets/operations/ui/OperationFilters.tsx` — фильтры (период, объект, категория, статус).
- `widgets/operations/ui/OperationDetailPage.tsx` — детальная страница.
- `widgets/operations/ui/OperationEditForm.tsx` — форма редактирования.
- `widgets/operations/ui/RecurringOperationsTab.tsx` — вкладка регулярных операций.
- `widgets/operations/ui/ProfitReport.tsx` — P&L-отчёт.

### Сущности (`entities/`)

- `entities/operation/lib/statuses.ts` — метки и цвета статусов.
- `entities/operation/lib/categories.ts` — доходные/расходные категории.
- `entities/operation/lib/formatMoney.ts` — форматирование копеек → ₽.

### Фичи (`features/`)

- `features/operations/api/hooks.ts` — добавить `useOperations`, `useCompleteOperation`.
- `features/finance/api/hooks.ts` — `useFinanceReport`.
- `features/recurring-operations/api/hooks.ts` — добавить глобальный `useRecurringOperations`, если появится endpoint.

### Общие (`shared/`)

- `shared/config/routes.ts` — добавить `financeOperations`, `financePayments`, `financeCreateOperation`, `operation`, `operationEdit`, `propertyOperations`.

## Бэкенд-изменения

### Новые endpoint'ы

1. `GET /operations`
   - Список операций текущего пользователя.
   - Query: `type`, `status` (можно несколько), `category`, `property_id`, `from`, `to`, `recurring_operation_id`, `limit`, `offset`.
   - Ответ: `OperationsResponse` (тот же, что у `GET /properties/:id/operations`).

2. `GET /finance/report` (или `GET /operations/report`)
   - P&L за период.
   - Query: `from`, `to`, `group_by=property|category|month`.
   - Ответ:
     ```json
     {
       "period": { "from": "...", "to": "..." },
       "totals": { "income_kopecks": 0, "expense_kopecks": 0, "profit_kopecks": 0 },
       "by_property": [...],
       "by_category": [...],
       "by_month": [...]
     }
     ```

3. `GET /recurring-operations`
   - Глобальный список регулярных операций пользователя (для вкладки «Регулярные»).
   - Query: `property_id`, `status` (active/paused).
   - Ответ: `RecurringOperationsResponse`.

### Расширения существующих endpoint'ов

- `GET /properties/:id/operations` — добавить фильтры `from`, `to`, `category`, `status`.
- `GET /properties/:id/operations/summary` — оставить как есть; использовать на странице объекта.
- `POST /operations/:id/complete` — уже существует; добавить хук на фронтенде.

## Потоки данных

1. **Дашборд**
   - `useOperations({ from, to })` + клиентская агрегация для сводных карточек.
   - Последние операции — первые N элементов из того же ответа.
2. **Список операций**
   - `useOperations` с query-параметрами фильтров.
   - URL-фильтры синхронизируются с query string (как на странице объектов).
3. **Вкладка «Прибыль»**
   - `useFinanceReport({ from, to })`.
4. **Вкладка «Регулярные»**
   - `useRecurringOperations({ propertyId? })`.
5. **Страница объекта**
   - `usePropertyOperationsSummary(id)` + `useOperationsByProperty(id, { from, to, ... })`.
6. **Детальная операции**
   - `useOperation(id)` + `useCompleteOperation` + `useDeleteOperation`.

## Состояния, не покрытые дизайном

| Состояние | Решение |
|-----------|---------|
| Нет операций вообще | `EmptyState` с CTA «Добавить операцию». |
| Нет операций по фильтру | Сообщение «Ничего не найдено» + кнопка сброса фильтров. |
| Нет объектов | Заблокировать создание операции, показать CTA «Добавить объект». |
| Ошибка загрузки | Страница/блок ошибки с кнопкой «Повторить». |
| Загрузка | Skeleton для карточек и списка. |
| Readonly подписка | Баннер + скрыть/заблокировать кнопки создания/редактирования/удаления. |
| Операция от аренды | В деталке показать источник; при редактировании — предупреждение, что запись станет исключением. |
| Редактирование регулярной операции | Модал «Изменить только эту запись» / «Изменить все будущие». |
| Удаление операции | Подтверждающая модалка; для серии — пояснение, что будущие запланированные записи удалятся. |
| Мобильная версия | Bottom nav уже содержит «Финансы»; списки адаптируются, фильтры уходят в drawer. |

## Согласование терминологии

- Переименовать пользовательские подписи в `/finance/create-operation` и `file.md` с «платёж» на «операция».
- В дашборде «Ближайшие платежи» оставляем, так как это напоминания о запланированных арендных платежах.
- В коде entity/endpoint остаётся `operation`.

## Что остаётся за рамками MVP

- Частичные платежи и зачёт переплаты.
- История изменений операции (audit log).
- Экспорт отчётов.
- Собственные категории пользователя.
- Интеграция с реальными платёжными системами для арендной оплаты.

## Следующий шаг

Передать дизайн в навык `writing-plans` для разбиения на задачи и оценки объёма работ.
