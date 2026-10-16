# Research: инверсия фолбэка в Caddy — порядок handle-блоков, snippets с аргументами, раскатка конфига из CI

- Тикет: #651 (карта #649 «Прокси-схема Caddy — инверсия фолбэка»)
- Дата: 2026-09-14
- Вопрос: подтвердить по официальной документации Caddy v2 и первоисточникам, как надёжно выразить контракт «лендинг = исключение (`/`, `/assets/*`, `/landing-fonts/*`), фронт-кабинет = catch-all», параметризовать stage/prod портами через snippets и раскатывать Caddyfile из CI без даунтайма и без риска потерять текущее поведение.
- Метод: первоисточники — канонические markdown-исходники официальной документации (репозиторий `caddyserver/website`, путь `src/docs/markdown/` — ровно то, что рендерит caddyserver.com/docs), исходники Caddy (`caddyserver/caddy`: `caddyconfig/caddyfile/importargs.go`, `parse.go` по тегам v2.6.4/v2.7.0/v2.8.0/master, официальный юнит `caddyserver/dist@master/init/caddy.service`), docs/api. Живые факты: сервер `alterix-server` (ssh, read-only) — `caddy version`, статус юнита. Локальные факты: `apps/landing/{index.html,nginx.conf,public,dist,src/styles/fonts.css}`, `docs/deployment.md` § Caddyfile.

## 0. База (факты на 2026-09-14)

- **Сервер: Caddy v2.11.4**, systemd-юнит `caddy.service` активен (`ExecStart=/usr/bin/caddy run --environ --config /etc/caddy/Caddyfile`). Официальный юнит из `caddyserver/dist` задаёт `ExecReload=/usr/bin/caddy reload --config /etc/caddy/Caddyfile --force` — то есть `systemctl reload caddy` = `caddy reload --force` против живого админ-API.
- Текущий Caddyfile (копия в `docs/deployment.md` § Caddyfile): whitelist `@frontend path /login* /dashboard* ...` → фронт 13000, catch-all → лендинг 13002; `handle_path /api/*` и `handle /webhooks/*` → бэкенд 18080; `encode zstd gzip` на уровне сайта.
- Лендинг (проверено по коду `apps/landing`): Vite со стандартным `base: '/'` и `assetsDir: 'assets'` → бандлы и хэшированные картинки в `/assets/*`; шрифты подключаются абсолютными URL `/landing-fonts/manrope-variable.woff2` (`src/styles/fonts.css`); favicon лежит в `public/icon.png` и подключён как `<link rel="icon" href="/icon.png" />` (`index.html:10`) — живая коллизия: `/icon.png` попадает во фронт, не в лендинг (решение карты #649: favicon переезжает на `/assets/*`).
- nginx лендинга (`apps/landing/nginx.conf`): `location /assets/` → `Cache-Control: public, max-age=31536000, immutable`; `location /` → SPA-fallback `try_files $uri $uri/ /index.html` с `Cache-Control: no-cache`. То есть **любой** путь, ушедший в лендинг-контейнер, вернёт index.html с 200 — после инверсии в лендинг должны уходить только перечисленные исключения, всё остальное — во фронт.
- Next.js-фронт: корневой `app/page.tsx` — redirect на `/properties`; неизвестные top-level пути фронт честно отвечает 404.

---

## 1. Семантика и порядок handle-блоков (Q1)

### Что документировано

- `handle` — «Evaluates a group of directives **mutually exclusively** from other `handle` blocks at the same level of nesting… when multiple `handle` directives appear in sequence, **only the first _matching_ `handle` block will be evaluated**. A handle with no matcher acts like a _fallback_ route» ([docs: handle](https://caddyserver.com/docs/caddyfile/directives/handle)).
- Порядок документирован и детерминирован: «The `handle` directives are **sorted according to the [directive sorting algorithm](https://caddyserver.com/docs/caddyfile/directives#sorting-algorithm) by their matchers**». Сам алгоритм ([docs: sorting algorithm](https://caddyserver.com/docs/caddyfile/directives#sorting-algorithm)):
  1. Разноимённые директивы сортируются по позиции в default order (`handle` и `handle_path` стоят рядом в группе «special routing & dispatching directives»).
  2. Одноимённые — по матчерам: **высший приоритет у директивы с одиночным path-матчером** (по длине пути; исключение — `*`-версия менее специфична: `/foo` > `/foo*`, `/foo/*` > `/foo*`); далее **все остальные матчеры — multi-value path и named — в порядке следования в Caddyfile**; **директива без матчера — всегда последняя**.
  3. «The contents of the `route` directive ignores all the above rules, and preserves the order the directives appear within».
- `handle_path` — «Works the same as the `handle` directive, but implicitly uses `uri strip_prefix` to strip the matched path prefix»; в сортировке «sorts at the same priority as a `handle` with a path matcher» ([docs: handle_path](https://caddyserver.com/docs/caddyfile/directives/handle_path)).
- Path-матчер: «Path matching is an **exact match by default**, not a prefix match»; префикс — только через хвостовой `*`; «Slashes are significant» (`/assets/*` не матчит `/assets` и `/assetsfoo`? — матчит `/assetsfoo` нельзя: `*` после `/` требует слеша; а вот `/assets*` матчил бы и `/assetsfoo` — важный выбор в пользу `/assets/*`); матч регистронезависимый; путь чистится от `..` и схлопывания слешей до матча ([docs: path matchers](https://caddyserver.com/docs/caddyfile/matchers#path-matchers)). `path /` матчит **только** `/` (квери-стринг в путь не входит).

### Что это значит для инверсии

Выражение «точный `/` + `/assets/*` → лендинг, всё остальное → фронт» — это ровно идиоматичный Caddy:

```caddyfile
@landing path / /assets/* /landing-fonts/*
handle @landing {
	reverse_proxy 127.0.0.1:13002
}
handle {
	reverse_proxy 127.0.0.1:13000
}
```

- **Полагаться на сортировку можно**: правило «named matcher — в порядке следования, no-matcher — последним» явно документировано, порядок `@landing` → fallback гарантирован. Для читаемости всё равно писать блоки в порядке приоритета; если хочется железной явности — обернуть в `route` (порядок литеральный), но это не обязательно и в существующем Caddyfile не используется.
- **Для `/assets/*` — `handle`, не `handle_path`**: `handle_path` снял бы префикс, а nginx-контейнер лендинга ждёт полный путь (`/assets/index-<hash>.js` — ровно так Vite их кладёт и `location /assets/` в nginx.conf так и ждёт).
- Ловушка `path /login*` из старой схемы: `/login*` матчит и `/loginfoo`; для лендинга это неактуально (`/` — точный, `/assets/*`/`/landing-fonts/*` — со слешем перед `*`), но помнить при любых будущих правках матчеров.
- `handle_errors` тут не нужен — это маршрутизация ошибок, не запросов.

---

## 2. Snippets с аргументами (Q2)

### Канонический синтаксис (проверено по исходникам, не по блогам)

Сниппет — блок с именем в скобках на верхнем уровне Caddyfile, вызывается через `import`; аргументы подставляются плейсхолдерами ([docs: concepts#snippets](https://caddyserver.com/docs/caddyfile/concepts#snippets)):

```caddyfile
(snippet) {
	respond "Yahaha! You found {args[0]}!"
}
a.example.com {
	import snippet "Example A"
}
```

История и статус форм — **по исходникам `caddyserver/caddy`** (`caddyconfig/caddyfile/importargs.go`, `parse.go`):

| Форма | Статус | С какого релиза | Источник |
|---|---|---|---|
| `{args.0}` (точка) | **deprecated**, работает с warning в лог: «Placeholder {args.0} deprecated, use {args[0]} instead» | v2.1.0 (ввёл, PR #3423) | `parse.go` v2.6.4: `repl.Set("args."+index, …)`; `importargs.go`: `argsRegexpIndexDeprecated = args\.(.+)` + TODO «Remove the deprecated {args.*} placeholder support» |
| `{args[0]}` (скобки) | **канон** | v2.7.0 (вместе с вариадиками, PR #5249, milestone v2.7.0) | `importargs.go`: `argsRegexpIndex = args\[(.+)]` |
| `{args.name}` (именованные) | **не существуют**: regexp `args\.(.+)` ловит `args.name`, `strconv.Atoi` падает → warning «invalid index», подстановки нет | — | `importargs.go`, ветка deprecated |
| `{args[1:]}` / `{args[a:b]}` (вариадик) | работает, но **только отдельным токеном** («Variadic placeholder … must be a token on its own») | v2.7.0 | `importargs.go: parseVariadic`; фикс false-positive на токенах с `:` — PR #5883 |

Ограничения и практика:

- Подстановка — текстовая замена внутри токена, поэтому **`reverse_proxy 127.0.0.1:{args[0]}` работает** (плейсхолдер внутри `host:port`). Аргумент — один токен: аргумент с пробелом разорвёт токенизацию; для портов это неактуально.
- `import` нельзя использовать как аргумент другой директивы («As a special case, it can appear anywhere within the Caddyfile (except as an argument to another directive)») — то есть `import {args[0]}` невозможен; динамический выбор сниппета не сделать ([docs: import](https://caddyserver.com/docs/caddyfile/directives/import)).
- **Вложенные import** (сниппет внутри импортируемого файла/сниппета) поддерживаются — в предупреждениях парсера трекается `import_chain` по цепочке импортов; открытых багов подмены args во вложенных импортах поиском по трекеру (`in:title import args`) не найдено.
- Передача необязательного **блока** в сниппет (`{block}`) — экспериментальная фича v2.9.x+, нам не нужна.
- Named-матчер (`@landing`), определённый внутри сниппета, безопасен: сниппет разворачивается в каждый site-блок своей копией токенов, named-матчеры скоупятся site-блоком.
- Сниппеты принято объявлять в начале файла до site-блоков; имя сниппета не должно совпадать с существующим файлом (fallback: «If the argument does not match a defined snippet, it will be tried as a file»).

**Для нашей версии v2.11.4 канон — `{args[0]}`; писать `{args.0}` не надо (deprecated-предупреждение в логах), named-аргументов не существует.**

---

## 3. Раскатка конфига из CI (Q3)

### Семантика validate/reload (документировано)

- `caddy validate` — «deserializes the config, then **loads and provisions all of its modules as if to start the config**, but the config is not actually started… a stronger error check than merely serializing a config as JSON» ([docs: command-line#validate](https://caddyserver.com/docs/command-line)). Ловит не только синтаксис, но и ошибки провижининга.
- `caddy reload` — «the correct, semantic way to change/reload the running configuration»; работает через админ-API (равносильно `POST /load`), поэтому админ-эндпоинт (по умолчанию `localhost:2019`) должен быть включён ([docs: command-line#reload](https://caddyserver.com/docs/command-line)).
- Zero-downtime и откат — прямая цитата [docs/api#post-load](https://caddyserver.com/docs/api): «Sets Caddy's configuration, overriding any previous configuration. It blocks until the reload completes or fails. Configuration changes are lightweight, efficient, and **incur zero downtime**. **If the new config fails for any reason, the old config is rolled back into place without downtime.**» Если конфиг идентичен текущему — reload пропускается (форсируется флагом `--force` или заголовком `Cache-Control: must-revalidate`).
- Итого при невалидном конфиге: `caddy reload`/`POST /load` возвращает ошибку (ненулевой exit code → CI-джоба красная), **работающий инстанс продолжает жить на старом конфиге**. systemd пометит reload как неудавшийся, но сервис остаётся active — даунтайма нет.
- Нюанс сигналов: SIGUSR1-перезагрузка доступна, только если процесс стартовал `caddy run` с конфиг-файлом, и **отключается**, если конфиг хоть раз менялся через API (`caddy reload`) или если reload идёт с другим именем файла/адаптером. Наш путь — всегда `systemctl reload caddy` (= `caddy reload --config /etc/caddy/Caddyfile --force`), смешивать с SIGUSR1 не надо.

### Подмена файла: scp поверх vs tmp+mv

- Caddy читает файл конфига **один раз, в момент вызова** `caddy reload` (и `caddy validate`). Демон сам файлы не вотчит. Поэтому «scp поверх живого» безопасен ровно до тех пор, пока между началом записи и reload нет параллельного reload (человек/таймер). 
- **tmp+mv — предпочтительный паттерн**: загрузка в соседний файл в той же ФС + `install -m 644`/`mv` — атомарный `rename(2)`, никакой параллельный читатель никогда не увидит частичный файл; плюс не меняет владельца/контекст на полпути. Это общая инженерная практика атомарной подмены конфигов (rename-атомарность POSIX), а не специфика Caddy.
- Стехастический нюанс: юнит имеет `PrivateTmp=true` — но это namespace **сервиса**; CLI-вызовы (`sudo caddy validate/reload`) идут вне него, а файл конфига живёт в `/etc/caddy/`, так что коллизии нет. Просто стейдж-файл класть рядом (`/etc/caddy/Caddyfile.new`), не в `/tmp` сервиса.

### Принятая последовательность для CI (рекомендация)

```bash
# 1. Аплоад атомарно (имя, начинающееся с "Caddyfile", сохраняет автоопределение адаптера)
scp Caddyfile alterix-server:/etc/caddy/Caddyfile.new
ssh alterix-server 'sudo install -m 644 -o root -g root /etc/caddy/Caddyfile.new /etc/caddy/Caddyfile.candidate'
# 2. Валидация НА СЕРВЕРЕ бинарём той же версии, что работает (v2.11.4)
ssh alterix-server 'sudo caddy validate --config /etc/caddy/Caddyfile.candidate'
# 3. Бэкап текущего (откат #649) и атомарная установка
ssh alterix-server 'sudo cp /etc/caddy/Caddyfile /etc/caddy/Caddyfile.prev && sudo mv /etc/caddy/Caddyfile.candidate /etc/caddy/Caddyfile'
# 4. Graceful reload (zero-downtime; при ошибке — старый конфиг живёт)
ssh alterix-server 'sudo systemctl reload caddy'
# 5. Smoke-проверки (раздел «Что проверить живьём»), при провале:
#    sudo install -m 644 /etc/caddy/Caddyfile.prev /etc/caddy/Caddyfile && sudo systemctl reload caddy
```

- Валидировать именно **на сервере** бинарём сервера — исключает рассинхрон версий между CI-проверкой и фактическим reload'ом. Опционально доапгрейдить быстрым префлайтом в CI `docker run caddy:2.11.4 caddy validate` (пин как у сервера) — ловит грубые ошибки до ssh.
- В CI также стоит гейтить формат: `caddy fmt --overwrite` + `git diff --exit-code` (каноническое форматирование Caddyfile, той же версией).
- Это расширяет существующее правило репозитория «на сервере нет репозитория и ничего не правится руками» (`docs/deployment.md`) на Caddyfile; ручной сценарий из доки (`validate && systemctl reload`) остаётся тем же самым, меняется только источник файла.

---

## 4. Альтернатива «404-fallback»: возможна ли и чем хуже инверсии (Q4)

### Технически — возможна, но не через handle_errors

- **`handle_errors` не ловит ответы upstream**: «some directives like `reverse_proxy` that write an error-class status response **will not trigger the error routes**» ([docs: handle_errors](https://caddyserver.com/docs/caddyfile/directives/handle_errors)). Наивная схема «прокси во фронт + handle_errors 404 → лендинг» **не работает**: 404, написанный Next.js, — это обычный ответ, не Caddy-ошибка.
- Рабочий паттерн — **`intercept`** («a generalized abstraction of the response interception… may be used with any handler that produces responses»): ответ буферизуется («the original response body is held back»), матчится response-матчером и перенаправляется в `handle_response` ([docs: intercept](https://caddyserver.com/docs/caddyfile/directives/intercept)):

```caddyfile
handle {
	intercept {
		@notfound status 404
		handle_response @notfound {
			rewrite * /
			reverse_proxy 127.0.0.1:13002   # отдаём лендинг; статус при желании — replace_status
		}
	}
	reverse_proxy 127.0.0.1:13000
}
```

  (внутрь самого `reverse_proxy` можно и его собственные `handle_response`/`replace_status` — `intercept` это обобщение того же механизма).

### Почему инверсия лучше для нашего случая

1. **Семантика 404 сохраняется.** После инверсии неизвестный путь кабинета честно получает 404 от Next.js — корректно и для пользователей, и для SEO. 404-fallback либо маскирует 404 (лендинг с 200 = soft-404 на всех опечатках — плохо для поисковиков), либо требует `replace_status`-хирургии статусов и заголовков.
2. **Маскировка багов**: перехват 404 прячет реальные поломки роутинга/ассетов фронта — каждая ошибка выглядит «лендингом».
3. **Цена рантайма**: intercept буферизует тело каждого 404-ответа и делает второй upstream-хоп; инверсия — статическое решение на краю, ноль лишних хопов на основном трафике.
4. **Сложность vs. выгода**: лендинг — одна страница; её исключение — это `path / /assets/* /landing-fonts/*`, три литерала, читаемых за секунду. 404-fallback — два upstream'а в цепочке, буферизация, заголовочная хирургия и скрытая связь между фронтовыми роутами и маркетинговым фолбэком.

**Вердикт: инверсия; 404-fallback фиксируется как отвергнутая альтернатива с указанной причиной.**

---

## 5. Подводные камни, которые надо сохранить при переработке (Q5)

| # | Что | Почему и как сохранить |
|---|---|---|
| 1 | `encode zstd gzip` на уровне сайта | Стоит в default order в группе middleware **до** `handle`-диспетчеров, оборачивает всю цепочку независимо от перестановки handle-блоков. Дефолты оставляем как есть: сжатие по `Accept-Encoding` (при равном q — первый в списке), `minimum_length` 512 байт, дефолтный список Content-Type (включая `application/javascript`, шрифты) ([docs: encode](https://caddyserver.com/docs/caddyfile/directives/encode)). Caddy **не пишет** `Cache-Control` сам — кеш-заголовки приходят от upstream'ов |
| 2 | `handle_path /api/*` — с префикс-стриппингом | Бэкенд ждёт пути без `/api` (смoke `/api/healthz` в `docs/deployment.md` подтверждает именно стриппинг). Не заменить на `handle` |
| 3 | `/webhooks/*` — **без** снятия префикса | T-Kassa присылает колбэки на полный путь `/webhooks/...`; остаётся `handle /webhooks/*` → бэкенд |
| 4 | `sw.js` — `Cache-Control: no-store` от Next.js | После инверсии `/sw.js` уходит во фронт catch-all'ом (как и сейчас через whitelist). `reverse_proxy` проксирует upstream-заголовки как есть; Caddy не переписывает `Cache-Control`, пока на пути нет директив `header` — **никаких header-правил на `/sw.js` не добавлять** (чек ADR 0032: `curl -I` на stage). `Content-Type` должен остаться JS-MIME |
| 5 | `/manifest.webmanifest`, `/offline.html`, `/icons/*`, `/_next/*`, `/fonts/*`, `/images/*` и статики фронта | Автоматически уходят во фронт catch-all'ом — whitelist-строка удаляется целиком, отдельные правила не нужны. Это и есть цель инверсии: новые top-level роуты Next.js работают без правок Caddy |
| 6 | Favicon-коллизия `/icon.png` | Сейчас `landing/index.html:10` ссылается на `/icon.png`, который (и сейчас, и после инверсии) достаётся фронту. По решению карты #649 favicon лендинга переезжает на `/assets/*` (`/assets/icon.png`) — правку `apps/landing` делать в том же изменении, что и Caddyfile |
| 7 | `/landing-fonts/*` | Обязательная часть исключения лендинга: `src/styles/fonts.css` подключает шрифты абсолютным путём; nginx отдаёт их через `location /` (no-cache) — путь должен остаться в матчере `@landing` |
| 8 | Кеш-политики лендинга | `location /assets/` → `immutable, max-age=31536000`; `location /` → `no-cache`. После инверсии `/assets/*` лендинга получает свой immutable-путь — заголовки ставит nginx, Caddy их не трогает |
| 9 | `www.rentlee.ru` → redir, admin-блоки | Не трогаются (админка уже catch-all и корректна). TLS/ACME не конфигурируются вручную — automatic HTTPS продолжит работать |
| 10 | Порядок выражений в матчере | `path / /assets/* /landing-fonts/*` — multi-value path-матчер (значения OR'ятся). `/` — точный; префиксные — обязательно `/prefix/*` (со слешем), не `/prefix*` |

---

## 6. Рекомендованный скелет Caddyfile (prod + snippet для портов)

Сниппет параметризуется тремя портами: бэкенд, лендинг, фронт. Формат аргументов — `{args[N]}` (канон v2.7+, см. §2). Порядок handle-блоков выписан в порядке документированной сортировки (см. §1) — для читаемости; детерминизм гарантирует алгоритм, а не удача.

```caddyfile
{
	# глобальных опций не требуется; админ-эндпоинт по умолчанию (localhost:2019)
	# нужен для `caddy reload` из пайплайна
}

# Публичный домен кабинета: лендинг — исключение, кабинет — catch-all.
# Аргументы: {args[0]} backend, {args[1]} landing, {args[2]} frontend.
(public_site) {
	encode zstd gzip

	handle_path /api/* {
		reverse_proxy 127.0.0.1:{args[0]}
	}

	handle /webhooks/* {
		# префикс НЕ снимается — T-Kassa шлёт колбэки на полный путь
		reverse_proxy 127.0.0.1:{args[0]}
	}

	# Лендинг — явное исключение: только корень и его Vite-ассеты.
	# handle, НЕ handle_path: nginx-контейнер ждёт полный путь /assets/*
	@landing path / /assets/* /landing-fonts/*
	handle @landing {
		reverse_proxy 127.0.0.1:{args[1]}
	}

	# Всё остальное — кабинет Next.js: новые top-level роуты работают без правок прокси.
	handle {
		reverse_proxy 127.0.0.1:{args[2]}
	}
}

www.rentlee.ru {
	redir https://rentlee.ru{uri} permanent
}

rentlee.ru {
	import public_site 18080 13002 13000
}

dev.rentlee.ru {
	import public_site 28080 23002 23000
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

Примечания:
- `redir`-блок для `www` и оба admin-блока сохранены как есть; при желании их тоже можно сниппетизировать по портам — поведение не меняется, в скоуп тикета не обязательно.
- Сортировка расстановкой блоков повторяет документированный алгоритм: `/webhooks/*` и `@landing` (named/multi-value) идут до безматчерного fallback; `handle_path /api/*` сортируется «at the same priority as a handle with a path matcher», матчи всех блоков попарно не пересекаются — порядок стабилен.
- Раскатка — по последовательности из §3 (upload → `install` → `caddy validate` серверным бинарём → backup+mv → `systemctl reload caddy` → smoke).

---

## 7. Что проверить живьём на stage (dev.rentlee.ru)

1. `curl -sI https://dev.rentlee.ru/` → 200 от лендинга, `id="root"` в теле; `Cache-Control: no-cache` (nginx лендинга).
2. `curl -sI https://dev.rentlee.ru/assets/<реальный-хэш>.js` → 200, `Cache-Control: public, max-age=31536000, immutable`, префикс `/assets/` **не** снят (лендинг отдал бандл, а не index.html).
3. `curl -sI https://dev.rentlee.ru/landing-fonts/manrope-variable.woff2` → 200, `content-type: font/woff2`.
4. Favicon после переезда: `curl -sI https://dev.rentlee.ru/assets/icon.png` → 200 от лендинга; `https://dev.rentlee.ru/icon.png` → 200 от **фронта** (Next), лендинг на него больше не ссылается.
5. `curl -fsS https://dev.rentlee.ru/login | grep -qi '<html'` — кабинет работает catch-all'ом; `curl -sS -o /dev/null -w '%{http_code}' https://dev.rentlee.ru/properties` → 200; произвольный новый top-level путь фронта (если есть в ветке) — 200 **без правок Caddy**.
6. Неизвестный путь: `curl -sS -o /dev/null -w '%{http_code}' https://dev.rentlee.ru/nosuchpath` → 404 от Next.js (не лендинг, не 200).
7. `curl -fsS https://dev.rentlee.ru/api/healthz | grep -q '"status":"ok"'` — префикс `/api` снят; `test "$(curl -sS -o /dev/null -w '%{http_code}' https://dev.rentlee.ru/api/me)" = 401`.
8. `curl -sI https://dev.rentlee.ru/sw.js` → 200; `cache-control: no-store` (не переписан Caddy); `content-type: text/javascript|application/javascript`.
9. `curl -sI https://dev.rentlee.ru/manifest.webmanifest` → 200, `content-type: application/manifest+json`.
10. `curl -sI -H 'Accept-Encoding: gzip, zstd' https://dev.rentlee.ru/login` → `content-encoding: zstd` (или gzip) — encode пережил переработку.
11. Webhook-путь (пассивно): T-Kassa-колбэк приходит на полный `/webhooks/...` — префикс на месте (смоук по логам бэкенда при следующем тестовом платеже).
12. `caddy validate --config /etc/caddy/Caddyfile` после раскатки — чисто; в логах Caddy нет deprecated-warning вида `{args.0} deprecated` (значит, в сниппете канонический `{args[N]}`).

---

## Источники

- Официальная документация (канонические исходники caddyserver.com — репозиторий caddyserver/website):
  - [handle](https://caddyserver.com/docs/caddyfile/directives/handle), [handle_path](https://caddyserver.com/docs/caddyfile/directives/handle_path), [route](https://caddyserver.com/docs/caddyfile/directives/route)
  - [Sorting algorithm / default directive order](https://caddyserver.com/docs/caddyfile/directives#sorting-algorithm)
  - [Path matchers](https://caddyserver.com/docs/caddyfile/matchers#path-matchers), [Snippets и args](https://caddyserver.com/docs/caddyfile/concepts#snippets), [import](https://caddyserver.com/docs/caddyfile/directives/import)
  - [encode](https://caddyserver.com/docs/caddyfile/directives/encode), [handle_errors](https://caddyserver.com/docs/caddyfile/directives/handle_errors), [intercept](https://caddyserver.com/docs/caddyfile/directives/intercept), [error](https://caddyserver.com/docs/caddyfile/directives/error)
  - [caddy validate / reload](https://caddyserver.com/docs/command-line), [Running: systemd, сигналы](https://caddyserver.com/docs/running), [API: POST /load](https://caddyserver.com/docs/api)
- Исходники Caddy (первоисточник синтаксиса args): [caddyconfig/caddyfile/importargs.go@master](https://github.com/caddyserver/caddy/blob/master/caddyconfig/caddyfile/importargs.go) (+ те же файлы по тегам `v2.6.4`, `v2.7.0`, `v2.8.0`); [PR #3423 (args, v2.1.0)](https://github.com/caddyserver/caddy/pull/3423), [PR #5249 (variadics, v2.7.0)](https://github.com/caddyserver/caddy/pull/5249), [PR #5883 (фикс вариадика на `:`)](https://github.com/caddyserver/caddy/pull/5883)
- systemd: [caddyserver/dist@master/init/caddy.service](https://github.com/caddyserver/dist/blob/master/init/caddy.service) (ExecReload с `--force`)
- Внутреннее: `docs/deployment.md` § Caddyfile, PWA-ассеты, Smoke Checks; карта #649 (решения гриллинга: favicon → `/assets/*`); `docs/research/pwa-manifest-installability.md` (кеш-требования sw.js); `docs/adr/0032` (silent SW update); `apps/landing/{index.html,nginx.conf,src/styles/fonts.css,public,dist}`; живой сервер alterix-server (Caddy v2.11.4, systemd-юнит)
