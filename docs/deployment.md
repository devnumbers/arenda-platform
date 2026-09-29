# Deployment

Документ описывает stage/prod деплой: GitHub Actions собирает образы на
GitHub-hosted раннерах (ADR 0045; раньше — self-hosted раннер на самом
сервере, списан), пушит в GHCR и деплоит на сервер по digest через SSH.
На сервере нет репозитория и сборки — только compose-файлы, env и бэкапы.
Caddy запущен на хосте и проксирует публичные домены на localhost-порты
контейнеров. Архитектурное решение — `docs/adr/0024-deploy-pipeline-ghcr-runner.md`
(размещение раннеров — `docs/adr/0045-github-hosted-runners.md`).

## Поток деплоя

```
push в dev (stage) или main (prod)
  → ci.yml (lint, тесты, миграции up/down, сканеры — blocking)
  → сборка 4 образов (matrix, параллельно, GitHub-hosted) → trivy → push в GHCR → cosign sign
  → _deploy.yml: cosign verify → рендер env из секрета ENV_FILE (со сверкой
    ключей против deploy/.env.<env>.example) → передача env на сервер через base64
    в envs ssh-шага → pg_dump-бэкап →
    миграции → up -d --wait → Caddy-фрагмент (guard → .prev → mv →
    validate → reload → smoke, откат фрагмента при провале) →
    внешние smoke → точечная чистка старых образов
```

При падении раскатки или smoke — автоматический откат образов на предыдущие
(`.previous-images` в deploy-каталоге); при падении шага Caddy-фрагмента —
откат самого фрагмента на `rentlee.caddy.prev`. БД автоматически НЕ
откатывается — см. разделы «Миграции» и «Rollback».

Workflow-файлы: `.github/workflows/{ci,security,_deploy,deploy-stage,deploy-prod}.yml`.
`security.yml` — это SAST (semgrep) и trivy-fs (ежедневный ночной прогон + non-blocking на PR);
уязвимости зависимостей закрывают govulncheck в `ci.yml` и гейт `make npm-audit`
(high+); Dependabot (security + сгруппированные weekly version updates в ветку
`dev`) отключён владельцем 29.09.2026 — `.github/dependabot.yml` снесён,
восстанавливается из git-истории.
Ручной деплой и откат — `workflow_dispatch` в `deploy-stage.yml` /
`deploy-prod.yml` с четырьмя digest-inputs.

## Окружения

- `dev` branch → stage: `/opt/arenda/stage`, compose project `arenda-stage`,
  GitHub Environment `stage` (deployment branch policy: только `dev`).
- `main` branch → prod: `/opt/arenda/prod`, compose project `arenda-prod`,
  GitHub Environment `production` (deployment branch policy: только `main`).
- Stage backend использует `APP_ENV=stage` и `PAYMENT_PROVIDER=fake`: валидация
  на stage строгая — как в `production`, с явными исключениями для фейкового
  платёжного провайдера и фейкового email-sender (тестирование без реальных
  денег). T-Kassa остаётся легальной на stage: sandbox-`T_KASSA_BASE_URL`
  проходит валидацию. Смена `APP_ENV`/`PAYMENT_PROVIDER` вносится в секрет
  `ENV_FILE` окружения `stage` вместе с этим example (набор ключей не менялся);
  обновлять секрет синхронно с деплоем новой версии бэка — старый бэк значение
  `APP_ENV=stage` не примет.
- Prod backend использует `APP_ENV=production`.
- Deploy-каталоги на сервере содержат только `docker-compose.<env>.yml`,
  `.env.<env>` (+ `.prev`), `.previous-images`, `.deploy-run-id`; репозитория
  и git-чекаута на сервере нет.
- Лендинг (`apps/landing`, Next.js standalone — ADR 0063) слушает `127.0.0.1:13002` в prod и `127.0.0.1:23002` в stage; Caddy отдаёт его как явное исключение на публичном домене (`/`, `/_next/*`, юрстраницы, `robots.txt`), всё остальное уходит кабинету — см. раздел «Caddy».
- При каждом деплое backend недоступен 5–15 секунд (один инстанс) — принято.
- Каждый деплой stage/prod также раскатывает Caddy-фрагмент rentlee
  (`deploy/caddy/rentlee.caddy` → `/etc/caddy/conf.d/rentlee.caddy`) —
  см. раздел «Caddy».

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

1. Установить Docker + compose plugin. Раннер на сервере не нужен: все джобы
   (CI, сборка образов, деплой) выполняются на GitHub-hosted раннерах
   (ADR 0045); сервер принимает только SSH от deploy-пайплайна (ключ-deploy
   пользователя, host key pinned, fail2ban на хосте).
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

## Runner Decommission (runbook)

Одноразовое списание self-hosted раннера с VPS после перехода на
GitHub-hosted раннеры (ADR 0045). Выполнялось 2026-08-21.

1. В GitHub: Settings → Actions → Runners → выбрать раннер → Remove
   (если сервис уже мёртв, принудительно). Registration token для CLI-удаления
   выдаётся там же (New runner → Configure).
2. На сервере:

   ```bash
   sudo systemctl stop actions.runner.*.service || true
   sudo systemctl disable actions.runner.*.service || true
   cd <каталог-раннера>   # обычно /opt/actions-runner или ~/actions-runner
   ./config.sh remove --token <registration-token>
   cd .. && rm -rf <каталог-раннера>
   ```

3. Проверить, что свободная память хоста выросла (`free -h`) — билды больше
   не конкурируют с продом.

## GitHub Environments и секреты

Два environment: `stage` и `production`. Набор секретов одинаковый, значения
свои.

| Секрет | Где | Назначение | Ротация |
| --- | --- | --- | --- |
| `ENV_FILE` | environment | Весь набор переменных окружения одним multiline-значением. Ключи должны точно совпадать с `deploy/.env.<env>.example` | При изменении любой переменной; заодно обновить example, иначе deploy упадёт на сверке ключей |
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

- Источник истины по **набору ключей** — `deploy/.env.stage.example` /
  `deploy/.env.prod.example` в репозитории (рядом с `deploy/docker-compose.<env>.yml`,
  чьи `env_file`-пути разрешаются от каталога compose-файла). Источник **значений** — секрет
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
- `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` — пара P-256 ключей (RFC 8292,
  base64url без padding) для идентификации application-сервера в Web Push.
  У stage и prod свои пары (проставляются в `ENV_FILE` соответствующего
  GitHub Environment). **Смена ключей ломает все живые push-подписки**
  (RFC 8292 §4.2: подписка привязана к ключу), поэтому ротация — только
  вместе с клиентским переподписным флоу. `VAPID_SUBJECT` — контактный URI
  (`mailto:` или `https:`) для push-сервисов, единый для окружений
  (`mailto:smirnowwwivan@mail.ru`). Публичный ключ отдаётся фронту в рантайме
  через `GET /push/vapid-public-key` (BFF-прокси `app/api/[...path]/route.ts`),
  приватный — серверный секрет; никуда кроме `ENV_FILE` не кладётся.
- Очередь доставки уведомлений (River, ADR 0059) настраивается ключами
  `NOTIFICATIONS_*` (потолки воркеров очередей, бюджеты ретраев, окно
  soft-stop, глобальный email-бюджет провайдера) — в `ENV_FILE` обычно не
  указывается, рабочие дефолты зашиты в конфиг. Схема очереди применяется
  шагом `migrate` после основных миграций (отдельная цепочка `river_migration`).

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
- Down-миграция `000086_property_deletion_detach` выполняет `SET NOT NULL` на
  `property_id` в таблицах арендного домена и исторически падала, пока в БД
  есть отвязанные строки (`property_id IS NULL`). После drop-миграции
  `000114` (спека #434) откат за 000086 всегда проходит через 000114-down,
  который восстанавливает эти таблицы пустыми — ручная чистка строк больше не
  нужна. Стандартный rollback это не ломает: автоматический откат
  `migrate down` не выполняет никогда, а откат БД — только restore из дампа
  (см. ниже).

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

## Caddy

Caddy работает на хосте как systemd-сервис и проксирует публичные домены на
localhost-порты контейнеров. Конфигурация — модель import conf.d (карта #649):

- главный файл `/etc/caddy/Caddyfile` сводится к глобальному блоку и
  `import /etc/caddy/conf.d/*.caddy`;
- каждый проект — свой файл в `/etc/caddy/conf.d/`; чужие проекты правятся
  руками на сервере, пайплайн их не трогает;
- рентли-часть — `/etc/caddy/conf.d/rentlee.caddy`, её источник правды —
  репозиторий: `deploy/caddy/rentlee.caddy`. Раскатывается пайплайном при
  каждом деплое stage/prod, руками на сервере не редактировать — правка будет
  затёрта следующим деплоем.

### Контракт маршрутизации (инверсия фолбэка)

Кабинет Next.js — дефолтный catch-all; лендинг — явное исключение:

- `/api/*` → бэкенд через `handle_path` (префикс `/api` снимается);
- `/webhooks/*` → бэкенд через `handle` (префикс НЕ снимается — T-Kassa шлёт
  колбэки на полный путь);
- лендинг (`@landing`): `/` (точный), `/privacy`, `/terms`, `/robots.txt`,
  `/sitemap.xml`, `/_next/*` (бандлы и image-оптимизатор Next),
  `/fonts/*` (self-hosted Onest), `/icon.png` (favicon) — через `handle`,
  НЕ `handle_path`: контейнер лендинга (Next standalone, ADR 0063)
  ждёт полный путь;
- всё остальное → кабинет (`127.0.0.1:13000` prod / `:23000` stage).

Следствия:

- новый top-level роут фронта работает без правок прокси;
- новый публичный путь лендинга = добавление в матчер `@landing`
  (`deploy/caddy/rentlee.caddy`) + файл в `apps/landing/public/`;
- `robots.txt` и `sitemap.xml` отдаёт лендинг (`apps/landing/public/`);
  `Disallow`-список robots.txt зеркалит `APP_ROUTE_PREFIXES` фронта минус
  `/login`;
- `/sw.js` уходит фронту catch-all'ом; `Cache-Control: no-store` ставит Next
  (`headers()` в `next.config.ts`), на `/sw.js` никаких заголовков Caddy не
  добавлять (ADR 0032); `Content-Type` обязан быть
  `text/javascript`/`application/javascript`, иначе регистрация SW упадёт
  (требование спецификации — JS MIME);
- дедупликация prod/stage-блоков — snippet `(rentlee_site)` с позиционными
  аргументами `{args[0]}` (канон Caddy ≥2.7; на сервере v2.11.4);
- SSE-стрим (`/api/notifications/stream`, ADR 0060) исключён из `encode`
  матчером `not header Content-Type text/event-stream`: дефолтный матчер
  сжатия включает `text/*`, и gzip начал бы буферизовать кадры после 512
  байт; `reverse_proxy` сам флешит `text/event-stream` немедленно —
  `flush_interval` не нужен;
- `www.rentlee.ru` — permanent redir на канон; admin-блоки — catch-all на
  админку с тем же `handle_path /api/*`;
- access-лог — общий `/var/log/caddy/access.log` (JSON, roll 50mb × 5),
  пишут все блоки хоста, включая чужие; ротацию делает сам Caddy.

### Раскатка фрагмента пайплайном

Шаг «Deploy Caddy fragment» в `_deploy.yml` (общий для stage и prod), после
раскатки образов и перед внешним smoke:

1. upload `deploy/caddy/rentlee.caddy` → `/tmp/arenda-caddy-<env>/`;
2. guard: в `/etc/caddy/Caddyfile` есть `import /etc/caddy/conf.d` — иначе
   шаг падает громко (защита от «фрагмент уехал, но не импортируется»; до
   разового расщепления монолита — runbook ниже — деплой останавливается
   здесь);
3. install как `/etc/caddy/conf.d/rentlee.caddy.new` (суффикс `.new` не
   матчит glob `*.caddy` — случайный подхват на лету исключён);
4. backup текущего фрагмента → `rentlee.caddy.prev`;
5. атомарный `mv` на место фрагмента;
6. `caddy validate --config /etc/caddy/Caddyfile` — главный файл с импортами
   (фрагмент резолвится только через import);
7. `systemctl reload caddy` — graceful, zero-downtime; при невалидном конфиге
   работающий инстанс остаётся на старом;
8. smoke инвертированной маршрутизации: лендинг-исключения = 200, кабинет =
   catch-all (200/307), чужой путь = 404, `/api/healthz` = ok, `/sw.js` =
   no-store + JS MIME, `robots.txt` = text/plain;
9. провал validate/reload/smoke → откат: `rentlee.caddy.prev` на место +
   reload, шаг падает громко (образы при провале смоука откатит штатный
   «Rollback on failure»).

Sudo: deploy-пользователю нужны passwordless-права на
`caddy validate --config /etc/caddy/Caddyfile`, `systemctl reload caddy` и
`install`/`cp`/`mv`/`rm` над файлами `/etc/caddy/conf.d/rentlee.caddy*`
(точечный sudoers — настроить один раз при расщеплении монолита).

Изменение конфига — только через репо: править `deploy/caddy/rentlee.caddy` →
влить в `dev` (stage) / `main` (prod) → деплой. Ручной сценарий «поправить
файл на сервере и reload» остаётся аварийным: файл фрагмента —
`/etc/caddy/conf.d/rentlee.caddy`, проверка и применение —
`sudo caddy validate --config /etc/caddy/Caddyfile && sudo systemctl reload caddy`;
следующая раскатка перезапишет ручную правку.

### Разовое расщепление монолита (одноразовый runbook)

Строго до первого пайплайн-выката фрагмента (иначе guard остановит деплой).
Поведение-сохраняющее: инверсию делает первый выкат фрагмента, не
расщепление. Выполняется руками, ад-хок SSH-командами (без скрипта в репо).

1. Снять монолит: `scp alterix-server:/etc/caddy/Caddyfile ./live`.
2. Расщепить руками: top-level site-блоки → `conf.d/<домен>.caddy`; пятёрка
   rentlee-блоков — в `conf.d/rentlee.caddy` в текущем живом виде (протухший
   whitelist и `log`-блоки переехать как есть).
3. Доказать эквивалентность: `caddy adapt` старого монолита vs собранного
   нового (импорты резолвятся), нормализация `jq -S`, дифф пустой — иначе не
   устанавливать.
4. Бэкапы: живой файл → `Caddyfile.prev-monolith`; 12 `Caddyfile.bak-*` →
   tarball `/root/caddy-bak-<дата>.tar.gz` + снос из `/etc/caddy`.
5. Установить (`install -m 644`), настроить sudoers deploy-пользователя
   (см. выше), `caddy validate --config /etc/caddy/Caddyfile` серверным
   бинарём, `systemctl reload caddy`.
6. Smoke: rentlee (`/login`, `/dashboard`, `/api/healthz`) и 2–3 чужих
   домена (byron-lounge, inten, kb-3) живы. При провале — откат из
   `Caddyfile.prev-monolith` + reload.
## Smoke Checks

Эти же проверки (плюс сверка `version` в `/api/healthz` с sha коммита)
выполняет deploy-джоба снаружи; вручную их можно повторить так:

```bash
curl -fsS https://dev.rentlee.ru/ | grep -q 'id="landing-root"'
curl -fsS https://dev.rentlee.ru/login | grep -qi '<html'
curl -fsS https://admin.dev.rentlee.ru/ | grep -q 'id="root"'
test "$(curl -fsS https://admin.dev.rentlee.ru/healthz)" = "ok"
curl -fsS https://dev.rentlee.ru/api/healthz | grep -q '"status":"ok"'
test "$(curl -sS -o /tmp/arenda-stage-me.out -w '%{http_code}' https://dev.rentlee.ru/api/me)" = "401"

curl -fsS https://rentlee.ru/ | grep -q 'id="landing-root"'
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

Контракт инвертированной маршрутизации (после переезда на `conf.d`,
карта #649) — вручную на любом окружении:

```bash
# Лендинг — явное исключение.
curl -fsS https://dev.rentlee.ru/privacy | grep -qi 'Политика конфиденциальности'
curl -fsS https://dev.rentlee.ru/robots.txt           # text/plain, Disallow-правила
curl -fsS https://dev.rentlee.ru/sitemap.xml          # xml, три URL
# Кабинет — catch-all.
test "$(curl -sS -o /dev/null -w '%{http_code}' https://dev.rentlee.ru/properties)" = "200"
test "$(curl -sS -o /dev/null -w '%{http_code}' https://dev.rentlee.ru/nosuchpath)" = "404"
# SW: no-store доходит от Next, Caddy его не перезаписывает (ADR 0032).
curl -sSI https://dev.rentlee.ru/sw.js | grep -i '^cache-control: no-store'
```

Пересекающиеся проверки выполняет шаг «Deploy Caddy fragment» после каждой
раскатки (он дополнительно грейтит `/login`, лендинг-ассеты и 404); их провал
откатывает фрагмент на `.prev` (см. раздел «Caddy»). Фрагмент один на оба
окружения: stage-деплой гоняет smoke против `dev.rentlee.ru`, prod-часть
фрагмента (`rentlee.ru`, admin) реально проверяется смоуком только на
prod-выкате — там её подстраховывают validate и откат.

Домен в `robots.txt` (`Sitemap:`) и в `sitemap.xml` (`<loc>`) — продовый
канон `https://rentlee.ru`; stage-зеркало этим файлом не индексируется.

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

### Метрики гигиены воркеров billing (тикет #433)

Backend экспортирует гейдж `billing.payments.stuck` (OTel, атрибут `kind`)
— число «висящих в полёте» исходов под сторожем reconciliation-воркера,
без чувствительных данных:

- `kind=pending` — оплаты без зафиксированного исхода дольше
  `PendingPaymentStaleness` (5 минут, конфиг billing): потерянный вебхук,
  незавершённая оплата. Порог алерта: > 0 дольше 1 часа — reconciliation
  не разрешает зависимости, дежурный смотрит раньше пользователя.
- `kind=refunding` — возвраты, зависшие в резервировании `refunding` дольше
  5 минут: возврат в полёте или неопределённый исход. Порог алерта: > 0
  дольше 1 часа — деньги пользователя в подвешенном состоянии, нужен ручной
  разбор (неопределённый исход при failed-статусе провайдера).

Оба гейджа пишет фаза `ExportStuckPaymentMetrics` billing-воркера в конце
каждого тика. Истёкшие сессии привязки карты удаляются фазой
`ProcessExpiredBindingSessions` того же тика (таблица не растёт неограниченно);
алерта на чистку нет — сбой фазы виден в логах воркера и росте таблицы
`card_binding_sessions`.

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
