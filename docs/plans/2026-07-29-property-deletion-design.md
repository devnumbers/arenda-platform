# Удаление объекта недвижимости

## Контекст
Необходимо дать пользователю возможность удалять объект недвижимости (`Property`) в бекенде Arenda Platform. Удаление возможно только при отсутствии открытой аренды.

## Целевое поведение
- `DELETE /properties/{id}?mode=cascade` — удалить объект и все связанные данные:
  - аренды (`leases`),
  - операции (`operations`),
  - регулярные операции (`recurring_operations`),
  - фотографии объекта (`property_photos` + S3-файлы),
  - напоминания (`reminders`).
- `DELETE /properties/{id}?mode=detach` — удалить только объект. Связанные аренды, операции, регулярные операции и напоминания сохраняются с `property_id = NULL`. Фотографии удаляются, потому что не имеют смысла без объекта.
- Ограничение: если у объекта есть открытая аренда (`awaiting_start`, `active`, `requires_action`), удаление запрещено с ошибкой `409 Conflict`.

## Выбранный подход
Hard-delete с режимом через query-параметр и изменением FK-правил в БД.

### Почему именно он
- Прямо соответствует заявленным требованиям.
- Перевод FK на `ON DELETE SET NULL` для бизнес-таблиц делает режим `detach` простым и безопасным: удаление объекта автоматически отвязывает дочерние записи.
- Для режима `cascade` сервис явно удаляет дочерние записи в правильном порядке, что контролируемо и понятно.
- `property_photos` остаётся на `ON DELETE CASCADE`, потому что фото не имеют ценности без объекта.

Альтернатива — soft-delete объекта (`properties.deleted_at`) — отклонена, потому что требует глобальной доработки всех выборок по объектам и отдельного решения для видимости "отвязанных" аренд.

## Архитектура

### API-контракт
```http
DELETE /properties/{id}?mode=cascade
DELETE /properties/{id}?mode=detach
```

- `mode` — обязательный query-параметр.
- Отсутствие или невалидное значение `mode` → `400 Bad Request`.
- У объекта открытая аренда → `409 Conflict`, тело `"У объекта есть открытая аренда"`.
- Объект не найден или не принадлежит пользователю → `404 Not Found`.
- Успех → `204 No Content`.

### Миграции
Миграция `000086_property_deletion_detach`:
1. Меняет FK-правило на `ON DELETE SET NULL` для:
   - `leases.property_id`
   - `operations.property_id`
   - `recurring_operations.property_id`
   - `reminders.property_id`
2. Делает `property_id` nullable в `leases`, `operations`, `recurring_operations` (`reminders.property_id` уже nullable).
3. `property_photos.property_id` остаётся `ON DELETE CASCADE`.

### Domain / Application
1. Добавить константу аудита `ActionPropertyDeleted`.
2. Добавить enum `DeletePropertyMode` со значениями `cascade` и `detach`.
3. Расширить `PropertyRepository`:
   - `Delete(ctx, id, ownerID uuid.UUID) error`
   - `DeleteOperationsByProperty(ctx, ownerID, propertyID uuid.UUID) error`
   - `DeleteRecurringOperationsByProperty(ctx, ownerID, propertyID uuid.UUID) error`
   - `DeleteLeasesByProperty(ctx, ownerID, propertyID uuid.UUID) error`
4. В `PropertyService` добавить метод:
   ```go
   DeleteProperty(ctx, ownerID, id uuid.UUID, mode DeletePropertyMode) error
   ```
   Порядок действий:
   1. Открыть транзакцию.
   2. Заблокировать объект через `GetByIDAndOwnerForUpdate`.
   3. Проверить `OccupancyProvider.IsOccupied`; если объект занят — вернуть `ErrPropertyHasOpenLease`.
   4. Получить список фото объекта.
   5. Если `mode == cascade`:
      - удалить `operations` (сначала, потому что ссылаются на `recurring_operations` и `leases`),
      - удалить `recurring_operations`,
      - удалить `leases`.
      `reminders` удалятся каскадно через `operation_id`/`recurring_operation_id`/`lease_id`.
   6. Удалить `properties` (каскадно удалит `property_photos`; для `detach` mode FK `SET NULL` автоматически обнулит `property_id` в `leases`, `operations`, `recurring_operations`, `reminders`).
   7. Записать аудит.
   8. Закоммитить транзакцию.
   9. Best-effort удаление файлов фото из S3 после коммита.

### HTTP handler
Добавить метод `DeleteProperty` в `PropertyHandlers`:
- извлечь `mode` из query,
- вызвать сервис,
- ошибки обработать через `handlePropertyError`,
- при успехе вернуть `204 No Content`.

### OpenAPI
Добавить операцию `DELETE /properties/{id}` с обязательным query-параметром `mode` (enum: `cascade`, `detach`) и перегенерировать `generated.gen.go`.

### Bruno
Добавить запросы:
- `tools/bruno/arenda-api/properties/delete property cascade.bru`
- `tools/bruno/arenda-api/properties/delete property detach.bru`
- Тест-кейс на блокировку при открытой аренде.

### Тесты
Проверка через Bruno и прямые запросы к БД:
- `cascade` полностью удаляет объект и связанные данные.
- `detach` сохраняет аренды/операции с `property_id = NULL` и удаляет фото.
- Открытая аренда блокирует оба режима.
- Несуществующий объект возвращает `404`.

## Ограничения и риски
- `detach` делает аренды "свободно плавающими": они больше не участвуют в occupancy-индексе и не видны в контексте объекта. Это ожидаемое поведение.
- Hard-delete безвозвратен. Если в будущем понадобится восстановление, стоит рассмотреть soft-delete отдельно.
- Удаление фото из S3 выполняется best-effort после коммита; ошибка cleanup не откатывает удаление объекта (как в существующем `DeletePropertyPhoto`).
- Down-миграция требует, чтобы перед откатом не было строк с `property_id IS NULL`; в противном случае `SET NOT NULL` упадёт. Это документировано в комментарии down-миграции.

## Пост-ревью доработки (2026-07-30)

По итогам ревью реализации приняты и внесены дополнительные решения:

- **Nullable `property_id` — полный end-to-end.** Состояние «без объекта» легально во всех слоях: БД → домен → API → админка → фронтенд. В Go-домене действует Nil-конвенция: `uuid.Nil` означает «не привязано» (хелпер `PropertyIDPtr`, по образцу соседних `LeaseIDPtr`/`RecurringOperationIDPtr`); на границе API Nil отображается в nil-указатель, а не в строку нулевого UUID. В OpenAPI `property_id` стал nullable и убран из `required` в `LeaseResponse`, `OperationResponse`, `RecurringOperationResponse`, `FinanceReportPropertyRow`, `AdminLease` и `AdminOperation`; create-запросы по-прежнему требуют объект.
- **При detach — пауза recurring через `billingLifecycle.Suspend`.** Перед удалением строки объекта в режиме `detach` сервис вызывает существующий `Suspend` в той же транзакции: регулярные операции приостанавливаются, будущие неотредактированные операции удаляются, напоминания отменяются — семантика полностью совпадает с архивированием.
- **Гонка CreateLease vs DeleteProperty закрыта row-lock'ом.** `CreateLease` блокирует строку объекта (`GetPropertyByIDAndOwnerForUpdate`) в своей транзакции и повторно проверяет, что объект существует и не в архиве, до создания аренды. Пред-проверка существования объекта оставлена как fast-path.
- **Финансовый отчёт показывает NULL-группу.** Операции без объекта не фильтруются, а выводятся отдельной строкой «Без объекта» (`property_id: null`) — иначе разбивка `ByProperty` расходилась бы с `Totals`.

Итоговые решения зафиксированы в ADR 0025 (`docs/adr/0025-property-deletion-modes.md`).
