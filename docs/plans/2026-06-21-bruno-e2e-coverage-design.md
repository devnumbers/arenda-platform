# Дизайн: аудит и покрытие Bruno/E2E

## Цель

1. Убедиться, что ручная коллекция `tools/bruno/arenda-api/` содержит запросы для **всех публичных endpoint'ов** backend.
2. Убедиться, что системные E2E-тесты `tools/e2e/bruno/arenda-api-e2e/system-e2e/` покрывают все пользовательские сценарии, включая webhook платежей.
3. Оба набора запросов должны успешно выполняться против локального backend.
4. Добавить lightweight coverage-шлюз, чтобы новые endpoint'ы не терялись в будущем.

## Контекст

Backend строит роуты из OpenAPI в `apps/backend/internal/platform/openapi/generated.gen.go`. Оттуда можно извлечь канонический список публичных endpoint'ов.

Текущие пробелы:
- **Ручная коллекция** не содержит раздела подписок (`/subscription/*`) и тарифов (`/tariffs`), а также ряда вложенных GET-запросов (`GET /leases/{id}/reminders`, `GET /properties/{id}/operations`, `GET /properties/{id}/recurring-operations`).
- **E2E** не покрывает webhook `POST /webhooks/payment/{provider}` (внутренний `/internal/fake-subscription-payment/{id}/confirm` уже используется в сценарии подписки).

## Подход

Выбран **подход 2: аудит + coverage-шлюз**.

### 1. Дополнение ручной коллекции `tools/bruno/arenda-api/`

Добавить папку `subscription/` и отдельные запросы:
- `GET /subscription`
- `PATCH /subscription/auto-renew`
- `POST /subscription/cancel`
- `POST /subscription/change`
- `GET /subscription/payment-methods`
- `POST /subscription/payment-methods`
- `DELETE /subscription/payment-methods/{id}`
- `POST /subscription/payment-methods/{id}/activate`
- `GET /subscription/payments`
- `GET /tariffs`

Добавить недостающие вложенные запросы в существующие папки:
- `GET /leases/{id}/reminders`
- `GET /properties/{id}/operations`
- `GET /properties/{id}/recurring-operations`

При необходимости добавить `POST /webhooks/payment/{provider}` в отдельную папку `webhooks/`.

### 2. Дополнение E2E `system-e2e/`

Добавить новую папку `85-webhooks/` с позитивным и негативным сценариями:
- Успешная обработка корректной подписи провайдера `fake`.
- Отказ при невалидной подписи/провайдере.

Сценарий использует данные из предшествующего прохождения подписки (либо отдельный setup, добавляющий payment-method и инициирующий платёж).

### 3. Coverage-шлюз `tools/e2e/check-bruno-coverage.sh`

Скрипт:
1. Извлекает роуты из `generated.gen.go` (регуляркой по `r.(Get|Post|Put|Patch|Delete)(options.BaseURL + "...")`).
2. Нормализует пути (`{id}`, `{propertyId}` и т.д. → `{*}`).
3. Сканирует `.bru`-файлы в `tools/bruno/arenda-api/` и `tools/e2e/bruno/arenda-api-e2e/system-e2e/`.
4. Сравнивает метод+путь и выводит:
   - endpoint'ы, отсутствующие в ручной коллекции;
   - endpoint'ы, отсутствующие в E2E;
   - endpoint'ы из allow-list, которые не требуют покрытия (например, `POST /auth/phone/send` в ручной коллекции есть, но `GET /internal/perf/db-pool` — внутренний).
5. Возвращает ненулевой exit code, если есть непокрытые публичные endpoint'ы.

Подключить скрипт к `make backend-lint` или запускать в e2e-раннере перед основным прогоном.

### 4. Проверка работоспособности

- Поднять локальную инфраструктуру: `make local-infra-up`.
- Запустить backend: `make backend-run`.
- Прогнать ручную коллекцию: `bru run tools/bruno/arenda-api --env Local --output junit` (или рекурсивно по папкам).
- Прогнать E2E: `./tools/e2e/run-e2e-with-db-checks.sh`.
- Запустить coverage-шлюз: `./tools/e2e/check-bruno-coverage.sh`.

### 5. Документация

- Обновить `tools/bruno/README.md` и `tools/e2e/README.md`: описать структуру, как добавлять запросы и как читать отчёт coverage-шлюза.
- Добавить запись в `CHANGELOG.md`.

## Критерии успеха

- Coverage-шлюз показывает 0 непокрытых публичных endpoint'ов.
- `bru run` на ручной коллекции проходит без реальных ошибок.
- `./tools/e2e/run-e2e-with-db-checks.sh` завершается с `All checks passed`.
- `make backend-lint` включает coverage-шлюз (или он вызывается явно в CI).
