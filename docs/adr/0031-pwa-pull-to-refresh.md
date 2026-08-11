# ADR 0031: PWA pull-to-refresh — кастомный жест в standalone

В standalone-PWA кабинета нет UI-способа перезагрузить приложение: на iOS WebKit физически отключает нативный pull-to-refresh для `display: standalone/fullscreen`, а на Android нативный PTR работает, но UX расходится с iOS. Решено реализовать **собственный PTR-жест на `touch`-событиях**, активный **только в standalone** (`isStandaloneMode()`), с единым поведением на всех платформах; по срабатыванию — `window.location.reload()`. Нативный Android-PTR не подавляем глобально (в standalone он и так не показывается).

## Considered Options

1. **Кастомный PTR-жест в standalone (выбрано).** Touch-обработка на `window`, простой CSS-спиннер, активация только при `scrollTop <= 0`. Поведение идентично на iOS/Android/desktop-тач.
2. *Нативный PTR везде.* Невозможно: Apple отключает нативный PTR в standalone намертво, CSS/мета-тегами не включается ([SO #75972895](https://stackoverflow.com/questions/75972895/ios-pwa-how-to-re-enable-pull-to-refresh)).
3. *Гибрид — нативный на Android + кастомный на iOS.* Два разных UX на двух платформах;Feature-detect «есть ли нативный PTR» ненадёжен; отказались ради единообразия.
4. *In-app кнопка «обновить» + SW-update-баннер вместо жеста.* Решает корневую причину «старая версия после деплоя», но не даёт интуитивного триггера, который теряет iOS-пользователь (смахивание PWA из переключателя задач — единственный способ сегодня).
5. *Готовая библиотека (`react-simple-pull-to-refresh` и т.п.).* Отвергнута: чужой UX сложно подогнать под дизайн-систему, + новая зависимость ради ~150 строк.

PTR не решает проблему активации нового service worker после деплоя (`skipWaiting`/`controllerchange`) — это отдельная задача, реализованная в [ADR 0032](./0032-pwa-service-worker-silent-update-on-navigation.md).

## Consequences

- **Где:** `apps/frontend/shared/ui/pull-to-refresh/` (компонент `PullToRefresh` + `*.module.css`), монтируется в `widgets/cabinet-layout/ui/CabinetLayout.tsx` рядом с `ServiceWorkerRegister`.
- **Активация:** только при `isStandaloneMode() === true`. В обычном браузере кастомный PTR не монтируется — пользователь получает нативное поведение платформы.
- **Механика:** `touchstart`/`touchmove`/`touchend` на `window`; порог срабатывания 70px, сопротивление 0.5 (визуальная дельта = `realDelta × 0.5`); старт только при `scrollTop <= 0`. Параметры — константы в начале модуля.
- **Действие по срабатывании:** `window.location.reload()`. «Мягкий» refresh через `queryClient.invalidateQueries()` сознательно не делаем — он не подтянет новый деплой.
- **Индикатор:** простой кольцевой CSS-спиннер (`@keyframes spin`), без текста/эмодзи, цвет `--color-accent` (`#2b7fff`). Три фазы: тянет (прозрачность 0→1) → порог пройден (активное состояние) → отпускание (reload ~300мс / пружинный возврат). Фолбэк-кандидат для реализации — HeroUI `Spinner`.
- **Конфликт со скроллом:** скролл документовый (`window`), вложенных контейнеров нет; PTR читает `document.documentElement.scrollTop` / `window.scrollY` — тот же контейнер, с которым работает `ScrollToTop`. Координация не требуется (PTR слушает тач, `ScrollToTop` — смену `pathname`).
- **Доступность:** PTR — supplementary mobile-only жест; клавиатурного/скринридер-дубликата нет (дублирующий способ — перезапуск PWA / обновление вкладки). `aria-live` не добавляем до явного требования accessibility-аудита.
- **`overscroll-behavior`:** глобально не меняем. Если при тесте на реальном Android-устройстве в standalone проявится отскок страницы параллельно с кастомным PTR — точечно добавим `overscroll-behavior-y: contain` через условный класс на `documentElement` на время жеста.
