# Deployment

Документ описывает stage/prod деплой: GitHub Actions собирает образы на
self-hosted runner'е, пушит в GHCR и деплоит на сервер по digest через SSH.
На сервере нет репозитория и сборки — только compose-файлы, env и бэкапы.
Caddy запущен на хосте и проксирует публичные домены на localhost-порты
контейнеров. Архитектурное решение — `docs/adr/0024-deploy-pipeline-ghcr-runner.md`,
дизайн — `docs/plans/2026-07-28-deploy-pipeline-redesign-design.md`.

## Поток деплоя

```
push в dev (stage) или main (prod)
  → ci.yml (lint, тесты, миграции up/down, сканеры — blocking)
  → сборка 4 образов (matrix, max-parallel: 2) → trivy → push в GHCR → cosign sign
  → _deploy.yml: cosign verify → рендер env из секрета ENV_FILE (со сверкой
    ключей против .env.<env>.example) → передача env на сервер через base64
    в envs ssh-шага → pg_dump-бэкап →
    миграции → up -d --wait → внешние smoke → точечная чистка старых образов
```

При падении раскатки или smoke — автоматический откат образов на предыдущие
(`.previous-images` в deploy-каталоге). БД автоматически НЕ откатывается —
см. разделы «Миграции» и «Rollback».

Workflow-файлы: `.github/workflows/{ci,security,_deploy,deploy-stage,deploy-prod}.yml`.
`security.yml` — это SAST (semgrep) и trivy-fs (ежедневный ночной прогон + non-blocking на PR);
уязвимости зависимостей закрыты Dependabot (`.github/dependabot.yml`: security
updates + сгруппированные weekly version updates в ветку `dev`) и govulncheck
в `ci.yml`.
Ручной деплой и откат — `workflow_dispatch` в `deploy-stage.yml` /
`deploy-prod.yml` с четырьмя digest-inputs.

## Окружения

- `dev` branch → stage: `/opt/arenda/stage`, compose project `arenda-stage`,
  GitHub Environment `stage` (deployment branch policy: только `dev`).
- `main` branch → prod: `/opt/arenda/prod`, compose project `arenda-prod`,
  GitHub Environment `production` (deployment branch policy: только `main`).
- Stage backend использует `APP_ENV=dev`, чтобы T-Kassa sandbox был допустим текущей валидацией конфигурации.
- Prod backend использует `APP_ENV=production`.
- Deploy-каталоги на сервере содержат только `docker-compose.<env>.yml`,
  `.env.<env>` (+ `.prev`), `.previous-images`, `.deploy-run-id`; репозитория
  и git-чекаута на сервере нет.
- Лендинг (`apps/landing`) слушает `127.0.0.1:13002` в prod и `127.0.0.1:23002` в stage; Caddy отдаёт его как fallback для `/`.
- При каждом деплое backend недоступен 5–15 секунд (один инстанс) — принято.

## DNS

Создать A/AAAA записи на IP сервера:

- `rentlee.ru`
- `www.rentlee.ru`
- `admin.rentlee.ru`
- `dev.rentlee.ru`
- `admin.dev.rentlee.ru`

## Server Bootstrap

Одноразовая подготовка нового сервера. Git-чекауты не нужны: deploy-каталоги
получают compose-файл через scp, а env — через base64 в envs ssh-шага пайплайна.

1. Установить Docker + compose plugin; установить и зарегистрировать
   self-hosted GitHub Actions runner (labels `self-hosted, linux, x64`) —
   runner на продовом VPS является принятым риском, компенсации см. в ADR 0024.
2. Создать deploy-пользователя и каталоги:

   ```bash
   mkdir -p /opt/arenda/stage /opt/arenda/prod /opt/arenda/backups/stage /opt/arenda/backups/prod
   chmod 700 /opt/arenda/backups /opt/arenda/backups/stage /opt/arenda/backups/prod
   ```

3. Сгенерировать SSH-ключ deploy-пользователя; приватный ключ — в секрет
   `SSH_PRIVATE_KEY` обоих GitHub Environments, публичный — в
   `~/.ssh/authorized_keys` deploy-пользователя. Снять fingerprint хоста
   (`ssh-keyscan <host> | ssh-keygen -lf -`) — в секрет
   `SSH_HOST_KEY_FINGERPRINT`.
4. Залогиниться в GHCR read-only токеном (тот же `GHCR_READ_TOKEN`, что в
   секретах): `echo "$TOKEN" | docker login ghcr.io -u <user> --password-stdin`.
5. Установить Caddy на хост, положить Caddyfile (ниже), настроить DNS.
6. Первый деплой выполняется самим пайплайном: он скопирует
   `docker-compose.<env>.yml` и отрендеренный `.env.<env>`, сделает бэкап
   (пустая БД — дамп маленький, это нормально только для самого первого
   запуска), применит миграции и поднимет стек.

## GitHub Environments и секреты

Два environment: `stage` и `production`. Набор секретов одинаковый, значения
свои.

| Секрет | Где | Назначение | Ротация |
| --- | --- | --- | --- |
| `ENV_FILE` | environment | Весь набор переменных окружения одним multiline-значением. Ключи должны точно совпадать с `.env.<env>.example` | При изменении любой переменной; заодно обновить example, иначе deploy упадёт на сверке ключей |
| `SSH_PRIVATE_KEY` | environment | Deploy-доступ на сервер (appleboy scp/ssh) | При смене ключа deploy-пользователя |
| `SSH_HOST` | environment | Хост VPS | При смене сервера |
| `SSH_USER` | environment | Deploy-пользователь | — |
| `SSH_HOST_KEY_FINGERPRINT` | environment | Проверка SSH host key | При пересоздании сервера; получить через `ssh-keyscan` |
| `GHCR_READ_TOKEN` | environment | `docker login ghcr.io` на сервере и `cosign verify` на runner'е | Fine-grained PAT, `read:packages` только этого репо, expiration 90 дней — перевыпускать заранее |
| `GHCR_USERNAME` | environment | Логин для GHCR | — |

Push в GHCR из workflow идёт под встроенным `GITHUB_TOKEN` (`packages: write`
только в джобе `images`), отдельный write-токен не нужен. Пакеты GHCR должны
оставаться приватными.

## Env: как это работает

- Источник истины по **набору ключей** — `.env.stage.example` /
  `.env.prod.example` в репозитории. Источник **значений** — секрет
  `ENV_FILE` соответствующего GitHub Environment.
- При деплое runner рендерит `ENV_FILE` в файл, сверяет множество ключей с
  example-файлом (несовпадение = fail, защита от drift'а), дописывает
  `*_IMAGE=...@sha256:...` и `service.version` в `OTEL_RESOURCE_ATTRIBUTES`,
  маскирует значения в логе и копирует файл на сервер (`chmod 600`, прежняя
  версия сохраняется как `.env.<env>.prev`).
- Ручные правки `.env.<env>` на сервере не предполагаются: следующий деплой
  их перезапишет. Изменение env = обновить секрет `ENV_FILE` (+ example при
  изменении набора ключей) и задеплоить.
- `AUTO_MIGRATE` задаётся в compose (`environment:`), а не в env-файле: в
  stage/prod он `false`, миграции выполняет отдельный шаг деплоя.

Требования к значениям (перенесены из старой схемы, актуальны):

- Пароль в `DATABASE_URL` должен совпадать с `POSTGRES_PASSWORD`.
- Для production `APP_BASE_URL` = `https://rentlee.ru`, `T_KASSA_BASE_URL` =
  `https://securepay.tinkoff.ru/v2/`. При необходимости prod можно временно
  направить на тестовую среду T-Kassa (`https://rest-api-test.tinkoff.ru/v2/`),
  но перед реальным трафиком вернуться на боевой URL.
- Для production-терминала T-Bank обязательно указать URL уведомлений — на него
  приходят вебхуки платежей и привязки карт (`AddCard` не поддерживает per-request
  `NotificationURL`): `https://rentlee.ru/webhooks/payment/tkassa`.
  Redirect-URL (`SuccessURL`/`FailURL` для платежей, `RedirectUrl`/`FailRedirectUrl`
  для `AddCard`) backend передаёт в T-Bank динамически из `APP_BASE_URL` в каждом
  запросе — это основной путь, настройка return URL в терминале не требуется.
  Для `AddCard` redirect-поля находятся вне официальной схемы API и исключены из
  подписи токена (prod-инцидент, error 204). Документированный fallback —
  настройка Success/Fail Add Card URL в параметрах терминала.
- Если `REGRU_S3_PUBLIC_BASE_URL` указывает на `https://cdn.rentlee.ru`
  (актуально только при `PHOTO_STORAGE_PROVIDER=s3`), этот DNS/публичный URL
  должен быть настроен до запуска backend; иначе указать рабочий публичный URL
  REG.RU S3.
- `PHOTO_STORAGE_PROVIDER=s3` включает REG.RU S3 и требует заполненные
  `REGRU_S3_*` значения. `REGRU_S3_*` также используются пайплайном для
  off-site копии дампов БД (endpoint/bucket/access/secret key).

## Миграции и бэкапы

- Перед каждой раскаткой deploy-скрипт делает дамп:
  `docker exec arenda-<env>-postgres pg_dump -U $POSTGRES_USER $POSTGRES_DB | gzip`
  в `/opt/arenda/backups/<env>/<timestamp>.sql.gz`. Архив проверяется
  (`gzip -t` + размер), хранятся последние 10 дампов, копия выгружается в
  REG.RU S3 (`s3://<bucket>/backups/<env>/`, падение выгрузки — warning, деплой
  продолжается). Каталоги бэкапов — `chmod 700`.
- Миграции выполняются отдельным compose-сервисом (`profiles: ["migrate"]`,
  команда `./arenda-api migrate`, общий env_file) после бэкапа и до раскатки.
  Падение миграции останавливает деплой — старая версия продолжает работать.
- `AUTO_MIGRATE=false` в stage/prod: backend при старте схему не трогает.
- Политика миграций — **expand-contract** (ADR 0024): миграция должна быть
  совместима с предыдущей версией кода; destructive-изменения — двумя
  релизами; `CREATE INDEX CONCURRENTLY` на больших таблицах. Авто-rollback
  никогда не делает `migrate down`; `down` прогоняется только в CI на
  эфемерной БД. Контроль — чеклист в `.github/PULL_REQUEST_TEMPLATE.md`.
- Down-миграция `000086_property_deletion_detach` **необратима после первого
  реального удаления объекта в режиме detach**: она выполняет `SET NOT NULL`
  на `property_id` в `leases`, `operations` и `recurring_operations` и упадёт,
  пока в БД есть отвязанные строки (`property_id IS NULL`). Ручной откат за
  пределы 000086 возможен только после ручной чистки таких строк
  (перепривязка или удаление). Стандартный rollback это не ломает:
  автоматический откат `migrate down` не выполняет никогда, а откат БД —
  только restore из дампа (см. ниже).

### Restore из дампа

Ручная процедура. Пример для stage; для prod заменить `ENV=prod`.

```bash
ENV=stage
cd /opt/arenda/$ENV
POSTGRES_USER=$(grep '^POSTGRES_USER=' .env.$ENV | cut -d= -f2-)
POSTGRES_DB=$(grep '^POSTGRES_DB=' .env.$ENV | cut -d= -f2-)
DUMP=/opt/arenda/backups/$ENV/<timestamp>.sql.gz   # или файл, скачанный из S3

# 1. Остановить backend, чтобы никто не писал в БД.
docker compose --env-file .env.$ENV -f docker-compose.$ENV.yml stop backend

# 2. Пересоздать базу (дамп plain-формата не содержит DROP).
docker exec arenda-$ENV-postgres psql -U "$POSTGRES_USER" -d postgres \
  -c "DROP DATABASE $POSTGRES_DB;" \
  -c "CREATE DATABASE $POSTGRES_DB OWNER $POSTGRES_USER;"

# 3. Восстановить дамп.
gunzip -c "$DUMP" | docker exec -i arenda-$ENV-postgres \
  psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1

# 4. Запустить стек.
docker compose --env-file .env.$ENV -f docker-compose.$ENV.yml up -d --wait

# 5. Smoke checks (см. ниже).
```

Процедуру нужно порепетировать на stage (восстановить дамп в отдельную базу и
проверить целостность) до того, как она понадобится в бою.

## Rollback

Откат кода — `workflow_dispatch` workflow `Deploy Stage` / `Deploy Prod` с
четырьмя digest-inputs (`backend_image`, `frontend_image`, `admin_image`,
`landing_image`) предыдущего релиза. Формат:
`ghcr.io/devnumbers/arenda-planform-<service>@sha256:<64 hex>`.

Где взять digest'ы:

- в логе прошлого успешного рана (джоба `Build & push <service>`, шаг
  «Resolve and validate digest») или в его артефактах `digest-<service>`
  (retention 7 дней);
- через GHCR UI (packages репозитория);
- локально: `docker buildx imagetools inspect ghcr.io/devnumbers/arenda-planform-<service>:<sha>`.

Ручной dispatch пропускает сборку и деплоит указанные digest'ы по той же
цепочке (verify → env → бэкап → миграции → раскатка → smoke). Миграции при
откате уже применены (схема новее кода) — поэтому expand-contract
обязателен; откат БД — только restore из дампа вручную.

Автоматический rollback внутри деплоя (падение раскатки или smoke) откатывает
только образы по `.previous-images` и явно пишет в лог, что БД не
откачена.

При самом первом деплое по новой схеме `.previous-images` пуст:
автоматический откат пропускается с предупреждением, а откат на build-образы
старой схемы невозможен — их нет в GHCR. До первого зелёного деплоя серверные
`.env.<env>`-файлы старой схемы держать как fallback, после — удалить вместе
с git-чекаутом.

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

## Smoke Checks

Эти же проверки (плюс сверка `version` в `/api/healthz` с sha коммита)
выполняет deploy-джоба снаружи; вручную их можно повторить так:

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
через `handle_path`. Поле `version` в ответе равно sha задеплоенного коммита
(при ручном dispatch — `manual`). `/api/me` без cookie должен отвечать `401`.

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
  убирать: хост общий, лимиты изолируют OOM внутри cgroup вместо kernel OOM
  killer по всему хосту. Сборка образов идёт на том же хосте (runner) —
  поэтому в workflow `max-parallel: 2` у matrix-сборок.
- Все runtime images имеют Dockerfile `HEALTHCHECK`; compose healthchecks
  остаются как orchestration checks для `depends_on` и для `up -d --wait`.
- Runtime base images закреплены по digest (build-стадии `node:24-alpine`
  намеренно по тегу). При обновлении базового образа сначала
  проверить новый digest через `docker buildx imagetools inspect <image>:<tag>`,
  затем обновить Dockerfiles/compose и повторить полный build/test.
- Чистка старых образов на сервере — только точечная из deploy-скрипта
  (наши GHCR-образы, current+previous сохраняются). Никаких
  `docker image prune -f` / `docker system prune`: на хосте чужие проекты.
- Deploy-каталоги `/opt/arenda/{stage,prod}` не содержат репозитория; всё
  лишнее туда не складывать — каталоги перезаписываются пайплайном.

## Observability

Централизованная наблюдаемость stage/prod (логи, трейсы, метрики, алерты)
работает на Uptrace self-hosted. Стек выделен в отдельный репозиторий —
[devnumbers/observability](https://github.com/devnumbers/observability) —
и разворачивается на сервере из `/opt/observability` (compose-проект и
сеть `observability`). Установка, настройка, алерты и процедуры
обслуживания описаны в README того репозитория.

Как проект подключён к стеку (потребительская сторона):

- Backend обоих окружений подключается к внешней docker-сети
  `observability` и отправляет трейсы и метрики по OTLP на
  `http://uptrace:14317` (порт наружу не публикуется).
- Блок `OTEL_*` (`OTEL_TRACES_EXPORTER`, `OTEL_METRICS_EXPORTER`,
  `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_HEADERS`,
  `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_TRACES_SAMPLER_ARG`) входит в
  `ENV_FILE` каждого окружения. При каждом деплое пайплайн обновляет
  `service.version` в `OTEL_RESOURCE_ATTRIBUTES` — это deployment marker
  в Uptrace.
- Логи: slog stdout остаётся logging source of truth; Vector собирает
  stdout контейнеров, access-логи Caddy и лог healthcheck-cron и шлёт в
  Uptrace. Локальный json-file driver и `docker logs` сохраняются.
- Healthcheck-cron (`tools/healthcheck/healthcheck.sh`) пишет JSON при
  сбое в `/var/log/arenda/healthcheck.log`, откуда Vector забирает запись.

Решение об observability зафиксировано в
`docs/adr/0021-centralized-observability-uptrace.md` (superseded —
перенесено в devnumbers/observability, ADR 0001).

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

## Notes

- Все сервисные порты в compose привязаны к `127.0.0.1`; Postgres не публикуется наружу.
- Бэкапы БД делаются при каждом деплое (см. «Миграции и бэкапы») — это
  событийные дампы, а не расписание; отдельный cron для регулярных бэкапов
  пока не настроен.
- Шифрование дампов (age) и выделенный DDL-пользователь для миграций —
  отложено, см. ADR 0024.
