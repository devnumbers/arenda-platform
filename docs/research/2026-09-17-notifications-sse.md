# Research #736: SSE-транспорт — бэк (chi) и фронт (Next.js)

Дата: 2026-09-17. Методология: исследование по первоисточникам (код репозитория, WHATWG HTML Spec, MDN, официальные доки Caddy и Next.js, PostgreSQL docs, инженерные postmortem'ы), каждое утверждение привязано к источнику. Репозиторий не изменялся (док лежит в бросовой ветке `research/notifications-sse`).

---

## 1. Контекст проекта (проверено по коду 17.09.2026)

| Факт | Где в коде/доках |
|---|---|
| Бэк: chi **v5.3.0**, Go **1.26.5** | `apps/backend/go.mod`; `Makefile` (`GO_VERSION`) |
| Цепочка middleware: `otelhttp → RequestID → RealIP → RequestLogger → Recovery → rateLimit(IP 20 rps/burst 40) → bodyLimit → securityHeaders → SessionMiddleware → Readonly` | `apps/backend/internal/platform/httpserver/server.go`; дефолты лимитов — `internal/platform/config/config.go` (`defaultRateLimit`) |
| **`http.Server{WriteTimeout: 30s}`** — жёсткий дедлайн на запись любого ответа, включая стримы | `apps/backend/cmd/api/main.go:244–250` |
| Авторизация: `SessionMiddleware` кладёт userID+actor в контекст; публичные пути — явный список; cookie `__Host-session_id`/`session_id`, `HttpOnly`, `SameSite=Lax`, sliding 7д/30д | `internal/platform/httpsupport/session.go`; ADR 0004/0011/0018 |
| CORS нет вообще — API строго same-origin | `docs/research/2026-09-16-session-security-best-practices.md` (grep по `internal/`) |
| Событийный транспорт: `InProcessDispatcher` — синхронный, in-process, без повторов и durability; лимитации задокументированы в ADR (в т.ч. «will not work correctly if the backend is scaled horizontally») | `internal/platform/events/dispatcher.go`; `docs/adr/0014-in-memory-event-dispatcher.md` |
| Подписки вайрятся в `cmd/api/main.go` (`subscribeUserRegistered`, `subscribeGraceEvents`), тип события — строка, payload — `any` | `apps/backend/cmd/api/main.go:306–330` |
| Контекст `notifications` жив: DirectNotificationService (email+Web Push best-effort), VAPID на месте, per-channel prefs | `internal/notifications/CONTEXT.md`, `application/direct_notification_service.go` |
| **Один инстанс бэка на env**, деплой = простой 5–15 с, принято владельцем | `docs/deployment.md` («При каждом деплое backend недоступен 5–15 секунд (один инстанс)») |
| Caddy на хосте: `encode zstd gzip` на уровне сайта; `handle_path /api/* → reverse_proxy 127.0.0.1:{backend}` — **в проде/stage Next в пути /api не участвует**; фрагмент раскатывается пайплайном с `caddy validate` + graceful reload | `deploy/caddy/rentlee.caddy`; `docs/deployment.md` раздел «Caddy»; сервер v2.11.4 |
| Фронт: клиент ходит на `/api${path}` same-origin; дев-прокси — route handler `app/api/[...path]/route.ts` c **`BACKEND_TIMEOUT_MS = 30000`** (AbortController на upstream-fetch) | `apps/frontend/shared/api/client.ts:24`; `apps/frontend/app/api/[...path]/route.ts:7,37–38` |
| Next **16.3.1**, react-query v5 (`staleTime: 30s`, `refetchOnWindowFocus: false`), реестр ключей `shared/api/query-keys.ts` — контракт кросс-фичевой инвалидации (`xxxKeys.all`) | `apps/frontend/package.json`; `shared/providers/query-provider.tsx`; `shared/api/query-keys.ts`; `apps/frontend/CODING_STANDARDS.md` («react-query conventions») |
| CSP фронта: `connect-src 'self'` — same-origin EventSource разрешён | `apps/frontend/shared/lib/csp.ts:18` |
| Service worker `/api` не перехватывает (pass-through), middleware-файл `proxy.ts` матчит только страницы (`/dashboard/...`) — на `/api` не влияет | `apps/frontend/public/sw.js` (шапка); `apps/frontend/proxy.ts:120–129` |
| `http.Flusher` в цепочке уже уважается логирующей обёрткой, но ни `loggingResponseWriter`, ни otelhttp-обёртка (httpsnoop) не имеют `Unwrap()` — важно для дедлайнов, см. §2.2 | `internal/platform/httpsupport/logging.go:51–67`; otelhttp v0.69.0 `handler.go` (httpsnoop.Wrap, маска методов без `Unwrap`/`SetWriteDeadline`) |

---

## 2. Бэк: SSE-хендлер на chi

### 2.1. Минимальный канон SSE-ответа

По WHATWG HTML Spec (§ server-sent events) и MDN ответ обязан быть `Content-Type: text/event-stream` (иначе браузер «проваливает» соединение и не переподключается автоматически); кадры — UTF-8 текст, поля `event`/`data`/`id`/`retry`, строки, начинающиеся с `:`, — комментарии-keepalive ([MDN: Using server-sent events](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)).

```go
// internal/platform/sse/stream.go — ядро записи кадра (платформенный пакет).
package sse

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Frame — один кадр SSE. JSON собирается в одну строку: SSE режет кадры по \n,
// поэтому внутри data не допускается перевод строки.
type Frame struct {
	ID    string // монотонный per-user seq; пусто — не отправлять
	Event string // имя события (addEventListener на фронте); пусто — generic "message"
	Data  string // готовый JSON, одна строка
}

func (f Frame) bytes() []byte {
	var b strings.Builder
	if f.ID != "" {
		fmt.Fprintf(&b, "id: %s\n", f.ID)
	}
	if f.Event != "" {
		fmt.Fprintf(&b, "event: %s\n", f.Event)
	}
	b.WriteString("data: ")
	b.WriteString(f.Data)
	b.WriteString("\n\n")
	return []byte(b.String())
}

// WriteHeaders ставит заголовки стрима и отменяет серверный write-дедлайн
// для этого соединения (Go >= 1.20, http.NewResponseController).
func WriteHeaders(w http.ResponseWriter) error {
	rc := http.NewResponseController(w)
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		return fmt.Errorf("sse: disable write deadline: %w", err)
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-store") // канон репо: httpsupport.HealthHandler
	h.Set("X-Accel-Buffering", "no")   // nginx-совместимые прокси; Caddy игнорирует — не вред
	w.WriteHeader(http.StatusOK)
	return nil
}

func WriteRetry(w http.ResponseWriter, ms int) {
	fmt.Fprintf(w, "retry: %d\n\n", ms) // подсказка браузеру темпа переподключения
}
```

Заголовок `Connection: keep-alive` в HTTP/2 не имеет смысла и в h1 управляется самим Go — его ставить не нужно.

### 2.2. Главный блокер: `WriteTimeout: 30 * time.Second`

Это **та самая «~30-секундная» грабля, и она не на фронте**. `WriteTimeout` в `net/http` ставит дедлайн на запись ответа с момента принятия заголовков — SSE-соединение умрёт на 30-й секунде в **каждом** окружении (в проде — на прямом пути браузер → Caddy → бэк, мимо Next). Канонический фикс — не трогать глобальный `WriteTimeout`, а снять дедлайн per-connection через `http.NewResponseController(w).SetWriteDeadline(time.Time{})` (Go 1.20+; [Stack Overflow: Http Server Read-Write timeouts and SSE](https://stackoverflow.com/questions/27097084/http-server-read-write-timeouts-and-server-side-events), [Adam Langley-разбор таймаутов: adam-p.ca](https://adam-p.ca/blog/2022/01/golang-http-server-timeouts/)).

**Но в этом репо прямой вызов не сработает.** `ResponseController` добирается до сетевого дедлайна только через интерфейсы `SetWriteDeadline`/`Unwrap() http.ResponseWriter` на цепочке обёрток ResponseWriter. В цепочке репо:

- `loggingResponseWriter` — `Unwrap()` нет (есть только `Flush`/`Hijack`): `internal/platform/httpsupport/logging.go`;
- otelhttp v0.69.0 оборачивает через `httpsnoop.Wrap`, который проксирует **фиксированный** набор интерфейсов (Flusher, Hijacker, Pusher, ReaderFrom…) — `Unwrap` и `SetWriteDeadline` в набор не входят: `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.69.0/handler.go:161–177`.

Варианты решения (для /tdd, в порядке предпочтения):

1. **Добавить `Unwrap() http.ResponseWriter` в `loggingResponseWriter` и проверить цепочку тестом** — если otelhttp-обёртка всё же перекрывает путь, это всплывёт на end-to-end тесте хендлера и придётся звать `SetWriteDeadline` до `otelhttp`… что невозможно — тогда вариант 2.
2. **`WriteTimeout: 0` глобально + явные дедлайны там, где нужны.** Защиту slowloris не теряем: `ReadHeaderTimeout: 5s` и `ReadTimeout: 10s` остаются (главный вектор — медленная отправка запроса); `IdleTimeout: 120s` держит keep-alive-пулы. `WriteTimeout` защищал только от клиентов, не читающих ответ, — при размере продукта и буферизующем Caddy посередине риск приемлем. Это самая простая и честная конфигурация для сервера, на котором живёт стрим.

Рекомендация: в тикете реализации попробовать вариант 1 (он не ослабляет безопасность), при осечке — вариант 2 с комментарием в `main.go` и записью в доку деплоя.

### 2.3. Хаб подписок (per-user fan-out)

`InProcessDispatcher` (ADR 0014) — **синхронный**, dispatch в горутине запроса, семантика «все обработчики по типу события». Для SSE нужен другой примитив — асинхронный хаб с неблокирующей доставкой и политикой медленного потребителя. Это не замена диспетчеру: издатели продолжают публиковать доменные события как сейчас; новый подписчик на диспетчере (или прямой вызов хаба издателем) раскладывает события по открытым стримам.

```go
// internal/platform/sse/hub.go
package sse

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

// Event — событие для доставки конкретному пользователю.
type Event struct {
	UserID uuid.UUID
	Frame  Frame // Event+Data заполнены; ID проставит хаб
}

// subscriber — одно открытое соединение.
type subscriber struct {
	userID uuid.UUID
	ch     chan Frame
}

const (
	subscriberBuffer = 32 // кадры; переполнение = медленный клиент
	// maxConnsPerUser — санитарный лимит (вкладки/устройства/залипшие коннекты).
	maxConnsPerUser = 8
)

// Hub ведёт реестр открытых SSE-соединений и рассылает события.
type Hub struct {
	mu      sync.RWMutex
	byUser  map[uuid.UUID][]*subscriber
	seq     atomic.Uint64 // per-user seq честнее, но global uint64 тоже монотонен и проще
	closeCh chan struct{}
}

func NewHub() *Hub { return &Hub{byUser: make(map[uuid.UUID][]*subscriber)} }

// Subscribe регистрирует соединение и возвращает канал кадров + функцию отписки.
func (h *Hub) Subscribe(userID uuid.UUID) (<-chan Frame, func()) {
	sub := &subscriber{userID: userID, ch: make(chan Frame, subscriberBuffer)}
	h.mu.Lock()
	if conns := h.byUser[userID]; len(conns) >= maxConnsPerUser {
		// вытесняем самый старый коннект — клиент переподключится сам
		close(conns[0].ch)
		h.byUser[userID] = append(conns[1:], sub)
	} else {
		h.byUser[userID] = append(h.byUser[userID], sub)
	}
	h.mu.Unlock()
	return sub.ch, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		conns := h.byUser[userID]
		for i, c := range conns {
			if c == sub {
				h.byUser[userID] = append(conns[:i], conns[i+1:]...)
				break
			}
		}
		if len(h.byUser[userID]) == 0 {
			delete(h.byUser, userID)
		}
	}
}

// Publish — неблокирующая доставка. Медленный потребитель: кадр дропается,
// соединение закрывается — клиент переподключится (EventSource) и перечитает
// состояние через react-query. Никогда не блокирует горутину издателя.
func (h *Hub) Publish(ctx context.Context, e Event) {
	id := strconv.FormatUint(h.seq.Add(1), 10)
	e.Frame.ID = id
	h.mu.RLock()
	conns := h.byUser[e.UserID]
	h.mu.RUnlock()
	for _, c := range conns {
		select {
		case c.ch <- e.Frame:
		default:
			close(c.ch) // slow consumer: дроп+закрытие
		}
	}
}
```

Присоединение к домену: `Hub` — платформенный адаптер; подписка на `events.Dispatcher` (`user_registered`-стиль из `cmd/api/main.go`) или прямой вызов из application-сервисов через узкий порт `StreamPublisher{ Publish(ctx, Event) }`, реализованный хабом. Слойка соблюдена: домен/приложение знают только порт, HTTP-стрим — адаптер в `internal/notifications/adapters/http`.

### 2.4. Хендлер (chi) с auth по cookie-сессии и heartbeat

Канон авторизации — существующий `SessionMiddleware`: он уже кладёт actor в контекст для всех непубличных путей (список `publicSessionSkippedPaths` пополнять **не нужно**). Хендлер лишь проверяет actor и сам пишет 401 (канон `OwnerIDFromContext`: «returns false without writing a response; callers should write their own unauthorized response» — `httpsupport/context.go`).

```go
// internal/notifications/adapters/http/stream_handler.go
package http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nambers/arenda-planform/apps/backend/internal/platform/httpsupport"
	"github.com/nambers/arenda-planform/apps/backend/internal/platform/sse"
)

const (
	heartbeatEvery = 25 * time.Second  // < типовых idle-таймаутов прокси (nginx 60s, CF 100s)
	maxConnTTL     = time.Hour         // пере-аутентификация при переподключении
)

func NewStreamHandler(hub *sse.Hub, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := httpsupport.OwnerIDFromContext(r)
		if !ok {
			httpsupport.WriteProblem(r.Context(), w, http.StatusUnauthorized,
				httpsupport.Problem(r.Context(), "Unauthorized", "Требуется авторизация"))
			return
		}
		if err := sse.WriteHeaders(w); err != nil { // см. §2.2
			httpsupport.WriteProblem(r.Context(), w, http.StatusInternalServerError,
				httpsupport.InternalError(r.Context(), err))
			return
		}
		if f, ok := w.(http.Flusher); ok { f.Flush() }
		sse.WriteRetry(w, 3000) // темп переподключения браузера

		frames, unsubscribe := hub.Subscribe(userID)
		defer unsubscribe()

		// Стартовое событие: клиент понимает, что стрим жив.
		fmt.Fprint(w, "event: connected\ndata: {}\n\n")
		if f, ok := w.(http.Flusher); ok { f.Flush() }

		lastEventID := r.Header.Get("Last-Event-ID") // v1: логируем, replay нет (§6)
		heartbeat := time.NewTicker(heartbeatEvery)
		defer heartbeat.Stop()
		ttl := time.NewTimer(maxConnTTL)
		defer ttl.Stop()

		for {
			select {
			case <-r.Context().Done(): // клиент ушёл
				return
			case frame, open := <-frames:
				if !open { return }
				if _, err := w.Write(frame.Bytes()); err != nil { return }
				if f, ok := w.(http.Flusher); ok { f.Flush() }
			case <-heartbeat.C:
				if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil { return }
				if f, ok := w.(http.Flusher); ok { f.Flush() }
			case <-ttl.C:
				return // закрытие: браузер переподключится, middleware пере-аутентифицирует
			}
		}
	}
}
```

Монтирование: как `/healthz` и admin-оверрайды — ручной роут в `httpserver.New()` (`r.Get("/notifications/stream", handler)` после `generated := openapi.HandlerWithOptions(...)`, «chi matches the last registered route» — комментарий в `server.go`). Контракт-first: путь задокументировать в `api/openapi/openapi.yaml` (GET, ответ `200` с `content: text/event-stream`, `security: [sessionCookie: []]` — схема `sessionCookie` уже есть, `openapi.yaml:3976`) с описанием, что стрим hand-mounted; сможет ли oapi-codegen сгенерировать интерфейсный метод для не-JSON ответа без поломки генерации — проверить в /tdd, фолбэк — стиль `/healthz` (ручной роут вне контракта с комментарием). Путь в контракте: `/notifications/stream` (без `/api` — Caddy префикс снимает, бэк ждёт пути без `/api`: `rentlee.caddy:22–24`).

### 2.5. Last-Event-ID / reconnect

Спека: при переподключении браузер шлёт последний полученный `id` в заголовке `Last-Event-ID`; `id:` без значения сбрасывает его ([WHATWG HTML Spec §9.2.6](https://html.spec.whatwg.org/multipage/server-sent-events.html), [MDN](https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events)). Рекомендация v1: **replay не делать** — потребитель транспорта всё равно «инвалидировать react-query», а на `open` после разрыва фронт перечитывает живое (§5.4). Хендлер читает заголовок и логирует (задел), клиенту это прозрачно. Replay v2 (если понадобится карте #714 для лент): кольцевой буфер последних N событий на пользователя in-memory c выдачей по seq > Last-Event-ID; после рестарта процесса seq сбрасывается — честный сигнал «начни с нуля» (новый `connected`, клиент инвалидирует всё).

---

## 3. Транспорт событий: хватит ли in-process dispatcher

**Сегодня — да.** Факты:

- stage и prod — **по одному инстансу бэка**, это принятое решение владельца (`docs/deployment.md`); ADR 0014 прямо фиксирует, что горизонтальное масштабирование ломает in-process шину — то есть ограничение не только осознано, но и задокументировано как триггер пересмотра.
- Сценарий «несколько реплик API» при текущем пайплайне (GHCR → SSH → compose, `docs/adr/0024`, `docs/adr/0045`) означает изменение compose-топологии, а не появление второй реплики внезапно: момент переключения на N реплик виден заранее, и именно тогда нужен мост.

**Когда реплик станет N — Postgres LISTEN/NOTIFY, не sticky.** Сравнение:

- *Sticky-сессии* (Caddy `lb_policy cookie`) привязывают пользователя к реплике: перекладывают проблему в балансировщик, ломаются на рестартах/деплоях (а деплои частые — 5–15 с простоя уже приняты), не помогают событиям «догнать» пользователя на другой реплике.
- *LISTEN/NOTIFY*: каждый инстанс держит одну выделенную LISTEN-коннекцию (не из пула — канон pgx: [jackc/pgx#1121](https://github.com/jackc/pgx/issues/1121), пакет [`pgxlisten`](https://pkg.go.dev/github.com/jackc/pgxlisten), паттерн Brandur [«Notifier»](https://brandur.org/notifier)); издатель после коммита делает `NOTIFY`, каждый инстанс получает копию и рассылает **своим** локальным коннектам через тот же `Hub`. Лимит payload **8000 байт** ([PostgreSQL NOTIFY](https://www.postgresql.org/docs/current/sql-notify.html)) — слать компактный JSON события (наш envelope §6 — сотни байт) или только id, данные дочитывать. Прозрачный мост: `Hub.Publish` в multi-replica-режиме заменяется на `NOTIFY` + локальный fan-out — интерфейс хаба не меняется.

Границы in-memory решения (оценка на текущий масштаб — десятки пользователей, не тысячи):

- **Память на коннект**: goroutine ≈ 2–8 КБ + буфер канала `subscriberBuffer=32` × размер кадра (~0.5 КБ) + write-буфер → порядок **15–50 КБ на соединение**. 1000 соединений ≈ 15–50 МБ — несущественно. Реальный потолок Go-стрима — десятки тысяч коннектов на инстанс, до него далеко.
- **Медленный потребитель**: ограниченный канал + drop-and-close (§2.3) — рост памяти ограничен конструктивно, клиент сам восстановится.
- **Таймауты**: `ReadTimeout: 10s` — не мешает (тело GET пустое, читается мгновенно); `WriteTimeout` — снять per-connection (§2.2); heartbeat 25 с держит NAT/прокси живыми (postmortem-кейс «соединение умирает через N минут тишины» — [dev.to: SSE still not production ready](https://dev.to/miketalbot/server-sent-events-are-still-not-production-ready-after-a-decade-a-lesson-for-me-a-warning-for-you-2gie)).
- **Деплой**: рестарт бэка рвёт все стримы; `retry: 3000` + перечитывание на `open` делают это невидимым (простой 5–15 с уже принят владельцем). Thundering herd переподключений при маленькой базе — не проблема; джиттер на фронте (§5.3) снимает и его.

Вердикт: **in-memory `Hub` + существующий `InProcessDispatcher` достаточны; LISTEN/NOTIFY — заготовленный мост на момент появления второй реплики; sticky не нужен.**

---

## 4. Reverse-proxy: Caddy

По `deploy/caddy/rentlee.caddy` (прод-путь браузер → Caddy → 127.0.0.1:18080):

- **`reverse_proxy` — менять ничего не нужно.** Официальная документация: дефолт «no periodic flushing», но «the option is ignored and responses are flushed immediately» для ответов с `Content-Type: text/event-stream` и для ответов с неизвестным `Content-Length` ([docs: reverse_proxy, flush_interval](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy)). Наш хендлер шлёт именно `text/event-stream` и без `Content-Length` → Caddy отдает кадры немедленно. Для явности можно добавить `flush_interval -1` в блок `/api/*` (опционально, самодокументация).
- **`encode zstd gzip` — реальный риск буферизации, нужен точечный фикс.** `encode` стоит на уровне сайта **до** `handle`-диспетчеров и оборачивает всю цепочку (`docs/research/2026-09-14-caddy-inversion-best-practices.md`, таблица решений). Дефолтный response-matcher сжатия включает `text/*` — то есть `text/event-stream` — при `minimum_length 512` ([docs: encode](https://caddyserver.com/docs/caddyfile/directives/encode)): как только в стриме накопится ≥512 байт, включится компрессия и начнёт буферизовать кадры (классическая грабля gzip+SSE — [Stack Overflow](https://stackoverflow.com/questions/23769001/is-it-possible-to-use-gzip-compression-with-server-sent-events-sse)). Фикс — исключить стрим из сжатия response-матчером:

  ```caddyfile
  (rentlee_site) {
      encode zstd gzip {
          match {
              not header Content-Type text/event-stream
          }
      }
      ...
      handle_path /api/* {
          reverse_proxy 127.0.0.1:{args[0]}
      }
  ```

- **`X-Accel-Buffering: no` от бэка Caddy игнорирует** — это контракт nginx. Не вредно слать (безопасно для будущих nginx-компоновок), но за буферизацию в Caddy отвечает только `encode`/`flush_interval`, а не этот заголовок.
- Живёт на stage/prod это через штатный конвейер карты #649: правка `deploy/caddy/rentlee.caddy` → пайплайн кладёт фрагмент, `caddy validate`, `systemctl reload caddy` (graceful), откат через `.prev` (`docs/deployment.md`, раздел «Caddy»). Долгоживущие коннекты переживают reload Caddy без разрыва — но деплой бэка их всё равно рвёт, что уже покрыто reconnect-политикой.
- TLS-терминация на Caddy: до браузера HTTP/2 → лимит «6 соединений на домен» (HTTP/1.1) не применяется; при h2 браузер согласовывает до 100 потоков ([MDN: EventSource](https://developer.mozilla.org/en-US/docs/Web/API/EventSource)).

---

## 5. Фронт: Next.js

### 5.1. Топология — прокси нужен только в dev

- **Прод/stage**: `/api/*` перехватывает Caddy и шлёт в бэк (`handle_path /api/*`, префикс снимается) — `rentlee.caddy:23–25`. Next-приложение в пути SSE **не участвует**.
- **Dev/e2e локально**: бэк доступен напрямую, но фронт ходит same-origin `/api/...` → catch-all route handler `app/api/[...path]/route.ts` проксирует на `BACKEND_URL`. Его `BACKEND_TIMEOUT_MS = 30000` (AbortController) и request-scope семантика делают его непригодным для стрима — грабля карты #714 подтверждена кодом: `route.ts:7,37–38`. (Нюанс: AbortController дергается только до получения заголовков апстрима — `clearTimeout` в `finally` срабатывает сразу после конструирования `NextResponse`; но полагаться на это хрупко, и 30-секундный `WriteTimeout` бэка всё равно режет стрим. Чинить надо оба конца.)

**Решение — raw route handler-стрим, отдельный файл.** Next 16 официально поддерживает стриминг raw-ответов из Route Handlers через Web Streams API — «This is useful for Server-Sent Events» (локальные доки `node_modules/next/dist/docs/01-app/02-guides/streaming.md:484–536`). Более специфичный route приоритетнее catch-all `[...path]`, а в проде этот файл всё равно замещён Caddy — **URL один и тот же во всех окружениях**:

```ts
// apps/frontend/app/api/notifications/stream/route.ts
import type { NextRequest } from 'next/server';
import { cookies } from 'next/headers';

// Только dev-прокси: в stage/prod /api/* уходит в бэк мимо Next
// (deploy/caddy/rentlee.caddy). Не буферизуем и не таймаутим.
export const dynamic = 'force-dynamic';

export async function GET(request: NextRequest): Promise<Response> {
  const backend = process.env.BACKEND_URL ?? 'http://localhost:8080';
  const cookieHeader = (await cookies()).toString();

  let upstream: Response;
  try {
    upstream = await fetch(`${backend}/notifications/stream`, {
      headers: {
        accept: 'text/event-stream',
        ...(cookieHeader ? { cookie: cookieHeader } : {}),
      },
      signal: request.signal, // дисконнект клиента доезжает до бэка
      cache: 'no-store',
    });
  } catch {
    return new Response(null, { status: 502 });
  }

  if (!upstream.ok || !upstream.body) {
    return new Response(null, { status: upstream.status }); // 401 и пр. — как есть
  }

  return new Response(upstream.body, {
    status: 200,
    headers: {
      'Content-Type': 'text/event-stream; charset=utf-8',
      'Cache-Control': 'no-store',
      'X-Accel-Buffering': 'no',
    },
  });
}
```

Ключевые отличия от catch-all-прокси: **нет** `AbortController` с 30-с таймаутом (разрыв — только через `request.signal`, т.е. по воле клиента), тело не читается в память — `upstream.body` отдаётся как есть. CSP и security-заголовки из `next.config.ts` на этот путь не мешают (стрим — не документ).

### 5.2. EventSource, не fetch-stream

| Критерий | `EventSource` | `fetch` + ReadableStream |
|---|---|---|
| Автопереподключение + `retry` + `Last-Event-ID` | встроено в браузер по спеке ([WHATWG](https://html.spec.whatwg.org/multipage/server-sent-events.html)) | писать самому |
| Парсинг кадров | встроен | свой парсер (частичные чанки, `id:`/`event:`/`data:`) |
| Кастомные заголовки | нельзя | можно |
| Метод | только GET | любой |

Кастомные заголовки нам не нужны (cookie-сессия, §7) — берём `EventSource`. Полифил не нужен: целевые браузеры проекта (PWA на iOS 16.4+, Android Chrome — контекст Web Push из `internal/notifications/CONTEXT.md`) поддерживают `EventSource` полностью.

### 5.3. Провайдер: подключение, backoff, 401

```tsx
// shared/api/sse/notifications-stream.ts — транспорт без бизнес-логики
export function connectNotificationsStream(
  handlers: Record<string, (payload: unknown) => void>,
  onOpen?: () => void,
): () => void {
  let es: EventSource | null = null;
  let closed = false;
  let attempt = 0;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const open = (): void => {
    es = new EventSource('/api/notifications/stream');
    es.onopen = () => { attempt = 0; onOpen?.(); };
    for (const [name, handler] of Object.entries(handlers)) {
      es.addEventListener(name, (e) => {
        if (!(e instanceof MessageEvent) || typeof e.data !== 'string') return;
        try { handler(JSON.parse(e.data) as unknown); } catch { /* битый кадр игнорируем */ }
      });
    }
    es.onerror = () => {
      // CLOSED = фатально (не-200, напр. 401): браузер сам не переподключится
      if (es && es.readyState === EventSource.CLOSED) {
        es.close();
        if (closed) return;
        const delay = Math.min(30_000, 1000 * 2 ** attempt++) + Math.random() * 1000;
        timer = setTimeout(open, delay); // экспоненциальный backoff + джиттер
      }
      // иначе CONNECTING — браузер уже переподключается сам (retry:)
    };
  };
  open();
  return () => { closed = true; clearTimeout(timer); es?.close(); };
}
```

Обязательный hook: при не-200 соединение «fail the connection» — EventSource **не** переподключается (это спековое поведение, отсюда ручной backoff только для `CLOSED`). Экспоненциальный backoff с джиттером — на случай фатальных ошибок и тихих сетей.

### 5.4. Несколько вкладок и react-query

- **v1: один коннект на вкладку.** Это просто, и даёт кросс-табовую синхронизацию бесплатно (каждая вкладка получает событие и инвалидирует свой кэш). По HTTP/2 (наш прод — TLS на Caddy) лимит соединений на домен не работает (до 100 потоков, [MDN](https://developer.mozilla.org/en-US/docs/Web/API/EventSource)); по HTTP/1.1 потолок 6 на браузер+домен — при 2–4 вкладках не достижим. Санитарный `maxConnsPerUser = 8` на бэке вытесняет старейший коннект — система самовосстанавливается.
- **Апгрейд-путь (когда коннектов станет жалко): лидер через Web Locks + BroadcastChannel.** Канон: единственная вкладка держит `navigator.locks.request('sse-leader', ...)` и EventSource, события раздаются `BroadcastChannel('rentli-sse')`, при закрытии лидера лок автоматом переходит следующей вкладке ([Leader election in browser tabs](https://greenvitriol.com/posts/browser-leader), [tab-election](https://github.com/dabblewriter/tab-election), [MDN: Broadcast Channel API](https://developer.mozilla.org/en-US/blog/exploring-the-broadcast-channel-api-for-cross-tab-communication/)). В v1 не делать — преждевременно.
- **Интеграция с react-query — по канону репо**: мутации инвалидируют через реестр `shared/api/query-keys.ts` (`void invalidateQueries({ queryKey: xxxKeys.all })`, fire-and-forget — `apps/frontend/CODING_STANDARDS.md`). SSE-событие — тот же триггер: словарь `тип события → набор ключей` живёт рядом с реестром ключей; `onopen` (в т.ч. после каждого переподключения) инвалидирует всё «живое» — это заменяет replay-буфер (§2.5).

```tsx
// shared/providers/notifications-stream-provider.tsx — на уровне QueryProvider
'use client';
import { useQueryClient } from '@tanstack/react-query';
import { useEffect, type ReactElement } from 'react';
import { connectNotificationsStream } from '@/shared/api/sse/notifications-stream';
import { accessKeys, notificationKeys, propertyKeys, taskKeys } from '@/shared/api/query-keys';

// Словарь «тип → что инвалидирует»: единое место контракта транспорта.
const INVALIDATIONS: Record<string, readonly string[][]> = {
  'notification.created': [[...notificationKeys.all]],
  'feed.changed': [[...notificationKeys.all]],
  'access.updated': [[...accessKeys.all], [...propertyKeys.all]],
};

export function NotificationsStreamProvider({ children }: Readonly<{ children: React.ReactNode }>): ReactElement {
  const queryClient = useQueryClient();
  useEffect(() => {
    const handlers = Object.fromEntries(
      Object.keys(INVALIDATIONS).map((type) => [type, () => {
        for (const keys of INVALIDATIONS[type] ?? []) {
          void queryClient.invalidateQueries({ queryKey: keys });
        }
      }]),
    );
    return connectNotificationsStream(handlers, () => {
      void queryClient.invalidateQueries(); // после разрыва: перечитать живое
    });
  }, [queryClient]);
  return <>{children}</>;
}
```

Тосты/счётчик непрочитанных карты #734 — такие же подписчики на события поверх этого же провайдера (через контекст/стор фичи), транспорт остаётся «глупым».

---

## 6. Формат envelope

Требования: пережить карту #714 (realtime участников/истории — те же стримы, другие типы), версионирование без поломки старых клиентов, читаемость в DevTools.

### 6.1. Кадр SSE

```
retry: 3000

event: connected
data: {}

id: 42
event: notification.created
data: {"v":1,"occurredAt":"2026-09-17T09:15:03Z","payload":{"id":"0198…","category":"subscription_grace","title":"Оплата не прошла","body":"Пополните баланс до 20.09","url":"/subscription"}}

: ping

id: 43
event: access.updated
data: {"v":1,"occurredAt":"2026-09-17T09:15:20Z","payload":{"propertyId":"0198…"}}
```

- `id` — **монотонная последовательность хаба** (строка числа): корректно работает с `Last-Event-ID` и будущим replay (§2.5). UUID не годится для replay-курсора — не упорядочен.
- `event` — стабильное **грубое** имя типа; транспорт семантики «что перечитать», а не «вот дельта состояния». Клиенты слушают через `addEventListener`; неизвестные имена браузер молча игнорирует (нет листенера — нет события) — **добавление новых типов обратно совместимо по определению**.
- `data` — JSON одной строкой (SSE режет кадры по `\n`, многострочный `data:` технически возможен, но не нужен).

### 6.2. JSON-envelope внутри `data`

```jsonc
{
  "v": 1,                            // версия схемы payload'а
  "occurredAt": "2026-09-17T09:15:03Z", // RFC3339 UTC, timestamptz-канон репо
  "payload": { /* свой на каждый тип, только id + дисплейные поля */ }
}
```

Правила версионирования:

1. **Имя типа не меняется никогда.** Новая семантика — новое имя (`history.appended` рядом с `feed.changed`).
2. **Эволюция — аддитивная**: новые поля в `payload` не ломают старых клиентов (они читают подмножество).
3. **Ломающее изменение payload'а — подъём `v`** (2 вместо 1) при том же имени или новое имя с суффиксом; фронт-обработчик игнорирует чужие `v`. Для мелкого продукта достаточно `v`-поля: подписчиков двое (#734, #714), рассинхронизация версий видна сразу.
4. Никаких доменных сущностей целиком — `payload` несёт id (и дисплейные поля для тостов); состояние клиент всегда дочитывает через react-query. Так транспорт не становится второй системой правды и не конфликтует с `docs/adr/0036` (деньги и доменные факты — не в кадрах стрима).

### 6.3. Стартовый реестр типов (карта #734; #714 допишет своих)

| `event` | payload (минимум) | потребители |
|---|---|---|
| `connected` | `{}` | подтверждение живости |
| `notification.created` | `id`, `category`, `title`, `body`, `url` | тост, счётчик, лента |
| `feed.changed` | — (триггер) | лента, счётчик |
| `notification.read` / `notification.deleted` | `id` | кросс-таб синк ленты |
| `access.updated` (#714) | `propertyId` | участники, доступы |
| `history.appended` (#714) | `propertyId`, `entryId` | живая история |

---

## 7. Auth при коннекте

- **EventSource шлёт cookies автоматически** — запрос same-origin, `withCredentials` не нужен вовсе (он только для cross-origin, [MDN](https://developer.mozilla.org/en-US/docs/Web/API/EventSource)). `SameSite=Lax` (ADR 0018) не мешает: Lax отсекает cookie только в *кросс-сайтовых* запросах, а наш стрим — same-site.
- **CORS не нужен**: в бэке его нет вообще, API строго same-origin (`docs/research/2026-09-16-session-security-best-practices.md`). CSP фронта `connect-src 'self'` разрешает same-origin EventSource (`shared/lib/csp.ts:18`).
- **Канон сессии сохраняется полностью**: `SessionMiddleware` отрабатывает на стриме как на обычном GET (путь не в `publicSessionSkippedPaths`), actor попадает в контекст, хендлер пишет 401 по канону `OwnerIDFromContext`. Нюанс: сессия, проверенная при коннекте, не перепроверяется до конца жизни соединения — закрывается `maxConnTTL = 1 час` (§2.4) плюс ревокация сессии не рвёт открытый стрим немедленно (приемлемо: стрим — read-only триггеры; при необходимости ужесточения — проверить сессию в heartbeat-тике).
- Дев-путь: route handler прокидывает cookie из `cookies()` в upstream-fetch (тот же приём, что в catch-all прокси `route.ts:23–30`) — бэк видит ту же сессию.

---

## 8. Вердикт: рекомендуемый контракт транспорта

| Параметр | Решение |
|---|---|
| Эндпоинт (браузер) | `GET /api/notifications/stream`, same-origin, только EventSource |
| Эндпоинт (бэк) | `GET /notifications/stream` (Caddy снимает `/api`), hand-mounted в `httpserver.New` по прецеденту `/healthz`, контракт — в `openapi.yaml` с `text/event-stream` |
| Auth | cookie-сессия через существующий `SessionMiddleware`; 401 problem+json до старта стрима |
| Формат | SSE: `retry: 3000` на старте; `id` = монотонный seq хаба; `event` = стабильное грубое имя; `data` = JSON `{v, occurredAt, payload}`; heartbeat `: ping` каждые 25 с; стартовое `connected` |
| Replay/Last-Event-ID | v1 — заголовок логируется, replay нет; `onopen` → инвалидации react-query. Replay-буфер — v2 по потребности #714 |
| Жизненный цикл | heartbeat 25 с; `maxConnTTL` 1 ч (пере-аутентификация); дедлайн записи снят per-connection; медленный потребитель — drop+close; лимит 8 коннектов/пользователь |
| Издатели | существующий `events.Dispatcher` → адаптер-мост в `Hub` (или порт `StreamPublisher`); новые типы — по реестру §6.3 |
| Прокси | Caddy: исключить `text/event-stream` из `encode` (response-matcher); `flush_interval` не обязателен (event-stream флешится сразу); `X-Accel-Buffering: no` — просто как совместимость |
| Фронт | EventSource-обёртка в `shared/api/sse`, провайдер рядом с `QueryProvider`, словарь «тип → ключи инвалидации» у реестра `query-keys.ts`; дев-стрим — raw route handler `app/api/notifications/stream/route.ts` (в проде замещён Caddy) |
| Масштабирование | один инстанс → in-memory `Hub`; N реплик → LISTEN/NOTIFY-мост (выделенная pgx-коннекция, payload < 8000 Б), sticky не нужен |

**Порядок внедрения** (для будущих тикетов): 1) `WriteTimeout`-фикс + `Hub` + хендлер бэка (+/- контракт) — TDD на хаб и хендлер; 2) правка Caddy-фрагмента (едет пайплайном); 3) дев-route handler + EventSource-провайдер + инвалидации; 4) первые издатели карты #734 (`notification.created` из DirectNotificationService-пайплайна).

**Риски**

| Риск | Митига |
|---|---|
| `WriteTimeout`/обёртки: `SetWriteDeadline` может не пробиться через otelhttp (httpsnoop без `Unwrap`) | Тест цепочки в /tdd; фолбэк `WriteTimeout: 0` с сохранением `ReadHeaderTimeout`/`ReadTimeout`/`IdleTimeout` |
| Caddy-фрагмент с encode-матчером: регресс сжатия обычных ответов | Матчер только на `Content-Type: text/event-stream`; smoke `curl -H 'Accept-Encoding: gzip' -N` на стрим + обычный JSON в деплой-чеке |
| oapi-codegen и не-JSON ответ | Если генерация не переварит `text/event-stream` — hand-mount по прецеденту `/healthz`, путь всё равно описан в yaml для человека |
| Зависание стрима «всё живо, но тихо» (клиент не видит разрыв) | heartbeat 25 с — клиент ловит разрыв на чтении; плюс `onopen`-инвалидации компенсируют пропуски |
| Сессия истекла, стрим ещё жив (до 1 ч) | Стрим read-only триггеров; ужесточение — проверка сессии в heartbeat-тике, если владелец попросит |
| Дев-режим: `next dev` исторически буферизовал стримы в старых версиях | Next 16 документирует стриминг route handlers как поддерживаемый; приёмка `/ui-walkthrough` на реальном стеке покрывает |
| Thundering herd переподключений после деплоя | `retry: 3000` + клиентский джиттер; масштаб продукта это не напрягает |

---

## 9. Источники

**Код и доки репозитория** (ветка `dev`, be1c9e0a): `apps/backend/cmd/api/main.go`; `apps/backend/internal/platform/httpserver/server.go`; `apps/backend/internal/platform/httpsupport/{session,context,logging,health}.go`; `apps/backend/internal/platform/events/dispatcher.go`; `apps/backend/internal/notifications/CONTEXT.md`; `apps/backend/api/openapi/openapi.yaml`; `apps/backend/internal/platform/config/config.go`; `deploy/caddy/rentlee.caddy`; `docs/deployment.md`; `docs/adr/0004/0011/0014/0018`; `docs/research/2026-09-16-session-security-best-practices.md`; `docs/research/2026-09-14-caddy-inversion-best-practices.md`; `apps/frontend/app/api/[...path]/route.ts`; `apps/frontend/shared/api/{client.ts,query-keys.ts}`; `apps/frontend/shared/lib/csp.ts`; `apps/frontend/shared/providers/query-provider.tsx`; `apps/frontend/proxy.ts`; `apps/frontend/public/sw.js`; `apps/frontend/CODING_STANDARDS.md`; локальные доки Next 16.3.1 (`node_modules/next/dist/docs/01-app/02-guides/streaming.md`).

**Внешние первоисточники:**

- WHATWG HTML Spec, Server-sent events (reconnection time, `retry`, `Last-Event-ID`, fail-connection) — https://html.spec.whatwg.org/multipage/server-sent-events.html
- MDN: EventSource (лимиты соединений h1/h2, same-origin/credentials) — https://developer.mozilla.org/en-US/docs/Web/API/EventSource
- MDN: Using server-sent events (формат кадров, keepalive-комментарии) — https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events/Using_server-sent_events
- Caddy docs: reverse_proxy / `flush_interval` (auto-flush для `text/event-stream`) — https://caddyserver.com/docs/caddyfile/directives/reverse_proxy
- Caddy docs: encode (`minimum_length 512`, дефолтный матчер `text/*`) — https://caddyserver.com/docs/caddyfile/directives/encode
- Caddy community: SSE buffering with reverse_proxy — https://caddy.community/t/server-sent-events-buffering-with-reverse-proxy/11722
- Stack Overflow: Http Server Read-Write timeouts and SSE (`SetWriteDeadline(time.Time{})`) — https://stackoverflow.com/questions/27097084/http-server-read-write-timeouts-and-server-side-events
- adam-p.ca: Diving into Go's HTTP server timeouts — https://adam-p.ca/blog/2022/01/golang-http-server-timeouts/
- Next.js 16 docs: Streaming in Route Handlers (Web Streams, SSE) — https://nextjs.org/docs/app/guides/streaming
- PostgreSQL docs: NOTIFY (payload < 8000 bytes) — https://www.postgresql.org/docs/current/sql-notify.html
- jackc/pgx#1121: dedicated LISTEN connection — https://github.com/jackc/pgx/issues/1121; pgxlisten — https://pkg.go.dev/github.com/jackc/pgxlisten
- Brandur: Notifier pattern — https://brandur.org/notifier
- Postmortem: SSE и тихие разрывы прокси — https://dev.to/miketalbot/server-sent-events-are-still-not-production-ready-after-a-decade-a-lesson-for-me-a-warning-for-you-2gie
- Leader election: Web Locks + BroadcastChannel — https://greenvitriol.com/posts/browser-leader, https://github.com/dabblewriter/tab-election
- MDN: Broadcast Channel API — https://developer.mozilla.org/en-US/blog/exploring-the-broadcast-channel-api-for-cross-tab-communication/
