# Дизайн: каркас фронтенда Arenda Platform

## Контекст

- Бэкенд готов, API описан в `apps/backend/api/openapi/openapi.yaml` (OpenAPI 3.0).
- Аутентификация через HttpOnly session cookie.
- Фронтенд уже инициализирован: Next.js 16, React 19, TypeScript, CSS Modules.
- Цель этого этапа — только каркас: типы, API-запросы, обработчики и базовые UI-компоненты. Страницы пока не делаем, дизайн из Figma подключается позже.

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
    shared/                         # сгенерированные типы, API-клиент, базовые UI-компоненты, провайдеры
```

Каждая feature содержит:

- `api/hooks.ts` — TanStack Query hooks (queries и mutations).
- `api/keys.ts` — query keys.
- `api/actions.ts` — server actions или функции для прокси (если понадобятся).
- `components/` — placeholder-компоненты фичи (минимальная разметка, готовая к стилям из Figma).
- `types.ts` — только доменные типы, которых нет в сгенерированном OpenAPI.

## Генерация типов и API-клиент

1. Добавить `openapi-typescript` как dev-dependency.
2. Скрипт `generate:api` генерирует `features/shared/api/generated.ts` из `apps/backend/api/openapi/openapi.yaml`.
3. Shared fetch-wrapper `features/shared/api/client.ts`:
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
- `features/shared/providers/query-provider.tsx` оборачивает приложение.
- Каждая feature экспортирует hooks:
  - `useProperties()`, `useProperty(id)`
  - `useCreateProperty()`, `useUpdateProperty()`, `useArchiveProperty()`, ...
  - аналогично для leases, operations, recurring operations, reminders, tenant contacts, billing, auth.
- Mutation hooks инвалидируют соответствующие query keys.

## UI-компоненты

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
