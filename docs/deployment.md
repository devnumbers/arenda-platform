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

Для production-терминала T-Bank указать URL уведомлений:

```text
https://rentlee.ru/webhooks/payment/tkassa
```

`SuccessURL` и `FailURL` backend передаёт в T-Bank динамически из
`APP_BASE_URL` для каждого платежа. Если `REGRU_S3_PUBLIC_BASE_URL` указывает
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
- Все runtime images имеют Dockerfile `HEALTHCHECK`; compose healthchecks
  остаются как orchestration checks для `depends_on`.
- Runtime base images закреплены по digest (build-стадии `node:24-alpine`
  намеренно по тегу). При обновлении базового образа сначала
  проверить новый digest через `docker buildx imagetools inspect <image>:<tag>`,
  затем обновить Dockerfiles/compose и повторить полный build/test.
- `git clean -ffdx` в deploy директории удаляет любой drift checkout-а. В этих
  директориях должны храниться только файлы репозитория и целевой `.env.*`.

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
