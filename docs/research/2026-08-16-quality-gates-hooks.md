# Механизмы гейтов качества: хуки Kimi Code / ZCode, git pre-commit

Дата: 2026-08-16
Вход для: тикет [#291](https://github.com/devnumbers/arenda-platform/issues/291) «[research] Механизмы гейтов: хуки Kimi Code / ZCode, git pre-commit (lefthook)», часть карты [#289](https://github.com/devnumbers/arenda-platform/issues/289) «Wayfinder: агентская инфраструктура репозитория».

Файл содержит только фактуру из первоисточников (официальные доки, README) и проверенные локальные факты репозитория. Рекомендаций нет; раздел «Выводы для решения» — сухие факты и trade-offs, решение принимает человек.

Замеры скорости сделаны однократными прогонами на машине разработчика (macOS arm64, go1.26.5, `node_modules` всех пакетов на месте, тёплый Go build/module cache) — порядок величин, не бенчмарк.

## 0. Текущее состояние гейтов в репозитории (локальные факты)

Проверено чтением файлов 2026-08-16.

- Гейты держатся на дисциплине агента: корневой `AGENTS.md` («Before claiming completion, run the relevant project checks», `make test`), `apps/backend/AGENTS.md:92-113` (`make backend-lint`, `make backend-test`, `go vet ./...`, `gopls`-диагностика), `apps/frontend/AGENTS.md:78-86` (`npm run lint`, `npm run build`, `make frontend-test`), `apps/admin/AGENTS.md:43-51` (`make admin-typecheck`, `make admin-build`, `make admin-test`).
- Git-хуков нет: в `.git/hooks/` только `*.sample`-файлы, `core.hooksPath` не задан (проверено `ls` + `git config core.hooksPath`).
- Хуков харнесса нет: в `~/.kimi-code/config.toml` секция `[[hooks]]` отсутствует (проверено `grep -i hook` — 0 совпадений).
- `Makefile`: `backend-lint` запускает пинненный `golangci-lint@v2.12.2` через `go run` с `--config ../../.golangci.yml` (`Makefile:7,43-44`); `backend-test` = `go test -race ./...` без БД (`Makefile:59-60`); интеграционные — testcontainers (`Makefile:67-68`); codegen-гейты `backend-tkassa-spec-check` (`Makefile:111-122`) и `attributes-check` (`Makefile:142-153`) сравнивают `git hash-object` артефактов до/после регенерации.
- `.golangci.yml`: golangci-lint v2, ~70 линтеров + форматтеры `gofumpt` и `gci` (`.golangci.yml:277-280`), `run.timeout: 5m` (`.golangci.yml:9`).
- CI `.github/workflows/ci.yml`: backend = lint (строки 24-25), tkassa-spec-check (27-28), `go test -race` в контейнере `golang:1.26-bookworm` (33-38), `go vet` (40-41), govulncheck (43-47); миграции up/down/up (49-89); frontend = `make attributes-check` (118-119), `npm run build` (121-123), `npm run lint` (125-127), `npx tsc --noEmit` (129-131); admin = typecheck (151-153), build (155-157); landing build; gitleaks через docker CLI (181-197); hadolint + compose config + docker build + trivy по 4 сервисам (199-252).
- `.github/workflows/security.yml`: semgrep и trivy-fs ночью по расписанию; на PR — `continue-on-error` (не блокируют).
- В корне репо **нет** `package.json` (проверено `ls`) — у корня нет node-зависимостей.
- Скрипты пакетов: frontend — `lint: eslint`, `test: vitest run`, отдельного `typecheck` нет (`apps/frontend/package.json:5-15`; CI вызывает `npx tsc --noEmit` напрямую); admin — `typecheck: tsc --noEmit`, `build: tsc && vite build`, `test: vitest run` (`apps/admin/package.json:6-13`); генератор атрибутов — `validate`/`generate` на чистом node + ajv (`tools/property-attributes/package.json`).

## 1. Хуки Kimi Code

Первоисточники: [Hooks](https://www.kimi.com/code/docs/en/kimi-code-cli/customization/hooks.html), [Configuration files](https://www.kimi.com/code/docs/en/kimi-code-cli/configuration/config-files.html) (обе страницы вычитаны 2026-08-16).

### Механика

- Правила пишутся в массиве `[[hooks]]` в `~/.kimi-code/config.toml`. Ровно четыре поля, лишние поля = конфиг не загрузится: `event` (обяз.), `matcher` (regex-фильтр цели события, необяз.), `command` (shell-команда, обяз.), `timeout` (1–600 с, дефолт 30).
- При срабатывании CLI передаёт JSON с деталями события в **stdin** скрипта (`hook_event_name`, `session_id`, `cwd`, для tool-событий — `tool_input` и т.п.).
- Ответ скрипта: exit code `0` = allow (stdout может быть дописан в контекст), `2` = block (stderr — причина блокировки, уходит в контекст модели), прочие non-zero = allow. Альтернатива — JSON в stdout: `{"hookSpecificOutput": {"permissionDecision": "deny", "permissionDecisionReason": "..."}}`.
- **Fail-open**: ошибка скрипта, таймаут или падение → allow. Документация прямо: хуки подходят для предупреждений и лёгкой блокировки, но «should not be used as the sole security barrier».
- Совпавшие по событию правила выполняются параллельно; одинаковые `command` запускаются один раз. Рабочая директория хука = директория проекта сессии.

### События (20)

`UserPromptSubmit`, `UserPromptQueued`, `PreToolUse`, `Stop`, `TurnStarted`, `PostToolUse`, `PostToolUseFailure`, `PermissionRequest`, `PermissionResult`, `SessionStart`, `SessionEnd`, `SessionHeartbeat`, `SubagentStart`, `SubagentStop`, `TaskStarted`, `StopFailure`, `Interrupt`, `PreCompact`, `PostCompact`, `Notification`.

**Блокирующие только три: `PreToolUse`, `Stop`, `UserPromptSubmit`.** Все остальные — observation-only: сработали и забыли, возвращаемое значение ни на что не влияет.

Ответы на вопросы тикета по событиям:

- «До вызова инструмента» — есть: `PreToolUse` (до проверки прав; блокируемое; matcher = regex по имени инструмента, например `Bash` или `Write|Edit`).
- «После вызова инструмента / правки файла» — есть `PostToolUse` (matcher = имя инструмента), но **только наблюдение, заблокировать или отменить правку нельзя**; stdout может добавить контекст.
- «На коммит» — отдельного события нет. Коммит агента перехватывается косвенно: `PreToolUse` + `matcher = "Bash"` + разбор `tool_input.command` в скрипте (в офиц. примере так режут `rm -rf`). Документация сама предупреждает, что это не production-grade парсер (кавычки, подстановки, цепочки команд).
- «После завершения хода» — `Stop` блокируемое: можно вернуть причину и заставить модель продолжить (например, «сначала прогони тесты»). Лимит продолжений в документации Kimi не указан.

### Уровень конфигурации

Конфиг — только пользовательский: `~/.kimi-code/config.toml` (переопределяется `KIMI_CODE_HOME`). Проектный файл `<project-root>/.kimi-code/local.toml` существует, но по документации содержит только `[workspace] additional_dir` и рекомендован к gitignore — **проектных (коммитимых в репо) хуков у Kimi Code нет**.

### Пример из официальной доки

```toml
[[hooks]]
event = "PreToolUse"
matcher = "Bash"
command = "node ~/.kimi-code/hooks/block-dangerous-bash.mjs"
timeout = 5
```

Скрипт читает stdin, при `rm -rf` в команде пишет причину в stderr и делает `process.exit(2)`.

## 2. Хуки ZCode

Первоисточник найден: официальная документация [ZCODE Docs — Hooks](https://zcode.z.ai/en/docs/hooks) (вычитана 2026-08-16). ZCode — харнесс от Z.ai (модели GLM), desktop-приложение с CLI; ранее в нашем контексте про его возможности ничего не было известно — теперь есть первоисточник.

### Механика и события

- Хук = локальный подпроцесс: ZCode пишет одну строку JSON в stdin, процесс отвечает exit code и JSON в stdout. Поля дублируются в camelCase и snake_case (совместимость с Claude Code-плагинами).
- События (7): `SessionStart`, `UserPromptSubmit`, `PreToolUse`, `PermissionRequest`, `PostToolUse`, `PostToolUseFailure`, `Stop`.
- Возможности богаче Kimi: `PreToolUse` может allow/ask/deny и **полностью заменить вход инструмента** (`updatedInput`, ре-валидируется по схеме); `PermissionRequest` может auto-allow/deny интерактивный запрос прав; `Stop` с `decision: "block"` + reason продлевает ход модели, но **не более 3 раз подряд** (защита от бесконечного цикла). `PostToolUse` — только добавить контекст, вывод инструмента заменить нельзя.
- Exit codes: `0` = успех (stdout парсится), `2` = block/deny в блокируемых событиях, прочие non-zero = хук падает «восстановимо» (диагностика в лог, ход не падает) — т.е. тоже fail-open.
- Конфиг: `~/.zcode/cli/config.json`, обязателен `"hooks": {"enabled": true}`; формат `hooks.events.<Event>[] = {matcher, hooks[]}`; исполнитель `type: "process"` (argv без shell) или `type: "command"` (строка в shell); `timeoutMs` (дефолт 60000), `maxOutputBytes` (дефолт 32768). Snapshot конфигурации на старт сессии — правки применяются только к новым сессиям.

### Ключевое ограничение для командного сценария

**Проектные хуки в текущей версии не выполняются вовсе** (цитата доки): любой `hooks` в `<workspace>/.zcode/config.json` или `<workspace>/zcode.json` игнорируется целиком (логируется как `config_project_hooks_ignored`), «for security reasons». Шаринг по команде — только через **плагин** (каталог с `.zcode-plugin/plugin.json` + `hooks/hooks.json`; плагин можно версионировать в репо и ставить через marketplace-источник в Settings → Plugins). Порядок выполнения: user hooks → plugin hooks.

### Зрелость

- Открытый баг: [zai-org/feedback#32](https://github.com/zai-org/feedback/issues/32) (2026-06-22) — хуки из `~/.zcode/cli/config.json` не триггерились нативным ZCode Agent при корректном конфиге; в [zai-org/feedback#164](https://github.com/zai-org/feedback/issues/164) просят хотя бы slash-команду `/hooks` для диагностики.
- CLI-часть официально недокументирована: сторонний аудит ([headless-relay cli-reference](https://github.com/dorukardahan/headless-relay/blob/main/references/cli-reference.md), live-проверка 2026-07) — «Z.ai does not document the bundled CLI», OAuth-логин сломан, неофициальный npm-обёртка [zcode-app-cli](https://www.npmjs.com/package/zcode-app-cli) существует отдельно от вендора.
- Git-событий (commit/push) в ZCode-хуках нет — как и у Kimi, перехват коммита только через `PreToolUse` по имени инструмента shell.

## 3. Git pre-commit: lefthook vs pre-commit vs husky

Первоисточники: [lefthook README](https://github.com/evilmartians/lefthook) + [lefthook.dev](https://lefthook.dev/) ([usage](https://lefthook.dev/usage/index.html), [parallel](https://lefthook.dev/configuration/parallel.html)); [pre-commit.com](https://pre-commit.com/); [husky](https://typicode.github.io/husky/) + [lint-staged README](https://github.com/lint-staged/lint-staged).

### Сравнение по критериям тикета

| Критерий | lefthook | pre-commit | husky (+ lint-staged) |
|---|---|---|---|
| Что это | Один Go-бинарь-менеджер git-хуков, «single dependency-free binary» | Python-пакет-менеджер хуков: тянет hook-репозитории и сам ставит их окружения (node, golang, …) | Тонкий запускатель нативных git-хуков через `core.hooksPath` (2 kB, 0 зависимостей, оверхед ~1 ms) |
| Зависимости на машине | Go ≥ 1.26 для `go install github.com/evilmartians/lefthook/v2@v2.1.10` (есть `go get -tool`, brew, npm, pipx и др.) | Python + pip/pipx (есть 0-dependency zipapp `.pyz`) | Node/npm в корне репо (нужен корневой `package.json` — у нас его нет); сама фильтрация файлов — через отдельный пакет lint-staged |
| Конфиг в репо | `lefthook.yml` (+ опц. `lefthook-local.yml` для личных skip/override) | `.pre-commit-config.yaml` | `.husky/*` shell-скрипты + секция `lint-staged` в `package.json` / `.lintstagedrc` |
| Только staged-файлы | Да: плейсхолдеры `{staged_files}`, `glob`/`exclude` фильтры, кастомный `files:`-генератор, `root:` для поддиректории | Да, по умолчанию: незакоммиченные изменения stash-атся, хуки получают список staged-файлов | Нет из коробки — это делает lint-staged (glob → команда, абсолютные пути, stash-бэкап) |
| Параллелизм | `parallel: true` — **opt-in, по умолчанию sequential** ([дока](https://lefthook.dev/configuration/parallel.html)) | Хуки параллелятся по файлам по умолчанию (`require_serial: true` отключает) | lint-staged: задачи конкурентны по умолчанию (`--concurrent`) |
| Автофикс обратно в коммит | `stage_fixed: true` | Хук обязан сам изменить файлы; изменения попадают в коммит | lint-staged сам stage-ит правки задач |
| Пропуск | `LEFTHOOK=0 git commit`; `skip: true` в local-конфиге; `--no-verify` | `SKIP=<id>[,<id>]` точечно по хукам; `--no-verify` | `--no-verify`; глобальное отключение предусмотрено |
| Установка хуков | `lefthook install` пишет `.git/hooks` (нужен install-шаг per clone) | `pre-commit install` пишет `.git/hooks/pre-commit` | Автоматически при `npm install` в корне (npm `prepare`-скрипт) |
| Монорепо | `root:` per job, glob-фильтры по путям — достаточно | `pre-commit hazmat cd <subdir>` (v4.5+) или per-package конфиги; `repo: local` + `language: unsupported` для вызова make/скриптов репо | lint-staged: ближайший конфиг к файлу, per-package конфиги |
| Прочее | `lefthook run <hook>` — прямой запуск группы; теги jobs; remotes для общих конфигов | Огромная экосистема готовых хуков; CI-режим `pre-commit run --all-files`; кэш окружений; поддерживает все client-side git hooks | Только запускает; вся логика — в вызываемых командах |

Общий факт уровня git: `git commit --no-verify` пропускает client-side хуки независимо от менеджера — обход всегда в одну команду.

### Совместимость с работой AI-агентов

- Агенты в этом репо коммитят через `Bash`-инструмент → git-хуки срабатывают одинаково для человека и любого харнесса (Kimi, ZCode, IDE). Это единственный из рассмотренных механизмов, не зависящий от харнесса.
- Частые WIP-коммиты агента × медленный хук → соблазн `--no-verify`; гейт снова начинает держаться на дисциплине (сейчас это правило AGENTS.md «не коммитить без явной просьбы» + запрет мутаций git без подтверждения).
- Хуки харнесса (разделы 1–2) и git-хуки — ортогональные слои: первые видят *намерение* агента до вызова инструмента, вторые видят *факт* коммита, независимо от того, кто вызвал `git`.

## 4. Какие проверки разумно вешать на pre-commit здесь (замеры)

Замеры 2026-08-16 на этой машине, тёплые кэши, одиночные прогоны:

| Проверка | Команда-замер | Время | Пригодность для pre-commit |
|---|---|---|---|
| Формат Go (gofumpt) | `gofumpt -l .` по всему backend | 0.19 с | Быстро, **но см. дрейф версий ниже** |
| golangci-lint по пакету | `golangci-lint@v2.12.2 run --config ../../.golangci.yml --build-tags=integration ./internal/platform/scheduler/...` | 2.7 с | Да, по пакетам staged-файлов |
| `go vet` по пакету | `go vet ./internal/platform/scheduler/...` | 0.37 с | Да (фактически покрыт govet внутри golangci-lint) |
| Быстрые unit по пакету | `go test ./internal/platform/scheduler/...` (cached) | 1.1 с | Да, по изменённым пакетам (без `-race` быстрее; `-race` из `make backend-test` — полный прогон) |
| ESLint по staged-файлу | `npx eslint shared/lib/format-money.ts` (frontend) | 2.1 с | Да; доминирует старт flat-конфига, число файлов почти не влияет |
| tsc frontend | `npx tsc --noEmit` | 2.0 с | Да, но всегда **весь проект** (tsc с файловыми аргументами игнорирует `tsconfig.json` — известное ограничение, в lint-staged README описан workaround «вызывать без аргументов») |
| tsc admin | `npm run typecheck` | 1.9 с | Да, аналогично весь проект |
| Codegen-гейт атрибутов | `make attributes-check` | 0.2 с | Да, триггерить по staged `tools/property-attributes/catalog.json` или `*/generated/*` |
| Полный `make backend-lint` | не замерялся (whole-repo, `run.timeout: 5m` в `.golangci.yml:9`) | минуты | Нет — уровень CI/pre-push |
| Полный `make backend-test` (`-race ./...`), vitest-сьюты, `next build`, `vite build`, testcontainers, govulncheck, gitleaks, trivy, hadolint | не замерялись | — | Нет — уровень CI |

Найденный подводный камень (проверен экспериментом): standalone `gofumpt` из `~/go/bin` — **v0.11.0 (go1.25.6)** — флагает 3 тестовых файла (`internal/access/application/policy_test.go`, `internal/identity/adapters/postgres/phone_crypto_test.go`, `internal/platform/scheduler/reminder_worker_test.go`), а пинненный `golangci-lint@v2.12.2` на пакете `internal/platform/scheduler` даёт **0 issues** — т.е. CI эти файлы принимает. Хук на `gofumpt` из PATH блокировал бы коммиты, которые CI пропускает. Корректный источник правды — пинненный тулчейн репо: `golangci-lint fmt` / `run` с `.golangci.yml` (команда автофикса зафиксирована в `apps/backend/AGENTS.md:94`; golangci-lint принимает директории/пакеты, файлы — только одного пакета за раз: [quick-start](https://golangci-lint.run/docs/welcome/quick-start/)).

## 5. Стыковка с make-целями и CI

- Pre-commit здесь — **предварительный** слой: staged-скоуп, секунды. CI (`.github/workflows/ci.yml`) остаётся источником правды: полные прогоны линтера и тестов, testcontainers, govulncheck, gitleaks, trivy, docker-сборки, миграции. Ничего из тяжёлого на pre-commit не выносится — дублирования не возникает, потому что скоупы разные.
- Точки переиспользования: хук может звать существующие цели/команды как есть — `make attributes-check` (уже self-sufficient: ставит `node_modules` генератору при отсутствии, `Makefile:139-144`), `golangci-lint run` с пином версии из `Makefile:7`, `npm run typecheck` админа. Новых make-целей для хуков не требуется, кроме, возможно, обёртки «staged .go → уникальные пакеты → golangci-lint run».
- Любой менеджер хуков требует install-шага после клонирования (lefthook: `lefthook install`; pre-commit: `pre-commit install`; husky: автоматически через npm `prepare`, но нужен корневой `package.json`). До install-шага гейт не существует — это opt-in per machine, и CI остаётся единственным гарантированным барьером.

## 6. Выводы для решения

1. **Хуки харнессов не могут быть командным гейтом.** У Kimi Code конфиг только пользовательский (`~/.kimi-code/config.toml`; проектный `local.toml` хуков не поддерживает). У ZCode проектные хуки в текущей версии игнорируются by design; командное распространение — только через плагин-механизм. В репозиторий коммитится только конфиг git-хуков.
2. **Хуки харнессов fail-open** (и Kimi, и ZCode): ошибка/таймаут скрипта = allow. Годятся как личная страховка агента (уведомления, лёгкая блокировка опасных команд, «не завершай ход без тестов» через `Stop`), но не как барьер — формулировка из официальной доки Kimi.
3. **Покрытие событий у обоих харнессов неполное для нашей задачи:** нет git-событий (коммит перехватывается только косвенно, через `PreToolUse` по shell-инструменту с парсингом команды — хрупко), нет блокирующего «после правки файла» (`PostToolUse` у обоих observation-only). Блокирующие у Kimi: `PreToolUse`, `Stop`, `UserPromptSubmit`; у ZCode те же по смыслу + `PermissionRequest`, с лимитом 3 продолжения на `Stop`.
4. **ZCode-экосистема молодая:** официальная дока по хукам есть и детальна, но CLI недокументирован вендором, есть открытый баг о несрабатывании хуков (zai-org/feedback#32).
5. **Git-хуки — единственный механизм, единый для людей и всех харнессов**, с конфигом в репо; цена — install-шаг per clone и тривиальный обход (`--no-verify`, `LEFTHOOK=0`, `SKIP=`, `HUSKY=0`). Для AI-агентов это одновременно фича (дешёвые WIP-коммиты) и возврат к дисциплине — финальным барьером остаётся CI.
6. **По зависимостям для этого репо:** Go (1.26.5) и Node (24) уже есть у всех; Python — новая зависимость (pre-commit framework); корневого `package.json` нет (husky+lint-staged потребуют его создать). lefthook ставится через `go install`/brew без новых рантаймов; конфиг один файл `lefthook.yml`.
7. **Скорость не является блокером:** все кандидатные проверки на staged-дифф на этой машине ≤ ~3 с каждая; суммарный бюджет pre-commit 5–10 с реалистичен. Параллелизм у lefthook — opt-in (`parallel: true`, дефолт sequential); у pre-commit и lint-staged параллелизм по умолчанию.
8. **Реальный подводный камень — дрейф версий форматтера:** standalone `gofumpt` v0.11.0 из PATH флагает 3 файла, которые пинненный `golangci-lint@v2.12.2` принимает. Хук обязан использовать пинненный тулчейн репо, иначе pre-commit будет строже CI.
9. **tsc по staged-файлам невозможен корректно** (tsc с файловыми аргументами игнорирует `tsconfig.json`); вариант — полный `tsc --noEmit` (~2 с на пакет здесь) при наличии staged TS.
10. **Trade-off по охвату:** даже идеальный pre-commit закрывает только «быстрое и локальное»; тяжёлые гейты (testcontainers, `-race` полный, сборки, сканеры) принципиально остаются в CI — pre-commit их не дублирует, а предваряет.
