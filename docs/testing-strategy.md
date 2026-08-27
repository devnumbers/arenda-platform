# Тестовая стратегия рефакторинга бекенда и монорепы

Сквозной тестовый контракт для всех тикетов рефакторинга бекенда и монорепы (Фазы 1–3). Источник: parent-issue [#190](https://github.com/devnumbers/arenda-platform/issues/190) (сессия `/improve-codebase-architecture` → `/grill-with-docs` → `/to-issues`, 2026-08-11).

Это **не** отдельный кодовый тикет, а контракт: какие виды тестов обязательны для каждого refactor-тикета, чтобы покрытие **росло**, а не деградировало в ходе рефакторинга.

## Принцип

«Максимально правильные и разнообразные тесты». Рефакторинг без тестов = регрессия в проде. Контекст: T13 ([#171](https://github.com/devnumbers/arenda-platform/issues/171)) уже показал, что класс багов «забытая post-construction инъекция» ловился только живым пользователем на стейдже — это недопустимый baseline.

## Тестовый контракт

Каждый тикет Фаз 1–3 обязан привнести или сохранить **минимум** следующие слои тестов, где применимо.

### 1. Unit-тесты (fake-repository, in-memory)

Чистая логика агрегатов / VO / функций — без БД, без сети.

- Pattern: hand-written fakes (текущая конвенция кодбейза).
- Эталон: `apps/backend/internal/properties/application/service_test.go`.
- Каждый новый метод агрегата или VO — ≥1 table-driven тест (happy + error paths + edge cases).

### 2. Integration-тесты (real Postgres, sqlc-backed adapter)

Реальный `pgxpool` поверх test-БД.

- Гейт по `TEST_DATABASE_URL`; пропуск через `t.Skip`, если не задана (существующая конвенция).
- Эталон: `apps/backend/internal/notifications/adapters/postgres/reminder_policy_integration_test.go`.
- Mapper-функции (`propertyFromRow`, `subscriptionFromRow` и т.п.) покрыты round-trip integration-тестом через БД.

### 3. Composition-root / wiring-тесты (новый шов)

Один assertion-тест на уровне `cmd/api` или `cmd/api/wire`, вызывающий реальные wire-функции и проверяющий инвариант сборки.

- Базовый инвариант: «ни один policy-bearing / access-bearing сервис не держит stub после полной сборки».
- Должен **краснеть**, если убрать любое критическое wiring-действие (исполнитель обязан продемонстрировать это закомментированием и возвращением).
- Частный случай — T13 ([#171](https://github.com/devnumbers/arenda-platform/issues/171)): defence-in-depth для wiring.

### 4. Role-matrix / table-driven (где есть авторизация)

Матрица (role × action → outcome).

- Эталон: `apps/backend/internal/notifications/application/reminder_policy_enforcement_test.go`.
- Каждое изменение access-seam расширяет или сохраняет coverage матрицы.

### 5. Property-based тесты (где есть инварианты-свойства)

Для VO с арифметикой (`Kopecks.Add/Mul` и т.п.).

- `testing/quick` или hand-rolled generators.
- Цель: ловить не конкретные значения, а **свойства** («Add коммутативно», «Mul на 1 = id», «конструктор с ≤0 всегда ошибается»).

### 6. Regression-тесты (на каждый bug-class)

Любой класс бага, всплывший в ходе рефакторинга, закрывается тестом, который его ловит.

- Имя файла: `*_regression_test.go` или ссылка на issue в комментарии.

## Diversity-требование (принцип «разнообразные»)

Не все тесты одного вида. Минимальный микс **на тикет**:

- ≥1 unit-тест с table-driven cases.
- ≥1 integration-тест (если тикет трогает adapter или БД).
- ≥1 negative test (что **должно** упасть — invalid input, nil, неверный переход состояния).
- ≥1 regression-style assertion (если тикет затрагивает wiring/policy/access).

Запрещено: «просто добавил `t.Run` happy-path и закрыл».

## CI gate (обязательный)

Каждый тикет держит зелёным:

```bash
make backend-test                          # go test -race (unit); -race обязателен — ловит data races
cd apps/backend && go vet ./...
make backend-lint                          # golangci-lint v2.12.2
```

Полный прогон (unit + integration + frontend + admin, требует Docker):

```bash
make test                                  # testcontainers поднимает контейнеры, гоняет всё
```

Для Фазы 3 (монорепа / admin):

```bash
make admin-typecheck
make admin-build
make frontend-test
make admin-test
```

## Acceptance criteria для контракта

- Parent-issue [#190](https://github.com/devnumbers/arenda-platform/issues/190) связан со всеми тикетами Фаз 1–3.
- Каждый тикет Фаз 1–3 ссылается на этот документ в acceptance criteria.
- Каждый тикет проходит CI gate выше.
- (HITL) Заказчик одобряет стратегию до старта Фазы 1.

## Состав Фаз

- **Фаза 1 — Domain hygiene:** [#191](https://github.com/devnumbers/arenda-platform/issues/191) Kopecks VO, [#192](https://github.com/devnumbers/arenda-platform/issues/192) инкапсуляция Property, [#193](https://github.com/devnumbers/arenda-platform/issues/193) split SubscriptionPaymentRepository по ISP.
- **Фаза 2 — Wiring:** [#194](https://github.com/devnumbers/arenda-platform/issues/194) ADR 0035, [#195](https://github.com/devnumbers/arenda-platform/issues/195) Access facade, [#196](https://github.com/devnumbers/arenda-platform/issues/196) composition-root cleanup, [#197](https://github.com/devnumbers/arenda-platform/issues/197) разбить `httpserver.Deps`.
- **Фаза 3 — Монорепа:** [#198](https://github.com/devnumbers/arenda-platform/issues/198) admin OpenAPI codegen, [#199](https://github.com/devnumbers/arenda-platform/issues/199) pnpm workspaces.
- **Reject-ADR (защита от re-suggesting):** [#200](https://github.com/devnumbers/arenda-platform/issues/200), [#201](https://github.com/devnumbers/arenda-platform/issues/201), [#202](https://github.com/devnumbers/arenda-platform/issues/202), [#203](https://github.com/devnumbers/arenda-platform/issues/203).
- **Related (не child):** [#171](https://github.com/devnumbers/arenda-platform/issues/171) T13 — частный случай правила №3 (composition-root тесты).

## Экранные e2e (Playwright) — обязательны для новых экранов

Решение владельца 2026-08-25 (ревизия спеки [#453](https://github.com/devnumbers/arenda-platform/issues/453) по карте [#443](https://github.com/devnumbers/arenda-platform/issues/443)): строка Testing Decisions «экранные компоненты автоматическими тестами не покрываются» отменена — каждый новый экран продукта покрывается Playwright e2e со сверкой дизайна с Figma. Инфраструктура — тикет [#456](https://github.com/devnumbers/arenda-platform/issues/456).

- **Тесты**: `apps/frontend/e2e` (`@playwright/test`; конфиг — `apps/frontend/playwright.config.ts`), один chromium-проект, последовательный прогон (`workers: 1` — сьют разделяет один засеянный бекенд).
- **Среда**: `tools/e2e/frontend/run-frontend-e2e.sh` поднимает одноразовый детерминированный стек — postgres 18 (compose-проект `arenda-e2e`, порт 5436, `apps/backend/docker-compose.e2e.yml`), бекенд (сборка `apps/backend`; миграции применяются на старте; `EMAIL_SENDER=fake`, `PAYMENT_PROVIDER=fake`) и прод-сборку фронта (next standalone, порт 3010; `/api` проксируется на бекенд через `BACKEND_URL`). По завершении стек сносится вместе с вольюмами.
- **Сидинг данных**: SQL-сид `tools/e2e/frontend/seed.sql` — прецедент SQL-сидов существующей API-e2e (`tools/e2e/sql`): владелец с известными телефоном и почтой, пред-аутентифицированная сессия (хеш HMAC от сырого токена в `E2E_SESSION_TOKEN`), активные объекты. Криптографию сид-строк считает `tools/e2e/frontend/e2e-crypto.mjs`: `users.phone` ищется по детерминированному шифротексту (`encryption.DeterministicEncrypt`), поэтому сид пишет его, а не открытый текст.
- **Вход в кабинет**: два пути — реальный UI-логин для сценариев авторизации (код входа извлекается из JSON-лога бекенда: фейковый email-отправитель логирует тело письма) и сид-сессия (кука `session_id`) для экранных тестов без прохода по логину.
- **Скриншоты для сверки с Figma**: каждая экранная спека снимает полный скриншот через хелпер `captureScreen` (`apps/frontend/e2e/fixtures.ts`) — файл в артефактах прогона и вложение в HTML-отчёт; CI-джоба `frontend-e2e` выгружает их артефактами прогона.
- **Сквозной сценарий жизни платежа** ([#468](https://github.com/devnumbers/arenda-platform/issues/468)): `apps/frontend/e2e/payment-lifecycle.spec.ts` — замыкающий tracer-bullet платежей (спека [#453](https://github.com/devnumbers/arenda-platform/issues/453)); одним прогоном ведёт платёж от визарда до удаления (список/страница/подэкраны, материализация мутационным тиком, оплата и история, пауза/возобновление, просрочка и гашение, правка, удаление с «удалить просроченные» и без) и служит регресс-щитом всей цепочки экранов; скриншотов для сверки с Figma не снимает — новых экранов не добавляет. Данные прошлого (просрочку) сеет mid-test SQL-хелпер `execE2eSql` из `fixtures.ts`: `docker exec` в контейнер postgres (`E2E_PG_CONTAINER` экспортирует оркестратор) — тот же канал, которым применяется `seed.sql`; визард создаёт только будущее, поэтому прошлое правилу приносит только SQL-сид. Названия создаваемых платежей включают номер retry-попытки, так что повтор упавшей попытки в CI не задевает остатки прошлой. Запускается обычным `make frontend-e2e` / джобой `frontend-e2e` вместе с остальными спеками.
- **Запуск**: `make frontend-e2e` (локально; нужен Docker и Node/Go), в CI — джоба `frontend-e2e`. В `make test` экранные e2e не входят: среда и прод-билд тяжёлые, контракт — отдельный таргет и отдельная джоба.
