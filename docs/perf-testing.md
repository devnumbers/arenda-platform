# Локальное нагрузочное тестирование backend

Документ описывает локальный harness на `vegeta` для read endpoint'ов API. Цель прогонов — получить полезный предел backend/PostgreSQL и явно увидеть, если раньше упёрся сам генератор нагрузки.

## Компоненты

| Компонент | Расположение | Назначение |
|-----------|--------------|------------|
| Perf PostgreSQL | `docker-compose.perf.yml` | PostgreSQL 18 на локальном perf-порту |
| Seed | `apps/backend/cmd/perfseed` | Генерирует владельцев и связанные сущности, пишет `perf/fixtures.json` |
| Runner | `apps/backend/cmd/perfvegeta` | Запускает `sustainable` и `breakdown` через `vegeta` |
| Отчёты | `apps/backend/perf/results/` | `summary.json` и `probes.csv` со всеми probe |

## Workflow В Два Терминала

Терминал 1: база и backend с live logs.

```bash
make perf-db-reset
make perf-backend-run
```

`make perf-backend-run` собирает backend в `.tmp/perf-api` и запускает binary в foreground. Логи сервера видны прямо в этом терминале; остановка через `Ctrl+C`.

После изменения perf PostgreSQL settings или init SQL используйте именно `make perf-db-reset`, потому что `pg_stat_statements` создаётся при инициализации нового Docker volume.

Терминал 2: seed и нагрузочный тест.

```bash
make perf-seed
make perf-sustainable
make perf-breakdown
```

По умолчанию `make perf-sustainable` и `make perf-breakdown` запускают endpoint `get_me`. Для другого endpoint:

```bash
make perf-sustainable ENDPOINT=list_properties
make perf-breakdown ENDPOINT=list_properties
```

Чтобы прогнать все read endpoint'ы по очереди, передайте пустой endpoint:

```bash
make perf-sustainable ENDPOINT=
make perf-breakdown ENDPOINT=
```

## Режимы

| Режим | Описание |
|---|---|
| `sustainable` | Сначала проверяет локальную базовую точку `startRate=5000/s`, затем бинарным поиском ищет максимальный RPS, где `success = 100%` и `p95 < 500 ms`. Если `startRate` не проходит, итоговый статус — `no_sustainable_rate`. |
| `breakdown` | Наращивает RPS шагами и останавливается на первом probe, где `success <= 90%` или раньше упёрся генератор. В отчёте сохраняются `last_success_rate` и `first_broken_rate`. Этот режим — stress map, а не подтверждённый sustainable limit. |

## Настройки Нагрузки

Perf-настройки не задаются через env. Они зафиксированы константами в `apps/backend/cmd/perfvegeta/main.go`, чтобы локальные результаты были воспроизводимыми:

| Параметр | Значение |
|---|---:|
| `startRate` | `5000/s` |
| `maxRate` | `50000/s` |
| `rateStep` | `500/s` |
| `tolerance` | `100/s` |
| `duration` | `30s` |
| `timeout` | `10s` |
| `probeCooldown` | `5s` |
| `sustainableConfirmations` | `3` |
| `connections` | `12000` |
| `maxConnections` | `12000` |
| `workers` | `2000` |
| `maxWorkers` | `12000` |
| `maxBody` | `0` |

Runner всегда передаёт `-max-connections`, держит `connections <= max_connections`, оставляет keep-alive включённым и выставляет `-max-body=0`, чтобы не сохранять тела ответов в результатах `vegeta`.

На старте runner читает локальные host limits (`ulimit -n`, Darwin `sysctl`, CPU/RAM) только для диагностики и предупреждений. Он не меняет `ulimit`, `sysctl` или настройки macOS.

## Make Variables И App Env

Переменные Makefile относятся к запуску приложения и БД, а не к tuning нагрузочного теста или backend pool/logging profile:

| Переменная | Значение по умолчанию | Назначение |
|------------|----------------------|------------|
| `PERF_POSTGRES_PORT` | `5434` | Локальный порт perf PostgreSQL |
| `PERF_HTTP_ADDR` | `:8081` | Адрес backend |
| `PERF_APP_BASE_URL` | `http://127.0.0.1:8081` | Base URL приложения |
| `PERF_DATABASE_URL` | `postgres://arenda:arenda@localhost:5434/arenda?sslmode=disable` | PostgreSQL для backend, seed и optional DB diagnostics runner |
| `PERF_MIGRATIONS_DIR` | `apps/backend/db/migrations` | Путь к миграциям при запуске perf backend из корня репозитория |
| `ENDPOINT` | `get_me` | Endpoint для `make perf-sustainable` и `make perf-breakdown`; пустое значение запускает все read endpoint'ы |

Perf-профиль backend hardcoded в коде: pool держит `max=64` и `min=16`, чтобы приложение не занимало все PostgreSQL connections. Perf-контейнер PostgreSQL запускается с `max_connections=128`, оставляя headroom для диагностики и служебных подключений. Быстрые успешные access logs hardcoded выключены; ошибки, медленные успешные запросы и slow DB acquire/query события остаются в live logs.

Perf PostgreSQL включает локальную диагностику:

- `pg_stat_statements` для top SQL по probe;
- `compute_query_id=auto` и `track_io_timing=on`;
- `auto_explain` для запросов медленнее `100ms`;
- `pg_stat_statements.track_planning` не включается по умолчанию, чтобы не добавлять лишний overhead во время high-RPS прогона.

Основной профиль один: `64/16`. Для сравнения других размеров pool нужно менять кодовый профиль осознанно, а не подменять env при запуске, чтобы локальные результаты оставались воспроизводимыми.

## Bottleneck

Каждый probe и итоговая строка содержат `bottleneck`:

| Значение | Когда используется |
|---|---|
| `generator` | `vegeta` или локальный TCP/FD stack сообщил `bind/can't assign requested address`, `too many open files`, connection allocation errors, либо фактическая подача сильно отстала от requested rate при ошибках генератора. Итоговый статус — `invalid_generator_limit`, такой результат не считается пределом backend. |
| `postgres` | Высокая latency/error rate совпала с DB-сигналами: PostgreSQL connection-limit ошибки, wait events, либо насыщение backend `pgxpool`. |
| `backend` | p95/timeout/error rate растёт без generator errors и без PostgreSQL saturation signals. |
| `unknown` | Недостаточно сигналов, чтобы честно отличить backend от PostgreSQL или генератора. |
| `none` | Probe прошёл SLO. |

PostgreSQL diagnostics используют `DATABASE_URL`, если он есть в окружении запуска; иначе берётся локальный perf URL `postgres://arenda:arenda@localhost:5434/arenda?sslmode=disable`. Backend pool diagnostics читаются из local-only endpoint `/internal/perf/db-pool`, который включается только для `APP_ENV=local`. Это диагностика тестируемого приложения, не настройка нагрузки.

Passing probe может иметь `bottleneck=none` и одновременно `pool_warning=true`. Это означает, что SLO ещё прошёл, но `pgxpool` уже показывал конкуренцию за соединения (`empty_acquire_count` или canceled acquires). Такой probe нельзя называть отказом, но его нужно учитывать при выборе pool profile.

## Отчёты

После каждого запуска создаётся директория вида:

```text
apps/backend/perf/results/run_YYYYMMDD_HHMMSS/
```

Внутри:

| Файл | Содержимое |
|---|---|
| `summary.json` | Итоги по endpoint'ам, все probes, `baseline_5000_probe`, `best_slo_pass`, `first_slo_fail_observed`, `first_generator_fail`, `unstable_boundary`, generator profile, host diagnostics, PostgreSQL diagnostics, top `pg_stat_statements`, backend pool diagnostics. Старые aliases `best_pass`, `first_slo_fail`, `last_good_rate` пока сохраняются для совместимости. |
| `probes.csv` | Плоская таблица всех probe: requested rate, vegeta rate, throughput, ratio, success, latency, wait, requests, pass, bottleneck, errors, PostgreSQL stats, top SQL marker, backend pool stats. |

Итоговая таблица в терминале показывает `endpoint`, `mode`, `status`, requested rate, vegeta rate, throughput, throughput ratio, `success`, `p95`, `wait`, `requests`, `pass`, `pool`, `bottleneck`, ошибки и причину классификации. Ниже печатаются маркеры `baseline_5000_probe`, `best_slo_pass`, `first_slo_fail_observed`, `first_generator_fail`, `unstable_boundary` и confirmation summary, чтобы успешная финальная точка не смешивалась с причиной ближайшего отказа.

`unstable_boundary=true` означает, что более низкий rate уже провалил SLO, но более высокий rate позже прошёл. Это сигнал шумного/зависимого от порядка прогона результата; используйте `sustainable` с confirmation attempts для подтверждения рабочей границы.

## Ограничения

- Покрыты только read endpoint'ы.
- Локальный результат зависит от ноутбука, loopback TCP stack и PostgreSQL на этой машине.
- Если итоговый `bottleneck=generator`, нужен более сильный или распределённый генератор; такой прогон не заявляет backend RPS limit.

## Очистка

```bash
make perf-db-down
```

Сгенерированные `perf/fixtures.json` и `perf/results/` игнорируются git'ом.
