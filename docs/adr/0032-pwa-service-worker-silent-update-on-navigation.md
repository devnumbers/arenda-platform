# ADR 0032: PWA service worker — silent-обновление по смене маршрута / скрытию

После деплоя новый service worker скачивался и садился в состояние `waiting`, но страница никогда не отправляла `SKIP_WAITING` — обработчик в `public/sw.js:288-293` оставался мёртвым, и новая версия не приезжала, пока пользователь не закрывал все вкладки. Решено реализовать **silent** SW-update-lifecycle (без UI, без тоста): новый SW активируется строго по триггерам **смены маршрута** (`usePathname`) ИЛИ **скрытия приложения** (`visibilitychange → hidden`), а reload происходит при смене маршрута или при возврате пользователя (`visibilitychange → visible`).

## Considered Options

1. **Silent, активация по смене маршрута / скрытию (выбрано, «B3b»).** Новый SW скачивается и ставится тихо. `SKIP_WAITING` отправляется только когда пользователь перешёл на другой экран (значит, не в середине формы) ИЛИ когда приложение свернулось / вкладка потеряла фокус. Reload — сразу при смене маршрута, либо при возврате, если контроллер сменился в скрытом состоянии. Без UI вообще.
2. *Silent auto-reload (как Vite PWA `autoUpdate`).* `skipWaiting` в install + auto-reload на `controllerchange`. Отвергнуто: может выстрелить во время ввода формы. Workbox-гайд не рекомендует для приложений с формами ([Handling SW updates](https://developer.chrome.com/docs/workbox/handling-service-worker-updates)).
3. *Тост с кнопкой «Обновить».* Показывается при `updatefound`. Пользователь не хочет UI; к тому же драфты форм уже сохраняются в `sessionStorage` (см. `widgets/leases/lib/use-lease-edit-draft.ts` и аналоги) — данные выживут даже при внезапном reload.
4. *Тост + авто-применение при смене маршрута.* Избыточно: либо тост даёт контроль, либо автоматика — оба механизма вместе конфликтуют.
5. *Только смена маршрута, без `visibilitychange` («B3a»).* Не покрывает кейс «открыл дашборд и ушёл на весь день»: обновление не приедет, пока пользователь не сходит по меню. Добавление `visibilitychange`-триггера закрывает этот сценарий.

## Decision

**Паттерн активации** (страница `apps/frontend/shared/lib/pwa/ServiceWorkerUpdater.tsx`):

```
updatefound → registration.installing statechange → state === 'installed'
  → pendingSkipWaiting = true

Триггеры отправки SKIP_WAITING (любой из):
  - usePathname() сменился (пользователь перешёл на другой экран)
  - visibilitychange → document.visibilityState === 'hidden'
  → registration.waiting?.postMessage({ type: 'SKIP_WAITING' })

controllerchange:
  - document.hidden → needsReloadOnVisible = true (отложенный reload)
  - иначе → window.location.reload() (с guard isReloading)

visibilitychange → document.visibilityState === 'visible':
  - needsReloadOnVisible → window.location.reload() (с guard isReloading)
```

**Периодическая проверка:** `setInterval(() => registration.update(), 30 * 60 * 1000)` — каждые 30 минут (браузер сам троттлит лишние проверки; по умолчанию без этого браузер проверяет только при навигации + раз в 24ч).

**Гварды:**
- `navigator.serviceWorker.controller === null` при старте → компонент не активирует логику (первый визит: SW только регистрируется, не релоадим в лицо).
- `isReloading`-флаг против двойного reload (многовкладочность: `controllerchange` стреляет во всех вкладках).
- `registration` получается через `navigator.serviceWorker.ready` (Promise) — не зависит от того, кто зарегистрировал SW; развязка с `ServiceWorkerRegister`.

**Получатель `SKIP_WAITING`** уже реализован в `public/sw.js:288-293` и не меняется. Drift-guard в `shared/lib/pwa/service-worker-updater.test.ts` ловит рассинхрон «страница шлёт → SW принимает» (тот класс багов, что привёл к этому ADR).

## Consequences

- **Где:** `apps/frontend/shared/lib/pwa/ServiceWorkerUpdater.tsx` (новый), монтируется в `widgets/cabinet-layout/ui/CabinetLayout.tsx` рядом с `ServiceWorkerRegister`. `public/sw.js` не трогается.
- **Когда приезжает обновление:** в течение 30 минут после деплоя (периодическая проверка) + при следующей смене маршрута / следующем сворачивании PWA. Для пользователей, не покидающих экран и не сворачивающих приложение весь день — на следующей навигации или при следующем запуске (браузер перерегистрирует SW сам). Это компромисс silent-режима без UI.
- **Безопасность форм:** триггер смены маршрута гарантирует, что пользователь не в середине ввода; `visibilitychange → hidden` означает, что пользователь уже ушёл. Драфты форм в `sessionStorage` — дополнительная подстраховка: данные выживают даже при редком стечении обстоятельств.
- **Многовкладочность:** `controllerchange` стреляет во всех вкладках кабинета одновременно; все они делают reload. `isReloading`-guard предотвращает двойной reload в одной вкладке.
- **iOS ITP (7-дневная очистка):** SW может быть удалён; при следующем открытии `ServiceWorkerRegister` пере-регистрирует его автоматически. Первый визит после очистки игнорируется гардом `navigator.serviceWorker.controller === null` — релоада в лицо не будет.

## Open verification items

- (а) Проверить на реальном iOS-устройстве поведение SW-update в рамках ITP-7дн: DevTools → Application → Service Workers, симулировать `updatefound` и убедиться, что `controllerchange` стрелляет корректно в standalone-режиме.
- (б) Подтвердить, что Caddy не срывает `Cache-Control: no-store` на `/sw.js` (`curl -I https://dev.rentlee.ru/sw.js` на stage) — иначе браузер не увидит новый SW и вся цепочка не запустится. Перенесено из `docs/research/pwa-manifest-installability.md` §4.
