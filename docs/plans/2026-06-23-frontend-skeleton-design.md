# Дизайн: каркас фронтенда Arenda Platform

## Контекст

- Бэкенд готов, API описан в `apps/backend/api/openapi/openapi.yaml` (OpenAPI 3.0).
- Аутентификация через HttpOnly session cookie.
- Фронтенд уже инициализирован: Next.js 16, React 19, TypeScript, CSS Modules.
- Цель этого этапа — каркас: типы, API-запросы, обработчики и FSD-структура. Базовые UI-компоненты и страницы делаем следующим шагом; дизайн из Figma подключается позже.

## Решения

- Типы данных: автогенерация из OpenAPI.
- Взаимодействие с бэкендом: Next.js API Route Handlers как прокси + TanStack Query на клиенте.
- Стили: CSS Modules, Tailwind не используется.
- Структура: feature-based.
- Объём: полный каркас под все ресурсы бэкенда.

## Архитектура и структура папок

```
apps/frontend/
  app/
    api/[...path]/route.ts          # catch-all прокси на бэкенд
    layout.tsx                      # корневой layout с QueryProvider
  features/
    auth/                           # вход/выход, текущий пользователь
    properties/                     # объекты
    leases/                         # аренды
    operations/                     # операции
    recurring-operations/           # регулярные операции
    reminders/                      # напоминания
    tenant-contacts/                # контакты арендаторов
    billing/                        # тарифы, подписка, способы оплаты
  shared/
    api/                            # сгенерированные типы, API-клиент
    providers/                      # QueryProvider
    ui/                             # базовые UI-компоненты (следующий шаг)
```

Каждая feature содержит:

- `api/hooks.ts` — TanStack Query hooks (queries и mutations).
- `api/keys.ts` — query keys.
- `components/` — placeholder-компоненты фичи (минимальная разметка, готовая к стилям из Figma; следующий шаг).

`shared/` содержит инфраструктуру, не привязанную к конкретной feature: сгенерированные типы, API-клиент, провайдеры и базовые UI-компоненты.

## Генерация типов и API-клиент

1. Добавить `openapi-typescript` как dev-dependency.
2. Скрипт `generate:api` генерирует `shared/api/generated.ts` из `apps/backend/api/openapi/openapi.yaml`.
3. Shared fetch-wrapper `shared/api/client.ts`:
   - базовый URL `/api` (локальный прокси);
   - JSON only;
   - парсит `Problem Details` (RFC 7807) от бэкенда и выбрасывает `ApiError` с `code`, `message`, `requestId`;
   - не требует CORS, потому что запросы идут на тот же origin.

## Прокси-роут

`app/api/[...path]/route.ts`:

- Перехватывает любой путь `/api/*`.
- Пересылает method, body и заголовки (кроме `host`) на `BACKEND_URL`.
- Пробрасывает cookies из входящего запроса в заголовок `Cookie`.
- Возвращает ответ бэкенда вместе с `Set-Cookie`.

## TanStack Query

- Добавить `@tanstack/react-query`.
- `shared/providers/query-provider.tsx` оборачивает приложение.
- Каждая feature экспортирует hooks:
  - `useProperties()`, `useProperty(id)`
  - `useCreateProperty()`, `useUpdateProperty()`, `useArchiveProperty()`, ...
  - аналогично для leases, operations, recurring operations, reminders, tenant contacts, billing, auth.
- Mutation hooks инвалидируют соответствующие query keys.

## UI-компоненты (следующий шаг)

- Только CSS Modules.
- Shared базовые компоненты: `Button`, `Input`, `Label`, `Card`, `Spinner`, `ErrorMessage`.
- Feature placeholder-компоненты: `PropertyCard`, `LeaseList`, `OperationRow`, `RecurringOperationItem`, `ReminderRow`, `TenantContactCard`, `SubscriptionInfo` — минимальная разметка, без логики.
- Компоненты принимают `className` для композиции.

## Обработка ошибок

- API-клиент превращает бэкенд-ошибки в единый `ApiError`.
- Хуки отдают `error`, который компоненты могут отрисовывать через `ErrorMessage`.
- На этапе каркаса — без глобального ErrorBoundary.

## Аутентификация

- Хуки `useSendPhoneCode`, `useVerifyPhoneCode`, `useLogout`.
- `useMe` — проверка текущей сессии.
- Cookie устанавливается бэкендом и проксируется через `/api`.

## Что не входит

- Реальные страницы и роуты в `app/` (кроме layout и прокси).
- Pixel-perfect реализация дизайна из Figma.
- Библиотеки форм и валидации.
- SSR для начальной загрузки данных (используем клиентские hooks).
- Тесты (пока не делаем).
- i18n.

## Зависимости

- `@tanstack/react-query`
- `openapi-typescript` (dev)
- `clsx` (опционально, для композиции классов)
