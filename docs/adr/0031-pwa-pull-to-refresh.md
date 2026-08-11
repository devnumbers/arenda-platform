# ADR 0031: PWA pull-to-refresh — кастомный жест в standalone

В standalone-PWA кабинета нет UI-способа обновить данные: на iOS WebKit физически отключает нативный pull-to-refresh для `display: standalone/fullscreen`, а на Android нативный PTR работает, но UX расходится с iOS. Решено реализовать **собственный PTR-жест на `touch`-событиях**, активный **только в standalone** (`isStandaloneMode()`), с единым поведением на всех платформах. По срабатывании жест делает **мягкий refresh данных** (`queryClient.invalidateQueries()`), а **не** `window.location.reload()` — потому что ответственность за подтягивание нового деплоя берёт на себя [ADR 0032](./0032-pwa-service-worker-silent-update-on-navigation.md) (silent SW-update-lifecycle), и дублировать её в PTR нет смысла. Нативный Android-PTR не подавляем глобально (в standalone он и так не показывается).

**Разделение ответственностей (после ADR 0032):**

| Механизм | Что обновляет | Триггер |
|---|---|---|
| `ServiceWorkerUpdater` (ADR 0032) | **Код приложения** (новый JS/CSS/SW) | смена роута / `visibilitychange → hidden` → `controllerchange` → reload |
| `PullToRefresh` (этот ADR) | **Данные на экране** (react-query cache) | жест pull-down в standalone |

## Considered Options

1. **Кастомный PTR-жест в standalone (выбрано).** Touch-обработка на `window`, простой CSS-спиннер, активация только при `scrollTop <= 0`. Поведение идентично на iOS/Android/desktop-тач.
2. *Нативный PTR везде.* Невозможно: Apple отключает нативный PTR в standalone намертво, CSS/мета-тегами не включается ([SO #75972895](https://stackoverflow.com/questions/75972895/ios-pwa-how-to-re-enable-pull-to-refresh)).
3. *Гибрид — нативный на Android + кастомный на iOS.* Два разных UX на двух платформах; feature-detect «есть ли нативный PTR» ненадёжен; отказались ради единообразия.
4. *In-app кнопка «обновить» + SW-update-баннер вместо жеста.* Не даёт интуитивного триггера, которого лишён iOS-пользователь в standalone.
5. *Готовая библиотека (`react-simple-pull-to-refresh` и т.п.).* Отвергнута: чужой UX сложно подогнать под дизайн-систему, + новая зависимость ради ~150 строк.

### Действие по срабатывании: soft-refresh vs hard-reload

Изначально (до ADR 0032) было решено `window.location.reload()` с обоснованием «мягкий refresh не подтянет новый деплой». После реализации [ADR 0032](./0032-pwa-service-worker-silent-update-on-navigation.md) это обоснование устарело: новый деплой теперь приезжает сам через silent SW-update-lifecycle. PTR переориентирован на **черезвычайную обязанность, которая осталась незакрытой** — дать пользователю способ получить свежие данные сейчас (react-query cache: `staleTime: 30s`, `refetchOnWindowFocus: false` — без PTR данные не обновятся, пока пользователь не сделает мутирующее действие или не перезагрузит страницу).

- **Мягкий refresh (выбрано):** `queryClient.invalidateQueries()` без аргументов → помечает все запросы stale и рефетчит активные (`refetchType: 'active'` по умолчанию). Экран перерисовывается с актуальными данными, **состояние UI сохраняется** (скролл, модалки, незавершённая форма). Это классический смысл PTR в нативных приложениях.
- *Hard-reload (`location.reload()`).* Теряет UI-состояние, перерисовывает весь React. Теперь избыточен — SW-updater уже делает reload при новом деплое.
- *Гибрид (soft обычно, hard если есть waiting SW).* Связывает два механизма, ломает разделение ответственностей, усложняет UX (пользователь не понимает, потянет ли он «обновление данных» или «перезагрузку приложения»).

## Consequences

- **Где:** `apps/frontend/shared/ui/pull-to-refresh/` (компонент `PullToRefresh` + `*.module.css`), монтируется в `widgets/cabinet-layout/ui/CabinetLayout.tsx` рядом с `ServiceWorkerRegister`/`ServiceWorkerUpdater`.
- **Активация:** только при `isStandaloneMode() === true`. В обычном браузере кастомный PTR не монтируется — пользователь получает нативное поведение платформы.
- **Механика жеста:** `touchstart`/`touchmove`/`touchend` на `window`; порог срабатывания 70px, сопротивление 0.5 (визуальная дельта = `realDelta × 0.5`); старт только при `scrollTop <= 0`. Параметры — константы в начале модуля.
- **Действие по срабатывании:** `queryClient.invalidateQueries()` (без аргументов). Выстрелил-и-забыл — не завязываемся на Promise, индикатор держим по `useIsFetching()` (см. ниже).
- **Индикатор обновления:** простой кольцевой CSS-спиннер (`@keyframes spin`), без текста/эмодзи, цвет `--color-accent` (`#2b7fff`). Три фазы: тянет (прозрачность 0→1) → порог пройден (активное состояние) → отпускание. Спиннер **висит, пока `useIsFetching() > 0`**, но не дольше верхней границы **3 сек** (защита от зависшего бэкенда/4 retry с backoff); затем пропадает независимо. Фолбэк-кандидат для реализации — HeroUI `Spinner`.
- **Обработка ошибок:** отдельной error-семантики от PTR нет. Упавшие запросы попадают в стандартный error-flow своих виджетов (`FinanceErrorState onRetry`, и т.д.) — PTR не дублирует их тостом. Старые данные остаются в кеше как stale.
- **Координация с `ServiceWorkerUpdater` (ADR 0032):** **не координируется.** Механизмы ортогональны: PTR обновляет данные, SW-updater — код. Если в момент срабатывания PTR активировался waiting SW и триггерит reload — он выигрывает гонку естественным образом, soft-refresh обрывается; пользователь не замечает разницы. Waiting SW активируется на смене роута/visibility, а не в момент жеста → коллизия крайне редка.
- **Конфликт со скроллом:** скролл документовый (`window`), вложенных контейнеров нет; PTR читает `document.documentElement.scrollTop` / `window.scrollY` — тот же контейнер, с которым работает `ScrollToTop`. Координация не требуется (PTR слушает тач, `ScrollToTop` — смену `pathname`).
- **Доступность:** PTR — supplementary mobile-only жест; клавиатурного/скринридер-дубликата нет (дублирующий способ — перезапуск PWA / обновление вкладки). `aria-live` не добавляем до явного требования accessibility-аудита.
- **`overscroll-behavior`:** глобально не меняем. Если при тесте на реальном Android-устройстве в standalone проявится отскок страницы параллельно с кастомным PTR — точечно добавим `overscroll-behavior-y: contain` через условный класс на `documentElement` на время жеста.
