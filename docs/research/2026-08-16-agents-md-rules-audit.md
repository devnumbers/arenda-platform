# Аудит поведенческих правил AGENTS.md: инструментальная подкреплённость

- Тикет: #292 (GitHub, devnumbers/arenda-platform)
- Карта: #289
- Дата: 2026-08-16
- Метод: полное чтение четырёх AGENTS.md (корень, backend, frontend, admin) и сверка каждого правила с реальными конфигами репозитория (`.golangci.yml`, `Makefile`, `.github/workflows/ci.yml`, `security.yml`, eslint/tsconfig/package.json обоих SPA, `tools/property-attributes/`). Веб не использовался.

## Легенда меток

- **[инструмент]** — правило уже принуждается автоматически. Для каждой метки указан подтверждающий файл/конфиг, проверенный чтением.
- **[переводимо]** — правило можно принудить инструментом; указан механизм и примерная цена.
- **[проза]** — инструментально не выразимо или неразумно; указано почему.

Частичное покрытие отмечено как «[инструмент] (частично)» — метка ставится, только если подтверждённая автоматика реально существует, с явным перечислением пробелов.

## Существующий инструментарий принуждения (фон, проверено чтением)

Backend:
- `.golangci.yml` — курируемый набор линтеров (баги, гигиена, архитектура): `depguard` с правилами `domain-clean` / `application-clean` / `platform-clean` (строки 71, 81–164), `forbidigo` с запретом stdlib `log`, `fmt.Print*` и ручного `.Begin` вне UoW для identity/billing (207–227), `sloglint context: all` (179–180), `gosec`, `bidichk`, `asciicheck`; форматтеры `gofumpt` + `gci` (277–280); версия языка берётся из `apps/backend/go.mod` (10).
- `Makefile:43–44` — `backend-lint`; `Makefile:59–68` — unit/integration тесты с `-race`.
- CI (`.github/workflows/ci.yml`): Lint (24–25), T-Kassa spec freshness (27–28), Test `-race` (33–38), Vet (40–41), govulncheck (43–47), миграции up/down/up (49–89).
- `Makefile:111–122` — `backend-tkassa-spec-check` (freshness-гейт патч-спеки Т-Кассы, в CI: ci.yml:27–28).
- `Makefile:142–153` — `attributes-check` (freshness-гейт каталога атрибутов, в CI: ci.yml:118–119); генератор — `tools/property-attributes/generate.mjs` (валидация catalog.json через ajv перед записью).

Frontend:
- `apps/frontend/eslint.config.mjs` — `eslint-config-next/core-web-vitals` + `typescript`. Пресет `typescript` подтягивает `@typescript-eslint` flat `recommended`, где `@typescript-eslint/no-explicit-any: 'error'` (подтверждено: `node_modules/@typescript-eslint/eslint-plugin/dist/configs/flat/recommended.js:29`).
- `apps/frontend/tsconfig.json:7` — `strict: true`; CI-гейт typecheck `npx tsc --noEmit` (ci.yml:129–131), lint (125–127), build (121–123).

Admin:
- `apps/admin/tsconfig.json:14–17` — `strict`, `noUnusedLocals`, `noUnusedParameters`, `noFallthroughCasesInSwitch`; CI-гейты typecheck (ci.yml:151–153) и build (155–157). ESLint в админке **отсутствует**: ни конфига, ни lint-скрипта в `apps/admin/package.json`, ни пакета eslint в devDependencies.

Общее: gitleaks (ci.yml:181–197, `.gitleaks.toml`), hadolint (ci.yml:214–218), trivy image scan (244–252), semgrep/trivy-fs (`.github/workflows/security.yml`, на PR — continue-on-error).

Замеченные расхождения CI с AGENTS.md (не правила, но важно для решения): CI **не** гоняет `frontend-test` (vitest), `admin-test` и backend integration-тесты, хотя AGENTS.md требует их перед completion. Также в CI нет freshness-гейта для `spec.gen.go`/sqlc/frontend-клиента openapi.

---

## 1. Архитектура и границы слоёв

**1.1. Направление зависимостей backend только внутрь: transport/adapters → application → domain** (`apps/backend/AGENTS.md`, «Architecture Rules»).
**[инструмент]** — `depguard` в `.golangci.yml:81–164`: `domain-clean` запрещает domain-пакетам `net/http`, `database/sql`, `pgx`, `aws-sdk-go-v2`, `platform` и `adapters` всех перечисленных контекстов; `application-clean` — аналогично для application; `platform-clean` (ADR 0034) держит platform свободной от bounded contexts. Гейт — `make backend-lint` (Makefile:43–44) и CI Lint (ci.yml:24–25).
Пробелы: deny-списки перечисляют контексты явно — новый контекст нужно добавлять в конфиг руками; cross-context импорты (domain одного контекста из domain другого) не ограничены.

**1.2. Domain не импортирует HTTP, OpenAPI generated types, pgx, sqlc, database/sql, config, adapters** (там же).
**[инструмент] (частично)** — тем же `depguard domain-clean`. Пробелы: `config` и пакет сгенерированных OpenAPI-типов явно в deny-списке не перечислены (openapi-gen фактически ловится только если лежит под `adapters`).

**1.3. Модульный монолит до ADR о разделении** (`apps/backend/AGENTS.md`).
**[проза]** — стратегическое архитектурное решение, фиксируется ADR; инструментом не выражается.

**1.4. Реестр bounded contexts; новые — по docs/ADR** (там же).
**[проза]** — дискреционное решение + процесс документирования. (Косвенно связано с пробелом в 1.1: новый контекст требует ручного добавления в depguard.)

**1.5. Аудит пишется в транзакции бизнес-операции (fail-safe)** (`apps/backend/AGENTS.md`, ADR 0020).
**[проза]** — семантический инвариант конкретного модуля; проверяется тестами per-case, универсального статического правила нет.

**1.6. Application владеет use cases, портами, оркестрацией и границами транзакций** (там же).
**[проза]** — семантика слоя. Частично подкреплено: `forbidigo` запрещает ручной `.Begin` в `identity/application` и `billing/application` (`.golangci.yml:213–227`, ADR 0033) — но это правило про UoW, не про всю полноту слоя.

**1.7. Adapter/platform владеют HTTP, persistence, config, logging, внешними сервисами, generated code** (там же).
**[проза]** — определение ответственности слоя; статически не отличить «владение» от «использования».

**1.8. Мелкие пакеты, явные имена, намеренные ошибки, скучные зависимости; простой Go** (там же).
**[проза]** — качественные критерии; линтеры метрик сложности осознанно отключены в проекте (`.golangci.yml:294`, «осознанно НЕ включаем»).

**1.9. Определения FSD-слоёв frontend: app/widgets/features/entities/shared** (`apps/frontend/AGENTS.md`, «Architecture (FSD)»).
**[проза]** — определения, не правила проверки.

**1.10. FSD: направление зависимостей app/widgets → features → entities → shared; без импортов вверх и вбок между слайсами** (там же).
**[переводимо]** — `eslint-plugin-boundaries` (готовый плагин под FSD) или дёшево: `no-restricted-imports` с path-паттернами по слоям. Цена: средняя (настройка + разовый baseline существующих нарушений). Выигрыш высокий: прямой аналог уже окупившегося depguard на backend.

**1.11. UI тупой; бизнес-логика в features/entities; серверные вызовы в shared/api или route handlers** (там же).
**[проза]** — семантическое правило; частично следствие 1.10, но «тупость UI» статически не проверяется.

**1.12. Admin: все обращения к backend через `dataProvider`, вся аутентификация через `authProvider`** (`apps/admin/AGENTS.md`, «Architecture»).
**[переводимо]** — eslint `no-restricted-globals`/`no-restricted-imports` (запрет `fetch` вне `dataProvider.ts`). Блокер: в админке нет eslint вообще — сначала внедрить (средняя цена), затем правило тривиально.

**1.13. Admin — standalone Vite SPA; Next.js-правила не применять** (там же).
**[проза]** — отрицательная инструкция процесса; инструментом не выражается.

**1.14. Server Components по умолчанию; `'use client'` только для интерактивности; server-side data fetching по умолчанию** (`apps/frontend/AGENTS.md`).
**[проза]** — дискреционный выбор на каждый компонент; существующие линтеры (react-server-components плагины) ловят лишь краевые ошибки, не «по умолчанию».

**1.15. Browser auth — opaque server-side сессии с HttpOnly cookie; не заменять на JWT без ADR** (`apps/backend/AGENTS.md`).
**[проза]** — архитектурное решение под защитой ADR-процесса.

**1.16. Фото через storage port (REG.RU S3); не добавлять MinIO** (там же).
**[переводимо]** — depguard deny `github.com/minio/*` (1 строка в `.golangci.yml`) + опционально проверка compose-файлов. Цена минимальная, выигрыш низкий (угроза гипотетическая).

## 2. Деньги, даты, идентификаторы

**2.1. Деньги — `BIGINT` копейки; integer-only арифметика; никогда не float на любом слое** (корневой `AGENTS.md`, «Repository Conventions»; `apps/backend/AGENTS.md` «API And Persistence»).
**[переводимо]** — варианты: (а) semgrep/кастомный линт на `float64`/`float32` в `internal/**/domain` и `application` (цена средняя, риск ложных срабатываний на неденежных float); (б) линт миграций: запрет `NUMERIC`/`REAL`/`DOUBLE` для money-колонок (цена низкая-средняя, см. 2.5); (в) единый тип `Money int64` + ревью. Полностью надёжного готового правила нет — деньги отличимы от прочих int64 только по контексту/именованию.

**2.2. Форматирование денег только в UI-слое: `formatMoneyKopecks` (frontend), `formatKopecks`/`MoneyField` (admin); никогда не hand-roll `/100`** (корневой `AGENTS.md`; `apps/frontend/AGENTS.md` «API & Data Flow»; `apps/admin/AGENTS.md` «Architecture»).
**[переводимо]** — кастомное eslint-правило или semgrep на паттерны деления/toLocaleString рядом с money-полями (хрупко, цена средняя); надёжнее — `no-restricted-imports`: запрет прямого использования форматирующих API вне `shared/lib/format-money.ts`/`src/fields.tsx` частично выражаем. Для админки требует eslint (блокер 1.12).

**2.3. Только RUB, мультивалютности нет** (корневой `AGENTS.md`, ADR 0036).
**[проза]** — пока валюта одна, проверять нечего; правило-напоминание для дизайна.

**2.4. Два денежных словаря: «Операции» vs «платёж/транзакция» только для Billing** (корневой `AGENTS.md`, ADR 0036).
**[проза]** — доменный язык; канонические термины живут в `CONTEXT-MAP.md`/`CONTEXT.md`. Теоретически greppable по UI-строкам, но крайне хрупко — неразумно.

**2.5. `date` для доменных дат, `timestamptz` для системных отметок, `BIGINT` для денег — в схеме БД** (`apps/backend/AGENTS.md`).
**[переводимо]** — линт миграций: CI-скрипт, проверяющий новые файлы `db/migrations` (запрет `timestamp` без tz, запрет float/numeric для money, соответствие именования). Цена средняя (кастомный скрипт + соглашения по именам колонок). Выигрыш высокий для финансового домена.

**2.6. Идентификаторы — UUIDv7, генерируются в приложении (`uuid.NewV7()`); у `id` нет `DEFAULT` в БД; в SQL — `uuidv7()`** (там же, ADR 0019).
**[переводимо]** — (а) линт миграций: запрет `DEFAULT` на колонках `id` (низкая-средняя цена, общий скрипт с 2.5); (б) `forbidigo` на `uuid.New()`/`uuid.NewV4()` в Go (низкая цена, готовый механизм). Выигрыш средний: защита от тихого отката конвенции.

## 3. API-контракт и сгенерированный код

**3.1. Contract-first: `api/openapi/openapi.yaml` меняется до изменения HTTP-поведения** (`apps/backend/AGENTS.md`).
**[проза]** — порядок работы (процесс). Косвенно подкрепляется freshness-гейтом из 3.4, если его ввести: «контракт изменился, а сгенерированный код нет» станет падением CI.

**3.2. Сгенерированные OpenAPI DTO живут на HTTP-границе и явно мапятся в application/domain-модели** (там же).
**[переводимо]** — depguard deny пакета сгенерированных openapi-типов для `domain`/`application` (1–2 строки в существующие `domain-clean`/`application-clean`, цена минимальная). Выигрыш высокий: закрывает и пробел из 1.2.

**3.3. Frontend: не протекать generated DTO в widgets/features; маппинг на API-границе** (`apps/frontend/AGENTS.md`).
**[переводимо]** — `no-restricted-imports` на `shared/api/generated.ts` вне `shared/api` и `entities` (одно готовое правило, цена низкая).

**3.4. Не править generated-файлы руками; менять источник и регенерировать** (`apps/backend/AGENTS.md`, `apps/frontend/AGENTS.md`).
**[инструмент] (частично) + [переводимо] (остаток)** — покрыто: каталог атрибутов (`attributes-check`, Makefile:142–153 + ci.yml:118–119) и спека Т-Кассы (`backend-tkassa-spec-check`, Makefile:111–122 + ci.yml:27–28). **Не покрыто**: `spec.gen.go` основного API (oapi-codegen), sqlc output, `apps/frontend/shared/api/generated.ts` (скрипт `generate:api` есть — package.json:12 — но гейта нет). Перевод: CI regenerate+diff по готовому паттерну `attributes-check`. Цена низкая. (Сопутствующая страховка: `.golangci.yml:249` `exclusions.generated: strict` только отключает линт для generated, ручную правку не ловит.)

**3.5. Каталог атрибутов генерируется из `tools/property-attributes/catalog.json`; регенерация `make attributes-gen`** (все три app-AGENTS.md).
**[инструмент]** — `attributes-check` (Makefile:142–153) в CI (ci.yml:118–119); генератор валидирует catalog.json схемой (ajv) перед записью (`tools/property-attributes/package.json`).

**3.6. Frontend: не редактировать generated API client; обновлять контракт и регенерировать** (`apps/frontend/AGENTS.md`).
**[переводимо]** — тот же freshness-гейт, что в 3.4 (npm run generate:api + hash-diff). Цена низкая.

**3.7. Admin: маппинг DTO на границе `dataProvider`; API-формы не протекают в resource-компоненты** (`apps/admin/AGENTS.md`).
**[переводимо]** — `no-restricted-imports` на generated-типы вне `dataProvider.ts`; требует eslint в админке (блокер 1.12).

**3.8. Переиспользовать backend-типы из OpenAPI; frontend-entity-типы явные и минимальные** (`apps/frontend/AGENTS.md`).
**[проза]** — «минимальность» и «явность» — дискреционные критерии.

## 4. БД и миграции

**4.1. Изменения схемы — версионными миграциями в `db/migrations` + соответствующие запросы в `db/queries`** (`apps/backend/AGENTS.md`).
**[проза] (частично подкреплено)** — как процесс не форсируется, но CI job `backend-migrations` (ci.yml:49–89) проверяет применимость всей цепочки миграций (up → down -all → up), т.е. битая миграция падает в CI.

**4.2. Инварианты держать в PostgreSQL: NOT NULL, FK, CHECK, индексы, триггеры** (там же).
**[проза]** — дизайн-решение на каждую сущность; «достаточность» инвариантов статически не проверяется.

**4.3. Явный PostgreSQL SQL через sqlc; не вводить ORM** (там же).
**[переводимо]** — depguard deny `gorm.io/*`, `entgo.io/*`, `xorm.io/*` (3 строки в `.golangci.yml`, цена минимальная, ложных срабатываний нет — этих пакетов нет в go.mod).

**4.4. Авторизация через policy port; owner-scoped запросы фильтруют по `scope`, не по `actor`; не возвращать голый `ownerID`** (там же, ADR 0028).
**[проза]** — семантический инвариант доступа; статически отличить `scope` от `ownerID` невозможно (это соглашение об именовании параметров, хрупко даже для кастомного линта).

## 5. Качество кода (стиль, типы)

**5.1. Backend-код gofumpt-clean с gci-порядком импортов** (`apps/backend/AGENTS.md`, «Quality Gates»).
**[инструмент]** — форматтеры `gofumpt` + `gci` в `.golangci.yml:277–280`; гейт `make backend-lint` (Makefile:43–44) + CI Lint (ci.yml:24–25); autofix-команда задокументирована там же.

**5.2. Frontend: `strict: true` и строжайшие практичные опции компилятора** (`apps/frontend/AGENTS.md`).
**[инструмент]** — `strict: true` в `apps/frontend/tsconfig.json:7`; CI typecheck `npx tsc --noEmit` (ci.yml:129–131). Оговорка: ослабление флага в самом tsconfig CI не падает — ловится ревью; «строжайшие практичные» — проза.

**5.3. Frontend: `eslint-config-next` (`core-web-vitals` + `typescript`)** (там же).
**[инструмент]** — `apps/frontend/eslint.config.mjs` (оба пресета подключены); `npm run lint` (package.json:9); CI Lint (ci.yml:125–127).

**5.4. Frontend: избегать `any`; `unknown` + narrowing; при неизбежности — комментарий и ADR** (там же).
**[инструмент]** — `@typescript-eslint/no-explicit-any: 'error'` входит в flat `recommended`, подтягиваемый `eslint-config-next/typescript` (подтверждено: `node_modules/@typescript-eslint/eslint-plugin/dist/configs/flat/recommended.js:29`); гейт — CI Lint. Проза-остаток: требование комментария/ADR для `eslint-disable`.

**5.5. Admin: избегать `any`; `unknown` + narrowing** (`apps/admin/AGENTS.md`).
**[переводимо]** — **не подтверждено инструментом**: в админке нет eslint (ни конфига, ни скрипта в `apps/admin/package.json`); `tsc strict` ловит только implicit any, явный `any` разрешён. Перевод: внедрить eslint с `@typescript-eslint/no-explicit-any` (средняя цена за внедрение, правило само готовое). Открывает дорогу правилам 1.12, 2.2, 3.7.

**5.6. Admin: `strict: true` и существующие strict-опции; `npm run typecheck` чист** (`apps/admin/AGENTS.md`).
**[инструмент]** — `apps/admin/tsconfig.json:14–17` (`strict`, `noUnusedLocals`, `noUnusedParameters`, `noFallthroughCasesInSwitch`); CI typecheck (ci.yml:151–153).

**5.7. Frontend: явные return-типы у функций, экспортируемых из shared/entities/features/widgets** (`apps/frontend/AGENTS.md`).
**[переводимо]** — готовое правило `@typescript-eslint/explicit-function-return-type` (или `explicit-module-boundary-types`) с overrides по каталогам слоёв. Цена низкая, но возможен шум на мелких утилитах — потребуется разовый baseline/настройка исключений.

**5.8. Frontend: `readonly`, `ReadonlyArray`, `as const` для иммутабельных данных** (там же).
**[проза]** — «предпочитать» дискреционно; `eslint-plugin-functional/prefer-readonly-type` существует, но шумный и меняет идиоматику — неразумно.

**5.9. Мелкие сфокусированные компоненты; без prop drilling; композиция/feature-context** (`apps/frontend/AGENTS.md`, `apps/admin/AGENTS.md`).
**[проза]** — качественные критерии дизайна компонентов.

**5.10. Нет глобального стейта для локального UI; URL-state для шейрабельного состояния; явные обработчики/редьюсеры** (`apps/frontend/AGENTS.md`).
**[проза]** — дискреционные решения по состоянию; статически не отличить «локальный UI state» от прочего.

**5.11. Правила навигации: `goBack`/`router.replace`/`router.push` по типу флоу; `router.replace` на страницу-источник запрещён** (`apps/frontend/AGENTS.md`, «Навигация и история браузера»).
**[проза]** — правила про динамику истории браузера; целевая страница — рантайм-переменная, статически не выводится. Проверяется тестами/Playwright per-case.

**5.12. Конвенции наблюдаемости backend: slog, OTel, request ID, PII redaction** (`apps/backend/AGENTS.md`, «Observability»; `docs/backend-observability.md`).
**[инструмент] (частично)** — `sloglint context: all` (`.golangci.yml:179–180`) кодирует правило `InfoContext/WarnContext/ErrorContext` (прямо отмечено в комментарии конфига); `spancheck` (173–175) ловит пропущенный `span.End()`. Проза-остаток: именование спанов, low-cardinality лейблы, PII redaction.

**5.13. Admin: новые пользовательские строки — с русской локализацией (ra-i18n-polyglot)** (`apps/admin/AGENTS.md`).
**[проза]** — статически отличить «user-facing строку» от служебной хрупко; кастомный линт на кириллицу в JSX дал бы ложные срабатывания.

## 6. Процесс / workflow

**6.1. Язык коммуникации — русский** (корневой `AGENTS.md`, «Scope»).
**[проза]** — инструкция агенту, не свойство кода.

**6.2. Обязательный workflow на Matt Pocock skills; скиллы вызываются по имени** (корневой, «Workflow System», «Workflow (Matt Pocock skills)»).
**[проза]** — процесс агента; скиллы установлены в `.agents/skills/`, но принудить их вызов репозиторными средствами нельзя (разве что хуками конкретного harness — вне скоупа репо).

**6.3. Перед изменением поведения читать `CONTEXT.md`, docs, ADR** (корневой, «Work Rules»).
**[проза]** — процесс.

**6.4. Проверять `git status` перед правками; не затирать чужие изменения** (там же).
**[проза]** — процесс.

**6.5. Перед добавлением сущностей/хелперов/абстракций искать существующие аналоги (Grep/lean-ctx)** (там же; аналоги в frontend/admin AGENTS.md).
**[проза]** — процесс исследования.

**6.6. Использовать lean-ctx для широкого исследования; перед правкой читать raw/full** (там же; продублировано во всех app-AGENTS.md).
**[проза]** — процесс.

**6.7. Проверять наличие MCP-инструментов перед использованием; объявлять fallback** (там же, «MCP Servers»).
**[проза]** — процесс.

**6.8. Высокорисковые действия — только за явным интентом; никаких wildcard-доверий** (там же).
**[проза]** — политика безопасности процесса (настраивается в harness, не в репо).

**6.9. Держать контекст маленьким; чистить между задачами** (там же).
**[проза]** — процесс.

**6.10. Перед completion: прогнать проверки и свежий ревью дифа** (там же).
**[проза]** — дисциплина агента. CI существует, но «перед completion» не принудить репозиторными средствами.

**6.11. Перед completion: `make test` (или гранулярные цели по ходу работы)** (там же; quality gates во всех app-AGENTS.md).
**[проза]** — дисциплина агента. Замеченное расхождение: CI **не** гоняет `frontend-test`, `admin-test` и backend integration-тесты — т.е. даже после merge часть «полного набора» нигде не принуждается. Кандидат на CI-шаги (цена низкая: джобы по образцу существующих).

**6.12. `/tdd` — дефолт: любое изменение поведения test-first** (там же).
**[проза]** — порядок написания кода не проверяется постфактум.

**6.13. Не создавать git worktree по умолчанию; правило репо перекрывает скиллы** (там же, «Repository Conventions»).
**[проза]** — процесс.

**6.14. `make local-infra-reset` — только при осознанном удалении данных** (там же).
**[проза]** — предостережение.

**6.15. Backend: обязательные скиллы `use-modern-go`, `go`, `postgresql-best-practices`, `devops-engineer`, `docker`; gopls обязателен (stop-procedure); gopls/jetbrains-диагностика как гейт перед completion** (`apps/backend/AGENTS.md`).
**[проза]** — всё это процесс агента; репозиторный аналог качественного гейта уже есть (lint/vet/test в CI), но вызов скиллов/MCP не принуждается.

**6.16. Frontend/admin: обязательные скиллы (next-best-practices, vercel-react-*, typescript и др.); playwright для UI-проверки; heroui-react как источник доков** (`apps/frontend/AGENTS.md`, `apps/admin/AGENTS.md`).
**[проза]** — процесс. Частный переводимый фрагмент: запрет импортов `@heroui/styles` (BEM) в React-компонентах выразим `no-restricted-imports` (цена низкая) — но сама верификация API компонентов через MCP остаётся прозой.

**6.17. Приоритет официальной документации над обучающими данными; context7 для сторонних библиотек; vendor-доки для security-sensitive** (все app-AGENTS.md).
**[проза]** — процесс.

**6.18. Issues и triage-метки через `gh` CLI** (корневой, «Agent skills»).
**[проза]** — процесс.

**6.19. Версия Go — единый источник правды** (`apps/backend/AGENTS.md`, «Target Go: 1.26»).
**[инструмент]** — `apps/backend/go.mod` как источник: CI `setup-go` берёт `go-version-file: apps/backend/go.mod` (ci.yml:21), golangci-lint версию языка берёт из go.mod (`.golangci.yml:10`). (Текст «1.26» в AGENTS.md — проза-дубль, расходится при смене go.mod.)

## 7. Безопасность

**7.1. Не хардкодить секреты; конфиг admin — из Vite env vars, не хардкодить URL** (`apps/admin/AGENTS.md`; корневой п.6.8).
**[инструмент] (частично)** — секреты: gitleaks в CI (ci.yml:181–197, конфиг `.gitleaks.toml`) — блокирующий гейт на PR. Hardcoded backend URL: не покрыто — **[переводимо]** (eslint-правило/semgrep на URL-литералы в `apps/admin/src`, цена низкая, но требует eslint в админке).

**7.2. Security-sensitive поведение (credentials, auth, payments, storage, Docker, CI/CD) сверять с официальными vendor-доками** (`apps/backend/AGENTS.md`).
**[проза]** — процесс верификации.

**7.3. (Фон, не из AGENTS.md) Существующие security-гейты** — `gosec`, `bidichk`, `asciicheck` в backend-lint (`.golangci.yml`); govulncheck (ci.yml:43–47); trivy image scan (ci.yml:244–252); hadolint (ci.yml:214–218); semgrep/trivy-fs (security.yml, на PR неблокирующие). Перечислены для полноты картины «что уже принуждается».

---

## Выводы для решения

Всего каталогизировано **67 правил**:

- **[инструмент] — 11** (плюс 4 с оговоркой «частично»: 1.2, 3.4, 4.1, 5.12, 7.1 — в счёт вошли как инструмент с явными пробелами): depguard-границы backend, gofumpt+gci, attributes-check, tsconfig strict + typecheck в CI (оба SPA), eslint-пресеты frontend, `no-explicit-any` на frontend, sloglint/spancheck, версия Go из go.mod, gitleaks.
- **[переводимо] — 15**: FSD-границы (1.10), dataProvider-граница админки (1.12), MinIO-deny (1.16), деньги-int64 (2.1), форматирование денег (2.2), линт миграций даты/деньги (2.5), UUIDv7 (2.6), DTO на HTTP-границе backend (3.2), DTO frontend (3.3), freshness-гейты generated (3.4/3.6), DTO admin (3.7), ORM-deny (4.3), no-explicit-any admin (5.5), explicit return types (5.7).
- **[проза] — 41**: весь workflow/скиллы/TDD/MCP-процесс, доменный язык, дискреционные архитектурные и UI-решения, семантические инварианты (scope/actor, аудит в транзакции, инварианты БД).

### Топ кандидатов на перевод (цена/выигрыш)

1. **Freshness-гейты для `spec.gen.go` (oapi-codegen), sqlc и `shared/api/generated.ts`** (3.4, 3.6) — паттерн `attributes-check` готов и копируется почти дословно; закрывает «contract-first» и «не править generated» на главном API. Цена низкая, выигрыш высокий.
2. **Depguard: deny ORM-пакетов (4.3) и openapi-gen/config для domain/application (3.2, 1.2)** — 3–5 строк в существующий `.golangci.yml`; закрывает реальные пробелы в уже работающем механизме. Цена минимальная.
3. **FSD-границы через `eslint-plugin-boundaries` или `no-restricted-imports` (1.10 + 3.3 + 6.16-фрагмент `@heroui/styles`)** — frontend-аналог depguard; заодно закрывает протекание generated DTO в widgets/features. Цена средняя (baseline), выигрыш высокий.
4. **ESLint в админку: `no-explicit-any` + `no-restricted-imports` (5.5 → 1.12, 2.2, 3.7, 7.1-URL)** — единственный стек без линтера; одно внедрение открывает пачку правил. Цена средняя.
5. **Линт миграций: `timestamptz` vs `timestamp`, `BIGINT` для денег, запрет `DEFAULT` на `id` (2.5 + 2.6)** — кастомный CI-скрипт по новым файлам `db/migrations`; защищает финансовые инварианты в точке, где ошибка дороже всего. Цена средняя, выигрыш высокий.

Почётное упоминание: `@typescript-eslint/explicit-function-return-type` для слоёв frontend (5.7, низкая цена, средний выигрыш — шум); добавление `frontend-test`/`admin-test` в CI (6.11, низкая цена — закрывает расхождение AGENTS.md и CI).

### Замеченные расхождения (не правила, но к делу)

- CI не гоняет `frontend-test`, `admin-test` и backend integration-тесты, хотя AGENTS.md требует их перед completion (6.11).
- `apps/frontend/AGENTS.md` называет Next.js `16.2.9`, фактически в `package.json` — `16.2.11` (eslint-config-next при этом 16.2.9) — мелкий дрейф документации.
- Depguard deny-списки перечисляют контексты явно — при добавлении нового bounded context (правило 1.4) гейт молча ослабевает, пока контекст не добавят в `.golangci.yml`.
