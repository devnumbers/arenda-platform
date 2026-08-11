# Код-модель сущности «Объект» (Property)

Дата: 2026-08-03
Вход для: тикет https://github.com/devnumbers/arenda-platform/issues/123 (типизированные характеристики объекта с пер-типовыми наборами, все необязательные).
Доменная дока: `docs/entities/obekt.md` (обязательные: название, тип, адрес; необязательные: описание, фотографии; 10 типов объекта).

Файл содержит только фактуру о текущем устройстве кода. Рекомендации — вне области этого документа.

## Бэкенд (apps/backend)

### Таблица и миграции

Миграции лежат в `apps/backend/db/migrations/` (golang-migrate, пары `NNNNNN_name.up/down.sql`). Таблица объекта создаётся одной из первых миграций и с тех пор структурно не менялась (кроме дефолта `id`):

- `apps/backend/db/migrations/000002_properties.up.sql:1-19` — таблица `properties`:
  - `id UUID PRIMARY KEY DEFAULT gen_random_uuid()` (дефолт позже снят, см. ниже)
  - `owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE`
  - `name TEXT NOT NULL`
  - `type TEXT NOT NULL CHECK (type IN ('apartment','room','apartments','house','commercial','office','warehouse','garage','parking','land'))`
  - `address TEXT NOT NULL`
  - `description TEXT` — nullable, без дефолта
  - `status TEXT NOT NULL CHECK (status IN ('active','maintenance','archived'))`
  - `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`
  - индексы `idx_properties_owner_status (owner_id, status)`, `idx_properties_owner_updated_at (owner_id, updated_at DESC)`; триггер `trg_properties_updated_at` на `set_updated_at()`
- `apps/backend/db/migrations/000079_uuid_v7_app_side.up.sql:9` — `ALTER TABLE properties ALTER COLUMN id DROP DEFAULT`: идентификаторы UUIDv7 генерируются в приложении (`uuid.NewV7()`), в БД дефолта нет (ADR `docs/adr/0019-uuid-v7-app-generated-ids.md`).

Связанные таблицы:

- `apps/backend/db/migrations/000038_property_photos.up.sql:1-8` — `property_photos`: `id UUID PK`, `property_id UUID NOT NULL REFERENCES properties(id) ON DELETE CASCADE`, `url TEXT NOT NULL`, `created_at`; индекс по `property_id`. Порядка фотографий в схеме нет (обложка = первое фото по доке, сортировка по `created_at` в запросах).
- `apps/backend/db/migrations/000087_property_contacts.up.sql:1-17` — `property_contacts`: `id`, `property_id` (CASCADE), `owner_id`, `name TEXT NOT NULL`, `phone TEXT NOT NULL`, `created_at`, `updated_at`.

Никаких других колонок у `properties` нет; пер-типовых полей в схеме объекта не существует.

### Enum типов объекта

Паттерн: **не Postgres enum**, а `TEXT` + `CHECK`-констрейнт в миграции, продублированный в Go-домене и в OpenAPI. Три места, которые держат список значений синхронно вручную:

1. БД: `000002_properties.up.sql:5` — CHECK по 10 значениям (`apartment`, `room`, `apartments`, `house`, `commercial`, `office`, `warehouse`, `garage`, `parking`, `land`).
2. Go-домен: `apps/backend/internal/properties/domain/property.go:12-52` — `type PropertyType string`, константы `PropertyTypeApartment`…`PropertyTypeLand`, `ParsePropertyType()` (ошибка `ErrInvalidPropertyType`), `Valid()`. Аналогично `PropertyStatus` (`active`/`maintenance`/`archived`) — `property.go:54-80`, `DeletePropertyMode` (`cascade`/`detach`) — `property.go:82-106`, `PropertyOccupancy` (`free`/`occupied`, только в памяти, не хранится) — `property.go:108-113`.
3. OpenAPI: `apps/backend/api/openapi/openapi.yaml:4447-4449` — `PropertyType: enum [apartment, room, apartments, house, commercial, office, warehouse, garage, parking, land]`; `PropertyStatus` — `:4451-4453`.

Тот же паттерн `TEXT + CHECK` используют все остальные enum'ы схемы (`leases.status`, `operations.type` income/expense и т.д., см. `000003_leases.up.sql:24,49,70`).

### Слои (DDD modular monolith)

Контекст `properties` в `apps/backend/internal/properties/`; направление зависимостей transport/adapters → application → domain (правила — `apps/backend/AGENTS.md`, раздел «API And Persistence»).

- **Domain**: `apps/backend/internal/properties/domain/property.go`
  - `Property` — `property.go:115-128`: `ID, OwnerID, Name, Type, Address, Description string, Status, Occupancy, Photos []Photo, CreatedAt, UpdatedAt, OverdueRentCount int`. `Photo` — `property.go:131-134` (`ID`, `URL`).
  - `NewProperty()` — `property.go:144-165`: генерирует UUIDv7, ставит `Status=active`, вызывает `Validate()`.
  - `Validate()` — `property.go:167-190`: валидация ручная, без библиотек — `strings.TrimSpace` + проверки длин: name ≤ 255, address ≤ 500, description ≤ 2000 (ошибки `property.go:136-142`); плюс `Type.Valid()`, `Status.Valid()`.
- **Application**: `apps/backend/internal/properties/application/`
  - `service.go:41-46` — `CreatePropertyCommand{Name, Type, Address, Description string}`; `service.go:48-54` — `UpdatePropertyCommand{Name, Type, Address, Description, Status *string}` (частичное обновление через указатели).
  - `CreateProperty` — `service.go:104`; `UpdateProperty` — `service.go:275-367` (merge указателей в доменную сущность, проверка переходов статусов, `Validate()`, запись, аудит `updatedPropertyFields` — `service.go:911`).
  - Порты — `application/ports.go`: `PropertyRepository` (:49-64), `PropertyPhotoRepository` (:80-89), `PhotoStorage` (S3, :73-77), `OccupancyProvider` (:32-36), `PropertyContactRepository` (:92-99).
  - Лимиты фото: `service.go:29-39` — `maxPhotoCount = 10`, `maxPhotoSize = 5 MiB`, content-types jpeg/png/webp.
- **Adapters (persistence)**: `apps/backend/internal/properties/adapters/postgres/repository.go` — тонкий маппер над sqlc-сгенерированными запросами (`internal/platform/generated/postgres`, конфиг `apps/backend/sqlc.yaml`: pgx/v5, `emit_json_tags`, `emit_interface`, `emit_prepared_queries`).
  - SQL-запросы — `apps/backend/db/queries/properties.sql`: `CreateProperty` (:1-4, INSERT всех 7 колонок), `GetPropertyByIDAndOwner` с подзапросом `overdue_rent_count` (:6-17), `ListActive/ListArchived` (`status IN ('active','maintenance')` / `='archived'`), `UpdateProperty` (:55-59, всегда UPDATE всех колонок), `Archive/Unarchive` (:61-69), админские `ListPropertiesAdmin`/`GetPropertyByIDAdmin` (:75-105).
  - **Хранение необязательного `description`**: `repository.go:48` и `repository.go:146` — `Description: pgtype.Text{String: property.Description, Valid: true}`: пустое описание записывается как `''` (пустая строка), а не `NULL`. Чтение — `pgconv.TextToString` (`repository.go:223`).
- **HTTP**: `apps/backend/internal/platform/httpapi/property_handlers.go` (672 строки; handlers реализуют сгенерированный oapi-codegen chi-интерфейс):
  - `CreateProperty` (POST /properties) — :106-145; `UpdateProperty` (PATCH /properties/{id}) — :227-262; `DeleteProperty` (DELETE, query `mode=cascade|detach`) — :265; `Archive/UnarchiveProperty` — :287/:310; фото `UploadPropertyPhoto` (multipart) — :429, `DeletePropertyPhoto` — :468; контакты CRUD — :484-589; подсказки адресов DaData `GetAddressSuggestions` — :603.
  - Маппинг в ответ: `propertyResponse` — :629-659: `description` включается в JSON **только если непустой** (`if property.Description != ""`, :641-643); `photos` — только если есть. Ошибки → RFC7807 problem+json через `handlePropertyError` (:63-103).

### OpenAPI

- Контракт-first: ручной YAML `apps/backend/api/openapi/openapi.yaml` (5081 строка). Правило из `apps/backend/AGENTS.md:73-82`: сначала правится `openapi.yaml`, затем регенерация:
  - `go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.7.1 -config api/openapi/oapi-codegen.yaml api/openapi/openapi.yaml` → `apps/backend/internal/platform/openapi/generated.gen.go` (chi-server + models + embedded-spec; конфиг `apps/backend/api/openapi/oapi-codegen.yaml`).
  - `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate` → `internal/platform/generated/postgres`.
  - Сгенерированные файлы руками не правятся; DTO остаются на HTTP-границе и маппятся в домен явно.
- Схемы объекта в `openapi.yaml`:
  - `PropertyCreateRequest` — :4052-4066: required `[name, type, address]`; `name maxLength 255`, `address maxLength 500`, `description maxLength 2000` (необязательное).
  - `PropertyUpdateRequest` — :4068-4083: все поля необязательные (включая `status`).
  - `PropertyResponse` — :4085-4121: `id, name, type, address, description?, status, occupancy (enum [free, occupied]), photos?, active_lease (nullable), overdue_rent_count, created_at, updated_at`.
  - `PropertyPhoto` — :4123-4132 (`id`, `url`).
  - `PropertyType` / `PropertyStatus` enum'ы — :4447-4453.

### Статусы, архив, занятость (коротко)

- `status` — хранимая колонка (`active`/`maintenance`/`archived`), CHECK в БД + доменный `PropertyStatus`; ручные переходы валидируются в `UpdateProperty` (`service.go:313-331`, `isUpdatableStatusTransition`), архивация/разархивация — отдельные методы `ArchiveProperty`/`UnarchiveProperty` (`service.go:369+`) с сайд-эффектами биллинга через порт `PropertyBillingLifecycle`.
- `occupancy` (`free`/`occupied`) **не хранится**: вычисляется через `OccupancyProvider` — `apps/backend/internal/properties/adapters/postgres/occupancy.go` (COUNT открытых аренд по объекту).
- Тарифный лимит считает `status IN ('active','maintenance')` — `properties.sql:71-73` (`CountActivePropertiesByOwner`).

## Фронтенд (apps/frontend)

Next.js 16 (App Router) + React 19 + HeroUI v3 + Tailwind 4 + `@tanstack/react-query`. FSD-структура: `app/`, `widgets/`, `features/`, `entities/`, `shared/`. **Библиотек форм нет**: ни react-hook-form, ни zod, ни formik в `package.json` — все формы на голом `useState` + ручные проверки; базовое поле — самописный `shared/ui/text-field/TextField.tsx` (не HeroUI Input, а кастомный компонент с `label/required/error/maxLength/multiline`).

### Синк типов с бэкендом

- Типы API генерируются из контракта: `npm run generate:api` = `openapi-typescript ../backend/api/openapi/openapi.yaml -o shared/api/generated.ts` (скрипт в `apps/frontend/package.json`). Хуки используют `components['schemas']['PropertyCreateRequest']` и т.п. — `apps/frontend/features/properties/api/hooks.ts:19-31`.
- Доменный тип на фронте — ручной: `apps/frontend/entities/property/model/types.ts:5-15` — union `PropertyType` из 10 литералов; `Property` — :22-33. Маппер DTO→модель: `apps/frontend/entities/property/model/mappers.ts`.
- Русские подписи типов — ручной словарь: `apps/frontend/features/properties/lib/property-types.ts:3-20` (`propertyTypeLabels`, `propertyTypeOptions`). Итого список типов на фронте задан дважды вручную (union + labels) плюс генерируемый enum в `shared/api/generated.ts`.

### Форма создания — многошаговый wizard

- Страница: `apps/frontend/app/(cabinet)/properties/new/page.tsx` (server component, обёртка) → `apps/frontend/widgets/properties/ui/PropertyCreateWizard.tsx`.
- 3 шага + экран успеха (`step: 1|2|3|4`):
  1. `PropertyTypeStep.tsx` — выбор типа чипами (`Button` по `propertyTypeOptions`).
  2. `PropertyAddressStep.tsx` — адрес с подсказками DaData (`useAddressSuggestions`, debounce 300 мс, выпадающий список, клавиатурная навигация), `maxLength=500`.
  3. `PropertyInfoStep.tsx` — название (обязательное, без maxLength) + описание (`maxLength=500`); кнопка «Создать объект».
  4. `PropertySuccessStep.tsx` — успех.
- Черновик wizard'а персистится в `sessionStorage` (`property-create-draft`) — `apps/frontend/widgets/properties/lib/use-property-create-draft.ts` (загрузка после гидратации, валидация черновика, очистка на шаге 4).
- Сабмит: `useCreateProperty` (`features/properties/api/hooks.ts:72-88`) → `POST /properties` с `{name, type, address, description?}`; валидация — только «тип выбран», «адрес непустой», «название непустое» (inline-проверки в шагах).

### Форма редактирования — одношаговая

- Страница: `apps/frontend/app/(cabinet)/properties/[id]/edit/page.tsx` → `apps/frontend/widgets/properties/ui/PropertyEditForm.tsx`.
- Один `<form>` на `useState`: `PropertyTypeSelect` (выпадающий список), `AddressField` (тот же DaData-виджет), название, описание. Инициализация из `useProperty(propertyId)` один раз (`hasInitialized` ref, :68-79).
- Валидация ручная: `name/address trim().length > 0`, тип выбран; сабмит только при `hasChanges` (diff с загруженными данными, :87-97); ошибки показываются после `submitAttempted`.
- **Локальные лимиты жёстче бэкенда**: `MAX_NAME_LENGTH = 50`, `MAX_DESCRIPTION_LENGTH = 500` (`PropertyEditForm.tsx:18-19`) против 255/2000 в OpenAPI и домене. В wizard создания описание тоже `maxLength=500`.
- Сабмит: `useUpdateProperty` → `PATCH /properties/{id}` (`hooks.ts:90-107`) — отправляются все четыре поля (name, type, address, description), не только изменённые.

### Карточка / детальная страница

- Страница: `apps/frontend/app/(cabinet)/properties/[id]/page.tsx` → `apps/frontend/widgets/property-detail/ui/PropertyDetailPage.tsx` (409 строк, клиентский компонент).
- Состав секций (по порядку рендера, :315-373): `PropertyGallery`, `PropertyStatusSection`, `PropertyLeaseCard`, `PropertyTenantCard`, два `PropertyOperationsSection` (просроченные/запланированные), `PropertyOperationsActions`, `PropertyOperationsCard` (сводка), `PropertyInfoCard`, `PropertyContactsSection`; плюс модалки (`PropertyBlockedModal`, `PropertyDeleteModal`, `PropertyEndLeaseModal`) и `PropertyActionMenu` (редактировать / на ремонт / в архив / экспорт / удалить).
- `PropertyInfoCard.tsx` — блок «Информация об объекте»: показывает **только `description`**; при его отсутствии — кнопка «Добавить описание» (ссылка на edit). Никаких других характеристик объекта на карточке нет.
- Данные: `useProperty`, `usePropertyLeases`, `usePropertyOperationsSummary`, `useOperations` (react-query, инвалидация через `propertyKeys`).

## Админка (apps/admin)

Vite + react-admin 5 + MUI 7. Ресурс объекта **read-only** (без Create/Edit):

- `apps/admin/src/App.tsx:49` — `<Resource name="properties" list={PropertyList} show={PropertyShow} …/>`.
- `apps/admin/src/properties.tsx`:
  - `PropertyList` (:36-47): колонки `name`, `address`, `type` (SelectField), `status` (ChoiceChipField), `occupancy`, `ownerId` (OwnerLinkField); фильтры — поиск `q` и `status`; сортировка ограничена whitelist'ом бэкенда (`name`, `createdAt`, `updatedAt`, `status`).
  - `PropertyShow` (:49-105): TabbedShowLayout — вкладка «Объект» (ownerId, name, type, status, occupancy, address, description, photos как ImageField, createdAt/updatedAt, id), вкладки «Договоры», «Операции», «Контакты» (ReferenceManyField).
- Подписи типов — ручной словарь `apps/admin/src/fields.tsx:46-57` (`propertyTypeChoices`, 10 значений; отдельно `propertyStatusChoices` :59-63, `occupancyChoices` :73-76).
- Бэкенд-сторона админки: `ListPropertiesAdmin`/`GetPropertyByIDAdmin` в `apps/backend/db/queries/properties.sql:75-105`, схема `AdminPropertyResponse` — `openapi.yaml:3861`.

## Паттерны хранения расширяемых данных

По всей схеме `apps/backend/db/migrations/`:

- **JSONB встречается ровно один раз**: `audit_log.context jsonb NOT NULL DEFAULT '{}'::jsonb` — `000080_audit_log.up.sql:13` (контекст аудит-события, `map[string]any` в Go — см. `service.go:353`). Это единственная JSONB-колонка проекта; доменных данных в JSONB нигде нет.
- **EAV / справочников атрибутов / «мета»-колонок нет**: поиск по `payload|metadata|attrs|settings|extra|specs|features` в миграциях не даёт совпадений. Все 20+ таблиц — плоские, с фиксированным набором колонок.
- **Пер-типовых полей нет ни у одной сущности**: дискриминаторы типа (`properties.type`, `operations.type`, `leases.status` и пр.) — всегда `TEXT + CHECK`, а набор колонок одинаков для всех значений типа (напр., `operations` income/expense делят одни колонки — `000003_leases.up.sql:64-78`).
- Единственный прецедент «связанного списка» у объекта — отдельные таблицы `property_photos` и `property_contacts` (1:N через `property_id`, CASCADE).
- Деньги — `BIGINT` в копейках (`rent_amount_kopecks`, `deposit_amount_kopecks`, `amount_kopecks` — `000003_leases.up.sql:27-28,51,72`).

## Ограничения текущих необязательных полей (образец: description)

- БД: `description TEXT` — nullable, **без NOT NULL и без DEFAULT** (`000002_properties.up.sql:7`). Фактически NULL не используется: репозиторий всегда пишет строку (пустую) — `repository.go:48,146`.
- Домен: поле `Description string` (не указатель), лимит 2000 символов в `Validate()` (`property.go:141,180`).
- API: в запросах `description` необязательное (`PropertyCreateRequest`/`PropertyUpdateRequest`, maxLength 2000); в ответе поле опускается, если пустое (`property_handlers.go:641-643`). Handler'ы нормализуют `nil` → `""` при создании (`property_handlers.go:120-123`).
- Фронт: `description?: string` (`entities/property/model/types.ts:27`), в формах `maxLength=500`; при сабмите пустая строка нормализуется в `undefined` (`PropertyEditForm.tsx:120`).
- Фотографии как второе необязательное поле: отдельная таблица + S3, лимит 10 шт./5 МиБ (`service.go:29-39`).

## Не выяснено / вне scope

- Точный порядок фотографий (обложка): сортировка по `created_at` предполагается из запросов, отдельной колонки порядка нет — детально не проверялось.
- Как именно `AdminPropertyResponse` вычисляет `occupancy` в админском list (по аналогии с owner-side, но запрос/код не разбирался построчно).
- Планы по изменению `docs/entities/obekt.md` — доменная дока описывает только текущие 5 полей; пер-типовые характеристики в ней не упоминаются.
