# Дизайн: страница редактирования арендатора

## Контекст

В проекте уже реализованы:
- Страница просмотра арендатора: `apps/frontend/app/(cabinet)/tenants/[id]/page.tsx`
- Виджет детали арендатора: `apps/frontend/widgets/tenant-detail/ui/`
- Страница создания арендатора: `apps/frontend/app/(cabinet)/tenants/new/page.tsx`
- Форма создания: `apps/frontend/widgets/tenants/ui/TenantFormStep.tsx`
- API арендаторов: `apps/frontend/features/tenant-contacts/api/`
- Backend: `PATCH /tenant-contacts/{id}` поддерживает частичное обновление всех полей, включая `email`
- Референс редактирования объекта: `apps/frontend/app/(cabinet)/properties/[id]/edit/page.tsx` + `PropertyEditForm`

## Цель

Сделать полноценную страницу редактирования контакта арендатора:
- Все поля арендатора доступны для редактирования
- Единая логика и стили с остальными страницами
- Использование Hero UI 3 и существующих shared-компонентов
- Продуманные состояния загрузки, ошибок, валидации и успеха

## Редактируемые поля

| Поле | Обязательное | Ограничения |
|------|--------------|---------------|
| Имя | Да | max 255, trim |
| Фамилия | Нет | max 255 |
| Отчество | Нет | max 255 |
| Телефон | Нет | формат `+7 (XXX) XXX-XX-XX` |
| Email | Нет | валидный email |
| Комментарий | Нет | max 500 |

## Выбранный подход

**Подход A: переиспользуемая форма `TenantForm`.**

Существующая `TenantFormStep` превращается в переиспользуемый компонент `TenantForm`, который принимает:
- `initialData?: TenantContactFormData` — начальные значения (для редактирования)
- `submitLabel: string` — текст кнопки
- `isLoading: boolean`
- `error?: string` — ошибка от сервера
- `onSubmit(data): void`
- `onCancel?(): void`

Это обеспечивает единую логику создания и редактирования и исключает дублирование полей и валидации.

## Архитектура

### Страница

```
app/(cabinet)/tenants/[id]/edit/page.tsx
```

Server Component, как `properties/[id]/edit/page.tsx`:
- Метаданные на русском
- Принимает `params: Promise<{ id: string }>`
- Рендерит `<TenantEditForm tenantId={id} />`
- Обертка с `max-width: 560px` и отступами

### Виджеты

```
widgets/tenants/ui/TenantForm.tsx        # общая форма
widgets/tenants/ui/TenantFormStep.tsx    # рефакторится для использования TenantForm
widgets/tenants/ui/TenantEditForm.tsx    # загрузка арендатора + PATCH + редирект
widgets/tenants/ui/TenantEditLoading.tsx # опционально, skeleton
widgets/tenants/ui/TenantEditError.tsx   # опционально, ошибка загрузки
```

`TenantEditForm`:
- Загружает арендатора через `useTenantContact(id)`
- Показывает skeleton на `isPending`
- Показывает ошибку с кнопкой «Повторить» на `isError`
- Инициализирует локальное состояние формы один раз (защита от перезаписи при фоновом refetch через `hasInitialized` ref)
- Валидирует перед отправкой
- Отправляет `PATCH /tenant-contacts/{id}` через `useUpdateTenantContact()`
- При успехе: `toast.success`, `router.push(ROUTES.tenant(id))`
- При ошибке: `toast.error` + показ server error под кнопкой

### Общая форма

`TenantForm` рендерит поля:
1. Имя (required)
2. Фамилия
3. Отчество
4. Телефон
5. Email
6. Комментарий (multiline, max 500)

Использует:
- `TextField` из `shared/ui/text-field`
- `Button` из `shared/ui/button`
- `IconLink` для кнопки «Назад»
- `formatPhoneInput` из `shared/lib/phone`

Валидация:
- Имя не пустое после `trim`
- Email либо пустой, либо валидный (`^[^\s@]+@[^\s@]+\.[^\s@]+$")
- Телефон либо пустой, либо ровно 18 символов
- Комментарий ≤ 500

### Навигация

- В `shared/config/routes.ts` добавить:
  ```ts
  tenantEdit: (id: string) => `/tenants/${id}/edit`,
  ```
- В `TenantDetailHeader` кнопка «Редактировать» ведет на `ROUTES.tenantEdit(id)`

## Состояния, не предусмотренные дизайном

| Состояние | Решение |
|-----------|---------|
| Загрузка данных арендатора | Skeleton-заглушка |
| Ошибка загрузки | Экран с сообщением и кнопкой «Повторить» |
| Контакт не найден (404) | Сообщение «Арендатор не найден» + ссылка назад |
| Невалидный UUID в URL | Стандартное поведение Next.js 404 |
| Ошибка сохранения (400) | Показать серверное сообщение; подсветить поля phone/email |
| Дубликат телефона (409) | Ошибка «Такой телефон уже добавлен» + подсветка телефона |
| Потеря соединения | Стандартная ошибка `ApiError` + retry |
| Фоновый refetch перезаписывает введенные данные | `hasInitialized` ref |
| Уход со страницы с несохраненными изменениями | Не реализовывать `beforeunload` (в проекте нет паттерна) |
| Пустые optional-поля при PATCH | Отправлять только измененные поля; очистка → `null` |

## Backend

Backend-изменения не требуются. `PATCH /tenant-contacts/{id}` уже поддерживает все поля.

## Тестирование

- Ручное: открыть `/tenants/[id]/edit`, изменить каждое поле, сохранить, проверить редирект и обновление детали
- Ручное: проверить валидацию имени, email, телефона
- Ручное: проверить 409 на дубликат телефона
- `npm run lint` (frontend)

## Изменяемые файлы

### Frontend
- `apps/frontend/shared/config/routes.ts`
- `apps/frontend/widgets/tenant-detail/ui/TenantDetailHeader.tsx`
- `apps/frontend/widgets/tenants/ui/TenantForm.tsx` — новый
- `apps/frontend/widgets/tenants/ui/TenantFormStep.tsx` — рефакторинг
- `apps/frontend/widgets/tenants/ui/TenantEditForm.tsx` — новый
- `apps/frontend/app/(cabinet)/tenants/[id]/edit/page.tsx` — новый
- `apps/frontend/app/(cabinet)/tenants/[id]/edit/page.module.css` — новый
- `apps/frontend/widgets/tenants/ui/index.ts`

### Backend
- Изменений не требуется.
