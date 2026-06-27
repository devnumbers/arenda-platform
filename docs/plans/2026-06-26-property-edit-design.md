# Дизайн страницы редактирования объекта

## Контекст

Страница редактирования объекта недвижимости позволяет собственнику изменить основные данные объекта и управлять его фотографиями. Маршрут: `/properties/[id]/edit`.

## Решённые вопросы

- **Формат формы:** одноэкранная форма (не wizard), как в макетах Figma.
- **Удаление фото:** реализуется вместе с endpoint удаления на бэкенде.
- **Тесты:** не пишутся в рамках этой задачи.

## Маршрутизация и страница

- Новый файл: `apps/frontend/app/(cabinet)/properties/[id]/edit/page.tsx`.
- Async server component, ожидает `params: Promise<{ id: string }>`.
- Рендерит клиентский виджет `PropertyEditForm` внутри стандартной обёртки с `max-width: 560px`.
- Title мета: `Редактировать объект — Arenda Platform`.

## UI-компоненты

### PropertyEditForm

Главный клиентский виджет. Отвечает за:

- загрузку объекта через `useProperty(id)`;
- локальный стейт формы (`type`, `address`, `name`, `description`);
- валидацию и сохранение;
- управление фотографиями.

### Секция «Данные»

| Поле | Компонент | Ограничения |
|------|-----------|---------------|
| Тип объекта | Кастомный селект на базе Hero UI `Popover` + `Listbox` | 10 вариантов из `propertyTypeOptions`, required |
| Адрес | `TextField` + `useAddressSuggestions` | required, maxLength 500 |
| Название | `TextField` | required, maxLength 50 |
| Описание | `TextField multiline` | optional, maxLength 500, счётчик |

### Секция «Фотографии»

- Сетка превью 5 колонок × 2 ряда (desktop / tablet / mobile).
- Каждая фото имеет кнопку удаления (trash icon, белая круглая).
- Кнопка «Добавить фото» заменяется на неактивную «Загружено максимум» с подписью `N из 10` при достижении лимита.
- Тап по фото открывает просмотрщик (reuse существующей галереи / lightbox, если есть).
- Поддерживаемые форматы: JPEG, PNG, WebP; max 5 МБ; max 10 файлов.

### Заголовок и кнопки

- Заголовок: `IconLink` со стрелкой назад + `<h1>Информация об объекте</h1>` + прозрачный placeholder для баланса (как в Figma).
- Основная кнопка: `Сохранить изменения`, primary, fullWidth, disabled пока форма невалидна или идёт сохранение.

## Поток данных

1. При загрузке страницы `useProperty(id)` получает объект и инициализирует локальный стейт.
2. Пользователь редактирует поля.
3. По нажатию «Сохранить изменения»:
   - вызывается `useUpdateProperty()` с `{ name, type, address, description }`;
   - параллельно загружаются новые фото через `useUploadPropertyPhoto()`;
   - для удалённых существующих фото вызывается `useDeletePropertyPhoto()` → `DELETE /properties/{id}/photos/{photoId}`.
4. При успехе инвалидируются `propertyKeys.detail(id)` и `propertyKeys.list`, показывается `toast.success('Объект обновлён')`, происходит редирект на `/properties/{id}`.

## Бэкенд

- Новый endpoint: `DELETE /properties/{propertyId}/photos/{photoId}`.
- Удаляет файл из хранилища и запись из массива `photos` объекта.
- Возвращает `204 No Content` или обновлённый `PropertyResponse`.
- Доступ только владельцу объекта.

## Обработка ошибок

- Ошибка загрузки объекта — компонент `PropertyDetailError` с кнопкой «Повторить».
- Ошибки валидации — inline под полем и/или disabled submit.
- Ошибки сохранения / загрузки фото — `toast.error(ApiError.detail ?? 'Ошибка')`.
- Несохранённые изменения: опционально `beforeunload` при уходе со страницы.

## Файлы, которые потребуется создать / изменить

- `apps/frontend/app/(cabinet)/properties/[id]/edit/page.tsx`
- `apps/frontend/widgets/properties/ui/PropertyEditForm.tsx`
- `apps/frontend/widgets/properties/ui/PropertyEditForm.module.css`
- `apps/frontend/widgets/properties/ui/PropertyTypeSelect.tsx` (кастомный селект типа)
- `apps/frontend/features/properties/api/hooks.ts` — добавить `useDeletePropertyPhoto`
- `apps/backend/api/openapi/openapi.yaml` — описать DELETE endpoint
- соответствующие backend handler / usecase / repository

## Исключено

- Автоматизированные тесты (unit / integration / e2e) — по договорённости не пишутся в этой задаче.
