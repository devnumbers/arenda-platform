# Дизайн страницы создания операции (платежа)

## Контекст

Страница позволяет собственнику создать доход или расход по объекту недвижимости. Маршрут: `/finance/create-operation`.

В отличие от макетов, в бэкенд-модели операции сейчас нет поля «название» — есть только `comment`. В рамках этой задачи поле `name` добавляется в обе сущности: `Operation` и `RecurringOperation`.

## Решённые вопросы

- **Что создаёт страница?** Одиночную операцию или регулярную операцию с периодичностью ежемесячно / ежегодно, либо одиночную операцию с напоминанием («Напомнить один раз»).
- **Тип операции (доход/расход):** передаётся в URL/контексте с предыдущего экрана, на странице переключателя нет.
- **Выбор объекта:** выпадающий список внутри формы.
- **Поле «Название платежа»:** добавляется на бэкенд как `name`.
- **Периодичность:** показываем только поддерживаемые бэкендом варианты — «Каждый месяц», «Каждый год» и «Напомнить один раз».
- **Шаг 3 (напоминание):** показывается всегда, напоминание опционально.

## Маршрут и страница

- Новый файл: `apps/frontend/app/(cabinet)/finance/create-operation/page.tsx`.
- Async server component, рендерит клиентский виджет `OperationCreateWizard`.
- Title мета: `Создание платежа — Arenda Platform`.

## Формат формы

Пошаговый wizard из 3 шагов (как в макетах Figma):

1. **Основная информация**
2. **Дата и периодичность**
3. **Напоминание**

Каждый шаг имеет кнопки «Назад» и «Отмена», индикатор прогресса `N из 3`.

## Шаг 1: Основная информация

| Поле | Компонент | Обязательное | Ограничения |
|------|-----------|--------------|---------------|
| Сумма платежа | `TextField` с префиксом `₽` | да | целое число копеек, ≥ 0 |
| Название платежа | `TextField` | да | maxLength 50 |
| Категория | кастомный селект на базе Hero UI Popover + Listbox | да | зависит от типа операции |
| Объект | кастомный селект на базе Hero UI Popover + Listbox | да | список активных объектов собственника |
| Комментарий | `TextField multiline` | нет | maxLength 500 |

Категории по типу операции:

- **Доход:** `Арендная плата`, `Прочий доход`
- **Расход:** `ЖКХ`, `Ремонт`, `Налог`, `Прочее`

## Шаг 2: Дата и периодичность

| Вариант | Что создаёт | Бэкенд |
|---------|-------------|--------|
| Напомнить один раз | одиночная операция | `POST /properties/{propertyId}/operations` |
| Каждый месяц | регулярная операция | `POST /properties/{propertyId}/recurring-operations` с `periodicity=monthly` |
| Каждый год | регулярная операция | `POST /properties/{propertyId}/recurring-operations` с `periodicity=yearly` |

Поле **Дата операции / Дата первого повтора** — date picker, обязательное.

Для регулярной операции:
- **День платежа** — число от 1 до 31, обязательное.
- **Дата окончания** — опциональная.

## Шаг 3: Напоминание

- Чекбокс / переключатель «Добавить SMS-напоминание».
- Если включено:
  - для одиночной операции: `POST /properties/{propertyId}/operations/{operationId}/reminders` с `offset_days` (1, 3 или 7).
  - для регулярной операции: `POST /properties/{propertyId}/recurring-operations/{recurringOperationId}/reminders` с `reminder_date`, рассчитанной от ближайшей будущей операции.
- Варианты напоминания: за 1 день, за 3 дня, за 7 дней. По умолчанию — за 1 день.

## Экран успеха

После успешного создания показывается экран успеха с:
- заголовком «Платёж создан»;
- кнопками перехода к списку финансов / к объекту.

## Бэкенд-изменения

### 1. Добавить поле `name` в `Operation`

- Миграция: `ALTER TABLE operations ADD COLUMN name TEXT NOT NULL DEFAULT '';`
- Обновить `domain.Operation` (`apps/backend/internal/leases/domain/operation.go`).
- Обновить sqlc-запросы (`apps/backend/db/queries/operations.sql`).
- Перегенерировать `apps/backend/internal/platform/generated/postgres/operations.sql.go`.
- Обновить `CreateOperationCommand`, `UpdateOperationCommand` и `CreateOperation`/`UpdateOperation` в `apps/backend/internal/leases/application/operation_service.go`.
- Обновить OpenAPI:
  - `OperationCreateRequest` — добавить `name` (required, maxLength 50).
  - `OperationUpdateRequest` — добавить `name`.
  - `OperationResponse` — добавить `name`.
- Обновить `operation_handlers.go` и `operationResponse` mapping.
- Обновить тесты и fakes.

### 2. Добавить поле `name` в `RecurringOperation`

- Миграция: `ALTER TABLE recurring_operations ADD COLUMN name TEXT NOT NULL DEFAULT '';`
- Обновить `domain.RecurringOperation`.
- Обновить sqlc-запросы для recurring operations.
- Перегенерировать generated код.
- Обновить `CreateRecurringOperationCommand`, `UpdateRecurringOperationCommand` и сервис.
- Обновить OpenAPI:
  - `RecurringOperationCreateRequest` — добавить `name`.
  - `RecurringOperationUpdateRequest` — добавить `name`.
  - `RecurringOperationResponse` — добавить `name`.
- Добавить периодичность `yearly` в `RecurringOperationPeriodicity` и OpenAPI enum, либо оставить только `monthly`, если yearly ещё не реализован.
- Обновить recurring operation handlers и тесты.

### 3. Генерация операций из регулярной операции

- Убедиться, что сгенерированные операции наследуют `name` из регулярной операции.

## Фронтенд-архитектура

### Виджеты

- `OperationCreateWizard` — управление шагами, локальным состоянием и сабмитом.
- `OperationBasicInfoStep` — шаг 1.
- `OperationScheduleStep` — шаг 2.
- `OperationReminderStep` — шаг 3.
- `OperationSuccessScreen` — экран успеха.
- `CategorySelect` — кастомный селект категории.
- `PropertySelect` — кастомный селект объекта.
- `FrequencySelect` — выбор периодичности (radio/кнопки).

### Фичи

- `features/operations/api` — хуки TanStack Query:
  - `useCreateOperation`
  - `useCreateRecurringOperation`
  - `useCreateOperationReminder`
  - `useCreateRecurringOperationReminder`
- `features/properties/api` — `useProperties` для списка объектов.

### Сущности

- `entities/operation/model/types.ts` — доменные типы `OperationType`, `OperationCategory`, `OperationFrequency`.
- `entities/property/model/types.ts` — `Property` для селекта.

## Поток данных

1. Страница получает `type` из search params (например, `?type=expense`).
2. `OperationCreateWizard` хранит состояние всех 3 шагов.
3. На шаге 2 по выбору частоты определяется, создавать одиночную или регулярную операцию.
4. По нажатию «Создать»:
   - вызывается нужный `create` hook;
   - если шаг 3 включён, вызывается соответствующий reminder hook.
5. При успехе показывается `OperationSuccessScreen`.

## Обработка ошибок

- Ошибки валидации — inline под полями и disabled submit.
- Ошибки API — `toast.error(ApiError.detail ?? 'Ошибка')`.
- Недоступные категории фильтруются по типу операции.
- Пустой список объектов — сообщение «Сначала добавьте объект».

## Файлы, которые потребуется создать / изменить

### Фронтенд

- `apps/frontend/app/(cabinet)/finance/create-operation/page.tsx`
- `apps/frontend/widgets/operations/ui/OperationCreateWizard.tsx` + `.module.css`
- `apps/frontend/widgets/operations/ui/OperationBasicInfoStep.tsx` + `.module.css`
- `apps/frontend/widgets/operations/ui/OperationScheduleStep.tsx` + `.module.css`
- `apps/frontend/widgets/operations/ui/OperationReminderStep.tsx` + `.module.css`
- `apps/frontend/widgets/operations/ui/OperationSuccessScreen.tsx` + `.module.css`
- `apps/frontend/widgets/operations/ui/CategorySelect.tsx`
- `apps/frontend/widgets/operations/ui/PropertySelect.tsx`
- `apps/frontend/widgets/operations/ui/FrequencySelect.tsx`
- `apps/frontend/widgets/operations/index.ts`
- `apps/frontend/features/operations/api/hooks.ts`
- `apps/frontend/features/operations/api/index.ts`
- `apps/frontend/entities/operation/model/types.ts`
- `apps/frontend/shared/api/generated.ts` (регенерация)

### Бэкенд

- `apps/backend/db/migrations/000040_add_operation_name.up.sql`
- `apps/backend/db/migrations/000040_add_operation_name.down.sql`
- `apps/backend/db/migrations/000041_add_recurring_operation_name.up.sql`
- `apps/backend/db/migrations/000041_add_recurring_operation_name.down.sql`
- `apps/backend/db/queries/operations.sql`
- `apps/backend/db/queries/recurring_operations.sql`
- `apps/backend/internal/leases/domain/operation.go`
- `apps/backend/internal/leases/application/operation_service.go`
- `apps/backend/internal/leases/application/recurring_operation_service.go`
- `apps/backend/internal/platform/httpapi/operation_handlers.go`
- `apps/backend/internal/platform/httpapi/recurring_operation_handlers.go`
- `apps/backend/api/openapi/openapi.yaml`
- `apps/backend/internal/platform/openapi/generated.gen.go` (регенерация)
- `apps/backend/internal/platform/generated/postgres/*.sql.go` (регенерация)
- Соответствующие тесты и fakes.

## Открытые вопросы

1. Поддерживает ли бэкенд сейчас периодичность `yearly` для регулярных операций? В домене только `monthly`, в OpenAPI enum тоже только `monthly`.
2. Какой точный URL-параметр передаёт тип операции: `?type=income|expense` или другой?
3. Нужен ли экран успеха по макету Figma или достаточно редиректа на `/finance`?
