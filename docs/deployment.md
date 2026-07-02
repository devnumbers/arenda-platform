# Deployment

Документ описывает stage/prod деплой через Docker Compose на сервере, где Caddy
запущен на хосте и проксирует публичные домены на localhost-порты контейнеров.

## Окружения

- `dev` branch -> stage: `/opt/arenda/stage`, compose project `arenda-stage`.
- `master` branch -> prod: `/opt/arenda/prod`, compose project `arenda-prod`.
- Stage backend использует `APP_ENV=dev`, чтобы T-Kassa sandbox был допустим текущей валидацией конфигурации.
- Prod backend использует `APP_ENV=production`.
- Настоящие `.env.stage` и `.env.prod` хранятся только на сервере с правами `600`.

## DNS

Создать A/AAAA записи на IP сервера:

- `rentlee.ru`
- `www.rentlee.ru`
- `admin.rentlee.ru`
- `dev.rentlee.ru`
- `admin.dev.rentlee.ru`

## Server Bootstrap

```bash
sudo mkdir -p /opt/arenda/stage /opt/arenda/prod
sudo chown -R deploy:deploy /opt/arenda

cd /opt/arenda/stage
git clone git@github.com:<owner>/<repo>.git .
cp .env.stage.example .env.stage
chmod 600 .env.stage

cd /opt/arenda/prod
git clone git@github.com:<owner>/<repo>.git .
cp .env.prod.example .env.prod
chmod 600 .env.prod
```

В `.env.stage` и `.env.prod` заменить все `replace-*` значения. Пароль в
`DATABASE_URL` должен совпадать с `POSTGRES_PASSWORD`.

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

	handle {
		reverse_proxy 127.0.0.1:13000
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

	handle {
		reverse_proxy 127.0.0.1:23000
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
test "$(curl -fsS http://127.0.0.1:23001/healthz)" = "ok"
```

Prod:

```bash
cd /opt/arenda/prod
DEPLOY_SHA=<sha>
git fetch --prune origin +refs/heads/master:refs/remotes/origin/master
git checkout --detach --force "$DEPLOY_SHA"
git reset --hard "$DEPLOY_SHA"
git clean -ffdx -e .env.prod
test "$(git rev-parse HEAD)" = "$DEPLOY_SHA"
docker compose -f docker-compose.prod.yml config
docker compose -f docker-compose.prod.yml up -d --build --remove-orphans
curl -fsS http://127.0.0.1:18080/healthz | grep -q '"status":"ok"'
curl -fsS http://127.0.0.1:13000/login >/dev/null
test "$(curl -fsS http://127.0.0.1:13001/healthz)" = "ok"
```

## Smoke Checks

```bash
curl -fsS https://dev.rentlee.ru/login | grep -qi '<html'
curl -fsS https://admin.dev.rentlee.ru/ | grep -q 'id="root"'
test "$(curl -fsS https://admin.dev.rentlee.ru/healthz)" = "ok"
curl -fsS https://dev.rentlee.ru/api/healthz | grep -q '"status":"ok"'
test "$(curl -sS -o /tmp/arenda-stage-me.out -w '%{http_code}' https://dev.rentlee.ru/api/me)" = "401"

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
- Base images закреплены по digest. При обновлении базового образа сначала
  проверить новый digest через `docker buildx imagetools inspect <image>:<tag>`,
  затем обновить Dockerfiles/compose и повторить полный build/test.
- `git clean -ffdx` в deploy директории удаляет любой drift checkout-а. В этих
  директориях должны храниться только файлы репозитория и целевой `.env.*`.

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
