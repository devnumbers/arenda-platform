# Serena (oraios/serena) как MCP-сервер семантической навигации по коду

- **Дата сбора:** 2026-08-16
- **Тикет:** https://github.com/devnumbers/arenda-platform/issues/290
- **Карта:** https://github.com/devnumbers/arenda-platform/issues/289
- **Метод:** чтение первоисточников — README и официальная документация (`oraios.github.io/serena`), исходный код репозитория (`src/solidlsp`, `src/serena`), открытые issues; метаданные репозитория и альтернатив — через GitHub API.
- **Контекст задачи:** в репо настроены MCP-серверы `lean-ctx` (поиск/сжатие/граф) и `gopls` (Go LSP), но для TypeScript (`apps/frontend`, `apps/admin`) нет никакой LSP-навигации — `apps/frontend/AGENTS.md:29` прямо резервирует место под будущий TS LSP/MCP-сервер. Ограничение: только бесплатные/локальные инструменты.

---

## 1. Что это и какие MCP-инструменты экспонирует

Serena — MCP-тулкит для кодящих агентов: семантическое извлечение, редактирование, рефакторинг и отладка кода **на уровне символов** (а не строк/регулярок), «the IDE for your agent» ([README](https://github.com/oraios/serena/blob/main/README.md)). Подключается к любому MCP-клиенту; LLM остаётся оркестратором, Serena даёт инструменты. Два бэкенда анализа кода:

- **LSP-бэкенд** (по умолчанию, бесплатный/open-source) — абстракция поверх language servers;
- **Serena JetBrains Plugin** (платный, есть trial) — использует индексы запущенной JetBrains IDE.

Полный список инструментов — [docs: Tools](https://oraios.github.io/serena/01-about/035_tools.html). В одной конфигурации активно только подмножество; _optional_ = выключен по умолчанию; [BETA] = свежие.

**symbol_tools (ядро, LSP-бэкенд):**
- `find_symbol` — глобальный/локальный поиск символов;
- `find_declaration`, `find_implementations`, `find_referencing_symbols`;
- `get_symbols_overview` — outline верхнеуровневых символов файла;
- `get_diagnostics_for_file` (+ optional `get_diagnostics_for_symbol`);
- `rename_symbol` — rename через LSP по всей кодобазе;
- `replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol`, `safe_delete_symbol` — символьное редактирование;
- `restart_language_server` (optional).

**file_tools (базовые):** `read_file`, `create_text_file`, `find_file`, `list_dir`, `search_for_pattern`, `replace_content`, `replace_in_files`; optional: `insert_at_line`, `replace_lines`, `delete_lines`. В контекстах `ide`/`claude-code` эти инструменты по умолчанию выключены, т.к. дублируют возможности харнесса ([README, Basic Features](https://github.com/oraios/serena/blob/main/README.md)).

**memory_tools:** `write_memory`, `read_memory`, `edit_memory`, `list_memories`, `delete_memory`, `rename_memory` — проектная «память» в Markdown-файлах (см. §4).

**workflow_tools:** `onboarding` (первичное изучение проекта с записью memories), `initial_instructions`, `serena_info` (optional).

**config_tools:** `activate_project`, `get_current_config`; optional: `open_dashboard`, `remove_project`.

**cmd_tools:** `execute_shell_command`.

**query_project_tools (optional):** `query_project`, `list_queryable_projects` — read-only запросы к другим проектам Serena.

**jetbrains_tools (только платный бэкенд, нам не relevant):** `jet_brains_find_*`, `jet_brains_rename`, `jet_brains_move` [BETA], `jet_brains_inline_symbol` [BETA], `jet_brains_safe_delete` [BETA], `jet_brains_type_hierarchy`, `jet_brains_run_inspections`, `jet_brains_debug` [BETA].

Сравнение возможностей бэкендов — таблицы в [README, Features](https://github.com/oraios/serena/blob/main/README.md): на LSP-бэкенде доступны find symbol/overview/referencing symbols, declaration/implementations (зависит от LS), diagnostics, rename (только символы) и всё символьное редактирование; move/inline/type hierarchy/propagate deletions — только JetBrains.

## 2. Поддерживаемые языки и языковые серверы

Заявлено 40+ языков из коробки, поддержка **нескольких языковых серверов параллельно** для полиглотных проектов ([docs: Language Support](https://oraios.github.io/serena/01-about/020_programming-languages.html)). Полный список ключей языков — enum `LanguageServerId` в [src/solidlsp/ls_config.py](https://github.com/oraios/serena/blob/main/src/solidlsp/ls_config.py) и шаблон [project.template.yml](https://github.com/oraios/serena/blob/main/src/serena/resources/project.template.yml).

**TypeScript / JavaScript** (ключ `typescript`; JavaScript обслуживается тем же сервером — «use language `typescript` for both»):
- Сервер: `typescript-language-server` (npm), поверх него `tsserver`. Serena сама ставит pinned-версии **typescript 5.9.3 + typescript-language-server 5.1.3** через npm в собственную managed-директорию (не в проект); версии и npm-registry переопределяются через `ls_specific_settings.typescript` ([typescript_language_server.py](https://github.com/oraios/serena/blob/main/src/solidlsp/language_servers/typescript_language_server.py)).
- Требование: **Node.js и npm в PATH** (assert в коде; у нас выполнено — frontend/admin на Node).
- Нюансы реализации: отключён Automatic Type Acquisition (`disableAutomaticTypingAcquisition`), т.е. типы берутся только из установленных в проекте пакетов; `.tsx`/`.jsx` корректно мапятся на `typescriptreact`/`javascriptreact`; директории `node_modules`, `dist`, `build` игнорируются; реализовано ожидание индексации tsserver через `$/progress` с таймаутами (по умолчанию `indexing_timeout: 30s`, `server_ready_timeout: 10s`).
- Альтернативный бэкенд: ключ `typescript_vts` — `@vtsls/language-server` ([vtsls](https://github.com/yioneko/vtsls), нативно собранный VS Code TS extension) ([ls_config.py:205](https://github.com/oraios/serena/blob/main/src/solidlsp/ls_config.py), [docs: Security, npm installs](https://oraios.github.io/serena/02-usage/070_security.html)).
- TypeScript **не** помечен experimental (в отличие от Angular, HTML, JSON, Solidity, SCSS, Deno; Kotlin LS — «pre-alpha»).

**Go** (ключ `go`):
- Сервер: **gopls**, причём Serena его **не скачивает** — «requires installation of `gopls`» ([docs: Language Support](https://oraios.github.io/serena/01-about/020_programming-languages.html); подтверждено в [docs: Security — «No Automatic Download»](https://oraios.github.io/serena/02-usage/070_security.html)). У нас gopls уже установлен и используется одноимённым MCP-сервером.
- Настройки пробрасываются в gopls через `ls_specific_settings.go.gopls_settings` (buildFlags, env) ([docs: Configuration — Go](https://oraios.github.io/serena/02-usage/050_configuration.html)).

## 3. Пересечение с lean-ctx и gopls

Текущий инструментарий репо: `lean-ctx` (BM25+embeddings поиск, сжатые чтения, граф зависимостей, session intelligence) и `gopls` MCP (Go: diagnostics, search, references, rename, package API, workspace, vulncheck).

| Функция Serena | Дубль в lean-ctx | Дубль в gopls MCP | Уникальность Serena |
|---|---|---|---|
| `find_symbol` | `ctx_search action=symbol`, `ctx_compose` | `go_search` (только Go) | TS/JS |
| `find_referencing_symbols` | `ctx_search action=find_related` | `go_symbol_references` (только Go) | TS/JS; LSP-точность |
| `get_diagnostics_for_file` | — | `go_diagnostics` (только Go) | TS/JS |
| `rename_symbol` | — | `go_rename_symbol` (только Go) | TS/JS |
| `get_symbols_overview` | `ctx_read mode=map/signatures` | `go_package_api` (пакетно) | per-file outline |
| `replace_symbol_body`, `insert_before/after_symbol`, `safe_delete_symbol` | — | — | **уникально для всего стека** |
| `search_for_pattern`, `list_dir`, `find_file`, `read_file`, `execute_shell_command` | `ctx_search regex`, `ctx_glob`, `ctx_read`, `ctx_shell` + встроенные Grep/Glob/Read/Bash | — | дубль (в контекстах `ide`/`claude-code` отключены самой Serena) |
| `write/read_memory`, `onboarding` | `ctx_session` (session intelligence) + AGENTS.md/CONTEXT.md в репо | — | частично: формализованный onboarding и versioned memories уникальны как механизм, но зона ответственности уже занята |

Уникально для Serena в нашем контуре: **вся TS/JS-семантика** (сейчас пусто) и **символьное редактирование** (атомарная замена тела функции/класса, вставки до/после символа, safe delete) — этого нет ни в lean-ctx, ни в gopls MCP.

**Риск «двух дублирующих команд» реален, но управляем.** Сама Serena это признаёт: контексты `ide`/`claude-code` выключают базовые file/search/shell-инструменты, чтобы не дублировать харнесc ([docs: Configuration — Contexts](https://oraios.github.io/serena/02-usage/050_configuration.html)), а доки рекомендуют выбирать базовый набор инструментов через конфигурацию, а не интерфейс клиента ([docs: Clients — Tool Selection](https://oraios.github.io/serena/02-usage/030_clients.html)). Механизмы: `excluded_tools` / `included_optional_tools` / `fixed_tools` в `project.yml` ([project.template.yml](https://github.com/oraios/serena/blob/main/src/serena/resources/project.template.yml)). Для нашего репо очевидная обрезка: `languages: ["typescript"]` (Go остаётся за gopls MCP) + исключение file/shell-инструментов (они есть у харнесса и lean-ctx). Известный смежный симптом: в Claude Code агенты склонны игнорировать MCP-инструменты в пользу встроенных, Serena компенсирует это reminder-хуками ([docs: Clients — Claude Code](https://oraios.github.io/serena/02-usage/030_clients.html)); для Kimi Code/ZCode таких хуков нет, поведение не проверено.

## 4. Цена эксплуатации

**Требования к окружению** ([docs: Installation](https://oraios.github.io/serena/02-usage/010_installation.html)):
- `uv` — единственный обязательный prerequisite; Python 3.13 подтягивается uv'ом: `uv tool install -p 3.13 serena-agent`, затем `serena init`. Обновление — `uv tool upgrade serena-agent`.
- Для TS: Node.js + npm в PATH (есть). Для Go: локальный gopls (есть).
- Supply chain: для скачиваемых архивов — pinned версии + SHA256 + host allowlist; для npm-пакетов (вкл. typescript) — pinned версии в Serena-managed директории, но **без lockfile/`npm ci`** и с доверием к пользовательскому npm-конфигу ([docs: Security](https://oraios.github.io/serena/02-usage/070_security.html)).

**Куда пишется состояние:**
- Глобально: `~/.serena/` — `serena_config.yml`, файлы языковых серверов, логи, глобальные memories (`~/.serena/memories/global/`); переносится переменной `SERENA_HOME` ([docs: Configuration — Serena Data Directory](https://oraios.github.io/serena/02-usage/050_configuration.html)).
- В проекте: `.serena/` в корне — `project.yml` (**задуман как версионируемый**), `project.local.yml` (локальные оверрайды), `memories/` (Markdown, версионируются по дизайну), `cache/` (кэш символов, `cache/<language>` — [src/solidlsp/ls.py:349,551](https://github.com/oraios/serena/blob/main/src/solidlsp/ls.py)).
- **В .gitignore репозитория ничего добавлять не нужно**: Serena при создании проекта сама пишет `.serena/.gitignore` с `/cache` и `/project.local.yml` ([src/serena/project.py:64-71](https://github.com/oraios/serena/blob/main/src/serena/project.py)). Осознанное решение требуется только о коммите `.serena/project.yml` и `.serena/memories/` (это параллельная версионируемая система знаний рядом с существующими AGENTS.md/CONTEXT.md). Расположение `.serena` можно вынести из репо через `project_serena_folder_location` в глобальном конфиге.
- `ignore_all_files_in_gitignore: true` по умолчанию — Serena уважает .gitignore проекта ([project.template.yml](https://github.com/oraios/serena/blob/main/src/serena/resources/project.template.yml)).

**Индексация** ([docs: Workflow — Indexing](https://oraios.github.io/serena/02-usage/040_workflow.html)):
- Опциональная: `serena project index` пре-кэширует символы через языковой сервер, убирая задержку первого символьного запроса. Запускается один раз; дальше индекс обновляется автоматически при изменении файлов. Без неё работает, но первый запрос медленнее.

**Память/CPU:**
- Сама Serena — Python-процесс; основное потребление — дочерние языковые серверы (для TS это node/tsserver). Документация чисел не даёт.
- Конкретный измеренный кейс из issue [#1814](https://github.com/oraios/serena/issues/1814) (открыт, 2026-08-05): на монорепо elastic/kibana (~190k TS-файлов) tsserver при первом кросс-файловом запросе референсов вырос до ~4 ГБ RSS и умер по V8 heap OOM; Serena при этом **молча вернула `{}` с `isError: false`**, и последующие `find_referencing_symbols` «быстро и уверенно» возвращали пусто — худший failure mode для агента. Масштаб нашего репо на порядки меньше, но форму отказа надо иметь в виду.
- Соседний кейс (Rust, закрыт): rust-analyzer 40+ ГБ на больших workspace — [#1556](https://github.com/oraios/serena/issues/1556).
- Onboarding читает много файлов и заполняет контекст — доки советуют после него начинать новую сессию ([docs: Memories & Onboarding — Tips](https://oraios.github.io/serena/02-usage/045_memories.html)).

## 5. Подключение как MCP-сервер

- Транспорт: stdio (по умолчанию, клиент запускает процесс) или HTTP/SSE (shared instance для нескольких агентов) ([docs: Clients — General](https://oraios.github.io/serena/02-usage/030_clients.html)).
- Команда запуска: `serena start-mcp-server --context <ctx> [--project <path|name> | --project-from-cwd]`. Универсальный конфиг для MCP-клиента:
  ```json
  {
    "mcpServers": {
      "serena": {
        "command": "serena",
        "args": ["start-mcp-server", "--context", "ide", "--project", "/abs/path/to/repo-root"]
      }
    }
  }
  ```
- Контексты: `desktop-app` (default, полный набор инструментов), `ide` / `claude-code` / `codex` / `grok` (single-project, дублирующие инструменты выключены), `agent` и др. Для Kimi Code/ZCode подходит `ide` (доки прямо рекомендуют его для «других терминальных клиентов и IDE-ассистентов»).
- Проект = директория на диске. Создание: `serena project create` (автодетект языков, генерация `.serena/project.yml`) или неявно при первой активации. Активация: `--project` при старте сервера, либо инструментом `activate_project` в сессии (в single-project контекстах отключён, если проект задан при старте) ([docs: Workflow](https://oraios.github.io/serena/02-usage/040_workflow.html)).
- Монорепо/несколько проектов ([docs: Workflow — Multiple Projects](https://oraios.github.io/serena/02-usage/040_workflow.html)):
  - Рекомендуемый паттерн — открыть **корень монорепо как один проект**; в `languages` перечислить все нужные языки (несколько LS работают параллельно; для каждого файла используется первый подходящий сервер).
  - `ls_workspace_folders` — сузить индексацию до подпапок (например `apps/frontend`); `ls_additional_workspace_folders` — кросс-пакетные ссылки без индексации.
  - Известное ограничение: `activate_project` переключает и memory-контекст, и LSP-корень одновременно — per-project memories внутри большого монорепо неудобны (issue [#1260](https://github.com/oraios/serena/issues/1260), открыт).
  - Несколько агентов к одному инстансу — HTTP-режим; разные проекты — отдельные процессы Serena.
- Важно из README: **не ставить Serena через MCP/plugin marketplaces** — там устаревшие команды установки; только официальный Quick Start.

## 6. Лицензия и зрелость

- **Лицензия MIT**, бесплатно; LSP-бэкенд полностью open-source. Платная часть — JetBrains plugin (нам не нужен). (GitHub API: `license: MIT`, [репозиторий](https://github.com/oraios/serena).)
- Активность (GitHub API на 2026-08-16): создан 2025-03-23; **28 087 stars**, 1 873 forks, 104 open issues; последний push 2026-08-14; последний релиз **v1.7.0 (2026-08-09)**; ~77 merged PR и 100+ коммитов за последние 30 дней; 100+ контрибьюторов (верхняя граница выборки API) ([releases](https://github.com/oraios/serena/releases)). Разработка очень активная; обратная сторона — быстрая смена команд/конфигов (предупреждение README про устаревшие marketplace-установки это подтверждает).
- Известные ограничения/риски:
  - [#1814](https://github.com/oraios/serena/issues/1814) (open): silent empty refs после OOM tsserver (см. §4);
  - [#1251](https://github.com/oraios/serena/issues/1251) (open, 2026-03-30): security findings — memory path traversal, shell command timeout, cross-project memory poisoning;
  - [#1260](https://github.com/oraios/serena/issues/1260) (open): связка memory/LSP-контекста в монорепо;
  - часть языков experimental (Angular, HTML, JSON, SCSS, Solidity, Deno…), Kotlin LS pre-alpha;
  - security-модель: локальная машина, MCP-клиент, репозиторий и конфиги считаются trusted; есть `execute_shell_command` и пишущие инструменты — рекомендован sandbox; сетевые сервисы по умолчанию слушают только localhost ([docs: Security](https://oraios.github.io/serena/02-usage/070_security.html)).

## 7. Альтернативы того же класса

| Инструмент | Лицензия / активность | Класс | Ключевые отличия от Serena |
|---|---|---|---|
| [isaacphi/mcp-language-server](https://github.com/isaacphi/mcp-language-server) | BSD-3-Clause; ~1.6k stars; последний push 2026-03-01 | Тонкий MCP поверх любого stdio LSP (Go-бинарь) | Минимализм: definition/references/diagnostics/rename/hover; гайды для gopls и typescript-language-server; LS ставишь сам. Нет символьного редактирования, memories, onboarding, dashboard. Меньше возможностей — меньше дублирования с lean-ctx. |
| [mizchi/lsmcp](https://github.com/mizchi/lsmcp) | MIT; ~450 stars; последний push 2025-10-27 | MCP поверх LSP, написан на TypeScript, TS-центричный | Детальный состав инструментов не проверялся; активность заметно ниже (почти год без коммитов). |
| [zilliztech/claude-context](https://github.com/zilliztech/claude-context) | MIT; ~12.4k stars; активен | **Другой класс**: семантический поиск по коду через embeddings + векторную БД (Milvus/Zilliz), не LSP | По духу конкурент lean-ctx, а не Serena: не даёт референсов/rename/диагностики. Дефолтные гайды требуют `OPENAI_API_KEY` — конфликтует с ограничением «только бесплатные/локальные». |

## Пробелы и ограничения исследования

1. **Производительность на репо нашего масштаба не измерялась** — числа по памяти есть только из чужого issue #1814 (~190k TS-файлов). Перед adopt имеет смысл локальный прогон на `apps/frontend` + `apps/admin`.
2. **Поддержка Kimi Code/ZCode напрямую не документирована**; ожидается работа через generic MCP stdio + контекст `ide` — не проверено.
3. **Совместимость с нашим стеком не проверялась**: pinned typescript 5.9.3 vs `typescript: ^5` в `apps/frontend` (Next.js 16), работа typescript-language-server с несколькими tsconfig в монорепо (frontend + admin одним сервером), поведение gopls под `go.work` — всё требует локальной проверки.
4. Поведение агента Kimi Code с инструментами Serena (будет ли использовать без reminder-хуков, как в Claude Code) — не проверено.
5. Состав инструментов `lsmcp` не проверялся глубже метаданных репозитория.

---

## Выводы для решения

Факты «за» (adopt):
- Закрывает конкретную дыру: полный набор symbol-инструментов для TypeScript/JavaScript (навигация, референсы, rename, диагностика) — того, что `apps/frontend/AGENTS.md:29` ждёт от «будущего TS LSP/MCP».
- Единственный в обзоре даёт **символьное редактирование** (`replace_symbol_body`, `insert_before/after_symbol`, `safe_delete_symbol`) — нет аналога ни в lean-ctx, ни в gopls MCP, ни во встроенных инструментах.
- Бесплатно (MIT), локально, зрелость высокая: 28k stars, релизы еженедельно, 77 merged PR/мес.
- Зависимости минимальны для нашего окружения: uv + Node (уже есть); TS-сервер Serena ставит сама в `~/.serena`, не загрязняя репо.

Факты «против» / цена (reject или отложить):
- Новая зависимость окружения каждого разработчика/агента: uv + Python 3.13; плюс процессы node/tsserver в памяти рядом с уже работающими gopls и lean-ctx.
- Значительное пересечение с lean-ctx (поиск, обзоры, memories/session) и gopls (весь Go-срез): без тщательной обрезки (`languages: ["typescript"]`, `excluded_tools`/`fixed_tools`, контекст `ide`) агент получит 2–3 способа сделать одно и то же. Serena даёт механизмы обрезки, но конфигурацию и обновление AGENTS.md придётся делать и поддерживать самим.
- Появляется вторая версионируемая система знаний (`.serena/project.yml` + `.serena/memories/`) рядом с AGENTS.md/CONTEXT.md — потенциальный источник рассинхрона; memories можно выключить режимом `no-memories`.
- Открытые эксплуатационные риски: молчаливые пустые результаты при падении LS (#1814), открытые security findings (#1251), ограничение монорепо-модели (#1260).
- Команды/конфиги быстро меняются (README предупреждает об устаревших инструкциях в marketplaces) — стоит пиновать версию и держать ссылку на официальные доки.

Промежуточный вариант: если нужна только TS-навигация без редактирования и памяти — минималистичный `isaacphi/mcp-language-server` (тонкая обёртка над typescript-language-server) дублируется с lean-ctx гораздо меньше, но и возможностей меньше.

Решение (adopt / reject / adopt-ограниченно) — за человеком по тикету #290.
