# Research: линт миграций (Squawk и альтернативы), FSD-границы, дополнительные quality-gates

- Тикет: #295 (GitHub, devnumbers/arenda-platform)
- Карта: #289
- Дата: 2026-08-16
- Метод: web-research (GitHub releases/docs, даты релизов на 2026-08-16) + сверка с `.golangci.yml`, `apps/backend/go.mod`, `apps/frontend/eslint.config.mjs`, `apps/backend/db/migrations/`.

## 1. Squawk — линтер миграций PostgreSQL

- Репо: [sbdchd/squawk](https://github.com/sbdchd/squawk) (организации `squawkhq` не существует — сайт только документация). Последний релиз **v2.62.0 (2026-08-06)**; активная разработка, ~1150★. Лицензия **Apache-2.0**.
- Установка: готовые бинари на GitHub Releases (linux/darwin/win, x64/arm64), `npm i -g squawk-cli`, docker `ghcr.io/sbdchd/squawk`, GitHub Action, pre-commit-хук. Rust-рантайм не нужен.
- Чисто статический анализ (свой парсер), **живая БД не требуется**; версия Postgres — флаг `--pg-version`. Список файлов/глобы/stdin; репортеры tty/gcc/json/gitlab; ненулевой exit code при нарушениях. 107 файлов — субсекунда.
- ~40 правил ([squawkhq.com/docs/rules](https://squawkhq.com/docs/rules)): `adding-not-nullable-field`, `adding-field-with-default`, `constraint-missing-not-valid`, `disallowed-unique-constraint`, `changing-column-type`, `require-concurrent-index-*`, `ban-drop-*`, `prefer-robust-stmts`, **`prefer-timestamptz`** (закрывает правило 2.5-tz аудита бесплатно).
- Исключения: `--exclude`, `--exclude-path`, `.squawk.toml` (`excluded_rules`, `excluded_paths`, `pg_version`, `assume_in_transaction`), инлайн `-- squawk-ignore`. **Baseline-файла нет** — задокументированный паттерн: линтовать только изменённые файлы (git diff / staged).
- Совместимость с golang-migrate: полная (plain SQL). Важно: golang-migrate с pgx/v5 выполняет файл одной транзакцией → `CREATE INDEX CONCURRENTLY` невозможен → в `.squawk.toml` обязательно `assume_in_transaction = true`, иначе `require-concurrent-index-*` заспамит все миграции.
- Конфликты с конвенциями репо (выключить в `.squawk.toml`): `prefer-identity`, `adding-serial-primary-key-field`, при желании `prefer-bigint-over-int/smallint` — противоречат uuid-v7-app-side (ADR 0019).

**Вердикт: брать** — единственный живой бесплатный линтер lock-safety миграций Postgres.

## 2. Альтернативы Squawk (все отклонены)

- **Atlas `migrate lint`** ([ariga/atlas](https://github.com/ariga/atlas)): формат golang-migrate поддерживается, но с v0.38 (окт 2025) lint убран в платный Atlas Pro; дефолтные бинари под Atlas EULA; требует dev-database для replay. **Не стоит** — платное гейтирование + тяжелее в CI.
- **pgspot** (timescale): security-скоуп для расширений (search_path), не lock-safety; Python. **Не по теме.**
- **pganalyze/lint**: эксперимент 2023, требует живую БД + HypoPG. **Нет.**
- **sqlfluff** (4.3.0, 2026-08-07): style/formatting-линтер, не migration-safety; Python. **Не для этой задачи.**
- **pgfence**: source-available лицензия — не проходит ограничение open-source.

## 3. Итог по линту миграций: гибрид

Squawk (lock-safety + timestamptz) + кастомный скрипт ~20 строк на доменные инварианты, которых у Squawk нет: запрет `numeric/decimal/real/double precision/float` (деньги-BIGINT, правило 2.5), запрет `DEFAULT` на `id` (2.6а, ADR 0019). Только кастомный grep отклонён: хрупок на многострочных statements/квотинге/комментариях, а lock-safety — самый дорогой класс инцидентов. Запуск по git-diff/staged (baseline не нужен) + pre-push полный прогон + CI.

## 4. FSD-границы (ESLint 9 flat config, frontend)

- **eslint-plugin-boundaries** ([v7.2.0, 2026-08-09](https://github.com/javierbrea/eslint-plugin-boundaries/releases/tag/v7.2.0)): MIT, очень активен, нативный flat config, поддержка TS/tsconfig-paths. Модель elements+policies точно ложится на FSD (иерархия `app→widgets→features→entities→shared` + запрет cross-slice). **Брать.**
- **@feature-sliced/eslint-config**: последний пуш 2024-10-17, experimental, фактически заброшен. **Не брать.**
- **steiger** (feature-sliced): MIT, жив, официальный структурный линтер FSD, но отдельный раннер (не ESLint); импорт-границы слабее boundaries. **Спорно — не сейчас.**
- **conarti/eslint-plugin-feature-sliced**: лицензия не указана. **Нет.**
- **dependency-cruiser** (v18.2.0): MIT, активен, standalone CLI со своим DSL; циклы/сироты бонусом. Жизнеспособен, но для чистых границ boundaries дешевле (живёт в существующем ESLint). **Не сейчас.**
- Запрет `shared/api/generated.ts` вне `shared/api` и `@heroui/styles` — одно правило `no-restricted-imports`, плагин не нужен.

## 5. Дополнительные проверки (сканирование по цене/выигрышу)

**Go** (сверено с `.golangci.yml` и `go.mod`):
- `perfsprint`, `usestdlibvars`, `copyloopvar` — уже включены; `intrange` осознанно выключен (дубль `modernize.rangeint`). Добавлять нечего.
- **go-arch-lint** (v1.17.0, жив, bus factor=1): функциональный дубль depguard в отдельном YAML. **Не стоит.**
- **goda** (loov): ad-hoc анализ графа зависимостей, не CI-gate. **Не гейт** (разве что dev-разведка).
- **testifylint**: комментарий `.golangci.yml:295` утверждает «нет testify», но `stretchr/testify v1.11.1` есть в `go.mod:18` и используется ровно в одном файле (`internal/notifications/adapters/http/push_subscription_handlers_test.go`). **Линтер не стоит** — дешевле переписать файл на stdlib и убрать testify из go.mod (попутная правка).

**TS**:
- **knip** (6.32.2, 2026-08-11, ISC): мёртвые файлы/экспорты/зависимости, workspaces из коробки (frontend+admin+landing одним прогоном). Цена — настройка entry-паттернов и причёсывание false positives. **Брать в advisory-режиме**, gate — после стабилизации конфига.
- **eslint-plugin-import-x** (v4.17.1): ценность — только `no-cycle`; дубль резолверов с `eslint-plugin-import` из eslint-config-next. **Спорно — не брать.**
- Иного существенного для Next 16 + react-admin 5 поверх eslint-config-next не выявлено; главный пробел админки — само отсутствие ESLint.
