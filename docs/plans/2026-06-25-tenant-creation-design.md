# Дизайн страницы создания арендатора

## Цель

Создать страницу `/tenants/new` для добавления нового арендатора (tenant contact) в личном кабинете собственника. Страница должна повторять поведение и визуальный стиль страниц создания объекта (`/properties/new`) и аренды (`/leases/new`).

## Контекст

- В MVP арендатор — это необязательный контакт собственника, не пользователь сервиса (`CONTEXT.md`).
- Бэкенд уже предоставляет endpoint `POST /tenant-contacts` и схему `TenantContactCreateRequest`:
  - `name: string`
  - `surname?: string`
  - `patronymic?: string`
  - `phone?: string`
  - `email?: string`
  - `comment?: string`
- Существующие визарды объектов и аренд используют `sessionStorage` для черновика, ручную валидацию и TanStack Query для API.
- UI-kit проекта: собственные компоненты `shared/ui/*`, построенные на основе Hero UI v3.

## Принятые решения

| Вопрос | Решение |
|--------|---------|
| Дополнительные контакты («Добавить контакт» на макете) | Не реализовывать в MVP — оставить одну форму с основным телефоном. |
| Email | Не показывать в основной форме (согласно макету). API-поле `email` не используем на странице. |
| Черновик формы | Сохранять в `sessionStorage`, как у property/lease. |
| Архитектурный подход | Строго по примеру `PropertyCreateWizard` / `LeaseCreateWizard`: виджет-визард, шаг формы, экран успеха, хук черновика. |
| Экран успеха | «Добавить позже» → `/tenants`; «Добавить платежи» → `/finance`. |
| Тесты | Не писать в рамках этой задачи. |

## Архитектура и файловая структура

```
apps/frontend/
├── app/(cabinet)/tenants/new/
│   ├── page.tsx              # метаданные + обёртка в центрированный контейнер
│   └── page.module.css       # max-width: 560px, padding как у properties/leases
└── widgets/tenants/
    ├── ui/
    │   ├── TenantCreateWizard.tsx      # управление step + draft
    │   ├── TenantCreateHeader.tsx      # иконка закрытия + заголовок
    │   ├── TenantFormStep.tsx          # поля формы + валидация
    │   ├── TenantSuccessStep.tsx       # экран успеха
    │   └── *.module.css
    └── lib/
        └── use-tenant-create-draft.ts  # sessionStorage + тип Draft
```

Используем существующие:
- `features/tenant-contacts/api/hooks.ts` → `useCreateTenantContact`
- `shared/ui/text-field/TextField.tsx`
- `shared/ui/button/Button.tsx`
- `shared/ui/icon-button/IconButton.tsx`
- `shared/config/routes.ts` → `ROUTES.tenants`, `ROUTES.finance`

## Компоненты и UI

### `TenantCreateHeader`
- Кнопка «Закрыть» (`IconButton` с иконкой Cancel) слева.
- Заголовок «Добавление арендатора» по центру.
  - Desktop: `Heading/H1` (28/36).
  - Mobile: `Heading/H2 Medium` (20/22).
- Клик по закрытию → `router.push(ROUTES.tenants)`.

### `TenantFormStep`
- Поля в столбик с gap 24px:
  - **Имя**\* — обязательное, `TextField`.
  - **Фамилия** — `TextField`.
  - **Отчество** — `TextField`.
  - **Комментарий** — многострочное поле с счётчиком `0/500`.
  - **Телефон** — `TextField`.
- Кнопка «Добавить арендатора» — primary, large.
  - Disabled, пока имя пустое или идёт запрос.
- На мобильном кнопка прилипает к низу (sticky footer); на десктопе — в конце формы.

### `TenantSuccessStep`
- Центральная карточка с иконкой профиля 96×96.
- Заголовок «Арендатор добавлен».
- Две кнопки:
  - «Добавить позже» (secondary) → `/tenants`.
  - «Добавить платежи» (primary) → `/finance`.

## Поток данных и валидация

### Тип черновика

```ts
type TenantCreateDraft = {
  step: 'form' | 'success';
  name: string;
  surname: string;
  patronymic: string;
  phone: string;
  comment: string;
};
```

- Храним в `sessionStorage` по ключу `tenant-create-draft`.
- При первом входе инициализируем пустым черновиком (`step: 'form'`).
- При успешном создании `step = 'success'`.
- При закрытии формы или успехе черновик очищается.

### Валидация

Ручная, как в `PropertyCreateWizard` / `LeaseCreateWizard`:
- **Имя** — обязательно после `trim`. Если пустое, кнопка отправки disabled.
- **Комментарий** — не более 500 символов; счётчик в реальном времени.
- **Телефон** — опционально. Если заполнен, базовая проверка формата (минимум 10 цифр, допустимы `+`, `(`, `)`, `-`, пробелы).

### Отправка

1. По клику «Добавить арендатора» вызываем `useCreateTenantContact`.
2. Формируем `TenantContactCreateRequest` из полей черновика:
   - `name` — обязательное.
   - `surname`, `patronymic`, `phone`, `comment` — опционально.
   - `email` не передаём (не запрашиваем в форме).
3. При `onSuccess`:
   - инвалидируем список арендаторов (`tenantContactsKeys.list`);
   - устанавливаем `draft.step = 'success'`.
4. При `onError` — показываем текст ошибки под формой.

## Обработка ошибок и краевые случаи

- **Ошибка API** — понятный текст под кнопкой отправки.
- **Сетевая ошибка** — та же плашка + кнопка остаётся активной для повторной отправки.
- **Обновление страницы** — черновик восстанавливается из `sessionStorage`.
- **Закрытие формы** — переход на `/tenants`, черновик очищается.
- **Refresh на экране успеха** — остаёмся на success, так как `step` тоже хранится в черновике.
- **Длинный комментарий** — ввод блокируется свыше 500 символов.

## Что не входит в задачу

- Дополнительные контактные блоки («Добавить контакт» на макете).
- Поле email в основной форме.
- Рефакторинг общего каркаса визарда в `shared`.
- Автоматические тесты.

## Следующий шаг

После утверждения этого дизайна — создать детальный план реализации (например, через навык `writing-plans`) и приступить к разработке.
