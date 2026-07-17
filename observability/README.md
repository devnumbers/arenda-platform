# Observability: Uptrace self-hosted

Централизованная наблюдаемость Arenda Platform на production VPS: логи всех
контейнеров, access-логи Caddy, трейсы и метрики backend, метрики хоста и
контейнеров, алерты в Telegram. Решение зафиксировано в ADR:
`docs/adr/0021-centralized-observability-uptrace.md`.

Стек (compose-проект `arenda-obs`, `docker-compose.obs.yml`):

| Сервис      | Образ                                        | Назначение                          | Лимиты        |
|-------------|----------------------------------------------|-------------------------------------|---------------|
| `clickhouse`| clickhouse/clickhouse-server:25.8.28.1       | телеметрия (спаны, логи, метрики)   | 1g / 1.0 cpu  |
| `uptrace-pg`| postgres:18                                  | метаданные Uptrace                  | 256m / 0.5    |
| `redis`     | redis:8.2.7-alpine                           | кэш Uptrace (обязателен для 2.0.3)  | 96m / 0.25    |
| `uptrace`   | uptrace/uptrace:2.0.3                        | UI :14318 + OTLP :14317             | 384m / 0.5    |
| `vector`    | timberio/vector:0.57.0-alpine                | сбор и отправка логов               | 128m / 0.25   |
| `otelcol`   | otel/opentelemetry-collector-contrib:0.156.0 | метрики хоста и контейнеров         | 256m / 0.25   |

Потоки данных:

- логи всех контейнеров (stdout, slog JSON) → Vector (`docker_logs`) → Uptrace;
- access-логи Caddy (`/var/log/caddy/access.log`, JSON) → Vector → Uptrace;
- сбои healthcheck-cron (`/var/log/arenda/healthcheck.log`) → Vector → Uptrace;
- трейсы и метрики backend (stage+prod) → OTLP/gRPC `uptrace:14317` → Uptrace;
- метрики хоста и docker stats → otelcol → OTLP `uptrace:14317` → Uptrace.

Атрибуты в Uptrace 2.0.3: поля `.env`/`.service` из Vector сохраняются как
`deployment_environment_name`/`service_name` — логи фильтруются по
`deployment_environment_name = "stage"|"prod"|"obs"`. Трейсы и метрики
backend несут OTel-атрибут `deployment_environment` со значениями
`stage|production` (см. `.env.stage.example` / `.env.prod.example`) — имена и
значения атрибутов у логов и трейсов различаются, учитывайте это в фильтрах
и мониторах.

Слепая зона: логи короткоживущих (одноразовых) контейнеров Vector
`docker_logs` может терять или дублировать — известное ограничение
источника; не используйте его как аудит one-shot джоб.

UI доступен только на `127.0.0.1:14318`; наружу — через Caddy
`logs.rentlee.ru` (basic auth). OTLP/gRPC `:14317` не публикуется наружу и
доступен только внутри docker-сети `arenda-obs` (в неё добавлен backend
обоих окружений).

## Предусловия

- Тот же VPS, где развёрнуты stage и prod (`/opt/arenda/stage`,
  `/opt/arenda/prod`).
- +2 ГБ свободной RAM под стек (лимиты суммарно ~2.1 ГБ).
- Docker + Docker Compose v2.
- DNS A-запись `logs.rentlee.ru` → IP сервера (добавляет владелец).

## Установка

Все команды выполняются на сервере.

1. Создать каталоги:

   ```bash
   sudo mkdir -p /opt/arenda/obs /var/log/arenda /var/log/caddy
   ```

2. Скопировать файлы стека из репозитория в `/opt/arenda/obs`
   (каталог вне stage/prod checkout'ов — они чистятся `git clean -ffdx`):

   ```bash
   cd /opt/arenda/obs
   # из свежего clone/архива репозитория:
   cp docker-compose.obs.yml /opt/arenda/obs/
   cp -r observability /opt/arenda/obs/
   cp .env.obs.example /opt/arenda/obs/.env.obs
   ```

3. Подготовить конфиг Uptrace (править файл не нужно — секреты подставляются
   из `.env.obs`):

   ```bash
   cp observability/uptrace.yml.example observability/uptrace.yml
   ```

4. Заполнить `/opt/arenda/obs/.env.obs` (схема и генераторы — в
   `.env.obs.example`): `UPTRACE_PG_PASSWORD`, `UPTRACE_CH_PASSWORD`,
   `UPTRACE_SECRET` (`openssl rand -hex 32`), `UPTRACE_ADMIN_PASSWORD`,
   `UPTRACE_SELF_TOKEN` и `UPTRACE_PROJECT_TOKEN` (`openssl rand -hex 16`).
   `UPTRACE_DSN` должен содержать значение `UPTRACE_PROJECT_TOKEN`:
   `http://<UPTRACE_PROJECT_TOKEN>@uptrace:14318?grpc=14317`.
   Файл не должен быть читаем посторонними: `chmod 600 .env.obs`.

5. Запустить стек (важно: **до** пересоздания stage/prod — они используют
   сеть `arenda-obs` как external):

   ```bash
   cd /opt/arenda/obs
   docker compose -f docker-compose.obs.yml --env-file .env.obs up -d
   docker compose -f docker-compose.obs.yml --env-file .env.obs ps
   ```

   Миграции PostgreSQL и ClickHouse выполняются автоматически при старте
   `uptrace` (entrypoint ждёт обе БД). Отдельный шаг миграций не нужен.

   Те же команды доступны как make-таргеты из любого checkout'а репозитория,
   где есть `.env.obs` рядом с `docker-compose.obs.yml`: `make obs-up`,
   `make obs-down`, `make obs-ps`, `make obs-logs`.

## Первый вход и DSN

- UI: `https://logs.rentlee.ru` (после настройки Caddy, см. ниже) или
  `http://127.0.0.1:14318` на самом сервере.
- Логин: `admin@rentlee.ru`, пароль: значение `UPTRACE_ADMIN_PASSWORD`.
  Пользователь и проекты создаются из `observability/uptrace.yml` при первом
  старте (`projects`, `auth.users`). Смените пароль в UI после первого входа.
- Проекты: `Arenda` (id 1, все данные платформы) и `Uptrace` (id 2,
  самомониторинг) — id выдаются seed'ом в алфавитном порядке ключей, а не в
  порядке списка из `uptrace.yml`; окружения в трейсах/метриках разделены
  атрибутом `deployment_environment`.
- DSN проекта «Arenda» уже определён конфигом:
  `http://<UPTRACE_PROJECT_TOKEN>@uptrace:14318?grpc=14317` (= `UPTRACE_DSN`
  из `.env.obs`). Его же видно в UI: проект Arenda → Settings → DSN.
- Куда подставляется DSN:
  - `.env.obs` → `UPTRACE_DSN` (Vector и otelcol);
  - `/opt/arenda/stage/.env.stage` и `/opt/arenda/prod/.env.prod` →
    `OTEL_EXPORTER_OTLP_HEADERS=uptrace-dsn=<то же значение>` (см. ниже).

## Telegram-алерты

1. Создать бота: в Telegram открыть `@BotFather` → `/newbot` → получить
   токен вида `123456:ABC...`.
2. Добавить токен в `.env.obs`: `UPTRACE_TELEGRAM_BOT_TOKEN=<токен>` и
   пересоздать стек: `docker compose -f docker-compose.obs.yml --env-file .env.obs up -d`.
3. Узнать chat id: написать боту любое сообщение (или добавить его в группу),
   открыть `https://api.telegram.org/bot<ТОКЕН>/getUpdates` → `chat.id`
   (у групп — отрицательное число).
4. В UI Uptrace: **Alerting → Channels → New channel → Telegram**, указать
   chat id. Документация: https://uptrace.dev/features/alerting/notifications.html
5. Создать мониторы: **Alerting → Monitors → New monitor → From YAML**,
   привязать канал. Стартовый набор (синтаксис:
   https://uptrace.dev/features/alerting/metric-monitors.html,
   https://uptrace.dev/features/alerting/error-monitors.html):

   ```yaml
   # Доля упавших HTTP-запросов (5xx/panic) по трейсам backend
   monitors:
     - name: Failed HTTP requests
       type: metric
       metrics:
         - uptrace_tracing_spans as $spans
       query:
         - perMin(count($spans{_status_code="error"})) as failed_requests
         - where _type = "httpserver"
       detector:
         type: manual
         max_value: 5
       num_eval_points: 3
   ```

   ```yaml
   # Всплеск error-логов (любой сервис: backend, caddy, фронты)
   monitors:
     - name: Error logs spike
       type: metric
       metrics:
         - uptrace_tracing_logs as $logs
       query:
         - perMin(sum($logs))
         - where _system in ("log:error", "log:fatal")
       detector:
         type: manual
         max_value: 10
       num_eval_points: 3
   ```

   ```yaml
   # Паника backend (recovery-middleware пишет level=error со словом panic)
   monitors:
     - name: PanicDetected
       type: error
       query:
         - group by _group_id
         - where _system in ("log:error", "log:fatal")
         - where _display_name contains "panic"
   ```

   ```yaml
   # Недоступность stage/prod (cron healthcheck.sh)
   monitors:
     - name: HealthcheckFailed
       type: error
       query:
         - group by _group_id
         - where _system = "log:error"
         - where _display_name contains "healthcheck failed"
   ```

   ```yaml
   # Диск хоста > 85%
   monitors:
     - name: Filesystem usage
       type: metric
       metrics:
         - system_filesystem_usage as $fs_usage
       query:
         - $fs_usage{state='used'} / $fs_usage as fs_util
         - group by host_name, mountpoint
         - where mountpoint !~ "/snap"
       column:
         name: fs_util
         unit: utilization
       detector:
         type: manual
         max_value: 0.85
       num_eval_points: 3
   ```

   ```yaml
   # RAM хоста > 90%
   monitors:
     - name: Memory usage
       type: metric
       metrics:
         - system_memory_usage as $mem
       query:
         - $mem{state='used'} / $mem as mem_util
         - group by host_name
       column:
         name: mem_util
         unit: utilization
       detector:
         type: manual
         max_value: 0.9
       num_eval_points: 3
   ```

   ```yaml
   # Средняя загрузка CPU хоста
   monitors:
     - name: CPU usage
       type: metric
       metrics:
         - system_cpu_load_average_15m as $load_avg_15m
         - system_cpu_time as $cpu_time
       query:
         - $load_avg_15m / uniq($cpu_time.cpu) as cpu_util
         - group by host_name
       column:
         name: cpu_util
         unit: utilization
       detector:
         type: manual
         max_value: 3
       num_eval_points: 10
   ```

   ```yaml
   # Контейнер > 90% своего mem_limit
   # Точные имена метрик docker_stats смотрите в UI (Metrics explorer):
   # container.memory.usage.* -> container_memory_usage_*
   monitors:
     - name: Container memory limit
       type: metric
       metrics:
         - container_memory_usage_total as $mem_used
         - container_memory_usage_limit as $mem_limit
       query:
         - $mem_used / $mem_limit as container_mem_util
         - group by container_name
       column:
         name: container_mem_util
         unit: utilization
       detector:
         type: manual
         max_value: 0.9
       num_eval_points: 5
   ```

## Caddy: сайт logs.rentlee.ru и access-логи

В `/etc/caddy/Caddyfile` добавить сайт (Caddy v2; bcrypt-хэш пароля генерируется
командой `caddy hash-password`):

```caddyfile
logs.rentlee.ru {
	basic_auth {
		admin <bcrypt-хэш из `caddy hash-password`>
	}
	# HSTS: UI доступен только по HTTPS.
	header Strict-Transport-Security "max-age=31536000"
	reverse_proxy 127.0.0.1:14318
	log {
		output file /var/log/caddy/access.log {
			roll_size 100mb
			roll_keep 5
		}
		format json
	}
}
```

(в Caddy < 2.8 директива называется `basicauth`).

В **каждый** существующий сайт (`dev.rentlee.ru`, `admin.dev.rentlee.ru`,
`rentlee.ru`, `admin.rentlee.ru`) добавить тот же блок `log`, чтобы Vector
собирал access-логи всех окружений в один JSON-файл:

```caddyfile
	log {
		output file /var/log/caddy/access.log {
			roll_size 100mb
			roll_keep 5
		}
		format json
	}
```

Оговорка: все сайты пишут в один `/var/log/caddy/access.log`, и у каждого
блока `log` свой независимый roller одного и того же пути — при одновременной
ротации архивы могут путаться/перезаписываться. На наших объёмах это
некритично (актуальный хвост всегда в основном файле), но при желании можно
развести сайты по per-site файлам (`access-dev.log`, `access-prod.log`, ...)
и заменить в `observability/vector.yaml` include источника `caddy_access` на
glob `"/var/log/caddy/*.log"`.

Проверка и применение:

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
ls -l /var/log/caddy/access.log   # должен появиться после первых запросов
```

Пароль от basic auth хранится только в серверном Caddyfile (в репозиторий не
коммитить).

## Cron healthcheck

Проверка `/healthz` обоих backend и spider фронтов раз в минуту; сбой пишет
JSON-строку в `/var/log/arenda/healthcheck.log` (далее Vector → Uptrace →
монитор HealthcheckFailed):

```bash
sudo install -m 0755 /opt/arenda/obs/observability/healthcheck.sh /opt/arenda/obs/healthcheck.sh
( sudo crontab -l 2>/dev/null; echo '* * * * * /opt/arenda/obs/healthcheck.sh' ) | sudo crontab -
```

## Включение OTEL в stage/prod

В `/opt/arenda/stage/.env.stage` и `/opt/arenda/prod/.env.prod` скопировать
блок OpenTelemetry из `.env.stage.example` / `.env.prod.example` (уже вставлен
в example-файлы после блока логов) и подставить DSN:

```dotenv
OTEL_SERVICE_NAME=arenda-api
OTEL_TRACES_EXPORTER=otlp
OTEL_METRICS_EXPORTER=otlp
OTEL_EXPORTER_OTLP_ENDPOINT=http://uptrace:14317
OTEL_EXPORTER_OTLP_HEADERS=uptrace-dsn=<UPTRACE_DSN из .env.obs>
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=stage      # в prod: production
OTEL_TRACES_SAMPLER_ARG=1.0
```

Пересоздать окружения (сеть `arenda-obs` уже создана стеком):

```bash
cd /opt/arenda/stage && docker compose -f docker-compose.stage.yml up -d
cd /opt/arenda/prod  && docker compose -f docker-compose.prod.yml up -d
```

## Smoke checks

1. `docker compose -f docker-compose.obs.yml --env-file .env.obs ps` — все
   сервисы `healthy` (otelcol — `running`, healthcheck у него нет: образ
   distroless без shell; процесс под контролем restart-политики).
2. UI `https://logs.rentlee.ru` открывается, пускает под basic auth и логином
   `admin@rentlee.ru`.
3. **Logs** (в UI 2.0.3 логи живут в разделе **Traces**, путь `/spans`, а не
   `/logs`): логи backend с `deployment_environment_name=stage`/`prod`,
   `service_name=backend`; access-логи `service_name=caddy`; сбои
   healthcheck — `service_name=healthcheck`.
4. **Traces**: спаны `arenda-api` с `deployment_environment=stage|production`.
5. **Metrics**: `system_cpu_*`, `system_memory_*`, `system_filesystem_*`,
   `container_*` (auto-дашборды Uptrace создаются самостоятельно).
6. Тестовый алерт: остановить любой фронт (`docker stop <container>`) → через
   минуту в Telegram приходит HealthcheckFailed; вернуть контейнер —
   приходит recovery.

## Доступ к UI

- **Основной:** `https://logs.rentlee.ru` — basic auth (Caddy) + логин Uptrace.
- **Запасной (SSH-туннель):**
  `ssh -L 14318:127.0.0.1:14318 <server>` → `http://127.0.0.1:14318`.

## Troubleshooting

- **`clickhouse` не стартует / падает:** смотреть
  `docker compose ... logs clickhouse`. Частые причины: нехватка RAM
  (лимит 1g, хосту нужен запас), повреждённый volume после жёсткого ребута.
- **`uptrace` в restart-loop:** `logs uptrace`. Ошибки YAML в
  `observability/uptrace.yml` (отступы!), пустые `${...}` — не заполнен
  `.env.obs` или стек запущен без `--env-file .env.obs`. Ошибки auth к БД —
  пароли в `.env.obs` не совпадают с уже созданными в volume
  (при смене пароля: пересоздать volume или поменять пароль в самой БД).
- **Нет логов в UI** (раздел **Traces**, `/spans`): `logs vector` — ошибки
  sink'а видны сразу (401/403 = неверный `UPTRACE_DSN`; connect refused =
  `uptrace` не healthy). Проверить, что файл `/var/log/caddy/access.log`
  существует и читаем.
- **`400 ...` в `logs vector` при отправке в Uptrace** (батчи дропаются,
  логи не текут): две встреченные причины. (1) `"DSN does not have a token"` —
  в Uptrace уходит литерал `${UPTRACE_DSN}`: Vector ≥ 0.57 по умолчанию
  отключает интерполяцию env в конфиге; в репо она включена флагом
  `--dangerously-allow-env-var-interpolation` в команде сервиса vector
  (`docker-compose.obs.yml`). (2) `got content-type "", wanted
  "application/json"` — Vector с `framing.method: bytes` не отправляет
  Content-Type; в репо он задан явно (`content-type: application/json` в
  `request.headers` sink'а). В обоих случаях лечение — поднять стек из
  актуальных репо-файлов; учтите: `up -d` не перечитывает изменённый
  mount-конфиг — `up -d --force-recreate vector`.
- **Гигантские лог-строки / «poison pill» в disk buffer vector:** симптомы —
  рост RSS vector до OOM и restart loop (одна строка в 219 МБ уже клала
  стек). Профилактика уже в `observability/vector.yaml` (усечение
  `.message` до ~16k символов). Лечение зависшего буфера: остановить и
  удалить vector с wipe'ом volume, затем поднять заново (чекпоинты и
  ядовитый батч сбрасываются, возможны дубликаты хвоста):
  `docker compose -f docker-compose.obs.yml --env-file .env.obs rm -s vector &&
   docker volume rm arenda-obs_vector-data &&
   docker compose -f docker-compose.obs.yml --env-file .env.obs up -d vector`
- **Нет трейсов backend:** backend в сети `arenda-obs`
  (`docker network inspect arenda-obs`), в `.env.stage`/`.env.prod` верный
  `OTEL_EXPORTER_OTLP_HEADERS` (DSN обязан совпадать с `UPTRACE_DSN`),
  backend пересоздан после правки env.
- **Нет метрик хоста/контейнеров:** `logs otelcol`. `permission denied` по
  docker.sock — otelcol должен идти с `user: "0"` (по умолчанию так и есть)
  либо удалите `user` и добавьте `group_add: ["<GID docker-группы хоста>"]`
  (`getent group docker`).
- **Telegram молчит:** `UPTRACE_TELEGRAM_BOT_TOKEN` добавлен и стек
  пересоздан; chat id верный (у групп — с минусом); канал привязан к монитору.
- **Смена `ch_schema`/TTL:** требует `docker compose ... exec uptrace /uptrace ch reset`
  — удаляет все телеметрические данные (метаданные в PostgreSQL сохраняются).
