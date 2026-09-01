# Параллельная разработка в ворктри: слоты портов, `.env`, compose-проекты

Контракт изоляции локальных dev-окружений для параллельных агентских сессий в git-ворктри.
Зафиксирован тикетом [#487](https://github.com/devnumbers/arenda-platform/issues/487) (карта [#485](https://github.com/devnumbers/arenda-platform/issues/485)); реализация — [#488](https://github.com/devnumbers/arenda-platform/issues/488), живая проверка — [#489](https://github.com/devnumbers/arenda-platform/issues/489), итоговый рецепт дополняет этот файл [#490](https://github.com/devnumbers/arenda-platform/issues/490).

## Слоты и формула портов

Слот 0 — основной чекаут: исторические порты без изменений (обратная совместимость). Слоты 1–9 — ворктри; ожидаемая эксплуатация — 2–3 одновременных сессий, диапазон до 9 оставляет запас.

Формула: **порт = BASE[сервис] + OFFSET[роль] + N**, где N — номер слота; роль dev = +0, роль e2e = +5.

BASE: postgres 5440, backend 8090, frontend 3020.

| сервис        | слот 0 (main) | слот 1 | слот 2 | слот 3 |
|---------------|---------------|--------|--------|--------|
| postgres dev  | 5433          | 5441   | 5442   | 5443   |
| postgres e2e  | 5436          | 5446   | 5447   | 5448   |
| backend dev   | 8080          | 8091   | 8092   | 8093   |
| backend e2e   | 8081          | 8096   | 8097   | 8098   |
| frontend dev  | 3000          | 3021   | 3022   | 3023   |
| frontend e2e  | 3010          | 3026   | 3027   | 3028   |

- Базы тестов (postgres test, порт 5435, compose-проект `arenda-test`) и testcontainers остаются прерогативой основного чекаута: в ворктри `backend-test`/`backend-test-integration` идут через testcontainers — свой контейнер на прогон, без общих портов и имён.
- Все порты схемы < 32768 — вне эфемерных диапазонов macOS/Linux; зоны сервисов не пересекаются с существующими ручными выделениями (5433/5435/5436, 8080/8081, 3000/3010).

## Compose-проекты

Имя compose-проекта — единственный механизм изоляции контейнеров, сетей и томов: контейнеры `<project>-<service>-<index>`, named-тома `<project>_<volume>` (префикс автоматический, external-томов в схеме нет).

- dev-стек ворктри: `arenda-wt<N>`; e2e-стек ворктри: `arenda-e2e-wt<N>`.
- Make-таргеты передают имя явно флагом `-p` (высший приоритет compose), вычисляя его из `AREND_SLOT` в `.env` чекаута; слот 0 → `arenda-local` (сегодняшнее поведение основного чекаута не меняется).
- `COMPOSE_PROJECT_NAME` дублируется в per-worktree `.env`: ручные вызовы `docker compose --env-file .env -f apps/backend/docker-compose.local.yml ...` без `-p` попадают в тот же проект.
- `name: arenda-local` в `docker-compose.local.yml` остаётся — дефолт для вызовов без `.env` и `-p`.
- `local-infra-down`/`local-infra-reset` и e2e-зачистка действуют только на проект своего слота; `down -v` чужие данные не трогает.

## Per-worktree `.env`

Ворктри не наследует untracked-файлы — `.env` в нём обязан появиться заново. `worktree-new` создаёт два файла (оба игнорируются git'ом: `.gitignore` — `.env`, `.env.*`, `.worktrees/`).

1. **Корневой `.env` ворктри** — копия корневого `.env` основного чекаута (реальные ключи: DADATA, ENCRYPTION_KEY, SMTP, T-Kassa sandbox; если корневого `.env` нет — копия `.env.example` с предупреждением, что ключи-плейсхолдеры надо вписать руками) + переопределяющий блок в конце файла (последнее вхождение побеждает и в shell-сорсинге `set -a; . ./.env`, и в `--env-file` compose):

   | переменная            | значение                                                     |
   |-----------------------|--------------------------------------------------------------|
   | `AREND_SLOT`          | N                                                            |
   | `POSTGRES_PORT`       | 5440+N                                                       |
   | `DATABASE_URL`        | `postgres://arenda:arenda@localhost:5440+N/arenda?sslmode=disable` |
   | `HTTP_ADDR`           | `:8090+N`                                                    |
   | `APP_BASE_URL`        | `http://localhost:8090+N`                                    |
   | `BACKEND_URL`         | `http://localhost:8090+N`                                    |
   | `COMPOSE_PROJECT_NAME`| `arenda-wt<N>`                                               |
   | `E2E_PG_PORT`         | 5440+5+N                                                     |
   | `E2E_BACKEND_PORT`    | 8090+5+N                                                     |
   | `E2E_FRONTEND_PORT`   | 3020+5+N                                                     |
   | `E2E_COMPOSE_PROJECT` | `arenda-e2e-wt<N>`                                           |

   Всё остальное наследуется из копии без изменений (`APP_ENV`, `LOG_*`, `TRUSTED_PROXIES`, `POSTGRES_USER/PASSWORD/DB`, `MIGRATIONS_DIR`, `COOKIE_SECURE`, `EMAIL_*`, `PAYMENT_PROVIDER`, `T_KASSA_*`, `ENCRYPTION_KEY`, `OTEL_*`, `DADATA_*`, `VAPID_*`, `PHOTO_*`/S3): одинаковые имена БД, пользователя и пароля не конфликтуют — у каждого слота свой контейнер, том и порт.

2. **`apps/frontend/.env.development.local`** — `BACKEND_URL=http://localhost:8090+N`.

### Почему `BACKEND_URL` обязан быть в per-worktree env (факт репо)

`BACKEND_URL` читает только серверный код фронта — `apps/frontend/proxy.ts` и `apps/frontend/app/api/[...path]/route.ts` — из `process.env` с фолбэком `http://localhost:8080`; `NEXT_PUBLIC_` не нужен. `make frontend-dev` корневой `.env` не сорсит, а Next читает env-файлы только из каталога приложения (`apps/frontend`) — корневой `.env` репо для него невидим. Сегодня фронт работает лишь потому, что фолбэк совпадает с дефолтным портом бэка: при слотовом бэке на 8091 фронт молча остался бы на 8080. Поэтому значение передаётся файлом `apps/frontend/.env.development.local` (высший файловый приоритет в dev) и дублируется в корневом `.env` ворктри — чтобы сорсинг `.env` (make, direnv, вручную) не экспортировал устаревший `:8080` поверх файла (process env сильнее файлов). Порт dev-сервера фронтенда задаётся флагом `next dev -p` (переменную `PORT` Next из `.env`-файлов не читает — HTTP-сервер поднимается до инициализации env); `make frontend-dev` вычисляет порт из слота, слот 0 — без флага (дефолт 3000).

## Реестр слотов

Центрального файла-реестра нет — реестр это сами `.env`-файлы чекаутов (центральный файл означал бы merge-конфликты). `worktree-new` сканирует `AREND_SLOT=` в корневом `.env` и `.worktrees/*/.env`, берёт первый свободный N из 1..9, дубликаты отклоняет. Слот освобождается сам: `git worktree remove .worktrees/<имя>` удаляет каталог вместе с `.env`. Живая правда о запущенных проектах — `docker compose ls`.

## make worktree-new

`make worktree-new WT=<имя>` — `<имя>` одновременно имя ветки и каталога (валидация: slug `[a-z0-9._-]`, ветка и каталог не существуют).

Делает (минимум — решение владельца при чартинге карты):

1. `git worktree add .worktrees/<имя> -b <имя>` от текущего HEAD.
2. Выделяет слот (см. «Реестр слотов»); свободных нет — отказ.
3. Генерирует корневой `.env` ворктри и `apps/frontend/.env.development.local` (см. «Per-worktree .env»).
4. Печатает факты подключения: слот; порты postgres/backend/frontend для dev- и e2e-ролей; `DATABASE_URL`; имена dev- и e2e-compose-проектов; команды старта с путём ворктри (`make local-infra-up && make migrate-up`, `make frontend-dev`, `make frontend-e2e`).

Не делает: зависимости (`npm install`, `go mod download`), бейзлайн-тесты, старт инфраструктуры, push ветки — это сторона скилла `using-git-worktrees`.

## Что реализует #488

- `-p $(COMPOSE_PROJECT)` в `COMPOSE_LOCAL` и вычисление слота/портов в Makefile (слот 0 → сегодняшнее поведение без изменений);
- e2e-таргеты сорсят корневой `.env` ворктри, чтобы `E2E_*` текли в раннер из process env;
- параметризация e2e-раннера: `E2E_COMPOSE_PROJECT` (дефолт `arenda-e2e`), `PG_CONTAINER` из имени проекта вместо захардкоженного `arenda-e2e-postgres-1`; `frontend-e2e-live-down` читает слот из `.env` вместо хардкода `-p arenda-e2e` и портов 3010/8081/5436;
- сам таргет `worktree-new`.

До параметризации раннера параллельные `frontend-e2e` опасны: `free_port()` убивает любой процесс на 8081/3010 — в том числе чужую сессию с теми же дефолтами.
