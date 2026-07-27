# Deployment

Документ описывает stage/prod деплой через Docker Compose на сервере, где Caddy
запущен на хосте и проксирует публичные домены на localhost-порты контейнеров.

## Окружения

- `dev` branch -> stage: `/opt/arenda/stage`, compose project `arenda-stage`.
- `main` branch -> prod: `/opt/arenda/prod`, compose project `arenda-prod`.
- Stage backend использует `APP_ENV=dev`, чтобы T-Kassa sandbox был допустим текущей валидацией конфигурации.
- Prod backend использует `APP_ENV=production`.
- Stage/prod директории на сервере принадлежат `root:root`; деплой выполняется под `root`.
- Настоящие `.env.stage` и `.env.prod` хранятся только на сервере с правами `600`.
- Лендинг (`apps/landing`) слушает `127.0.0.1:13002` в prod и `127.0.0.1:23002` в stage; Caddy отдаёт его как fallback для `/`.

## DNS

Создать A/AAAA записи на IP сервера:

- `rentlee.ru`
- `www.rentlee.ru`
- `admin.rentlee.ru`
- `dev.rentlee.ru`
- `admin.dev.rentlee.ru`
- `logs.rentlee.ru`

## Server Bootstrap

```bash
mkdir -p /opt/arenda/stage /opt/arenda/prod
chown -R root:root /opt/arenda

cd /opt/arenda/stage
git clone git@github.com:devnumbers/arenda-platform.git .
cp .env.stage.example .env.stage
chmod 600 .env.stage

cd /opt/arenda/prod
git clone git@github.com:devnumbers/arenda-platform.git .
cp .env.prod.example .env.prod
chmod 600 .env.prod
```

В `.env.stage` и `.env.prod` заменить все `replace-*` значения. Пароль в
`DATABASE_URL` должен совпадать с `POSTGRES_PASSWORD`. Для production
`APP_BASE_URL` должен быть `https://rentlee.ru`, а `T_KASSA_BASE_URL` -
`https://securepay.tinkoff.ru/v2/`.

При необходимости production можно временно направить на тестовую среду T-Kassa
(`https://rest-api-test.tinkoff.ru/v2/`), но перед реальным трафиком нужно
вернуться на `https://securepay.tinkoff.ru/v2/`.

Для production-терминала T-Bank обязательно указать URL уведомлений — на него
приходят вебхуки платежей и привязки карт (`AddCard` не поддерживает per-request
`NotificationURL`):

```text
https://rentlee.ru/webhooks/payment/tkassa
```

Redirect-URL (`SuccessURL`/`FailURL` для платежей, `RedirectUrl`/`FailRedirectUrl`
для `AddCard`) backend передаёт в T-Bank динамически из `APP_BASE_URL` в каждом
запросе — это основной путь, настройка return URL в терминале не требуется.
Для `AddCard` redirect-поля находятся вне официальной схемы API и исключены из
подписи токена (prod-инцидент, error 204). Документированный fallback —
настройка Success/Fail Add Card URL в параметрах терминала. Если `REGRU_S3_PUBLIC_BASE_URL` указывает
на `https://cdn.rentlee.ru`, этот DNS/публичный URL должен быть настроен до
запуска backend; иначе указать рабочий публичный URL REG.RU S3.

`PHOTO_STORAGE_PROVIDER=s3` включает REG.RU S3 и требует заполненные
`REGRU_S3_*` значения. Для временного production-запуска без готового S3 можно
поставить `PHOTO_STORAGE_PROVIDER=fake`: backend запустится без проверки бакета,
но загрузку фотографий объектов в таком режиме использовать нельзя.

## Caddyfile

```caddyfile
www.rentlee.ru {
	redir https://rentlee.ru{uri} permanent
}

rentlee.ru {
	encode zstd gzip

	handle_path /api/* {
		reverse_proxy 127.0.0.1:18080
	}

	handle /webhooks/* {
		reverse_proxy 127.0.0.1:18080
	}

	@frontend path /login* /dashboard* /properties* /leases* /tenants* /finance* /profile* /subscription* /support* /ui-kit* /_next/* /fonts/* /images/* /file.svg /globe.svg /next.svg /vercel.svg /window.svg /icon.png
	handle @frontend {
		reverse_proxy 127.0.0.1:13000
	}

	handle {
		reverse_proxy 127.0.0.1:13002
	}
}

admin.rentlee.ru {
	encode zstd gzip

	handle_path /api/* {
		reverse_proxy 127.0.0.1:18080
	}

	handle {
		reverse_proxy 127.0.0.1:13001
	}
}

dev.rentlee.ru {
	encode zstd gzip

	handle_path /api/* {
		reverse_proxy 127.0.0.1:28080
	}

	handle /webhooks/* {
		reverse_proxy 127.0.0.1:28080
	}

	@frontend path /login* /dashboard* /properties* /leases* /tenants* /finance* /profile* /subscription* /support* /ui-kit* /_next/* /fonts/* /images/* /file.svg /globe.svg /next.svg /vercel.svg /window.svg /icon.png
	handle @frontend {
		reverse_proxy 127.0.0.1:23000
	}

	handle {
		reverse_proxy 127.0.0.1:23002
	}
}

admin.dev.rentlee.ru {
	encode zstd gzip

	handle_path /api/* {
		reverse_proxy 127.0.0.1:28080
	}

	handle {
		reverse_proxy 127.0.0.1:23001
	}
}
```

При добавлении нового top-level роута в Next.js-фронт его нужно добавить в
`@frontend path` и перечитать Caddy: `sudo caddy validate --config /etc/caddy/Caddyfile && sudo systemctl reload caddy`.

Проверка и применение:

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl reload caddy
```

## Manual Deploy

Stage:

```bash
cd /opt/arenda/stage
DEPLOY_SHA=<sha>
git fetch --prune origin +refs/heads/dev:refs/remotes/origin/dev
git checkout --detach --force "$DEPLOY_SHA"
git reset --hard "$DEPLOY_SHA"
git clean -ffdx -e .env.stage
test "$(git rev-parse HEAD)" = "$DEPLOY_SHA"
docker compose -f docker-compose.stage.yml config
docker compose -f docker-compose.stage.yml up -d --build --remove-orphans
curl -fsS http://127.0.0.1:28080/healthz | grep -q '"status":"ok"'
curl -fsS http://127.0.0.1:23000/login >/dev/null
test "$(curl -fsS http://127.0.0.1:23002/healthz)" = "ok"
test "$(curl -fsS http://127.0.0.1:23001/healthz)" = "ok"
```

Prod:

```bash
cd /opt/arenda/prod
DEPLOY_SHA=<sha>
git fetch --prune origin +refs/heads/main:refs/remotes/origin/main
git checkout --detach --force "$DEPLOY_SHA"
git reset --hard "$DEPLOY_SHA"
git clean -ffdx -e .env.prod
test "$(git rev-parse HEAD)" = "$DEPLOY_SHA"
docker compose -f docker-compose.prod.yml config
docker compose -f docker-compose.prod.yml up -d --build --remove-orphans
curl -fsS http://127.0.0.1:18080/healthz | grep -q '"status":"ok"'
curl -fsS http://127.0.0.1:13000/login >/dev/null
test "$(curl -fsS http://127.0.0.1:13002/healthz)" = "ok"
test "$(curl -fsS http://127.0.0.1:13001/healthz)" = "ok"
```

## Smoke Checks

```bash
curl -fsS https://dev.rentlee.ru/ | grep -q 'id="root"'
curl -fsS https://dev.rentlee.ru/login | grep -qi '<html'
curl -fsS https://admin.dev.rentlee.ru/ | grep -q 'id="root"'
test "$(curl -fsS https://admin.dev.rentlee.ru/healthz)" = "ok"
curl -fsS https://dev.rentlee.ru/api/healthz | grep -q '"status":"ok"'
test "$(curl -sS -o /tmp/arenda-stage-me.out -w '%{http_code}' https://dev.rentlee.ru/api/me)" = "401"

curl -fsS https://rentlee.ru/ | grep -q 'id="root"'
curl -fsS https://rentlee.ru/login | grep -qi '<html'
curl -fsS https://admin.rentlee.ru/ | grep -q 'id="root"'
test "$(curl -fsS https://admin.rentlee.ru/healthz)" = "ok"
curl -fsS https://rentlee.ru/api/healthz | grep -q '"status":"ok"'
test "$(curl -sS -o /tmp/arenda-prod-me.out -w '%{http_code}' https://rentlee.ru/api/me)" = "401"
case "$(curl -sS -o /tmp/arenda-www.out -w '%{http_code} %{redirect_url}' https://www.rentlee.ru/)" in
  30[18]\ https://rentlee.ru*) ;;
  *) cat /tmp/arenda-www.out; exit 1 ;;
esac
```

`/api/healthz` подтверждает, что Caddy попал в backend и `/api` был снят
через `handle_path`. `/api/me` без cookie должен отвечать `401`.

## Runtime Guards

- Stage/prod compose ограничивает CPU/RAM на уровне сервисов и включает
  ротацию Docker JSON-логов: `max-size=10m`, `max-file=5`.
- PostgreSQL в stage/prod явно настроен под `mem_limit` контейнера через
  аргументы `command` в compose: `shared_buffers` = 25% лимита (prod 192MB /
  stage 96MB), `effective_cache_size` ≈ 2/3 лимита (prod 512MB / stage 256MB),
  `jit=off`, `max_parallel_workers_per_gather=0` (контейнер ограничен 1 CPU).
  PostgreSQL не масштабируется автоматически ни под RAM хоста, ни под лимит
  cgroup: при изменении `mem_limit` или апгрейде сервера параметры
  пересчитываются по этой формуле и правятся в compose вручную. Лимиты не
  убирать: хост общий (5.8GB RAM, swap=0, ~30 контейнеров), лимиты изолируют
  OOM внутри cgroup вместо kernel OOM killer по всему хосту.
- Все runtime images имеют Dockerfile `HEALTHCHECK`; compose healthchecks
  остаются как orchestration checks для `depends_on`.
- Runtime base images закреплены по digest (build-стадии `node:24-alpine`
  намеренно по тегу). При обновлении базового образа сначала
  проверить новый digest через `docker buildx imagetools inspect <image>:<tag>`,
  затем обновить Dockerfiles/compose и повторить полный build/test.
- `git clean -ffdx` в deploy директории удаляет любой drift checkout-а. В этих
  директориях должны храниться только файлы репозитория и целевой `.env.*`.

## Observability

Централизованная наблюдаемость stage/prod (логи, трейсы, метрики, алерты)
работает на Uptrace self-hosted на этом же сервере; решение зафиксировано в
`docs/adr/0021-centralized-observability-uptrace.md`. Здесь — краткий обзор;
полная инструкция по установке и настройке — `observability/README.md`.

- Стек живёт в отдельном compose-проекте `arenda-obs` (`docker-compose.obs.yml`
  + каталог `observability/` в репо) и разворачивается из `/opt/arenda/obs` —
  вне stage/prod checkout'ов, которые чистятся `git clean -ffdx`. Сервисы:
  ClickHouse, PostgreSQL, Uptrace, Redis, Vector, OTel Collector; суммарный
  лимит RAM ~2.6 ГБ, все порты привязаны к `127.0.0.1`.
- Что собирается: stdout всех контейнеров (структурированные slog-логи backend
  остаются logging source of truth, локальный json-file driver и `docker logs`
  сохраняются), access-логи Caddy (`/var/log/caddy/access.log`), лог сбоев
  healthcheck-cron (`/var/log/arenda/healthcheck.log`), JS-ошибки браузера
  (frontend/admin/landing шлют их в backend на публичный `POST /client-errors`,
  backend логирует в stdout), OTel-трейсы и метрики backend обоих окружений,
  метрики хоста и контейнеров (OTel Collector: hostmetrics + docker_stats).
- Backend подключается к внешней docker-сети `arenda-obs` и отправляет
  трейсы/метрики по OTLP на `http://uptrace:14317` (порт наружу не
  публикуется). В `.env.stage`/`.env.prod` добавляется блок `OTEL_*`
  (`OTEL_SERVICE_NAME`, `OTEL_TRACES_EXPORTER`, `OTEL_METRICS_EXPORTER`,
  `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`,
  `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_TRACES_SAMPLER_ARG`).
- UI: `https://logs.rentlee.ru` — отдельный блок в серверном Caddyfile:
  `basic_auth` (bcrypt-хэш, учётка хранится только в Caddyfile на сервере) +
  `reverse_proxy 127.0.0.1:14318`. DNS A-запись добавляет владелец. Запасной
  доступ — SSH-туннель `ssh -L 14318:127.0.0.1:14318 <server>`.
- Алерты на email отправляются через SMTP (секция `mailer.smtp` в
  `observability/uptrace.yml`, креды — в `.env.obs`); канал и получатели
  настраиваются в UI Uptrace (Alerting → Channels), в репо не хранятся.

Команды на сервере в `/opt/arenda/obs` (Makefile туда не копируется — прямые
вызовы compose, как в `observability/README.md`):

```bash
cd /opt/arenda/obs
docker compose -f docker-compose.obs.yml --env-file .env.obs up -d    # поднять стек
docker compose -f docker-compose.obs.yml --env-file .env.obs down     # остановить стек
docker compose -f docker-compose.obs.yml --env-file .env.obs ps       # статус сервисов
docker compose -f docker-compose.obs.yml --env-file .env.obs logs -f  # логи стека
```

Те же действия доступны как make-таргеты (`make obs-up`, `make obs-down`,
`make obs-ps`, `make obs-logs`) из любого checkout'а репозитория, где рядом
с `docker-compose.obs.yml` есть `.env.obs`.

Smoke checks после выкатки стека (на сервере, из `/opt/arenda/obs`):

```bash
docker compose -f docker-compose.obs.yml --env-file .env.obs ps      # все сервисы healthy (otelcol — running, healthcheck у него нет)
curl -fsS -o /dev/null http://127.0.0.1:14318                        # UI Uptrace отвечает локально
test "$(curl -sS -o /dev/null -w '%{http_code}' https://logs.rentlee.ru)" = "401"  # basic_auth требует учётку
```

Дальше — визуально в UI: логи контейнеров смотрятся в разделе Traces
(/spans; отдельного /logs в Uptrace 2.0.3 нет) и фильтруются по атрибутам
`deployment_environment_name`/`service_name`, трейсы запросов появляются
после вызовов API, графики метрик хоста и контейнеров наполняются.

## T-Bank TLS Certificates

Backend image включает публичные CA-сертификаты, которые T-Bank требует для
серверных API-запросов к T-Kassa endpoint-ам:

- `Russian Trusted Root CA`
- `Russian Trusted Sub CA`

Официальные источники и checksum хранятся в
`apps/backend/certs/tbank/README.md`. Сертификаты добавляются в Docker image на
этапе сборки через `/usr/local/share/ca-certificates/` и `update-ca-certificates`;
на host вручную их ставить не нужно.

HARICA roots уже входят в текущий Debian `ca-certificates` runtime image. Если
будущая base image перестанет их содержать, добавить HARICA нужно тем же
механизмом из официального T-Bank bundle.

Проверка TLS из backend image:

```bash
docker buildx build --load -f apps/backend/Dockerfile -t arenda-backend:tkassa-ca .
docker run --rm --entrypoint curl arenda-backend:tkassa-ca -Iv https://rest-api-test.tinkoff.ru/v2/Init
```

Для stage T-Bank также требует добавить IP сервера `80.78.254.79` в белый список
тестовой среды для URL `rest-api-test.tinkoff.ru`.

## Rollback

```bash
cd /opt/arenda/<stage-or-prod>
DEPLOY_SHA=<previous-sha>
git fetch --prune origin
git checkout --detach --force "$DEPLOY_SHA"
git reset --hard "$DEPLOY_SHA"
git clean -ffdx -e .env.stage # для prod заменить на -e .env.prod
test "$(git rev-parse HEAD)" = "$DEPLOY_SHA"
docker compose -f docker-compose.<stage-or-prod>.yml up -d --build --remove-orphans
```

После rollback повторить smoke checks для нужного окружения.

## Notes

- Все сервисные порты в compose привязаны к `127.0.0.1`; Postgres не публикуется наружу.
- `SMS_SENDER=disabled` безопасен для stage/prod и не логирует SMS-коды. Старые SMS-сценарии смены телефона будут недоступны до подключения реального SMS-провайдера.
- Перед реальным production traffic нужны отдельные резервные копии Postgres и S3; это не входит в текущую compose-конфигурацию.
